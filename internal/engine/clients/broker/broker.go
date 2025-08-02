package broker

import "context"

type Broker interface {
	Subscribe(ctx context.Context, callback func(ctx context.Context, data []byte) error, opts ...SubscribeOption) error
	Publish(ctx context.Context, data []byte, opts ...PublishOption) error
}
