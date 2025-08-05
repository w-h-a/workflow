package http

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/w-h-a/workflow/internal/task"
)

type Parser struct{}

func (p *Parser) ParseTaskId(ctx context.Context, r *http.Request) (string, error) {
	vars := mux.Vars(r)

	taskKey := vars["id"]

	if len(taskKey) == 0 {
		return "", fmt.Errorf("task id is required")
	}

	return taskKey, nil
}

func (p *Parser) ParsePostBody(ctx context.Context, r *http.Request) (*task.Task, error) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	defer r.Body.Close()

	return task.Factory(bs)
}
