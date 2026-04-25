package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// User repro: open namespace selector, type a filter that matches no
// namespace → overlay shows "Loading namespaces..." instead of "No
// matching namespaces". Cause: filteredOverlayItems returned a nil
// slice (var filtered []model.Item never appended to) when no rows
// matched. RenderNamespaceOverlay treats nil as "still fetching" and
// empty as "no match".
//
// Fix contract: filteredOverlayItems must return a non-nil empty
// slice when a filter is active and matches nothing, so the overlay
// can tell the user the filter excluded everything rather than
// looking like a stuck loader.
func TestFilteredOverlayItems_ActiveFilterNoMatchReturnsEmptySlice(t *testing.T) {
	m := baseModelCov()
	m.overlayItems = []model.Item{
		{Name: "default"},
		{Name: "kube-system"},
	}
	m.overlayFilter.Value = "weird-text-no-namespace-has"

	got := m.filteredOverlayItems()
	assert.NotNil(t, got,
		"filter that matches nothing must return an empty slice, not nil — nil collides with the renderer's pre-fetch loader signal")
	assert.Empty(t, got)
}

// Inactive filter still returns the original (which may be nil if the
// fetch hasn't completed yet — the loading state).
func TestFilteredOverlayItems_NoFilterReturnsOriginal(t *testing.T) {
	m := baseModelCov()
	m.overlayItems = nil
	m.overlayFilter.Value = ""

	got := m.filteredOverlayItems()
	assert.Nil(t, got, "no filter + no items yet must preserve nil so renderer shows the loader")
}
