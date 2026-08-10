package watermark

import (
	"fmt"
	"os"
	"strings"
)

// ResolveFontPath returns configured font path or a Windows CJK fallback TTF/TTC.
func ResolveFontPath(configured string) (string, error) {
	if p := strings.TrimSpace(configured); p != "" {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return "", fmt.Errorf("watermark font_path not found: %s", p)
		}
		return p, nil
	}
	// Prefer TTF: gopdf AddTTFFont is unreliable with many TTC collections.
	candidates := []string{
		`C:\Windows\Fonts\simhei.ttf`,
		`C:\Windows\Fonts\simsunb.ttf`,
		`C:\Windows\Fonts\msyh.ttf`,
		`C:\Windows\Fonts\msyh.ttc`,
		`C:\Windows\Fonts\simsun.ttc`,
	}
	for _, c := range candidates {
		st, err := os.Stat(c)
		if err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("no watermark font: set watermark.font_path")
}
