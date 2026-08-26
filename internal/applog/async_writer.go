package applog

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

const (
	asyncLogQueue      = 8192
	asyncBacklogWarnAt = 16384
)

var (
	trackMu sync.Mutex
	tracked []*asyncWriter
)

func resetTracked() {
	trackMu.Lock()
	tracked = nil
	trackMu.Unlock()
}

func registerAsync(a *asyncWriter) {
	trackMu.Lock()
	tracked = append(tracked, a)
	trackMu.Unlock()
}

// BacklogBytes is unflushed log payload still in RAM (all async writers).
func BacklogBytes() int64 {
	trackMu.Lock()
	defer trackMu.Unlock()
	var n int64
	for _, w := range tracked {
		n += w.queued.Load()
	}
	return n
}

// asyncWriter copies writes onto an unbounded in-memory queue so callers
// do not wait on console or disk, and no lines are dropped.
// After each line is written to the sink, that buffer is released for GC.
type asyncWriter struct {
	w      io.Writer
	mu     sync.Mutex
	cond   *sync.Cond
	q      [][]byte
	queued atomic.Int64
	done   chan struct{}
	closed bool
	warned bool
}

func newAsyncWriter(w io.Writer, queue int) *asyncWriter {
	if queue < 1 {
		queue = asyncLogQueue
	}
	a := &asyncWriter{
		w:    w,
		q:    make([][]byte, 0, queue),
		done: make(chan struct{}),
	}
	a.cond = sync.NewCond(&a.mu)
	registerAsync(a)
	go a.loop()
	return a
}

func (a *asyncWriter) Write(p []byte) (int, error) {
	cp := append([]byte(nil), p...)
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return 0, fmt.Errorf("applog: async writer closed")
	}
	a.q = append(a.q, cp)
	a.queued.Add(int64(len(cp)))
	if !a.warned && len(a.q) >= asyncBacklogWarnAt {
		a.warned = true
		n := len(a.q)
		a.cond.Signal()
		a.mu.Unlock()
		_, _ = fmt.Fprintf(os.Stderr, "applog: log backlog %d lines (disk/console slow; converting continues, logs delayed not dropped)\n", n)
		return len(p), nil
	}
	a.cond.Signal()
	a.mu.Unlock()
	return len(p), nil
}

func (a *asyncWriter) loop() {
	defer close(a.done)
	for {
		a.mu.Lock()
		for len(a.q) == 0 && !a.closed {
			a.cond.Wait()
		}
		if len(a.q) == 0 && a.closed {
			a.mu.Unlock()
			return
		}
		batch := a.q
		a.q = nil
		if a.warned && len(batch) < asyncBacklogWarnAt/4 {
			a.warned = false
		}
		a.mu.Unlock()
		for i, p := range batch {
			_, _ = a.w.Write(p)
			a.queued.Add(-int64(len(p)))
			batch[i] = nil
		}
	}
}

func (a *asyncWriter) Close() error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	a.cond.Broadcast()
	a.mu.Unlock()
	<-a.done
	return nil
}
