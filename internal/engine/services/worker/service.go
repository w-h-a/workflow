package worker

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
	"github.com/w-h-a/workflow/internal/task"
)

type Service struct {
	runner runner.Runner
	broker broker.Broker
}

func (s *Service) Start() error {
	opts := []broker.SubscribeOption{
		broker.SubscribeWithQueue(broker.PENDING),
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
	var t *task.Task

	_ = json.Unmarshal(data, &t)

	started := time.Now()

	t.State = task.Running
	t.StartedAt = &started

	startedBs, _ := json.Marshal(t)

	startedOpts := []broker.PublishOption{
		broker.PublishWithQueue(broker.STARTED),
	}

	if err := s.broker.Publish(ctx, startedBs, startedOpts...); err != nil {
		return err
	}

	runOpts := []runner.RunOption{
		runner.RunWithID(t.ID),
		runner.RunWithImage(t.Image),
		runner.RunWithCmd(t.Cmd),
		runner.RunWithEnv(t.Env),
		runner.RunWithMemory(t.Memory),
		runner.RunWithRestartPolicy(t.RestartPolicy),
	}

	result, err := s.runner.Run(ctx, runOpts...)
	finished := time.Now()
	if err != nil {
		t.Error = err.Error()
		t.State = task.Failed
		t.FailedAt = &finished

		failedBs, _ := json.Marshal(t)

		failedOpts := []broker.PublishOption{
			broker.PublishWithQueue(broker.FAILED),
		}

		if err := s.broker.Publish(ctx, failedBs, failedOpts...); err != nil {
			return err
		}

		return nil
	}

	t.Result = result
	t.State = task.Completed
	t.CompletedAt = &finished

	completedBs, _ := json.Marshal(t)

	completedOpts := []broker.PublishOption{
		broker.PublishWithQueue(broker.COMPLETED),
	}

	if err := s.broker.Publish(ctx, completedBs, completedOpts...); err != nil {
		return err
	}

	return nil
}

func New(r runner.Runner, b broker.Broker) *Service {
	return &Service{
		runner: r,
		broker: b,
	}
}
