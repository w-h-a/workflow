package memory

import (
	"context"
	"sync"

	"github.com/w-h-a/workflow/internal/engine/clients/reader"
	"github.com/w-h-a/workflow/internal/engine/clients/readwriter"
	"github.com/w-h-a/workflow/internal/engine/clients/writer"
)

type memoryReadWriter struct {
	options readwriter.Options
	store   map[string][]byte
	mtx     sync.RWMutex
}

func (rw *memoryReadWriter) ReadByKey(ctx context.Context, key string, opts ...reader.ReadByKeyOption) ([]byte, error) {
	rw.mtx.RLock()
	defer rw.mtx.RUnlock()

	data, ok := rw.store[key]
	if !ok {
		return nil, reader.ErrNotFound
	}

	return append([]byte{}, data...), nil
}

func (rw *memoryReadWriter) Write(ctx context.Context, key string, data []byte, opts ...writer.WriteOption) error {
	rw.mtx.Lock()
	defer rw.mtx.Unlock()

	rw.store[key] = append([]byte{}, data...)

	return nil
}

func NewReadWriter(opts ...readwriter.Option) readwriter.ReadWriter {
	options := readwriter.NewOptions(opts...)

	rw := &memoryReadWriter{
		options: options,
		store:   map[string][]byte{},
		mtx:     sync.RWMutex{},
	}

	return rw
}
