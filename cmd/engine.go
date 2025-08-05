package cmd

import (
	"log/slog"
	"sync"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/w-h-a/workflow/internal/engine"
	memorybroker "github.com/w-h-a/workflow/internal/engine/clients/broker/memory"
	memoryreadwriter "github.com/w-h-a/workflow/internal/engine/clients/readwriter/memory"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
	"github.com/w-h-a/workflow/internal/engine/clients/runner/docker"
	"github.com/w-h-a/workflow/internal/engine/config"
)

func StartEngine(ctx *cli.Context) error {
	// cfg
	config.New()

	// clients
	runnerClient := docker.NewRunner(
		runner.WithHost("unix:///Users/wesleyanderson/.docker/run/docker.sock"),
	)

	brokerClient := memorybroker.NewBroker()

	readwriterClient := memoryreadwriter.NewReadWriter()

	// server + services
	httpServer, c, w := engine.Factory(
		runnerClient,
		brokerClient,
		readwriterClient,
	)

	// wait group and error chan
	wg := &sync.WaitGroup{}
	ch := make(chan error, 3)

	// start worker
	workerStop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.InfoContext(ctx.Context, "starting worker")
		ch <- w.Start(workerStop)
	}()

	// start coordinator
	coordinatorStop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.InfoContext(ctx.Context, "starting coordinator")
		ch <- c.Start(coordinatorStop)
	}()

	// start http server
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.InfoContext(ctx.Context, "starting http server", "address", config.HttpAddress())
		ch <- httpServer.Start()
	}()

	// block
	err := <-ch
	if err != nil {
		slog.ErrorContext(ctx.Context, "failed to start", "error", err)
		return err
	}

	// graceful shutdown
	slog.InfoContext(ctx.Context, "stopping...")

	wait := make(chan struct{})

	go func() {
		defer close(wait)
		wg.Wait()
	}()

	close(workerStop)
	close(coordinatorStop)

	select {
	case <-wait:
	case <-time.After(30 * time.Second):
	}

	slog.InfoContext(ctx.Context, "successfully stopped")

	return nil
}
