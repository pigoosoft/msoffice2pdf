//go:build windows

package converter

import (
	"context"
	"os/exec"

	"golang.org/x/sys/windows"
)

// runOpenOfficeCmd starts soffice and waits, killing the process tree on ctx cancel
// via a kill-on-close Job Object (same pattern as subprocessConverter).
func runOpenOfficeCmd(ctx context.Context, cmd *exec.Cmd) error {
	job, jobErr := createKillOnCloseJob()
	if jobErr != nil {
		job = 0
	}

	if err := cmd.Start(); err != nil {
		if job != 0 {
			_ = windows.CloseHandle(job)
			job = 0
		}
		return err
	}

	assigned := false
	if job != 0 {
		if err := assignPidToJob(job, uint32(cmd.Process.Pid)); err != nil {
			_ = err
		} else {
			assigned = true
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		if job != 0 {
			_ = windows.CloseHandle(job)
			job = 0
		}
		if !assigned && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done
		return ctx.Err()
	case err := <-done:
		if job != 0 {
			_ = windows.CloseHandle(job)
			job = 0
		}
		return err
	}
}
