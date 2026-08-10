package applog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"msoffice2pdf/internal/config"
)

type multiCloser struct {
	closers []io.Closer
}

func (m multiCloser) Close() error {
	var first error
	for i := len(m.closers) - 1; i >= 0; i-- {
		if err := m.closers[i].Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// multiHandler fans out records to all handlers (stdlib may omit NewMultiHandler).
type multiHandler []slog.Handler

func (h multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, hh := range h {
		if hh.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var first error
	for _, hh := range h {
		if !hh.Enabled(ctx, r.Level) {
			continue
		}
		if err := hh.Handle(ctx, r.Clone()); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make(multiHandler, len(h))
	for i, hh := range h {
		out[i] = hh.WithAttrs(attrs)
	}
	return out
}

func (h multiHandler) WithGroup(name string) slog.Handler {
	out := make(multiHandler, len(h))
	for i, hh := range h {
		out[i] = hh.WithGroup(name)
	}
	return out
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Init configures slog.Default. Always attaches JSON stdout.
// When cfg.FileLoggingEnabled(), also attaches FileTextHandler to a daily file.
// Caller must Close the returned closer on shutdown (even if nop).
func Init(cfg config.LogConfig) (io.Closer, error) {
	lv := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{Level: lv}
	stdout := slog.NewJSONHandler(os.Stdout, opts)

	if !cfg.FileLoggingEnabled() {
		slog.SetDefault(slog.New(stdout))
		return nopCloser{}, nil
	}

	dir := strings.TrimSpace(cfg.FileDir)
	dw, err := NewDailyBufferedWriter(dir, cfg.FlushInterval)
	if err != nil {
		return nil, fmt.Errorf("applog file init: %w", err)
	}
	fileH := NewFileTextHandler(dw, opts)
	slog.SetDefault(slog.New(multiHandler{stdout, fileH}))
	return multiCloser{closers: []io.Closer{dw}}, nil
}
