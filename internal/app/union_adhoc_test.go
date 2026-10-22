package app

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func adHocPickerModel(t *testing.T, names ...string) Model {
	t.Helper()
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	m.nav.Context = ""
	items := make([]model.Item, 1, 1+len(names))
	items[0] = model.Item{Name: "set-1", Kind: unionSetItemKind, Extra: "set-1", Category: unionSetCategory}
	for _, n := range names {
		m.client.AddTestContext(n, "https://"+n+".example.local:6443")
		items = append(items, model.Item{Name: n, IsContext: true, Category: contextCategory})
	}
	m.middleItems = items
	m.leftItems = items
	m.setCursor(1)
	return m
}

func pressSpace(m Model) Model {
	r, _ := m.Update(keyMsg(" "))
	return r.(Model)
}

func TestAdHoc_SpaceMarksContextAndAdvances(t *testing.T) {
	m := adHocPickerModel(t, "a", "b", "c")
	m = pressSpace(m)
	assert.Equal(t, []string{"a"}, m.markedContexts())
	assert.Equal(t, 2, m.cursor())
	m.setCursor(1)
	m = pressSpace(m)
	assert.Empty(t, m.markedContexts())
}

func TestAdHoc_SpaceRejectsUnionSetRow(t *testing.T) {
	m := adHocPickerModel(t, "a")
	m.setCursor(0)
	m = pressSpace(m)
	assert.Empty(t, m.selectedItems)
	assert.True(t, m.statusMessageErr)
}

func TestAdHoc_NinthMarkRefused(t *testing.T) {
	names := make([]string, 9)
	for i := range names {
		names[i] = fmt.Sprintf("c%d", i)
	}
	m := adHocPickerModel(t, names...)
	for range 9 {
		m = pressSpace(m)
	}
	assert.Len(t, m.markedContexts(), MaxUnionContexts)
	assert.True(t, m.statusMessageErr)
}

func TestAdHoc_MarkedContextsKeepPickerOrderUnderFilter(t *testing.T) {
	m := adHocPickerModel(t, "a", "b", "c")
	m.setCursor(3)
	m = pressSpace(m)
	m.setCursor(1)
	m = pressSpace(m)
	m.filterText = "zzz"
	assert.Equal(t, []string{"a", "c"}, m.markedContexts())
}

func TestAdHoc_EnterWithZeroOrOneMarkIsUnchanged(t *testing.T) {
	m := adHocPickerModel(t, "a", "b")
	r, _ := m.navigateChild()
	assert.NotEqual(t, overlayNamespace, r.(Model).overlay)
	assert.Empty(t, r.(Model).pendingUnionContexts)

	m = adHocPickerModel(t, "a", "b")
	m = pressSpace(m)
	m.setCursor(2)
	r, _ = m.navigateChild()
	rm := r.(Model)
	assert.Empty(t, rm.pendingUnionContexts)
	assert.Equal(t, "b", rm.nav.Context)
	assert.Empty(t, rm.selectedItems, "single-mark Enter clears marks as before")
}

func markTwo(t *testing.T) Model {
	m := adHocPickerModel(t, "a", "b", "c")
	m.setCursor(1)
	m = pressSpace(m)
	m.setCursor(3)
	m = pressSpace(m)
	m.setCursor(1)
	r, _ := m.navigateChild()
	return r.(Model)
}

func TestAdHoc_EnterWithTwoMarksOpensNamespaceOverlay(t *testing.T) {
	rm := markTwo(t)
	assert.Equal(t, overlayNamespace, rm.overlay)
	assert.Equal(t, []string{"a", "c"}, rm.pendingUnionContexts)
	assert.Equal(t, adHocUnionPending, rm.pendingUnionSetName)
	assert.False(t, rm.unionMode)
	assert.Len(t, rm.selectedItems, 2)
}

func TestAdHoc_NamespaceEnterActivatesReadOnlyUnion(t *testing.T) {
	rm := markTwo(t)
	rm.overlayItems = []model.Item{{Name: "cloud-cd"}, {Name: "kube-system"}}
	rm.overlayCursor = 0
	r, cmd := rm.handleNamespaceOverlayKey(keyMsg("enter"))
	am := r.(Model)
	require.NotNil(t, cmd)
	assert.True(t, am.unionMode)
	assert.Equal(t, []string{"a", "c"}, am.unionContexts)
	assert.Equal(t, UnionContextSentinel, am.nav.Context)
	assert.Equal(t, "cloud-cd", am.namespace)
	assert.True(t, am.readOnly)
	assert.True(t, am.tabs[am.activeTab].readOnly)
	assert.Empty(t, am.unionSetName)
	assert.True(t, am.unionStartedFromPicker)
	assert.Empty(t, am.pendingUnionSetName)
	assert.Empty(t, am.pendingUnionContexts)
	assert.Empty(t, am.selectedItems)
	assert.Equal(t, overlayNone, am.overlay)
}

