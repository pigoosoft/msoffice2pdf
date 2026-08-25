//go:build !windows

package converter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if baseEnv == nil {
		baseEnv = os.Environ()
	}
	cmd.Env = PasswordEnv(baseEnv, password)

	dstPath := destPathFromArgs(args)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("converter: start convert-worker: %w", err)
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
		return fmt.Errorf("converter: %w", ctx.Err())
	case err := <-done:
		return mapWorkerResult(err, stderr.String(), dstPath)
	}
}
