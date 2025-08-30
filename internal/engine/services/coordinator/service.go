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
)

type Service struct {
	broker     broker.Broker
	readwriter readwriter.ReadWriter
	queues     map[string]int
	locks      map[string]*sync.RWMutex
	mtx        sync.RWMutex
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
	opts := []reader.ReadOption{
		reader.ReadWithPage(page),
		reader.ReadWithSize(size),
	}

	rp, err := s.readwriter.Read(ctx, opts...)
	if err != nil {
		return nil, err
	}

	tasks := make([]*task.Task, 0, len(rp.Items))

	for _, bs := range rp.Items {
		task, _ := task.Factory(bs)
		tasks = append(tasks, task)
	}

	return &TasksWithMetadata{
		Tasks:      tasks,
		Size:       rp.Size,
		Number:     rp.Number,
		TotalPages: rp.TotalPages,
		TotalTasks: rp.TotalItems,
	}, nil
}

func (s *Service) RetrieveTask(ctx context.Context, id string) (*task.Task, error) {
	return s.retrieveTask(ctx, id)
}

func (s *Service) ScheduleTask(ctx context.Context, t *task.Task) (*task.Task, error) {
	t.ID = strings.ReplaceAll(uuid.NewString(), "-", "")

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
		return nil, err
	}

	return t, nil
}

func (s *Service) CancelTask(ctx context.Context, id string) (*task.Task, error) {
	taskLock := s.retrieveTaskLock(id)

	taskLock.Lock()
	defer taskLock.Unlock()

	t, err := s.retrieveTask(ctx, id)
	if err != nil {
		return nil, err
	}

	if t.State != task.Started {
		return nil, task.ErrTaskNotCancellable
	}

	now := time.Now()

	t.State = task.Cancelled
	t.CancelledAt = &now

	if err := s.persistAndPublish(ctx, t, string(task.Cancelled)); err != nil {
		return nil, err
	}

	return t, nil
}

func (s *Service) RestartTask(ctx context.Context, id string) (*task.Task, error) {
	taskLock := s.retrieveTaskLock(id)

	taskLock.Lock()
	defer taskLock.Unlock()

	t, err := s.retrieveTask(ctx, id)
	if err != nil {
		return nil, err
	}

	if t.State == task.Scheduled || t.State == task.Started {
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
		return nil, err
	}

	return t, nil
}

func (s *Service) CheckHealth(ctx context.Context) error {
	if err := s.readwriter.CheckHealth(ctx); err != nil {
		return err
	}

	return s.broker.CheckHealth(ctx)
}

func (s *Service) handleTask(ctx context.Context, data []byte) error {
	t, _ := task.Factory(data)

	switch t.State {
	case task.Started:
		if err := s.readwriter.Write(ctx, t.ID, data); err != nil {
			return err
		}

		return nil
	case task.Completed:
		if err := s.readwriter.Write(ctx, t.ID, data); err != nil {
			return err
		}

		s.removeTaskLock(t.ID)

		return nil
	case task.Failed:
		if err := s.readwriter.Write(ctx, t.ID, data); err != nil {
			return err
		}

		s.removeTaskLock(t.ID)

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
	bs, _ := json.Marshal(t)

	if err := s.readwriter.Write(ctx, t.ID, bs); err != nil {
		return err
	}

	queueName := t.Queue
	if len(queueName) == 0 {
		queueName = defaultQueueName
	}

	opts := []broker.PublishOption{
		broker.PublishWithQueue(queueName),
	}

	if err := s.broker.Publish(ctx, bs, opts...); err != nil {
		return err
	}

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
	}
}
