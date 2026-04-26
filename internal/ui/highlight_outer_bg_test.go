package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

// When a search highlight is applied to a substring inside text that's
// later wrapped by an outer style with its own background (cursor row,
// category bar, parent-column highlight), the inner highlight's
// trailing reset wipes the outer background for the rest of the line.
// The fix re-asserts the outer style's open codes after each inner
// reset so the outer bg comes back for the post-match segment.
//
// Concrete shape:
//   broken: "<outerOpen>before<innerOpen>MATCH<innerClose> after<outerClose>"
//                                                          ^ no bg
//   fixed:  "<outerOpen>before<innerOpen>MATCH<innerClose><outerOpen> after<outerClose>"
//                                                                    ^ outer bg restored

func TestHighlightMatchStyledOver_RestoresOuterBackgroundAfterMatch(t *testing.T) {
	// Force ANSI so styles emit real codes in this test.
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(originalProfile) })
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)

	outer := lipgloss.NewStyle().
		Background(lipgloss.Color("4")).
		Foreground(lipgloss.Color("15"))
	inner := lipgloss.NewStyle().
		Background(lipgloss.Color("11")).
		Foreground(lipgloss.Color("0")).
		Bold(true)

	got := outer.Render(HighlightMatchStyledOver("before MATCH after", "MATCH", inner, outer))

	// Extract the outer style's open codes by rendering a marker.
	outerOpen := styleOpenCodes(outer)
	assert.NotEmpty(t, outerOpen, "test setup: outer style must produce open codes")

	// The post-match plain segment must be preceded by the outer's
	// open codes (re-assertion after the inner reset). If the bug is
	// unfixed, " after" sits naked between the inner reset and the
	// outer's terminal close.
	matchEndIdx := strings.Index(got, "MATCH") + len("MATCH")
	tail := got[matchEndIdx:]
	assert.Contains(t, tail, outerOpen,
		"after the inner reset, the outer style's open codes must be "+
			"re-asserted so the post-match segment keeps the outer background")
}

func TestHighlightMatchStyledOver_NoMatchReturnsLineUntouched(t *testing.T) {
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(originalProfile) })
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)

	outer := lipgloss.NewStyle().Background(lipgloss.Color("4"))
	inner := lipgloss.NewStyle().Background(lipgloss.Color("11"))

	got := HighlightMatchStyledOver("no match here", "zzz", inner, outer)
	assert.Equal(t, "no match here", got,
		"with no matches, the function must return the line unchanged "+
			"so the caller's outer wrapping handles the row entirely")
}

func TestHighlightMatchStyledOver_EmptyQueryReturnsLine(t *testing.T) {
	outer := lipgloss.NewStyle()
	inner := lipgloss.NewStyle()
	assert.Equal(t, "anything", HighlightMatchStyledOver("anything", "", inner, outer))
}

// TestRenderOverPrestyled_NoEscFragmentation guards against the bug
// where an outer style was applied to an already-highlighted line via
// lipgloss.Render: lipgloss treats every byte of the embedded ANSI
// sequence as content and wraps each individually, producing
// "\x1b\x1b[0m..." doubled-ESC streams that NO_COLOR / ANSI-profile
// terminals display as literal "[1;7mNetw[0m..." text and that throw
// off the visible-width calculation so the line wraps to the next row.
//
// RenderOverPrestyled emits a single SGR open + content + reset
// sequence, leaving inner highlights intact.
func TestRenderOverPrestyled_NoEscFragmentation(t *testing.T) {
	for _, p := range []struct {
		name string
		prof termenv.Profile
	}{
		{"TrueColor", termenv.TrueColor},
		{"ANSI256", termenv.ANSI256},
		{"ANSI", termenv.ANSI}, // the NO_COLOR-mode profile
	} {
		t.Run(p.name, func(t *testing.T) {
			originalProfile := lipgloss.DefaultRenderer().ColorProfile()
			t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(originalProfile) })
			lipgloss.DefaultRenderer().SetColorProfile(p.prof)

			// Mimic the user's report: a Bold+Underline category bar
			// (the NO_COLOR shape of CategoryBarStyle) wrapping a line
			// that was first run through HighlightMatchStyledOver to
			// highlight the search match "net" inside "Networking".
			bar := lipgloss.NewStyle().Bold(true).Underline(true)
			hl := lipgloss.NewStyle().Bold(true).Reverse(true)

			pre := HighlightMatchStyledOver("Networking", "net", hl, bar)
			got := RenderOverPrestyled(pre, bar)

			// No two consecutive ESC bytes (the smoking gun for the
			// per-char fragmentation produced by bar.Render(pre)).
			assert.NotContains(t, got, "\x1b\x1b",
				"output must not contain doubled-ESC sequences (NO_COLOR repro: %q)", got)

			// Visible width must equal the literal "Networking" length.
			assert.Equal(t, lipgloss.Width("Networking"), lipgloss.Width(got),
				"visible width must match the unstyled string; got %q", got)

			// Sanity: the inner highlight ("net" → reverse) is still
			// present as a single embedded sequence, not fragmented.
			assert.Contains(t, got, hl.Render("Net"),
				"inner highlight must survive intact inside the outer wrap")
		})
	}
}

// Multiple matches: every inner reset must be followed by an outer
// re-assertion, not just the first one.
func TestHighlightMatchStyledOver_MultipleMatchesAllRestored(t *testing.T) {
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(originalProfile) })
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)

	outer := lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15"))
	inner := lipgloss.NewStyle().Background(lipgloss.Color("11")).Foreground(lipgloss.Color("0")).Bold(true)

	got := outer.Render(HighlightMatchStyledOver("aMa bMb cMc", "M", inner, outer))

	outerOpen := styleOpenCodes(outer)
	// Each "M" produces an inner Render with its own reset. Count
	// outer-open occurrences inside the rendered string — there
	// should be at least one before the first match (outer prepend)
	// plus one re-assertion per match.
	occurrences := strings.Count(got, outerOpen)
	assert.GreaterOrEqual(t, occurrences, 4,
		"outer open codes should appear at the start plus once after "+
			"each of the 3 inner highlight resets (got %d)", occurrences)
}
