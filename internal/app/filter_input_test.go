package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
)

// --- handleFilterKey: escape ---

func TestHandleFilterKeyEscape(t *testing.T) {
	ti := &TextInput{Value: "test", Cursor: 4}
	action := handleFilterKey(ti, "esc")
	assert.Equal(t, filterEscape, action)
	// handleFilterKey does NOT clear the input; the caller decides behavior.
	assert.Equal(t, "test", ti.Value)
}

// --- handleFilterKey: enter ---

func TestHandleFilterKeyEnter(t *testing.T) {
	ti := &TextInput{Value: "test", Cursor: 4}
	action := handleFilterKey(ti, "enter")
	assert.Equal(t, filterAccept, action)
	assert.Equal(t, "test", ti.Value)
}

// --- handleFilterKey: ctrl+c ---

func TestHandleFilterKeyCtrlC(t *testing.T) {
	ti := &TextInput{Value: "test", Cursor: 4}
	action := handleFilterKey(ti, "ctrl+c")
	assert.Equal(t, filterClose, action)
}

// --- handleFilterKey: backspace ---

func TestHandleFilterKeyBackspace(t *testing.T) {
	ti := &TextInput{Value: "abc", Cursor: 3}
	action := handleFilterKey(ti, "backspace")
	assert.Equal(t, filterContinue, action)
	assert.Equal(t, "ab", ti.Value)
	assert.Equal(t, 2, ti.Cursor)
}

func TestHandleFilterKeyBackspaceEmpty(t *testing.T) {
	ti := &TextInput{Value: "", Cursor: 0}
	action := handleFilterKey(ti, "backspace")
	assert.Equal(t, filterContinue, action)
	assert.Equal(t, "", ti.Value)
}

// --- handleFilterKey: ctrl+w (delete word) ---

func TestHandleFilterKeyCtrlW(t *testing.T) {
	ti := &TextInput{Value: "hello world", Cursor: 11}
	action := handleFilterKey(ti, "ctrl+w")
	assert.Equal(t, filterContinue, action)
	assert.Equal(t, "hello ", ti.Value)
}

func TestHandleFilterKeyCtrlWEmpty(t *testing.T) {
	ti := &TextInput{Value: "", Cursor: 0}
	action := handleFilterKey(ti, "ctrl+w")
	assert.Equal(t, filterContinue, action)
	assert.Equal(t, "", ti.Value)
}

// --- handleFilterKey: ctrl+a (home) ---

func TestHandleFilterKeyCtrlA(t *testing.T) {
	ti := &TextInput{Value: "hello", Cursor: 3}
	action := handleFilterKey(ti, "ctrl+a")
	assert.Equal(t, filterNavigate, action)
	assert.Equal(t, 0, ti.Cursor)
}

// --- handleFilterKey: ctrl+e (end) ---

func TestHandleFilterKeyCtrlE(t *testing.T) {
	ti := &TextInput{Value: "hello", Cursor: 0}
	action := handleFilterKey(ti, "ctrl+e")
	assert.Equal(t, filterNavigate, action)
	assert.Equal(t, 5, ti.Cursor)
}

// --- handleFilterKey: left ---

func TestHandleFilterKeyLeft(t *testing.T) {
	ti := &TextInput{Value: "hello", Cursor: 3}
	action := handleFilterKey(ti, "left")
	assert.Equal(t, filterNavigate, action)
	assert.Equal(t, 2, ti.Cursor)
}

func TestHandleFilterKeyLeftAtStart(t *testing.T) {
	ti := &TextInput{Value: "hello", Cursor: 0}
	action := handleFilterKey(ti, "left")
	assert.Equal(t, filterNavigate, action)
	assert.Equal(t, 0, ti.Cursor)
}

// --- handleFilterKey: right ---

func TestHandleFilterKeyRight(t *testing.T) {
	ti := &TextInput{Value: "hello", Cursor: 3}
	action := handleFilterKey(ti, "right")
	assert.Equal(t, filterNavigate, action)
	assert.Equal(t, 4, ti.Cursor)
}

func TestHandleFilterKeyRightAtEnd(t *testing.T) {
	ti := &TextInput{Value: "hello", Cursor: 5}
	action := handleFilterKey(ti, "right")
	assert.Equal(t, filterNavigate, action)
	assert.Equal(t, 5, ti.Cursor)
}

// --- handleFilterKey: printable char insert ---

