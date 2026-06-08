package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/urfave/cli/v3"

	"github.com/thavel/apidep/pkg/file"
)

var Init = &cli.Command{
	Name:  "init",
	Usage: "Generate an api.ref.yml by scanning api files",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Value:   defaultApiref,
			Usage:   "output file path",
		},
		&cli.StringFlag{
			Name:    "workdir",
			Aliases: []string{"w"},
			Value:   ".",
			Usage:   "workdir to scan",
		},
		&cli.IntFlag{
			Name:    "depth",
			Aliases: []string{"d"},
			Value:   0,
			Usage:   "maximum scan depth",
		},
	},
	Action: initAction,
}

func initAction(ctx context.Context, cmd *cli.Command) error {
	refPath := cmd.String("output")
	workdir := cmd.String("workdir")
	maxDepth := cmd.Int("depth")

	scanned, err := file.Scan(workdir, maxDepth)
	if err != nil {
		return fmt.Errorf("scan error: %w", err)
	}
	if len(scanned) == 0 {
		slog.Warn("no api definition files found")
		return nil
	}

	if err := file.WriteRef(refPath, &file.ApiRef{Refs: scanned}); err != nil {
		return fmt.Errorf("write %s: %w", refPath, err)
	}
	slog.Info("ref file created", "file", refPath, "refs", len(scanned))

	return nil
}
