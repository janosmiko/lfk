package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// restoreWhichKeyGlobals snapshots the package-global UI config the which-key
// tests mutate and restores it after the test, so test order can't leak state.
func restoreWhichKeyGlobals(t *testing.T) {
	t.Helper()
	kb := ui.ActiveKeybindings
	enabled := ui.ConfigWhichKeyEnabled
	delay := ui.ConfigWhichKeyDelayMs
	leaderDelay := ui.ConfigWhichKeyLeaderDelayMs
	dim := ui.ConfigDimOverlay
	t.Cleanup(func() {
		ui.ActiveKeybindings = kb
		ui.ConfigWhichKeyEnabled = enabled
		ui.ConfigWhichKeyDelayMs = delay
		ui.ConfigWhichKeyLeaderDelayMs = leaderDelay
		ui.ConfigDimOverlay = dim
	})
}

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
	out, _, handled := m.handleGotoChord(tea.KeyPressMsg{Code: 'p', Text: "p"})
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

func TestHandleGotoChord_PreviousNamespace(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := gotoTestModel()
	m.nav.Level = model.LevelResources
	m.namespace = "kube-system"
	m.selectedNamespaces = map[string]bool{"kube-system": true}
	m.previousNsScope = &nsScope{namespace: "default", selectedNamespaces: map[string]bool{"default": true}}
	m.pendingG = true
	out, _, handled := m.handleGotoChord(tea.KeyPressMsg{Code: '\\', Text: "\\"})
	if !handled {
		t.Fatal(`g\ should be handled`)
	}
	rm := out.(Model)
	if rm.namespace != "default" {
		t.Fatalf(`g\ did not jump to previous namespace, got %q`, rm.namespace)
	}
	if !rm.selectedNamespaces["default"] || rm.selectedNamespaces["kube-system"] {
		t.Fatalf("selectedNamespaces not swapped to previous scope: %v", rm.selectedNamespaces)
	}
}

// With PreviousNamespace unbound, g\ is not a previous-namespace jump and
// falls through to the goto-chord lookup (no such target -> consumed as an
// unmapped chord, namespace untouched).
func TestHandleGotoChord_PreviousNamespaceUnbound(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	kb.PreviousNamespace = ""
	ui.ActiveKeybindings = kb
	m := gotoTestModel()
	m.nav.Level = model.LevelResources
	m.namespace = "kube-system"
	m.previousNsScope = &nsScope{namespace: "default"}
	m.pendingG = true
	out, _, handled := m.handleGotoChord(tea.KeyPressMsg{Code: '\\', Text: "\\"})
	if !handled {
		t.Fatal(`g\ should still be consumed as an unmapped chord`)
	}
	if rm := out.(Model); rm.namespace != "kube-system" {
		t.Fatalf("unbound g\\ must not jump; namespace changed to %q", rm.namespace)
	}
}

