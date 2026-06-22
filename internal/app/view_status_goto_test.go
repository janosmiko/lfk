package app

import "testing"

func TestExplorerHintEntries_IncludesGoto(t *testing.T) {
	m := gotoTestModel()
	found := false
	for _, h := range m.explorerHintEntries() {
		if h.Desc == "goto" {
			found = true
		}
	}
	if !found {
		t.Fatal("explorer hint bar missing a goto hint")
	}
}
