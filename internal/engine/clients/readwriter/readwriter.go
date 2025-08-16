package readwriter

import (
	"github.com/w-h-a/workflow/internal/engine/clients/reader"
	"github.com/w-h-a/workflow/internal/engine/clients/writer"
)

type ReadWriterType string

const (
	Memory ReadWriterType = "memory"
)

var (
	ReadWriterTypes = map[string]ReadWriterType{
		"memory": Memory,
	}
)

type ReadWriter interface {
	reader.Reader
	writer.Writer
}
