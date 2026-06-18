package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
