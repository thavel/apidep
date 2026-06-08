package cmd

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/thavel/apidep/pkg/file"
)

var CI = &cli.Command{
	Name:  "ci",
	Usage: "Check that api deps described in api.dep.yml are up-to-date and valid",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "file",
			Aliases: []string{"f"},
			Value:   defaultApidep,
			Usage:   "path to the api.dep.yml file",
		},
		&cli.BoolFlag{
			Name:  "no-validate",
			Value: false,
			Usage: "disable api file type validation",
		},
		&cli.IntFlag{
			Name:    "concurrency",
			Aliases: []string{"c"},
			Value:   3,
			Usage:   "concurrent resolve tasks",
		},
		&cli.StringFlag{
			Name:    "lock",
			Aliases: []string{"l"},
			Value:   defaultLockFile,
			Usage:   "path to the lock file",
		},
	},
	Action: ciAction,
}

func ciAction(ctx context.Context, cmd *cli.Command) error {
	depPath := cmd.String("file")
	lockPath := cmd.String("lock")
	noValidate := cmd.Bool("no-validate")
	concurrency := cmd.Int("concurrency")

	apiDep, err := file.ReadDep(depPath)
	if err != nil {
		return err
	}

	deps, err := resolve(apiDep, concurrency)
	if err != nil {
		return err
	}
	rootOutput := apiDep.Output

	lf, err := file.ReadLock(lockPath)
	if err != nil {
		return fmt.Errorf("load lock file: %w", err)
	}

	for _, rd := range deps {
		if err := checkDep(rd, rootOutput, lf, noValidate); err != nil {
			return err
		}
	}

	slog.Info("ci: all dependency files up-to-date and valid")
	return nil
}

func checkDep(rd ResolvedDep, rootOutput string, lock *file.ApiLock, noValidate bool) error {
	dep := &rd.Dep
	source := rd.Source

	remoteContents := make(map[string][]byte, len(dep.Refs))
	for _, ref := range dep.Refs {
		remoteContent, err := source.Fetch(ref.Path)
		if err != nil {
			return fmt.Errorf("error fetching %s: %w", ref.Path, err)
		}
		remoteContents[ref.Path] = remoteContent

		dest := file.Output(ref.Path, rootOutput, dep.Output, ref.Output, defaultOutput)

		localContent, err := os.ReadFile(dest)
		if err != nil {
			return fmt.Errorf("%s: missing", dest)
		}

		if !bytes.Equal(localContent, remoteContent) {
			return fmt.Errorf("%s: content differs from remote", dest)
		}

		if !noValidate {
			if err := file.Validate(dest, ref.Type); err != nil {
				return fmt.Errorf("%s: invalid: %w", dest, err)
			}
		}
	}

	// Verify lock file consistency
	expectedHash := file.Hash(remoteContents)
	entry, ok := lock.Get(dep.Source)
	if !ok {
		return fmt.Errorf("%s: missing from lock file", dep.Source)
	}
	if entry.Hash != expectedHash {
		return fmt.Errorf("%s: lock hash mismatch", dep.Source)
	}
	if commit := source.Commit(); entry.Commit != commit {
		return fmt.Errorf(
			"%s: commit changed (locked=%s current=%s)",
			dep.Source, entry.Commit[:8], commit[:8],
		)
	}

	return nil
}
