package app

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

func TestKeymapsOverlayItems_ReturnsNonEmptyWithKeyAndName(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()

	items := m.keymapsOverlayItems()
	if len(items) == 0 {
		t.Fatal("expected at least one keymap item for the explorer catalog")
	}
	for _, it := range items {
		if it.displayKey == "" {
			t.Errorf("item %q has no displayed key", it.Name)
		}
		if it.Name == "" {
			t.Error("item has no label")
		}
		if it.rawKey == "" {
			t.Errorf("item %q has no rawKey", it.Name)
		}
	}
}

func TestKeymapsFilteredItems_FiltersByText(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()

	all := m.keymapsOverlayItems()
	if len(all) < 2 {
		t.Fatal("need at least two catalog entries for a meaningful filter test")
	}
	target := all[0].Name

	m.keymapsFilter.Set(target)
	filtered := m.filteredKeymapsItems()
	if len(filtered) == 0 {
		t.Fatalf("filtering by %q dropped every item", target)
	}
	for _, it := range filtered {
		if !ui.MatchLine(it.Name, target) && !ui.MatchLine(it.Description, target) && !ui.MatchLine(it.displayKey, target) {
			t.Errorf("item %q does not match filter %q", it.Name, target)
		}
	}

	m.keymapsFilter.Set("this-should-match-nothing-xyz")
	if got := m.filteredKeymapsItems(); len(got) != 0 {
		t.Errorf("expected no matches, got %d", len(got))
	}
}

func TestKeymapsOverlayKey_EnterClosesAndReturnsCmd(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel().openKeymapsOverlay()

	mdl, cmd := m.handleKeymapsOverlayKey(keyMsg("enter"))
	out := mdl.(Model)
	if out.overlay != overlayNone {
		t.Errorf("overlay should close on enter, got %v", out.overlay)
	}
	if cmd == nil {
		t.Fatal("enter must return a cmd that replays the selected key")
	}
	if _, ok := cmd().(interface{ String() string }); !ok {
		t.Fatalf("cmd must produce a message, got %T", cmd())
	}
}

func TestKeymapsOverlayKey_EscCloses(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel().openKeymapsOverlay()

	mdl, _ := m.handleKeymapsOverlayKey(keyMsg("esc"))
	out := mdl.(Model)
	if out.overlay != overlayNone {
		t.Errorf("overlay should close on esc, got %v", out.overlay)
	}
}

func TestKeymapsOverlayKey_CursorMovesWithJK(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel().openKeymapsOverlay()

	if len(m.filteredKeymapsItems()) < 2 {
		t.Fatal("need at least two catalog entries to test cursor movement")
	}

	mdl, _ := m.handleKeymapsOverlayKey(keyMsg("j"))
	out := mdl.(Model)
	if out.keymapsCursor != 1 {
		t.Fatalf("j should move cursor to 1, got %d", out.keymapsCursor)
	}

	mdl, _ = out.handleKeymapsOverlayKey(keyMsg("k"))
	out = mdl.(Model)
	if out.keymapsCursor != 0 {
		t.Fatalf("k should move cursor back to 0, got %d", out.keymapsCursor)
	}
}

func TestKeymapsOverlayItems_BadgeContainsGroupAndKey(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()

	items := m.keymapsOverlayItems()
	for _, it := range items {
		if it.Badge == "" {
			t.Errorf("item %q has no badge", it.Name)
		}
		if it.Description == "" {
			t.Errorf("item %q has no group in Description (needed for filter matching)", it.Name)
		}
	}
}

func TestKeymapsOverlay_OpensInNormalModeAndSlashFilters(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel().openKeymapsOverlay()
	if m.keymapsFilterMode {
		t.Fatal("keymaps overlay should open in normal mode")
	}
	mdl, _ := m.handleKeymapsOverlayKey(keyMsg("/"))
	if !mdl.(Model).keymapsFilterMode {
		t.Fatal("/ should enter filter mode")
	}
}

func TestRenderOverlayKeymaps_LastItemVisibleAfterG(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel().openKeymapsOverlay()
	m.height = 30
	overlayKeymapsScrollPos = 0
	t.Cleanup(func() { overlayKeymapsScrollPos = 0 })

	mdl, _ := m.handleKeymapsOverlayKey(keyMsg("G"))
	m = mdl.(Model)
	items := m.filteredKeymapsItems()
	last := items[len(items)-1].Name

	view, _, _ := m.renderOverlayKeymaps()
	if !strings.Contains(stripANSI(view), last) {
		t.Fatalf("last item %q not rendered after G:\n%s", last, stripANSI(view))
	}
}

func TestKeymapsWhichKeyLeader_OpensOverlay(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()

	mdl, cmd := m.handleExplorerKey(keyMsg(ui.ActiveKeybindings.WhichKeyLeader))
	out := mdl.(Model)
	if out.overlay != overlayKeymaps {
		t.Fatalf("leader key should open the keymaps overlay, got overlay=%v", out.overlay)
	}
	if cmd != nil {
		t.Error("opening the overlay should not schedule a cmd")
	}
}
