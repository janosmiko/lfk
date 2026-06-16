package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestRebuildLogView_TextFilter(t *testing.T) {
	var m Model
	m.logView.rawLines = []string{"apple pie", "banana split", "apple tart"}
	m.logView.filterQuery = "apple"
	m.rebuildLogView()
	if len(m.logView.lines) != 2 {
		t.Fatalf("got %d lines, want 2: %v", len(m.logView.lines), m.logView.lines)
	}
}

func TestRebuildLogView_NoFilterAliasesRaw(t *testing.T) {
	var m Model
	m.logView.rawLines = []string{"a", "b", "c"}
	m.rebuildLogView()
	if len(m.logView.lines) != 3 {
		t.Fatalf("no filter should show all 3, got %d", len(m.logView.lines))
	}
}

func TestRebuildLogView_Severity(t *testing.T) {
	var m Model
	m.logView.rawLines = []string{
		`{"level":"info","msg":"a"}`,
		`{"level":"error","msg":"b"}`,
		"\tcontinuation of the error",
		`{"level":"debug","msg":"c"}`,
	}
	m.logView.sevThreshold = ui.SevError
	m.rebuildLogView()
	// error line + its continuation tail (inherits error) survive; info/debug drop.
	if len(m.logView.lines) != 2 {
		t.Fatalf("got %d lines, want 2 (error + continuation): %v", len(m.logView.lines), m.logView.lines)
	}
}

func TestRebuildLogView_ClampsCursor(t *testing.T) {
	var m Model
	m.logView.rawLines = []string{"keep one", "drop", "drop", "drop"}
	m.logView.cursor = 3
	m.logView.scroll = 3
	m.logView.filterQuery = "keep"
	m.rebuildLogView()
	if m.logView.cursor >= len(m.logView.lines) {
		t.Fatalf("cursor %d not clamped into %d lines", m.logView.cursor, len(m.logView.lines))
	}
}

func TestRebuildLogView_EmptyResultClampsAndExitsVisual(t *testing.T) {
	var m Model
	m.logView.rawLines = []string{"alpha", "beta"}
	m.logView.cursor = 1
	m.logView.scroll = 1
	m.logView.visualMode = true
	m.logView.filterQuery = "no-such-text"
	m.rebuildLogView()
	if len(m.logView.lines) != 0 {
		t.Fatalf("want 0 lines, got %d", len(m.logView.lines))
	}
	if m.logView.cursor != -1 {
		t.Errorf("cursor = %d, want -1 on empty view", m.logView.cursor)
	}
	if m.logView.scroll != 0 {
		t.Errorf("scroll = %d, want 0 on empty view", m.logView.scroll)
	}
	if m.logView.visualMode {
		t.Error("visualMode should be cleared on empty view")
	}
}

func TestRebuildLogView_FirstLineContinuationDropped(t *testing.T) {
	var m Model
	// First line has no detectable level (SevUnknown -> inherits SevUnknown=0),
	// so at an ERROR threshold it is dropped; the error line survives.
	m.logView.rawLines = []string{
		"\tat com.example.Foo.bar(Foo.java:42)",
		`{"level":"error","msg":"boom"}`,
	}
	m.logView.sevThreshold = ui.SevError
	m.rebuildLogView()
	if len(m.logView.lines) != 1 {
		t.Fatalf("want 1 line (only the error), got %d: %v", len(m.logView.lines), m.logView.lines)
	}
}

func TestAppendRawLogLine_FilteredView(t *testing.T) {
	var m Model
	m.logView.filterQuery = "keep"
	m.appendRawLogLine("keep this")
	m.appendRawLogLine("drop this")
	m.appendRawLogLine("keep that")
	if len(m.logView.rawLines) != 3 {
		t.Fatalf("rawLines = %d, want 3", len(m.logView.rawLines))
	}
	if len(m.logView.lines) != 2 {
		t.Fatalf("filtered lines = %d, want 2: %v", len(m.logView.lines), m.logView.lines)
	}
}

func TestAppendRawLogLine_NoFilter(t *testing.T) {
	var m Model
	m.appendRawLogLine("a")
	m.appendRawLogLine("b")
	if len(m.logView.lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(m.logView.lines))
	}
}

func TestTabPersistence_FilterRoundTrip(t *testing.T) {
	var m Model
	m.logView.rawLines = []string{"keep a", "drop", "keep b"}
	m.logView.filterQuery = "keep"
	m.logView.sevThreshold = ui.SevWarn
	m.rebuildLogView()

	var ts TabState
	ts.logLines = append([]string(nil), m.logView.rawLines...)
	ts.logFilterQuery = m.logView.filterQuery
	ts.logSevThreshold = m.logView.sevThreshold

	var m2 Model
	m2.logView.rawLines = append([]string(nil), ts.logLines...)
	m2.logView.filterQuery = ts.logFilterQuery
	m2.logView.sevThreshold = ts.logSevThreshold
	m2.rebuildLogView()

	if m2.logView.filterQuery != "keep" || m2.logView.sevThreshold != ui.SevWarn {
		t.Fatalf("filter state not restored: q=%q sev=%d", m2.logView.filterQuery, m2.logView.sevThreshold)
	}
	if len(m2.logView.rawLines) != 3 {
		t.Fatalf("rawLines not restored: %d", len(m2.logView.rawLines))
	}
}

func TestFilterInput_LiveNarrows(t *testing.T) {
	var m Model
	m.mode = modeLogs
	m.logView.rawLines = []string{"alpha", "beta", "alphabet"}
	m.rebuildLogView()
	mdl, _ := m.handleLogKeyFilter()
	m = mdl
	if !m.logView.filterActive {
		t.Fatal("filterActive should be true after opening")
	}
	for _, r := range "alph" {
		mm, _ := m.handleLogFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mm.(Model)
	}
	if m.logView.filterQuery != "alph" {
		t.Fatalf("filterQuery = %q, want alph", m.logView.filterQuery)
	}
	if len(m.logView.lines) != 2 {
		t.Fatalf("live filter should show 2 (alpha, alphabet), got %d: %v", len(m.logView.lines), m.logView.lines)
	}
	mm, _ := m.handleLogFilterKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mm.(Model)
	if m.logView.filterActive || m.logView.filterQuery != "" {
		t.Fatal("esc should cancel and clear the filter")
	}
	if len(m.logView.lines) != 3 {
		t.Fatalf("clearing filter should restore 3 lines, got %d", len(m.logView.lines))
	}
}

func TestAppendRawLogLine_SeverityFilter(t *testing.T) {
	var m Model
	m.logView.sevThreshold = ui.SevError
	m.appendRawLogLine(`{"level":"info","msg":"a"}`)
	m.appendRawLogLine(`{"level":"error","msg":"b"}`)
	m.appendRawLogLine("\tcontinuation tail")
	if len(m.logView.rawLines) != 3 {
		t.Fatalf("rawLines = %d, want 3", len(m.logView.rawLines))
	}
	// error line + its continuation tail (inherits error) shown; info dropped.
	if len(m.logView.lines) != 2 {
		t.Fatalf("lines = %d, want 2 (error+continuation): %v", len(m.logView.lines), m.logView.lines)
	}
}
