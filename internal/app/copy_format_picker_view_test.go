package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestRenderOverlayCopyFormat_ShowsAllFormats(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	content, w, h := m.renderOverlayCopyFormat()
	assert.NotEmpty(t, content, "render returns non-empty content")
	assert.Greater(t, w, 0, "width must be positive")
	assert.Greater(t, h, 0, "height must be positive")
	// All three format labels should appear in the rendered output.
	assert.Contains(t, content, "YAML")
	assert.Contains(t, content, "JSON")
	assert.Contains(t, content, "Table")
	// Hints live on the bottom hint bar, not inside the overlay.
	assert.NotContains(t, content, "apply")
	assert.NotContains(t, content, "cancel")
	assert.NotContains(t, content, "shortcut")
}

func TestRenderOverlayCopyFormat_ShowsSingleFormatAtClusters(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "kind-1"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	content, _, _ := m.renderOverlayCopyFormat()
	assert.Contains(t, content, "Table")
	assert.Contains(t, content, "[t]", "Table row marker present")
	assert.NotContains(t, content, "[y]", "YAML row must not appear at cluster level")
	assert.NotContains(t, content, "[j]", "JSON row must not appear at cluster level")
}

func TestRenderOverlayCopyFormat_NoActivePickerReturnsEmpty(t *testing.T) {
	m := baseExplorerModel()
	// Picker never opened
	content, w, h := m.renderOverlayCopyFormat()
	assert.Empty(t, content)
	assert.Equal(t, 0, w)
	assert.Equal(t, 0, h)
}
