package applog

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"msoffice2pdf/internal/config"
)

func TestDailyWriterDetailFileName(t *testing.T) {
	dir := t.TempDir()
	w, err := newDailyWriter(dir, 0, "_detail")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("detail-line\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, time.Now().Format("20060102")+"_detail.log")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "detail-line") {
		t.Fatalf("got %q", b)
	}
}

func TestInitWritesJSONToDetailFile(t *testing.T) {
	prevLog := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(prevLog)
		consoleSink = os.Stdout
	})

	dir := t.TempDir()
	on := true
	closer, err := Init(config.LogConfig{
		Level:         "info",
		FileEnabled:   &on,
		FileDir:       dir,
		FlushInterval: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	const marker = "detail-json-marker"
	slog.Info(marker)
	if _, err := ConsoleWriter().Write([]byte("gin-like-line\n")); err != nil {
		t.Fatal(err)
	}
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}

	day := time.Now().Format("20060102")
	detail, err := os.ReadFile(filepath.Join(dir, day+"_detail.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(detail), marker) {
		t.Fatalf("detail file missing JSON marker: %s", detail)
	}
	if !strings.Contains(string(detail), "gin-like-line") {
		t.Fatalf("detail file missing console sink line: %s", detail)
	}
	summary, err := os.ReadFile(filepath.Join(dir, day+".log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(summary), marker) {
		t.Fatalf("summary file missing marker: %s", summary)
	}
}
