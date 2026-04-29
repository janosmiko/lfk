package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

// --- RenderHelpScreen ---

func TestRenderHelpScreen_DefaultState(t *testing.T) {
	// No filter: should contain keybindings content.
	result := RenderHelpScreen(80, 40, 0, "", "", "", -1)
	assert.Contains(t, result, "Keybindings")
}

func TestRenderHelpScreen_FilterApplied(t *testing.T) {
	// Filter applied: content should contain matching entries.
	result := RenderHelpScreen(80, 40, 0, "nav", "", "", -1)
	assert.Contains(t, result, "nav")
}

func TestRenderHelpScreen_FilterFiltersEntries(t *testing.T) {
	// Filter excludes non-matching entries from the visible content.
	// Box height stays constant (covered by FilterDoesNotShrinkBox);
	// what changes is which lines render.
	filtered := RenderHelpScreen(120, 100, 0, "bookmark", "", "", -1)
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
	full := RenderHelpScreen(120, 100, 0, "", "", "", -1)
	narrowed := RenderHelpScreen(120, 100, 0, "thereisnokeycontainingthisstring", "", "", -1)
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
	full := RenderHelpScreen(120, 200, 0, "", "", "", -1)
	searched := RenderHelpScreen(120, 200, 0, "", "Bookmark", "", -1)

	fullLines := strings.Split(full, "\n")
	searchedLines := strings.Split(searched, "\n")
	assert.Equal(t, len(fullLines), len(searchedLines),
		"search must not remove lines — line count must match the unfiltered render")
}

// Current-match line gets a distinct style so the user can see which
// match the next n/N press will move from. Probe across line indices
// to find one that contains the search query (so the swap from
// SearchHighlightStyle → SelectedSearchHighlightStyle on that line
// produces visibly different output).
func TestRenderHelpScreen_CurrentMatchStyledDifferently(t *testing.T) {
	// Tests run without a TTY, so termenv defaults to a stripped color
	// profile and lipgloss drops the foreground/decoration codes that
	// distinguish the two highlight styles — making them render
	// identically. Force the renderer to ANSI mode and re-apply the
	// theme so SelectedSearchHighlightStyle picks up its color codes.
	// Other tests in this package toggle ConfigNoColor; we have to
	// re-apply state defensively at start because Go test ordering
	// inside a package isn't guaranteed and a prior test may have left
	// styles blank.
	original := lipgloss.DefaultRenderer().ColorProfile()
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(original)
		ConfigNoColor = originalNoColor
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)
	ApplyTheme(DefaultTheme())
	// ApplyTheme can re-detect/restore the color profile from
	// originalColorProfile, so re-force ANSI right after.
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)

	withoutCurrent := RenderHelpScreen(120, 200, 0, "", "filter", "", -1)

	// Find a line index where flipping currentMatchLine actually changes
	// the render — i.e. the line contains a "filter" match. The exact
	// index depends on help content ordering; we just need any one.
	totalLines := len(BuildHelpLines("", ""))
	for i := range totalLines {
		withCurrent := RenderHelpScreen(120, 200, 0, "", "filter", "", i)
		if withoutCurrent != withCurrent {
			return // found a difference — contract holds
		}
	}
	t.Fatalf("no line index produced a different render — current-match style is not applied")
}
