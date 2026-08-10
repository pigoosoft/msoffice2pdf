package applog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

// FileTextHandler writes one line per record:
// {datetime} {uid|System} {LEVEL} {action} {msg attrs...}
type FileTextHandler struct {
	w    io.Writer
	opts slog.HandlerOptions
	mu   sync.Mutex
	goas []groupOrAttrs
}

type groupOrAttrs struct {
	group string
	attrs []slog.Attr
}

func NewFileTextHandler(w io.Writer, opts *slog.HandlerOptions) *FileTextHandler {
	h := &FileTextHandler{w: w}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo
	}
	return h
}

func (h *FileTextHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *FileTextHandler) Handle(ctx context.Context, r slog.Record) error {
	uid := "System"
	if v, ok := UIDFromContext(ctx); ok {
		uid = v
	}
	action := "-"
	var rest []string
	if r.Message != "" {
		rest = append(rest, r.Message)
	}

	collect := func(a slog.Attr) {
		a.Value = a.Value.Resolve()
		if a.Equal(slog.Attr{}) {
			return
		}
		switch a.Key {
		case "uid":
			if s := strings.TrimSpace(a.Value.String()); s != "" {
				uid = s
			}
			return
		case "action":
			if s := a.Value.String(); s != "" {
				action = s
			}
			return
		case slog.TimeKey, slog.LevelKey, slog.MessageKey, slog.SourceKey:
			return
		}
		rest = append(rest, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
	}

	h.mu.Lock()
	goas := append([]groupOrAttrs(nil), h.goas...)
	h.mu.Unlock()
	for _, g := range goas {
		for _, a := range g.attrs {
			collect(a)
		}
	}
	r.Attrs(func(a slog.Attr) bool {
		collect(a)
		return true
	})

	level := strings.ToUpper(r.Level.String())
	line := fmt.Sprintf("%s %s %s %s %s\n",
		r.Time.Local().Format("2006-01-02 15:04:05.000"),
		uid,
		level,
		action,
		strings.TrimSpace(strings.Join(rest, " ")),
	)

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, line)
	return err
}

func (h *FileTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	h2.goas = append(append([]groupOrAttrs(nil), h.goas...), groupOrAttrs{attrs: attrs})
	return &h2
}

func (h *FileTextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := *h
	h2.goas = append(append([]groupOrAttrs(nil), h.goas...), groupOrAttrs{group: name})
	return &h2
}
