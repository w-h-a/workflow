package nats

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

type natsBroker struct {
	options broker.Options
	conn    *nats.Conn
	js      nats.JetStreamContext
	exit    chan struct{}
	wg      sync.WaitGroup
	once    sync.Once
	tracer  trace.Tracer
}

func (b *natsBroker) Subscribe(ctx context.Context, callback func(ctx context.Context, data []byte) error, opts ...broker.SubscribeOption) error {
	options := broker.NewSubscribeOptions(opts...)

	slog.InfoContext(ctx, "subscribing to stream", "stream", options.Queue)

	_, err := b.js.StreamInfo(options.Queue)
	if err != nil {
		if err != nats.ErrStreamNotFound {
			slog.ErrorContext(ctx, "failed to get stream info", "error", err)
			return err
		}

		slog.InfoContext(ctx, "stream not found, adding new stream", "stream", options.Queue)

		_, err := b.js.AddStream(&nats.StreamConfig{
			Name:      options.Queue,
			Subjects:  []string{options.Queue},
			Retention: nats.LimitsPolicy,
			MaxAge:    time.Hour * 24 * 7,
		})
		if err != nil {
			slog.ErrorContext(ctx, "failed to add stream", "error", err)
			return err
		}
	}

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()

		maxAttempts := 20

		for attempt := 1; attempt <= maxAttempts; attempt++ {
			sub, err := b.js.PullSubscribe(
				options.Queue,
				options.Queue,
				nats.BindStream(options.Queue),
				nats.PullMaxWaiting(128),
			)
			if err != nil {
				slog.ErrorContext(ctx, "failed to subscribe to stream", "error", err, "attempt", attempt)
				time.Sleep(time.Second * time.Duration(attempt))
				continue
			}

		consumerLoop:
			for {
				select {
				case <-b.exit:
					sub.Unsubscribe()
					return
				default:
					fetchCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
					msgs, err := sub.Fetch(1, nats.Context(fetchCtx))
					cancel()
					if err != nil {
						if err == nats.ErrTimeout || err == context.DeadlineExceeded {
							continue
						}
						slog.ErrorContext(ctx, "failed to fetch messages", "error", err, "attempt", attempt)
						sub.Unsubscribe()
						break consumerLoop
					}

					for _, msg := range msgs {
						carrier := propagation.MapCarrier{}

						for k, v := range msg.Header {
							if len(v) > 0 {
								carrier.Set(k, v[0])
							}
						}

						propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
						extractedCtx := propagator.Extract(ctx, carrier)

						msgCtx, msgSpan := b.tracer.Start(extractedCtx, "Process NATS Message", trace.WithAttributes(
							semconv.MessagingSystemKey.String("nats"),
							semconv.MessagingOperationReceive,
							semconv.MessagingDestinationNameKey.String(options.Queue),
						))

						func() {
							defer msgSpan.End()

							if err := callback(msgCtx, msg.Data); err != nil {
								msgSpan.RecordError(err)
								msgSpan.SetStatus(codes.Error, err.Error())
								slog.ErrorContext(msgCtx, "failed to process incoming data", "error", err)

								if err := msg.Nak(); err != nil {
									msgSpan.RecordError(err)
									msgSpan.SetStatus(codes.Error, err.Error())
									slog.ErrorContext(msgCtx, "failed to nak message", "error", err)
								}
							} else {
								if err := msg.Ack(); err != nil {
									msgSpan.RecordError(err)
									msgSpan.SetStatus(codes.Error, err.Error())
									slog.ErrorContext(msgCtx, "failed to ack message", "error", err)
								}
							}
						}()
					}
				}
			}
		}

		slog.ErrorContext(ctx, "subscriber failed to connect after max attempts", "queue", options.Queue, "maxAttempts", maxAttempts)
	}()

	return nil
}

func (b *natsBroker) Publish(ctx context.Context, data []byte, opts ...broker.PublishOption) error {
	options := broker.NewPublishOptions(opts...)

	ctx, span := b.tracer.Start(ctx, "Publish to Queue", trace.WithAttributes(
		semconv.MessagingSystemKey.String("nats"),
		semconv.MessagingOperationPublish,
		semconv.MessagingDestinationNameKey.String(options.Queue),
	))
	defer span.End()

	msg := nats.NewMsg(options.Queue)
	msg.Data = data

	carrier := propagation.MapCarrier{}

	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	propagator.Inject(ctx, carrier)

	for k, v := range carrier {
		msg.Header.Set(k, v)
	}

	if _, err := b.js.PublishMsg(msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to publish message")
		return broker.ErrPublishing
	}

	span.SetStatus(codes.Ok, "message published")

	return nil
}

func (b *natsBroker) CheckHealth(ctx context.Context) error {
	if b.conn.IsClosed() {
		return nats.ErrConnectionClosed
	}
	return nil
}

func (b *natsBroker) Close(ctx context.Context) error {
	done := make(chan struct{})

	b.once.Do(func() {
		slog.InfoContext(ctx, "starting graceful shutdown of nats client")

		close(b.exit)

		go func() {
			b.wg.Wait()
			b.conn.Close()
			close(done)
		}()
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		slog.InfoContext(ctx, "graceful shutdown of nats client complete")
	}

	return nil
}

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)

	conn, err := nats.Connect(
		options.Location,
		nats.MaxReconnects(5),
		nats.ReconnectWait(time.Second*5),
	)
	if err != nil {
		detail := "failed to connect to nats broker"
		slog.ErrorContext(context.Background(), detail, "error", err)
		panic(detail)
	}

	js, err := conn.JetStream()
	if err != nil {
		detail := "failed to create jetstream context"
		slog.ErrorContext(context.Background(), detail, "error", err)
		panic(detail)
	}

	b := &natsBroker{
		options: options,
		conn:    conn,
		js:      js,
		exit:    make(chan struct{}),
		wg:      sync.WaitGroup{},
		once:    sync.Once{},
		tracer:  otel.Tracer("nats-broker"),
	}

	return b
}
