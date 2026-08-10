//go:build !windows

package converter

import (
	"context"
	"os/exec"
	"syscall"
)

// runOpenOfficeCmd starts soffice in its own process group and waits.
// On ctx cancel it sends SIGKILL to the process group (-pid).
func runOpenOfficeCmd(ctx context.Context, cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		<-done
		return ctx.Err()
	case err := <-done:
		return err
	}
}
