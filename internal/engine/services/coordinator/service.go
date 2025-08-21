package coordinator

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"strings"
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
}

func (s *Service) Start(ch chan struct{}) error {
	var exits []chan struct{}

	for name, concurrency := range s.queues {
		for range concurrency {
			opts := []broker.SubscribeOption{
				broker.SubscribeWithQueue(name),
			}

			exit, err := s.broker.Subscribe(context.Background(), s.handleTask, opts...)
			if err != nil {
				for _, exit := range exits {
					close(exit)
				}
				return err
			}

			exits = append(exits, exit)
		}
	}

	<-ch

	for _, exit := range exits {
		close(exit)
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
	bs, err := s.readwriter.ReadById(ctx, id)
	if err != nil && errors.Is(err, reader.ErrRecordNotFound) {
		return nil, task.ErrTaskNotFound
	} else if err != nil {
		return nil, err
	}

	return task.Factory(bs)
}

func (s *Service) ScheduleTask(ctx context.Context, t *task.Task) (*task.Task, error) {
	now := time.Now()

	t.ID = strings.ReplaceAll(uuid.NewString(), "-", "")
	t.State = task.Scheduled
	t.ScheduledAt = &now

	bs, _ := json.Marshal(t)

	name := t.Queue
	if len(name) == 0 {
		name = broker.SCHEDULED
	}

	opts := []broker.PublishOption{
		broker.PublishWithQueue(name),
	}

	if err := s.broker.Publish(ctx, bs, opts...); err != nil {
		return nil, err
	}

	if err := s.readwriter.Write(ctx, t.ID, bs); err != nil {
		return nil, err
	}

	return t, nil
}

func (s *Service) CancelTask(ctx context.Context, t *task.Task) (*task.Task, error) {
	now := time.Now()

	t.State = task.Cancelled
	t.CancelledAt = &now

	bs, _ := json.Marshal(t)

	name := t.Queue
	if len(name) == 0 {
		name = broker.CANCELLED
	}

	opts := []broker.PublishOption{
		broker.PublishWithQueue(name),
	}

	if err := s.broker.Publish(ctx, bs, opts...); err != nil {
		return nil, err
	}

	if err := s.readwriter.Write(ctx, t.ID, bs); err != nil {
		return nil, err
	}

	return t, nil
}

func (s *Service) handleTask(ctx context.Context, data []byte) error {
	t, _ := task.Factory(data)

	if t.State != task.Failed || t.Retry == nil || t.Retry.Attempts >= t.Retry.Limit {
		if err := s.readwriter.Write(ctx, t.ID, data); err != nil {
			return err
		}

		return nil
	}

	t.Retry.Attempts = t.Retry.Attempts + 1
	t.State = task.Scheduled
	t.Error = ""

	bs, _ := json.Marshal(t)

	name := t.Queue
	if len(name) == 0 {
		name = broker.SCHEDULED
	}

	dur, _ := time.ParseDuration(t.Retry.InitialDelay)

	go func() {
		delay := dur * time.Duration(math.Pow(float64(2), float64(t.Retry.Attempts-1)))
		time.Sleep(delay)
		opts := []broker.PublishOption{
			broker.PublishWithQueue(name),
		}
		if err := s.broker.Publish(ctx, bs, opts...); err != nil {
			// span
			slog.ErrorContext(ctx, "failed to retry task")
		}
	}()

	return nil
}

func New(b broker.Broker, rw readwriter.ReadWriter, qs map[string]int) *Service {
	return &Service{
		broker:     b,
		readwriter: rw,
		queues:     qs,
	}
}
