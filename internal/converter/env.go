package converter

import (
	"log/slog"
	"strings"
)

// LogUnmappedAllowedExts logs ERROR for each allowed_exts entry lacking ext_engines mapping.
// extEngines keys must already be bare lowercase extensions (post config.Validate).
func LogUnmappedAllowedExts(allowedExts []string, extEngines map[string]string) {
	var missing []string
	for _, pattern := range allowedExts {
		ext := normalizeExtKey(pattern)
		if ext == "" || ext == "*" {
			continue
		}
		if eng, ok := extEngines[ext]; !ok || eng == "" {
			slog.Error("converter: allowed ext has no ext_engines mapping", "ext", ext)
			missing = append(missing, ext)
		}
	}
	if len(missing) > 0 {
		slog.Error("converter: ext_engines incomplete for allowed_exts", "missing", missing)
	}
}

func normalizeExtKey(pattern string) string {
	p := strings.TrimSpace(pattern)
	p = strings.TrimPrefix(p, "*.")
	p = strings.TrimPrefix(p, ".")
	return strings.ToLower(strings.TrimSpace(p))
}
