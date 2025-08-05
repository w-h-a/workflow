package reader

import "context"

type Reader interface {
	ReadById(ctx context.Context, id string, opts ...ReadByIdOption) ([]byte, error)
}
