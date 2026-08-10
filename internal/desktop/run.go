package desktop

import (
	"msoffice2pdf/internal/applog"
	"msoffice2pdf/internal/appruntime"
	"msoffice2pdf/internal/config"
)

// Run blocks until the window closes. Runtime starts stopped.
// On window close, Stop is invoked if the runtime is still active.
func Run(cfg *config.Config, configPath string, ring *applog.Ring, rt *appruntime.Runtime) error {
	return runFyne(cfg, configPath, ring, rt)
}
