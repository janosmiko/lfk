package ui

import (
	"fmt"
	"math"
	"strconv"
)

// parseHexColor parses a CSS hex color string ("#rrggbb" or "#rgb") and
// returns the RGB components in [0, 1]. Returns ok=false for any unrecognised
// input (named colors, malformed strings, wrong length) so callers can decide
// to leave the original color unchanged rather than crash.
func parseHexColor(s string) (r, g, b float64, ok bool) {
	if len(s) == 0 || s[0] != '#' {
		return 0, 0, 0, false
	}
	hex := s[1:]
	switch len(hex) {
	case 3:
		// Expand #rgb -> #rrggbb
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	case 6:
		// Already the right length.
	default:
		return 0, 0, 0, false
	}
	ri, err := strconv.ParseInt(hex[0:2], 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	gi, err := strconv.ParseInt(hex[2:4], 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	bi, err := strconv.ParseInt(hex[4:6], 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return float64(ri) / 255.0, float64(gi) / 255.0, float64(bi) / 255.0, true
}

// formatHexColor converts RGB components in [0, 1] to a lowercase "#rrggbb"
// CSS hex string. Values outside [0, 1] are clamped.
func formatHexColor(r, g, b float64) string {
	clamp := func(v float64) int {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 255
		}
		return int(math.Round(v * 255))
	}
	return fmt.Sprintf("#%02x%02x%02x", clamp(r), clamp(g), clamp(b))
}

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

// rgbToHSL converts an sRGB color (components in [0, 1]) to HSL where:
//   - h is in [0, 1] representing degrees [0, 360)
//   - s is in [0, 1]
//   - l is in [0, 1]
func rgbToHSL(r, g, b float64) (h, s, l float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2.0
	if max == min {
		// Achromatic.
		return 0, 0, l
	}
	d := max - min
	if l > 0.5 {
		s = d / (2.0 - max - min)
	} else {
		s = d / (max + min)
	}
	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default: // b
		h = (r-g)/d + 4
	}
	h /= 6
	return h, s, l
}

// hslToRGB converts an HSL color (h in [0,1], s in [0,1], l in [0,1]) to sRGB.
func hslToRGB(h, s, l float64) (r, g, b float64) {
	if s == 0 {
		// Achromatic.
		return l, l, l
	}
	hue2rgb := func(p, q, t float64) float64 {
		if t < 0 {
			t += 1
		}
		if t > 1 {
			t -= 1
		}
		switch {
		case t < 1.0/6.0:
			return p + (q-p)*6*t
		case t < 1.0/2.0:
			return q
		case t < 2.0/3.0:
			return p + (q-p)*(2.0/3.0-t)*6
		default:
			return p
		}
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	r = hue2rgb(p, q, h+1.0/3.0)
	g = hue2rgb(p, q, h)
	b = hue2rgb(p, q, h-1.0/3.0)
	return r, g, b
}

// EnforceMinContrast nudges the fg hex color's HSL lightness so it meets a
// minimum WCAG contrast ratio against the bg hex color. The value parameter is
// the user-facing normalized knob in [0, 1]:
//
//   - 0.0  = off (no-op; returns fg unchanged)
//   - 0.175 approx = WCAG AA threshold (4.5:1) for normal text
//   - 0.3   approx = WCAG AAA threshold (7.0:1)
//   - 1.0   = maximum (targets 21:1, forces fg toward pure black or white)
//
// The mapping is: wcagTarget = 1.0 + clamp(value, 0, 1) * 20.0
//
// Only the HSL lightness channel is adjusted; hue and saturation are
// preserved, so chromatic colors keep their identity at moderate values.
// At value=1.0 the extremes (L=0 or L=1) are achromatic by definition —
// hue collapse at max value is an acceptable and documented trade-off.
//
// If fg or bg fails parseHexColor (named color, malformed, empty) the
// original fg is returned unchanged, never panicking.
func EnforceMinContrast(fg, bg string, value float64) string {
	if value <= 0 {
		return fg
	}

	fgR, fgG, fgB, ok := parseHexColor(fg)
	if !ok {
		return fg
	}
	bgR, bgG, bgB, ok := parseHexColor(bg)
	if !ok {
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

	lFg := relativeLuminance(fgR, fgG, fgB)
	lBg := relativeLuminance(bgR, bgG, bgB)

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

	h, s, l := rgbToHSL(fgR, fgG, fgB)

	// Binary search over lightness in [0, 1] for 40 iterations. This is more
	// than enough for convergence to well under 0.001 precision in L.
	const iterations = 40
	var lo, hi float64
	if goLighter {
		lo, hi = l, 1.0
	} else {
		lo, hi = 0.0, l
	}

	for i := range iterations {
		_ = i
		mid := (lo + hi) / 2.0
		cr, cg, cb := hslToRGB(h, s, mid)
		lMid := relativeLuminance(cr, cg, cb)
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

	nr, ng, nb := hslToRGB(h, s, finalL)
	return formatHexColor(nr, ng, nb)
}
