package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestSelectRange_RealCtrlSpaceKeypress feeds the key exactly as a terminal
// delivers it under Bubble Tea v2 — Code: KeySpace, Mod: ModCtrl — rather than
// re-encoding the configured binding string, which would only confirm the
// spelling the config already assumes. The default binding was "ctrl+@", a
// spelling v2 emits only under LegacyKeyEncoding.CtrlAt(true); this app never
// opts in, so the dispatch case never matched a real press.
func TestSelectRange_RealCtrlSpaceKeypress(t *testing.T) {
	prev := ui.ActiveKeybindings
	t.Cleanup(func() { ui.ActiveKeybindings = prev })
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := basePush4Model()
	m.nav.Level = model.LevelResources
	m.selectionAnchor = -1

	ctrlSpace := tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl}
	out, _ := m.handleKey(ctrlSpace)
	rm := out.(Model)

	if rm.selectionAnchor != 0 {
		t.Fatalf("ctrl+space (%q) did not reach the range handler: selectionAnchor = %d, want 0",
			ctrlSpace.String(), rm.selectionAnchor)
	}
	if len(rm.selectedItems) != 1 {
		t.Fatalf("ctrl+space (%q) selected %d items, want 1", ctrlSpace.String(), len(rm.selectedItems))
	}

	// Anchor set on row 0; move to row 2 and press again — the range 0..2 must
	// all become selected.
	rm.setCursor(2)
	out2, _ := rm.handleKey(ctrlSpace)
	rm2 := out2.(Model)
	if len(rm2.selectedItems) != 3 {
		t.Fatalf("ctrl+space range from anchor 0 to cursor 2 selected %d items, want 3", len(rm2.selectedItems))
	}
}

// TestSelectRange_LegacyCtrlAtConfigStillWorks pins the compatibility half of
// the fix: a user config that already spells the chord "ctrl+@" must keep
// working, since that is what every lfk config written before this fix says.
func TestSelectRange_LegacyCtrlAtConfigStillWorks(t *testing.T) {
	prev := ui.ActiveKeybindings
	t.Cleanup(func() { ui.ActiveKeybindings = prev })
	kb := ui.DefaultKeybindings()
	kb.SelectRange = "ctrl+@"
	ui.ActiveKeybindings = kb

	m := basePush4Model()
	m.nav.Level = model.LevelResources
	m.selectionAnchor = -1

	out, _ := m.handleKey(tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl})
	rm := out.(Model)
	if rm.selectionAnchor != 0 {
		t.Fatalf("legacy select_range: \"ctrl+@\" config broke: selectionAnchor = %d, want 0", rm.selectionAnchor)
	}
}