func TestHandleFilterKeyInsertChar(t *testing.T) {
	ti := &TextInput{Value: "", Cursor: 0}
	action := handleFilterKey(ti, "a")
	assert.Equal(t, filterContinue, action)
	assert.Equal(t, "a", ti.Value)
	assert.Equal(t, 1, ti.Cursor)
}

func TestHandleFilterKeyInsertCharMidString(t *testing.T) {
	ti := &TextInput{Value: "hllo", Cursor: 1}
	action := handleFilterKey(ti, "e")
	assert.Equal(t, filterContinue, action)
	assert.Equal(t, "hello", ti.Value)
	assert.Equal(t, 2, ti.Cursor)
}

// --- handleFilterKey: non-printable unhandled ---

func TestHandleFilterKeyUnhandled(t *testing.T) {
	// Multi-char key names like "tab" are ignored by handleFilterKey.
	// Paste events are handled separately via handlePastedText.
	ti := &TextInput{Value: "test", Cursor: 4}
	action := handleFilterKey(ti, "tab")
	assert.Equal(t, filterIgnored, action)
	assert.Equal(t, "test", ti.Value)
}

func TestHandleFilterKeyUnhandledControlChar(t *testing.T) {
	// Single non-printable byte (e.g. raw control char) is ignored.
	ti := &TextInput{Value: "test", Cursor: 4}
	action := handleFilterKey(ti, "\x01")
	assert.Equal(t, filterIgnored, action)
	assert.Equal(t, "test", ti.Value)
}

// --- stringFilterInput adapter ---

func TestStringFilterInputInsertAppendsToEnd(t *testing.T) {
	s := "hello"
	sfi := &stringFilterInput{ptr: &s}
	sfi.Insert("!")
	assert.Equal(t, "hello!", s)
}

func TestStringFilterInputInsert(t *testing.T) {
	s := "hllo"
	sfi := &stringFilterInput{ptr: &s}
	sfi.Insert("e")
	assert.Equal(t, "hlloe", s)
}

func TestStringFilterInputBackspace(t *testing.T) {
	s := "abc"
	sfi := &stringFilterInput{ptr: &s}
	sfi.Backspace()
	assert.Equal(t, "ab", s)
}

func TestStringFilterInputBackspaceEmpty(t *testing.T) {
	s := ""
	sfi := &stringFilterInput{ptr: &s}
	sfi.Backspace()
	assert.Equal(t, "", s)
}

func TestStringFilterInputDeleteWord(t *testing.T) {
	s := "hello world"
	sfi := &stringFilterInput{ptr: &s}
	sfi.DeleteWord()
	assert.Equal(t, "hello ", s)
}

func TestStringFilterInputDeleteWordSingleWord(t *testing.T) {
	s := "hello"
	sfi := &stringFilterInput{ptr: &s}
	sfi.DeleteWord()
	assert.Equal(t, "", s)
}

func TestStringFilterInputDeleteWordEmpty(t *testing.T) {
	s := ""
	sfi := &stringFilterInput{ptr: &s}
	sfi.DeleteWord()
	assert.Equal(t, "", s)
}

func TestStringFilterInputClear(t *testing.T) {
	s := "hello"
	sfi := &stringFilterInput{ptr: &s}
	sfi.Clear()
	assert.Equal(t, "", s)
}

func TestStringFilterInputHomeEndLeftRight(t *testing.T) {
	s := "hello"
	sfi := &stringFilterInput{ptr: &s}
	// These are no-ops for raw strings but should not panic.
	sfi.Home()
	sfi.End()
	sfi.Left()
	sfi.Right()
	assert.Equal(t, "hello", s)
}

// --- TextInput satisfies FilterInput ---

func TestTextInputSatisfiesFilterInput(t *testing.T) {
	var fi FilterInput = &TextInput{}
	assert.NotNil(t, fi)
}

// --- stringFilterInput satisfies FilterInput ---

func TestStringFilterInputSatisfiesFilterInput(t *testing.T) {
	s := ""
	var fi FilterInput = &stringFilterInput{ptr: &s}
	assert.NotNil(t, fi)
}

// --- Integration: handleFilterKey with stringFilterInput ---

