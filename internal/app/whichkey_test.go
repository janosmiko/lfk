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
