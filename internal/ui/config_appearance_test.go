package ui

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/logger"
)

// snapshotAppearanceGlobals saves and restores the appearance-related runtime
// globals so each test starts from known compiled defaults.
func snapshotAppearanceGlobals(t *testing.T) {
	t.Helper()
	prevNoColor := ConfigNoColor
	prevTransparent := ConfigTransparentBg
	prevContrast := ConfigMinContrastRatio
	prevDim := ConfigDimOverlay
	prevIcon := IconMode
	prevRowTint := ConfigRowStatusTint
	prevLayout := ConfigExplorerLayout
	t.Cleanup(snapshotAllConfigGlobals(t))
	t.Cleanup(func() {
		ConfigNoColor = prevNoColor
		ConfigTransparentBg = prevTransparent
		ConfigMinContrastRatio = prevContrast
		ConfigDimOverlay = prevDim
		IconMode = prevIcon
		ConfigRowStatusTint = prevRowTint
		ConfigExplorerLayout = prevLayout
	})
	ConfigNoColor = false
	ConfigTransparentBg = false
	ConfigMinContrastRatio = 0
	ConfigDimOverlay = true
	IconMode = "unicode"
	ConfigRowStatusTint = RowStatusTintForeground
	ConfigExplorerLayout = LayoutNormal
}

// TestAppearance_GroupApplies verifies the appearance group wires every field
// into its runtime global.
func TestAppearance_GroupApplies(t *testing.T) {
	snapshotAppearanceGlobals(t)

	path := writeConfigFile(t, `appearance:
  no_color: true
  transparent_background: true
  min_contrast_ratio: 0.5
  dim_overlay: false
  icons: simple
  row_status_tint: "off"
  layout: fullscreen
`)
	LoadConfig(path)

	assert.True(t, ConfigNoColor, "no_color")
	assert.True(t, ConfigTransparentBg, "transparent_background")
	assert.InDelta(t, 0.5, ConfigMinContrastRatio, 1e-9, "min_contrast_ratio")
	assert.False(t, ConfigDimOverlay, "dim_overlay")
	assert.Equal(t, "simple", IconMode, "icons")
	assert.Equal(t, RowStatusTintOff, ConfigRowStatusTint, "row_status_tint")
	assert.Equal(t, LayoutFullscreen, ConfigExplorerLayout, "layout")
}

// TestAppearance_GroupOverridesFlatAlias verifies the appearance group wins over
// a flat alias when both are set.
func TestAppearance_GroupOverridesFlatAlias(t *testing.T) {
	snapshotAppearanceGlobals(t)

	path := writeConfigFile(t, `no_color: false
dim_overlay: true
appearance:
  no_color: true
  dim_overlay: false
`)
	LoadConfig(path)

	assert.True(t, ConfigNoColor, "appearance.no_color wins over flat")
	assert.False(t, ConfigDimOverlay, "appearance.dim_overlay wins over flat")
}

// TestAppearance_OmittedKeepsFlatAlias verifies a flat alias still applies when
// the appearance group is present but omits that field.
func TestAppearance_OmittedKeepsFlatAlias(t *testing.T) {
	snapshotAppearanceGlobals(t)

	path := writeConfigFile(t, `no_color: true
appearance:
  dim_overlay: false
`)
	LoadConfig(path)

	assert.True(t, ConfigNoColor, "flat no_color preserved when group omits it")
	assert.False(t, ConfigDimOverlay, "appearance.dim_overlay applied")
}

// TestRowStatusTint_InvalidFallsBack verifies an unknown row_status_tint value
// is rejected and the compiled default (foreground) stays active.
func TestRowStatusTint_InvalidFallsBack(t *testing.T) {
	snapshotAppearanceGlobals(t)

	path := writeConfigFile(t, "row_status_tint: neon\n")
	LoadConfig(path)

	assert.Equal(t, RowStatusTintForeground, ConfigRowStatusTint, "invalid value falls back to default")
}

// TestRowStatusTint_AppearanceGroupWins verifies the appearance group value
// beats the flat alias.
func TestRowStatusTint_AppearanceGroupWins(t *testing.T) {
	snapshotAppearanceGlobals(t)

	path := writeConfigFile(t, "row_status_tint: background\nappearance:\n  row_status_tint: \"off\"\n")
	LoadConfig(path)

	assert.Equal(t, RowStatusTintOff, ConfigRowStatusTint, "appearance.row_status_tint wins over flat")
}

