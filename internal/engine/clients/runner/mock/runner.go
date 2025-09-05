package mock

import (
	"context"

	testmock "github.com/stretchr/testify/mock"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
)

type mockRunner struct {
	testmock.Mock
}

func (r *mockRunner) Run(ctx context.Context, opts ...runner.RunOption) (string, error) {
	args := r.Called(ctx, opts)
	return args.String(0), args.Error(1)
}

func (r *mockRunner) CreateVolume(ctx context.Context, opts ...runner.CreateVolumeOption) error {
	args := r.Called(ctx, opts)
	return args.Error(0)
}

func (r *mockRunner) DeleteVolume(ctx context.Context, opts ...runner.DeleteVolumeOption) error {
	args := r.Called(ctx, opts)
	return args.Error(0)
}

func (r *mockRunner) CheckHealth(ctx context.Context) error {
	args := r.Called(ctx)
	return args.Error(0)
}

func (r *mockRunner) Close(ctx context.Context) error {
	args := r.Called(ctx)
	return args.Error(0)
}

func NewRunner(opts ...runner.Option) *mockRunner {
	return &mockRunner{}
}
