//go:build windows

package converter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

func runConvertWorker(ctx context.Context, args []string, password string, baseEnv []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("converter: executable path: %w", err)
	}

	cmd := exec.Command(exe, args...)
	var stderr bytes.Buffer
	cmd.Stdout = nil
	cmd.Stderr = &stderr

	if baseEnv == nil {
		baseEnv = os.Environ()
	}
	cmd.Env = PasswordEnv(baseEnv, password)

	dstPath := destPathFromArgs(args)

	// Pattern A: close job handle exactly once (no defer) so timeout kill
	// via CloseHandle does not double-close.
	job, jobErr := createKillOnCloseJob()
	if jobErr != nil {
		job = 0
	}

	if err := cmd.Start(); err != nil {
		if job != 0 {
			_ = windows.CloseHandle(job)
			job = 0
		}
		return fmt.Errorf("converter: start convert-worker: %w", err)
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
		return fmt.Errorf("converter: %w", ctx.Err())
	case err := <-done:
		if job != 0 {
			_ = windows.CloseHandle(job)
			job = 0
		}
		return mapWorkerResult(err, stderr.String(), dstPath)
	}
}

func createKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func assignPidToJob(job windows.Handle, pid uint32) error {
	h, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		pid,
	)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.AssignProcessToJobObject(job, h)
}
