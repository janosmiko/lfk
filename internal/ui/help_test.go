package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/stretchr/testify/assert"
)

// --- RenderHelpScreen ---

func TestRenderHelpScreen_DefaultState(t *testing.T) {
	// No filter, no search: help line should contain navigation hints.
	ti := textinput.New()
	result := RenderHelpScreen(80, 40, 0, "", false, &ti)
	assert.Contains(t, result, "Keybindings")
	assert.Contains(t, result, "j/k")
	assert.Contains(t, result, "scroll")
	assert.Contains(t, result, "^d/^u")
	assert.Contains(t, result, "half-page")
	assert.Contains(t, result, "/")
	assert.Contains(t, result, "search")
	assert.Contains(t, result, "close")
}

func TestRenderHelpScreen_SearchActive(t *testing.T) {
	// Active search: help line shows "search: " + input view.
	ti := textinput.New()
	ti.SetValue("nav")
	result := RenderHelpScreen(80, 40, 0, "nav", true, &ti)
	assert.Contains(t, result, "search:")
	assert.Contains(t, result, "nav")
}

func TestRenderHelpScreen_FilterApplied(t *testing.T) {
	// Filter applied but not actively searching: help line shows filter + edit/close hints.
	ti := textinput.New()
	result := RenderHelpScreen(80, 40, 0, "nav", false, &ti)
	assert.Contains(t, result, "filter:")
	assert.Contains(t, result, "nav")
	assert.Contains(t, result, "edit")
	assert.Contains(t, result, "close")
}

func TestRenderHelpScreen_FilterFiltersEntries(t *testing.T) {
	// With a filter that matches specific entries, unmatched entries should be excluded.
	ti := textinput.New()
	full := RenderHelpScreen(120, 100, 0, "", false, &ti)
	filtered := RenderHelpScreen(120, 100, 0, "bookmark", false, &ti)

	// The filtered output should be shorter than the full output.
	fullLines := strings.Split(full, "\n")
	filteredLines := strings.Split(filtered, "\n")
	assert.Less(t, len(filteredLines), len(fullLines),
		"filtered help should have fewer lines than unfiltered")

	// Filtered output should contain "Bookmark" section.
	assert.Contains(t, filtered, "Bookmark")
}

func TestHelpContentLineCount(t *testing.T) {
	// Without filter, should return a positive count.
	count := HelpContentLineCount("")
	assert.Greater(t, count, 0, "unfiltered help should have content lines")

	// With a restrictive filter, should return fewer lines.
	filteredCount := HelpContentLineCount("bookmark")
	assert.Less(t, filteredCount, count, "filtered help should have fewer lines")
}
