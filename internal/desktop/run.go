package desktop

import (
	"fmt"

	"msoffice2pdf/internal/applog"
	"msoffice2pdf/internal/appruntime"
	"msoffice2pdf/internal/config"
)

// Run blocks until the window closes. Runtime starts stopped.
// Stub until Task 5 implements the Fyne UI; callers fall back to console mode.
func Run(cfg *config.Config, configPath string, ring *applog.Ring, rt *appruntime.Runtime) error {
	return fmt.Errorf("desktop ui not implemented")
}
