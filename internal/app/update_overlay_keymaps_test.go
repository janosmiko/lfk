package app

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
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

func TestKeymapsOverlayItems_IncludesNewTab(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()

	items := m.keymapsOverlayItems()
	found := false
	for _, it := range items {
		if it.Name == "New tab" {
			found = true
			if it.displayKey != "t" {
				t.Errorf("New tab displayKey = %q, want %q", it.displayKey, "t")
			}
		}
	}
	if !found {
		t.Fatal("keymaps overlay is missing \"New tab\"")
	}
}

func TestDispatchKeymapsSelection_GotoChordReplaysBothKeys(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := gotoTestModel().openKeymapsOverlay()

	var target keymapItem
	found := false
	for _, it := range m.keymapsOverlayItems() {
		if it.Name == "Go to Deployments" {
			target = it
			found = true
		}
	}
	if !found {
		t.Fatal("keymaps overlay is missing \"Go to Deployments\"")
	}
	if target.rawKey != "gd" {
		t.Fatalf("rawKey = %q, want %q", target.rawKey, "gd")
	}

	mdl, cmd := dispatchKeymapsSelection(m, target)
	if cmd == nil {
		t.Fatal("dispatching a goto chord must return a replay cmd")
	}
	chord, ok := cmd().(keymapsChordMsg)
	if !ok || len(chord) != 2 {
		t.Fatalf("cmd produced %T, want a 2-key keymapsChordMsg", cmd())
	}
	if chord[0].String() != "g" || chord[1].String() != "d" {
		t.Fatalf("chord keys = %q,%q, want \"g\",\"d\"", chord[0].String(), chord[1].String())
	}

	out, _ := mdl.(Model).Update(chord)
	final := out.(Model)
	if final.nav.Level != model.LevelResources {
		t.Fatalf("nav.Level = %v, want LevelResources", final.nav.Level)
	}
	if final.nav.ResourceType.Kind != "Deployment" {
		t.Fatalf("nav.ResourceType.Kind = %q, want Deployment", final.nav.ResourceType.Kind)
	}
}

// A rebound Goto* field can be any chord IsSingleKeypress accepts, not just a
// single plain rune (docs/config-reference.md cites "gctrl+p" as valid).
func TestDispatchKeymapsSelection_GotoChordWithModifierSuffix(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ActiveKeybindings.GotoDeployments = "gctrl+p"
	m := gotoTestModel().openKeymapsOverlay()

	var target keymapItem
	found := false
	for _, it := range m.keymapsOverlayItems() {
		if it.Name == "Go to Deployments" {
			target = it
			found = true
		}
	}
	if !found {
		t.Fatal("keymaps overlay is missing \"Go to Deployments\"")
	}
	if target.rawKey != "gctrl+p" {
		t.Fatalf("rawKey = %q, want %q", target.rawKey, "gctrl+p")
	}

	mdl, cmd := dispatchKeymapsSelection(m, target)
	chord, ok := cmd().(keymapsChordMsg)
	if !ok || len(chord) != 2 {
		t.Fatalf("cmd produced %T, want a 2-key keymapsChordMsg", cmd())
	}
	if chord[0].String() != "g" || chord[1].String() != "ctrl+p" {
		t.Fatalf("chord keys = %q,%q, want g and ctrl+p", chord[0].String(), chord[1].String())
	}
	out, _ := mdl.(Model).Update(chord)
	final := out.(Model)
	if final.nav.ResourceType.Kind != "Deployment" {
		t.Fatalf("nav.ResourceType.Kind = %q, want Deployment", final.nav.ResourceType.Kind)
	}
}

// A named-key suffix ("gtab") must dispatch the same as a plain-rune one.
func TestDispatchKeymapsSelection_GotoChordWithNamedKeySuffix(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ActiveKeybindings.GotoDeployments = "gtab"
	m := gotoTestModel().openKeymapsOverlay()

	var target keymapItem
	for _, it := range m.keymapsOverlayItems() {
		if it.Name == "Go to Deployments" {
			target = it
		}
	}
	if target.rawKey != "gtab" {
		t.Fatalf("rawKey = %q, want %q", target.rawKey, "gtab")
	}

	mdl, cmd := dispatchKeymapsSelection(m, target)
	chord, ok := cmd().(keymapsChordMsg)
	if !ok || len(chord) != 2 {
		t.Fatalf("cmd produced %T, want a 2-key keymapsChordMsg", cmd())
	}
	if chord[0].String() != "g" || chord[1].String() != "tab" {
		t.Fatalf("chord keys = %q,%q, want g and tab", chord[0].String(), chord[1].String())
	}
	out, _ := mdl.(Model).Update(chord)
	final := out.(Model)
	if final.nav.ResourceType.Kind != "Deployment" {
		t.Fatalf("nav.ResourceType.Kind = %q, want Deployment", final.nav.ResourceType.Kind)
	}
}

