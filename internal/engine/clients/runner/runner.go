package runner

import "context"

type RuntimeType string

const (
	Docker RuntimeType = "docker"
)

type Runner interface {
	Run(ctx context.Context, opts ...RunOption) (string, error)
	Stop(ctx context.Context, opts ...StopOption) error
}
