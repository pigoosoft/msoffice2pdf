package converter

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestUserInstallationURI(t *testing.T) {
	if runtime.GOOS == "windows" {
		got := userInstallationURI(`C:\data\lo\abc`)
		if got != "file:///C:/data/lo/abc" {
			t.Fatalf("got %q", got)
		}
		return
	}
	got := userInstallationURI("/var/lo/abc")
	if got != "file:///var/lo/abc" {
		t.Fatalf("got %q", got)
	}
}

func TestFindConvertedPDF(t *testing.T) {
	dir := t.TempDir()
	name := "report.pdf"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findConvertedPDF(dir, "report.docx")
	if err != nil || got != path {
		t.Fatalf("got %q err %v", got, err)
	}

	t.Run("fallback single pdf", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "other.pdf")
		if err := os.WriteFile(path, []byte("%PDF"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := findConvertedPDF(dir, "report.docx")
		if err != nil || got != path {
			t.Fatalf("got %q err %v", got, err)
		}
	})

	t.Run("none", func(t *testing.T) {
		dir := t.TempDir()
		_, err := findConvertedPDF(dir, "report.docx")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("multiple", func(t *testing.T) {
		dir := t.TempDir()
		for _, n := range []string{"a.pdf", "b.pdf"} {
			if err := os.WriteFile(filepath.Join(dir, n), []byte("%PDF"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, err := findConvertedPDF(dir, "report.docx")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
