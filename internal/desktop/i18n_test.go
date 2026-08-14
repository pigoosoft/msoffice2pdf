package desktop

import (
	"testing"

	"msoffice2pdf/internal/config"
)

func TestLooksChineseLocale(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"C", false},
		{"POSIX", false},
		{"en_US.UTF-8", false},
		{"zh", true},
		{"zh_CN", true},
		{"zh-CN", true},
		{"zh_TW.UTF-8", true},
	}
	for _, c := range cases {
		if got := looksChineseLocale(c.in); got != c.want {
			t.Fatalf("%q got %v want %v", c.in, got, c.want)
		}
	}
}

func TestResolveLanguageFromConfig(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"en", config.DesktopLangEN},
		{"en-US", config.DesktopLangEN},
		{"zh", config.DesktopLangZH},
		{"zh-CN", config.DesktopLangZH},
		{"ja", config.DesktopLangEN},
		{"not-a-lang", config.DesktopLangEN},
	}
	for _, c := range cases {
		if got := resolveLanguage(c.in); got != c.want {
			t.Fatalf("%q got %s want %s", c.in, got, c.want)
		}
	}
}

func TestResolveLanguageEmptyFollowsChineseEnv(t *testing.T) {
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	t.Setenv("LANG", "")
	t.Setenv("LC_MESSAGES", "")
	if got := resolveLanguage(""); got != config.DesktopLangZH {
		t.Fatalf("got %s", got)
	}
}

func TestOptionLangRoundTrip(t *testing.T) {
	if optionFromLang(config.DesktopLangZH) != langOptionZH {
		t.Fatal("zh option")
	}
	if langFromOption(langOptionZH) != config.DesktopLangZH {
		t.Fatal("zh from option")
	}
	if langFromOption("EN") != config.DesktopLangEN {
		t.Fatal("en from option")
	}
}
