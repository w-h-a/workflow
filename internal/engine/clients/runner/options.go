package runner

import (
	"context"
	"time"
)

type Option func(o *Options)

type Options struct {
	Host          string
	RegistryUser  string
	RegistryPass  string
	PruneInterval time.Duration
	Context       context.Context
}

func WithHost(host string) Option {
	return func(o *Options) {
		o.Host = host
	}
}

func WithRegistryUser(user string) Option {
	return func(o *Options) {
		o.RegistryUser = user
	}
}

func WithRegistryPass(pass string) Option {
	return func(o *Options) {
		o.RegistryPass = pass
	}
}

func WithPruneInterval(interval time.Duration) Option {
	return func(o *Options) {
		o.PruneInterval = interval
	}
}

func NewOptions(opts ...Option) Options {
	options := Options{
		Context:       context.Background(),
		PruneInterval: 24 * time.Hour,
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}

type RunOption func(o *RunOptions)

type RunOptions struct {
	ID       string
	Image    string
	Cmd      []string
	Env      []string
	Mounts   []map[string]string
	Networks []string
	Context  context.Context
}

func RunWithID(id string) RunOption {
	return func(o *RunOptions) {
		o.ID = id
	}
}

func RunWithImage(image string) RunOption {
	return func(o *RunOptions) {
		o.Image = image
	}
}

func RunWithCmd(cmd []string) RunOption {
	return func(o *RunOptions) {
		o.Cmd = cmd
	}
}

func RunWithEnv(env []string) RunOption {
	return func(o *RunOptions) {
		o.Env = env
	}
}

func RunWithMounts(mounts []map[string]string) RunOption {
	return func(o *RunOptions) {
		o.Mounts = mounts
	}
}

func RunWithNetworks(networks []string) RunOption {
	return func(o *RunOptions) {
		o.Networks = networks
	}
}

func NewRunOptions(opts ...RunOption) RunOptions {
	options := RunOptions{
		Context: context.Background(),
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}

type CreateVolumeOption func(o *CreateVolumeOptions)

type CreateVolumeOptions struct {
	Name    string
	Context context.Context
}

func CreateVolumeWithName(name string) CreateVolumeOption {
	return func(o *CreateVolumeOptions) {
		o.Name = name
	}
}

func NewCreateVolumeOptions(opts ...CreateVolumeOption) CreateVolumeOptions {
	options := CreateVolumeOptions{
		Context: context.Background(),
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}

type DeleteVolumeOption func(o *DeleteVolumeOptions)

type DeleteVolumeOptions struct {
	Name    string
	Context context.Context
}

func DeleteVolumeWithName(name string) DeleteVolumeOption {
	return func(o *DeleteVolumeOptions) {
		o.Name = name
	}
}

func NewDeleteVolumeOptions(opts ...DeleteVolumeOption) DeleteVolumeOptions {
	options := DeleteVolumeOptions{
		Context: context.Background(),
	}

	for _, fn := range opts {
		fn(&options)
	}

	return options
}
