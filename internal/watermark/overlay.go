package watermark

import (
	"fmt"
	"strings"

	"github.com/signintech/gopdf"
)

const fontFamily = "wm"

func writeOverlay(path string, pageW, pageH float64, fontPath string, opt Options) error {
	primary := strings.TrimSpace(opt.Primary)
	secondary := strings.TrimSpace(opt.Secondary)
	if primary == "" && secondary == "" {
		return fmt.Errorf("watermark: empty text")
	}

	r, g, b, err := ParseColorHex(opt.Color)
	if err != nil {
		return err
	}

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{Unit: gopdf.UnitPT, PageSize: gopdf.Rect{W: pageW, H: pageH}})
	pdf.AddPage()
	if err := pdf.AddTTFFont(fontFamily, fontPath); err != nil {
		return fmt.Errorf("watermark: load font: %w", err)
	}
	tr, err := gopdf.NewTransparency(opt.Opacity, "")
	if err != nil {
		return fmt.Errorf("watermark: transparency: %w", err)
	}
	if err := pdf.SetTransparency(tr); err != nil {
		return fmt.Errorf("watermark: set transparency: %w", err)
	}
	pdf.SetTextColor(r, g, b)

	primSize := PrimaryFontSize(pageW, opt.FontSize)
	secSize := SecondaryFontSize(primSize)
	var anchors [][2]float64
	if UseDensityCount(opt.DensityCount) {
		anchors = AnchorPointsByCount(pageW, pageH, opt.Angle, opt.DensityCount)
	} else {
		unitW, unitH, err := measureUnit(pdf, primSize, secSize, primary, secondary)
		if err != nil {
			return err
		}
		stepAlong, stepAcross := UnitSpacing(opt.Density, unitW, unitH)
		anchors = AnchorPoints(pageW, pageH, stepAlong, stepAcross, opt.Angle)
	}

	for _, a := range anchors {
		if err := drawUnit(pdf, a[0], a[1], opt.Angle, primSize, secSize, primary, secondary); err != nil {
			return err
		}
	}
	if err := pdf.WritePdf(path); err != nil {
		return fmt.Errorf("watermark: write overlay: %w", err)
	}
	return nil
}

func unitLines(primarySize, secondarySize float64, primary, secondary string) []struct {
	text string
	size float64
} {
	var lines []struct {
		text string
		size float64
	}
	if primary != "" {
		lines = append(lines, struct {
			text string
			size float64
		}{primary, primarySize})
	}
	if secondary != "" {
		lines = append(lines, struct {
			text string
			size float64
		}{secondary, secondarySize})
	}
	return lines
}

func measureUnit(pdf *gopdf.GoPdf, primarySize, secondarySize float64, primary, secondary string) (w, h float64, err error) {
	lines := unitLines(primarySize, secondarySize, primary, secondary)
	if len(lines) == 0 {
		return 0, 0, fmt.Errorf("watermark: empty text")
	}
	lineGap := primarySize * 0.35
	for i, ln := range lines {
		if err := pdf.SetFont(fontFamily, "", ln.size); err != nil {
			return 0, 0, err
		}
		tw, err := pdf.MeasureTextWidth(ln.text)
		if err != nil {
			return 0, 0, err
		}
		if tw > w {
			w = tw
		}
		h += ln.size
		if i > 0 {
			h += lineGap
		}
	}
	return w, h, nil
}

func drawUnit(pdf *gopdf.GoPdf, x, y, angle, primarySize, secondarySize float64, primary, secondary string) error {
	pdf.Rotate(angle, x, y)
	defer pdf.RotateReset()

	lines := unitLines(primarySize, secondarySize, primary, secondary)
	if len(lines) == 0 {
		return nil
	}
	lineGap := primarySize * 0.35
	totalH := 0.0
	for i, ln := range lines {
		totalH += ln.size
		if i > 0 {
			totalH += lineGap
		}
	}
	// Start above center so the block is vertically centered on (x,y).
	cursorY := y - totalH/2

	for _, ln := range lines {
		if err := pdf.SetFont(fontFamily, "", ln.size); err != nil {
			return err
		}
		w, err := pdf.MeasureTextWidth(ln.text)
		if err != nil {
			return err
		}
		pdf.SetXY(x-w/2, cursorY)
		if err := pdf.Cell(nil, ln.text); err != nil {
			return err
		}
		cursorY += ln.size + lineGap
	}
	return nil
}
