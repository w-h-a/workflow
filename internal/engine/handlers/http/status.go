package http

import (
	"net/http"

	"github.com/w-h-a/workflow/internal/engine/config"
)

type Status struct{}

func (s *Status) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]any{
		"env":     config.Env(),
		"name":    config.Name(),
		"version": config.Version(),
	}

	wrtRsp(w, http.StatusOK, status)
}

func NewStatusHandler() *Status {
	return &Status{}
}
