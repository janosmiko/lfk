package app

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEscClosesFilterModal(t *testing.T) {
	m := Model{
		overlay:            overlayLogFilter,
		logFilterModalOpen: true,
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, overlayNone, rm.overlay)
	assert.False(t, rm.logFilterModalOpen)
}

func TestTabTogglesFocusInFilterModal(t *testing.T) {
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: true,
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.False(t, rm.logFilterFocusInput)

	rm2, _ := rm.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.True(t, rm2.logFilterFocusInput)
}

func TestJKMovesListCursor(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	r1, _ := NewPatternRule("a", PatternSubstring, false)
	r2, _ := NewPatternRule("b", PatternSubstring, false)
	r3, _ := NewPatternRule("c", PatternSubstring, false)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logFilterListCursor: 0,
		logRules:            []Rule{r1, r2, r3},
		logSeverityDetector: d,
	}

	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 1, rm.logFilterListCursor)

	rm2, _ := rm.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 2, rm2.logFilterListCursor)

	// At bottom, j stays.
	rm3, _ := rm2.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 2, rm3.logFilterListCursor)

	rm4, _ := rm3.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, 1, rm4.logFilterListCursor)
}

func TestTypingFillsInput(t *testing.T) {
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: true,
	}
	for _, r := range "foo" {
		rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = rm
	}
	assert.Equal(t, "foo", m.logFilterInput.Value)
}

// TestSpaceInsertsLiteralSpace pins the fix for a user-reported bug where
// pressing Space in input mode did nothing — bubbletea delivers Space as
// tea.KeySpace (a distinct Key type), not tea.KeyRunes, so the Runes path
// never fires. Spaces matter for group syntax ("foo AND bar") and for
// multi-word patterns in general.
func TestSpaceInsertsLiteralSpace(t *testing.T) {
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: true,
	}
	// Type "foo", then space, then "bar".
	for _, r := range "foo" {
		rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = rm
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeySpace})
	m = rm
	for _, r := range "bar" {
		rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = rm
	}
	assert.Equal(t, "foo bar", m.logFilterInput.Value)
}

// TestSpaceInSavePromptInsertsLiteralSpace covers the save-preset prompt
// path, which has its own input dispatcher.
func TestSpaceInSavePromptInsertsLiteralSpace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	r1, _ := NewPatternRule("a", PatternSubstring, false)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logSavePresetPrompt: true,
		logRules:            []Rule{r1},
	}
	for _, r := range "my" {
		rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = rm
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeySpace})
	m = rm
	for _, r := range "preset" {
		rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = rm
	}
	assert.Equal(t, "my preset", m.logFilterInput.Value)
}

func TestEnterAddsRule(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: true,
		logSeverityDetector: d,
		logFilterEditingIdx: -1,
	}
	m.logFilterInput.Set("-foo")
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Len(t, rm.logRules, 1)
	assert.Equal(t, RuleExclude, rm.logRules[0].Kind())
	assert.Equal(t, "", rm.logFilterInput.Value, "input cleared")
}

func TestEnterRebuildsVisibleIndices(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: true,
		logSeverityDetector: d,
		logFilterEditingIdx: -1,
		logLines:            []string{"foo", "bar"},
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, d)
	m.logFilterInput.Set("foo")
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, []int{0}, rm.logVisibleIndices)
}

func TestDDeletesRule(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	r1, _ := NewPatternRule("a", PatternSubstring, false)
	r2, _ := NewPatternRule("b", PatternSubstring, false)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logRules:            []Rule{r1, r2},
		logSeverityDetector: d,
		logFilterListCursor: 0,
	}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, d)
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	assert.Len(t, rm.logRules, 1)
	assert.Equal(t, r2, rm.logRules[0])
}

func TestELoadsRuleForEdit(t *testing.T) {
	r1, _ := NewPatternRule("foo", PatternSubstring, false)
	r2, _ := NewPatternRule("bar", PatternSubstring, true) // exclude
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logRules:            []Rule{r1, r2},
		logFilterListCursor: 1,
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	assert.True(t, rm.logFilterFocusInput)
	assert.Equal(t, "-bar", rm.logFilterInput.Value)
	assert.Equal(t, 1, rm.logFilterEditingIdx)
}

func TestMTogglesIncludeMode(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logIncludeMode:      IncludeAny,
		logSeverityDetector: d,
	}
	m.logFilterChain = NewFilterChain(nil, IncludeAny, d)
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	assert.Equal(t, IncludeAll, rm.logIncludeMode)

	rm2, _ := rm.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	assert.Equal(t, IncludeAny, rm2.logIncludeMode)
}

