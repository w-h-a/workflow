package memory

import (
	"context"
	"log/slog"
	"sync"

	"github.com/w-h-a/workflow/internal/engine/clients/broker"
)

type memoryBroker struct {
	options broker.Options
	queues  map[string]chan []byte
	mtx     sync.RWMutex
}

func (b *memoryBroker) Subscribe(ctx context.Context, callback func(ctx context.Context, data []byte) error, opts ...broker.SubscribeOption) error {
	options := broker.NewSubscribeOptions(opts...)

	// span
	slog.InfoContext(ctx, "subscribing to group", "group", options.Group)

	b.mtx.Lock()
	defer b.mtx.Unlock()

	q, ok := b.queues[options.Group]
	if !ok {
		q = make(chan []byte)
		b.queues[options.Group] = q
	}

	go func() {
		ctx := context.Background()

		for data := range q {
			if err := callback(ctx, data); err != nil {
				// span
				slog.ErrorContext(ctx, "failed to process incoming data", "data", data, "error", err)
			}
		}
	}()

	return nil
}

func (b *memoryBroker) Publish(ctx context.Context, data []byte, opts ...broker.PublishOption) error {
	options := broker.NewPublishOptions(opts...)

	// span
	slog.InfoContext(ctx, "sending to topic", "data", data, "topic", options.Topic)

	b.mtx.RLock()
	defer b.mtx.RUnlock()

	q, ok := b.queues[options.Topic]
	if !ok {
		return broker.ErrUnknownTopic
	}

	q <- data

	return nil
}

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)

	b := &memoryBroker{
		options: options,
		queues:  map[string]chan []byte{},
		mtx:     sync.RWMutex{},
	}

	return b
}
