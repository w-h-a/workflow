package main

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
	"github.com/w-h-a/workflow/internal/engine/clients/runner/docker"
	"github.com/w-h-a/workflow/internal/engine/services/worker"
	"github.com/w-h-a/workflow/internal/task"
)

func main() {
	runnerClient := docker.NewRunner(
		runner.WithHost("unix:///Users/wesleyanderson/.docker/run/docker.sock"),
	)

	w := worker.New(runnerClient)

	t := task.Task{
		ID:    uuid.New().String(),
		State: task.Pending,
		Name:  "test-container-1",
		Image: "postgres:13",
		Env: []string{
			"POSTGRES_USER=user",
			"POSTGRES_PASSWORD=secret",
		},
	}

	w.EnqueueTask(context.Background(), t)

	if err := w.RunTask(context.Background()); err != nil {
		panic(err)
	}

	time.Sleep(10 * time.Second)

	if err := w.StopTask(context.Background(), t); err != nil {
		panic(err)
	}
}
