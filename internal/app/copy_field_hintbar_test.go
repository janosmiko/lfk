package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOverlayHintBar_CopyFieldPickerActive(t *testing.T) {
	m := copyFieldOpenPicker(t, copyFieldTestModel(t))
	assert.Equal(t, overlayCopyField, m.overlay)
	hints := m.overlayHintBarSelector()
	assert.NotEmpty(t, hints, "selector hint bar must render for the field picker overlay")
	for _, frag := range []string{"filter", "navigate", "copy value", "close"} {
		assert.Contains(t, hints, frag, "field picker hint bar must contain %q", frag)
	}
}
