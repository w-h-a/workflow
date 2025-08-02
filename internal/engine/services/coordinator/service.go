package coordinator

import (
	"github.com/google/uuid"
	"github.com/w-h-a/workflow/internal/task"
)

// Coordinator is responsible for accepting tasks from clients,
// scheduling tasks for workers, and for exposing the cluster's state.
type Service struct {
	// broker
	taskCache     map[string][]task.Task
	workers       []string
	workerToTasks map[string][]uuid.UUID
	taskToWorker  map[uuid.UUID]string
}
