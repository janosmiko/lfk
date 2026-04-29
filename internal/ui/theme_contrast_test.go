package ui

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- parseHexColor tests ----

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantR  float64
		wantG  float64
		wantB  float64
		wantOK bool
	}{
		{"black 6-digit", "#000000", 0, 0, 0, true},
		{"white 6-digit", "#ffffff", 1, 1, 1, true},
		{"white uppercase", "#FFFFFF", 1, 1, 1, true},
		{"mixed case", "#FF0080", 1, 0, float64(0x80) / 255.0, true},
		{"3-digit black", "#000", 0, 0, 0, true},
		{"3-digit white", "#fff", 1, 1, 1, true},
		{"3-digit mixed", "#f08", 1, 0, float64(0x88) / 255.0, true},
		{"tokyonight primary", "#7aa2f7", float64(0x7a) / 255.0, float64(0xa2) / 255.0, float64(0xf7) / 255.0, true},
		{"empty string", "", 0, 0, 0, false},
		{"no hash", "ff0000", 0, 0, 0, false},
		{"named color", "red", 0, 0, 0, false},
		{"too short", "#ff00", 0, 0, 0, false},
		{"too long", "#ff000000", 0, 0, 0, false},
		{"invalid hex", "#gggggg", 0, 0, 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, g, b, ok := parseHexColor(tc.input)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.InDelta(t, tc.wantR, r, 1e-10)
				assert.InDelta(t, tc.wantG, g, 1e-10)
				assert.InDelta(t, tc.wantB, b, 1e-10)
			}
		})
	}
}

// ---- formatHexColor tests ----

