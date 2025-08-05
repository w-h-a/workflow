package reader

import "context"

type Reader interface {
	ReadByKey(ctx context.Context, key string, opts ...ReadByKeyOption) ([]byte, error)
}
