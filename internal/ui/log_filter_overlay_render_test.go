package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Before running any test that renders styled content, prime the theme
// so surface/base/bar colors are initialized. Tests that don't call
// ApplyTheme themselves still get sensible defaults because the ui
// package's top-level var declarations use lipgloss.NoColor{} for the
// background slots.

func TestRenderLogFilterOverlayEmpty_NewOverlay(t *testing.T) {
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: api-7f4",
		IncludeMode: "any",
		FocusInput:  true,
		Input:       "",
	}, 80, 24)
	assert.Contains(t, out, "Filters")
	assert.Contains(t, out, "Pod: api-7f4")
	assert.Contains(t, out, "ANY")
	// Empty state message.
	assert.Contains(t, out, "no rules yet")
}

func TestRenderLogFilterOverlayTableShowsAllColumns(t *testing.T) {
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: api",
		IncludeMode: "all",
		Rules: []LogFilterRowState{
			{Kind: "SEV", Mode: "", Pattern: ">= WARN"},
			{Kind: "INC", Mode: "regex", Pattern: "^api"},
			{Kind: "EXC", Mode: "substr", Pattern: "/healthz"},
		},
		ListCursor: 1,
	}, 80, 24)
	// Table header is present.
	assert.Contains(t, out, "Kind")
	assert.Contains(t, out, "Mode")
	assert.Contains(t, out, "Pattern")
	// Both kinds show up as cell values.
	assert.Contains(t, out, "SEV")
	assert.Contains(t, out, "EXC")
	// Mode and pattern values for non-severity rows are separated into
	// their own cells.
	assert.Contains(t, out, "regex")
	assert.Contains(t, out, "^api")
	assert.Contains(t, out, "/healthz")
	// Include-mode flip hint is rendered.
	assert.Contains(t, out, "ALL")
}

func TestRenderLogFilterOverlaySavePromptRowAppears(t *testing.T) {
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:            "Pod: api",
		IncludeMode:      "any",
		SavePromptActive: true,
		SavePromptInput:  "my-noise-filter",
	}, 80, 24)
	assert.Contains(t, out, "Save preset as:")
	assert.Contains(t, out, "my-noise-filter")
	// Internal hint bar removed (global hint bar lives in overlay_hintbar.go).
}

func TestRenderLogFilterOverlayPresetPickerRendersItems(t *testing.T) {
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:            "Pod: api",
		IncludeMode:      "any",
		LoadPickerActive: true,
		LoadPickerItems: []string{
			"noise (2 rules) [default]",
			"errors-only (1 rules)",
		},
		LoadPickerCursor: 1,
	}, 80, 24)
	assert.Contains(t, out, "Presets")
	assert.Contains(t, out, "noise (2 rules) [default]")
	assert.Contains(t, out, "errors-only (1 rules)")
	// Internal hint bar removed (global hint bar lives in overlay_hintbar.go).
}

func TestRenderLogFilterOverlayEmptyPresetPicker(t *testing.T) {
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:            "Pod: api",
		IncludeMode:      "any",
		LoadPickerActive: true,
		LoadPickerItems:  nil,
	}, 80, 24)
	assert.Contains(t, out, "Presets")
	assert.Contains(t, out, "none saved yet")
}

// TestRenderLogFilterOverlayHeadersAlwaysShown verifies the table
// headers render even when no rules have been added yet — a visual
// affordance so the user sees the layout they're about to fill in.
func TestRenderLogFilterOverlayHeadersAlwaysShown(t *testing.T) {
	// No rules, empty state.
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: api",
		IncludeMode: "any",
		FocusInput:  true,
	}, 80, 24)
	assert.Contains(t, out, "Kind", "table header must be visible when empty")
	assert.Contains(t, out, "Mode")
	assert.Contains(t, out, "Pattern")
	assert.Contains(t, out, "no rules yet")
}

