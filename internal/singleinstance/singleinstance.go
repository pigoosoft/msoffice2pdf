// Package singleinstance enforces a single machine-wide serve process
// and rejects startup when the configured HTTP port is already bound.
package singleinstance

import (
	"errors"
	"fmt"
	"net"
	"time"
)

// ErrAlreadyRunning is returned when another serve instance holds the lock.
var ErrAlreadyRunning = errors.New("another msoffice2pdf serve instance is already running")

// Lock is held for the lifetime of the serve process.
type Lock interface {
	Release() error
}

// CheckPortFree reports whether something is already accepting TCP connections
// on the configured server port (localhost IPv4/IPv6). Plain Listen is not used
// on Windows because SO_REUSEADDR can succeed while another process still serves.
func CheckPortFree(port int) error {
	if port <= 0 {
		return fmt.Errorf("server port must be > 0")
	}
	addrs := []string{
		fmt.Sprintf("127.0.0.1:%d", port),
		fmt.Sprintf("[::1]:%d", port),
	}
	for _, addr := range addrs {
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return fmt.Errorf("server port %d is already in use", port)
		}
	}
	return nil
}
