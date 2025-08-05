package http

import (
	"net/http"

	"github.com/w-h-a/workflow/internal/engine/services/coordinator"
)

type Task struct {
	parser      *Parser
	coordinator *coordinator.Service
}

func (t *Task) PostTask(w http.ResponseWriter, r *http.Request) {
	ctx := reqToCtx(r)

	task, err := t.parser.ParsePostBody(ctx, r)
	if err != nil {
		wrtRsp(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	scheduled, err := t.coordinator.ScheduleTask(ctx, task)
	if err != nil {
		wrtRsp(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	wrtRsp(w, http.StatusOK, scheduled)
}

func NewTaskHandler(coordinatorService *coordinator.Service) *Task {
	return &Task{
		parser:      &Parser{},
		coordinator: coordinatorService,
	}
}