func TestHandleFilterKeyWithStringAdapter(t *testing.T) {
	s := ""
	sfi := &stringFilterInput{ptr: &s}

	action := handleFilterKey(sfi, "h")
	assert.Equal(t, filterContinue, action)
	assert.Equal(t, "h", s)

	action = handleFilterKey(sfi, "i")
	assert.Equal(t, filterContinue, action)
	assert.Equal(t, "hi", s)

	action = handleFilterKey(sfi, "backspace")
	assert.Equal(t, filterContinue, action)
	assert.Equal(t, "h", s)

	action = handleFilterKey(sfi, "esc")
	assert.Equal(t, filterEscape, action)
}

// --- Verify converted handlers produce same results as before ---

func TestNamespaceFilterModeViaShared_Esc(t *testing.T) {
	m := Model{
		overlay:       overlayNamespace,
		nsFilterMode:  true,
		overlayFilter: TextInput{Value: "test", Cursor: 4},
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleNamespaceFilterMode(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.nsFilterMode)
	assert.Empty(t, result.overlayFilter.Value)
	assert.Equal(t, 0, result.overlayCursor)
}

func TestNamespaceFilterModeViaShared_Enter(t *testing.T) {
	m := Model{
		overlay:       overlayNamespace,
		nsFilterMode:  true,
		overlayFilter: TextInput{Value: "kube", Cursor: 4},
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleNamespaceFilterMode(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.nsFilterMode)
	assert.Equal(t, 0, result.overlayCursor)
}

func TestNamespaceFilterModeViaShared_Typing(t *testing.T) {
	m := Model{
		overlay:       overlayNamespace,
		nsFilterMode:  true,
		overlayFilter: TextInput{Value: "", Cursor: 0},
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleNamespaceFilterMode(runeKey('d'))
	result := ret.(Model)
	assert.Equal(t, "d", result.overlayFilter.Value)
	assert.Equal(t, 0, result.overlayCursor)
}

func TestNamespaceFilterModeViaShared_Backspace(t *testing.T) {
	m := Model{
		overlay:       overlayNamespace,
		nsFilterMode:  true,
		overlayFilter: TextInput{Value: "abc", Cursor: 3},
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleNamespaceFilterMode(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "ab", result.overlayFilter.Value)
}

func TestTemplateFilterModeViaShared_CtrlW(t *testing.T) {
	m := Model{
		overlay:            overlayTemplates,
		templateSearchMode: true,
		templateFilter:     TextInput{Value: "hello world", Cursor: 11},
		templateCursor:     0,
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleTemplateFilterMode(tea.KeyMsg{Type: tea.KeyCtrlW})
	result := ret.(Model)
	assert.Equal(t, "hello ", result.templateFilter.Value)
}

func TestBookmarkFilterModeViaShared_Enter(t *testing.T) {
	m := Model{
		overlay:            overlayBookmarks,
		bookmarkSearchMode: bookmarkModeFilter,
		bookmarkFilter:     TextInput{Value: "test", Cursor: 4},
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleBookmarkFilterMode(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Equal(t, bookmarkModeNormal, result.bookmarkSearchMode)
}

func TestCanISubjectFilterModeViaShared_Typing(t *testing.T) {
	m := Model{
		overlay:               overlayCanISubject,
		canISubjectFilterMode: true,
		overlayFilter:         TextInput{Value: "", Cursor: 0},
		tabs:                  []TabState{{}},
		width:                 80,
		height:                40,
	}
	ret, _ := m.handleCanISubjectFilterMode(runeKey('a'))
	result := ret.(Model)
	assert.Equal(t, "a", result.overlayFilter.Value)
	assert.Equal(t, 0, result.overlayCursor)
}

func TestLogPodFilterModeViaShared_Backspace(t *testing.T) {
	m := Model{
		overlay:            overlayPodSelect,
		logPodFilterActive: true,
		logPodFilterText:   "abc",
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
	ret, _ := m.handleLogPodFilterMode(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "ab", result.logPodFilterText)
}

func TestLogContainerFilterModeViaShared_CtrlW(t *testing.T) {
	m := Model{
		overlay:                  overlayLogContainerSelect,
		logContainerFilterActive: true,
		logContainerFilterText:   "hello world",
		tabs:                     []TabState{{}},
		width:                    80,
		height:                   40,
	}
	ret, _ := m.handleLogContainerFilterMode(tea.KeyMsg{Type: tea.KeyCtrlW})
	result := ret.(Model)
	assert.Equal(t, "hello ", result.logContainerFilterText)
}

func TestCovStringFilterInputNavigation(t *testing.T) {
	s := "hello"
	fi := &stringFilterInput{ptr: &s}

	// These are all no-ops for stringFilterInput but need coverage.
	fi.Home()
	fi.End()
	fi.Left()
	fi.Right()
	assert.Equal(t, "hello", s)

	fi.Insert("!")
	assert.Equal(t, "hello!", s)

	fi.Backspace()
	assert.Equal(t, "hello", s)

	fi.DeleteWord()
	assert.Equal(t, "", s)

	fi.Clear()
	assert.Equal(t, "", s)
}

func TestCovErrorLogOverlayKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.overlayErrorLog = true
	m.errorLog = []ui.ErrorLogEntry{{Level: "ERR", Message: "error1"}}
	result, _ := m.handleErrorLogOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.overlayErrorLog)
}

func TestCovErrorLogOverlayKeyDown(t *testing.T) {
	m := baseModelHandlers2()
	m.overlayErrorLog = true
	m.errorLog = []ui.ErrorLogEntry{{Level: "ERR", Message: "error1"}, {Level: "ERR", Message: "error2"}}
	m.errorLogCursorLine = 0
	result, _ := m.handleErrorLogOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.errorLogCursorLine)
}

func TestCovErrorLogOverlayKeyUp(t *testing.T) {
	m := baseModelHandlers2()
	m.overlayErrorLog = true
	m.errorLog = []ui.ErrorLogEntry{{Level: "ERR", Message: "error1"}, {Level: "ERR", Message: "error2"}}
	m.errorLogCursorLine = 1
	result, _ := m.handleErrorLogOverlayKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.errorLogCursorLine)
}

func TestCovErrorLogOverlayKeyBigG(t *testing.T) {
	m := baseModelHandlers2()
	m.overlayErrorLog = true
	m.errorLog = []ui.ErrorLogEntry{{Level: "E", Message: "a"}, {Level: "E", Message: "b"}, {Level: "E", Message: "c"}}
	result, _ := m.handleErrorLogOverlayKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.errorLogCursorLine)
}

func TestCovErrorLogOverlayKeyGG(t *testing.T) {
	m := baseModelHandlers2()
	m.overlayErrorLog = true
	m.errorLog = []ui.ErrorLogEntry{{Level: "E", Message: "a"}, {Level: "E", Message: "b"}}
	m.errorLogCursorLine = 1
	result, _ := m.handleErrorLogOverlayKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
	result, _ = rm.handleErrorLogOverlayKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.errorLogCursorLine)
}

func TestCovErrorLogOverlayKeyCtrlD(t *testing.T) {
	m := baseModelHandlers2()
	m.overlayErrorLog = true
	m.errorLog = make([]ui.ErrorLogEntry, 30)
	for i := range m.errorLog {
		m.errorLog[i] = ui.ErrorLogEntry{Level: "E", Message: "err"}
	}
	m.errorLogCursorLine = 0
	result, _ := m.handleErrorLogOverlayKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Greater(t, rm.errorLogCursorLine, 0)
}

func TestCovErrorLogOverlayKeyCtrlU(t *testing.T) {
	m := baseModelHandlers2()
	m.overlayErrorLog = true
	m.errorLog = make([]ui.ErrorLogEntry, 30)
	for i := range m.errorLog {
		m.errorLog[i] = ui.ErrorLogEntry{Level: "E", Message: "err"}
	}
	m.errorLogCursorLine = 20
	result, _ := m.handleErrorLogOverlayKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Less(t, rm.errorLogCursorLine, 20)
}

func TestCovHandleCommandBarKeyEsc(t *testing.T) {
	m := baseModelCov()
	m.commandBarActive = true
	m.commandBarInput = TextInput{Value: "test"}

	r, _ := m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, r.(Model).commandBarActive)
	assert.Empty(t, r.(Model).commandBarInput.Value)
}

func TestCovHandleCommandBarKeyEnterEmpty(t *testing.T) {
	m := baseModelCov()
	m.commandBarActive = true
	m.commandBarInput = TextInput{Value: ""}
	m.commandHistory = &commandHistory{cursor: -1}

	r, cmd := m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, r.(Model).commandBarActive)
	assert.Nil(t, cmd)
}

