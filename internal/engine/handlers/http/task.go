package http

import (
	"log/slog"
	"net/http"

	"github.com/w-h-a/workflow/internal/engine/services/coordinator"
)

type Task struct {
	parser             *Parser
	coordinatorService *coordinator.Service
}

func (t *Task) PostTask(w http.ResponseWriter, r *http.Request) {
	ctx := reqToCtx(r)

	task, err := t.parser.ParsePostBody(ctx, r)
	if err != nil {
		wrtRsp(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	slog.InfoContext(ctx, "ready to process task", "task", *task)
	// TODO
}

func NewTaskHandler(coordinatorService *coordinator.Service) *Task {
	return &Task{
		parser:             &Parser{},
		coordinatorService: coordinatorService,
	}
}
