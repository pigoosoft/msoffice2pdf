package ofd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestConvertMinimalOFDProducesPDF(t *testing.T) {
	src := fixturePath(t, "minimal.ofd")
	writeMinimalOFD(t, src, fixtureOpts{docBodies: 1})
	dst := filepath.Join(t.TempDir(), "out.pdf")

	if err := Convert(context.Background(), src, dst, Options{}); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	assertValidPDF(t, dst, 1)
}

func TestConvertTwoDocBodiesTwoPages(t *testing.T) {
	src := fixturePath(t, "two.ofd")
	writeMinimalOFD(t, src, fixtureOpts{docBodies: 2})
	dst := filepath.Join(t.TempDir(), "out.pdf")

	if err := Convert(context.Background(), src, dst, Options{}); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	assertValidPDF(t, dst, 2)
}

func TestConvertEmptyZipInvalidPackage(t *testing.T) {
	src := fixturePath(t, "empty.ofd")
	writeEmptyZip(t, src)
	dst := filepath.Join(t.TempDir(), "out.pdf")

	err := Convert(context.Background(), src, dst, Options{})
	if !errors.Is(err, ErrInvalidPackage) {
		t.Fatalf("got %v, want ErrInvalidPackage", err)
	}
}

func TestConvertNoPages(t *testing.T) {
	src := fixturePath(t, "nopages.ofd")
	writeMinimalOFD(t, src, fixtureOpts{docBodies: 1, noPages: true})
	dst := filepath.Join(t.TempDir(), "out.pdf")

	err := Convert(context.Background(), src, dst, Options{})
	if !errors.Is(err, ErrNoPages) {
		t.Fatalf("got %v, want ErrNoPages", err)
	}
}

func TestConvertCancelledContext(t *testing.T) {
	src := fixturePath(t, "cancel.ofd")
	writeMinimalOFD(t, src, fixtureOpts{docBodies: 1})
	dst := filepath.Join(t.TempDir(), "out.pdf")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Convert(ctx, src, dst, Options{})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Fatalf("dst should not exist, stat=%v", statErr)
	}
}

func assertValidPDF(t *testing.T, path string, wantPages int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("missing %%PDF header")
	}
	if !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatalf("missing %%%%EOF")
	}
	n, err := api.PageCountFile(path)
	if err != nil {
		t.Fatalf("PageCountFile: %v", err)
	}
	if n != wantPages {
		t.Fatalf("pages=%d want=%d", n, wantPages)
	}
}
