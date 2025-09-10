package broker

import "context"

type BrokerType string

const (
	Memory BrokerType = "memory"
	Nats   BrokerType = "nats"
	Rabbit BrokerType = "rabbit"
)

var (
	BrokerTypes = map[string]BrokerType{
		"memory": Memory,
		"nats":   Nats,
		"rabbit": Rabbit,
	}
)

type Broker interface {
	Subscribe(ctx context.Context, callback func(ctx context.Context, data []byte) error, opts ...SubscribeOption) error
	Publish(ctx context.Context, data []byte, opts ...PublishOption) error
	CheckHealth(ctx context.Context) error
	Close(ctx context.Context) error
}
