package watermark

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseColorHex parses #RRGGBB into RGB components.
func ParseColorHex(s string) (uint8, uint8, uint8, error) {
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[0] != '#' {
		return 0, 0, 0, fmt.Errorf("color must be #RRGGBB, got %q", s)
	}
	r, err := strconv.ParseUint(s[1:3], 16, 8)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("color must be #RRGGBB, got %q", s)
	}
	g, err := strconv.ParseUint(s[3:5], 16, 8)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("color must be #RRGGBB, got %q", s)
	}
	b, err := strconv.ParseUint(s[5:7], 16, 8)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("color must be #RRGGBB, got %q", s)
	}
	return uint8(r), uint8(g), uint8(b), nil
}

// PrimaryFontSize returns configured size or an auto size from page width (pt).
func PrimaryFontSize(pageWidthPt, configured float64) float64 {
	if configured > 0 {
		return configured
	}
	v := pageWidthPt * 0.06
	if v < 18 {
		return 18
	}
	if v > 72 {
		return 72
	}
	return v
}

// SecondaryFontSize is ~70% of the primary size.
func SecondaryFontSize(primary float64) float64 {
	return primary * 0.7
}

// UnitSpacing returns along-text and across-row steps from the watermark unit
// bounding box so neighboring tiles do not overlap. density only widens the gap.
func UnitSpacing(density string, unitW, unitH float64) (along, across float64) {
	if unitW < 1 {
		unitW = 1
	}
	if unitH < 1 {
		unitH = 1
	}
	// Minimum clear gap: slightly larger than the text block itself.
	// Density multipliers are ~2 levels sparser than the first tiled revision
	// (each level ≈×1.22 on along/across).
	along = unitW * 1.25
	across = unitH * 2.0
	switch density {
	case "light":
		along *= 2.30
		across *= 2.30
	case "heavy":
		along *= 1.55
		across *= 1.62
	default: // medium
		along *= 1.85
		across *= 1.92
	}
	return along, across
}

// DensityStep is kept for callers that only have page size; prefer UnitSpacing.
func DensityStep(density string, pageW, pageH float64) float64 {
	base := math.Max(pageW, pageH)
	along, _ := UnitSpacing(density, base*0.35, base*0.08)
	return along
}

// AnchorPoints returns a multi-row staggered lattice of tile centers covering the page.
// Coordinates use top-left origin with Y downward (gopdf). angleDeg tilts the lattice
// (CCW from +X in a Y-up sense); odd rows are offset by half stepAlong (staggered).
func AnchorPoints(pageW, pageH, stepAlong, stepAcross, angleDeg float64) [][2]float64 {
	if stepAlong <= 0 {
		stepAlong = math.Max(pageW, pageH) * 0.40
	}
	if stepAcross <= 0 {
		stepAcross = stepAlong * 0.75
	}
	rad := angleDeg * math.Pi / 180
	ux := math.Cos(rad)
	uy := -math.Sin(rad)
	// Perpendicular in Y-down space (rotate u by +90° on screen).
	px, py := -uy, ux

	cx := pageW / 2
	cy := pageH / 2
	margin := math.Max(stepAlong, stepAcross)
	span := math.Hypot(pageW, pageH) + 2*margin
	maxCol := int(span/stepAlong) + 2
	maxRow := int(span/stepAcross) + 2

	var out [][2]float64
	for row := -maxRow; row <= maxRow; row++ {
		stagger := 0.0
		if row%2 != 0 {
			stagger = stepAlong * 0.5
		}
		for col := -maxCol; col <= maxCol; col++ {
			along := float64(col)*stepAlong + stagger
			across := float64(row) * stepAcross
			x := cx + along*ux + across*px
			y := cy + along*uy + across*py
			if x < -margin || x > pageW+margin || y < -margin || y > pageH+margin {
				continue
			}
			out = append(out, [2]float64{x, y})
		}
	}
	return out
}

// AnchorPointsByCount places exactly n staggered lattice centers covering the page.
// n must be in 1..19. Spacing scales with page size so tiles spread across the page.
func AnchorPointsByCount(pageW, pageH, angleDeg float64, n int) [][2]float64 {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return [][2]float64{{pageW / 2, pageH / 2}}
	}

	cols := int(math.Ceil(math.Sqrt(float64(n))))
	if cols < 1 {
		cols = 1
	}
	rows := int(math.Ceil(float64(n) / float64(cols)))
	if rows < 1 {
		rows = 1
	}

	rad := angleDeg * math.Pi / 180
	ux := math.Cos(rad)
	uy := -math.Sin(rad)
	px, py := -uy, ux

	cx := pageW / 2
	cy := pageH / 2
	// Spread across ~70% of page diagonal so edge units stay mostly on-page.
	span := math.Hypot(pageW, pageH) * 0.70
	stepAlong := span / float64(cols)
	stepAcross := span / float64(rows)
	if cols == 1 {
		stepAlong = span * 0.5
	}
	if rows == 1 {
		stepAcross = span * 0.5
	}

	out := make([][2]float64, 0, n)
	for i := 0; i < n; i++ {
		row := i / cols
		col := i % cols
		// Center the occupied cols on this row when last row is short.
		rowLen := cols
		if row == rows-1 {
			rem := n - row*cols
			if rem > 0 {
				rowLen = rem
			}
		}
		stagger := 0.0
		if row%2 != 0 {
			stagger = stepAlong * 0.5
		}
		// Center grid: map col in [0,rowLen) to offset around 0.
		along := (float64(col)-float64(rowLen-1)/2)*stepAlong + stagger
		across := (float64(row) - float64(rows-1)/2) * stepAcross
		x := cx + along*ux + across*px
		y := cy + along*uy + across*py
		out = append(out, [2]float64{x, y})
	}
	return out
}
