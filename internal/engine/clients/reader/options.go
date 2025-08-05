package reader

import "context"

type Option func(o *Options)

type Options struct {
	Location string
	Context  context.Context
}

func WithLocation(loc string) Option {
	return func(o *Options) {
		o.Location = loc
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

type ReadByKeyOption func(o *ReadByKeyOptions)

type ReadByKeyOptions struct {
	Context context.Context
}

func NewReadByKeyOptions(opts ...ReadByKeyOption) ReadByKeyOptions {
	options := ReadByKeyOptions{
		Context: context.Background(),
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}
