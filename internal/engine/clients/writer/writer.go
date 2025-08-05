package writer

import "context"

type Writer interface {
	Write(ctx context.Context, id string, data []byte, opts ...WriteOption) error
}
