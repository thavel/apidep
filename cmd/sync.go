package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/urfave/cli/v3"

	"github.com/thavel/apidep/pkg/file"
	"github.com/thavel/apidep/pkg/provider"
)

const (
	defaultApidep   = "api.dep.yml"
	defaultApiref   = "api.ref.yml"
	defaultLockFile = "api.lock.yml"
	defaultOutput   = "apidep/"
	defaultVersion  = "HEAD"
)

type ResolvedDep struct {
	Dep    file.Dep
	Source file.Source
}

var (
	mux       sync.Mutex
	providers = []file.Provider{
		&provider.FS{},
		&provider.Git{},
	}
	Sync = &cli.Command{
		Name:  "sync",
		Usage: "Synchronize api dependencies",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "file",
				Aliases: []string{"f"},
				Value:   defaultApidep,
				Usage:   "path to the local apidep file",
			},
			&cli.IntFlag{
				Name:    "concurrency",
				Aliases: []string{"c"},
				Value:   3,
				Usage:   "concurrent sync tasks",
			},
			&cli.BoolFlag{
				Name:  "no-validate",
				Value: false,
				Usage: "disable api file validation after sync",
			},
			&cli.StringFlag{
				Name:    "lock",
				Aliases: []string{"l"},
				Value:   defaultLockFile,
				Usage:   "path to the lock file",
			},
		},
		Action: syncAction,
	}
)

func syncAction(ctx context.Context, cmd *cli.Command) error {
	depPath := cmd.String("file")
	lockPath := cmd.String("lock")
	concurrency := cmd.Int("concurrency")
	noValidate := cmd.Bool("no-validate")

	apiDep, err := file.ReadDep(depPath)
	if err != nil {
		return err
	}

	deps, err := resolve(apiDep, concurrency)
	if err != nil {
		return err
	}
	rootOutput := apiDep.Output

	var (
		wg       sync.WaitGroup
		lockDeps []file.DepLock
		lockMux  sync.Mutex
	)
	sem := make(chan struct{}, concurrency)
	for _, rd := range deps {
		wg.Add(1)
		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			slog.Info("fetching", "source", rd.Dep.Source)

			depLock, err := syncDep(rd, rootOutput, noValidate)
			if err != nil {
				slog.Error("syncing", "source", rd.Dep.Source, "err", err)
				return
			}
			lockMux.Lock()
			defer lockMux.Unlock()
			lockDeps = append(lockDeps, *depLock)
		}()
	}
	wg.Wait()

	if len(lockDeps) > 0 {
		lock, err := file.ReadLock(lockPath)
		if err != nil {
			return fmt.Errorf("load lock file: %w", err)
		}
		for _, entry := range lockDeps {
			lock.Upsert(entry)
		}
		if err := file.WriteLock(lockPath, lock); err != nil {
			return fmt.Errorf("save lock file: %w", err)
		}
		slog.Info("lock file updated", "path", lockPath)
	}

	return nil
}

func syncDep(rd ResolvedDep, root string, noValidate bool) (*file.DepLock, error) {
	dep := &rd.Dep
	source := rd.Source

	contents := make(map[string][]byte, len(dep.Refs))
	for _, ref := range dep.Refs {
		content, err := source.Fetch(ref.Path)
		if err != nil {
			return nil, fmt.Errorf("error fetching %s: %w", ref.Path, err)
		}
		dest := file.Output(ref.Path, root, dep.Output, ref.Output, defaultOutput)
		status, err := file.WriteDep(&mux, dest, content)
		if err != nil {
			return nil, fmt.Errorf("error writing output: %w", err)
		}
		if status != file.FileUnchanged && !noValidate {
			if err := file.Validate(dest, ref.Type); err != nil {
				slog.Warn("validating", "path", dest, "err", err)
			}
		}
		switch status {
		case file.FileNew:
			slog.Info("new", "file", dest)
		case file.FileUnchanged:
			slog.Info("unchanged", "file", dest)
		case file.FileUpdated:
			slog.Info("updated", "file", dest)
		}
		contents[ref.Path] = content
	}

	return &file.DepLock{
		Source: dep.Source,
		Hash:   file.Hash(contents),
		Commit: source.Commit(),
	}, nil
}

func resolve(apiDep *file.ApiDep, concurrency int) ([]ResolvedDep, error) {
	resolved := make([]struct {
		dep ResolvedDep
		err error
	}, len(apiDep.Deps))

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for i, dep := range apiDep.Deps {
		wg.Add(1)
		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			if len(dep.Refs) > 0 && len(dep.Ref) > 0 {
				resolved[i].err = fmt.Errorf("%s: refs and ref are mutually exclusive", dep.Source)
				return
			}

			var src file.Source
			var err error
			for _, p := range providers {
				if p.Match(dep.Source) {
					src, err = p.Parse(dep.Source, dep.Version)
					break
				}
			}
			if src == nil && err == nil {
				err = fmt.Errorf("unsupported source %q", dep.Source)
			}
			if err != nil {
				resolved[i].err = fmt.Errorf("%s: resolve source: %w", dep.Source, err)
				return
			}

			refs := dep.Refs
			if len(refs) == 0 {
				refPath := defaultApiref
				if len(dep.Ref) > 0 {
					refPath = dep.Ref
				}
				refContent, err := src.Fetch(refPath)
				if err != nil {
					resolved[i].err = fmt.Errorf("%s: fetch %s: %w", dep.Source, refPath, err)
					return
				}
				refFile, err := file.ParseRef(refPath, refContent)
				if err != nil {
					resolved[i].err = fmt.Errorf("%s: parse %s: %w", dep.Source, refPath, err)
					return
				}
				refs = refFile.Refs
			}

			dep.Refs = refs
			dep.Ref = ""
			resolved[i].dep = ResolvedDep{Dep: dep, Source: src}
		}()
	}
	wg.Wait()

	res := make([]ResolvedDep, 0, len(apiDep.Deps))
	for _, r := range resolved {
		if r.err != nil {
			return nil, r.err
		}
		res = append(res, r.dep)
	}
	return res, nil
}
