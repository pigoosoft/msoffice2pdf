// Package consoleattach couples or decouples the process from a console/terminal
// so one binary can serve both desktop UI and CLI modes.
package consoleattach

// EnsureCLI makes stdout/stderr usable for CLI and --noui paths.
// On Windows (GUI subsystem) it attaches to the parent console or allocates one.
// On Unix it is a no-op.
func EnsureCLI() {
	ensureCLI()
}

// DetachForUI disconnects the process from the launching terminal so closing
// that terminal does not stop the desktop UI. Call only after pre-UI checks
// and logger init, immediately before opening the desktop shell.
// On Windows (GUI subsystem) it is a no-op.
func DetachForUI() {
	detachForUI()
}