func TestCovHandleCommandBarKeyEnterQuit(t *testing.T) {
	m := baseModelCov()
	m.commandBarActive = true
	m.commandBarInput = TextInput{Value: "q", Cursor: 1}
	m.commandHistory = &commandHistory{cursor: -1}

	_, cmd := m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd)
}

func TestCovHandleCommandBarKeyUpDown(t *testing.T) {
	m := baseModelCov()
	m.commandBarActive = true
	m.commandHistory = &commandHistory{
		entries: []string{"first", "second"},
		cursor:  -1,
	}
	m.commandBarInput = TextInput{Value: "current", Cursor: 7}

	r, _ := m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, "second", r.(Model).commandBarInput.Value)

	m2 := r.(Model)
	r, _ = m2.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, "current", r.(Model).commandBarInput.Value)
}

func TestCovHandleCommandBarKeyTab(t *testing.T) {
	m := baseModelCov()
	m.commandBarActive = true
	m.commandBarInput = TextInput{Value: "get", Cursor: 3}
	m.commandBarSuggestions = []string{"get", "get pods"}
	m.commandBarSelectedSuggestion = 0
	m.commandHistory = &commandHistory{cursor: -1}

	r, _ := m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.NotEqual(t, "get", r.(Model).commandBarInput.Value)
}

