package runner

import "context"

type RuntimeType string

const (
	Docker RuntimeType = "docker"
)

type Runner interface {
	Start(ctx context.Context, opts ...StartOption) error
	Stop(ctx context.Context, opts ...StopOption) error
}