func TestShiftJKReordersRules(t *testing.T) {
	r1, _ := NewPatternRule("a", PatternSubstring, false)
	r2, _ := NewPatternRule("b", PatternSubstring, false)
	r3, _ := NewPatternRule("c", PatternSubstring, false)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logRules:            []Rule{r1, r2, r3},
		logFilterListCursor: 1,
	}

	// Move down (J): r2 swaps with r3 → [r1, r3, r2]; cursor → 2
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})
	assert.Equal(t, []Rule{r1, r3, r2}, rm.logRules)
	assert.Equal(t, 2, rm.logFilterListCursor)

	// Move up (K) from position 2: r2 swaps back with r3 → [r1, r2, r3]; cursor → 1
	rm2, _ := rm.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")})
	assert.Equal(t, []Rule{r1, r2, r3}, rm2.logRules)
	assert.Equal(t, 1, rm2.logFilterListCursor)
}

func TestListModeIgnoresUnboundKeys(t *testing.T) {
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
	}
	// Keys that aren't list-mode commands (j/k/a/d/e/m/J/K/S/L) are no-ops
	// in list mode. This prevents accidental keystrokes from dropping the
	// user into typing mode unexpectedly. Use `a` to add a rule.
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	assert.False(t, rm.logFilterFocusInput, "unbound key must not switch modes")
	assert.Empty(t, rm.logFilterInput.Value)
}

func TestQClosesFilterModalFromListMode(t *testing.T) {
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.Equal(t, overlayNone, rm.overlay, "q should close the overlay from list mode")
	assert.False(t, rm.logFilterModalOpen)
}

func TestCtrlCClosesFilterModalFromListMode(t *testing.T) {
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, overlayNone, rm.overlay, "ctrl+c should close the overlay from list mode")
	assert.False(t, rm.logFilterModalOpen)
}

// TestCursorSkipsSeverityRow verifies that j / k and the initial
// overlay cursor never land on the pinned severity slot.
func TestCursorSkipsSeverityRow(t *testing.T) {
	det, _ := newSeverityDetector(nil)
	sev := SeverityRule{Floor: SeverityWarn}
	inc1, _ := NewPatternRule("a", PatternSubstring, false)
	inc2, _ := NewPatternRule("b", PatternSubstring, false)

	// Opening the overlay should land the cursor on the first editable
	// row (index 1 since severity is pinned at 0).
	m := Model{
		mode:                modeLogs,
		logSeverityDetector: det,
		logFilterEditingIdx: -1,
		logIncludeMode:      IncludeAny,
		logRules:            []Rule{sev, inc1, inc2},
	}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, det)
	m = m.handleLogKeyF()
	assert.Equal(t, 1, m.logFilterListCursor, "overlay opens with cursor on first editable row")

	// k from row 1 must NOT drop onto the severity slot.
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, 1, rm.logFilterListCursor, "k on first editable row stays put (can't enter severity)")

	// j moves to the next editable row.
	rm2, _ := rm.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 2, rm2.logFilterListCursor, "j advances to the next rule")

	// k from row 2 goes back to row 1, not severity.
	rm3, _ := rm2.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, 1, rm3.logFilterListCursor)
}

// TestCursorParkedWhenOnlySeverity verifies that with ONLY a severity
// rule in the list, the cursor does not land on the severity row —
// it's parked at -1 so no row is highlighted. j / k are no-ops in
// this state since there are no editable rows.
func TestCursorParkedWhenOnlySeverity(t *testing.T) {
	det, _ := newSeverityDetector(nil)
	sev := SeverityRule{Floor: SeverityError}
	m := Model{
		mode:                modeLogs,
		logSeverityDetector: det,
		logFilterEditingIdx: -1,
		logIncludeMode:      IncludeAny,
		logRules:            []Rule{sev},
	}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, det)

	// Opening the overlay must NOT land the cursor on severity.
	m = m.handleLogKeyF()
	assert.Equal(t, -1, m.logFilterListCursor,
		"cursor is parked (-1) when only severity exists — severity is not selectable")

	// j / k are no-ops in this empty-editable state.
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, -1, rm.logFilterListCursor, "j with only severity keeps cursor parked")

	rm2, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, -1, rm2.logFilterListCursor, "k with only severity keeps cursor parked")

	// d is a no-op too (severity is read-only regardless).
	rm3, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	assert.Len(t, rm3.logRules, 1, "severity cannot be deleted via d")
}

// TestSeverityReadOnly verifies that d and e no-op when the cursor is
// on the pinned severity row. Severity can only be changed via the >/<
// quick-cycle hotkeys.
func TestSeverityReadOnly(t *testing.T) {
	det, _ := newSeverityDetector(nil)
	sev := SeverityRule{Floor: SeverityWarn}
	inc1, _ := NewPatternRule("a", PatternSubstring, false)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logSeverityDetector: det,
		logIncludeMode:      IncludeAny,
		logFilterEditingIdx: -1,
		logRules:            []Rule{sev, inc1},
		logFilterListCursor: 0, // on severity row
	}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, det)

	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	assert.Len(t, rm.logRules, 2, "d on severity must be a no-op")
	_, stillSev := rm.logRules[0].(SeverityRule)
	assert.True(t, stillSev, "severity still pinned")

	rm2, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	assert.False(t, rm2.logFilterFocusInput, "e on severity must NOT switch to input mode")
	assert.Empty(t, rm2.logFilterInput.Value, "no edit payload loaded")
}

