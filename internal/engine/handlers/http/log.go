package http

import (
	"net/http"

	"github.com/w-h-a/workflow/internal/engine/services/streamer"
)

type Logs struct {
	parser   *Parser
	streamer *streamer.Service
}

func (l *Logs) StreamLogs(w http.ResponseWriter, r *http.Request) {
	ctx := reqToCtx(r)

	taskId, err := l.parser.ParseTaskId(ctx, r)
	if err != nil {
		wrtRsp(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	logStream, err := l.streamer.StreamLogs(ctx, taskId)
	if err != nil {
		http.Error(w, "failed to stream logs", http.StatusInternalServerError)
		return
	}

	defer close(logStream.Stop)

	for {
		select {
		case <-ctx.Done():
			return
		case logLine, ok := <-logStream.Logs:
			if !ok {
				return
			}

			data := []byte("data: " + logLine + "\n\n")
			if _, err := w.Write(data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func NewLogsHandler(streamerService *streamer.Service) *Logs {
	return &Logs{
		parser:   &Parser{},
		streamer: streamerService,
	}
}
