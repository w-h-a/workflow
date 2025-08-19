package engine

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/w-h-a/pkg/serverv2"
	httpserver "github.com/w-h-a/pkg/serverv2/http"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"github.com/w-h-a/workflow/internal/engine/clients/readwriter"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
	"github.com/w-h-a/workflow/internal/engine/config"
	httphandlers "github.com/w-h-a/workflow/internal/engine/handlers/http"
	"github.com/w-h-a/workflow/internal/engine/services/coordinator"
	"github.com/w-h-a/workflow/internal/engine/services/worker"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

func NewCoordinator(
	brokerClient broker.Broker,
	readwriterClient readwriter.ReadWriter,
) (serverv2.Server, *coordinator.Service) {
	// services
	coordinatorService := coordinator.New(
		brokerClient,
		readwriterClient,
		map[string]int{
			broker.STARTED:   1,
			broker.COMPLETED: 1,
			broker.FAILED:    1,
		},
	)

	// base server options
	opts := []serverv2.ServerOption{
		serverv2.ServerWithNamespace(config.Env()),
		serverv2.ServerWithName(config.Name()),
		serverv2.ServerWithVersion(config.Version()),
	}

	// create http router
	router := mux.NewRouter()

	httpStatus := httphandlers.NewStatusHandler()
	router.Methods(http.MethodGet).Path("/status").HandlerFunc(httpStatus.GetStatus)

	httpTasks := httphandlers.NewTasksHandler(coordinatorService)
	router.Methods(http.MethodGet).Path("/tasks/{id}").HandlerFunc(httpTasks.GetOneTask)
	router.Methods(http.MethodPut).Path("/tasks/cancel/{id}").HandlerFunc(httpTasks.PutCancelTask)
	router.Methods(http.MethodPost).Path("/tasks").HandlerFunc(httpTasks.PostTask)

	// create http server
	httpOpts := []serverv2.ServerOption{
		serverv2.ServerWithAddress(config.HttpAddress()),
	}

	httpOpts = append(httpOpts, opts...)

	httpServer := httpserver.NewServer(httpOpts...)

	handler := otelhttp.NewHandler(
		router,
		"",
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string { return r.URL.Path }),
		otelhttp.WithTracerProvider(otel.GetTracerProvider()),
		otelhttp.WithPropagators(otel.GetTextMapPropagator()),
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/status" }),
	)

	httpServer.Handle(handler)

	return httpServer, coordinatorService
}

func NewWorker(
	brokerClient broker.Broker,
	runnerClient runner.Runner,
) *worker.Service {
	workerService := worker.New(
		runnerClient,
		brokerClient,
		config.Queues(),
	)

	return workerService
}
