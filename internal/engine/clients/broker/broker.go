package broker

import "context"

type BrokerType string

const (
	Memory BrokerType = "memory"
	Rabbit BrokerType = "rabbit"
)

type Broker interface {
	Subscribe(ctx context.Context, callback func(ctx context.Context, data []byte) error, opts ...SubscribeOption) (chan struct{}, error)
	Publish(ctx context.Context, data []byte, opts ...PublishOption) error
}
