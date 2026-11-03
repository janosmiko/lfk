package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A cancelled union set namespace picker must not leak into the next ordinary
// namespace selection.
func TestCloseCurrentOverlay_ClearsPendingUnionSet(t *testing.T) {
	m := basePush80Model()
	m.overlay = overlayNamespace
	m.pendingUnionSetName = "prod"

	r, _ := m.closeCurrentOverlay()

	assert.Empty(t, r.(Model).pendingUnionSetName)
}
