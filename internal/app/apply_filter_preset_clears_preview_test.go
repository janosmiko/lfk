package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// When a filter preset matches zero items, applyFilterPreset must drop
// the right-pane state that was loaded for the previously-selected
// resource. Otherwise the children pane keeps rendering the prior pod's
// containers / YAML / metrics while the middle column stares back empty.
//
// loadPreview() short-circuits on `sel == nil`, so it can't do this
// cleanup itself; applyFilterPreset has to clear the state explicitly
// before it returns.
func TestApplyFilterPresetEmptyMatchClearsPreview(t *testing.T) {
	m := baseModelOverlay()
	m.middleItems = []model.Item{
		{Name: "pod-1", Status: "Running"},
		{Name: "pod-2", Status: "Running"},
	}
	// Simulate the right pane state populated for the previously
	// selected pod-1: containers list, YAML body, metrics bar, events
	// rollup, and a resource-tree map.
	m.rightItems = []model.Item{{Name: "container-a", Kind: "Container"}}
	m.previewYAML = "kind: Pod\nmetadata:\n  name: pod-1\n"
	m.metricsContent = "cpu: 10m"
	m.previewEventsContent = "Warning: ImagePullBackOff"
	m.resourceTree = &model.ResourceNode{Kind: "Pod", Name: "pod-1"}
	// previewLoading lingers true from the in-flight cursor-change
	// load that pod-1 triggered before the preset was applied — the
	// resourcesLoadedMsg handler is gated on requestGen and will drop
	// the response after our bump, so applyFilterPreset has to flip
	// previewLoading off itself or the renderer keeps spinning.
	m.previewLoading = true
	prevGen := m.requestGen

	preset := FilterPreset{
		Name: "Failing",
		MatchFn: func(item model.Item) bool {
			return item.Status == "Failed"
		},
	}

	result, _ := m.applyFilterPreset(preset)
	rm := result.(Model)

	assert.Empty(t, rm.middleItems,
		"sanity: preset matches zero pods")
	assert.Greater(t, rm.requestGen, prevGen,
		"empty preset must bump requestGen so the in-flight prior preview "+
			"is discarded by its gen-gated handler")
	assert.False(t, rm.previewLoading,
		"empty preset must reset previewLoading — the in-flight load that "+
			"set it true is now stale and won't clear it via the normal path")
	assert.Nil(t, rm.rightItems,
		"empty preset must clear children pane (rightItems)")
	assert.Empty(t, rm.previewYAML,
		"empty preset must clear cached YAML preview")
	assert.Empty(t, rm.metricsContent,
		"empty preset must clear pinned metrics footer")
	assert.Empty(t, rm.previewEventsContent,
		"empty preset must clear preview events rollup")
	assert.Nil(t, rm.resourceTree,
		"empty preset must clear resource map tree")
}

// Symmetric check: when the preset DOES match, applyFilterPreset must
// not pre-emptively wipe rightItems — the existing flow expects the
// follow-up loadPreview to swap them in for the new selection, and a
// pre-clear here would replace a smooth transition with a flash of
// "No resources found".
func TestApplyFilterPresetNonEmptyMatchKeepsPreviewBuffer(t *testing.T) {
	m := baseModelOverlay()
	m.middleItems = []model.Item{
		{Name: "pod-1", Status: "Running"},
		{Name: "pod-2", Status: "Failed"},
	}
	prior := []model.Item{{Name: "container-a", Kind: "Container"}}
	m.rightItems = prior

	preset := FilterPreset{
		Name: "Failing",
		MatchFn: func(item model.Item) bool {
			return item.Status == "Failed"
		},
	}

	result, _ := m.applyFilterPreset(preset)
	rm := result.(Model)

	assert.Len(t, rm.middleItems, 1, "sanity: pod-2 matched")
	assert.Equal(t, prior, rm.rightItems,
		"non-empty preset must NOT clear rightItems pre-emptively — "+
			"loadPreview swaps them in for the new selection")
}
