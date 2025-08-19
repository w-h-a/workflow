package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
	"github.com/w-h-a/workflow/internal/task"
)

type Service struct {
	runner  runner.Runner
	broker  broker.Broker
	queues  map[string]int
	cancels map[string]context.CancelFunc
	mtx     sync.RWMutex
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

func (s *Service) handleTask(ctx context.Context, data []byte) error {
	t, _ := task.Factory(data)

	switch t.State {
	case task.Scheduled:
		return s.runTask(ctx, t)
	case task.Cancelled:
		return s.cancelTask(ctx, t)
	}

	return nil
}

func (s *Service) runTask(ctx context.Context, t *task.Task) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.mtx.Lock()
	s.cancels[t.ID] = cancel
	s.mtx.Unlock()

	defer func() {
		s.mtx.Lock()
		defer s.mtx.Unlock()
		delete(s.cancels, t.ID)
	}()

	started := time.Now()

	t.State = task.Started
	t.StartedAt = &started

	startedBs, _ := json.Marshal(t)

	startedOpts := []broker.PublishOption{
		broker.PublishWithQueue(broker.STARTED),
	}

	if err := s.broker.Publish(ctx, startedBs, startedOpts...); err != nil {
		return err
	}

	var vs []string

	for _, v := range t.Volumes {
		volName := strings.ReplaceAll(uuid.NewString(), "-", "")
		opts := []runner.CreateVolumeOption{
			runner.CreateVolumeWithName(volName),
		}
		if err := s.runner.CreateVolume(ctx, opts...); err != nil {
			return err
		}
		defer func(volName string) {
			opts := []runner.DeleteVolumeOption{
				runner.DeleteVolumeWithName(volName),
			}
			if err := s.runner.DeleteVolume(ctx, opts...); err != nil {
				// span
				slog.ErrorContext(ctx, "failed to delete volume", "name", volName)
			}
		}(volName)
		vs = append(vs, fmt.Sprintf("%s:%s", volName, v))
	}

	t.Volumes = vs

	for _, pre := range t.Pre {
		pre.ID = strings.ReplaceAll(uuid.NewString(), "-", "")
		pre.Volumes = t.Volumes
		runOpts := []runner.RunOption{
			runner.RunWithID(pre.ID),
			runner.RunWithImage(pre.Image),
			runner.RunWithCmd(pre.Cmd),
			runner.RunWithEnv(pre.Env),
			runner.RunWithVolumes(pre.Volumes),
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
		pre.Result = result
	}

	runOpts := []runner.RunOption{
		runner.RunWithID(t.ID),
		runner.RunWithImage(t.Image),
		runner.RunWithCmd(t.Cmd),
		runner.RunWithEnv(t.Env),
		runner.RunWithVolumes(t.Volumes),
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

	for _, post := range t.Post {
		post.ID = strings.ReplaceAll(uuid.NewString(), "-", "")
		post.Volumes = t.Volumes
		runOpts := []runner.RunOption{
			runner.RunWithID(post.ID),
			runner.RunWithImage(post.Image),
			runner.RunWithCmd(post.Cmd),
			runner.RunWithEnv(post.Env),
			runner.RunWithVolumes(post.Volumes),
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
		post.Result = result
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

func (s *Service) cancelTask(_ context.Context, t *task.Task) error {
	s.mtx.RLock()
	cancel, ok := s.cancels[t.ID]
	s.mtx.RUnlock()
	if !ok {
		return nil
	}

	cancel()

	return nil
}

func New(r runner.Runner, b broker.Broker, qs map[string]int) *Service {
	return &Service{
		runner:  r,
		broker:  b,
		queues:  qs,
		cancels: map[string]context.CancelFunc{},
		mtx:     sync.RWMutex{},
	}
}
