package coordinator

import (
	"github.com/google/uuid"
	"github.com/w-h-a/workflow/internal/task"
)

type Service struct {
	// broker
	taskCache     map[string][]task.Task
	eventCache    map[string][]task.TaskEvent
	workers       []string
	workerToTasks map[string][]uuid.UUID
	taskToWorker  map[uuid.UUID]string
}
