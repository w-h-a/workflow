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

func Factory(
	runnerClient runner.Runner,
	brokerClient broker.Broker,
	readwriterClient readwriter.ReadWriter,
) (serverv2.Server, *coordinator.Service, *worker.Service) {
	// services
	coordinatorService := coordinator.New(brokerClient, readwriterClient)
	workerService := worker.New(runnerClient, brokerClient)

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

	httpTask := httphandlers.NewTaskHandler(coordinatorService)
	router.Methods(http.MethodGet).Path("/task/{id}").HandlerFunc(httpTask.GetOneTask)
	router.Methods(http.MethodPost).Path("/task").HandlerFunc(httpTask.PostTask)

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

	return httpServer, coordinatorService, workerService
}
