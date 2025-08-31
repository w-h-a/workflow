package cmd

import (
	"net/http"

	"github.com/urfave/cli/v2"
	containerlogs "github.com/w-h-a/workflow/internal/container_logs"
)

func StreamContainerLogs(ctx *cli.Context) error {
	taskID := ctx.String("task_id")

	containerlogs.RunStreamerClient(&http.Client{}, taskID)

	return nil
}
