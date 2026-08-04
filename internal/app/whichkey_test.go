package app

import (
	"strings"
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

// wkGridCells builds n entries whose labels are descLen columns wide, which is
// all whichKeyGridFor measures.
func wkGridCells(n, descLen int) []whichKeyCell {
	cells := make([]whichKeyCell, n)
	for i := range cells {
		cells[i] = whichKeyCell{key: "k", desc: strings.Repeat("d", descLen)}
	}
	return cells
}

// TestWhichKeyGridFor pins the ported which-key.nvim box arithmetic
// (view.lua:340-344). The property that matters is that every column is the
// SAME width and the columns divide the container evenly — the old per-column
// widest sizing plus gap-spreading is what read as ragged.
func TestWhichKeyGridFor(t *testing.T) {
	// max_row_width = 1 (key) + 1 (the single gap) + 20 = 22.
	// box_width = clamp(22, 20, 100) = 22; box_count = floor(100/(22+3)) = 4;
	// box_width = floor(100/4) = 25.
	g := whichKeyGridFor(wkGridCells(15, 20), 100)
	if g.boxN != 4 || g.boxW != 25 {
		t.Fatalf("container 100: got box_count=%d box_width=%d, want 4 and 25", g.boxN, g.boxW)
	}
	if g.rowN != 4 {
		t.Fatalf("15 entries over 4 columns must be 4 rows, got %d", g.rowN)
	}

	// Even division: the columns fill the container with less than one column
	// of slack left, and never overflow it.
	for _, container := range []int{40, 60, 74, 100, 114, 200} {
		for _, descLen := range []int{3, 12, 20, 45} {
			g := whichKeyGridFor(wkGridCells(9, descLen), container)
			if used := g.boxN * g.boxW; used > container || container-used >= g.boxN {
				t.Errorf("container=%d desc=%d: %d columns of %d use %d — not an even division",
					container, descLen, g.boxN, g.boxW, used)
			}
		}
	}

	// layout.width.min floors the column width, so tiny entries still get
	// readable columns rather than a dozen sliver ones.
	if g := whichKeyGridFor(wkGridCells(15, 1), 100); g.boxN != 4 {
		t.Fatalf("min column width must cap the column count at 4, got %d (box_width=%d)", g.boxN, g.boxW)
	}

	// A container narrower than one column collapses to a single column
	// instead of producing a zero-width or negative layout.
	g = whichKeyGridFor(wkGridCells(15, 20), 14)
	if g.boxN != 1 || g.boxW != 14 {
		t.Fatalf("narrow container: got box_count=%d box_width=%d, want 1 and 14", g.boxN, g.boxW)
	}
	if g.descW[0] < 0 || g.keyW[0] < 1 {
		t.Fatalf("narrow container produced an unusable cell: keyW=%d descW=%d", g.keyW[0], g.descW[0])
	}
}

// TestWhichKeyGridFor_KeyFieldIsPerColumn is the whole point of Change 2: the
// key field is sized from the widest key IN THAT COLUMN, exactly like
// which-key.nvim accumulates a width per table column (layout.lua:74). Sizing
// it from the widest key in the panel instead indents every single-character
// key by the width of the one "ctrl+alt+y" that lives four columns over.
func TestWhichKeyGridFor_KeyFieldIsPerColumn(t *testing.T) {
	// container 60 -> box_width 30, box_count 2, so 4 entries fill 2 rows x 2
	// columns column-major: d/e in column 0, ctrl+alt+y/x in column 1.
	cells := []whichKeyCell{
		{"d", "Delete"},
		{"e", "Edit"},
		{"ctrl+alt+y", "Mouse capture"},
		{"x", "Exec"},
	}
	g := whichKeyGridFor(cells, 60)
	if g.boxN != 2 || g.rowN != 2 {
		t.Fatalf("precondition: want a 2x2 grid, got box_count=%d rows=%d", g.boxN, g.rowN)
	}
	if g.keyW[0] != 1 {
		t.Errorf("column 0 holds only 1-char keys; key field = %d, want 1", g.keyW[0])
	}
	if want := len("ctrl+alt+y"); g.keyW[1] != want {
		t.Errorf("column 1 key field = %d, want %d (its own widest key)", g.keyW[1], want)
	}
	if g.descW[0] <= g.descW[1] {
		t.Errorf("a narrower key field must leave a wider label field: descW=%v", g.descW)
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
