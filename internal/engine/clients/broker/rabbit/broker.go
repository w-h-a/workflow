package rabbit

import (
	"context"
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
)

type rabbitBroker struct {
	options broker.Options
	conn    *amqp.Connection
	queues  map[string]amqp.Queue
	mtx     sync.RWMutex
}

func (b *rabbitBroker) Subscribe(ctx context.Context, callback func(ctx context.Context, data []byte) error, opts ...broker.SubscribeOption) (chan struct{}, error) {
	options := broker.NewSubscribeOptions(opts...)

	// span
	slog.InfoContext(ctx, "subscribing to queue", "queue", options.Queue)

	rbch, err := b.conn.Channel()
	if err != nil {
		return nil, broker.ErrCreatingChannel
	}

	if err := b.declareQueue(options.Queue, rbch); err != nil {
		return nil, broker.ErrCreatingQueue
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
		return nil, broker.ErrSubscribing
	}

	ch := make(chan struct{})

	go func() {
		for {
			select {
			case <-ch:
				rbch.Close()
				return
			case msg, ok := <-msgs:
				if !ok {
					// span
					slog.InfoContext(ctx, "delivery channel closed")
					rbch.Close()
					return
				}

				if err := callback(ctx, msg.Body); err != nil {
					// span
					slog.ErrorContext(ctx, "failed to process incoming data", "data", msg.Body, "error", err)

					if err := msg.Reject(false); err != nil {
						// span
						slog.ErrorContext(ctx, "failed to reject")
					}
				} else {
					if err := msg.Ack(false); err != nil {
						// span
						slog.ErrorContext(ctx, "failed to ack")
					}
				}
			}
		}
	}()

	return ch, nil
}

func (b *rabbitBroker) Publish(ctx context.Context, data []byte, opts ...broker.PublishOption) error {
	options := broker.NewPublishOptions(opts...)

	// span
	slog.InfoContext(ctx, "publishing to queue", "data", data, "queue", options.Queue)

	rbch, err := b.conn.Channel()
	if err != nil {
		return broker.ErrCreatingChannel
	}

	defer rbch.Close()

	if err := b.declareQueue(options.Queue, rbch); err != nil {
		return broker.ErrCreatingQueue
	}

	if err := rbch.PublishWithContext(
		ctx,
		"",            // exchange
		options.Queue, // routing key
		false,         // mandatory
		false,         // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        data,
		},
	); err != nil {
		return broker.ErrPublishing
	}

	return nil
}

func (b *rabbitBroker) declareQueue(name string, rbch *amqp.Channel) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	_, ok := b.queues[name]
	if ok {
		return nil
	}

	rbqu, err := rbch.QueueDeclare(
		name,
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait,
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	b.queues[name] = rbqu

	return nil
}

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)

	conn, err := amqp.Dial(options.Location)
	if err != nil {
		detail := "failed to connect to rabbitmq broker"
		slog.ErrorContext(context.Background(), detail, "error", err)
		panic(detail)
	}

	b := &rabbitBroker{
		options: options,
		conn:    conn,
		queues:  map[string]amqp.Queue{},
		mtx:     sync.RWMutex{},
	}

	return b
}
