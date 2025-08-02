package broker

import "context"

type Option func(o *Options)

type Options struct {
	Context context.Context
}

func NewOptions(opts ...Option) Options {
	options := Options{
		Context: context.Background(),
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}

type PublishOption func(o *PublishOptions)

type PublishOptions struct {
	Topic   string
	Context context.Context
}

func PublishWithTopic(topic string) PublishOption {
	return func(o *PublishOptions) {
		o.Topic = topic
	}
}

func NewPublishOptions(opts ...PublishOption) PublishOptions {
	options := PublishOptions{
		Context: context.Background(),
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}

type SubscribeOption func(o *SubscribeOptions)

type SubscribeOptions struct {
	// Topic => Group is one:many (think sns and sqs where there is more than one sqs subscriber to an sns topic)
	Group   string
	Context context.Context
}

func SubscribeWithGroup(group string) SubscribeOption {
	return func(o *SubscribeOptions) {
		o.Group = group
	}
}

func NewSubscribeOptions(opts ...SubscribeOption) SubscribeOptions {
	options := SubscribeOptions{
		Context: context.Background(),
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}
