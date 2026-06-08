package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/thavel/apidep/cmd"
	"github.com/thavel/apidep/pkg/logger"
)

func main() {
	slog.SetDefault(slog.New(
		logger.NewCliHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
	))

	app := &cli.Command{
		Name:  "apidep",
		Usage: "API dependency manager",
		Commands: []*cli.Command{
			cmd.Init,
			cmd.Sync,
			cmd.Validate,
			cmd.CI,
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		slog.Error("apidep error", "err", err)
		os.Exit(1)
	}
}
