package runner

import "context"

type Option func(o *Options)

type Options struct {
	Host    string
	Context context.Context
}

func WithHost(host string) Option {
	return func(o *Options) {
		o.Host = host
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

type StartOption func(o *StartOptions)

type StartOptions struct {
	ID            string
	Name          string
	Image         string
	Memory        int64
	Env           []string
	RestartPolicy string
	Context       context.Context
}

func StartWithID(id string) StartOption {
	return func(o *StartOptions) {
		o.ID = id
	}
}

func StartWithName(name string) StartOption {
	return func(o *StartOptions) {
		o.Name = name
	}
}

func StartWithImage(image string) StartOption {
	return func(o *StartOptions) {
		o.Image = image
	}
}

func StartWithMemory(memory int64) StartOption {
	return func(o *StartOptions) {
		o.Memory = memory
	}
}

func StartWithEnv(env []string) StartOption {
	return func(o *StartOptions) {
		o.Env = env
	}
}

func StartWithRestartPolicy(policy string) StartOption {
	return func(o *StartOptions) {
		o.RestartPolicy = policy
	}
}

func NewStartOptions(opts ...StartOption) StartOptions {
	options := StartOptions{
		Context: context.Background(),
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}

type StopOption func(o *StopOptions)

type StopOptions struct {
	ID      string
	Context context.Context
}

func StopWithID(id string) StopOption {
	return func(o *StopOptions) {
		o.ID = id
	}
}

func NewStopOptions(opts ...StopOption) StopOptions {
	options := StopOptions{
		Context: context.Background(),
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}
