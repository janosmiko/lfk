package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
)

// TestApplyTextFilter_FuzzySort_ScoresAcceptedField reproduces the bug where
// an item pulled in by a strong Category match gets ranked by its (weak or
// absent) Name match instead, because the sort only ever scored Name.
func TestApplyTextFilter_FuzzySort_ScoresAcceptedField(t *testing.T) {
	m := newTestModel()
	m.filterBroadMode = true
	m.nav.Level = model.LevelResourceTypes
	m.filterText = "~net"

	items := []model.Item{
		{Name: "node-exporter-tool", Category: "Storage"},
		{Name: "foobar-service", Category: "Networking"},
	}

	got := m.applyTextFilter(items)

	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(got), got)
	}
	if got[0].Category != "Networking" {
		t.Fatalf("expected the strong Category match to rank first, got order: %q then %q",
			got[0].Name, got[1].Name)
	}
}
