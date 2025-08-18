package readwriter

import (
	"github.com/w-h-a/workflow/internal/engine/clients/reader"
	"github.com/w-h-a/workflow/internal/engine/clients/writer"
)

type ReadWriterType string

const (
	Memory   ReadWriterType = "memory"
	Postgres ReadWriterType = "postgres"
)

var (
	ReadWriterTypes = map[string]ReadWriterType{
		"memory":   Memory,
		"postgres": Postgres,
	}
)

type ReadWriter interface {
	reader.Reader
	writer.Writer
}
