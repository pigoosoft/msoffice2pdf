package validate_test

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/validate"
)

func boolPtr(v bool) *bool { return &v }

func writeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.ofd")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, body := range entries {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOFDMagicAndStructure(t *testing.T) {
	cfg := config.UploadConfig{
		ValidateMagic: boolPtr(true),
		ValidateOFD:   map[string][]string{"*.ofd": {"OFD.xml"}},
	}
	pathOK := writeZip(t, map[string]string{"OFD.xml": "<ofd/>"})
	if err := validate.File(pathOK, "a.ofd", cfg); err != nil {
		t.Fatal(err)
	}
	pathBad := writeZip(t, map[string]string{"readme.txt": "x"})
	if err := validate.File(pathBad, "a.ofd", cfg); err == nil || !errors.Is(err, validate.ErrStructure) {
		t.Fatalf("want structure, got %v", err)
	}
}
