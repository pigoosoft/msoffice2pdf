package watermark

import (
	"context"
	"strings"
)

// Options controls text watermark appearance for one Apply call.
type Options struct {
	Primary      string
	Secondary    string
	Angle        float64
	Density      string
	DensityCount int // 1..19 replaces Density; 0 uses Density
	Opacity      float64
	Color        string
	FontSize     float64 // 0 = auto from page width
	FontPath     string
}

// UseDensityCount reports whether DensityCount should drive tiling instead of Density.
func UseDensityCount(n int) bool {
	return n > 0 && n < 20
}

// Watermarker applies a watermark to an existing PDF file.
type Watermarker interface {
	Apply(ctx context.Context, pdfPath string, opt Options) error
}

// Service implements Watermarker.
type Service struct{}

// Need reports whether any watermark text is present.
func Need(primary, secondary string) bool {
	return strings.TrimSpace(primary) != "" || strings.TrimSpace(secondary) != ""
}
