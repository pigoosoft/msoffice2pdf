package converter

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func destPathFromArgs(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--dst=") {
			return strings.TrimPrefix(a, "--dst=")
		}
		if a == "--dst" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func mapWorkerResult(err error, stderr, dstPath string) error {
	msg := strings.TrimSpace(stderr)
	if err == nil {
		if dstPath != "" {
			if _, statErr := os.Stat(dstPath); statErr != nil {
				return fmt.Errorf("converter: subprocess ok but dst missing: %w", statErr)
			}
		}
		return nil
	}
	if pwErr := ParseWorkerPasswordError(msg); pwErr != nil {
		return fmt.Errorf("converter: %w", pwErr)
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
