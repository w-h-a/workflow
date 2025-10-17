package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/w-h-a/workflow/api/log"
	"github.com/w-h-a/workflow/api/task"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Service struct {
	runner  runner.Runner
	broker  broker.Broker
	queues  map[string]int
	cancels map[string]context.CancelFunc
	mtx     sync.RWMutex
	tracer  trace.Tracer
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

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := s.broker.Close(shutdownCtx); err != nil {
			slog.ErrorContext(shutdownCtx, "broker shutdown failed", "error", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := s.runner.Close(shutdownCtx); err != nil {
			slog.ErrorContext(shutdownCtx, "runner shutdown failed", "error", err)
		}
	}()

	wg.Wait()

	return nil
}

func (s *Service) CheckHealth(ctx context.Context) error {
	if err := s.runner.CheckHealth(ctx); err != nil {
		return err
	}

	return s.broker.CheckHealth(ctx)
}

func (s *Service) handleTask(ctx context.Context, data []byte) error {
	ctx, span := s.tracer.Start(ctx, "Worker.handleTask")
	defer span.End()

	t, _ := task.Factory(data)
	span.SetAttributes(
		attribute.String("task.id", t.ID),
		attribute.String("task.state", string(t.State)),
	)

	switch t.State {
	case task.Scheduled:
		if err := s.runTask(ctx, t); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
		span.SetStatus(codes.Ok, "task ran successfully")
		return nil
	case task.Cancelled:
		if err := s.cancelTask(ctx, t); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
		span.SetStatus(codes.Ok, "task cancelled successfully")
		return nil
	}

	return nil
}

func (s *Service) runTask(ctx context.Context, t *task.Task) error {
	ctx, span := s.tracer.Start(ctx, "Wroker.runTask", trace.WithAttributes(
		attribute.String("task.id", t.ID),
		attribute.String("task.image", t.Image),
	))
	defer span.End()

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
		broker.PublishWithQueue(string(task.Started)),
	}

	ctx, publishStartedSpan := s.tracer.Start(ctx, "Publish Started State")

	if err := s.broker.Publish(ctx, startedBs, startedOpts...); err != nil {
		publishStartedSpan.RecordError(err)
		publishStartedSpan.SetStatus(codes.Error, err.Error())
		publishStartedSpan.End()
		return err
	}

	publishStartedSpan.SetStatus(codes.Ok, "started state published")
	publishStartedSpan.End()

	logHandler := func(logLine string) {
		logEntry := &log.Entry{
			TaskID: t.ID,
			Log:    logLine,
		}
		logData, _ := json.Marshal(logEntry)
		logOpts := []broker.PublishOption{
			broker.PublishWithQueue(string(log.Queue)),
		}
		ctx, publishLogSpan := s.tracer.Start(ctx, "Publish Container Log", trace.WithAttributes(
			attribute.String("task.id", t.ID),
			attribute.String("log", logLine),
		))
		defer publishLogSpan.End()
		if err := s.broker.Publish(ctx, logData, logOpts...); err != nil {
			publishLogSpan.RecordError(err)
			publishLogSpan.SetStatus(codes.Error, err.Error())
		} else {
			publishLogSpan.SetStatus(codes.Ok, "container log published")
		}
	}

	var ms []*task.Mount

	for _, m := range t.Mounts {
		ctx, createdVolumeSpan := s.tracer.Start(ctx, "Create Volume")

		volName := strings.ReplaceAll(uuid.NewString(), "-", "")

		opts := []runner.CreateVolumeOption{
			runner.CreateVolumeWithName(volName),
		}

		if err := s.runner.CreateVolume(ctx, opts...); err != nil {
			createdVolumeSpan.RecordError(err)
			createdVolumeSpan.SetStatus(codes.Error, err.Error())
			createdVolumeSpan.End()

			finished := time.Now()
			t.Error = err.Error()
			t.State = task.Failed
			t.FailedAt = &finished

			failedBs, _ := json.Marshal(t)

			failedOpts := []broker.PublishOption{
				broker.PublishWithQueue(string(task.Failed)),
			}

			ctx, publishFailedSpan := s.tracer.Start(ctx, "Publish Failed State")

			if err := s.broker.Publish(ctx, failedBs, failedOpts...); err != nil {
				publishFailedSpan.RecordError(err)
				publishFailedSpan.SetStatus(codes.Error, err.Error())
				publishFailedSpan.End()
				return err
			}

			publishFailedSpan.SetStatus(codes.Ok, "failed state published")
			publishFailedSpan.End()

			return nil
		}

		createdVolumeSpan.SetStatus(codes.Ok, "volume created")
		createdVolumeSpan.End()

		defer func(volName string) {
			opts := []runner.DeleteVolumeOption{
				runner.DeleteVolumeWithName(volName),
			}

			ctx, deleteVolumeSpan := s.tracer.Start(context.Background(), "Volume Deferred", trace.WithAttributes(
				attribute.String("volume.name", volName),
			))
			defer deleteVolumeSpan.End()

			if err := s.runner.DeleteVolume(ctx, opts...); err != nil {
				deleteVolumeSpan.RecordError(err)
				deleteVolumeSpan.SetStatus(codes.Error, err.Error())
			} else {
				deleteVolumeSpan.SetStatus(codes.Ok, "volume deleted")
			}
		}(volName)

		ms = append(ms, &task.Mount{
			Source: volName,
			Target: m.Target,
		})
	}

	t.Mounts = ms

	for _, pre := range t.Pre {
		pre.ID = strings.ReplaceAll(uuid.NewString(), "-", "")
		pre.Mounts = t.Mounts
		pre.Networks = t.Networks

		ctx, preRunSpan := s.tracer.Start(ctx, "Run Pre-Task", trace.WithAttributes(
			attribute.String("pre-task.id", pre.ID),
		))

		result, err := s.run(ctx, pre, logHandler)

		finished := time.Now()
		if err != nil {
			preRunSpan.RecordError(err)
			preRunSpan.SetStatus(codes.Error, err.Error())
			preRunSpan.End()

			t.Error = err.Error()
			t.State = task.Failed
			t.FailedAt = &finished

			failedBs, _ := json.Marshal(t)

			failedOpts := []broker.PublishOption{
				broker.PublishWithQueue(string(task.Failed)),
			}

			ctx, publishFailedSpan := s.tracer.Start(ctx, "Publish Failed State")

			if err := s.broker.Publish(ctx, failedBs, failedOpts...); err != nil {
				publishFailedSpan.RecordError(err)
				publishFailedSpan.SetStatus(codes.Error, err.Error())
				publishFailedSpan.End()
				return err
			}

			publishFailedSpan.SetStatus(codes.Ok, "failed state published")
			publishFailedSpan.End()

			return nil
		}

		preRunSpan.SetStatus(codes.Ok, "pre-task run was a success")
		preRunSpan.End()

		pre.Result = result
	}

	result, err := s.run(ctx, t, logHandler)

	finished := time.Now()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		t.Error = err.Error()
		t.State = task.Failed
		t.FailedAt = &finished

		failedBs, _ := json.Marshal(t)

		failedOpts := []broker.PublishOption{
			broker.PublishWithQueue(string(task.Failed)),
		}

		ctx, publishFailedSpan := s.tracer.Start(ctx, "Publish Failed State")

		if err := s.broker.Publish(ctx, failedBs, failedOpts...); err != nil {
			publishFailedSpan.RecordError(err)
			publishFailedSpan.SetStatus(codes.Error, err.Error())
			publishFailedSpan.End()
			return err
		}

		publishFailedSpan.SetStatus(codes.Ok, "failed state published")
		publishFailedSpan.End()

		return nil
	}

	for _, post := range t.Post {
		post.ID = strings.ReplaceAll(uuid.NewString(), "-", "")
		post.Mounts = t.Mounts
		post.Networks = t.Networks

		ctx, postRunSpan := s.tracer.Start(ctx, "Run Post-Task", trace.WithAttributes(
			attribute.String("post-task.id", post.ID),
		))

		result, err := s.run(ctx, post, logHandler)

		finished := time.Now()
		if err != nil {
			postRunSpan.RecordError(err)
			postRunSpan.SetStatus(codes.Error, err.Error())
			postRunSpan.End()

			t.Error = err.Error()
			t.State = task.Failed
			t.FailedAt = &finished

			failedBs, _ := json.Marshal(t)

			failedOpts := []broker.PublishOption{
				broker.PublishWithQueue(string(task.Failed)),
			}

			ctx, publishFailedSpan := s.tracer.Start(ctx, "Publish Failed State")

			if err := s.broker.Publish(ctx, failedBs, failedOpts...); err != nil {
				publishFailedSpan.RecordError(err)
				publishFailedSpan.SetStatus(codes.Error, err.Error())
				publishFailedSpan.End()
				return err
			}

			publishFailedSpan.SetStatus(codes.Ok, "failed state published")
			publishFailedSpan.End()

			return nil
		}

		postRunSpan.SetStatus(codes.Ok, "post-task run was a success")
		postRunSpan.End()

		post.Result = result
	}

	t.Result = result
	t.State = task.Completed
	t.CompletedAt = &finished

	completedBs, _ := json.Marshal(t)

	completedOpts := []broker.PublishOption{
		broker.PublishWithQueue(string(task.Completed)),
	}

	ctx, publishCompletedSpan := s.tracer.Start(ctx, "Publish Completed State")

	if err := s.broker.Publish(ctx, completedBs, completedOpts...); err != nil {
		publishCompletedSpan.RecordError(err)
		publishCompletedSpan.SetStatus(codes.Error, err.Error())
		publishCompletedSpan.End()
		return err
	}

	publishCompletedSpan.SetStatus(codes.Ok, "completed state published")
	publishCompletedSpan.End()

	span.SetStatus(codes.Ok, "task completed successfully")

	return nil
}

