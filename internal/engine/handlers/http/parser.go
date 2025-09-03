package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/w-h-a/workflow/internal/task"
)

const (
	defaultPage = "1"
	defaultSize = "10"
	minPage     = 1
	maxSize     = 20
	minSize     = 1
)

type Parser struct{}

func (p *Parser) ParseTasksQuery(ctx context.Context, r *http.Request) (int, int, error) {
	ps := r.URL.Query().Get("page")
	if len(ps) == 0 {
		ps = defaultPage
	}

	page, err := strconv.Atoi(ps)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid page number: %s", ps)
	}

	if page < minPage {
		page = minPage
	}

	ss := r.URL.Query().Get("size")
	if len(ss) == 0 {
		ss = defaultSize
	}

	size, err := strconv.Atoi(ss)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid size: %s", ss)
	}

	if size < minSize {
		size = minSize
	} else if size > maxSize {
		size = maxSize
	}

	return page, size, nil
}

func (p *Parser) ParseTaskId(ctx context.Context, r *http.Request) (string, error) {
	vars := mux.Vars(r)

	taskKey := vars["id"]

	if len(taskKey) == 0 {
		return "", fmt.Errorf("task id is required")
	}

	return taskKey, nil
}

func (p *Parser) ParsePostTaskBody(ctx context.Context, r *http.Request) (*task.Task, error) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	defer r.Body.Close()

	return task.Factory(bs)
}