// TestListModePageKeys verifies ctrl+d / ctrl+u / ctrl+f / ctrl+b move
// the list cursor by page / half-page in the filter overlay.
func TestListModePageKeys(t *testing.T) {
	rules := make([]Rule, 30)
	for i := range rules {
		r, _ := NewPatternRule("p", PatternSubstring, false)
		rules[i] = r
	}
	det, _ := newSeverityDetector(nil)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logSeverityDetector: det,
		logIncludeMode:      IncludeAny,
		logRules:            rules,
		logFilterListCursor: 0,
	}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, det)

	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.Equal(t, listPageSize, rm.logFilterListCursor, "ctrl+d moves by listPageSize")

	rm2, _ := rm.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	assert.Equal(t, listPageSize+2*listPageSize, rm2.logFilterListCursor)

	rm3, _ := rm2.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.Equal(t, 2*listPageSize, rm3.logFilterListCursor)

	rm4, _ := rm3.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	assert.Equal(t, 0, rm4.logFilterListCursor)
}

// TestInsertModeDeletionKeys verifies ctrl+w deletes the previous word
// and ctrl+u deletes the entire input in input/insert mode.
func TestInsertModeDeletionKeys(t *testing.T) {
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: true,
	}
	m.logFilterInput.Set("foo bar baz")
	m.logFilterInput.End()

	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Equal(t, "foo bar ", rm.logFilterInput.Value, "ctrl+w removes the last word")

	rm2, _ := rm.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.Empty(t, rm2.logFilterInput.Value, "ctrl+u clears the line")
}

// TestGtLtCycleSeverityInOverlay verifies that `>` / `<` in the filter
// modal's list mode cycle the severity floor, mirroring the log-view
// shortcut so the user doesn't need to close the overlay to change it.
func TestGtLtCycleSeverityInOverlay(t *testing.T) {
	det, _ := newSeverityDetector(nil)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false, // list mode
		logSeverityDetector: det,
		logIncludeMode:      IncludeAny,
		logFilterEditingIdx: -1,
	}

	// `>` from no severity → DEBUG floor at index 0 (pinned).
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(">")})
	require.Len(t, rm.logRules, 1)
	sev, ok := rm.logRules[0].(SeverityRule)
	require.True(t, ok, "severity rule should be pinned at index 0")
	assert.Equal(t, SeverityDebug, sev.Floor)

	// `<` cycles backward → off.
	rm2, _ := rm.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})
	// Empty: one `>` from off → DEBUG, then `<` from DEBUG → off (no rule).
	// Actually nextSeverityFloor(DEBUG, -1) = Unknown (off), which clears the rule.
	assert.Empty(t, rm2.logRules, "`<` should clear the rule when at DEBUG")
}

func TestAKeyAddsNewRule(t *testing.T) {
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	assert.True(t, rm.logFilterFocusInput, "`a` switches to input mode")
	assert.Empty(t, rm.logFilterInput.Value, "`a` leaves the input empty for fresh typing")
	assert.Equal(t, -1, rm.logFilterEditingIdx, "`a` clears any in-progress edit")
}

func TestSOpensSavePrompt(t *testing.T) {
	// Redirect the preset sidecar under a tempdir so we don't touch the user config.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	r1, _ := NewPatternRule("a", PatternSubstring, false)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logRules:            []Rule{r1},
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	assert.True(t, rm.logSavePresetPrompt)
}

func TestSWithNoRulesDoesNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	assert.False(t, rm.logSavePresetPrompt)
}

func TestSavePromptEscCancels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logSavePresetPrompt: true,
	}
	m.logFilterInput.Set("mypreset")
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, rm.logSavePresetPrompt)
	assert.Equal(t, "", rm.logFilterInput.Value)
}

func TestSavePromptEnterWritesSidecar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	r1, _ := NewPatternRule("a", PatternSubstring, false)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logSavePresetPrompt: true,
		logRules:            []Rule{r1},
		logIncludeMode:      IncludeAny,
		actionCtx:           actionContext{kind: "Pod"},
	}
	m.logFilterInput.Set("mypreset")

	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, rm.logSavePresetPrompt)
	assert.Equal(t, "", rm.logFilterInput.Value)
	// After saving, focus must return to list (nav) mode regardless of
	// what focus was before the prompt opened. The S shortcut is only
	// reachable from list mode, but the prompt itself disables input
	// focus while active — we want the post-save state to be explicit.
	assert.False(t, rm.logFilterFocusInput, "should return to list mode after save")

	// Verify sidecar file contains the preset.
	path, err := logPresetsPath()
	assert.NoError(t, err)
	_, err = os.Stat(path)
	assert.NoError(t, err, "expected sidecar file to exist at %s", path)
	f, err := readPresetFile(path)
	assert.NoError(t, err)
	assert.Len(t, f.Presets["Pod"], 1)
	assert.Equal(t, "mypreset", f.Presets["Pod"][0].Name)
	assert.Len(t, f.Presets["Pod"][0].Rules, 1)
}

