package ui

import (
	"fmt"
	"math"

	"github.com/lucasb-eyer/go-colorful"
)

// linearize converts a single sRGB channel value in [0, 1] to linear light
// using the WCAG-specified sRGB piecewise function.
func linearize(ch float64) float64 {
	if ch <= 0.04045 {
		return ch / 12.92
	}
	return math.Pow((ch+0.055)/1.055, 2.4)
}

// relativeLuminance computes the WCAG 2.1 relative luminance of an sRGB color
// whose components are in [0, 1].
func relativeLuminance(r, g, b float64) float64 {
	return 0.2126*linearize(r) + 0.7152*linearize(g) + 0.0722*linearize(b)
}

// contrastRatio computes the WCAG 2.1 contrast ratio between two luminance
// values. The result is always >= 1 (the ratio of the lighter to the darker,
// offset by 0.05 to avoid division by zero for pure black).
func contrastRatio(l1, l2 float64) float64 {
	lighter := math.Max(l1, l2)
	darker := math.Min(l1, l2)
	return (lighter + 0.05) / (darker + 0.05)
}

// rowTintBgBlend is how far a row-tint background moves from the theme base
// toward the severity color: strong enough to read as a tint, muted enough
// that the selection highlight stays clearly distinct (issue #540).
const rowTintBgBlend = 0.22

// rowTintCursorBlend keeps a selected tinted row reading as both "failed"
// and "cursor" instead of losing the cursor highlight (issue #540 UAT).
const rowTintCursorBlend = 0.5

// blendHex blends base toward tint by amount (0 = base, 1 = tint).
// Unparsable inputs return tint unchanged.
func blendHex(base, tint string, amount float64) string {
	baseCol, errBase := colorful.Hex(base)
	tintCol, errTint := colorful.Hex(tint)
	if errBase != nil || errTint != nil {
		return tint
	}
	c := baseCol.BlendRgb(tintCol, amount)
	// Truncates rather than rounds so existing theme colors stay byte-identical.
	return fmt.Sprintf("#%02x%02x%02x", uint8(c.R*255), uint8(c.G*255), uint8(c.B*255))
}

// derivedParentHighlightBg blends Border toward Base until bold Text over it
// clears the WCAG AA large-text floor (3.0:1). Border skips fg-readability
// enforcement because it also has a decorative role on column outlines, so
// a near-white "bright black" theme could collapse Text-on-Border to
// invisibility.
func derivedParentHighlightBg(t Theme) string {
	const target = 3.0

	textCol, errText := colorful.Hex(t.Text)
	borderCol, errBorder := colorful.Hex(t.Border)
	baseCol, errBase := colorful.Hex(t.Base)
	if errText != nil || errBorder != nil || errBase != nil {
		return t.Border
	}

	lText := relativeLuminance(textCol.R, textCol.G, textCol.B)
	if contrastRatio(lText, relativeLuminance(borderCol.R, borderCol.G, borderCol.B)) >= target {
		return t.Border
	}

	// Binary search the blend amount in [0, 1] (0 = Border, 1 = Base).
	// 30 iterations converges to far below 1/255 in each channel, more
	// precision than hex-truncation can represent.
	const iterations = 30
	lo, hi := 0.0, 1.0
	for range iterations {
		mid := (lo + hi) / 2.0
		blended := borderCol.BlendRgb(baseCol, mid)
		if contrastRatio(lText, relativeLuminance(blended.R, blended.G, blended.B)) >= target {
			hi = mid
		} else {
			lo = mid
		}
	}
	return borderCol.BlendRgb(baseCol, hi).Hex()
}

// EnforceMinContrast nudges fg's HSL lightness (hue and saturation kept) to
// meet a WCAG contrast ratio against bg. value is the normalized knob in
// [0, 1]: 0 is off, 0.175 is the AA threshold (4.5:1), 0.3 is the AAA
// threshold (7.0:1), 1.0 targets 21:1 via wcagTarget = 1.0 + clamp(value, 0,
// 1) * 20.0. At value=1.0 hue collapses to achromatic black or white, an
// accepted tradeoff. An unparsable fg or bg returns fg unchanged.
func EnforceMinContrast(fg, bg string, value float64) string {
	if value <= 0 {
		return fg
	}

	fgCol, err := colorful.Hex(fg)
	if err != nil {
		return fg
	}
	bgCol, err := colorful.Hex(bg)
	if err != nil {
		return fg
	}

	clamp01 := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 1
		}
		return v
	}

	target := 1.0 + clamp01(value)*20.0

	lFg := relativeLuminance(fgCol.R, fgCol.G, fgCol.B)
	lBg := relativeLuminance(bgCol.R, bgCol.G, bgCol.B)

	if contrastRatio(lFg, lBg) >= target {
		return fg
	}

	// Determine nudge direction by preserving the designer's existing
	// fg/bg relationship: if fg was already darker than bg, go darker;
	// if lighter, go lighter. This matters most for mid-luminance
	// backgrounds (e.g. a selected-row highlight blue around L ≈ 0.35)
	// where "go lighter against anything below 0.5" would flip a dark fg
	// past the bg toward pure white, silently tanking contrast because
	// the lighter side of a mid bg has far less headroom than the darker
	// side. Tie (ratio 1, fg == bg — pathological but possible in a
	// broken theme): fall back to the WCAG crossover point at L ≈ 0.179
	// where going lighter vs darker have equal max achievable contrast.
	goLighter := lFg > lBg
	if math.Abs(lFg-lBg) < 1e-9 {
		goLighter = lBg < 0.179
	}

	h, s, l := fgCol.Hsl()

	// Binary search over lightness in [0, 1] for 40 iterations. This is more
	// than enough for convergence to well under 0.001 precision in L.
	const iterations = 40
	var lo, hi float64
	if goLighter {
		lo, hi = l, 1.0
	} else {
		lo, hi = 0.0, l
	}

	for range iterations {
		mid := (lo + hi) / 2.0
		midCol := colorful.Hsl(h, s, mid)
		lMid := relativeLuminance(midCol.R, midCol.G, midCol.B)
		if contrastRatio(lMid, lBg) >= target {
			if goLighter {
				hi = mid // can go darker while still meeting target
			} else {
				lo = mid // can go lighter while still meeting target
			}
		} else {
			if goLighter {
				lo = mid // need to go lighter
			} else {
				hi = mid // need to go darker
			}
		}
	}

	// Use the extreme of the converged range that meets the target. If even the
	// endpoint doesn't hit target (e.g. bg is itself extreme), use it anyway --
	// it's the best we can do while staying within [0, 1].
	var finalL float64
	if goLighter {
		finalL = hi
	} else {
		finalL = lo
	}

	return colorful.Hsl(h, s, finalL).Hex()
}
