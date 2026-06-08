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
	t.Cleanup(func() {
		ConfigNoColor = prevNoColor
		ConfigTransparentBg = prevTransparent
		ConfigMinContrastRatio = prevContrast
		ConfigDimOverlay = prevDim
		IconMode = prevIcon
	})
	ConfigNoColor = false
	ConfigTransparentBg = false
	ConfigMinContrastRatio = 0
	ConfigDimOverlay = true
	IconMode = "unicode"
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
`)
	LoadConfig(path)

	assert.True(t, ConfigNoColor, "no_color")
	assert.True(t, ConfigTransparentBg, "transparent_background")
	assert.InDelta(t, 0.5, ConfigMinContrastRatio, 1e-9, "min_contrast_ratio")
	assert.False(t, ConfigDimOverlay, "dim_overlay")
	assert.Equal(t, "simple", IconMode, "icons")
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
