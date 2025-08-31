package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/w-h-a/workflow/cmd"
)

func main() {
	app := &cli.App{
		Name:  "workflow",
		Usage: "workflow clients/engine",
		Commands: []*cli.Command{
			{
				Name: "engine",
				Action: func(ctx *cli.Context) error {
					return cmd.StartEngine(ctx)
				},
			},
			{
				Name: "container-logs",
				Action: func(ctx *cli.Context) error {
					return cmd.StreamContainerLogs(ctx)
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "task_id",
						Usage:    "Provide the task id for the container logs to stream",
						Required: true,
					},
				},
			},
			{
				Name: "migration",
				Action: func(ctx *cli.Context) error {
					return cmd.RunMigrations(ctx)
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "location",
						Usage:    "Provide the db location",
						Required: true,
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		slog.ErrorContext(context.Background(), "failed", "error", err)
		os.Exit(1)
	}
}