func TestSavePromptEnterEmptyNameCancels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	r1, _ := NewPatternRule("a", PatternSubstring, false)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logSavePresetPrompt: true,
		logRules:            []Rule{r1},
		actionCtx:           actionContext{kind: "Pod"},
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, rm.logSavePresetPrompt)
}

func TestSavePromptTypingAppendsToInput(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logSavePresetPrompt: true,
	}
	for _, r := range "foo" {
		rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = rm
	}
	assert.Equal(t, "foo", m.logFilterInput.Value)
}

func TestLOpensPresetPicker(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	assert.True(t, rm.logLoadPresetOpen)
}

func TestLoadPickerEscCloses(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logLoadPresetOpen:   true,
	}
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, rm.logLoadPresetOpen)
}

func TestLoadPickerEnterApplies(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Seed the sidecar with one preset.
	f := PresetFile{
		Presets: map[string][]LogPreset{
			"Pod": {
				{
					Name:        "noise",
					IncludeMode: "any",
					Rules: []PresetRule{
						{Type: "include", Pattern: "foo", Mode: "substring"},
					},
				},
			},
		},
	}
	path, _ := logPresetsPath()
	assert.NoError(t, writePresetFile(path, f))

	d, _ := newSeverityDetector(nil)
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logLoadPresetOpen:   true,
		logLoadPresetCursor: 0,
		actionCtx:           actionContext{kind: "Pod"},
		logSeverityDetector: d,
	}

	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, rm.logLoadPresetOpen)
	assert.Len(t, rm.logRules, 1)
	assert.Equal(t, IncludeAny, rm.logIncludeMode)
	assert.NotNil(t, rm.logFilterChain)
}

func TestLoadPickerJKMovesCursor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	f := PresetFile{
		Presets: map[string][]LogPreset{
			"Pod": {
				{Name: "p1"},
				{Name: "p2"},
				{Name: "p3"},
			},
		},
	}
	path, _ := logPresetsPath()
	assert.NoError(t, writePresetFile(path, f))

	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logLoadPresetOpen:   true,
		logLoadPresetCursor: 0,
		actionCtx:           actionContext{kind: "Pod"},
	}

	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 1, rm.logLoadPresetCursor)

	rm2, _ := rm.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 2, rm2.logLoadPresetCursor)

	// At bottom, j stays.
	rm3, _ := rm2.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 2, rm3.logLoadPresetCursor)

	rm4, _ := rm3.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, 1, rm4.logLoadPresetCursor)
}

func TestLoadPickerDDeletes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	f := PresetFile{
		Presets: map[string][]LogPreset{
			"Pod": {
				{Name: "p1"},
				{Name: "p2"},
			},
		},
	}
	path, _ := logPresetsPath()
	assert.NoError(t, writePresetFile(path, f))

	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logLoadPresetOpen:   true,
		logLoadPresetCursor: 0,
		actionCtx:           actionContext{kind: "Pod"},
	}
	_, _ = m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	loaded, err := readPresetFile(path)
	assert.NoError(t, err)
	assert.Len(t, loaded.Presets["Pod"], 1)
	assert.Equal(t, "p2", loaded.Presets["Pod"][0].Name)
}

func TestLoadPickerSpaceTogglesDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	f := PresetFile{
		Presets: map[string][]LogPreset{
			"Pod": {
				{Name: "p1", Default: false},
				{Name: "p2", Default: false},
			},
		},
	}
	path, _ := logPresetsPath()
	assert.NoError(t, writePresetFile(path, f))

	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logLoadPresetOpen:   true,
		logLoadPresetCursor: 1,
		actionCtx:           actionContext{kind: "Pod"},
	}
	// First space sets p2 as default.
	_, _ = m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})

	loaded, err := readPresetFile(path)
	assert.NoError(t, err)
	assert.False(t, loaded.Presets["Pod"][0].Default)
	assert.True(t, loaded.Presets["Pod"][1].Default)

	// Second space clears default on p2 (since it was already default).
	_, _ = m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	loaded2, err := readPresetFile(path)
	assert.NoError(t, err)
	assert.False(t, loaded2.Presets["Pod"][0].Default)
	assert.False(t, loaded2.Presets["Pod"][1].Default)
}
