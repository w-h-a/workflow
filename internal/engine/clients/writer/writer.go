package writer

import "context"

type Writer interface {
	Write(ctx context.Context, key string, data []byte, opts ...WriteOption) error
}
