package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/thavel/apidep/pkg/file"
)

var Validate = &cli.Command{
	Name:  "validate",
	Usage: "Validate api refs files described in api.ref.yml",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "ref",
			Aliases: []string{"r"},
			Value:   defaultApiref,
			Usage:   "path to the api.ref.yml file",
		},
	},
	Action: validateAction,
}

func validateAction(ctx context.Context, cmd *cli.Command) error {
	refPath := cmd.String("ref")

	if _, err := os.Stat(refPath); err != nil {
		return fmt.Errorf("no ref file found")
	}
	apiRef, err := file.ReadRef(refPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", refPath, err)
	}

	if len(apiRef.Refs) == 0 {
		slog.Info("no refs found", "file", refPath)
		return nil
	}
	return validateRefs(".", apiRef.Refs)
}

func validateRefs(baseDir string, refs []file.Ref) error {
	var firstErr error

	for _, ref := range refs {
		path := filepath.Join(baseDir, ref.Path)

		if err := file.Validate(path, ref.Type); err != nil {
			slog.Error("invalid file", "path", ref.Path, "type", ref.Type, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		slog.Info("valid", "file", ref.Path, "type", ref.Type)
	}

	if firstErr == nil {
		slog.Info("all files are valid")
	}
	return firstErr
}
