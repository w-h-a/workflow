package http

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/w-h-a/pkg/serverv2"
	httpserver "github.com/w-h-a/pkg/serverv2/http"
	"github.com/w-h-a/workflow/internal/engine/config"
	"github.com/w-h-a/workflow/internal/engine/services/coordinator"
	"github.com/w-h-a/workflow/internal/engine/services/streamer"
	"github.com/w-h-a/workflow/internal/engine/services/worker"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

func NewCoordinatorServer(
	coordinatorService *coordinator.Service,
) serverv2.Server {
	// base server options
	opts := []serverv2.ServerOption{
		serverv2.ServerWithNamespace(config.Env()),
		serverv2.ServerWithName(config.Name()),
		serverv2.ServerWithVersion(config.Version()),
	}

	// create http router
	router := mux.NewRouter()

	httpStatus := NewStatusHandler(coordinatorService)
	router.Methods(http.MethodGet).Path("/status").HandlerFunc(httpStatus.GetStatus)

	httpTasks := NewTasksHandler(coordinatorService)
	router.Methods(http.MethodGet).Path("/tasks").HandlerFunc(httpTasks.GetTasks)
	router.Methods(http.MethodGet).Path("/tasks/{id}").HandlerFunc(httpTasks.GetOneTask)
	router.Methods(http.MethodPut).Path("/tasks/cancel/{id}").HandlerFunc(httpTasks.PutCancelTask)
	router.Methods(http.MethodPut).Path("/tasks/restart/{id}").HandlerFunc(httpTasks.PutRestartTask)
	router.Methods(http.MethodPost).Path("/tasks").HandlerFunc(httpTasks.PostTask)

	// create http server
	httpOpts := []serverv2.ServerOption{
		serverv2.ServerWithAddress(config.CoordinatorHttp()),
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

	return httpServer
}

func NewWorkerServer(
	workerService *worker.Service,
) serverv2.Server {
	// base server options
	opts := []serverv2.ServerOption{
		serverv2.ServerWithNamespace(config.Env()),
		serverv2.ServerWithName(config.Name()),
		serverv2.ServerWithVersion(config.Version()),
	}

	// create http router
	router := mux.NewRouter()

	httpStatus := NewStatusHandler(workerService)
	router.Methods(http.MethodGet).Path("/status").HandlerFunc(httpStatus.GetStatus)

	// create http server
	httpOpts := []serverv2.ServerOption{
		serverv2.ServerWithAddress(config.WorkerHttp()),
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

	return httpServer
}

func NewStreamerServer(
	streamerService *streamer.Service,
) serverv2.Server {
	// base server options
	opts := []serverv2.ServerOption{
		serverv2.ServerWithNamespace(config.Env()),
		serverv2.ServerWithName(config.Name()),
		serverv2.ServerWithVersion(config.Version()),
	}

	// create http router
	router := mux.NewRouter()

	httpStatus := NewStatusHandler(streamerService)
	router.Methods(http.MethodGet).Path("/status").HandlerFunc(httpStatus.GetStatus)

	httpLogs := NewLogsHandler(streamerService)
	router.Methods(http.MethodGet).Path("/logs/{id}").HandlerFunc(httpLogs.StreamLogs)

	// create http server
	httpOpts := []serverv2.ServerOption{
		serverv2.ServerWithAddress(config.StreamerHttp()),
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

	return httpServer
}
