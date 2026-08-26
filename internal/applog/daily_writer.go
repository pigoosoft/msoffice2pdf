package applog

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DailyBufferedWriter appends to {dir}/yyyymmdd{nameSuffix}.log with buffering,
// optional periodic Sync, and calendar-day rotation (local time).
// nameSuffix "" → yyyymmdd.log; "_detail" → yyyymmdd_detail.log.
type DailyBufferedWriter struct {
	dir           string
	nameSuffix    string
	flushInterval time.Duration

	mu     sync.Mutex
	day    string // "20060102"
	file   *os.File
	buf    *bufio.Writer
	ticker *time.Ticker
	done   chan struct{}
	closed bool
}

func NewDailyBufferedWriter(dir string, flushInterval time.Duration) (*DailyBufferedWriter, error) {
	return newDailyWriter(dir, flushInterval, "")
}

func newDailyWriter(dir string, flushInterval time.Duration, nameSuffix string) (*DailyBufferedWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir log dir: %w", err)
	}
	w := &DailyBufferedWriter{
		dir:           dir,
		nameSuffix:    nameSuffix,
		flushInterval: flushInterval,
		done:          make(chan struct{}),
	}
	if err := w.rotateLocked(time.Now()); err != nil {
		return nil, err
	}
	if flushInterval > 0 {
		w.ticker = time.NewTicker(flushInterval)
		go w.flushLoop()
	}
	return w, nil
}

func (w *DailyBufferedWriter) flushLoop() {
	for {
		select {
		case <-w.ticker.C:
			_ = w.Sync()
		case <-w.done:
			return
		}
	}
}

func (w *DailyBufferedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, fmt.Errorf("applog: writer closed")
	}
	now := time.Now()
	if now.Format("20060102") != w.day {
		if err := w.rotateLocked(now); err != nil {
			return 0, err
		}
	}
	return w.buf.Write(p)
}

func (w *DailyBufferedWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	return w.syncLocked()
}

func (w *DailyBufferedWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	if w.ticker != nil {
		w.ticker.Stop()
	}
	select {
	case <-w.done:
	default:
		close(w.done)
	}
	if err := w.syncLocked(); err != nil {
		if w.file != nil {
			_ = w.file.Close()
		}
		return err
	}
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	w.buf = nil
	return err
}

func (w *DailyBufferedWriter) rotateLocked(now time.Time) error {
	if err := w.syncLocked(); err != nil {
		return err
	}
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
		w.buf = nil
	}
	day := now.Format("20060102")
	path := filepath.Join(w.dir, day+w.nameSuffix+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	w.file = f
	w.buf = bufio.NewWriterSize(f, 32*1024)
	w.day = day
	return nil
}

func (w *DailyBufferedWriter) syncLocked() error {
	if w.buf == nil {
		return nil
	}
	if err := w.buf.Flush(); err != nil {
		return err
	}
	return w.file.Sync()
}