// TestExplorerLayout_ValidValues applies each valid layout value.
func TestExplorerLayout_ValidValues(t *testing.T) {
	snapshotAppearanceGlobals(t)

	for _, layout := range []string{LayoutNormal, LayoutSidebarHidden, LayoutFullscreen} {
		t.Run(layout, func(t *testing.T) {
			snapshotAppearanceGlobals(t)
			path := writeConfigFile(t, "appearance:\n  layout: "+layout+"\n")
			LoadConfig(path)
			assert.Equal(t, layout, ConfigExplorerLayout, "layout %s applied", layout)
		})
	}
}

// TestExplorerLayout_InvalidFallsBack verifies an unknown layout value is
// rejected and the compiled default stays active.
func TestExplorerLayout_InvalidFallsBack(t *testing.T) {
	snapshotAppearanceGlobals(t)

	path := writeConfigFile(t, "appearance:\n  layout: \"banana\"\n")
	LoadConfig(path)

	assert.Equal(t, LayoutNormal, ConfigExplorerLayout, "invalid layout falls back to default")
}

// TestExplorerLayout_ResetsOnReload verifies a reload that no longer sets
// appearance.layout returns to the compiled default instead of keeping the
// previous load's value.
func TestExplorerLayout_ResetsOnReload(t *testing.T) {
	snapshotAppearanceGlobals(t)

	path := writeConfigFile(t, "appearance:\n  layout: fullscreen\n")
	LoadConfig(path)
	assert.Equal(t, LayoutFullscreen, ConfigExplorerLayout, "first load applies fullscreen")

	path = writeConfigFile(t, "appearance:\n  no_color: true\n")
	LoadConfig(path)
	assert.Equal(t, LayoutNormal, ConfigExplorerLayout, "reload without layout resets to default")
}

// TestExplorerLayout_ResetsOnReloadWithInvalidValue verifies a reload with an
// invalid appearance.layout resets to the compiled default (rather than
// keeping a previous load's value) and still warns.
func TestExplorerLayout_ResetsOnReloadWithInvalidValue(t *testing.T) {
	snapshotAppearanceGlobals(t)

	path := writeConfigFile(t, "appearance:\n  layout: fullscreen\n")
	LoadConfig(path)
	assert.Equal(t, LayoutFullscreen, ConfigExplorerLayout, "first load applies fullscreen")

	origLogger := logger.Logger
	var buf bytes.Buffer
	logger.Logger = slog.New(slog.NewTextHandler(&buf, nil))
	t.Cleanup(func() { logger.Logger = origLogger })

	path = writeConfigFile(t, "appearance:\n  layout: \"banana\"\n")
	LoadConfig(path)

	assert.Equal(t, LayoutNormal, ConfigExplorerLayout, "reload with invalid layout resets to default")
	assert.Contains(t, buf.String(), "appearance.layout", "warning must name the offending key")
}

// TestExplorerLayout_FlatAlias applies the deprecated flat layout key.
func TestExplorerLayout_FlatAlias(t *testing.T) {
	snapshotAppearanceGlobals(t)

	path := writeConfigFile(t, "layout: sidebar_hidden\n")
	LoadConfig(path)

	assert.Equal(t, LayoutSidebarHidden, ConfigExplorerLayout, "flat layout key applied")
}

// TestExplorerLayout_CaseAndWarning covers case-insensitive matching and the
// unknown-value warning, asserting the warning names the key and accepted
// values without echoing what the user typed.
func TestExplorerLayout_CaseAndWarning(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantValue string
		wantWarn  bool
	}{
		{
			name:      "uppercase value accepted",
			raw:       "FULLSCREEN",
			wantValue: LayoutFullscreen,
			wantWarn:  false,
		},
		{
			name:      "unknown value keeps default and warns",
			raw:       "hunter2-secret-layout",
			wantValue: LayoutNormal,
			wantWarn:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snapshotAppearanceGlobals(t)

			origLogger := logger.Logger
			var buf bytes.Buffer
			logger.Logger = slog.New(slog.NewTextHandler(&buf, nil))
			t.Cleanup(func() { logger.Logger = origLogger })

			path := writeConfigFile(t, "appearance:\n  layout: \""+tc.raw+"\"\n")
			LoadConfig(path)

			assert.Equal(t, tc.wantValue, ConfigExplorerLayout)

			out := buf.String()
			if tc.wantWarn {
				assert.Contains(t, out, "appearance.layout", "warning must name the offending key")
				assert.NotContains(t, out, tc.raw, "raw config value must not be logged")
			} else {
				assert.NotContains(t, out, "appearance.layout", "no warning expected for a valid value")
			}
		})
	}
}
