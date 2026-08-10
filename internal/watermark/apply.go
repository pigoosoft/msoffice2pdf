package watermark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// Apply stamps a tiled text watermark onto pdfPath in place.
// Uses the same ctx for cancellation/timeout; returns ctx.Err() when cancelled.
func (Service) Apply(ctx context.Context, pdfPath string, opt Options) error {
	return Apply(ctx, pdfPath, opt)
}

// Apply stamps a tiled text watermark onto pdfPath in place.
func Apply(ctx context.Context, pdfPath string, opt Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !Need(opt.Primary, opt.Secondary) {
		return nil
	}

	fontPath, err := ResolveFontPath(opt.FontPath)
	if err != nil {
		return err
	}

	dims, err := api.PageDimsFile(pdfPath)
	if err != nil {
		return fmt.Errorf("watermark: page dims: %w", err)
	}
	if len(dims) == 0 {
		return fmt.Errorf("watermark: pdf has no pages")
	}

	tmpDir, err := os.MkdirTemp("", "msoffice2pdf-wm-*")
	if err != nil {
		return fmt.Errorf("watermark: temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	current := pdfPath
	var intermediates []string
	defer func() {
		for _, p := range intermediates {
			_ = os.Remove(p)
		}
	}()

	conf := model.NewDefaultConfiguration()

	for i, d := range dims {
		if err := ctx.Err(); err != nil {
			return err
		}
		overlayPath := filepath.Join(tmpDir, fmt.Sprintf("overlay-%d.pdf", i+1))
		if err := writeOverlay(overlayPath, d.Width, d.Height, fontPath, opt); err != nil {
			return err
		}

		outPath := filepath.Join(tmpDir, fmt.Sprintf("stamped-%d.pdf", i+1))
		wm, err := api.PDFWatermark(overlayPath, "pos:c, rot:0, scale:1", true, false, types.POINTS)
		if err != nil {
			return fmt.Errorf("watermark: configure: %w", err)
		}
		wm.Scale = 1
		wm.ScaleAbs = true
		wm.Diagonal = model.NoDiagonal
		wm.Opacity = 1

		pageSel := []string{strconv.Itoa(i + 1)}
		if err := api.AddWatermarksFile(current, outPath, pageSel, wm, conf); err != nil {
			return fmt.Errorf("watermark: stamp page %d: %w", i+1, err)
		}
		if current != pdfPath {
			intermediates = append(intermediates, current)
		}
		current = outPath
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	finalTmp := pdfPath + ".wm.tmp.pdf"
	_ = os.Remove(finalTmp)
	data, err := os.ReadFile(current)
	if err != nil {
		return fmt.Errorf("watermark: read result: %w", err)
	}
	if err := os.WriteFile(finalTmp, data, 0o644); err != nil {
		return fmt.Errorf("watermark: write temp: %w", err)
	}
	_ = os.Remove(pdfPath)
	if err := os.Rename(finalTmp, pdfPath); err != nil {
		return fmt.Errorf("watermark: replace: %w", err)
	}
	return nil
}
