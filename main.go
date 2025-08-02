package main

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"github.com/w-h-a/workflow/internal/engine/clients/broker/memory"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
	"github.com/w-h-a/workflow/internal/engine/clients/runner/docker"
	"github.com/w-h-a/workflow/internal/engine/services/worker"
	"github.com/w-h-a/workflow/internal/task"
)

func main() {
	ctx := context.Background()

	runnerClient := docker.NewRunner(
		runner.WithHost("unix:///Users/wesleyanderson/.docker/run/docker.sock"),
	)

	brokerClient := memory.NewBroker()

	w := worker.New(runnerClient, brokerClient)

	if err := w.Subscribe(ctx); err != nil {
		panic(err)
	}

	t := task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Pending,
		Name:  "test-container-1",
		Image: "postgres:13",
		Env: []string{
			"POSTGRES_USER=user",
			"POSTGRES_PASSWORD=secret",
		},
	}

	bs, _ := json.Marshal(t)

	opts := []broker.PublishOption{
		broker.PublishWithTopic(w.Name()),
	}

	if err := brokerClient.Publish(ctx, bs, opts...); err != nil {
		panic(err)
	}

	time.Sleep(10 * time.Second)

	if err := w.StopTask(ctx, t); err != nil {
		panic(err)
	}
}
