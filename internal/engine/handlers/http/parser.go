package http

import (
	"context"
	"io"
	"net/http"

	"github.com/w-h-a/workflow/internal/task"
)

type Parser struct{}

func (p *Parser) ParsePostBody(ctx context.Context, r *http.Request) (*task.Task, error) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	defer r.Body.Close()

	return task.Factory(bs)
}