func TestFormatHexColor(t *testing.T) {
	tests := []struct {
		name    string
		r, g, b float64
		want    string
	}{
		{"black", 0, 0, 0, "#000000"},
		{"white", 1, 1, 1, "#ffffff"},
		{"red", 1, 0, 0, "#ff0000"},
		{"green", 0, 1, 0, "#00ff00"},
		{"blue", 0, 0, 1, "#0000ff"},
		{"clamps below 0", -1, 0, 0, "#000000"},
		{"clamps above 1", 2, 0, 0, "#ff0000"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatHexColor(tc.r, tc.g, tc.b)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ---- relativeLuminance tests ----

func TestRelativeLuminance(t *testing.T) {
	tests := []struct {
		name    string
		r, g, b float64
		want    float64
		delta   float64
	}{
		{"pure black", 0, 0, 0, 0, 1e-10},
		{"pure white", 1, 1, 1, 1, 1e-10},
		{"pure red", 1, 0, 0, 0.2126, 1e-4},
		{"pure green", 0, 1, 0, 0.7152, 1e-4},
		{"pure blue", 0, 0, 1, 0.0722, 1e-4},
		// mid-grey: each channel 0.5 -> linearized to ~0.2140
		// luminance = 0.2126*0.2140 + 0.7152*0.2140 + 0.0722*0.2140 ~= 0.2140
		{"mid grey 0.5", 0.5, 0.5, 0.5, 0.2140, 5e-4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := relativeLuminance(tc.r, tc.g, tc.b)
			assert.InDelta(t, tc.want, got, tc.delta, "luminance of (%g,%g,%g)", tc.r, tc.g, tc.b)
		})
	}
}

// ---- contrastRatio tests ----

func TestContrastRatio(t *testing.T) {
	tests := []struct {
		name      string
		fg        string
		bg        string
		wantRatio float64
		delta     float64
	}{
		{"black on white = 21", "#000000", "#ffffff", 21.0, 0.01},
		{"white on black = 21", "#ffffff", "#000000", 21.0, 0.01},
		{"same color = 1", "#7aa2f7", "#7aa2f7", 1.0, 0.01},
		// WCAG reference: #595959 on #ffffff = 7.0:1 (AAA)
		// actual value varies slightly; verify it is in expected range
		{"tokyonight text on base", "#c0caf5", "#24283b", 9.0, 1.5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fgR, fgG, fgB, ok := parseHexColor(tc.fg)
			require.True(t, ok)
			bgR, bgG, bgB, ok := parseHexColor(tc.bg)
			require.True(t, ok)
			l1 := relativeLuminance(fgR, fgG, fgB)
			l2 := relativeLuminance(bgR, bgG, bgB)
			got := contrastRatio(l1, l2)
			assert.InDelta(t, tc.wantRatio, got, tc.delta, "contrast ratio %s on %s", tc.fg, tc.bg)
			// ratio must always be >= 1
			assert.GreaterOrEqual(t, got, 1.0)
		})
	}
}

// ---- RGB/HSL roundtrip tests ----

func TestRGBHSLRoundtrip(t *testing.T) {
	// For chromatic colors the round-trip should be lossless.
	testColors := []struct{ r, g, b float64 }{
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
		{float64(0x7a) / 255.0, float64(0xa2) / 255.0, float64(0xf7) / 255.0}, // #7aa2f7
		{float64(0xbb) / 255.0, float64(0x9a) / 255.0, float64(0xf7) / 255.0}, // #bb9af7
	}
	for _, c := range testColors {
		h, s, l := rgbToHSL(c.r, c.g, c.b)
		r2, g2, b2 := hslToRGB(h, s, l)
		assert.InDelta(t, c.r, r2, 1e-9, "R roundtrip for (%g,%g,%g)", c.r, c.g, c.b)
		assert.InDelta(t, c.g, g2, 1e-9, "G roundtrip for (%g,%g,%g)", c.r, c.g, c.b)
		assert.InDelta(t, c.b, b2, 1e-9, "B roundtrip for (%g,%g,%g)", c.r, c.g, c.b)
	}
}

// ---- EnforceMinContrast tests ----

func TestEnforceMinContrastOff(t *testing.T) {
	// value=0 is "off" -- must return fg unchanged even when contrast is terrible.
	fg := "#4e5575" // tokyonight dimmed -- dark color against dark base
	bg := "#24283b" // tokyonight base
	result := EnforceMinContrast(fg, bg, 0)
	assert.Equal(t, fg, result, "value=0 must return fg unchanged")
}

func TestEnforceMinContrastNegative(t *testing.T) {
	fg := "#4e5575"
	bg := "#24283b"
	result := EnforceMinContrast(fg, bg, -0.5)
	assert.Equal(t, fg, result, "negative value must return fg unchanged")
}

func TestEnforceMinContrastAlreadyMeets(t *testing.T) {
	// White on black already has contrast 21:1. A low value (<1.0) should leave it unchanged.
	fg := "#ffffff"
	bg := "#000000"
	result := EnforceMinContrast(fg, bg, 0.1) // target ~3.0 -- already met by 21:1
	assert.Equal(t, fg, result, "already-passing pair should be returned unchanged")
}

func TestEnforceMinContrastLightensAgainstDark(t *testing.T) {
	// Dark bg, mid fg -> fg should be nudged lighter to improve contrast.
	bg := "#1a1b26" // very dark base
	fg := "#4e5575" // dimmed, low contrast against bg

	result := EnforceMinContrast(fg, bg, 0.175) // target ~4.5 (AA)

	require.NotEmpty(t, result)
	// The result must be parseable.
	rR, rG, rB, ok := parseHexColor(result)
	require.True(t, ok, "result must be a valid hex color, got %q", result)

	bgR, bgG, bgB, ok2 := parseHexColor(bg)
	require.True(t, ok2)

	lFg := relativeLuminance(rR, rG, rB)
	lBg := relativeLuminance(bgR, bgG, bgB)
	ratio := contrastRatio(lFg, lBg)
	target := 1.0 + 0.175*20.0
	assert.GreaterOrEqual(t, ratio, target-0.1, "result contrast ratio %g should meet target %g", ratio, target)

	// Verify fg moved lighter (higher luminance) compared to original.
	origR, origG, origB, _ := parseHexColor(fg)
	origL := relativeLuminance(origR, origG, origB)
	assert.GreaterOrEqual(t, lFg, origL, "fg should move to higher luminance (lighter) against dark bg")
}

func TestEnforceMinContrastDarkensAgainstLight(t *testing.T) {
	// Light bg, mid-light fg -> fg should be nudged darker.
	bg := "#ffffff"
	fg := "#c0caf5" // tokyonight text -- light, low contrast against white

	result := EnforceMinContrast(fg, bg, 0.175) // target ~4.5

	require.NotEmpty(t, result)
	rR, rG, rB, ok := parseHexColor(result)
	require.True(t, ok, "result must be a valid hex color, got %q", result)

	bgR, bgG, bgB, _ := parseHexColor(bg)
	lFg := relativeLuminance(rR, rG, rB)
	lBg := relativeLuminance(bgR, bgG, bgB)
	ratio := contrastRatio(lFg, lBg)
	target := 1.0 + 0.175*20.0
	assert.GreaterOrEqual(t, ratio, target-0.1, "result contrast ratio %g should meet target %g", ratio, target)

	// Verify fg moved darker (lower luminance) compared to original.
	origR, origG, origB, _ := parseHexColor(fg)
	origL := relativeLuminance(origR, origG, origB)
	assert.LessOrEqual(t, lFg, origL, "fg should move to lower luminance (darker) against light bg")
}

func TestEnforceMinContrastMaxValue(t *testing.T) {
	// value=1.0 targets ratio 21.0. Against pure black, only white achieves that.
	fg := "#7aa2f7"
	bg := "#000000"
	result := EnforceMinContrast(fg, bg, 1.0)

	require.NotEmpty(t, result)
	rR, rG, rB, ok := parseHexColor(result)
	require.True(t, ok, "result must be a valid hex color, got %q", result)

	bgR, bgG, bgB, _ := parseHexColor(bg)
	lFg := relativeLuminance(rR, rG, rB)
	lBg := relativeLuminance(bgR, bgG, bgB)
	ratio := contrastRatio(lFg, lBg)
	// At value=1.0 the target is 21.0. Pure black bg means only white (ratio ~21)
	// achieves that. We accept the binary search might not hit exact 21 due to
	// floating point, so check the fg is at or near pure white (luminance ~1).
	assert.InDelta(t, 1.0, lFg, 0.01, "fg luminance should be near 1 (white) for max value against black")
	_ = ratio // documented: may not hit exactly 21 due to binary search resolution
}

func TestEnforceMinContrastPreservesHuePartial(t *testing.T) {
	// For a moderate value on a chromatic fg, hue should stay close to input.
	// Acceptable drift: within ~5 degrees.
	// Use a very dark fg against a dark bg so a nudge is always required,
	// but keep value moderate so hue is not fully collapsed.
	fg := "#2e3147" // very dark blue-ish, low contrast against dark base
	bg := "#24283b" // dark base

	origR, origG, origB, _ := parseHexColor(fg)
	origH, _, _ := rgbToHSL(origR, origG, origB)

	result := EnforceMinContrast(fg, bg, 0.175) // AA nudge

	if result == fg {
		t.Skip("already meets target, hue preservation trivially holds")
	}

	rR, rG, rB, ok := parseHexColor(result)
	require.True(t, ok)
	newH, _, _ := rgbToHSL(rR, rG, rB)

	// Compute angular difference in degrees (hue is in [0,1] mapped to [0,360]).
	diff := math.Abs(origH-newH) * 360
	if diff > 180 {
		diff = 360 - diff
	}
	assert.LessOrEqual(t, diff, 5.0, "hue drift should be <= 5 degrees for moderate nudge, got %g", diff)
}

// TestEnforceMinContrastPreservesFgBgRelationship reproduces the bug where
// raising the contrast ratio above 0.3 on the default Tokyonight Storm
// SelectedFg/SelectedBg pair pushed fg toward pure white, collapsing the
// visual relationship the theme designer picked (dark fg on medium bg).
//
// Root cause: the nudge direction was chosen from the bg's luminance
// (`lBg < 0.5`), which for a mid-luminance bg like `#7aa2f7` (L ≈ 0.35)
// flipped the fg toward white even though going darker was the side with
// enough headroom to reach the target ratio.
//
// The fix: pick direction from the existing fg vs bg relationship, so the
// designer's intent (dark-on-light or light-on-dark) is preserved.
// TestApplyThemePreservesParentHighlightReadability reproduces a bug where
// raising min_contrast_ratio above 0.3 on the default theme made the LEFT
// (parent) pane's selected-row text invisible.
//
// Root cause: ParentHighlightStyle renders t.Text on top of t.Border. The
// mutator was enforcing Border → Base (so Border, used as a decorative
// column-border color, meets contrast against the base background), which
// pushed Border's luminance way up. But Border is ALSO the background for
// the parent-pane selection highlight — and once Border's luminance matches
// Text's, the highlighted text collapses into its own background.
//
// Border's dual role (decorative fg + parent-highlight bg) means it
// shouldn't be subject to fg-readability enforcement. The fix is to exclude
// Border from the enforced pair list so the designer's chosen subtle
// border color stays put, preserving its role as a bg.
func TestApplyThemePreservesParentHighlightReadability(t *testing.T) {
	prev := ConfigMinContrastRatio
	t.Cleanup(func() {
		ConfigMinContrastRatio = prev
		ApplyTheme(DefaultTheme())
	})

	// Bumping above the reported threshold.
	ConfigMinContrastRatio = 0.5
	ApplyTheme(DefaultTheme())

	tr, tg, tb, okT := parseHexColor(ActiveTheme.Text)
	br, bgG, bb, okB := parseHexColor(ActiveTheme.Border)
	require.True(t, okT, "Text must remain a valid hex after enforcement")
	require.True(t, okB, "Border must remain a valid hex after enforcement")

	ratio := contrastRatio(
		relativeLuminance(tr, tg, tb),
		relativeLuminance(br, bgG, bb),
	)
	// 3.0 is the WCAG AA threshold for large text — a reasonable floor
	// for a bold highlighted row in the parent column. Anything below
	// roughly this number, the parent-pane selection becomes unreadable,
	// which is the user-visible symptom.
	assert.GreaterOrEqual(t, ratio, 3.0,
		"Text on Border (parent highlight pair) dropped to ratio %.2f — below the readability floor. Text=%s Border=%s",
		ratio, ActiveTheme.Text, ActiveTheme.Border)
}

func TestEnforceMinContrastPreservesFgBgRelationship(t *testing.T) {
	cases := []struct {
		name  string
		fg    string
		bg    string
		value float64
	}{
		// The exact pair the user reported. SelectedFg is darker than
		// SelectedBg; the mutator must keep it on the darker side.
		{"selected dark-on-mid default", "#24283b", "#7aa2f7", 0.3},
		// Also verify the mirror case: a light fg on a slightly lighter
		// bg (both above 0.5) must stay on the lighter side rather than
		// being driven toward black.
		{"light-on-lighter", "#e0e0e0", "#f5f5f5", 0.3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fr, fg, fb, _ := parseHexColor(tc.fg)
			br, bgGreen, bb, _ := parseHexColor(tc.bg)
			lFgBefore := relativeLuminance(fr, fg, fb)
			lBg := relativeLuminance(br, bgGreen, bb)
			fgStartedDarker := lFgBefore < lBg

			got := EnforceMinContrast(tc.fg, tc.bg, tc.value)

			gr, gg, gb, ok := parseHexColor(got)
			require.True(t, ok, "mutator must return a valid hex color")
			lFgAfter := relativeLuminance(gr, gg, gb)

			if fgStartedDarker {
				assert.Less(t, lFgAfter, lBg,
					"fg was darker than bg; mutator must keep it darker, not flip it past bg (got %s, luminance %.4f, bg luminance %.4f)",
					got, lFgAfter, lBg)
			} else {
				assert.Greater(t, lFgAfter, lBg,
					"fg was lighter than bg; mutator must keep it lighter, not flip it past bg (got %s, luminance %.4f, bg luminance %.4f)",
					got, lFgAfter, lBg)
			}
		})
	}
}

