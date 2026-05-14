package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestOverlayHintBar_PickerActive(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	// overlay should now be overlayCopyFormat
	assert.Equal(t, overlayCopyFormat, m.overlay)
	hints := m.overlayHintBarSelector()
	assert.NotEmpty(t, hints, "selector hint bar must render for the picker overlay")
	// Spot-check key entries
	for _, frag := range []string{"navigate", "shortcut", "apply", "cancel"} {
		assert.Contains(t, hints, frag, "picker hint bar must contain %q", frag)
	}
}