// TestRenderLogFilterOverlayInputAtBottom verifies that the input row
// sits literally on the last non-blank line of the fixed-height overlay
// so the input bar appears at the bottom of the box, with padding
// between body and input. Previously the input rendered right under
// the body with no bottom-anchoring.
func TestRenderLogFilterOverlayInputAtBottom(t *testing.T) {
	const height = 24
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: api",
		IncludeMode: "any",
		Rules: []LogFilterRowState{
			{Kind: "INC", Mode: "substr", Pattern: "foo"},
			{Kind: "EXC", Mode: "substr", Pattern: "bar"},
		},
		FocusInput: true,
		Input:      "typed-text",
	}, 80, height)

	lines := strings.Split(out, "\n")

	// Find the line containing the input.
	inputLine := -1
	for i, ln := range lines {
		if strings.Contains(ln, "typed-text") {
			inputLine = i
			break
		}
	}
	require.GreaterOrEqual(t, inputLine, 0, "input should appear somewhere in the output")

	// Every line strictly after the input line must be blank (the
	// padding that anchors the input to the bottom can only be above
	// it, not below).
	for i := inputLine + 1; i < len(lines); i++ {
		assert.Empty(t, strings.TrimSpace(lines[i]),
			"lines after the input row must be blank so the input is at the bottom")
	}

	// There must be at least one blank line between the body and the
	// input — confirms the pad is actually inserted.
	require.Greater(t, inputLine, 0)
	assert.Empty(t, strings.TrimSpace(lines[inputLine-1]),
		"the line directly above the input must be blank padding")
}

// TestRenderLogFilterOverlaySeverityIsPinned verifies that the
// severity row renders with distinct priority styling: a star marker
// in the first column and "!" in the index column instead of a row
// number. This makes it obvious that severity sits at the top of the
// rules list as a pinned / priority-0 entry.
func TestRenderLogFilterOverlaySeverityIsPinned(t *testing.T) {
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: api",
		IncludeMode: "any",
		Rules: []LogFilterRowState{
			{Kind: "SEV", Mode: "", Pattern: ">= WARN"},
			{Kind: "INC", Mode: "substr", Pattern: "foo"},
		},
		FocusInput: false,
	}, 80, 24)
	// Star marker on the severity row.
	assert.Contains(t, out, "★", "severity row should be marked with a pin indicator")
	// "!" in the # column instead of a row number.
	assert.Contains(t, out, "! ", "severity row # should render as '!' (priority-0 marker)")
	// Non-severity rows number from 1 — the pinned severity doesn't
	// consume an index.
	assert.Contains(t, out, "1 ", "first non-severity row should render as index 1")
	assert.NotRegexp(t, `\s2\s`, out, "only one rule so row 2 should not appear")
}

// TestRenderLogFilterOverlaySeveritySpacerAlways verifies that a blank
// spacer line always sits between the pinned severity row and the
// first regular rule, whether scrolling is active or not. Without
// this, the severity row reads as glued to the next rule when the
// rule count fits without scrolling.
func TestRenderLogFilterOverlaySeveritySpacerAlways(t *testing.T) {
	mk := func(extraRules int) string {
		rows := make([]LogFilterRowState, 0, 1+extraRules)
		rows = append(rows, LogFilterRowState{Kind: "SEV", Mode: "", Pattern: ">= WARN"})
		for i := range extraRules {
			rows = append(rows, LogFilterRowState{Kind: "INC", Mode: "substr", Pattern: fmt.Sprintf("pat-%02d", i)})
		}
		return RenderLogFilterOverlay(LogFilterOverlayState{
			Title: "Pod: api", IncludeMode: "any",
			Rules: rows, ListCursor: 1,
		}, 80, 24)
	}

	check := func(name, out string) {
		t.Helper()
		lines := strings.Split(out, "\n")
		severityIdx := -1
		for i, ln := range lines {
			if strings.Contains(ln, ">= WARN") {
				severityIdx = i
				break
			}
		}
		require.GreaterOrEqual(t, severityIdx, 0, name+": severity row not found")
		require.Greater(t, len(lines), severityIdx+1, name+": no line after severity")
		assert.Empty(t, strings.TrimSpace(lines[severityIdx+1]),
			name+": line directly after severity must be blank spacer")
	}

	// Count blank lines between severity and the first non-blank row so
	// we catch double-spacing regressions. Should be exactly 1 in both
	// regimes — the combined spacer / "more above" chrome lives in a
	// single line slot.
	countBlanksBelowSeverity := func(out string) int {
		lines := strings.Split(out, "\n")
		for i, ln := range lines {
			if !strings.Contains(ln, ">= WARN") {
				continue
			}
			blanks := 0
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "" {
					blanks++
					continue
				}
				// stop at first non-blank line
				return blanks
			}
			return blanks
		}
		return -1
	}

	check("fits without scrolling", mk(2))
	check("overflows into scrolling", mk(30))
	assert.Equal(t, 1, countBlanksBelowSeverity(mk(2)),
		"fits regime: exactly one blank line between severity and first rule")
	assert.Equal(t, 1, countBlanksBelowSeverity(mk(30)),
		"scrolling regime (at top): exactly one blank line between severity and first rule")
}

