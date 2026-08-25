//go:build !windows

package converter

import (
	"context"
	"os"
	"path/filepath"
)

func newConverter(opts Options) Converter {
	engines := map[string]Engine{}
	for _, n := range opts.Engines {
		switch n {
		case EngineOpenOffice:
			engines[n] = &openOfficeEngine{
				command:     opts.OpenOfficeCommand,
				userProfile: opts.OpenOfficeUserProfile,
			}
		case EngineOFD:
			engines[n] = &ofdEngine{}
		}
	}
	if len(engines) == 0 {
		return stubConverter{}
	}
	return &routingConverter{engines: engines, extEngines: opts.ExtEngines}
}

type stubConverter struct{}

var minimalPDF = []byte("%PDF-1.1\n%\xe2\xe3\xcf\xd3\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF\n")

func (stubConverter) Convert(ctx context.Context, srcPath, dstPath, password string) error {
	_ = password
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dstPath, minimalPDF, 0o644)
}
