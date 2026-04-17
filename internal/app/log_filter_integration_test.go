package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// syntheticLogLines returns a deterministic mix of INFO / ERROR / WARN / healthz /
// plain api log lines used by the end-to-end filter integration test.
func syntheticLogLines() []string {
	return []string{
		"[INFO] api: starting up",        // 0
		"[ERROR] api: failed to connect", // 1
		"GET /healthz 200 OK",            // 2
		"[INFO] api: request ok",         // 3
		"[ERROR] api: panic recovered",   // 4
		"GET /healthz 200 OK",            // 5
		"[WARN] api: slow response",      // 6
		"[INFO] api: idle",               // 7
		"GET /healthz 200 OK",            // 8
		"[ERROR] api: timeout",           // 9
	}
}

// typeIntoFilterInput feeds each rune of s into the overlay key dispatcher
// as an individual KeyRunes message, mirroring what Bubble Tea delivers
// when the user types characters one at a time.
func typeIntoFilterInput(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		nm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = nm
	}
	return m
}

// TestLogFilterOverlayEndToEnd exercises the full filter-modal flow:
//
//  1. Open the filter modal with `f`.
//  2. Type `-healthz` + Enter to add an exclude rule.
//  3. Close the modal with Esc.
//  4. Verify logVisibleIndices, the source-of-truth that RenderLogViewer
//     projects through, does NOT include the `/healthz` line indices.
//  5. Render through RenderLogViewer and verify the string output does
//     not contain any `/healthz` line.
//  6. Add a severity `>error` rule and verify only ERROR lines remain.
//  7. Delete the first rule via `d` and verify `/healthz` comes back but
//     INFO remains filtered by the severity floor.
//
// This is the goal posts for the log-filter feature. It initially fails
// because of wiring bugs in the value-receiver handlers.
func TestLogFilterOverlayEndToEnd(t *testing.T) {
	det, errs := newSeverityDetector(nil)
	require.Empty(t, errs)

	lines := syntheticLogLines()

	m := Model{
		mode:                modeLogs,
		width:               120,
		height:              40,
		logLines:            lines,
		logSeverityDetector: det,
		logIncludeMode:      IncludeAny,
		logFilterEditingIdx: -1,
		actionCtx:           actionContext{kind: "Pod", name: "api"},
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()

	// Step 1: press `f` to open the filter modal (opens in list/nav mode).
	m = m.handleLogKeyF()
	require.Equal(t, overlayLogFilter, m.overlay, "f should open the filter overlay")
	require.True(t, m.logFilterModalOpen)
	require.False(t, m.logFilterFocusInput, "overlay opens in list mode")

	// Press `a` to enter input mode to add a new rule.
	nm0, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = nm0
	require.True(t, m.logFilterFocusInput, "`a` should switch to input mode")

	// Step 2a: type `-healthz` via the overlay dispatcher.
	m = typeIntoFilterInput(t, m, "-healthz")
	require.Equal(t, "-healthz", m.logFilterInput.Value)

	// Step 2b: press Enter to commit the rule.
	nm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm
	require.Len(t, m.logRules, 1, "one rule added after Enter")
	assert.Equal(t, RuleExclude, m.logRules[0].Kind(), "rule should be an exclude")
	assert.Empty(t, m.logFilterInput.Value, "input should be cleared after commit")
	// Enter (commit) returns to list mode — no separate Esc needed to exit
	// input focus.
	require.False(t, m.logFilterFocusInput, "commit should return to list mode")

	// Step 3: close via one Esc from list mode.
	nm, _ = m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm
	assert.Equal(t, overlayNone, m.overlay, "Esc from list mode closes the overlay")
	assert.False(t, m.logFilterModalOpen, "modal should be closed")

	// Step 4: verify the visible-indices slice excludes every /healthz row.
	healthzIndices := map[int]bool{2: true, 5: true, 8: true}
	for _, idx := range m.logVisibleIndices {
		assert.False(t, healthzIndices[idx], "visible index %d should not be a /healthz line", idx)
	}
	// Sanity: at least the non-healthz lines are visible.
	assert.Contains(t, m.logVisibleIndices, 0, "INFO line should remain visible")
	assert.Contains(t, m.logVisibleIndices, 1, "ERROR line should remain visible")

	// Step 5: render through RenderLogViewer and assert the output string
	// does not mention /healthz.
	rendered := ui.RenderLogViewer(
		m.logLines, m.logVisibleIndices,
		m.logScroll, m.width, 30,
		m.logFollow, m.logWrap, m.logLineNumbers, m.logTimestamps, m.logPrevious, m.logHidePrefixes,
		m.logTitle, m.logSearchQuery, m.logSearchInput.Value,
		m.logSearchActive, false, false, m.logHasMoreHistory, m.logLoadingHistory,
		"", false,
		m.logCursor, m.logVisualMode, m.logVisualStart, m.logVisualType,
		m.logVisualCol, m.logVisualCurCol, len(m.logRules), "", "", false,
	)
	assert.NotContains(t, rendered, "/healthz",
		"rendered log viewer must not contain filtered /healthz lines")
	// Sanity: non-excluded content should still render.
	assert.Contains(t, rendered, "starting up")

	// Step 6: reopen the overlay and add a severity floor `>error`.
	m = m.handleLogKeyF()
	// Overlay opens in list mode — press `a` to enter input mode.
	nm, _ = m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = nm
	m = typeIntoFilterInput(t, m, ">error")
	nm, _ = m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm
	require.Len(t, m.logRules, 2, "now two rules: -healthz and >error")

	// Close the modal — commit returned to list mode, so one Esc closes.
	nm, _ = m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm
	assert.Equal(t, overlayNone, m.overlay)

	// With severity >= ERROR AND exclude /healthz, only ERROR lines remain.
	// ERROR indices in the fixture are 1, 4, 9.
	assert.ElementsMatch(t, []int{1, 4, 9}, m.logVisibleIndices,
		"severity floor plus /healthz exclude should leave only ERROR lines")

	// Step 7: reopen and delete the /healthz exclude via `d`.
	m = m.handleLogKeyF()
	// Overlay already opens in list mode — no Tab needed.
	require.False(t, m.logFilterFocusInput, "overlay should open in list mode")
	// Severity is pinned at index 0. The /healthz exclude is at index 1.
	m.logFilterListCursor = 1
	nm, _ = m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = nm
	require.Len(t, m.logRules, 1, "one rule remaining after delete")
	assert.Equal(t, RuleSeverity, m.logRules[0].Kind(), "only severity rule should survive")

	// Close and verify that /healthz lines would come back if severity allowed
	// them. Since /healthz lines have unknown severity, SeverityRule.Allows
	// returns true for them — they should now be visible again.
	nm, _ = m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm
	assert.Equal(t, overlayNone, m.overlay)

	// /healthz lines (unknown severity) pass the severity rule (floor ERROR).
	// WARN (idx 6) is below ERROR so it is filtered out.
	// Expected visible: 1 (ERROR), 2 (healthz), 4 (ERROR), 5 (healthz), 8 (healthz), 9 (ERROR)
	assert.ElementsMatch(t, []int{1, 2, 4, 5, 8, 9}, m.logVisibleIndices,
		"after deleting the exclude rule, /healthz (unknown severity) returns, INFO/WARN stay out")
	// INFO indices (0, 3, 7) and WARN (6) must not be present.
	for _, idx := range []int{0, 3, 6, 7} {
		assert.NotContains(t, m.logVisibleIndices, idx, "line %d should be filtered by severity floor ERROR", idx)
	}

	// Final render must contain at least one /healthz line since the exclude
	// rule was removed.
	rendered2 := ui.RenderLogViewer(
		m.logLines, m.logVisibleIndices,
		m.logScroll, m.width, 30,
		m.logFollow, m.logWrap, m.logLineNumbers, m.logTimestamps, m.logPrevious, m.logHidePrefixes,
		m.logTitle, m.logSearchQuery, m.logSearchInput.Value,
		m.logSearchActive, false, false, m.logHasMoreHistory, m.logLoadingHistory,
		"", false,
		m.logCursor, m.logVisualMode, m.logVisualStart, m.logVisualType,
		m.logVisualCol, m.logVisualCurCol, len(m.logRules), "", "", false,
	)
	assert.True(t, strings.Contains(rendered2, "/healthz"),
		"after deleting the exclude rule, /healthz should render again")
	assert.NotContains(t, rendered2, "starting up", "INFO lines should remain filtered")
}

// TestExcludeSubstringNoMatch reproduces a user-reported bug: adding an
// exclude rule like `-xyz` where the pattern doesn't match ANY line should
// leave all lines visible (nothing to exclude). The buggy behavior was
// that the rendered viewer showed nothing.
func TestExcludeSubstringNoMatch(t *testing.T) {
	det, errs := newSeverityDetector(nil)
	require.Empty(t, errs)

	lines := syntheticLogLines()

	m := Model{
		mode:                modeLogs,
		width:               120,
		height:              40,
		logLines:            lines,
		logSeverityDetector: det,
		logIncludeMode:      IncludeAny,
		logFilterEditingIdx: -1,
		actionCtx:           actionContext{kind: "Pod", name: "api"},
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()

	// Open modal, press `a` to enter input mode, type a nonexistent
	// exclude pattern, press Enter.
	m = m.handleLogKeyF()
	nm0, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = nm0
	m = typeIntoFilterInput(t, m, "-xyz-no-such-pattern")
	nm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm
	require.Len(t, m.logRules, 1)
	assert.Equal(t, RuleExclude, m.logRules[0].Kind())

	// All lines should be visible because nothing matches the exclude.
	assert.Len(t, m.logVisibleIndices, len(lines),
		"exclude with no matches should leave every line visible")

	// Close (Esc × 2: input → list → close).
	for range 2 {
		nm, _ = m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
		m = nm
	}
	require.Equal(t, overlayNone, m.overlay)

	rendered := ui.RenderLogViewer(
		m.logLines, m.logVisibleIndices,
		m.logScroll, m.width, 30,
		m.logFollow, m.logWrap, m.logLineNumbers, m.logTimestamps, m.logPrevious, m.logHidePrefixes,
		m.logTitle, m.logSearchQuery, m.logSearchInput.Value,
		m.logSearchActive, false, false, m.logHasMoreHistory, m.logLoadingHistory,
		"", false,
		m.logCursor, m.logVisualMode, m.logVisualStart, m.logVisualType,
		m.logVisualCol, m.logVisualCurCol, len(m.logRules), "", "", false,
	)
	// Every line should still be present.
	for _, line := range lines {
		assert.Contains(t, rendered, line,
			"exclude with no matches should keep every line in the rendered output")
	}
}

// TestExcludeSubstringWithStreamedLines exercises the incremental-append
// path — rule is added BEFORE log lines arrive, then lines stream in via
// maybeAppendVisibleIndex. This is what happens on a live pod log.
func TestExcludeSubstringWithStreamedLines(t *testing.T) {
	det, errs := newSeverityDetector(nil)
	require.Empty(t, errs)

	m := Model{
		mode:                modeLogs,
		width:               120,
		height:              40,
		logSeverityDetector: det,
		logIncludeMode:      IncludeAny,
		logFilterEditingIdx: -1,
		actionCtx:           actionContext{kind: "Pod", name: "api"},
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()

	// Set up an exclude rule BEFORE any log lines arrive.
	m = m.handleLogKeyF()
	nm0, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = nm0
	m = typeIntoFilterInput(t, m, "-healthz")
	nm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm
	require.Len(t, m.logRules, 1)

	// Close (Esc × 2: input → list → close).
	for range 2 {
		nm, _ = m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
		m = nm
	}
	require.Equal(t, overlayNone, m.overlay)

	// Simulate streamed log lines arriving one by one.
	streamed := []string{
		"[INFO] api: starting up",
		"GET /healthz 200",
		"[ERROR] api: boom",
		"GET /healthz 200",
		"[INFO] api: request served",
	}
	for _, line := range streamed {
		m.logLines = append(m.logLines, line)
		m.maybeAppendVisibleIndex(len(m.logLines) - 1)
	}

	// Only the non-healthz lines should be in the visible set.
	// Expected visible indices: 0, 2, 4 (the three non-healthz lines).
	assert.ElementsMatch(t, []int{0, 2, 4}, m.logVisibleIndices,
		"streamed lines must be evaluated against the active exclude rule")

	rendered := ui.RenderLogViewer(
		m.logLines, m.logVisibleIndices,
		m.logScroll, m.width, 30,
		m.logFollow, m.logWrap, m.logLineNumbers, m.logTimestamps, m.logPrevious, m.logHidePrefixes,
		m.logTitle, m.logSearchQuery, m.logSearchInput.Value,
		m.logSearchActive, false, false, m.logHasMoreHistory, m.logLoadingHistory,
		"", false,
		m.logCursor, m.logVisualMode, m.logVisualStart, m.logVisualType,
		m.logVisualCol, m.logVisualCurCol, len(m.logRules), "", "", false,
	)
	assert.NotContains(t, rendered, "/healthz", "excluded lines should not render")
	assert.Contains(t, rendered, "starting up", "non-excluded lines should render")
	assert.Contains(t, rendered, "boom", "non-excluded lines should render")
}

// TestQuickSeverityCycleKeys exercises the `>` and `<` single-keystroke
// quick filters in the log view: they cycle the severity floor forward
// and backward without opening the filter modal.
func TestQuickSeverityCycleKeys(t *testing.T) {
	det, errs := newSeverityDetector(nil)
	require.Empty(t, errs)

	m := Model{
		mode:                modeLogs,
		width:               120,
		height:              40,
		logLines:            syntheticLogLines(),
		logSeverityDetector: det,
		logIncludeMode:      IncludeAny,
		logFilterEditingIdx: -1,
		actionCtx:           actionContext{kind: "Pod", name: "api"},
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()

	// `>` cycles: off → DEBUG → INFO → WARN → ERROR → off.
	m = m.handleLogKeyCycleSeverityUp()
	require.Len(t, m.logRules, 1)
	assert.Equal(t, SeverityDebug, m.logRules[0].(SeverityRule).Floor)

	m = m.handleLogKeyCycleSeverityUp() // INFO
	m = m.handleLogKeyCycleSeverityUp() // WARN
	assert.Equal(t, SeverityWarn, m.logRules[0].(SeverityRule).Floor)

	m = m.handleLogKeyCycleSeverityUp() // ERROR
	assert.Equal(t, SeverityError, m.logRules[0].(SeverityRule).Floor)

	m = m.handleLogKeyCycleSeverityUp() // off
	assert.Empty(t, m.logRules, "past ERROR should clear the rule")

	// `<` cycles backward: off → ERROR → WARN → INFO → DEBUG → off.
	m = m.handleLogKeyCycleSeverityDown()
	require.Len(t, m.logRules, 1)
	assert.Equal(t, SeverityError, m.logRules[0].(SeverityRule).Floor)

	// ERROR floor hides INFO and WARN; keeps ERROR lines (1, 4, 9) AND
	// Unknown-severity lines (healthz at 2, 5, 8) — SeverityRule.Allows
	// intentionally keeps Unknown (spec §4.4).
	assert.ElementsMatch(t, []int{1, 2, 4, 5, 8, 9}, visibleSevIndices(m))

	m = m.handleLogKeyCycleSeverityDown() // WARN
	assert.Equal(t, SeverityWarn, m.logRules[0].(SeverityRule).Floor)

	// An existing severity rule gets replaced, not duplicated, when the
	// user cycles again. Invariant: at most one SeverityRule at any time.
	for range 20 {
		m = m.handleLogKeyCycleSeverityUp()
		sevCount := 0
		for _, r := range m.logRules {
			if _, ok := r.(SeverityRule); ok {
				sevCount++
			}
		}
		assert.LessOrEqual(t, sevCount, 1, "never more than one severity rule")
	}
}

// visibleSevIndices filters source indices that have ERROR severity.
func visibleSevIndices(m Model) []int {
	out := []int{}
	for _, idx := range m.logVisibleIndices {
		if idx >= 0 && idx < len(m.logLines) {
			out = append(out, idx)
		}
	}
	return out
}

// TestHistoryPrependRebuildsVisibleIndices reproduces: filter is active,
// user scrolls to the top and ctrl-u triggers maybeLoadMoreHistory which
// prepends older lines. Before the fix, scroll/cursor were shifted by
// prepended count (a raw-source delta) even though they're visible-coords
// in filter mode — the viewport ended up past the visible range and
// rendered nothing. Also, logVisibleIndices was stale so historical
// lines never got evaluated against the filter.
func TestHistoryPrependRebuildsVisibleIndices(t *testing.T) {
	det, errs := newSeverityDetector(nil)
	require.Empty(t, errs)

	// Start with some current lines (mix of keep + drop).
	current := []string{
		"[ERROR] api: current 1", // 0 keep
		"GET /healthz 200",       // 1 drop
		"[INFO] api: current 3",  // 2 drop
		"GET /healthz 200",       // 3 drop
		"[ERROR] api: current 5", // 4 keep
	}
	m := Model{
		mode:                modeLogs,
		width:               120,
		height:              40,
		logLines:            current,
		logSeverityDetector: det,
		logIncludeMode:      IncludeAny,
		logFilterEditingIdx: -1,
		actionCtx:           actionContext{kind: "Pod", name: "api"},
	}
	// Exclude /healthz and only keep ERROR and above (via severity).
	exc, _ := NewPatternRule("/healthz", PatternSubstring, true)
	sev := SeverityRule{Floor: SeverityError}
	m.logRules = []Rule{exc, sev}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()

	// User has scrolled cursor to the top of the filtered view (first
	// ERROR line in the current buffer, at source idx 0 / visible idx 0).
	m.logCursor = 0
	m.logScroll = 0

	// Simulate history prepend: 5 older lines (3 ERROR, 2 healthz).
	older := []string{
		"[ERROR] api: old 1", // drop: healthz nope, severity keep
		"GET /healthz 200",   // drop: excluded
		"[ERROR] api: old 3",
		"GET /healthz 200",
		"[ERROR] api: old 5",
	}
	msg := logHistoryMsg{lines: older}
	m = m.updateLogHistory(msg)

	// Visible indices must be rebuilt.  ERROR lines from the prepend are
	// source indices 0, 2, 4 (prepended), then shifted "current" indices.
	// Current ERRORs were at 0 and 4; after prepending 5, they're at 5 and 9.
	// So expected visible: [0, 2, 4, 5, 9].
	assert.Equal(t, []int{0, 2, 4, 5, 9}, m.logVisibleIndices,
		"rebuilt visible indices must include the prepended ERROR lines")

	// Cursor must be re-anchored to the same source line (was visible 0
	// pointing at source 0; now source 5). In the new visible slice, 5 is
	// at visible position 3.
	assert.Equal(t, 3, m.logCursor, "cursor should re-anchor to the same source line after prepend")
}

// TestPinSeverityFirst verifies the severity-pinning contract:
// SeverityRule always sits at index 0 of m.logRules regardless of the
// order rules were added. This mirrors the UI display (severity at the
// top) and makes the "severity is AND with everything" semantic visible.
func TestPinSeverityFirst(t *testing.T) {
	inc1, _ := NewPatternRule("a", PatternSubstring, false)
	inc2, _ := NewPatternRule("b", PatternSubstring, false)
	sev := SeverityRule{Floor: SeverityWarn}

	t.Run("severity in middle gets pinned to front", func(t *testing.T) {
		out := pinSeverityFirst([]Rule{inc1, sev, inc2})
		require.Len(t, out, 3)
		assert.Equal(t, sev, out[0])
		assert.Equal(t, inc1, out[1])
		assert.Equal(t, inc2, out[2])
	})

	t.Run("no severity is a no-op", func(t *testing.T) {
		out := pinSeverityFirst([]Rule{inc1, inc2})
		assert.Equal(t, []Rule{inc1, inc2}, out)
	})

	t.Run("duplicate severity rules are collapsed", func(t *testing.T) {
		sev2 := SeverityRule{Floor: SeverityError}
		out := pinSeverityFirst([]Rule{inc1, sev, inc2, sev2})
		require.Len(t, out, 3, "duplicates dropped — at most one severity")
		assert.Equal(t, sev, out[0], "first severity wins")
	})

	t.Run("empty input", func(t *testing.T) {
		assert.Empty(t, pinSeverityFirst(nil))
	})
}

// TestSeverityRemainsPinnedAfterDelete checks that deleting a non-severity
// rule leaves the severity rule at index 0.
func TestSeverityRemainsPinnedAfterDelete(t *testing.T) {
	det, _ := newSeverityDetector(nil)
	sev := SeverityRule{Floor: SeverityWarn}
	inc1, _ := NewPatternRule("a", PatternSubstring, false)
	inc2, _ := NewPatternRule("b", PatternSubstring, false)

	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logSeverityDetector: det,
		logIncludeMode:      IncludeAny,
		logFilterEditingIdx: -1,
		logRules:            []Rule{sev, inc1, inc2},
		logFilterListCursor: 1, // cursor on inc1
	}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, det)

	m = m.deleteSelectedRule()
	require.Len(t, m.logRules, 2)
	assert.Equal(t, sev, m.logRules[0], "severity still at index 0")
	assert.Equal(t, inc2, m.logRules[1])
}

// TestSeverityAndSemanticsRegardlessOfIncludeMode documents that severity
// is ANDed with includes in both ANY and ALL top-level modes. A line
// with severity < floor is dropped even if includes would otherwise
// match under ANY semantics — severity is a hard gate.
func TestSeverityAndSemanticsRegardlessOfIncludeMode(t *testing.T) {
	det, _ := newSeverityDetector(nil)
	inc, _ := NewPatternRule("api", PatternSubstring, false)
	sev := SeverityRule{Floor: SeverityError}
	rules := []Rule{sev, inc}

	// ANY mode: severity still gates. INFO line with "api" matches the
	// include but fails severity → dropped.
	chainAny := NewFilterChain(rules, IncludeAny, det)
	assert.False(t, chainAny.Keep("[INFO] api request"),
		"ANY mode: severity floor drops INFO line even though include matches")
	assert.True(t, chainAny.Keep("[ERROR] api request"),
		"ANY mode: ERROR + api → keep")

	// ALL mode: same — severity is ANDed regardless.
	chainAll := NewFilterChain(rules, IncludeAll, det)
	assert.False(t, chainAll.Keep("[INFO] api request"))
	assert.True(t, chainAll.Keep("[ERROR] api request"))
}

// TestFilterScrollClampsToVisibleCount reproduces the user-reported bug
// where scrolling up hid all filtered messages that only reappeared when
// scrolling down. Root cause: clampLogScroll used len(m.logLines) as the
// max when computing maxScroll — but with a filter active, logScroll is
// a visible-coordinate. So logScroll could legally sit at a value bigger
// than len(logVisibleIndices), which pushed the content window past the
// end of the projected slice and rendered nothing.
func TestFilterScrollClampsToVisibleCount(t *testing.T) {
	det, errs := newSeverityDetector(nil)
	require.Empty(t, errs)

	// 100 raw lines; only 10 pass the filter.
	lines := make([]string, 100)
	for i := range lines {
		if i%10 == 0 {
			lines[i] = "[ERROR] api: line " + strings.Repeat("x", i%5)
		} else {
			lines[i] = "GET /healthz 200"
		}
	}

	exc, _ := NewPatternRule("/healthz", PatternSubstring, true)
	m := Model{
		mode:                modeLogs,
		width:               120,
		height:              40,
		logLines:            lines,
		logSeverityDetector: det,
		logIncludeMode:      IncludeAny,
		logFilterEditingIdx: -1,
		logRules:            []Rule{exc},
		actionCtx:           actionContext{kind: "Pod", name: "api"},
	}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()
	require.Len(t, m.logVisibleIndices, 10, "filter should leave 10 visible lines")

	// Pathological: simulate a stale scroll value pointing past the visible
	// range (could arise from raw-source arithmetic elsewhere).
	m.logScroll = 50
	m.clampLogScroll()

	// After clamping, logScroll must fit within the visible buffer.
	assert.LessOrEqual(t, m.logScroll, 9,
		"logScroll must be clamped against visible count, not raw count")
}

// TestExcludeClampsCursorAndScroll reproduces the user-reported "shows
// nothing" symptom: the user has scrolled down in the log viewer, then
// adds an exclude rule that shrinks visible lines. The old cursor/scroll
// was past the new visible range, so the renderer's content window
// projected outside the slice and produced a blank view.
func TestExcludeClampsCursorAndScroll(t *testing.T) {
	det, errs := newSeverityDetector(nil)
	require.Empty(t, errs)

	lines := make([]string, 20)
	for i := range lines {
		if i%2 == 0 {
			lines[i] = "GET /healthz 200"
		} else {
			lines[i] = "[INFO] api: request"
		}
	}

	m := Model{
		mode:                modeLogs,
		width:               120,
		height:              40,
		logLines:            lines,
		logCursor:           18, // scrolled deep into the buffer
		logScroll:           12,
		logSeverityDetector: det,
		logIncludeMode:      IncludeAny,
		logFilterEditingIdx: -1,
		actionCtx:           actionContext{kind: "Pod", name: "api"},
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()

	// Add an exclude rule that keeps only 10 lines (the odd indices).
	m = m.handleLogKeyF()
	nm0, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = nm0
	m = typeIntoFilterInput(t, m, "-healthz")
	nm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm

	// After the rule is applied, visible count is 10.
	require.Len(t, m.logVisibleIndices, 10)
	// Cursor and scroll must be clamped into the new range.
	assert.LessOrEqual(t, m.logCursor, 9, "cursor must be clamped to visible range")
	assert.LessOrEqual(t, m.logScroll, 9, "scroll must be clamped to visible range")

	// Render must contain at least one visible INFO line.
	rendered := ui.RenderLogViewer(
		m.logLines, m.logVisibleIndices,
		m.logScroll, m.width, 30,
		m.logFollow, m.logWrap, m.logLineNumbers, m.logTimestamps, m.logPrevious, m.logHidePrefixes,
		m.logTitle, m.logSearchQuery, m.logSearchInput.Value,
		m.logSearchActive, false, false, m.logHasMoreHistory, m.logLoadingHistory,
		"", false,
		m.logCursor, m.logVisualMode, m.logVisualStart, m.logVisualType,
		m.logVisualCol, m.logVisualCurCol, len(m.logRules), "", "", false,
	)
	assert.Contains(t, rendered, "request",
		"render must show the remaining lines after filter + clamp")
}
