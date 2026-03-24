package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- RenderHelpScreen ---

func TestRenderHelpScreen_DefaultState(t *testing.T) {
	// No filter: should contain keybindings content.
	result := RenderHelpScreen(80, 40, 0, "")
	assert.Contains(t, result, "Keybindings")
}

func TestRenderHelpScreen_FilterApplied(t *testing.T) {
	// Filter applied: content should contain matching entries.
	result := RenderHelpScreen(80, 40, 0, "nav")
	assert.Contains(t, result, "nav")
}

func TestRenderHelpScreen_FilterFiltersEntries(t *testing.T) {
	// With a filter that matches specific entries, unmatched entries should be excluded.
	full := RenderHelpScreen(120, 100, 0, "")
	filtered := RenderHelpScreen(120, 100, 0, "bookmark")

	// The filtered output should be shorter than the full output.
	fullLines := strings.Split(full, "\n")
	filteredLines := strings.Split(filtered, "\n")
	assert.Less(t, len(filteredLines), len(fullLines),
		"filtered help should have fewer lines than unfiltered")

	// Filtered output should contain "Bookmark" section.
	assert.Contains(t, filtered, "Bookmark")
}