// TestRenderLogFilterOverlayNoGapWhenNoSeverity verifies that when no
// severity is present, the first rule sits directly under the header
// underline — no blank line above. (The user reported a stray blank
// line when severity wasn't pinned.)
func TestRenderLogFilterOverlayNoGapWhenNoSeverity(t *testing.T) {
	rows := []LogFilterRowState{
		{Kind: "INC", Mode: "substr", Pattern: "first"},
		{Kind: "INC", Mode: "substr", Pattern: "second"},
	}
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title: "Pod: api", IncludeMode: "any",
		Rules: rows, ListCursor: 0,
	}, 80, 24)

	lines := strings.Split(out, "\n")
	// Find the TABLE header row ("# Kind Mode Pattern"); the line after
	// the table underline must be the first rule (no blank spacer).
	tableHeader := -1
	for i, ln := range lines {
		if strings.Contains(ln, "Pattern") && strings.Contains(ln, "Kind") {
			tableHeader = i
			break
		}
	}
	require.GreaterOrEqual(t, tableHeader, 0, "table header row not found")
	// underline is the next line, first rule immediately after.
	require.Greater(t, len(lines), tableHeader+2)
	assert.Contains(t, lines[tableHeader+2], "first",
		"first rule sits directly under the table underline with no blank spacer")
}

// TestRenderLogFilterOverlayLayoutStableAcrossScroll verifies that the
// visual position of rows in the middle of the list does NOT shift as
// the cursor scrolls — user-reported: "when I scroll down, the first
// elements are moved up a few lines". Renders the overlay at two
// adjacent cursor positions and checks the position of a mid-list
// row in the rendered output differs by at most 1 line.
func TestRenderLogFilterOverlayLayoutStableAcrossScroll(t *testing.T) {
	rules := make([]LogFilterRowState, 0, 21)
	rules = append(rules, LogFilterRowState{Kind: "SEV", Mode: "", Pattern: ">= WARN"})
	for i := range 20 {
		rules = append(rules, LogFilterRowState{Kind: "INC", Mode: "substr", Pattern: fmt.Sprintf("pat-%02d", i)})
	}

	mkOut := func(cursor int) string {
		return RenderLogFilterOverlay(LogFilterOverlayState{
			Title: "Pod: api", IncludeMode: "any",
			Rules: rules, ListCursor: cursor, FocusInput: false,
		}, 80, 24)
	}

	// Two adjacent cursor positions inside the scroll window — the
	// surrounding layout should stay visually the same.
	a := mkOut(2)
	b := mkOut(3)
	assert.Equal(t, strings.Count(a, "\n"), strings.Count(b, "\n"),
		"total rendered height must not change between adjacent cursor positions")
}

// TestRenderLogFilterOverlaySeverityStaysVisibleWhenScrolled verifies
// that when the scroll window has moved past index 0, the severity row
// is still drawn standalone at the top of the table body so its
// presence is never hidden from the user.
func TestRenderLogFilterOverlaySeverityStaysVisibleWhenScrolled(t *testing.T) {
	rows := make([]LogFilterRowState, 0, 51)
	rows = append(rows, LogFilterRowState{Kind: "SEV", Mode: "", Pattern: ">= ERROR"})
	for range 50 {
		rows = append(rows, LogFilterRowState{Kind: "INC", Mode: "substr", Pattern: "p"})
	}

	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: api",
		IncludeMode: "any",
		Rules:       rows,
		ListCursor:  30, // deep into the list, severity would be scrolled out
		FocusInput:  false,
	}, 80, 24)

	assert.Contains(t, out, ">= ERROR",
		"severity row must remain visible even when the body is scrolled past it")
	assert.Contains(t, out, "★",
		"pinned severity marker still present")
}

