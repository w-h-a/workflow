package http

import (
	"errors"
	"net/http"

	"github.com/w-h-a/workflow/internal/engine/services/coordinator"
	"github.com/w-h-a/workflow/internal/task"
)

type Task struct {
	parser      *Parser
	coordinator *coordinator.Service
}

func (t *Task) GetOneTask(w http.ResponseWriter, r *http.Request) {
	ctx := reqToCtx(r)

	taskId, err := t.parser.ParseTaskId(ctx, r)
	if err != nil {
		wrtRsp(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	result, err := t.coordinator.RetrieveTask(ctx, taskId)
	if err != nil && errors.Is(err, task.ErrTaskNotFound) {
		wrtRsp(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	} else if err != nil {
		wrtRsp(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	wrtRsp(w, http.StatusOK, result)
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

func (t *Task) PutCancelTask(w http.ResponseWriter, r *http.Request) {
	ctx := reqToCtx(r)

	taskId, err := t.parser.ParseTaskId(ctx, r)
	if err != nil {
		wrtRsp(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	result, err := t.coordinator.RetrieveTask(ctx, taskId)
	if err != nil && errors.Is(err, task.ErrTaskNotFound) {
		wrtRsp(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	} else if err != nil {
		wrtRsp(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	if result.State != task.Started {
		wrtRsp(w, http.StatusBadRequest, map[string]any{"error": "task is not cancellable"})
		return
	}

	cancelled, err := t.coordinator.CancelTask(ctx, result)
	if err != nil {
		wrtRsp(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	wrtRsp(w, http.StatusOK, cancelled)
}

func NewTaskHandler(coordinatorService *coordinator.Service) *Task {
	return &Task{
		parser:      &Parser{},
		coordinator: coordinatorService,
	}
}