func TestAdHoc_NamespaceOverlayRejectsAll(t *testing.T) {
	rm := markTwo(t)
	rm.overlayItems = []model.Item{{Name: "All Namespaces", Status: "all"}, {Name: "x"}}
	rm.overlayCursor = 0
	r, _ := rm.handleNamespaceOverlayKey(keyMsg("enter"))
	assert.False(t, r.(Model).unionMode)
	r, _ = rm.handleNamespaceOverlayKey(keyMsg(" "))
	assert.False(t, r.(Model).unionMode)
	assert.Empty(t, r.(Model).selectedNamespaces)
}

func TestAdHoc_EscCancelsAndKeepsMarks(t *testing.T) {
	rm := markTwo(t)
	r, _ := rm.handleNamespaceOverlayKey(keyMsg("esc"))
	cm := r.(Model)
	assert.Equal(t, overlayNone, cm.overlay)
	assert.Empty(t, cm.pendingUnionSetName)
	assert.Empty(t, cm.pendingUnionContexts)
	assert.Len(t, cm.selectedItems, 2)
}

func activeAdHoc(t *testing.T) Model {
	rm := markTwo(t)
	rm.overlayItems = []model.Item{{Name: "cloud-cd"}}
	rm.overlayCursor = 0
	r, _ := rm.handleNamespaceOverlayKey(keyMsg("enter"))
	return r.(Model)
}

func TestAdHoc_CtrlRUnlocksAndSurvivesDrillAndBack(t *testing.T) {
	am := activeAdHoc(t)
	r, _ := am.handleKeyReadOnlyToggle()
	um := r.(Model)
	assert.False(t, um.readOnly)
	um.applyReadOnlyForContext(UnionContextSentinel)
	assert.False(t, um.readOnly, "unlock must survive back-nav")
	r, _ = um.handleKeyReadOnlyToggle()
	assert.True(t, r.(Model).readOnly)
}

func TestAdHoc_LockSurvivesDrillAndBackWhenNeverToggled(t *testing.T) {
	am := activeAdHoc(t)
	am.applyReadOnlyForContext(UnionContextSentinel)
	assert.True(t, am.readOnly)
}

func TestAdHoc_CLIReadOnlyRefusesUnlock(t *testing.T) {
	am := activeAdHoc(t)
	am.cliReadOnly = true
	r, _ := am.handleKeyReadOnlyToggle()
	assert.True(t, r.(Model).readOnly)
}

func TestAdHoc_NamedSetStartsReadWriteAfterAdHoc(t *testing.T) {
	orig := ui.ConfigUnionSets
	t.Cleanup(func() { ui.ConfigUnionSets = orig })
	set := ui.UnionSetConfig{Name: "s", Namespace: "ns", Contexts: []ui.UnionSetContextConfig{{Context: "a"}, {Context: "b"}}}
	ui.ConfigUnionSets = []ui.UnionSetConfig{set}
	am := activeAdHoc(t)
	r, _ := am.activateUnionSet(set, "ns")
	nm := r.(Model)
	assert.False(t, nm.readOnly)
	nm.applyReadOnlyForContext(UnionContextSentinel)
	assert.False(t, nm.readOnly)
}

func TestAdHoc_BackReturnsToPicker(t *testing.T) {
	am := activeAdHoc(t)
	assert.True(t, am.tabs[am.activeTab].readOnly)
	back, _ := am.navigateParent()
	bm := back.(Model)
	assert.False(t, bm.unionMode)
	assert.Equal(t, model.LevelClusters, bm.nav.Level)
}

func TestAdHoc_UIMarkerOverlayTitleAndHints(t *testing.T) {
	m := adHocPickerModel(t, "alpha", "beta")
	m = pressSpace(m)
	assert.Contains(t, stripANSI(m.View().Content), "1 selected")

	m.setCursor(2)
	m = pressSpace(m)
	m.setCursor(1)
	r, _ := m.navigateChild()
	rm := r.(Model)
	rm.overlayItems = []model.Item{{Name: "cloud-cd"}}
	assert.Contains(t, stripANSI(rm.View().Content), "Union (read-only): alpha, beta")
	hint := stripANSI(rm.overlayHintBarSelector())
	assert.NotContains(t, hint, "exclude")
}
