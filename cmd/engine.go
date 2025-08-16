package cmd

import (
	"log/slog"
	"sync"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/w-h-a/pkg/serverv2"
	"github.com/w-h-a/workflow/internal/engine"
	memorybroker "github.com/w-h-a/workflow/internal/engine/clients/broker/memory"
	memoryreadwriter "github.com/w-h-a/workflow/internal/engine/clients/readwriter/memory"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
	"github.com/w-h-a/workflow/internal/engine/clients/runner/docker"
	"github.com/w-h-a/workflow/internal/engine/config"
	"github.com/w-h-a/workflow/internal/engine/services/coordinator"
	"github.com/w-h-a/workflow/internal/engine/services/worker"
)

func StartEngine(ctx *cli.Context) error {
	// cfg
	config.New()

	// broker client
	brokerClient := memorybroker.NewBroker()

	// wait group & stop channels
	var wg sync.WaitGroup
	stopChannels := map[string]chan struct{}{}
	numServices := 0

	// worker
	var w *worker.Service
	if config.Mode() == "standalone" || config.Mode() == "worker" {
		runnerClient := docker.NewRunner(
			runner.WithHost("unix:///Users/wesleyanderson/.docker/run/docker.sock"),
		)

		w = engine.NewWorker(
			brokerClient,
			runnerClient,
		)

		numServices++
	}

	// coordinator
	var httpServer serverv2.Server
	var c *coordinator.Service
	if config.Mode() == "standalone" || config.Mode() == "coordinator" {
		readwriterClient := memoryreadwriter.NewReadWriter()

		httpServer, c = engine.NewCoordinator(
			brokerClient,
			readwriterClient,
		)

		numServices += 2
	}

	// error chan
	ch := make(chan error, numServices)

	// start worker
	if w != nil {
		stop := make(chan struct{})
		stopChannels["worker"] = stop

		wg.Add(1)
		go func() {
			defer wg.Done()
			slog.InfoContext(ctx.Context, "starting worker")
			ch <- w.Start(stop)
		}()
	}

	// start coordinator
	if c != nil {
		stop := make(chan struct{})
		stopChannels["coordinator"] = stop

		wg.Add(1)
		go func() {
			defer wg.Done()
			slog.InfoContext(ctx.Context, "starting coordinator")
			ch <- c.Start(stop)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			slog.InfoContext(ctx.Context, "starting http server", "address", config.HttpAddress())
			ch <- httpServer.Start()
		}()
	}

	// block
	err := <-ch
	if err != nil {
		slog.ErrorContext(ctx.Context, "failed to start", "error", err)
		return err
	}

	// graceful shutdown
	slog.InfoContext(ctx.Context, "stopping...")

	if stop, ok := stopChannels["worker"]; ok {
		close(stop)
	}

	if stop, ok := stopChannels["coordinator"]; ok {
		close(stop)
	}

	wait := make(chan struct{})
	go func() {
		defer close(wait)
		wg.Wait()
	}()

	select {
	case <-wait:
	case <-time.After(30 * time.Second):
	}

	slog.InfoContext(ctx.Context, "successfully stopped")

	return nil
}
