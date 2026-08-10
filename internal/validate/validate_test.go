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

func TestMagicRejectsJPEGAsDocx(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.docx")
	// minimal JPEG SOI
	if err := os.WriteFile(p, []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0, 0, 0}, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.UploadConfig{
		ValidateNew: map[string][]string{"*.docx": {"word/document.xml"}},
	}
	err := validate.File(p, "x.docx", cfg)
	if !errors.Is(err, validate.ErrMagic) {
		t.Fatalf("want ErrMagic, got %v", err)
	}
}

func TestZIPMissingDocumentXML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.docx")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("[Content_Types].xml")
	_, _ = w.Write([]byte("<Types/>"))
	_ = zw.Close()
	_ = f.Close()

	cfg := config.UploadConfig{
		ValidateNew: map[string][]string{"*.docx": {"word/document.xml"}},
	}
	err = validate.File(p, "x.docx", cfg)
	if !errors.Is(err, validate.ErrStructure) {
		t.Fatalf("want ErrStructure, got %v", err)
	}
}
