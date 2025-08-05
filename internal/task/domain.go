package task

import (
	"time"
)

type State string

const (
	Pending   State = "PENDING"
	Scheduled State = "SCHEDULED"
	Running   State = "RUNNING"
	Cancelled State = "CANCELLED"
	Stopped   State = "STOPPED"
	Completed State = "COMPLETED"
	Failed    State = "FAILED"
)

type Task struct {
	ID            string     `json:"id"`
	State         State      `json:"state"`
	Image         string     `json:"image"`
	Cmd           []string   `json:"cmd,omitempty"`
	Env           []string   `json:"env,omitempty"`
	Memory        int64      `json:"memory,omitempty"`
	Disk          int64      `json:"disk,omitempty"`
	RestartPolicy string     `json:"restartPolicy,omitempty"`
	ScheduledAt   *time.Time `json:"scheduledAt,omitempty"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
	FailedAt      *time.Time `json:"failedAt,omitempty"`
	Result        string     `json:"result,omitempty"`
	Error         string     `json:"error,omitempty"`
}
