package applog

import (
	"bytes"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type slowWriter struct {
	delay time.Duration
	n     atomic.Int64
}

func (s *slowWriter) Write(p []byte) (int, error) {
	time.Sleep(s.delay)
	s.n.Add(int64(len(p)))
	return len(p), nil
}

func TestAsyncWriterDoesNotBlockOnSlowSink(t *testing.T) {
	sink := &slowWriter{delay: 80 * time.Millisecond}
	w := newAsyncWriter(sink, 64)
	t.Cleanup(func() { _ = w.Close() })

	start := time.Now()
	const writes = 20
	for i := 0; i < writes; i++ {
		if _, err := w.Write([]byte("x\n")); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	if elapsed > 50*time.Millisecond {
		t.Fatalf("Write blocked on slow sink: %v", elapsed)
	}
}

func TestAsyncWriterDoesNotDropWhenQueueSaturated(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	sink := writerFunc(func(p []byte) (int, error) {
		time.Sleep(2 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	})
	w := newAsyncWriter(sink, 4)
	const n = 80
	start := time.Now()
	for i := 0; i < n; i++ {
		line := []byte{byte('A' + i%26), '\n'}
		if _, err := w.Write(line); err != nil {
			t.Fatal(err)
		}
	}
	if time.Since(start) > 80*time.Millisecond {
		t.Fatalf("Write blocked callers: %v", time.Since(start))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	got := buf.Len()
	mu.Unlock()
	if got != n*2 {
		t.Fatalf("dropped logs: got %d bytes want %d", got, n*2)
	}
}

func TestAsyncWriterCloseFlushes(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	sink := writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	})
	w := newAsyncWriter(sink, 8)
	if _, err := w.Write([]byte("hello-async\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	got := buf.String()
	mu.Unlock()
	if got != "hello-async\n" {
		t.Fatalf("got %q", got)
	}
}

func TestAsyncWriterBacklogClearsAfterFlush(t *testing.T) {
	resetTracked()
	var buf bytes.Buffer
	var mu sync.Mutex
	sink := writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	})
	w := newAsyncWriter(sink, 8)
	if _, err := w.Write([]byte("abc\n")); err != nil {
		t.Fatal(err)
	}
	if BacklogBytes() != 4 {
		t.Fatalf("queued: %d", BacklogBytes())
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if BacklogBytes() != 0 {
		t.Fatalf("after flush want 0, got %d", BacklogBytes())
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

var _ io.Writer = writerFunc(nil)
