package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	if len(m.logTop.drillField) == 0 {
		t.Error("expected drill constraint after enter")
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
