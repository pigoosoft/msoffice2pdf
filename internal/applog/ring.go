package applog

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	RingCapDefault = 1000
	RingCapMin     = 100
	RingCapMax     = 5000
)

type Entry struct {
	Time   time.Time
	Level  slog.Level
	UID    string
	Action string
	Text   string
}

type Ring struct {
	mu    sync.Mutex
	cap   int
	buf   []Entry
	start int
	len   int
}

func NewRing(capacity int) *Ring {
	r := &Ring{}
	r.SetCapacity(capacity)
	return r
}

func clampCap(n int) int {
	if n <= 0 {
		return RingCapDefault
	}
	if n > RingCapMax {
		return RingCapMax
	}
	return n
}

func (r *Ring) Capacity() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cap
}

func (r *Ring) SetCapacity(n int) {
	n = clampCap(n)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.buf == nil {
		r.cap = n
		r.buf = make([]Entry, n)
		r.start, r.len = 0, 0
		return
	}
	all := r.snapshotLocked(slog.LevelDebug, "", "")
	if len(all) > n {
		all = all[len(all)-n:]
	}
	r.cap = n
	r.buf = make([]Entry, n)
	r.start, r.len = 0, 0
	for _, e := range all {
		r.pushLocked(e)
	}
}

func (r *Ring) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.start, r.len = 0, 0
}

func (r *Ring) pushLocked(e Entry) {
	if r.len < r.cap {
		idx := (r.start + r.len) % r.cap
		r.buf[idx] = e
		r.len++
		return
	}
	r.buf[r.start] = e
	r.start = (r.start + 1) % r.cap
}

func (r *Ring) Handler(opts *slog.HandlerOptions) slog.Handler {
	h := &ringHandler{ring: r}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelDebug // capture all; UI filters later
	}
	return h
}

type ringHandler struct {
	ring *Ring
	opts slog.HandlerOptions
	goas []groupOrAttrs
}

func (h *ringHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *ringHandler) Handle(ctx context.Context, r slog.Record) error {
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
	for _, g := range h.goas {
		for _, a := range g.attrs {
			collect(a)
		}
	}
	r.Attrs(func(a slog.Attr) bool {
		collect(a)
		return true
	})
	text := fmt.Sprintf("%s %s %s %s %s",
		r.Time.Local().Format("2006-01-02 15:04:05.000"),
		uid,
		strings.ToUpper(r.Level.String()),
		action,
		strings.TrimSpace(strings.Join(rest, " ")),
	)
	h.ring.mu.Lock()
	h.ring.pushLocked(Entry{Time: r.Time, Level: r.Level, UID: uid, Action: action, Text: text})
	h.ring.mu.Unlock()
	return nil
}

func (h *ringHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	h2.goas = append(append([]groupOrAttrs(nil), h.goas...), groupOrAttrs{attrs: attrs})
	return &h2
}

func (h *ringHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := *h
	h2.goas = append(append([]groupOrAttrs(nil), h.goas...), groupOrAttrs{group: name})
	return &h2
}

func (r *Ring) Snapshot(minLevel slog.Level, uidSub, actionSub string) []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked(minLevel, uidSub, actionSub)
}

func (r *Ring) snapshotLocked(minLevel slog.Level, uidSub, actionSub string) []Entry {
	out := make([]Entry, 0, r.len)
	for i := 0; i < r.len; i++ {
		e := r.buf[(r.start+i)%r.cap]
		if e.Level < minLevel {
			continue
		}
		if uidSub != "" && !strings.Contains(e.UID, uidSub) {
			continue
		}
		if actionSub != "" && !strings.Contains(e.Action, actionSub) {
			continue
		}
		out = append(out, e)
	}
	return out
}
