package app

import (
	"strings"
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
		`{"level":"debug","msg":"d"}`,
		`{"level":"info","msg":"a"}`,
		`{"level":"warn","msg":"w"}`,
		`{"level":"error","msg":"b"}`,
	}
	// INFO+ drops debug.
	m.logView.sevThreshold = ui.LogInfo
	m.rebuildLogView()
	if len(m.logView.lines) != 3 {
		t.Fatalf("INFO+: want 3 (info/warn/error), got %d: %v", len(m.logView.lines), m.logView.lines)
	}
	// WARN+ keeps warn+error.
	m.logView.sevThreshold = ui.LogWarn
	m.rebuildLogView()
	if len(m.logView.lines) != 2 {
		t.Fatalf("WARN+: want 2 (warn/error), got %d: %v", len(m.logView.lines), m.logView.lines)
	}
	// ERROR+ keeps only error.
	m.logView.sevThreshold = ui.LogError
	m.rebuildLogView()
	if len(m.logView.lines) != 1 {
		t.Fatalf("ERROR+: want 1 (error), got %d: %v", len(m.logView.lines), m.logView.lines)
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

func TestRebuildLogView_PlainTextKeywordFiltering(t *testing.T) {
	var m Model
	// Plain-text lines (no structured level) are bucketed by keyword scan,
	// defaulting to INFO. This is the user-facing contract for text logs.
	m.logView.rawLines = []string{
		"Running full sweep",            // no keyword -> INFO
		"disk space warning: 90% used",  // warn keyword -> WARN
		"connection error: timeout",     // error keyword -> ERROR
		"DEBUG entering reconcile loop", // debug keyword -> DEBUG
	}
	// off (no filter active) -> all 4 shown.
	m.rebuildLogView()
	if len(m.logView.lines) != 4 {
		t.Fatalf("off: want 4, got %d: %v", len(m.logView.lines), m.logView.lines)
	}
	// INFO+ -> hide debug, show the other 3.
	m.logView.sevThreshold = ui.LogInfo
	m.rebuildLogView()
	if len(m.logView.lines) != 3 {
		t.Fatalf("INFO+: want 3 (no debug), got %d: %v", len(m.logView.lines), m.logView.lines)
	}
	// WARN+ -> only warn + error lines.
	m.logView.sevThreshold = ui.LogWarn
	m.rebuildLogView()
	if len(m.logView.lines) != 2 {
		t.Fatalf("WARN+: want 2 (warn+error), got %d: %v", len(m.logView.lines), m.logView.lines)
	}
	// ERROR+ -> only the error line.
	m.logView.sevThreshold = ui.LogError
	m.rebuildLogView()
	if len(m.logView.lines) != 1 {
		t.Fatalf("ERROR+: want 1 (error), got %d: %v", len(m.logView.lines), m.logView.lines)
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
	m.logView.sevThreshold = ui.LogWarn
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

	if m2.logView.filterQuery != "keep" || m2.logView.sevThreshold != ui.LogWarn {
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

func TestSeverityStep(t *testing.T) {
	// Thresholds cycle off(0) -> INFO -> WARN -> ERROR, clamped at ERROR.
	var m Model
	m.logView.rawLines = []string{
		`{"level":"debug","msg":"d"}`,
		`{"level":"info","msg":"a"}`,
		`{"level":"warn","msg":"b"}`,
		`{"level":"error","msg":"c"}`,
	}
	m.rebuildLogView()
	m = m.severityStep(+1)
	if m.logView.sevThreshold != ui.LogInfo {
		t.Fatalf("after +1 want LogInfo(%d), got %d", ui.LogInfo, m.logView.sevThreshold)
	}
	if len(m.logView.lines) != 3 {
		t.Fatalf("INFO+ should show 3 (info/warn/error), got %d", len(m.logView.lines))
	}
	m = m.severityStep(+1)
	if m.logView.sevThreshold != ui.LogWarn {
		t.Fatalf("after +2 want LogWarn(%d), got %d", ui.LogWarn, m.logView.sevThreshold)
	}
	if len(m.logView.lines) != 2 {
		t.Fatalf("WARN+ should show 2 (warn/error), got %d", len(m.logView.lines))
	}
	// Clamp at ERROR.
	for range 10 {
		m = m.severityStep(+1)
	}
	if m.logView.sevThreshold != ui.LogError {
		t.Fatalf("should clamp at LogError(%d), got %d", ui.LogError, m.logView.sevThreshold)
	}
	if len(m.logView.lines) != 1 {
		t.Fatalf("ERROR+ should show 1 (error), got %d", len(m.logView.lines))
	}
	// Clamp at off.
	for range 10 {
		m = m.severityStep(-1)
	}
	if m.logView.sevThreshold != 0 {
		t.Fatalf("should clamp at off, got %d", m.logView.sevThreshold)
	}
}

func TestAppendRawLogLine_SeverityFilter(t *testing.T) {
	var m Model
	m.logView.sevThreshold = ui.LogError
	m.appendRawLogLine(`{"level":"info","msg":"a"}`)  // structured INFO -> hidden
	m.appendRawLogLine(`{"level":"error","msg":"b"}`) // structured ERROR -> shown
	m.appendRawLogLine("plain tail with no keyword")  // plain -> INFO default -> hidden
	m.appendRawLogLine("connection failure detected") // plain error keyword -> shown
	if len(m.logView.rawLines) != 4 {
		t.Fatalf("rawLines = %d, want 4", len(m.logView.rawLines))
	}
	if len(m.logView.lines) != 2 {
		t.Fatalf("lines = %d, want 2 (json error + plain failure line): %v", len(m.logView.lines), m.logView.lines)
	}
}

func TestLogView_FilterAndSeverityIndicators(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogs
	m.logView.title = "logs"
	m.logView.rawLines = []string{`{"level":"error","msg":"boomtoken"}`}
	m.logView.filterQuery = "boomtoken"
	m.logView.sevThreshold = ui.LogWarn
	m.rebuildLogView()
	out := stripANSI(m.View())
	if !strings.Contains(out, "[F:boomtoken]") {
		t.Errorf("expected [F:boomtoken] filter indicator in title, got:\n%s", out)
	}
	if !strings.Contains(out, "[≥WARN]") {
		t.Errorf("expected [>=WARN] severity indicator in title, got:\n%s", out)
	}
}

func TestLogView_FilterPromptWhenActive(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogs
	m.logView.title = "logs"
	m.logView.rawLines = []string{"alpha", "beta"}
	m.logView.filterActive = true
	m.logView.filterInput.Set("alph")
	m.logView.filterQuery = "alph"
	m.rebuildLogView()
	out := stripANSI(m.View())
	if !strings.Contains(out, "esc:clear") {
		t.Errorf("expected filter prompt footer (esc:clear), got:\n%s", out)
	}
}

func TestCountVisibleRaw(t *testing.T) {
	var m Model
	lines := []string{
		`{"level":"info","msg":"a"}`,  // info bucket
		`{"level":"error","msg":"b"}`, // error bucket
		"plain no keyword",            // info default
		"connection failed",           // error keyword
	}
	if got := m.countVisibleRaw(lines); got != 4 {
		t.Fatalf("no filter: got %d, want 4", got)
	}
	m.logView.sevThreshold = ui.LogError
	if got := m.countVisibleRaw(lines); got != 2 {
		t.Fatalf("ERROR+: got %d, want 2 (two error-bucket lines)", got)
	}
	m.logView.filterQuery = "failed"
	if got := m.countVisibleRaw(lines); got != 1 {
		t.Fatalf("ERROR+ & \"failed\": got %d, want 1", got)
	}
}
