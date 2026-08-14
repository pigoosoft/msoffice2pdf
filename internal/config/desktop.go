package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	DesktopLangEN = "en"
	DesktopLangZH = "zh"
)

// NormalizeDesktopLanguage maps a config/OS value to en or zh.
// ok is false when raw is empty or not a supported language.
func NormalizeDesktopLanguage(raw string) (lang string, ok bool) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return DesktopLangEN, false
	}
	v = strings.ReplaceAll(v, "_", "-")
	switch {
	case v == "en" || strings.HasPrefix(v, "en-"):
		return DesktopLangEN, true
	case v == "zh" || strings.HasPrefix(v, "zh-") || strings.HasPrefix(v, "zh."):
		return DesktopLangZH, true
	default:
		return DesktopLangEN, false
	}
}

// SetDesktopLanguage writes desktop.language into the YAML file at path,
// changing only that key (or appending a desktop section). Other content is kept.
func SetDesktopLanguage(path, lang string) error {
	canonical, ok := NormalizeDesktopLanguage(lang)
	if !ok || (canonical != DesktopLangEN && canonical != DesktopLangZH) {
		return fmt.Errorf("desktop.language must be en or zh, got %q", lang)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat config: %w", err)
	}
	out := upsertDesktopLanguageYAML(data, canonical)
	if err := writeFileReplace(path, out, info.Mode()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func upsertDesktopLanguageYAML(src []byte, lang string) []byte {
	nl := "\n"
	if strings.Contains(string(src), "\r\n") {
		nl = "\r\n"
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	lines := strings.Split(text, "\n")

	desktopIdx := -1
	languageIdx := -1
	inDesktop := false
	for i, line := range lines {
		if isTopLevelKey(line, "desktop") {
			desktopIdx = i
			inDesktop = true
			languageIdx = -1
			continue
		}
		if !inDesktop {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			inDesktop = false
			continue
		}
		if languageIdx < 0 && isKeyLine(line, "language") {
			languageIdx = i
		}
	}

	if languageIdx >= 0 {
		lines[languageIdx] = rewriteLanguageLine(lines[languageIdx], lang)
	} else if desktopIdx >= 0 {
		insert := "  language: " + lang
		rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[desktopIdx]), "desktop:"))
		if rest != "" && !strings.HasPrefix(rest, "#") {
			comment := ""
			if i := strings.Index(rest, " #"); i >= 0 {
				comment = rest[i:]
			}
			lines[desktopIdx] = "desktop:" + comment
		}
		lines = append(lines[:desktopIdx+1], append([]string{insert}, lines[desktopIdx+1:]...)...)
	} else {
		if len(lines) == 0 {
			lines = []string{"desktop:", "  language: " + lang}
		} else {
			if lines[len(lines)-1] != "" {
				lines = append(lines, "")
			}
			lines = append(lines, "desktop:", "  language: "+lang)
		}
	}
	return []byte(strings.Join(lines, nl))
}

func isTopLevelKey(line, key string) bool {
	if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
		return false
	}
	return strings.HasPrefix(line, key+":")
}

func isKeyLine(line, key string) bool {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") {
		return false
	}
	return strings.HasPrefix(t, key+":")
}

func rewriteLanguageLine(line, lang string) string {
	indentLen := len(line) - len(strings.TrimLeft(line, " \t"))
	indent := line[:indentLen]
	comment := ""
	if i := strings.Index(line, " #"); i >= 0 {
		comment = line[i:]
	}
	return indent + "language: " + lang + comment
}

func writeFileReplace(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return err2
		}
	}
	return nil
}
