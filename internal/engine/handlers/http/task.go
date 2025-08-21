package http

import (
	"errors"
	"net/http"

	"github.com/w-h-a/workflow/internal/engine/services/coordinator"
	"github.com/w-h-a/workflow/internal/task"
)

type Tasks struct {
	parser      *Parser
	coordinator *coordinator.Service
}

func (t *Tasks) GetTasks(w http.ResponseWriter, r *http.Request) {
	ctx := reqToCtx(r)

	page, size, err := t.parser.ParseTasksQuery(ctx, r)
	if err != nil {
		wrtRsp(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	tasks, err := t.coordinator.RetrieveTasks(ctx, page, size)
	if err != nil {
		wrtRsp(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	wrtRsp(w, http.StatusOK, tasks)
}

func (t *Tasks) GetOneTask(w http.ResponseWriter, r *http.Request) {
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

func (t *Tasks) PostTask(w http.ResponseWriter, r *http.Request) {
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

func (t *Tasks) PutCancelTask(w http.ResponseWriter, r *http.Request) {
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

func NewTasksHandler(coordinatorService *coordinator.Service) *Tasks {
	return &Tasks{
		parser:      &Parser{},
		coordinator: coordinatorService,
	}
}
