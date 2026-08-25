package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// podWithDetails is a pod row carrying the detail columns the middle list
// already loaded, hovered while its container preview is still on the wire.
func podWithDetails() model.Item {
	return model.Item{
		Name: "api-1", Namespace: "default", Kind: "Pod", Status: "Running",
		Columns: []model.KeyValue{{Key: "Node", Value: "ip-10-0-0-1"}},
	}
}

// The silent watch-tick refresh of the pod list clears previewLoading every
// interval while the hovered pod's container preview is still in flight. The
// right pane then read an empty rightItems as "this pod has no containers" and
// flashed "No resources found" once a second. It must show the pod's own
// details instead, the way every other kind already does.
func TestRightPanePrefersPodDetailsOverTheEmptyState(t *testing.T) {
	m := basePush80Model()
	m.middleItems = []model.Item{podWithDetails()}
	m.setCursor(0)
	m.rightItems = nil
	m.previewLoading = false

	out := stripANSI(m.renderRightResources(60, 20))

	assert.NotContains(t, out, "No resources found",
		"a pod whose containers have not arrived is not a pod without containers")
	assert.Contains(t, out, "ip-10-0-0-1", "the pane must show the pod's own details")
}

// Same hole one level down: the owned view routes a Pod with no children to the
// same empty state.
func TestOwnedPanePrefersPodDetailsOverTheEmptyState(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{podWithDetails()}
	m.setCursor(0)
	m.rightItems = nil
	m.previewLoading = false

	out := stripANSI(m.renderRightOwned(60, 20))

	assert.NotContains(t, out, "No resources found")
	assert.Contains(t, out, "ip-10-0-0-1")
}

// Nothing is known about the pod yet, so the pane has no details to fall back
// on and must keep the loader rather than claim the pod has no containers.
func TestRightPaneKeepsTheLoaderWhenThePodHasNoDetailsYet(t *testing.T) {
	m := basePush80Model()
	m.middleItems = []model.Item{{Name: "api-1", Namespace: "default", Kind: "Pod"}}
	m.setCursor(0)
	m.rightItems = nil
	m.previewLoading = true

	out := stripANSI(m.renderRightResources(60, 20))

	assert.True(t, strings.Contains(out, "Loading"), "expected a loader, got %q", out)
}
