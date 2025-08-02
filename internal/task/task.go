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
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	State         State     `json:"state"`
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime"`
	Image         string    `json:"image"`
	CMD           []string  `json:"cmd"`
	Memory        int64     `json:"memory"`
	Disk          int64     `json:"disk"`
	Env           []string  `json:"env"`
	RestartPolicy string    `json:"restartPolicy"`
}