func TestCovHandleCommandBarKeyShiftTab(t *testing.T) {
	m := baseModelCov()
	m.commandBarActive = true
	m.commandBarSuggestions = []string{"a", "b", "c"}
	m.commandBarSelectedSuggestion = 0

	r, _ := m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	assert.Equal(t, 2, r.(Model).commandBarSelectedSuggestion)
}

func TestCovHandleCommandBarKeyBackspace(t *testing.T) {
	m := baseModelCov()
	m.commandBarActive = true
	m.commandBarInput = TextInput{Value: "get", Cursor: 3}
	m.commandHistory = &commandHistory{cursor: -1}

	r, _ := m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "ge", r.(Model).commandBarInput.Value)
}

func TestCovHandleCommandBarKeyCtrlW(t *testing.T) {
	m := baseModelCov()
	m.commandBarActive = true
	m.commandBarInput = TextInput{Value: "get pods", Cursor: 8}
	m.commandHistory = &commandHistory{cursor: -1}

	r, _ := m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Equal(t, "get ", r.(Model).commandBarInput.Value)
}

func TestCovHandleCommandBarKeyCtrlC(t *testing.T) {
	m := baseModelCov()
	m.commandBarActive = true
	m.commandBarInput = TextInput{Value: "test"}

	r, _ := m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.False(t, r.(Model).commandBarActive)
}

func TestCovHandleCommandBarKeyInsert(t *testing.T) {
	m := baseModelCov()
	m.commandBarActive = true
	m.commandBarInput = TextInput{}
	m.commandHistory = &commandHistory{cursor: -1}

	r, _ := m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Equal(t, "x", r.(Model).commandBarInput.Value)
}

func TestCovHandleCommandBarKeyRightLeft(t *testing.T) {
	m := baseModelCov()
	m.commandBarActive = true
	m.commandBarInput = TextInput{Value: "hello", Cursor: 3}

	// Without suggestions: moves cursor.
	r, _ := m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 4, r.(Model).commandBarInput.Cursor)

	r, _ = m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 2, r.(Model).commandBarInput.Cursor)

	// With suggestions: cycles.
	m.commandBarSuggestions = []string{"a", "b", "c"}
	m.commandBarSelectedSuggestion = 0

	r, _ = m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 1, r.(Model).commandBarSelectedSuggestion)

	m.commandBarSelectedSuggestion = 0
	r, _ = m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 2, r.(Model).commandBarSelectedSuggestion)
}

func TestCovHandleCommandBarKeyCtrlAE(t *testing.T) {
	m := baseModelCov()
	m.commandBarActive = true
	m.commandBarInput = TextInput{Value: "hello", Cursor: 3}

	r, _ := m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyCtrlA})
	assert.Equal(t, 0, r.(Model).commandBarInput.Cursor)

	r, _ = m.handleCommandBarKey(tea.KeyMsg{Type: tea.KeyCtrlE})
	assert.Equal(t, 5, r.(Model).commandBarInput.Cursor)
}

func TestCovHandleErrorLogOverlayKeyEsc(t *testing.T) {
	m := baseModelCov()
	m.overlayErrorLog = true

	r, _ := m.handleErrorLogOverlayKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, r.(Model).overlayErrorLog)
}

