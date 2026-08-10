package converter

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const tempSandboxPrefix = "msoffice2pdf-com-"

func createTempSandboxDir() (string, error) {
	id := strings.ReplaceAll(uuid.NewString(), "-", "")
	dir := filepath.Join(os.TempDir(), tempSandboxPrefix+id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("converter: temp sandbox: %w", err)
	}
	return dir, nil
}

// removeTempSandbox deletes a per-task TEMP dir. Excel often keeps Diagnostics
// logs open briefly after Quit, so we retry before giving up (sweep handles leftovers).
func removeTempSandbox(dir string) {
	var last error
	for i := 0; i < 10; i++ {
		last = os.RemoveAll(dir)
		if last == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	slog.Warn("converter: remove temp sandbox", "dir", dir, "err", last)
}

func sandboxEnv(dir string) []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+3)
	for _, e := range env {
		upper := strings.ToUpper(e)
		if strings.HasPrefix(upper, "TEMP=") ||
			strings.HasPrefix(upper, "TMP=") ||
			strings.HasPrefix(upper, "TMPDIR=") {
			continue
		}
		out = append(out, e)
	}
	out = append(out,
		"TEMP="+dir,
		"TMP="+dir,
		"TMPDIR="+dir,
	)
	return out
}
