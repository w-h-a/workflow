package worker

import (
	"github.com/google/uuid"
	"github.com/w-h-a/workflow/internal/task"
)

type Service struct {
	name string
	// broker
	cache     map[uuid.UUID]task.Task
	taskCount int
}
