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

var consoleSink io.Writer = os.Stdout

// ConsoleWriter returns stdout, plus the daily JSON detail file when file logging is enabled.
// Gin and GORM should write here so console detail is also captured in yyyymmdd_detail.log.
func ConsoleWriter() io.Writer {
	return consoleSink
}

// Init configures slog.Default. JSON goes to ConsoleWriter (stdout, and
// {file_dir}/yyyymmdd_detail.log when file logging is on) via an unbounded
// async queue so convert workers do not wait on console or disk and no lines
// are dropped. When cfg.FileLoggingEnabled(), also attaches FileTextHandler
// to {file_dir}/yyyymmdd.log (same async queue).
// Optional extra handlers (e.g. ring buffer) are appended to the fan-out.
// Caller must Close the returned closer on shutdown (even if nop).
func Init(cfg config.LogConfig, extra ...slog.Handler) (io.Closer, error) {
	resetTracked()
	lv := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{Level: lv}

	if !cfg.FileLoggingEnabled() {
		aw := newAsyncWriter(os.Stdout, asyncLogQueue)
		consoleSink = aw
		stdout := slog.NewJSONHandler(aw, opts)
		handlers := []slog.Handler{stdout}
		handlers = append(handlers, extra...)
		slog.SetDefault(slog.New(multiHandler(handlers)))
		return aw, nil
	}

	dir := strings.TrimSpace(cfg.FileDir)
	summaryW, err := newDailyWriter(dir, cfg.FlushInterval, "")
	if err != nil {
		return nil, fmt.Errorf("applog file init: %w", err)
	}
	detailW, err := newDailyWriter(dir, cfg.FlushInterval, "_detail")
	if err != nil {
		_ = summaryW.Close()
		return nil, fmt.Errorf("applog detail file init: %w", err)
	}
	asyncConsole := newAsyncWriter(io.MultiWriter(os.Stdout, detailW), asyncLogQueue)
	asyncSummary := newAsyncWriter(summaryW, asyncLogQueue)
	consoleSink = asyncConsole
	stdout := slog.NewJSONHandler(asyncConsole, opts)
	fileH := NewFileTextHandler(asyncSummary, opts)
	handlers := []slog.Handler{stdout, fileH}
	handlers = append(handlers, extra...)
	slog.SetDefault(slog.New(multiHandler(handlers)))
	// Close async queues first (drain), then the daily files they write to.
	return multiCloser{closers: []io.Closer{summaryW, detailW, asyncSummary, asyncConsole}}, nil
}
