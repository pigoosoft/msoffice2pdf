package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeDesktopLanguage(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"", "en", false},
		{"en", "en", true},
		{"EN", "en", true},
		{"en-US", "en", true},
		{"zh", "zh", true},
		{"zh-CN", "zh", true},
		{"zh_CN", "zh", true},
		{"zh-TW", "zh", true},
		{"ja", "en", false},
		{"fr-FR", "en", false},
	}
	for _, c := range cases {
		got, ok := NormalizeDesktopLanguage(c.in)
		if got != c.want || ok != c.wantOK {
			t.Fatalf("%q: got (%s,%v) want (%s,%v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

func TestUpsertDesktopLanguageYAML_append(t *testing.T) {
	src := []byte("server:\n  port: 8080\n")
	out := string(upsertDesktopLanguageYAML(src, "zh"))
	if !strings.Contains(out, "desktop:\n  language: zh") {
		t.Fatalf("missing desktop section:\n%s", out)
	}
	if !strings.Contains(out, "server:\n  port: 8080") {
		t.Fatalf("lost existing keys:\n%s", out)
	}
}

func TestUpsertDesktopLanguageYAML_replace(t *testing.T) {
	src := []byte("watermark:\n  text: x\n\ndesktop:\n  language: en\n")
	out := string(upsertDesktopLanguageYAML(src, "zh"))
	if strings.Count(out, "language:") != 1 {
		t.Fatalf("expected one language key:\n%s", out)
	}
	if !strings.Contains(out, "  language: zh") {
		t.Fatalf("not replaced:\n%s", out)
	}
	if !strings.Contains(out, "  text: x") {
		t.Fatalf("lost sibling content:\n%s", out)
	}
}

func TestUpsertDesktopLanguageYAML_insertUnderDesktop(t *testing.T) {
	src := []byte("desktop:\n")
	out := string(upsertDesktopLanguageYAML(src, "en"))
	if out != "desktop:\n  language: en" && out != "desktop:\n  language: en\n" {
		t.Fatalf("unexpected:\n%q", out)
	}
}

func TestSetDesktopLanguage_roundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	orig := "server:\n  port: 8080\n\n# keep this comment\nwatermark:\n  text: \"\"\n"
	if err := os.WriteFile(path, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	if err := SetDesktopLanguage(path, "zh-CN"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "# keep this comment") {
		t.Fatalf("comment lost:\n%s", got)
	}
	if !strings.Contains(got, "  language: zh") {
		t.Fatalf("language not written:\n%s", got)
	}
	if err := SetDesktopLanguage(path, "en"); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got = string(data)
	if !strings.Contains(got, "  language: en") {
		t.Fatalf("language not updated:\n%s", got)
	}
	if strings.Contains(got, "language: zh") {
		t.Fatalf("old value remains:\n%s", got)
	}
}
