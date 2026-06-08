package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"golang.org/x/term"
)

const (
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiReset  = "\033[0m"
)

type CliHandler struct {
	w    io.Writer
	opts slog.HandlerOptions
}

func (h *CliHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return true
}

func (h *CliHandler) Handle(ctx context.Context, rec slog.Record) error {
	prefix, suffix := "", ""
	if isTerminal(h.w) {
		switch rec.Level {
		case slog.LevelError:
			prefix, suffix = ansiRed, ansiReset
		case slog.LevelWarn:
			prefix, suffix = ansiYellow, ansiReset
		}
	}
	if rec.NumAttrs() > 0 {
		var attrs []string
		rec.Attrs(func(a slog.Attr) bool {
			attrs = append(attrs, a.String())
			return true
		})
		_, err := fmt.Fprintf(
			h.w,
			"%s%s (%s)%s\n",
			prefix, rec.Message, strings.Join(attrs, ", "), suffix,
		)
		return err
	}
	_, err := fmt.Fprintf(h.w, "%s%s%s\n", prefix, rec.Message, suffix)
	return err
}

func (h *CliHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *CliHandler) WithGroup(name string) slog.Handler {
	return h
}

func NewCliHandler(w io.Writer, opts *slog.HandlerOptions) *CliHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &CliHandler{
		w:    w,
		opts: *opts,
	}
}

func isTerminal(w io.Writer) bool {
	if f, ok := w.(interface{ Fd() uintptr }); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}