// A rebound JumpTop prefix must still be recognized and split correctly.
func TestDispatchKeymapsSelection_GotoChordWithReboundPrefix(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ActiveKeybindings.JumpTop = "z"
	ui.ActiveKeybindings.GotoDeployments = "zctrl+p"
	m := gotoTestModel().openKeymapsOverlay()

	var target keymapItem
	for _, it := range m.keymapsOverlayItems() {
		if it.Name == "Go to Deployments" {
			target = it
		}
	}
	if target.rawKey != "zctrl+p" {
		t.Fatalf("rawKey = %q, want %q", target.rawKey, "zctrl+p")
	}

	_, cmd := dispatchKeymapsSelection(m, target)
	chord, ok := cmd().(keymapsChordMsg)
	if !ok || len(chord) != 2 {
		t.Fatalf("cmd produced %T, want a 2-key keymapsChordMsg", cmd())
	}
	if chord[0].String() != "z" || chord[1].String() != "ctrl+p" {
		t.Fatalf("chord keys = %q,%q, want z and ctrl+p", chord[0].String(), chord[1].String())
	}
}

func TestIsGotoChordRawKey_AcceptsAnyIsSingleKeypressSuffix(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ActiveKeybindings.JumpTop = "g"

	cases := map[string]bool{
		"gd":       true,
		"gctrl+p":  true,
		"gtab":     true,
		"gshift+a": true,
		"g":        false, // prefix alone is not a chord
		"d":        false, // no prefix at all
		"gg":       true,  // second "g" is itself a single keypress suffix
	}
	for raw, want := range cases {
		if got := isGotoChordRawKey(raw); got != want {
			t.Errorf("isGotoChordRawKey(%q) = %v, want %v", raw, got, want)
		}
	}
}

// GotoPods was mistakenly left reachable only from the g-prefix popup.
func TestKeymapsOverlayItems_IncludesRemainingGotoChords(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := gotoTestModel().openKeymapsOverlay()

	var target keymapItem
	found := false
	for _, it := range m.keymapsOverlayItems() {
		if it.Name == "Go to Pods" {
			target = it
			found = true
		}
	}
	if !found {
		t.Fatal("keymaps overlay is missing \"Go to Pods\"")
	}
	if target.rawKey != "gp" {
		t.Fatalf("rawKey = %q, want %q", target.rawKey, "gp")
	}

	mdl, cmd := dispatchKeymapsSelection(m, target)
	chord, ok := cmd().(keymapsChordMsg)
	if !ok || len(chord) != 2 {
		t.Fatalf("cmd produced %T, want a 2-key keymapsChordMsg", cmd())
	}
	out, _ := mdl.(Model).Update(chord)
	final := out.(Model)
	if final.nav.ResourceType.Kind != "Pod" {
		t.Fatalf("nav.ResourceType.Kind = %q, want Pod", final.nav.ResourceType.Kind)
	}
}

// The single-char chord "g\\" must replay the same as any other goto chord.
func TestDispatchKeymapsSelection_PreviousNamespaceReplaysChord(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.previousNsScope = &nsScope{namespace: "kube-system"}
	m = m.openKeymapsOverlay()

	var target keymapItem
	found := false
	for _, it := range m.keymapsOverlayItems() {
		if it.Name == "Previous namespace" {
			target = it
			found = true
		}
	}
	if !found {
		t.Fatal("keymaps overlay is missing \"Previous namespace\"")
	}
	if target.rawKey != `g\` {
		t.Fatalf("rawKey = %q, want %q", target.rawKey, `g\`)
	}

	_, cmd := dispatchKeymapsSelection(m, target)
	chord, ok := cmd().(keymapsChordMsg)
	if !ok || len(chord) != 2 {
		t.Fatalf("cmd produced %T, want a 2-key keymapsChordMsg", cmd())
	}
	if chord[0].String() != "g" || chord[1].String() != `\` {
		t.Fatalf("chord keys = %q,%q, want g and backslash", chord[0].String(), chord[1].String())
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

func TestRenderOverlayKeymaps_FilterShowsModeLabel(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel().openKeymapsOverlay()
	m.height = 30
	m.keymapsFilterMode = true
	m.keymapsFilter.Set("~nwtb")

	view, _, _ := m.renderOverlayKeymaps()
	if !strings.Contains(stripANSI(view), "[fuzzy]") {
		t.Errorf("keymaps filter must show the fuzzy mode label for a ~ query:\n%s", stripANSI(view))
	}
}
