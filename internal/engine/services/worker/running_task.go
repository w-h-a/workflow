package worker

import (
	"context"

	"github.com/w-h-a/workflow/internal/task"
)

type runningTask struct {
	task   *task.Task
	cancel context.CancelFunc
}
