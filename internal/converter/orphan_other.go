//go:build !windows

package converter

import "os/exec"

// CleanupOrphanOfficeProcesses terminates leftover office/WPS/soffice processes for enabled engines.
func CleanupOrphanOfficeProcesses(engines []string) {
	for _, image := range ImagesForEngines(engines) {
		cmd := exec.Command("pkill", "-x", image)
		_ = cmd.Run()
	}
}

// CleanupOrphanConvertWorkers is a no-op on non-Windows builds.
func CleanupOrphanConvertWorkers() {}

// CleanupOrphansAtStart clears leftover office processes for enabled engines.
func CleanupOrphansAtStart(engines []string) {
	CleanupOrphanOfficeProcesses(engines)
}
