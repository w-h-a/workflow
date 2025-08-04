package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/w-h-a/workflow/internal/engine"
	"github.com/w-h-a/workflow/internal/engine/clients/broker/memory"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
	"github.com/w-h-a/workflow/internal/engine/clients/runner/docker"
	"github.com/w-h-a/workflow/internal/engine/config"
)

func main() {
	// cfg
	config.New()

	// ctx
	ctx := context.Background()

	// clients
	runnerClient := docker.NewRunner(
		runner.WithHost("unix:///Users/wesleyanderson/.docker/run/docker.sock"),
	)

	brokerClient := memory.NewBroker()

	// server + services
	httpServer, w := engine.Factory(
		runnerClient,
		brokerClient,
	)

	// wait group and error chan
	wg := &sync.WaitGroup{}
	ch := make(chan error, 2)

	// start worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.InfoContext(ctx, "starting worker")
		ch <- w.Start()
	}()

	// start http server
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.InfoContext(ctx, "starting http server", "address", config.HttpAddress())
		ch <- httpServer.Start()
	}()

	// block
	err := <-ch
	if err != nil {
		slog.ErrorContext(ctx, "failed to start", "error", err)
		return
	}

	// graceful shutdown
	slog.InfoContext(ctx, "stopping...")

	wait := make(chan struct{})

	go func() {
		defer close(wait)
		wg.Wait()
	}()

	select {
	case <-wait:
	case <-time.After(30 * time.Second):
	}

	slog.InfoContext(ctx, "successfully stopped")
}
