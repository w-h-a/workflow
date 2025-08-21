package reader

import "context"

type Reader interface {
	Read(ctx context.Context, opts ...ReadOption) (*Page, error)
	ReadById(ctx context.Context, id string, opts ...ReadByIdOption) ([]byte, error)
}
