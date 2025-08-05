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

func (b *memoryBroker) Subscribe(ctx context.Context, callback func(ctx context.Context, data []byte) error, opts ...broker.SubscribeOption) (chan struct{}, error) {
	options := broker.NewSubscribeOptions(opts...)

	// span
	slog.InfoContext(ctx, "subscribing to queue", "queue", options.Queue)

	b.mtx.Lock()
	q, ok := b.queues[options.Queue]
	if !ok {
		q = make(chan []byte, 10)
		b.queues[options.Queue] = q
	}
	b.mtx.Unlock()

	ch := make(chan struct{})

	go func() {
		for {
			select {
			case <-ch:
				return
			case data := <-q:
				if err := callback(ctx, data); err != nil {
					// span
					slog.ErrorContext(ctx, "failed to process incoming data", "data", data, "error", err)
				}
			}
		}
	}()

	return ch, nil
}

func (b *memoryBroker) Publish(ctx context.Context, data []byte, opts ...broker.PublishOption) error {
	options := broker.NewPublishOptions(opts...)

	// span
	slog.InfoContext(ctx, "publishing to queue", "data", data, "queue", options.Queue)

	b.mtx.Lock()
	q, ok := b.queues[options.Queue]
	if !ok {
		q = make(chan []byte, 10)
		b.queues[options.Queue] = q
	}
	b.mtx.Unlock()

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
