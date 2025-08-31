package rabbit

import (
	"context"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

type rabbitBroker struct {
	options broker.Options
	pool    []*amqp.Connection
	next    int
	mtx     sync.RWMutex
	exit    chan struct{}
	wg      sync.WaitGroup
	once    sync.Once
	tracer  trace.Tracer
}

func (b *rabbitBroker) Subscribe(ctx context.Context, callback func(ctx context.Context, data []byte) error, opts ...broker.SubscribeOption) error {
	options := broker.NewSubscribeOptions(opts...)

	slog.InfoContext(ctx, "subscribing to queue", "queue", options.Queue)

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()

		maxAttempts := 20

		for attempt := 1; attempt <= maxAttempts; attempt++ {
			conn, err := b.getConnection()
			if err != nil {
				slog.ErrorContext(ctx, "failed to get a connection from pool", "error", err, "attempt", attempt)
				time.Sleep(time.Second * time.Duration(attempt))
				continue
			}

			rbch, err := conn.Channel()
			if err != nil {
				slog.ErrorContext(ctx, "failed to create channel", "error", err, "attempt", attempt)
				time.Sleep(time.Second * time.Duration(attempt))
				continue
			}

			if _, err := rbch.QueueDeclare(
				options.Queue,
				b.options.Durable,
				false, // delete when unused
				false, // exclusive
				false, // no-wait,
				nil,   // arguments
			); err != nil {
				slog.ErrorContext(ctx, "failed to declare queue", "error", err, "attempt", attempt)
				rbch.Close()
				time.Sleep(time.Second * time.Duration(attempt))
				continue
			}

			if err := rbch.Qos(1, 0, false); err != nil {
				slog.ErrorContext(ctx, "failed to set Qos", "error", err, "attempt", attempt)
				rbch.Close()
				time.Sleep(time.Second * time.Duration(attempt))
				continue
			}

			msgs, err := rbch.Consume(
				options.Queue,
				"",    // consumer name,
				false, // auto-ack,
				false, // exclusive
				false, // no-local
				false, // no-wait
				nil,   // args
			)
			if err != nil {
				slog.ErrorContext(ctx, "failed to subscribe", "error", err, "attempt", attempt)
				rbch.Close()
				time.Sleep(time.Second * time.Duration(attempt))
				continue
			}

		consumerLoop:
			for {
				select {
				case <-b.exit:
					rbch.Close()
					return
				case msg, ok := <-msgs:
					if !ok {
						break consumerLoop
					}

					carrier := propagation.MapCarrier{}

					for k, v := range msg.Headers {
						if val, ok := v.(string); ok {
							carrier.Set(k, val)
						}
					}

					propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
					extractedCtx := propagator.Extract(ctx, carrier)

					msgCtx, msgSpan := b.tracer.Start(extractedCtx, "Process RabbitMQ Message", trace.WithAttributes(
						semconv.MessagingSystemKey.String("rabbitmq"),
					))

					func() {
						defer msgSpan.End()

						if err := callback(msgCtx, msg.Body); err != nil {
							msgSpan.RecordError(err)
							msgSpan.SetStatus(codes.Error, err.Error())
							slog.ErrorContext(msgCtx, "failed to process incoming data", "data", msg.Body, "error", err)

							if err := msg.Reject(false); err != nil {
								msgSpan.RecordError(err)
								msgSpan.SetStatus(codes.Error, err.Error())
								slog.ErrorContext(msgCtx, "failed to reject", "error", err)
							}
						} else {
							if err := msg.Ack(false); err != nil {
								msgSpan.RecordError(err)
								msgSpan.SetStatus(codes.Error, err.Error())
								slog.ErrorContext(msgCtx, "failed to ack", "error", err)
							}
						}
					}()
				}
			}
		}

		slog.ErrorContext(ctx, "subscriber failed to connect after max attempts", "queue", options.Queue, "maxAttempts", maxAttempts)
	}()

	return nil
}

func (b *rabbitBroker) Publish(ctx context.Context, data []byte, opts ...broker.PublishOption) error {
	options := broker.NewPublishOptions(opts...)

	ctx, span := b.tracer.Start(ctx, "Publish to Queue", trace.WithAttributes(
		semconv.MessagingSystemKey.String("rabbitmq"),
		semconv.MessagingOperationPublish,
		semconv.MessagingDestinationNameKey.String(options.Queue),
	))
	defer span.End()

	conn, err := b.getConnection()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get connection from pool")
		return err
	}

	rbch, err := conn.Channel()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create channel")
		return broker.ErrCreatingChannel
	}

	defer rbch.Close()

	if _, err := rbch.QueueDeclare(
		options.Queue,
		b.options.Durable,
		false, // delete when unused
		false, // exclusive
		false, // no-wait,
		nil,   // arguments
	); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to declare queue")
		return err
	}

	publishing := amqp.Publishing{
		ContentType: "application/json",
		Body:        data,
		Headers:     amqp.Table{},
	}

	carrier := propagation.MapCarrier{}

	propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	propagator.Inject(ctx, carrier)

	for k, v := range carrier {
		publishing.Headers[k] = v
	}

	if b.options.Durable {
		publishing.DeliveryMode = amqp.Persistent
	}

	if err := rbch.PublishWithContext(
		ctx,
		"",            // exchange
		options.Queue, // routing key
		false,         // mandatory
		false,         // immediate
		publishing,
	); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to publish message")
		return broker.ErrPublishing
	}

	span.SetStatus(codes.Ok, "message published")

	return nil
}

func (b *rabbitBroker) CheckHealth(ctx context.Context) error {
	conn, err := b.getConnection()
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	defer ch.Close()

	return nil
}

func (b *rabbitBroker) Close(ctx context.Context) error {
	done := make(chan struct{})

	b.once.Do(func() {
		slog.InfoContext(ctx, "starting graceful shutdown of rabbit client")

		close(b.exit)

		go func() {
			b.wg.Wait()

			b.mtx.Lock()
			defer b.mtx.Unlock()

			for i, conn := range b.pool {
				if !conn.IsClosed() {
					if err := conn.Close(); err != nil {
						slog.ErrorContext(ctx, "failed to close connection", "error", err, "pool_index", i)
					}
				}
			}

			close(done)
		}()
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
	}

	slog.InfoContext(ctx, "graceful shutdown of rabbit client complete")

	return nil
}

func (b *rabbitBroker) getConnection() (*amqp.Connection, error) {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	conn := b.pool[b.next]
	index := b.next
	b.next = (index + 1) % len(b.pool)

	if !conn.IsClosed() {
		return conn, nil
	}

	new, err := amqp.Dial(b.options.Location)
	if err != nil {
		return nil, err
	}

	b.pool[index] = new

	return new, nil
}

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)

	pool := make([]*amqp.Connection, 3)

	for i := range pool {
		conn, err := amqp.Dial(options.Location)
		if err != nil {
			detail := "failed to connect to rabbitmq broker"
			slog.ErrorContext(context.Background(), detail, "error", err)
			panic(detail)
		}
		pool[i] = conn
	}

	b := &rabbitBroker{
		options: options,
		pool:    pool,
		next:    0,
		mtx:     sync.RWMutex{},
		exit:    make(chan struct{}),
		wg:      sync.WaitGroup{},
		once:    sync.Once{},
		tracer:  otel.Tracer("rabbitmq-broker"),
	}

	return b
}
