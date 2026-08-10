//go:build windows

package converter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func newSubprocessConverter(opts Options) Converter {
	return &subprocessConverter{
		excelPageFit: opts.ExcelPageFit,
		tempSandbox:  opts.TempSandbox,
		extEngines:   opts.ExtEngines,
		extAppKinds:  opts.ExtAppKinds,
	}
}

type subprocessConverter struct {
	excelPageFit string
	tempSandbox  bool
	extEngines   map[string]string
	extAppKinds  map[string]AppKind
}

func (c *subprocessConverter) Convert(ctx context.Context, srcPath, dstPath string) error {
	srcPath, err := filepath.Abs(srcPath)
	if err != nil {
		return fmt.Errorf("converter: abs src: %w", err)
	}
	dstPath, err = filepath.Abs(dstPath)
	if err != nil {
		return fmt.Errorf("converter: abs dst: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("converter: mkdir dst: %w", err)
	}
	_ = os.Remove(dstPath)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("converter: executable path: %w", err)
	}

	fit := c.excelPageFit
	if fit == "" {
		fit = "fit_width"
	}

	bare := strings.ToLower(strings.TrimPrefix(filepath.Ext(srcPath), "."))
	engine, ok := c.extEngines[bare]
	if !ok || engine == "" {
		return fmt.Errorf("converter: no engine mapped for .%s", bare)
	}
	kind, ok := c.extAppKinds[bare]
	if !ok {
		return fmt.Errorf("converter: no app kind for .%s (from validate_*)", bare)
	}

	cmd := exec.Command(exe, "convert-worker",
		"--src", srcPath,
		"--dst", dstPath,
		"--excel-page-fit", fit,
		"--engine", engine,
		"--app-kind", string(kind),
	)
	var stderr bytes.Buffer
	cmd.Stdout = nil
	cmd.Stderr = &stderr

	if c.tempSandbox {
		dir, err := createTempSandboxDir()
		if err != nil {
			return err
		}
		defer removeTempSandbox(dir)
		cmd.Env = sandboxEnv(dir)
	}

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
		msg := strings.TrimSpace(stderr.String())
		if err == nil {
			if _, statErr := os.Stat(dstPath); statErr != nil {
				return fmt.Errorf("converter: subprocess ok but dst missing: %w", statErr)
			}
			return nil
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if msg != "" {
				return fmt.Errorf("converter: %s", msg)
			}
			if ee.ExitCode() == 2 {
				return fmt.Errorf("converter: convert-worker failed")
			}
			return fmt.Errorf("converter: subprocess crashed (exit %d)", ee.ExitCode())
		}
		if msg != "" {
			return fmt.Errorf("converter: %s", msg)
		}
		return fmt.Errorf("converter: subprocess: %w", err)
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
