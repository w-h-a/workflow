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

type RunOption func(o *RunOptions)

type RunOptions struct {
	ID      string
	Image   string
	Cmd     []string
	Env     []string
	Volumes []string
	Context context.Context
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

func RunWithVolumes(volumes []string) RunOption {
	return func(o *RunOptions) {
		o.Volumes = volumes
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
