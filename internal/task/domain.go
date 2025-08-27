package task

import (
	"time"
)

type State string

const (
	Scheduled State = "SCHEDULED"
	Cancelled State = "CANCELLED"
	Started   State = "STARTED"
	Completed State = "COMPLETED"
	Failed    State = "FAILED"
)

type Task struct {
	ID          string     `json:"id,omitempty"`
	State       State      `json:"state,omitempty"`
	Image       string     `json:"image,omitempty"`
	Cmd         []string   `json:"cmd,omitempty"`
	Env         []string   `json:"env,omitempty"`
	ScheduledAt *time.Time `json:"scheduledAt,omitempty"`
	CancelledAt *time.Time `json:"cancelledAt,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	FailedAt    *time.Time `json:"failedAt,omitempty"`
	Queue       string     `json:"queue,omitempty"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	Pre         []*Task    `json:"pre,omitempty"`
	Post        []*Task    `json:"post,omitempty"`
	Volumes     []string   `json:"volumes,omitempty"`
	Networks    []string   `json:"networks,omitempty"`
	Retry       *Retry     `json:"retry,omitempty"`
	Timeout     string     `json:"timeout,omitempty"`
}

type Retry struct {
	Limit    int `json:"limit,omitempty"`
	Attempts int `json:"attempts,omitempty"`
}
