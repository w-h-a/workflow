package runner

import "context"

type Runner interface {
	Start(ctx context.Context, opts ...StartOption) error
	Stop(ctx context.Context, opts ...StopOption) error
}