func TestHandleGotoChord_GPassesThroughToJumpTop(t *testing.T) {
	m := gotoTestModel()
	m.pendingG = true
	_, _, handled := m.handleGotoChord(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if handled {
		t.Fatal("second g must NOT be consumed by goto; it belongs to jump-top")
	}
}

// An unregistered second key (e.g. gP) is swallowed: the popup closes and
// nothing happens — which-key semantics, no fall-through to the key's normal
// explorer action.
func TestHandleGotoChord_UnregisteredConsumesAndCloses(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := gotoTestModel()
	m.pendingG = true
	m.whichKey.shown = true
	out, cmd, handled := m.handleGotoChord(tea.KeyPressMsg{Code: 'P', Text: "P"})
	if !handled {
		t.Fatal("unregistered second key must be consumed (handled=true)")
	}
	if cmd != nil {
		t.Fatal("unregistered second key must be a noop (no command)")
	}
	rm := out.(Model)
	if rm.pendingG || rm.whichKey.shown {
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
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := gotoTestModel()
	m.pendingG = true
	m.whichKey.shown = true
	out, cmd, handled := m.handleGotoChord(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !handled || cmd != nil {
		t.Fatal("esc must be consumed as a noop while the prefix is armed")
	}
	rm := out.(Model)
	if rm.pendingG || rm.whichKey.shown {
		t.Fatal("esc must close the prefix and the popup")
	}
}

// Completing gg (jump to top) must clear whichKey.shown along with pendingG so
// no stale visibility flag lingers.
func TestExplorerJumpTop_GGClearsWhichKeyShown(t *testing.T) {
	m := gotoTestModel()
	m.pendingG = true
	m.whichKey.shown = true
	out, _ := m.handleExplorerJumpTop()
	rm := out.(Model)
	if rm.pendingG || rm.whichKey.shown {
		t.Fatalf("gg must clear both pendingG and whichKey.shown; got pendingG=%v whichKey.shown=%v", rm.pendingG, rm.whichKey.shown)
	}
}

func TestLayoutWhichKey(t *testing.T) {
	// layoutWhichKey takes pre-measured display widths, so these build the
	// widths the equivalent cell strings would have measured to.
	mk := func(n int) []int {
		out := make([]int, n)
		for i := range out {
			out[i] = len("x x") // width 3
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

	// 15 entries -> whichKeyMaxCols columns, ceil(15/cols) rows; stretched to
	// the target width with the slack in the gaps and no trailing space past
	// the last column (inner == sum(colW) + sum(gaps)). Expressed against the
	// constant rather than a literal so a geometry re-tune does not silently
	// turn this into a test of the old shape.
	cols := whichKeyMaxCols
	lay := layoutWhichKey(mk(15), 100, 200)
	if wantRows := (15 + cols - 1) / cols; len(lay.colW) != cols || lay.rows != wantRows {
		t.Fatalf("15 entries: want %d cols / %d rows, got %d cols / %d rows", cols, wantRows, len(lay.colW), lay.rows)
	}
	if lay.inner != 100 {
		t.Fatalf("grid must stretch to target inner 100, got %d", lay.inner)
	}
	if sumOf(lay.colW)+sumOf(lay.gaps) != lay.inner {
		t.Fatalf("no-trailing-gap invariant broken: cols=%v gaps=%v inner=%d", lay.colW, lay.gaps, lay.inner)
	}

	// Adding entries keeps the column count and grows rows (expand vertically).
	lay2 := layoutWhichKey(mk(23), 100, 200)
	if wantRows := (23 + cols - 1) / cols; len(lay2.colW) != cols || lay2.rows != wantRows {
		t.Fatalf("23 entries: want %d cols / %d rows, got %d cols / %d rows", cols, wantRows, len(lay2.colW), lay2.rows)
	}

	// Each column is sized to its own widest entry. 4 entries, 4 columns, 1 row.
	wide := []int{len("short"), len("a-very-long-entry"), len("short"), len("short")}
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
	restoreWhichKeyGlobals(t)
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
	if !kinds["Pod"] || !kinds["Deployment"] {
		t.Fatalf("left pane does not hold resource-type items after goto; kinds=%v", kinds)
	}
}

// TestGotoResourceType_ClearsQuickFilter verifies that a goto jump between
// resource types clears the committed quick filter instead of leaking it into
// the destination list (TASK-839: pods' filter hid every deployment after gd).
// The old level's filter must still be remembered for back-nav restore.
func TestGotoResourceType_ClearsQuickFilter(t *testing.T) {
	m := gotoTestModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.filterText = "nginx"
	m.filterInput.Set("nginx")
	m.filterBroadMode = true
	m.searchInput.Set("nginx")
	m.activeFilterPreset = &FilterPreset{Name: "p"}
	m.unfilteredMiddleItems = []model.Item{{Name: "pod-a"}}
	oldKey := m.navKey()

	out, _ := m.gotoResourceType("Deployment", "apps")
	rm := out.(Model)

	if rm.filterText != "" || rm.filterInput.Value != "" {
		t.Fatalf("quick filter leaked into destination: filterText=%q filterInput=%q", rm.filterText, rm.filterInput.Value)
	}
	if rm.filterActive {
		t.Fatal("filterActive must be false after a goto jump")
	}
	if rm.filterBroadMode {
		t.Fatal("filterBroadMode must not carry into the destination")
	}
	if rm.activeFilterPreset != nil || rm.unfilteredMiddleItems != nil {
		t.Fatal("filter preset state must be cleared by a goto jump")
	}
	if rm.searchInput.Value != "" {
		t.Fatalf("search highlight must not bleed into destination, got %q", rm.searchInput.Value)
	}
	if f, ok := rm.filterMemory[oldKey]; !ok || f.text != "nginx" || !f.broad {
		t.Fatalf("old level's filter (incl. broad mode) must be saved for back-nav restore; got %+v (ok=%v)", f, ok)
	}
}

// TestGotoResourceType_DoesNotRestoreDestinationFilter pins the product
// decision that a goto is a fresh start like a descend: a filter previously
// committed on the destination list is NOT re-applied by the jump (only
// back-nav via navigateParent restores saved filters).
func TestGotoResourceType_DoesNotRestoreDestinationFilter(t *testing.T) {
	m := gotoTestModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	// Simulate a filter remembered for the Deployments list from an earlier visit.
	probe := m
	probe.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true}
	m.filterMemory = map[string]savedFilter{probe.navKey(): {text: "old-deploy-filter"}}

	out, _ := m.gotoResourceType("Deployment", "apps")
	rm := out.(Model)

	if rm.filterText != "" || rm.filterInput.Value != "" {
		t.Fatalf("goto must not restore the destination's saved filter; got filterText=%q filterInput=%q", rm.filterText, rm.filterInput.Value)
	}
}

// TestGotoResourceType_BackNavLandsOnJumpedType verifies that after jumping to a
// resource type via goto (gv) and pressing h/left to go back, the cursor lands on
// the resource type that was jumped to rather than a stale or default highlight.
func TestGotoResourceType_BackNavLandsOnJumpedType(t *testing.T) {
	m := gotoTestModel() // LevelResourceTypes, ctx with Pod + Deployment
	// Seed the types list as the middle column with the cursor on the first
	// item (Pods), simulating a real session before the jump.
	typesItems := model.BuildSidebarItems(m.discoveredResources["ctx"])
	m.setMiddleItems(typesItems)
	m.setCursor(0)

	// Jump to Deployment (gv -> Deployment).
	out, _ := m.gotoResourceType("Deployment", "apps")
	m = out.(Model)
	if m.nav.Level != model.LevelResources {
		t.Fatalf("nav.Level = %v, want LevelResources", m.nav.Level)
	}

	// Press h/left to return to the resource-types list.
	out, _ = m.navigateParent()
	rm := out.(Model)
	if rm.nav.Level != model.LevelResourceTypes {
		t.Fatalf("nav.Level = %v, want LevelResourceTypes after back-nav", rm.nav.Level)
	}

	visible := rm.visibleMiddleItems()
	c := rm.cursor()
	if c < 0 || c >= len(visible) {
		t.Fatalf("cursor %d out of range (visible=%d)", c, len(visible))
	}
	want := model.ResourceTypeEntry{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments"}
	if got := visible[c]; got.Extra != want.ResourceRef() {
		t.Fatalf("cursor landed on %q (%s), want Deployment (%s)", got.Name, got.Extra, want.ResourceRef())
	}
}
