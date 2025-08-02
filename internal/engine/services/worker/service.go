package worker

import (
	"context"
	"log/slog"

	"github.com/w-h-a/workflow/internal/engine/clients/runner"
	"github.com/w-h-a/workflow/internal/task"
)

type Service struct {
	runner runner.Runner
	queue  chan task.Task
}

func (s *Service) EnqueueTask(ctx context.Context, t task.Task) {
	s.queue <- t
}

func (s *Service) RunTask(ctx context.Context) error {
	t := <-s.queue
	return s.StartTask(ctx, t)
}

func (s *Service) StartTask(ctx context.Context, t task.Task) error {
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

func (s *Service) StopTask(ctx context.Context, t task.Task) error {
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

func New(r runner.Runner) *Service {
	return &Service{
		runner: r,
		queue:  make(chan task.Task, 10),
	}
}
