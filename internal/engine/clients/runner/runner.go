package runner

import "context"

type RuntimeType string

const (
	Docker RuntimeType = "docker"
)

var (
	RuntimeTypes = map[string]RuntimeType{
		"docker": Docker,
	}
)

type Runner interface {
	Run(ctx context.Context, opts ...RunOption) (string, error)
	CreateVolume(ctx context.Context, opts ...CreateVolumeOption) error
	DeleteVolume(ctx context.Context, opts ...DeleteVolumeOption) error
	CheckHealth(ctx context.Context) error
	Close(ctx context.Context) error
}
