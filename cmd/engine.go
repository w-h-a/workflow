package cmd

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/w-h-a/pkg/serverv2"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	memorybroker "github.com/w-h-a/workflow/internal/engine/clients/broker/memory"
	"github.com/w-h-a/workflow/internal/engine/clients/broker/nats"
	"github.com/w-h-a/workflow/internal/engine/clients/broker/rabbit"
	"github.com/w-h-a/workflow/internal/engine/clients/notifier"
	"github.com/w-h-a/workflow/internal/engine/clients/notifier/local"
	"github.com/w-h-a/workflow/internal/engine/clients/readwriter"
	memoryreadwriter "github.com/w-h-a/workflow/internal/engine/clients/readwriter/memory"
	"github.com/w-h-a/workflow/internal/engine/clients/readwriter/postgres"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
	"github.com/w-h-a/workflow/internal/engine/clients/runner/docker"
	"github.com/w-h-a/workflow/internal/engine/config"
	"github.com/w-h-a/workflow/internal/engine/handlers/http"
	"github.com/w-h-a/workflow/internal/engine/services/coordinator"
	"github.com/w-h-a/workflow/internal/engine/services/streamer"
	"github.com/w-h-a/workflow/internal/engine/services/worker"
	"github.com/w-h-a/workflow/internal/log"
	"github.com/w-h-a/workflow/internal/task"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/host"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	globallog "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	logsdk "go.opentelemetry.io/otel/sdk/log"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func StartEngine(ctx *cli.Context) error {
	// cfg
	config.New()

	// resource
	name := config.Name()

	resource, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(name),
		),
		resource.WithProcess(),
	)
	if err != nil {
		return err
	}

	// logs
	logsExporter, err := initLogsExporter()
	if err != nil {
		return err
	}

	lp := logsdk.NewLoggerProvider(
		logsdk.WithResource(resource),
		logsdk.WithProcessor(
			logsdk.NewBatchProcessor(logsExporter),
		),
	)

	defer lp.Shutdown(context.Background())

	globallog.SetLoggerProvider(lp)

	logger := otelslog.NewLogger(
		config.Name(),
		otelslog.WithLoggerProvider(lp),
	)

	slog.SetDefault(logger)

	// traces
	traceExporter, err := initTracesExporter()
	if err != nil {
		return err
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithResource(resource),
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
		tracesdk.WithSpanProcessor(
			tracesdk.NewBatchSpanProcessor(
				traceExporter,
			),
		),
	)

	defer tp.Shutdown(context.Background())

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	// metrics
	metricsExporter, err := initMetricsExporter()
	if err != nil {
		return err
	}

	mp := metricsdk.NewMeterProvider(
		metricsdk.WithResource(resource),
		metricsdk.WithReader(
			metricsdk.NewPeriodicReader(
				metricsExporter,
				metricsdk.WithInterval(15*time.Second),
				metricsdk.WithProducer(runtime.NewProducer()),
			),
		),
	)

	defer mp.Shutdown(context.Background())

	otel.SetMeterProvider(mp)

	if err := host.Start(); err != nil {
		return err
	}

	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second)); err != nil {
		return err
	}

	// broker client
	brokerClient := initBroker()

	// wait group & stop channels
	var wg sync.WaitGroup
	stopChannels := map[string]chan struct{}{}
	numServices := 0

	// streamer
	var streamerServer serverv2.Server
	var s *streamer.Service
	if config.Mode() == "standalone" || config.Mode() == "streamer" {
		s = streamer.New(
			brokerClient,
			map[string]int{
				string(log.Queue): 1,
			},
		)

		streamerServer = http.NewStreamerServer(s)

		numServices += 2
	}

	// worker
	var workerServer serverv2.Server
	var w *worker.Service
	if config.Mode() == "standalone" || config.Mode() == "worker" {
		runnerClient := initRunner()

		w = worker.New(
			runnerClient,
			brokerClient,
			config.WorkerQueues(),
		)

		workerServer = http.NewWorkerServer(w)

		numServices += 2
	}

	// coordinator
	var coordinatorServer serverv2.Server
	var c *coordinator.Service
	if config.Mode() == "standalone" || config.Mode() == "coordinator" {
		readwriterClient := initReadWriter()
		notifierClient := initNotifier()

		c = coordinator.New(
			brokerClient,
			readwriterClient,
			notifierClient,
			map[string]int{
				string(task.Started):   1,
				string(task.Completed): 1,
				string(task.Failed):    1,
			},
		)

		coordinatorServer = http.NewCoordinatorServer(c)

		numServices += 2
	}

	// error chan
	ch := make(chan error, numServices)

	// start streamer
	if s != nil {
		stop := make(chan struct{})
		stopChannels["streamer"] = stop

		wg.Add(1)
		go func() {
			defer wg.Done()
			slog.InfoContext(ctx.Context, "starting streamer")
			ch <- s.Start(stop)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			slog.InfoContext(ctx.Context, "starting streamer http server", "address", config.StreamerHttp())
			ch <- streamerServer.Start()
		}()
	}

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

		wg.Add(1)
		go func() {
			defer wg.Done()
			slog.InfoContext(ctx.Context, "starting worker http server", "address", config.WorkerHttp())
			ch <- workerServer.Start()
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
			slog.InfoContext(ctx.Context, "starting coordinator http server", "address", config.CoordinatorHttp())
			ch <- coordinatorServer.Start()
		}()
	}

	// block
	err = <-ch
	if err != nil {
		slog.ErrorContext(ctx.Context, "failed to start", "error", err)
		return err
	}

	// graceful shutdown
	slog.InfoContext(ctx.Context, "stopping...")

	if stop, ok := stopChannels["streamer"]; ok {
		close(stop)
	}

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

func initLogsExporter() (logsdk.Exporter, error) {
	return stdoutlog.New()
}

func initTracesExporter() (tracesdk.SpanExporter, error) {
	return otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpoint(config.TracesAddress()),
		otlptracehttp.WithInsecure(),
	)
}

func initMetricsExporter() (metricsdk.Exporter, error) {
	return otlpmetrichttp.New(
		context.Background(),
		otlpmetrichttp.WithEndpoint(config.MetricsAddress()),
		otlpmetrichttp.WithInsecure(),
	)
}

func initBroker() broker.Broker {
	switch config.Broker() {
	case string(broker.Nats):
		return nats.NewBroker(
			broker.WithLocation(config.BrokerLocation()),
		)
	case string(broker.Rabbit):
		return rabbit.NewBroker(
			broker.WithLocation(config.BrokerLocation()),
			broker.WithDurable(config.BrokerDurable()),
		)
	default:
		return memorybroker.NewBroker()
	}
}

func initRunner() runner.Runner {
	return docker.NewRunner(
		runner.WithHost(config.RunnerHost()),
		runner.WithRegistryUser(config.RunnerRegistryUser()),
		runner.WithRegistryPass(config.RunnerRegistryPass()),
		runner.WithPruneInterval(config.RunnerPruneInterval()),
	)
}

func initReadWriter() readwriter.ReadWriter {
	switch config.ReadWriter() {
	case string(readwriter.Postgres):
		return postgres.NewReadWriter(
			readwriter.WithLocation(config.ReadWriterLocation()),
		)
	default:
		return memoryreadwriter.NewReadWriter()
	}
}

func initNotifier() notifier.Notifier {
	return local.NewNotifier()
}
