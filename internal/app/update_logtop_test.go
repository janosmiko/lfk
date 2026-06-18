package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/logagg"
)

func newLogTopModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.mode = modeLogTop
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":200}`,
		`2026-06-18T10:00:01Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":500}`,
		`2026-06-18T10:00:02Z {"RequestMethod":"POST","RequestPath":"/b","DownstreamStatus":200}`,
	}
	m.logTopResetAndParse()
	return m
}

func TestLogTopKey_NavigateAndDrill(t *testing.T) {
	m := newLogTopModel(t)
	// move down
	mdl, _ := m.handleLogTopKey(key("j"))
	m = mdl.(Model)
	if m.logTop.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.logTop.cursor)
	}
	// drill into the selected group
	mdl, _ = m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mdl.(Model)
	if len(m.logTop.drillStack) == 0 {
		t.Error("expected drill frame on stack after enter")
	}
}

func TestLogTopKey_EscReturnsToLogs(t *testing.T) {
	m := newLogTopModel(t)
	mdl, _ := m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mdl.(Model)
	if m.mode != modeLogs {
		t.Errorf("mode after esc = %v, want modeLogs", m.mode)
	}
}

// TestLogTopKey_EscRebuildsLogView verifies that returning from Log Top via esc
// immediately populates the log viewer projection (logView.lines) from rawLines,
// so the user does not see a blank screen on return.
func TestLogTopKey_EscRebuildsLogView(t *testing.T) {
	m := newLogTopModel(t)
	if len(m.logView.rawLines) == 0 {
		t.Fatal("precondition: rawLines must be non-empty for this test to be meaningful")
	}
	mdl, _ := m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEsc})
	got := mdl.(Model)
	if got.mode != modeLogs {
		t.Fatalf("mode after esc = %v, want modeLogs", got.mode)
	}
	if len(got.logView.lines) == 0 {
		t.Errorf("logView.lines is empty after esc from Log Top; viewer would appear blank")
	}
}

func TestLogTopKey_GOpensGroupByOverlay(t *testing.T) {
	m := newLogTopModel(t)
	mdl, _ := m.handleLogTopKey(key("g"))
	got := mdl.(Model)
	if got.overlay != overlayLogTopGroupBy {
		t.Fatalf("pressing g: overlay = %v, want overlayLogTopGroupBy", got.overlay)
	}
}

func TestLogTopKey_POpensProfileOverlay(t *testing.T) {
	m := newLogTopModel(t)
	mdl, _ := m.handleLogTopKey(key("p"))
	got := mdl.(Model)
	if got.overlay != overlayLogTopProfile {
		t.Fatalf("pressing p: overlay = %v, want overlayLogTopProfile", got.overlay)
	}
}

// TestLogTopDrill_DescendAndReturn exercises the full drill-down/pop cycle:
// enter descends to status rows pinned to the selected path, esc restores
// the original groupBy and rows, and a second esc returns to log viewer mode.
func TestLogTopDrill_DescendAndReturn(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogTop
	// Two paths; /multi has two different statuses.
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/single","DownstreamStatus":200}`,
		`2026-06-18T10:00:01Z {"RequestMethod":"GET","RequestPath":"/multi","DownstreamStatus":200}`,
		`2026-06-18T10:00:02Z {"RequestMethod":"GET","RequestPath":"/multi","DownstreamStatus":500}`,
	}
	m.logTopResetAndParse()

	// After parse: groupBy should be [method, path]; should have 2 rows.
	if len(m.logTop.groupBy) == 0 {
		t.Fatal("groupBy empty after reset")
	}
	if len(m.logTop.rows) != 2 {
		t.Fatalf("initial rows = %d, want 2", len(m.logTop.rows))
	}

	// Find the /multi row and position cursor on it.
	multiIdx := -1
	for i, r := range m.logTop.rows {
		for _, v := range r.Values {
			if v == "/multi" {
				multiIdx = i
			}
		}
	}
	if multiIdx < 0 {
		t.Fatal("/multi row not found in initial rows")
	}
	m.logTop.cursor = multiIdx

	// Press enter -> drill into /multi.
	mdl, _ := m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mdl.(Model)

	if len(m.logTop.drillStack) != 1 {
		t.Fatalf("drillStack len = %d, want 1", len(m.logTop.drillStack))
	}
	if len(m.logTop.groupBy) != 1 || m.logTop.groupBy[0] != logagg.FieldStatus {
		t.Errorf("groupBy after drill = %v, want [status]", m.logTop.groupBy)
	}
	if len(m.logTop.rows) != 2 {
		t.Errorf("rows after drill = %d, want 2 (/multi has 200 and 500)", len(m.logTop.rows))
	}
	// Verify that only /multi's statuses appear (not /single's 200).
	for _, r := range m.logTop.rows {
		found := false
		for _, v := range r.Values {
			if v == "200" || v == "500" {
				found = true
			}
		}
		if !found {
			t.Errorf("unexpected row values in drill: %v", r.Values)
		}
	}

	// Press esc -> pop frame and restore.
	mdl, _ = m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mdl.(Model)

	if len(m.logTop.drillStack) != 0 {
		t.Errorf("drillStack not empty after esc: %v", m.logTop.drillStack)
	}
	// groupBy must be restored to method+path (2 fields).
	if len(m.logTop.groupBy) != 2 {
		t.Errorf("groupBy after esc = %v, want [method path]", m.logTop.groupBy)
	}
	if len(m.logTop.rows) != 2 {
		t.Errorf("rows after esc = %d, want 2 (full path list)", len(m.logTop.rows))
	}
	if m.mode != modeLogTop {
		t.Errorf("mode after first esc = %v, want modeLogTop", m.mode)
	}

	// Press esc again -> return to log viewer.
	mdl, _ = m.handleLogTopKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mdl.(Model)
	if m.mode != modeLogs {
		t.Errorf("mode after second esc = %v, want modeLogs", m.mode)
	}
}
