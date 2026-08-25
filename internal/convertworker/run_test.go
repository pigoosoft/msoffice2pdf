package convertworker_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"msoffice2pdf/internal/convertworker"
	"msoffice2pdf/internal/ofd"
)

func TestRunOFDWithoutAppKindHitsOFDConvert(t *testing.T) {
	src, dst := invalidOFDPaths(t)
	err := convertworker.Run([]string{"--src", src, "--dst", dst, "--engine", "ofd"})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "--app-kind must be") {
		t.Fatalf("must not require --app-kind for OFD: %v", err)
	}
	if !errors.Is(err, ofd.ErrInvalidPackage) {
		t.Fatalf("got %v, want ERR_OFD_INVALID_PACKAGE", err)
	}
}

func TestRunMSOfficeRequiresAppKind(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.docx")
	dst := filepath.Join(dir, "out.pdf")
	err := convertworker.Run([]string{"--src", src, "--dst", dst, "--engine", "msoffice"})
	if err == nil {
		t.Fatal("expected error")
	}
	want := "--app-kind must be writer, spreadsheet, or presentation"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("got %v, want substring %q", err, want)
	}
}

func TestRunOFDIgnoresAppKind(t *testing.T) {
	src, dst := invalidOFDPaths(t)
	err := convertworker.Run([]string{"--src", src, "--dst", dst, "--engine", "ofd", "--app-kind", "writer"})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "--app-kind must be") {
		t.Fatalf("must not fail for app-kind: %v", err)
	}
	if !errors.Is(err, ofd.ErrInvalidPackage) {
		t.Fatalf("got %v, want ERR_OFD_INVALID_PACKAGE (COM path used?)", err)
	}
}

func invalidOFDPaths(t *testing.T) (src, dst string) {
	t.Helper()
	dir := t.TempDir()
	src = filepath.Join(dir, "bad.ofd")
	dst = filepath.Join(dir, "out.pdf")
	if err := os.WriteFile(src, []byte("not an ofd"), 0o644); err != nil {
		t.Fatal(err)
	}
	return src, dst
}
