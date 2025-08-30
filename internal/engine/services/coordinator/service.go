package coordinator

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"github.com/w-h-a/workflow/internal/engine/clients/reader"
	"github.com/w-h-a/workflow/internal/engine/clients/readwriter"
	"github.com/w-h-a/workflow/internal/task"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Service struct {
	broker     broker.Broker
	readwriter readwriter.ReadWriter
	queues     map[string]int
	locks      map[string]*sync.RWMutex
	mtx        sync.RWMutex
	tracer     trace.Tracer
}

func (s *Service) Start(ch chan struct{}) error {
	for name, concurrency := range s.queues {
		for range concurrency {
			opts := []broker.SubscribeOption{
				broker.SubscribeWithQueue(name),
			}

			if err := s.broker.Subscribe(context.Background(), s.handleTask, opts...); err != nil {
				return err
			}
		}
	}

	<-ch

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer shutdownCancel()

	if err := s.broker.Close(shutdownCtx); err != nil {
		slog.ErrorContext(shutdownCtx, "broker shutdown failed", "error", err)
	}

	return nil
}

func (s *Service) RetrieveTasks(ctx context.Context, page, size int) (*TasksWithMetadata, error) {
	ctx, span := s.tracer.Start(ctx, "Coordinator.RetrieveTasks", trace.WithAttributes(
		attribute.Int("page", page),
		attribute.Int("size", size),
	))
	defer span.End()

	opts := []reader.ReadOption{
		reader.ReadWithPage(page),
		reader.ReadWithSize(size),
	}

	rp, err := s.readwriter.Read(ctx, opts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	tasks := make([]*task.Task, 0, len(rp.Items))

	for _, bs := range rp.Items {
		task, _ := task.Factory(bs)
		tasks = append(tasks, task)
	}

	span.SetStatus(codes.Ok, "tasks retrieved successfully")

	return &TasksWithMetadata{
		Tasks:      tasks,
		Size:       rp.Size,
		Number:     rp.Number,
		TotalPages: rp.TotalPages,
		TotalTasks: rp.TotalItems,
	}, nil
}

func (s *Service) RetrieveTask(ctx context.Context, id string) (*task.Task, error) {
	ctx, span := s.tracer.Start(ctx, "Coordinator.RetrieveTask", trace.WithAttributes(
		attribute.String("task.id", id),
	))
	defer span.End()

	t, err := s.retrieveTask(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "task retrieved successfully")

	return t, nil
}

func (s *Service) ScheduleTask(ctx context.Context, t *task.Task) (*task.Task, error) {
	ctx, span := s.tracer.Start(ctx, "Coordinator.ScheduleTask")
	defer span.End()

	t.ID = strings.ReplaceAll(uuid.NewString(), "-", "")
	span.SetAttributes(attribute.String("task.id", t.ID))

	now := time.Now()

	t.State = task.Scheduled
	t.ScheduledAt = &now
	t.Result = ""
	t.Error = ""
	t.StartedAt = nil
	t.CancelledAt = nil
	t.CompletedAt = nil
	t.FailedAt = nil

	if err := s.persistAndPublish(ctx, t, string(task.Scheduled)); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "task scheduled successfully")

	return t, nil
}

func (s *Service) CancelTask(ctx context.Context, id string) (*task.Task, error) {
	ctx, span := s.tracer.Start(ctx, "Coordinator.CancelTask", trace.WithAttributes(
		attribute.String("task.id", id),
	))
	defer span.End()

	taskLock := s.retrieveTaskLock(id)

	taskLock.Lock()
	defer taskLock.Unlock()

	t, err := s.retrieveTask(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if t.State != task.Started {
		span.RecordError(task.ErrTaskNotCancellable)
		span.SetStatus(codes.Error, task.ErrTaskNotCancellable.Error())
		return nil, task.ErrTaskNotCancellable
	}

	now := time.Now()

	t.State = task.Cancelled
	t.CancelledAt = &now

	if err := s.persistAndPublish(ctx, t, string(task.Cancelled)); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "task cancelled successfully")

	return t, nil
}

