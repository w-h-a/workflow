package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
	"github.com/w-h-a/workflow/internal/task"
)

type Service struct {
	name   string
	runner runner.Runner
	broker broker.Broker
}

func (s *Service) Name() string {
	return s.name
}

func (s *Service) Start() error {
	opts := []broker.SubscribeOption{
		broker.SubscribeWithGroup(s.name),
	}

	exit, err := s.broker.Subscribe(context.Background(), s.handleTask, opts...)
	if err != nil {
		return err
	}

	ch := make(chan os.Signal, 1)

	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	<-ch

	close(exit)

	return nil
}

func (s *Service) handleTask(ctx context.Context, data []byte) error {
	var t task.Task

	_ = json.Unmarshal(data, &t)

	switch t.State {
	case task.Pending, task.Scheduled:
		return s.startTask(ctx, t)
	case task.Cancelled:
		return s.stopTask(ctx, t)
	default:
		return fmt.Errorf("invalid task state: %v", t.State)
	}
}

func (s *Service) startTask(ctx context.Context, t task.Task) error {
	opts := []runner.StartOption{
		runner.StartWithID(t.ID),
		runner.StartWithName(t.Name),
		runner.StartWithImage(t.Image),
		runner.StartWithMemory(t.Memory),
		runner.StartWithEnv(t.Env),
		runner.StartWithRestartPolicy(t.RestartPolicy),
	}

	if err := s.runner.Start(ctx, opts...); err != nil {
		// span
		slog.ErrorContext(ctx, "failed to run task", "taskID", t.ID, "error", err)
		return err
	}

	return nil
}

func (s *Service) stopTask(ctx context.Context, t task.Task) error {
	opts := []runner.StopOption{
		runner.StopWithID(t.ID),
	}

	if err := s.runner.Stop(ctx, opts...); err != nil {
		// span
		slog.ErrorContext(ctx, "failed to stop task", "taskID", t.ID, "error", err)
		return err
	}

	// span
	slog.InfoContext(ctx, "stopped and removed task", "taskID", t.ID)

	return nil
}

func New(r runner.Runner, b broker.Broker) *Service {
	name := fmt.Sprintf("worker-%s", strings.ReplaceAll(uuid.NewString(), "-", ""))

	return &Service{
		name:   name,
		runner: r,
		broker: b,
	}
}
