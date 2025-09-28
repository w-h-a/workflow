package notifier

import "context"

type Option func(o *Options)

type Options struct {
	URL     string
	Secret  string
	Context context.Context
}

func WithURL(url string) Option {
	return func(o *Options) {
		o.URL = url
	}
}

func WithSecret(secret string) Option {
	return func(o *Options) {
		o.Secret = secret
	}
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

type NotifyOption func(o *NotifyOptions)

type NotifyOptions struct {
	Context context.Context
}

func NewNotifyOptions(opts ...NotifyOption) NotifyOptions {
	options := NotifyOptions{
		Context: context.Background(),
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}
