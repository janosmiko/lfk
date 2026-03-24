package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- RenderFilterPresetOverlay ---

func TestRenderFilterPresetOverlay(t *testing.T) {
	t.Run("empty presets shows message", func(t *testing.T) {
		result := RenderFilterPresetOverlay(nil, 0, "")
		assert.Contains(t, result, "Quick Filters")
		assert.Contains(t, result, "No filter presets available")
	})

	t.Run("presets rendered with names and descriptions", func(t *testing.T) {
		presets := []FilterPresetEntry{
			{Name: "Running", Description: "Show running pods", Key: "r"},
			{Name: "Failed", Description: "Show failed pods", Key: "f"},
		}
		result := RenderFilterPresetOverlay(presets, 0, "")
		assert.Contains(t, result, "Quick Filters")
		assert.Contains(t, result, "Running")
		assert.Contains(t, result, "Show running pods")
		assert.Contains(t, result, "Failed")
		assert.Contains(t, result, "[r]")
		assert.Contains(t, result, "[f]")
	})

	t.Run("cursor highlights selected item", func(t *testing.T) {
		presets := []FilterPresetEntry{
			{Name: "A", Description: "desc a", Key: "a"},
			{Name: "B", Description: "desc b", Key: "b"},
		}
		result := RenderFilterPresetOverlay(presets, 1, "")
		assert.Contains(t, result, "B")
	})

	t.Run("active preset shows check mark", func(t *testing.T) {
		presets := []FilterPresetEntry{
			{Name: "Running", Description: "desc", Key: "r"},
		}
		result := RenderFilterPresetOverlay(presets, 0, "Running")
		assert.Contains(t, result, "\u2713")
	})

	t.Run("non-active preset no check mark", func(t *testing.T) {
		presets := []FilterPresetEntry{
			{Name: "Running", Description: "desc", Key: "r"},
		}
		result := RenderFilterPresetOverlay(presets, 0, "Other")
		// The check mark should not appear for the Running preset.
		assert.NotContains(t, result, "\u2713 ")
	})

	t.Run("footer hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		presets := []FilterPresetEntry{{Name: "P", Description: "d", Key: "p"}}
		result := RenderFilterPresetOverlay(presets, 0, "")
		assert.NotContains(t, result, "enter: apply")
		assert.NotContains(t, result, "esc: close")
	})
}

// --- RenderRBACOverlay ---

func TestRenderRBACOverlay(t *testing.T) {
	t.Run("basic rendering with mixed results", func(t *testing.T) {
		results := []RBACCheckEntry{
			{Verb: "get", Allowed: true},
			{Verb: "create", Allowed: false},
			{Verb: "delete", Allowed: true},
		}
		result := RenderRBACOverlay(results, "pods")
		assert.Contains(t, result, "RBAC Permissions: pods")
		assert.Contains(t, result, "get")
		assert.Contains(t, result, "create")
		assert.Contains(t, result, "delete")
		// Check mark for allowed.
		assert.Contains(t, result, "\u2713")
		// Cross mark for denied.
		assert.Contains(t, result, "\u2717")
	})

	t.Run("empty results shows title", func(t *testing.T) {
		result := RenderRBACOverlay(nil, "secrets")
		assert.Contains(t, result, "RBAC Permissions: secrets")
		// Hint moved to status bar.
		assert.NotContains(t, result, "Press any key to close")
	})

	t.Run("all allowed", func(t *testing.T) {
		results := []RBACCheckEntry{
			{Verb: "get", Allowed: true},
			{Verb: "list", Allowed: true},
		}
		result := RenderRBACOverlay(results, "configmaps")
		assert.Contains(t, result, "\u2713")
		assert.NotContains(t, result, "\u2717")
	})
}

// --- RenderBatchLabelOverlay ---

