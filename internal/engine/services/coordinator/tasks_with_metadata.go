package coordinator

import "github.com/w-h-a/workflow/internal/task"

type TasksWithMetadata struct {
	Tasks      []*task.Task `json:"tasks"`
	Size       int          `json:"size"`
	Number     int          `json:"number"`
	TotalPages int          `json:"totalPages"`
	TotalTasks int          `json:"totalTasks"`
}
