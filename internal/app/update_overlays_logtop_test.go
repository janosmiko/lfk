package app

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/logagg"
)

func TestLogTopGroupBy_TogglesAndApplies(t *testing.T) {
	m := newLogTopModel(t)
	m = m.openLogTopGroupBy()
	if m.overlay != overlayLogTopGroupBy {
		t.Fatalf("overlay = %v, want overlayLogTopGroupBy", m.overlay)
	}
	cands := m.logTopGroupByCandidates()
	want := map[string]bool{logagg.FieldMethod: true, logagg.FieldPath: true, logagg.FieldStatus: true}
	for k := range want {
		found := false
		for _, c := range cands {
			if c == k {
				found = true
			}
		}
		if !found {
			t.Errorf("candidate %q missing from %v", k, cands)
		}
	}
	// Apply with enter closes the overlay and rebuilds rows.
	mdl, _ := m.handleLogTopGroupByKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mdl.(Model)
	if m.overlay != overlayNone {
		t.Errorf("overlay after enter = %v, want overlayNone", m.overlay)
	}
}

func TestLogTopProfile_Switches(t *testing.T) {
	m := newLogTopModel(t)
	m = m.openLogTopProfile()
	if m.overlay != overlayLogTopProfile {
		t.Fatalf("overlay = %v, want overlayLogTopProfile", m.overlay)
	}
}

// TestLogTopGroupByOverlay_NoLineExceedsBoxWidth guards the overlay chrome fix:
// every rendered line must fit within the returned box width.
func TestLogTopGroupByOverlay_NoLineExceedsBoxWidth(t *testing.T) {
	m := newLogTopModel(t)
	m = m.openLogTopGroupBy()
	content, w, _ := m.renderLogTopGroupByOverlay()
	for line := range strings.SplitSeq(content, "\n") {
		lw := lipgloss.Width(line)
		if lw > w {
			t.Errorf("overlay line width %d > box width %d: %q", lw, w, stripANSI(line))
		}
	}
}

// TestLogTopGroupByCandidates_ExcludesDurationMS guards that duration_ms is
// excluded from group-by candidates (it is a continuous metric, not a grouping
// dimension).
func TestLogTopGroupByCandidates_ExcludesDurationMS(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogTop
	// CLF lines that include a duration field.
	m.logView.rawLines = []string{
		`10.0.0.1 - - [18/Jun/2026:10:00:00 +0000] "GET /api/v1 HTTP/1.1" 200 0 "-" "-" 1 "svc@kubernetes" "http://10.0.0.2" 5ms`,
		`10.0.0.1 - - [18/Jun/2026:10:00:01 +0000] "POST /api/v2 HTTP/1.1" 201 0 "-" "-" 2 "svc@kubernetes" "http://10.0.0.2" 10ms`,
	}
	m.logTopResetAndParse()
	cands := m.logTopGroupByCandidates()
	for _, c := range cands {
		if c == logagg.FieldDurationMS {
			t.Errorf("logTopGroupByCandidates() contains %q; it should be excluded", logagg.FieldDurationMS)
		}
	}
	// Sanity: method/path/status must still be present.
	for _, w := range []string{logagg.FieldMethod, logagg.FieldPath, logagg.FieldStatus} {
		if !slices.Contains(cands, w) {
			t.Errorf("expected candidate %q to be present, got %v", w, cands)
		}
	}
}
