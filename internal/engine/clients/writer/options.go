package writer

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

type WriteOption func(o *WriteOptions)

type WriteOptions struct {
	Context context.Context
}

func NewWriteOptions(opts ...WriteOption) WriteOptions {
	options := WriteOptions{
		Context: context.Background(),
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}
