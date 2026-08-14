package desktop

import "strings"

func ShouldUseUI(noui bool, goos, displayEnv string) bool {
	if noui {
		return false
	}
	switch goos {
	case "windows", "darwin":
		return true
	case "linux":
		return strings.TrimSpace(displayEnv) != ""
	default:
		return false
	}
}
