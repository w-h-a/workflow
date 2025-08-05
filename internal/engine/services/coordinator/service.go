package coordinator

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"github.com/w-h-a/workflow/internal/engine/clients/readwriter"
	"github.com/w-h-a/workflow/internal/task"
)

type Service struct {
	broker     broker.Broker
	readwriter readwriter.ReadWriter
}

func (s *Service) Start() error {
	startedOpts := []broker.SubscribeOption{
		broker.SubscribeWithQueue(broker.STARTED),
	}

	started, err := s.broker.Subscribe(context.Background(), s.handleTask, startedOpts...)
	if err != nil {
		return err
	}

	completedOpts := []broker.SubscribeOption{
		broker.SubscribeWithQueue(broker.COMPLETED),
	}

	completed, err := s.broker.Subscribe(context.Background(), s.handleTask, completedOpts...)
	if err != nil {
		return err
	}

	failedOpts := []broker.SubscribeOption{
		broker.SubscribeWithQueue(broker.FAILED),
	}

	failed, err := s.broker.Subscribe(context.Background(), s.handleTask, failedOpts...)
	if err != nil {
		return err
	}

	ch := make(chan os.Signal, 1)

	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	<-ch

	close(started)
	close(completed)
	close(failed)

	return nil
}

func (s *Service) ScheduleTask(ctx context.Context, t *task.Task) (*task.Task, error) {
	now := time.Now()

	t.ID = strings.ReplaceAll(uuid.NewString(), "-", "")
	t.State = task.Scheduled
	t.ScheduledAt = &now

	bs, _ := json.Marshal(t)

	opts := []broker.PublishOption{
		broker.PublishWithQueue(broker.SCHEDULED),
	}

	if err := s.broker.Publish(ctx, bs, opts...); err != nil {
		return nil, err
	}

	return t, nil
}

func (s *Service) handleTask(ctx context.Context, data []byte) error {
	var t *task.Task

	_ = json.Unmarshal(data, &t)

	slog.InfoContext(ctx, "received task", "task", *t)

	return nil
}

func New(b broker.Broker, rw readwriter.ReadWriter) *Service {
	return &Service{
		broker:     b,
		readwriter: rw,
	}
}
