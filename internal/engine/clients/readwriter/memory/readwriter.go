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
	ids     []string
	mtx     sync.RWMutex
}

func (rw *memoryReadWriter) Read(ctx context.Context, opts ...reader.ReadOption) (*reader.Page, error) {
	options := reader.NewReadOptions(opts...)

	offset := (options.Page - 1) * options.Size

	rw.mtx.RLock()
	defer rw.mtx.RUnlock()

	results := [][]byte{}

	for i := offset; i < (options.Size+offset) && i < len(rw.ids); i++ {
		data, ok := rw.store[rw.ids[i]]
		if !ok {
			continue
		}
		results = append(results, append([]byte{}, data...))
	}

	totalPages := (len(rw.ids) + options.Size - 1) / options.Size

	return &reader.Page{
		Items:      results,
		Size:       len(results),
		Number:     options.Page,
		TotalPages: totalPages,
		TotalItems: len(rw.ids),
	}, nil
}

func (rw *memoryReadWriter) ReadById(ctx context.Context, id string, opts ...reader.ReadByIdOption) ([]byte, error) {
	rw.mtx.RLock()
	defer rw.mtx.RUnlock()

	data, ok := rw.store[id]
	if !ok {
		return nil, reader.ErrRecordNotFound
	}

	return append([]byte{}, data...), nil
}

func (rw *memoryReadWriter) Write(ctx context.Context, id string, data []byte, opts ...writer.WriteOption) error {
	rw.mtx.Lock()
	defer rw.mtx.Unlock()

	if _, exists := rw.store[id]; !exists {
		rw.ids = append([]string{id}, rw.ids...)
	}

	rw.store[id] = append([]byte{}, data...)

	return nil
}

func NewReadWriter(opts ...readwriter.Option) readwriter.ReadWriter {
	options := readwriter.NewOptions(opts...)

	rw := &memoryReadWriter{
		options: options,
		store:   map[string][]byte{},
		ids:     []string{},
		mtx:     sync.RWMutex{},
	}

	return rw
}
