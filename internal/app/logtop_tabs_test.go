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
	m.logTop.sortKey = logagg.SortErr
	m.saveCurrentTab()

	// Mutate live state, then reload.
	m.logTop = logTopState{}
	m.loadTab(m.activeTab)

	if m.logTop.profile != logagg.ProfileTraefikJSON {
		t.Errorf("profile not restored: %q", m.logTop.profile)
	}
	if len(m.logTop.groupBy) != 2 || m.logTop.sortKey != logagg.SortErr {
		t.Errorf("groupBy/sort not restored: %v / %v", m.logTop.groupBy, m.logTop.sortKey)
	}
}