func TestRenderBatchLabelOverlay(t *testing.T) {
	t.Run("add labels mode", func(t *testing.T) {
		result := RenderBatchLabelOverlay(0, "app=nginx", false)
		assert.Contains(t, result, "Add Labels")
		assert.Contains(t, result, "Enter key=value:")
		assert.Contains(t, result, "app=nginx")
		// Tab hint moved to status bar.
	})

	t.Run("remove labels mode", func(t *testing.T) {
		result := RenderBatchLabelOverlay(0, "app", true)
		assert.Contains(t, result, "Remove Labels")
		assert.Contains(t, result, "Enter key to remove:")
		assert.Contains(t, result, "app")
	})

	t.Run("add annotations mode", func(t *testing.T) {
		result := RenderBatchLabelOverlay(1, "note=test", false)
		assert.Contains(t, result, "Add Annotations")
		assert.Contains(t, result, "Enter key=value:")
	})

	t.Run("remove annotations mode", func(t *testing.T) {
		result := RenderBatchLabelOverlay(1, "note", true)
		assert.Contains(t, result, "Remove Annotations")
		assert.Contains(t, result, "Enter key to remove:")
	})

	t.Run("input cursor shown", func(t *testing.T) {
		result := RenderBatchLabelOverlay(0, "test", false)
		assert.Contains(t, result, "\u2588")
	})

	t.Run("footer hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		result := RenderBatchLabelOverlay(0, "", false)
		assert.NotContains(t, result, "Enter: apply")
		assert.NotContains(t, result, "Esc: cancel")
	})
}

// --- RenderEventTimelineOverlay ---

func TestRenderEventTimelineOverlay(t *testing.T) {
	t.Run("empty events shows message", func(t *testing.T) {
		result := RenderEventTimelineOverlay(nil, "my-pod", 0, 80, 30)
		assert.Contains(t, result, "Event Timeline - my-pod")
		assert.Contains(t, result, "No events found")
		// Hint moved to status bar.
	})

	t.Run("events rendered with reason and message", func(t *testing.T) {
		events := []EventTimelineEntry{
			{
				Timestamp: time.Now().Add(-5 * time.Minute),
				Type:      "Normal",
				Reason:    "Pulled",
				Message:   "Successfully pulled image",
				Source:    "kubelet",
				Count:     1,
			},
			{
				Timestamp: time.Now().Add(-10 * time.Minute),
				Type:      "Warning",
				Reason:    "BackOff",
				Message:   "Back-off restarting failed container",
				Source:    "kubelet",
				Count:     3,
			},
		}
		result := RenderEventTimelineOverlay(events, "my-pod", 0, 100, 30)
		assert.Contains(t, result, "Event Timeline - my-pod")
		assert.Contains(t, result, "Pulled")
		assert.Contains(t, result, "Successfully pulled image")
		assert.Contains(t, result, "BackOff")
		assert.Contains(t, result, "Back-off restarting failed container")
		assert.Contains(t, result, "kubelet")
		assert.Contains(t, result, "2 events")
	})

	t.Run("count > 1 shows repeat indicator", func(t *testing.T) {
		events := []EventTimelineEntry{
			{
				Timestamp: time.Now().Add(-1 * time.Minute),
				Type:      "Normal",
				Reason:    "Killing",
				Message:   "Stopping container",
				Count:     5,
			},
		}
		result := RenderEventTimelineOverlay(events, "pod", 0, 80, 20)
		assert.Contains(t, result, "x5")
	})

	t.Run("involved name different from resource shows it", func(t *testing.T) {
		events := []EventTimelineEntry{
			{
				Timestamp:    time.Now().Add(-1 * time.Minute),
				Type:         "Normal",
				Reason:       "Scheduled",
				Message:      "Assigned to node",
				InvolvedName: "other-pod",
				InvolvedKind: "Pod",
				Count:        1,
			},
		}
		result := RenderEventTimelineOverlay(events, "my-pod", 0, 80, 20)
		assert.Contains(t, result, "Pod/other-pod")
	})

	t.Run("scroll hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		events := make([]EventTimelineEntry, 50)
		for i := range events {
			events[i] = EventTimelineEntry{
				Timestamp: time.Now().Add(-time.Duration(i) * time.Minute),
				Type:      "Normal",
				Reason:    "Test",
				Message:   "msg",
				Count:     1,
			}
		}
		result := RenderEventTimelineOverlay(events, "pod", 0, 80, 15)
		assert.NotContains(t, result, "j/k: scroll")
	})
}
