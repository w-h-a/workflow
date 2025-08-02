package task

import (
	"time"
)

const (
	// Pending is the initial state for every task
	Pending State = iota
	// Scheduled is the state once the coordinator has assigned a worker to the task
	Scheduled
	// Running is the state when a worker successfully starts the task
	Running
	// Completed is the state when the worker successfully completes the task
	Completed
	// Failed is the state when the worker has failed to start or complete the task
	Failed
)

type State int

type Task struct {
	ID            string
	Name          string
	State         State
	StartTime     time.Time
	EndTime       time.Time
	Image         string
	CMD           []string
	Memory        int64
	Disk          int64
	Env           []string
	RestartPolicy string
}

type TaskEvent struct {
	ID        string
	State     State
	Timestamp time.Time
	Task      Task
}
