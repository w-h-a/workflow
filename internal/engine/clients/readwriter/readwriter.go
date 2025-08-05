package readwriter

import (
	"github.com/w-h-a/workflow/internal/engine/clients/reader"
	"github.com/w-h-a/workflow/internal/engine/clients/writer"
)

type ReadWriter interface {
	reader.Reader
	writer.Writer
}
