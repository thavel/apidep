package logger

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func record(level slog.Level, msg string, attrs ...slog.Attr) slog.Record {
	rec := slog.NewRecord(time.Now(), level, msg, 0)
	rec.AddAttrs(attrs...)
	return rec
}

func TestHandle_NoAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := NewCliHandler(&buf, nil)

	require.NoError(t, h.Handle(t.Context(), record(slog.LevelInfo, "hello")))
	assert.Equal(t, "hello\n", buf.String())
}

func TestHandle_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := NewCliHandler(&buf, nil)

	rec := record(slog.LevelInfo, "fetching",
		slog.String("source", "./remote"), slog.Int("count", 2))
	require.NoError(t, h.Handle(t.Context(), rec))
	assert.Equal(t, "fetching (source=./remote, count=2)\n", buf.String())
}

func TestHandle_NonTerminalHasNoColor(t *testing.T) {
	var buf bytes.Buffer
	h := NewCliHandler(&buf, nil)

	require.NoError(t, h.Handle(t.Context(), record(slog.LevelError, "boom")))
	// A bytes.Buffer is not a terminal, so no ANSI escape codes are emitted.
	assert.Equal(t, "boom\n", buf.String())
	assert.NotContains(t, buf.String(), "\033[")
}

func TestEnabled_AlwaysTrue(t *testing.T) {
	h := NewCliHandler(&bytes.Buffer{}, nil)
	assert.True(t, h.Enabled(t.Context(), slog.LevelDebug))
}

func TestIsTerminal_NonTerminal(t *testing.T) {
	assert.False(t, isTerminal(&bytes.Buffer{}))
}
