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
		},
	}

	if err := app.Run(os.Args); err != nil {
		slog.ErrorContext(context.Background(), "failed", "error", err)
		os.Exit(1)
	}
}
