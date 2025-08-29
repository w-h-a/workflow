package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/w-h-a/workflow/internal/engine/config"
)

type HealthChecker interface {
	CheckHealth(ctx context.Context) error
}

type Status struct {
	service HealthChecker
}

func (s *Status) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := reqToCtx(r)

	status := map[string]any{
		"env":     config.Env(),
		"name":    config.Name(),
		"version": config.Version(),
	}

	if err := s.service.CheckHealth(ctx); err != nil {
		slog.ErrorContext(ctx, "health check failed", "error", err)
		status["status"] = "DOWN"
		wrtRsp(w, http.StatusServiceUnavailable, status)
		return
	}

	status["status"] = "UP"
	wrtRsp(w, http.StatusOK, status)
}

func NewStatusHandler(service HealthChecker) *Status {
	return &Status{
		service: service,
	}
}