func (s *Service) run(ctx context.Context, t *task.Task, logHandler func(logLine string)) (string, error) {
	ctx, span := s.tracer.Start(ctx, "Worker.run", trace.WithAttributes(
		attribute.String("task.id", t.ID),
		attribute.String("task.image", t.Image),
		attribute.String("task.cmd", strings.Join(t.Cmd, " ")),
	))
	defer span.End()

	if len(t.Timeout) > 0 {
		dur, _ := time.ParseDuration(t.Timeout)
		timeoutCtx, cancel := context.WithTimeout(ctx, dur)
		defer cancel()
		ctx = timeoutCtx
	}

	var mounts []map[string]string

	for _, m := range t.Mounts {
		mounts = append(mounts, map[string]string{
			"source": m.Source,
			"target": m.Target,
		})
	}

	runOpts := []runner.RunOption{
		runner.RunWithID(t.ID),
		runner.RunWithImage(t.Image),
		runner.RunWithCmd(t.Cmd),
		runner.RunWithEnv(t.Env),
		runner.RunWithMounts(mounts),
		runner.RunWithNetworks(t.Networks),
		runner.RunWithLogHandler(logHandler),
	}

	result, err := s.runner.Run(ctx, runOpts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return result, err
	}

	span.SetStatus(codes.Ok, "runner ran successfully")

	return result, nil
}

func (s *Service) cancelTask(ctx context.Context, t *task.Task) error {
	_, span := s.tracer.Start(ctx, "Worker.cancelTask", trace.WithAttributes(
		attribute.String("task.id", t.ID),
	))
	defer span.End()

	s.mtx.RLock()
	cancel, ok := s.cancels[t.ID]
	s.mtx.RUnlock()
	if !ok {
		span.SetStatus(codes.Ok, "task already cancelled or not running")
		return nil
	}

	cancel()

	span.SetStatus(codes.Ok, "task cancellation signal sent")

	return nil
}

func New(r runner.Runner, b broker.Broker, qs map[string]int) *Service {
	return &Service{
		runner:  r,
		broker:  b,
		queues:  qs,
		cancels: map[string]context.CancelFunc{},
		mtx:     sync.RWMutex{},
		tracer:  otel.Tracer("worker-service"),
	}
}
