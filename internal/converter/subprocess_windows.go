//go:build windows

package converter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func newSubprocessConverter(opts Options) Converter {
	return &subprocessConverter{
		excelPageFit: opts.ExcelPageFit,
		tempSandbox:  opts.TempSandbox,
		extEngines:   opts.ExtEngines,
		extAppKinds:  opts.ExtAppKinds,
	}
}

type subprocessConverter struct {
	excelPageFit string
	tempSandbox  bool
	extEngines   map[string]string
	extAppKinds  map[string]AppKind
}

func (c *subprocessConverter) Convert(ctx context.Context, srcPath, dstPath, password string) error {
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

	fit := c.excelPageFit
	if fit == "" {
		fit = "fit_width"
	}

	bare := strings.ToLower(strings.TrimPrefix(filepath.Ext(srcPath), "."))
	engine, ok := c.extEngines[bare]
	if !ok || engine == "" {
		return fmt.Errorf("converter: no engine mapped for .%s", bare)
	}
	kind, ok := c.extAppKinds[bare]
	if !ok {
		return fmt.Errorf("converter: no app kind for .%s (from validate_*)", bare)
	}

	args := []string{
		"convert-worker",
		"--src", srcPath,
		"--dst", dstPath,
		"--excel-page-fit", fit,
		"--engine", engine,
		"--app-kind", string(kind),
	}

	var baseEnv []string
	if c.tempSandbox {
		dir, err := createTempSandboxDir()
		if err != nil {
			return err
		}
		defer removeTempSandbox(dir)
		baseEnv = sandboxEnv(dir)
	}

	return runConvertWorker(ctx, args, password, baseEnv)
}
