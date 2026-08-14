//go:build unix

package consoleattach

import (
	"os"
	"os/signal"
	"syscall"
)

func ensureCLI() {
	// Already attached to the launching terminal when run from a shell.
}

func detachForUI() {
	if !hasControllingTTY() {
		return
	}

	signal.Ignore(syscall.SIGHUP)

	// Leave the controlling terminal's session when possible.
	// Fails if already a session leader; SIGHUP ignore + stdio redirect still help.
	_, _ = syscall.Setsid()

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return
	}
	fd := int(devNull.Fd())
	_ = syscall.Dup2(fd, 0)
	_ = syscall.Dup2(fd, 1)
	_ = syscall.Dup2(fd, 2)
	os.Stdin = devNull
	os.Stdout = devNull
	os.Stderr = devNull
}

func hasControllingTTY() bool {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
