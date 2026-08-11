//go:build linux || darwin

package singleinstance

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Fixed path so Linux and macOS share one lock across users (avoid per-user TempDir
// on macOS, which would allow multiple "machine" instances).
const lockFileName = "msoffice2pdf-serve.lock"

type unixLock struct {
	f *os.File
}

// Acquire takes an exclusive non-blocking flock (Linux / macOS).
func Acquire() (Lock, error) {
	path := filepath.Join("/tmp", lockFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return nil, fmt.Errorf("singleinstance open lock file: %w", err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if err == unix.EWOULDBLOCK || err == unix.EAGAIN {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("singleinstance flock: %w", err)
	}
	return &unixLock{f: f}, nil
}

func (l *unixLock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	_ = unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
	err := l.f.Close()
	l.f = nil
	return err
}