func TestEnforceMinContrastInvalidInputUnchanged(t *testing.T) {
	tests := []struct {
		name string
		fg   string
		bg   string
	}{
		{"named fg", "red", "#000000"},
		{"malformed fg", "#zzzzzz", "#000000"},
		{"empty fg", "", "#000000"},
		{"invalid bg", "#7aa2f7", "blue"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := EnforceMinContrast(tc.fg, tc.bg, 0.5)
			assert.Equal(t, tc.fg, result, "invalid color should round-trip unchanged")
		})
	}
}

// ---- Config loading tests ----

func TestLoadConfig_MinContrastRatio(t *testing.T) {
	orig := ConfigMinContrastRatio
	t.Cleanup(func() { ConfigMinContrastRatio = orig })

	tests := []struct {
		name       string
		yaml       string
		startValue float64
		want       float64
	}{
		{
			name:       "explicit 0.175 sets AA threshold",
			yaml:       "min_contrast_ratio: 0.175\n",
			startValue: 0,
			want:       0.175,
		},
		{
			name:       "explicit 0.0 sets to off",
			yaml:       "min_contrast_ratio: 0.0\n",
			startValue: 0.5,
			want:       0.0,
		},
		{
			name:       "explicit 1.0 sets maximum",
			yaml:       "min_contrast_ratio: 1.0\n",
			startValue: 0,
			want:       1.0,
		},
		{
			name:       "value above 1.0 clamps to 1.0",
			yaml:       "min_contrast_ratio: 5.0\n",
			startValue: 0,
			want:       1.0,
		},
		{
			name:       "negative value clamps to 0.0",
			yaml:       "min_contrast_ratio: -1.0\n",
			startValue: 0.5,
			want:       0.0,
		},
		{
			name:       "unset leaves default 0",
			yaml:       "colorscheme: dracula\n",
			startValue: 0,
			want:       0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ConfigMinContrastRatio = tc.startValue

			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			err := os.WriteFile(path, []byte(tc.yaml), 0o600)
			require.NoError(t, err)

			LoadConfig(path)
			assert.InDelta(t, tc.want, ConfigMinContrastRatio, 1e-10)
		})
	}
}