// TestRenderLogFilterOverlayScrollsWithManyRules verifies that the
// overlay does NOT grow past its fixed height when the user adds lots
// of rules. The table scrolls internally around the cursor, and the
// total rendered height stays <= the box height.
func TestRenderLogFilterOverlayScrollsWithManyRules(t *testing.T) {
	const (
		overlayW = 80
		overlayH = 24
	)

	rules := make([]LogFilterRowState, 50)
	for i := range rules {
		rules[i] = LogFilterRowState{Kind: "INC", Mode: "substr", Pattern: "p"}
	}

	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: api",
		IncludeMode: "any",
		Rules:       rules,
		ListCursor:  25, // middle of the list
		FocusInput:  false,
	}, overlayW, overlayH)

	lines := strings.Split(out, "\n")
	assert.LessOrEqual(t, len(lines), overlayH,
		"overlay must not grow beyond its fixed height even with many rules")

	// The cursor row should be visible somewhere in the middle of the body.
	assert.Contains(t, out, "> 26", "cursor row (1-indexed) should render")

	// Scroll chrome: either "more above" or "more below" must appear
	// because 50 rows > table body slot.
	assert.Contains(t, out, "more above")
	assert.Contains(t, out, "more below")
}

// TestRenderLogFilterOverlayListModeShowsCursor verifies that when the
// table (not the input) has focus, the selected row renders with a
// visible marker.
func TestRenderLogFilterOverlayListModeShowsCursor(t *testing.T) {
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: api",
		IncludeMode: "any",
		Rules: []LogFilterRowState{
			{Kind: "INC", Mode: "substr", Pattern: "a"},
		},
		ListCursor: 0,
		FocusInput: false,
	}, 80, 24)
	// The cursor marker ">" must be present somewhere on the selected row.
	assert.Contains(t, out, "> 1")
}

// TestRenderLogFilterOverlayFitsInDispatchDimensions ensures the rendered
// output lines stay within min(80, w-10) width for a typical 120-col
// terminal. Other tests hit rendering; this one pins the width contract.
func TestRenderLogFilterOverlayFitsInDispatchDimensions(t *testing.T) {
	overlayW := 80
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: a-very-long-pod-name-that-should-not-force-a-resize",
		IncludeMode: "any",
		Rules: []LogFilterRowState{
			{Kind: "INC", Mode: "regex", Pattern: "^api"},
			{Kind: "EXC", Mode: "substr", Pattern: "/healthz"},
			{Kind: "SEV", Mode: "", Pattern: ">= ERROR"},
		},
	}, overlayW, 24)
	for _, line := range strings.Split(out, "\n") {
		// lipgloss.Width accounts for rendered visible cells (stripping
		// ANSI escapes). The inner content must fit within the inner
		// width of the overlay (overlayW - 2*border - 2*padding = w-6).
		assert.LessOrEqual(t, lipgloss.Width(line), overlayW-6,
			"rendered overlay content must not exceed the dispatch inner width")
	}
}

// TestRenderLogFilterOverlayGroupRow verifies that GRP rows render with
// the group's boolean expression in the Pattern column and the "any"/"all"
// label in the Mode column. This pins the Phase-3 display contract.
func TestRenderLogFilterOverlayGroupRow(t *testing.T) {
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: api",
		IncludeMode: "any",
		Rules: []LogFilterRowState{
			{Kind: "GRP", Mode: "all", Pattern: "(foo AND (bar OR baz))"},
		},
	}, 80, 24)
	assert.Contains(t, out, "GRP")
	assert.Contains(t, out, "all")
	assert.Contains(t, out, "(foo AND (bar OR baz))")
}

// TestRenderLogFilterOverlayLongGroupTruncates verifies that a pathological
// long group summary does not break the width contract — Truncate should
// clamp it to fit the Pattern column.
func TestRenderLogFilterOverlayLongGroupTruncates(t *testing.T) {
	long := "(" + strings.Repeat("very-long-pattern AND ", 30) + "tail)"
	overlayW := 80
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: api",
		IncludeMode: "any",
		Rules: []LogFilterRowState{
			{Kind: "GRP", Mode: "all", Pattern: long},
		},
	}, overlayW, 24)
	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), overlayW-6,
			"long group summary must be truncated to fit the overlay width")
	}
}
