package converter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type ofdEngine struct{}

func (e *ofdEngine) Name() string { return EngineOFD }

func (e *ofdEngine) Validate() error { return nil }

func (e *ofdEngine) ProcessImages() []string { return nil }

func (e *ofdEngine) Convert(ctx context.Context, srcPath, dstPath, password string) error {
	srcPath, err := filepath.Abs(srcPath)
	if err != nil {
		return fmt.Errorf("converter: abs src: %w", err)
	}
	dstPath, err = filepath.Abs(dstPath)
	if err != nil {
		return fmt.Errorf("converter: abs dst: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("converter: mkdir dst: %w", err)
	}
	_ = os.Remove(dstPath)
	return runConvertWorker(ctx, ofdWorkerArgs(srcPath, dstPath), password, os.Environ())
}

func ofdWorkerArgs(src, dst string) []string {
	return []string{"convert-worker", "--src", src, "--dst", dst, "--engine", EngineOFD}
}
