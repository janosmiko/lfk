package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- RenderLogContainerSelectOverlay ---

func TestRenderLogContainerSelectOverlay(t *testing.T) {
	t.Run("empty items shows message", func(t *testing.T) {
		ResetOverlayContainerScroll()
		result := RenderLogContainerSelectOverlay(nil, 0, nil, "", false, false)
		assert.Contains(t, result, "Filter Containers")
		assert.Contains(t, result, "No matching containers")
	})

	t.Run("items with all-containers virtual item", func(t *testing.T) {
		ResetOverlayContainerScroll()
		items := []model.Item{
			{Name: "All Containers", Status: "all"},
			{Name: "nginx", Status: "Running"},
			{Name: "sidecar", Status: "Running"},
		}
		result := RenderLogContainerSelectOverlay(items, 0, nil, "", false, false)
		assert.Contains(t, result, "Filter Containers")
		assert.Contains(t, result, "All Containers")
		assert.Contains(t, result, "nginx")
		assert.Contains(t, result, "sidecar")
		// All Containers should have check mark when no specific selection.
		assert.Contains(t, result, "\u2713")
	})

	t.Run("selected containers show check marks", func(t *testing.T) {
		ResetOverlayContainerScroll()
		items := []model.Item{
			{Name: "All Containers", Status: "all"},
			{Name: "nginx", Status: "Running"},
			{Name: "sidecar", Status: "Running"},
		}
		result := RenderLogContainerSelectOverlay(items, 1, []string{"nginx"}, "", false, false)
		assert.Contains(t, result, "nginx")
		assert.Contains(t, result, "\u2713")
	})

	t.Run("cursor highlights item", func(t *testing.T) {
		ResetOverlayContainerScroll()
		items := []model.Item{
			{Name: "container1"},
			{Name: "container2"},
		}
		result := RenderLogContainerSelectOverlay(items, 1, nil, "", false, false)
		assert.Contains(t, result, "container2")
	})

	t.Run("filter mode active shows cursor block", func(t *testing.T) {
		ResetOverlayContainerScroll()
		items := []model.Item{{Name: "nginx"}}
		result := RenderLogContainerSelectOverlay(items, 0, nil, "ngi", true, false)
		assert.Contains(t, result, "ngi")
		assert.Contains(t, result, "\u2588")
	})

	t.Run("filter text without filter mode", func(t *testing.T) {
		ResetOverlayContainerScroll()
		items := []model.Item{{Name: "nginx"}}
		result := RenderLogContainerSelectOverlay(items, 0, nil, "ngi", false, false)
		assert.Contains(t, result, "/ ngi")
	})

	t.Run("no filter shows placeholder", func(t *testing.T) {
		ResetOverlayContainerScroll()
		items := []model.Item{{Name: "nginx"}}
		result := RenderLogContainerSelectOverlay(items, 0, nil, "", false, false)
		assert.Contains(t, result, "/ to filter")
	})

	t.Run("hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		ResetOverlayContainerScroll()
		items := []model.Item{{Name: "nginx"}}
		result := RenderLogContainerSelectOverlay(items, 0, nil, "", false, true)
		assert.NotContains(t, result, "P: switch pod")
		assert.NotContains(t, result, "space: select")
		assert.NotContains(t, result, "enter: apply")
		assert.NotContains(t, result, "esc: close")
	})
}