func TestCovHandleErrorLogOverlayKeyFullscreen(t *testing.T) {
	m := baseModelCov()
	m.overlayErrorLog = true

	r, _ := m.handleErrorLogOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	assert.True(t, r.(Model).errorLogFullscreen)

	m2 := r.(Model)
	r, _ = m2.handleErrorLogOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	assert.False(t, r.(Model).errorLogFullscreen)
}

func TestCovHandleErrorLogOverlayKeyVisualMode(t *testing.T) {
	m := baseModelCov()
	m.overlayErrorLog = true

	r, _ := m.handleErrorLogOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'V'}})
	assert.Equal(t, byte('V'), r.(Model).errorLogVisualMode)

	// Esc cancels visual mode.
	m2 := r.(Model)
	r, _ = m2.handleErrorLogOverlayKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, byte(0), r.(Model).errorLogVisualMode)
}

func TestCovHandleQuitConfirmOverlayKey(t *testing.T) {
	m := baseModelCov()
	m.overlay = overlayQuitConfirm

	// 'n' cancels.
	r, _ := m.handleQuitConfirmOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, overlayNone, r.(Model).overlay)
}

func TestCovQuitConfirmKeyY(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayQuitConfirm
	result, cmd := m.handleQuitConfirmOverlayKey(keyMsg("y"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd) // tea.Quit
}

func TestCovQuitConfirmKeyBigY(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayQuitConfirm
	result, cmd := m.handleQuitConfirmOverlayKey(keyMsg("Y"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd) // tea.Quit
}

func TestCovQuitConfirmKeyN(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayQuitConfirm
	result, _ := m.handleQuitConfirmOverlayKey(keyMsg("n"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovQuitConfirmKeyEsc(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayQuitConfirm
	result, _ := m.handleQuitConfirmOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovQuitConfirmKeyDefault(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayQuitConfirm
	result, _ := m.handleQuitConfirmOverlayKey(keyMsg("x"))
	_ = result.(Model)
}

func TestCovStringFilterInput(t *testing.T) {
	s := "hello"
	fi := &stringFilterInput{ptr: &s}

	fi.Insert("!")
	assert.Equal(t, "hello!", s)

	fi.Backspace()
	assert.Equal(t, "hello", s)

	fi.DeleteWord()
	assert.Equal(t, "", s)

	// DeleteWord on empty: no-op.
	fi.DeleteWord()
	assert.Equal(t, "", s)

	// Backspace on empty: no-op.
	fi.Backspace()
	assert.Equal(t, "", s)

	// Clear.
	s = "test"
	fi.Clear()
	assert.Equal(t, "", s)

	// No-op cursor methods.
	fi.Home()
	fi.End()
	fi.Left()
	fi.Right()
}

// --- handlePastedText ---

func TestHandlePastedTextSingleLine(t *testing.T) {
	ti := &TextInput{Value: "", Cursor: 0}
	action := handlePastedText(ti, []rune("hello world"))
	assert.Equal(t, filterContinue, action)
	assert.Equal(t, "hello world", ti.Value)
}

func TestHandlePastedTextTrailingNewline(t *testing.T) {
	ti := &TextInput{Value: "", Cursor: 0}
	action := handlePastedText(ti, []rune("hello\n"))
	assert.Equal(t, filterContinue, action)
	assert.Equal(t, "hello", ti.Value)
}

func TestHandlePastedTextMultiline(t *testing.T) {
	ti := &TextInput{Value: "", Cursor: 0}
	action := handlePastedText(ti, []rune("line1\nline2"))
	assert.Equal(t, filterPasteMultiline, action)
	assert.Equal(t, "", ti.Value) // not inserted yet
}

func TestHandlePastedTextEmpty(t *testing.T) {
	ti := &TextInput{Value: "pre", Cursor: 3}
	action := handlePastedText(ti, []rune(""))
	assert.Equal(t, filterIgnored, action)
	assert.Equal(t, "pre", ti.Value)
}

func TestHandlePastedTextOnlyNewlines(t *testing.T) {
	ti := &TextInput{Value: "pre", Cursor: 3}
	action := handlePastedText(ti, []rune("\n\n"))
	assert.Equal(t, filterIgnored, action)
}

func TestCovStringFilterInputDeleteWordWithSpaces(t *testing.T) {
	s := "hello world  "
	fi := &stringFilterInput{ptr: &s}
	fi.DeleteWord()
	assert.Equal(t, "hello ", s)
}
