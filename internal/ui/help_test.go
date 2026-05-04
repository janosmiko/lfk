package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

// Searching for a digit must not corrupt the rendered output.
//
// The previous "/" search path ran HighlightMatchStyled on the
// already-styled, already-truncated help lines. SGR sequences carry
// digits as parameters (e.g. \x1b[33;1m for bold + yellow fg), so a
// byte-indexed search for "1" matched bytes inside the escape
// sequences and the highlight wrapper split them into fragments
// terminals rendered as literal "[33;" / ";1m" text on screen.
//
// Asserts: stripping ANSI from the rendered output produces the same
// visible characters whether the user typed a digit query or no
// query at all. Search adds highlight color but never visible chars.
func TestRenderHelpScreen_DigitSearchDoesNotLeakEscapeFragments(t *testing.T) {
	original := lipgloss.DefaultRenderer().ColorProfile()
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(original)
		ConfigNoColor = originalNoColor
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	ApplyTheme(DefaultTheme())
	// ApplyTheme can re-detect/restore the color profile from
	// originalColorProfile (theme.go:109-110), so re-force TrueColor
	// here. Without this, the test runs under the harness's default
	// stripped profile, lipgloss emits no SGR digits, and the
	// regression we're guarding against — digit-query corruption
	// inside SGR sequences — could not occur in the first place.
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)

	plain := RenderHelpScreen(120, 200, 0, "", "", "", -1)

	// Each digit query exercises a different byte that the old code
	// could match inside SGR parameters. "1" was the most visible in
	// the user report; the others guard against regressions in the
	// other common SGR digits.
	for _, q := range []string{"1", "0", "5", "33"} {
		t.Run("query="+q, func(t *testing.T) {
			searched := RenderHelpScreen(120, 200, 0, "", q, "", -1)

			assert.Equal(t, ansi.Strip(plain), ansi.Strip(searched),
				"digit search must not change the visible characters in the help screen — only the highlight color")

			assert.NotContains(t, searched, "\x1b\x1b",
				"rendered output must not contain doubled-ESC bytes (smoking gun for fragmented SGR sequences)")
		})
	}
}

// helpRecomputeMatches in the app layer iterates over BuildHelpLines
// and counts hits via MatchLine. When BuildHelpLines returned styled
// strings, a "1" query would substring-match bytes inside SGR codes
// (e.g. the "1" in \x1b[33;1m) and inflate the match count to roughly
// every styled row, pointing n/N at rows with no visible "1".
//
// Asserts: the plain lines BuildHelpLines now returns contain no ESC
// bytes, so MatchLine sees only the visible characters.
//
// Forces a TrueColor profile so lipgloss actually emits SGR escapes
// for any styling it would apply — without this, the test harness's
// default stripped profile makes lipgloss render plain text anyway
// and the assertion would pass vacuously even if BuildHelpLines went
// back to returning styled output.
// Within a single help section, every entry row's key column must have
// the same display width so descriptions align in a clean column.
// Regression: previously the column was a fixed 14 chars, so any key
// longer than that (e.g. "Ctrl+F / Ctrl+B / PgDn / PgUp") overflowed
// and pushed its description right of the rest, breaking alignment.
func TestBuildHelpSpecs_KeysAlignWithinSection(t *testing.T) {
	specs := buildHelpSpecs("", "")

	var currentSection string
	widths := make([]int, 0, 16)

	check := func() {
		if len(widths) <= 1 {
			return
		}
		first := widths[0]
		for _, w := range widths {
			if w != first {
				t.Errorf("section %q: key widths inconsistent (got %v, want all %d)",
					currentSection, widths, first)
				return
			}
		}
	}

	for _, s := range specs {
		switch s.kind {
		case helpLineSectionHeader:
			check()
			currentSection = s.text
			widths = widths[:0]
		case helpLineEntry:
			widths = append(widths, lipgloss.Width(s.key))
		}
	}
	check()
}

func TestBuildHelpLines_ReturnsPlainText(t *testing.T) {
	original := lipgloss.DefaultRenderer().ColorProfile()
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(original)
		ConfigNoColor = originalNoColor
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	ApplyTheme(DefaultTheme())
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)

	lines := BuildHelpLines("", "")
	for i, line := range lines {
		assert.NotContains(t, line, "\x1b",
			"BuildHelpLines must return plain text (no ANSI escapes) — line %d: %q", i, line)
	}
}
