package migrator

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

type MigrateOption func(o *MigrateOptions)

type MigrateOptions struct {
	Context context.Context
}

func NewMigrateOptions(opts ...MigrateOption) MigrateOptions {
	options := MigrateOptions{
		Context: context.Background(),
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}