func TestApplyThemeHonoursContrastKnob(t *testing.T) {
	origKnob := ConfigMinContrastRatio
	origNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigMinContrastRatio = origKnob
		ConfigNoColor = origNoColor
		ApplyTheme(DefaultTheme())
	})

	// Use a theme with deliberately low contrast: text and base are very close.
	lowContrastTheme := Theme{
		Primary:    "#7aa2f7",
		Secondary:  "#9ece6a",
		Text:       "#30344a", // near-dark on dark base -- low contrast
		SelectedFg: "#24283b",
		SelectedBg: "#7aa2f7",
		Border:     "#4e5575",
		Dimmed:     "#2e3147", // also very low contrast
		Error:      "#f7768e",
		Warning:    "#e0af68",
		Purple:     "#bb9af7",
		Base:       "#24283b",
		BarBg:      "#313446",
		Surface:    "#2a2e40",
	}

	ConfigNoColor = false

	// Without the knob: ActiveTheme.Text should still be near the input.
	ConfigMinContrastRatio = 0
	ApplyTheme(lowContrastTheme)
	textWithoutKnob := ActiveTheme.Text

	// With the knob at 0.175 (WCAG AA): Text should be lighter.
	ConfigMinContrastRatio = 0.175
	ApplyTheme(lowContrastTheme)
	textWithKnob := ActiveTheme.Text

	// The text color must have changed.
	// (If the input already met target this assertion would fail, but the
	// lowContrastTheme above is designed to be below AA.)
	assert.NotEqual(t, textWithoutKnob, textWithKnob, "text color should shift when contrast knob is active")

	// Verify the new color actually meets the target ratio against Base.
	tR, tG, tB, ok := parseHexColor(textWithKnob)
	require.True(t, ok, "shifted text color must be a valid hex")
	bR, bG, bB, ok2 := parseHexColor(lowContrastTheme.Base)
	require.True(t, ok2)

	lFg := relativeLuminance(tR, tG, tB)
	lBg := relativeLuminance(bR, bG, bB)
	ratio := contrastRatio(lFg, lBg)
	target := 1.0 + 0.175*20.0
	assert.GreaterOrEqual(t, ratio, target-0.15,
		"shifted text ratio %g should meet WCAG target %g", ratio, target)
}
