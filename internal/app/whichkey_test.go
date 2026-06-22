package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// gotoTestModel returns an explorer model at LevelResourceTypes with a
// discovered resource set containing Pods and Deployments for the active context.
func gotoTestModel() Model {
	m := basePush80Model()
	m.nav.Context = "ctx"
	m.nav.Level = model.LevelResourceTypes
	m.discoveredResources = map[string][]model.ResourceTypeEntry{
		"ctx": {
			{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
			{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true},
		},
	}
	return m
}

func TestGotoResourceType_SwitchesType(t *testing.T) {
	m := gotoTestModel()
	out, _ := m.gotoResourceType("Pod", "")
	rm := out.(Model)
	if rm.nav.ResourceType.Kind != "Pod" {
		t.Fatalf("nav.ResourceType.Kind = %q, want Pod", rm.nav.ResourceType.Kind)
	}
	if rm.nav.Level != model.LevelResources {
		t.Fatalf("nav.Level = %v, want LevelResources", rm.nav.Level)
	}
}

func TestGotoResourceType_NoContextErrors(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	out, _ := m.gotoResourceType("Pod", "")
	rm := out.(Model)
	if !rm.statusMessageErr {
		t.Fatal("expected an error status when no cluster is selected")
	}
}

func TestGotoResourceType_TypeNotServedErrors(t *testing.T) {
	m := gotoTestModel()
	out, _ := m.gotoResourceType("Application", "argoproj.io")
	rm := out.(Model)
	if !rm.statusMessageErr {
		t.Fatal("expected an error status when the type is not discovered")
	}
}

func TestHandleGotoChord_NavigatesOnMatch(t *testing.T) {
	m := gotoTestModel()
	m.pendingG = true
	out, _, handled := m.handleGotoChord(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if !handled {
		t.Fatal("gp should be handled as a goto chord")
	}
	rm := out.(Model)
	if rm.nav.ResourceType.Kind != "Pod" {
		t.Fatalf("gp did not navigate to Pods, got %q", rm.nav.ResourceType.Kind)
	}
	if rm.pendingG {
		t.Fatal("pendingG should be cleared after a goto chord")
	}
}

func TestHandleGotoChord_GPassesThroughToJumpTop(t *testing.T) {
	m := gotoTestModel()
	m.pendingG = true
	_, _, handled := m.handleGotoChord(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if handled {
		t.Fatal("second g must NOT be consumed by goto; it belongs to jump-top")
	}
}

// An unregistered second key (e.g. gP) is swallowed: the popup closes and
// nothing happens — which-key semantics, no fall-through to the key's normal
// explorer action.
func TestHandleGotoChord_UnregisteredConsumesAndCloses(t *testing.T) {
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := gotoTestModel()
	m.pendingG = true
	m.whichKeyShown = true
	out, cmd, handled := m.handleGotoChord(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if !handled {
		t.Fatal("unregistered second key must be consumed (handled=true)")
	}
	if cmd != nil {
		t.Fatal("unregistered second key must be a noop (no command)")
	}
	rm := out.(Model)
	if rm.pendingG || rm.whichKeyShown {
		t.Fatal("unregistered second key must close the prefix and the popup")
	}
	// A real goto sets Level to LevelResources; an unregistered key must leave
	// the nav level untouched (here: LevelResourceTypes).
	if rm.nav.Level != model.LevelResourceTypes {
		t.Fatalf("unregistered second key must not navigate; nav.Level = %v", rm.nav.Level)
	}
	if rm.statusMessageErr {
		t.Fatal("unregistered second key must be silent (no error status)")
	}
}

// esc (and any non-g key) is swallowed while the prefix is armed: it closes
// the popup as a noop instead of falling through to its normal explorer action.
func TestHandleGotoChord_EscClosesPopup(t *testing.T) {
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := gotoTestModel()
	m.pendingG = true
	m.whichKeyShown = true
	out, cmd, handled := m.handleGotoChord(tea.KeyMsg{Type: tea.KeyEsc})
	if !handled || cmd != nil {
		t.Fatal("esc must be consumed as a noop while the prefix is armed")
	}
	rm := out.(Model)
	if rm.pendingG || rm.whichKeyShown {
		t.Fatal("esc must close the prefix and the popup")
	}
}

// Completing gg (jump to top) must clear whichKeyShown along with pendingG so
// no stale visibility flag lingers.
func TestExplorerJumpTop_GGClearsWhichKeyShown(t *testing.T) {
	m := gotoTestModel()
	m.pendingG = true
	m.whichKeyShown = true
	out, _ := m.handleExplorerJumpTop()
	rm := out.(Model)
	if rm.pendingG || rm.whichKeyShown {
		t.Fatalf("gg must clear both pendingG and whichKeyShown; got pendingG=%v whichKeyShown=%v", rm.pendingG, rm.whichKeyShown)
	}
}

func TestLayoutWhichKey(t *testing.T) {
	mk := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = "x x" // width 3
		}
		return out
	}

	sumOf := func(xs []int) int {
		s := 0
		for _, x := range xs {
			s += x
		}
		return s
	}

	// 15 entries -> 4 columns, ceil(15/4)=4 rows; stretched to the target width
	// with the slack in the gaps and no trailing space past the last column
	// (inner == sum(colW) + sum(gaps)).
	lay := layoutWhichKey(mk(15), 100, 200)
	if len(lay.colW) != 4 || lay.rows != 4 {
		t.Fatalf("15 entries: want 4 cols / 4 rows, got %d cols / %d rows", len(lay.colW), lay.rows)
	}
	if lay.inner != 100 {
		t.Fatalf("grid must stretch to target inner 100, got %d", lay.inner)
	}
	if sumOf(lay.colW)+sumOf(lay.gaps) != lay.inner {
		t.Fatalf("no-trailing-gap invariant broken: cols=%v gaps=%v inner=%d", lay.colW, lay.gaps, lay.inner)
	}

	// Adding entries keeps 4 columns and grows rows (expand vertically).
	lay2 := layoutWhichKey(mk(23), 100, 200)
	if len(lay2.colW) != 4 || lay2.rows != 6 { // ceil(23/4)=6
		t.Fatalf("23 entries: want 4 cols / 6 rows, got %d cols / %d rows", len(lay2.colW), lay2.rows)
	}

	// Each column is sized to its own widest entry. 4 entries, 4 columns, 1 row.
	wide := []string{"short", "a-very-long-entry", "short", "short"}
	lay3 := layoutWhichKey(wide, 100, 200)
	if lay3.colW[1] != len("a-very-long-entry") || lay3.colW[0] != len("short") {
		t.Fatalf("per-column widths wrong: %v", lay3.colW)
	}

	// Narrow terminal reduces the column count to fit.
	lay4 := layoutWhichKey(mk(15), 5, 5)
	if len(lay4.colW) >= 4 {
		t.Fatalf("narrow width must reduce columns, got %d", len(lay4.colW))
	}
}

func TestGotoTargets_IncludesBuiltins(t *testing.T) {
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := gotoTestModel()
	byChord := map[string]string{}
	for _, gt := range m.gotoTargets() {
		byChord[gt.Chord] = gt.Kind
	}
	if byChord["gp"] != "Pod" || byChord["gd"] != "Deployment" {
		t.Fatalf("built-in goto targets missing: %+v", byChord)
	}
}

// TestGotoResourceType_LeftPaneIsResourceTypes verifies that after a goto fired
// from LevelResources, the left pane holds the resource-types sidebar so the
// user can press h/left to return to the types list correctly.
func TestGotoResourceType_LeftPaneIsResourceTypes(t *testing.T) {
	m := gotoTestModel()
	// Start at LevelResources (already at types level for gotoTestModel, but
	// simulate being at resources level with leftItems = resource-types sidebar).
	m.nav.Level = model.LevelResourceTypes
	// Descend: push left so leftItems becomes the resource-types sidebar.
	typesItems := model.BuildSidebarItems(m.discoveredResources["ctx"])
	m.setMiddleItems(typesItems)
	m.pushLeft()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1"}

	// Now fire goto Deployment from LevelResources.
	out, _ := m.gotoResourceType("Deployment", "apps")
	rm := out.(Model)

	if rm.nav.ResourceType.Kind != "Deployment" {
		t.Fatalf("nav.ResourceType.Kind = %q, want Deployment", rm.nav.ResourceType.Kind)
	}
	// leftItems should be resource-types sidebar (contains Pod and Deployment items).
	kinds := map[string]bool{}
	for _, item := range rm.leftItems {
		if item.Kind != "__collapsed_group__" {
			kinds[item.Kind] = true
		}
	}
	if !kinds["Pod"] && !kinds["Deployment"] {
		t.Fatalf("left pane does not hold resource-type items after goto; kinds=%v", kinds)
	}
}
