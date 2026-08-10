//go:build windows

package desktop

import (
	"strings"

	"golang.org/x/sys/windows"
)

func isChineseOSLocale() bool {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(langs) == 0 {
		return false
	}
	return strings.HasPrefix(strings.ToLower(langs[0]), "zh")
}
