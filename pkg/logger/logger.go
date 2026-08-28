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

// CliHandler is a slog.Handler that prints plain, human-readable lines and
// colorizes warnings and errors when writing to a terminal.
type CliHandler struct {
	w    io.Writer
	opts slog.HandlerOptions
}

// Enabled implements slog.Handler; every level is enabled.
func (h *CliHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return true
}

// Handle implements slog.Handler.
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

// WithAttrs implements slog.Handler; attributes are not retained.
func (h *CliHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

// WithGroup implements slog.Handler; groups are ignored.
func (h *CliHandler) WithGroup(name string) slog.Handler {
	return h
}

// NewCliHandler returns a CliHandler writing to w.
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
