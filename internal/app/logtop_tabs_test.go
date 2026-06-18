package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/logagg"
)

func TestLogTop_TabRoundTrip(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogTop
	m.logTop.profile = logagg.ProfileTraefikJSON
	m.logTop.groupBy = []string{logagg.FieldMethod, logagg.FieldPath}
	m.logTop.sortCol = logTopMetricERR
	m.logTop.sortAsc = true
	m.saveCurrentTab()

	// Mutate live state, then reload.
	m.logTop = logTopState{}
	m.loadTab(m.activeTab)

	if m.logTop.profile != logagg.ProfileTraefikJSON {
		t.Errorf("profile not restored: %q", m.logTop.profile)
	}
	if len(m.logTop.groupBy) != 2 {
		t.Errorf("groupBy not restored: %v", m.logTop.groupBy)
	}
	if m.logTop.sortCol != logTopMetricERR || !m.logTop.sortAsc {
		t.Errorf("sortCol/sortAsc not restored: %v / %v", m.logTop.sortCol, m.logTop.sortAsc)
	}
}