func (s *Service) RestartTask(ctx context.Context, id string) (*task.Task, error) {
	ctx, span := s.tracer.Start(ctx, "Coordinator.RestartTask", trace.WithAttributes(
		attribute.String("task.id", id),
	))
	defer span.End()

	taskLock := s.retrieveTaskLock(id)

	taskLock.Lock()
	defer taskLock.Unlock()

	t, err := s.retrieveTask(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if t.State == task.Scheduled || t.State == task.Started {
		span.RecordError(task.ErrTaskNotRestartable)
		span.SetStatus(codes.Error, task.ErrTaskNotRestartable.Error())
		return nil, task.ErrTaskNotRestartable
	}

	if len(t.Mounts) > 0 {
		for _, m := range t.Mounts {
			m.Source = ""
		}
	}

	for _, pre := range t.Pre {
		pre.Mounts = nil
	}

	for _, post := range t.Post {
		post.Mounts = nil
	}

	now := time.Now()

	t.State = task.Scheduled
	t.ScheduledAt = &now
	t.Result = ""
	t.Error = ""
	t.StartedAt = nil
	t.CancelledAt = nil
	t.CompletedAt = nil
	t.FailedAt = nil

	if t.Retry != nil {
		t.Retry.Attempts = 0
	}

	if err := s.persistAndPublish(ctx, t, string(task.Scheduled)); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetStatus(codes.Ok, "task restarted successfully")

	return t, nil
}

func (s *Service) CheckHealth(ctx context.Context) error {
	if err := s.readwriter.CheckHealth(ctx); err != nil {
		return err
	}

	return s.broker.CheckHealth(ctx)
}

func (s *Service) handleTask(ctx context.Context, data []byte) error {
	ctx, span := s.tracer.Start(ctx, "Coodinator.handleTask")
	defer span.End()

	t, _ := task.Factory(data)
	span.SetAttributes(
		attribute.String("task.id", t.ID),
		attribute.String("task.state", string(t.State)),
	)

	switch t.State {
	case task.Started:
		ctx, child := s.tracer.Start(ctx, "Write Started State")
		defer child.End()

		if err := s.readwriter.Write(ctx, t.ID, data); err != nil {
			child.RecordError(err)
			child.SetStatus(codes.Error, err.Error())
			return err
		}

		child.SetStatus(codes.Ok, "task started event handled")

		return nil
	case task.Completed:
		ctx, child := s.tracer.Start(ctx, "Write Completed State")
		defer child.End()

		if err := s.readwriter.Write(ctx, t.ID, data); err != nil {
			child.RecordError(err)
			child.SetStatus(codes.Error, err.Error())
			return err
		}

		s.removeTaskLock(t.ID)

		child.SetStatus(codes.Ok, "task completed event handled")

		return nil
	case task.Failed:
		ctx, child := s.tracer.Start(ctx, "Write Failed State")
		defer child.End()

		if err := s.readwriter.Write(ctx, t.ID, data); err != nil {
			child.RecordError(err)
			child.SetStatus(codes.Error, err.Error())
			return err
		}

		s.removeTaskLock(t.ID)

		child.SetStatus(codes.Ok, "task failed event handled")

		return nil
	}

	return nil
}

func (s *Service) retrieveTask(ctx context.Context, id string) (*task.Task, error) {
	bs, err := s.readwriter.ReadById(ctx, id)
	if err != nil && errors.Is(err, reader.ErrRecordNotFound) {
		return nil, task.ErrTaskNotFound
	} else if err != nil {
		return nil, err
	}

	return task.Factory(bs)
}

func (s *Service) persistAndPublish(ctx context.Context, t *task.Task, defaultQueueName string) error {
	ctx, span := s.tracer.Start(ctx, "Coodinator.persistAndPublish", trace.WithAttributes(
		attribute.String("task.id", t.ID),
	))
	defer span.End()

	bs, _ := json.Marshal(t)

	if err := s.readwriter.Write(ctx, t.ID, bs); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	queueName := t.Queue
	if len(queueName) == 0 {
		queueName = defaultQueueName
	}
	span.SetAttributes(attribute.String("queue", queueName))

	opts := []broker.PublishOption{
		broker.PublishWithQueue(queueName),
	}

	if err := s.broker.Publish(ctx, bs, opts...); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "task persisted and published")

	return nil
}

// TODO: needs distributed locking?
func (s *Service) retrieveTaskLock(id string) *sync.RWMutex {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	lock, ok := s.locks[id]
	if !ok {
		lock = &sync.RWMutex{}
		s.locks[id] = lock
	}

	return lock
}

func (s *Service) removeTaskLock(id string) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	delete(s.locks, id)
}

func New(b broker.Broker, rw readwriter.ReadWriter, qs map[string]int) *Service {
	return &Service{
		broker:     b,
		readwriter: rw,
		queues:     qs,
		locks:      map[string]*sync.RWMutex{},
		mtx:        sync.RWMutex{},
		tracer:     otel.Tracer("coordinator-service"),
	}
}
