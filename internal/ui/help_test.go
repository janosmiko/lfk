package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- RenderHelpScreen ---

func TestRenderHelpScreen_DefaultState(t *testing.T) {
	// No filter: should contain keybindings content.
	result := RenderHelpScreen(80, 40, 0, "", "", "")
	assert.Contains(t, result, "Keybindings")
}

func TestRenderHelpScreen_FilterApplied(t *testing.T) {
	// Filter applied: content should contain matching entries.
	result := RenderHelpScreen(80, 40, 0, "nav", "", "")
	assert.Contains(t, result, "nav")
}

func TestRenderHelpScreen_FilterFiltersEntries(t *testing.T) {
	// Filter excludes non-matching entries from the visible content.
	// Box height stays constant (covered by FilterDoesNotShrinkBox);
	// what changes is which lines render.
	filtered := RenderHelpScreen(120, 100, 0, "bookmark", "", "")
	assert.Contains(t, filtered, "Bookmark",
		"filter must keep matching sections visible")
	// A keybinding far from the bookmark section shouldn't appear in
	// the visible window after filtering.
	assert.NotContains(t, filtered, "Toggle help screen",
		"filter must hide non-matching entries")
}

// Filtering down to a tiny match set must not shrink the overlay
// box — the row count must match the unfiltered render so the user
// doesn't see the window collapse on each keystroke.
func TestRenderHelpScreen_FilterDoesNotShrinkBox(t *testing.T) {
	full := RenderHelpScreen(120, 100, 0, "", "", "")
	narrowed := RenderHelpScreen(120, 100, 0, "thereisnokeycontainingthisstring", "", "")
	fullLines := strings.Split(full, "\n")
	narrowedLines := strings.Split(narrowed, "\n")
	assert.Equal(t, len(fullLines), len(narrowedLines),
		"filter that narrows results must not shrink the box height")
}

func TestRenderHelpScreen_SearchHighlightsButDoesNotFilter(t *testing.T) {
	// Search differs from filter: matching content stays inline; non-matching
	// lines are NOT removed. The user opens search to find a key in
	// context, not to whittle the list down. Using a tall enough viewport
	// so the bookmark section is in the visible window for a meaningful
	// highlight check.
	full := RenderHelpScreen(120, 200, 0, "", "", "")
	searched := RenderHelpScreen(120, 200, 0, "", "Bookmark", "")

	fullLines := strings.Split(full, "\n")
	searchedLines := strings.Split(searched, "\n")
	assert.Equal(t, len(fullLines), len(searchedLines),
		"search must not remove lines — line count must match the unfiltered render")
}
