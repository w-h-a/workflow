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
