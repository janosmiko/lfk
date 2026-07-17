package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	t.Cleanup(func() {
		ConfigNoColor = prevNoColor
		ConfigTransparentBg = prevTransparent
		ConfigMinContrastRatio = prevContrast
		ConfigDimOverlay = prevDim
		IconMode = prevIcon
		ConfigRowStatusTint = prevRowTint
	})
	ConfigNoColor = false
	ConfigTransparentBg = false
	ConfigMinContrastRatio = 0
	ConfigDimOverlay = true
	IconMode = "unicode"
	ConfigRowStatusTint = RowStatusTintForeground
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
`)
	LoadConfig(path)

	assert.True(t, ConfigNoColor, "no_color")
	assert.True(t, ConfigTransparentBg, "transparent_background")
	assert.InDelta(t, 0.5, ConfigMinContrastRatio, 1e-9, "min_contrast_ratio")
	assert.False(t, ConfigDimOverlay, "dim_overlay")
	assert.Equal(t, "simple", IconMode, "icons")
	assert.Equal(t, RowStatusTintOff, ConfigRowStatusTint, "row_status_tint")
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
