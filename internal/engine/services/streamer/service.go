package streamer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/w-h-a/workflow/api/log"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Service struct {
	broker  broker.Broker
	queues  map[string]int
	streams map[string][]chan string
	mtx     sync.RWMutex
	tracer  trace.Tracer
}

func (s *Service) Start(ch chan struct{}) error {
	for name, concurrency := range s.queues {
		for range concurrency {
			opts := []broker.SubscribeOption{
				broker.SubscribeWithQueue(name),
			}

			if err := s.broker.Subscribe(context.Background(), s.handleLog, opts...); err != nil {
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

func (s *Service) StreamLogs(ctx context.Context, id string) (*LogStream, error) {
	ctx, span := s.tracer.Start(ctx, "Streamer.StreamLogs", trace.WithAttributes(
		attribute.String("task.id", id),
	))
	defer span.End()

	s.mtx.Lock()
	logs := make(chan string)
	s.streams[id] = append(s.streams[id], logs)
	s.mtx.Unlock()

	logStream := &LogStream{
		Logs: logs,
		Stop: make(chan struct{}),
	}

	go func() {
		select {
		case <-ctx.Done():
		case <-logStream.Stop:
		}

		s.mtx.Lock()
		for i, ch := range s.streams[id] {
			if ch == logs {
				s.streams[id] = append(s.streams[id][:i], s.streams[id][i+1:]...)
				break
			}
		}
		if len(s.streams[id]) == 0 {
			delete(s.streams, id)
		}
		close(logs)
		s.mtx.Unlock()
	}()

	span.SetStatus(codes.Ok, "log stream established")

	return logStream, nil
}

func (s *Service) CheckHealth(ctx context.Context) error {
	return s.broker.CheckHealth(ctx)
}

func (s *Service) handleLog(ctx context.Context, data []byte) error {
	ctx, span := s.tracer.Start(ctx, "Streamer.handleLog")
	defer span.End()

	e, _ := log.Factory(data)

	s.mtx.RLock()
	logChs, ok := s.streams[e.TaskID]
	s.mtx.RUnlock()

	if ok {
		for _, logCh := range logChs {
			func() {
				defer func() {
					if r := recover(); r != nil {
						span.SetStatus(codes.Error, "log stream closed unexpectedly during send")
						slog.WarnContext(ctx, "recovered from panic sending to log stream", "task.id", e.TaskID)
					}
				}()

				select {
				case <-ctx.Done():
					span.SetStatus(codes.Error, "stream context cancelled")
				case logCh <- e.Log:
					span.SetStatus(codes.Ok, "log line streamed successfully")
				}
			}()
		}
	} else {
		span.SetStatus(codes.Ok, "no active stream for task ID")
	}

	return nil
}

func New(b broker.Broker, qs map[string]int) *Service {
	return &Service{
		broker:  b,
		queues:  qs,
		streams: map[string][]chan string{},
		mtx:     sync.RWMutex{},
		tracer:  otel.Tracer("streamer-service"),
	}
}
