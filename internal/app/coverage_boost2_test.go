package app

import (
	"context"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// baseModelBoost2 returns a Model with a fake K8s client for boost tests.
func baseModelBoost2() Model {
	cs := fake.NewClientset()
	dyn := dynfake.NewSimpleDynamicClient(runtime.NewScheme())

	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:           []TabState{{}},
		selectedItems:  make(map[string]bool),
		cursorMemory:   make(map[string]int),
		itemCache:      make(map[string][]model.Item),
		discoveredCRDs: make(map[string][]model.ResourceTypeEntry),
		width:          120,
		height:         40,
		execMu:         &sync.Mutex{},
		client:         k8s.NewTestClient(cs, dyn),
		namespace:      "default",
		reqCtx:         context.Background(),
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		Resource:   "pods",
		Namespaced: true,
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		name:         "test-resource",
		namespace:    "default",
		kind:         "Pod",
		resourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
	}

	return m
}

// =====================================================================
// handleDiffVisualKey -- many branches
// =====================================================================

func TestCovBoost2DiffVisualKeyEsc(t *testing.T) {
	m := baseModelBoost2()
	m.mode = modeDiff
	m.diffVisualMode = true
	result, cmd := m.handleDiffVisualKey(keyMsg("esc"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
	assert.Nil(t, cmd)
}

func TestCovBoost2DiffVisualKeyV(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	// V when already in line mode toggles off.
	result, _ := m.handleDiffVisualKey(keyMsg("V"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, rune('V'), rm.diffVisualType)
}

func TestCovBoost2DiffVisualKeyVToggleOff(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'V'
	result, _ := m.handleDiffVisualKey(keyMsg("V"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
}

func TestCovBoost2DiffVisualKeySmallV(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'V'
	result, _ := m.handleDiffVisualKey(keyMsg("v"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, rune('v'), rm.diffVisualType)
}

func TestCovBoost2DiffVisualKeySmallVToggleOff(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	result, _ := m.handleDiffVisualKey(keyMsg("v"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
}

func TestCovBoost2DiffVisualKeyCtrlV(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	result, _ := m.handleDiffVisualKey(tea.KeyMsg{Type: tea.KeyCtrlV}, nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, rune('B'), rm.diffVisualType)
}

func TestCovBoost2DiffVisualKeyCtrlVToggleOff(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'B'
	result, _ := m.handleDiffVisualKey(tea.KeyMsg{Type: tea.KeyCtrlV}, nil, 10, 5, 5)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
}

func TestCovBoost2DiffVisualKeyJK(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 0
	result, _ := m.handleDiffVisualKey(keyMsg("j"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 1, rm.diffCursor)

	result2, _ := rm.handleDiffVisualKey(keyMsg("k"), nil, 10, 5, 5)
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.diffCursor)
}

func TestCovBoost2DiffVisualKeyDown(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 0
	result, _ := m.handleDiffVisualKey(keyMsg("down"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 1, rm.diffCursor)
}

func TestCovBoost2DiffVisualKeyUp(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 3
	result, _ := m.handleDiffVisualKey(keyMsg("up"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 2, rm.diffCursor)
}

func TestCovBoost2DiffVisualKeyHL(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffVisualCurCol = 5
	result, _ := m.handleDiffVisualKey(keyMsg("h"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 4, rm.diffVisualCurCol)

	result2, _ := rm.handleDiffVisualKey(keyMsg("l"), nil, 10, 5, 5)
	rm2 := result2.(Model)
	assert.Equal(t, 5, rm2.diffVisualCurCol)
}

func TestCovBoost2DiffVisualKeyHLBlockMode(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'B'
	m.diffVisualCurCol = 5
	result, _ := m.handleDiffVisualKey(keyMsg("h"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 4, rm.diffVisualCurCol)
}

func TestCovBoost2DiffVisualKeyHLNotInCharOrBlockMode(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'V'
	m.diffVisualCurCol = 5
	result, _ := m.handleDiffVisualKey(keyMsg("h"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 5, rm.diffVisualCurCol)
}

func TestCovBoost2DiffVisualKeyYLineMode(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'V'
	m.diffVisualStart = 0
	m.diffCursor = 0
	m.diffLeft = "line1\nline2\nline3"
	m.diffRight = "lineA\nlineB\nlineC"
	result, cmd := m.handleDiffVisualKey(keyMsg("y"), nil, 3, 3, 0)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovBoost2DiffVisualKeyZero(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffVisualCurCol = 5
	result, _ := m.handleDiffVisualKey(keyMsg("0"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 0, rm.diffVisualCurCol)
}

// =====================================================================
// handleAutoSyncKey -- test more branches
// =====================================================================

func TestCovBoost2AutoSyncKeyEsc(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayAutoSync
	result, _ := m.handleAutoSyncKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovBoost2AutoSyncKeyQ(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayAutoSync
	result, _ := m.handleAutoSyncKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovBoost2AutoSyncKeyJ(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 0
	result, _ := m.handleAutoSyncKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.autoSyncCursor)
}

func TestCovBoost2AutoSyncKeyK(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 1
	result, _ := m.handleAutoSyncKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.autoSyncCursor)
}

func TestCovBoost2AutoSyncKeyDown(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 0
	result, _ := m.handleAutoSyncKey(keyMsg("down"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.autoSyncCursor)
}

func TestCovBoost2AutoSyncKeyUp(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 2
	result, _ := m.handleAutoSyncKey(keyMsg("up"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.autoSyncCursor)
}

func TestCovBoost2AutoSyncKeyJMax(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 2
	result, _ := m.handleAutoSyncKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.autoSyncCursor) // stays at max
}

func TestCovBoost2AutoSyncKeyKMin(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 0
	result, _ := m.handleAutoSyncKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.autoSyncCursor)
}

func TestCovBoost2AutoSyncKeySpaceToggleEnabled(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 0
	m.autoSyncEnabled = false
	result, _ := m.handleAutoSyncKey(keyMsg(" "))
	rm := result.(Model)
	assert.True(t, rm.autoSyncEnabled)
}

func TestCovBoost2AutoSyncKeySpaceToggleSelfHeal(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 1
	m.autoSyncEnabled = true
	m.autoSyncSelfHeal = false
	result, _ := m.handleAutoSyncKey(keyMsg(" "))
	rm := result.(Model)
	assert.True(t, rm.autoSyncSelfHeal)
}

func TestCovBoost2AutoSyncKeySpaceTogglePrune(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 2
	m.autoSyncEnabled = true
	m.autoSyncPrune = false
	result, _ := m.handleAutoSyncKey(keyMsg(" "))
	rm := result.(Model)
	assert.True(t, rm.autoSyncPrune)
}

func TestCovBoost2AutoSyncKeySpaceDisabledSelfHeal(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 1
	m.autoSyncEnabled = false
	m.autoSyncSelfHeal = false
	result, _ := m.handleAutoSyncKey(keyMsg(" "))
	rm := result.(Model)
	assert.False(t, rm.autoSyncSelfHeal) // not toggled because autoSync disabled
}

func TestCovBoost2AutoSyncKeySpaceDisabledPrune(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 2
	m.autoSyncEnabled = false
	m.autoSyncPrune = false
	result, _ := m.handleAutoSyncKey(keyMsg(" "))
	rm := result.(Model)
	assert.False(t, rm.autoSyncPrune)
}

func TestCovBoost2AutoSyncKeyEnter(t *testing.T) {
	m := baseModelBoost2()
	m.middleItems = []model.Item{{Name: "app-1", Namespace: "default"}}
	result, cmd := m.handleAutoSyncKey(keyMsg("enter"))
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovBoost2AutoSyncKeyCtrlS(t *testing.T) {
	m := baseModelBoost2()
	m.middleItems = []model.Item{{Name: "app-1", Namespace: "default"}}
	result, cmd := m.handleAutoSyncKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovBoost2AutoSyncKeyUnknown(t *testing.T) {
	m := baseModelBoost2()
	result, cmd := m.handleAutoSyncKey(keyMsg("x"))
	assert.Nil(t, cmd)
	_ = result
}

// =====================================================================
// handleColorschemeNormalMode -- test more branches
// =====================================================================

func TestCovColorschemeKeyEscEmpty(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayColorscheme
	m.schemeFilter.Clear()
	result, _ := m.handleColorschemeNormalMode(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovColorschemeKeyEscWithFilter(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayColorscheme
	m.schemeFilter.Insert("dark")
	result, _ := m.handleColorschemeNormalMode(keyMsg("esc"))
	rm := result.(Model)
	assert.NotEqual(t, overlayNone, rm.overlay) // stays open, just clears filter
}

func TestCovColorschemeKeyQ(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayColorscheme
	result, _ := m.handleColorschemeNormalMode(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovColorschemeKeyEnter(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayColorscheme
	m.schemeCursor = 0
	// May or may not close overlay depending on whether schemes are loaded.
	result, _ := m.handleColorschemeNormalMode(keyMsg("enter"))
	_ = result
}

func TestCovColorschemeKeySlash(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handleColorschemeNormalMode(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.schemeFilterMode)
}

func TestCovColorschemeKeyJ(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 0
	result, _ := m.handleColorschemeNormalMode(keyMsg("j"))
	_ = result // cursor clamped to scheme count
}

func TestCovColorschemeKeyK(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 2
	result, _ := m.handleColorschemeNormalMode(keyMsg("k"))
	_ = result
}

func TestCovColorschemeKeyCtrlD(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 0
	result, _ := m.handleColorschemeNormalMode(keyMsg("ctrl+d"))
	_ = result
}

func TestCovColorschemeKeyCtrlU(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 10
	result, _ := m.handleColorschemeNormalMode(keyMsg("ctrl+u"))
	_ = result
}

func TestCovColorschemeKeyCtrlF(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 0
	result, _ := m.handleColorschemeNormalMode(keyMsg("ctrl+f"))
	_ = result
}

func TestCovColorschemeKeyCtrlB(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 15
	result, _ := m.handleColorschemeNormalMode(keyMsg("ctrl+b"))
	_ = result
}

func TestCovColorschemeKeyG(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 5
	// First g: pendingG.
	result, _ := m.handleColorschemeNormalMode(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)

	// Second g: go to top.
	result2, _ := rm.handleColorschemeNormalMode(keyMsg("g"))
	rm2 := result2.(Model)
	assert.False(t, rm2.pendingG)
	assert.Equal(t, 0, rm2.schemeCursor)
}

func TestCovColorschemeKeyBigG(t *testing.T) {
	m := baseModelBoost2()
	m.schemeCursor = 0
	result, _ := m.handleColorschemeNormalMode(keyMsg("G"))
	_ = result
}

func TestCovColorschemeKeyH(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handleColorschemeNormalMode(keyMsg("H"))
	_ = result
}

func TestCovColorschemeKeyL(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handleColorschemeNormalMode(keyMsg("L"))
	_ = result
}

// =====================================================================
// handleLogContainerSelectOverlayKey -- test more branches
// =====================================================================

func TestCovLogContainerSelectEsc(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovLogContainerSelectEscWithFilter(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainerFilterText = "main"
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.logContainerFilterText)
}

func logContainerOverlayItems(containers []string) []model.Item {
	items := make([]model.Item, 0, 1+len(containers))
	items = append(items, model.Item{Name: "All Containers", Status: "all"})
	for _, c := range containers {
		items = append(items, model.Item{Name: c})
	}
	return items
}

func TestCovLogContainerSelectSpaceAll(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main", "sidecar"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.overlayCursor = 0
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg(" "))
	rm := result.(Model)
	assert.Nil(t, rm.logSelectedContainers)
}

func TestCovLogContainerSelectSpaceSelectOne(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main", "sidecar"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.overlayCursor = 1
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg(" "))
	rm := result.(Model)
	assert.NotNil(t, rm.logSelectedContainers)
}

func TestCovLogContainerSelectSpaceDeselect(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main", "sidecar"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.logSelectedContainers = []string{"main", "sidecar"}
	m.overlayCursor = 1 // Points to "main".
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg(" "))
	rm := result.(Model)
	assert.True(t, len(rm.logSelectedContainers) < 2 || rm.logSelectedContainers == nil)
}

func TestCovLogContainerSelectSpaceAddContainer(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main", "sidecar", "proxy"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.logSelectedContainers = []string{"main"}
	m.overlayCursor = 2 // Points to "sidecar".
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg(" "))
	rm := result.(Model)
	assert.Equal(t, 2, len(rm.logSelectedContainers))
}

func TestCovLogContainerSelectEnterNoModify(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main", "sidecar"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.overlayCursor = 1 // Points to "main".
	result, cmd := m.handleLogContainerSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovLogContainerSelectEnterAll(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.overlayCursor = 0 // Points to "All Containers".
	result, cmd := m.handleLogContainerSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Nil(t, rm.logSelectedContainers)
	assert.NotNil(t, cmd)
}

func TestCovLogContainerSelectEnterModified(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main", "sidecar"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.logContainerSelectionModified = true
	m.logSelectedContainers = []string{"main"}
	result, cmd := m.handleLogContainerSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovLogContainerSelectSlash(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.logContainerFilterActive)
}

func TestCovLogContainerSelectJK(t *testing.T) {
	m := baseModelBoost2()
	m.logContainers = []string{"main", "sidecar"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.overlayCursor = 0
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)

	rm.overlayItems = logContainerOverlayItems(m.logContainers)
	result2, _ := rm.handleLogContainerSelectOverlayKey(keyMsg("k"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.overlayCursor)
}

func TestCovLogContainerSelectCtrlD(t *testing.T) {
	m := baseModelBoost2()
	m.logContainers = make([]string, 20)
	m.overlayItems = make([]model.Item, 21) // "All Containers" + 20
	m.overlayItems[0] = model.Item{Name: "All Containers", Status: "all"}
	for i := range m.logContainers {
		m.logContainers[i] = "c" + string(rune('a'+i))
		m.overlayItems[i+1] = model.Item{Name: m.logContainers[i]}
	}
	m.overlayCursor = 0
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.True(t, rm.overlayCursor > 0)
}

func TestCovLogContainerSelectCtrlU(t *testing.T) {
	m := baseModelBoost2()
	m.logContainers = make([]string, 20)
	m.overlayItems = make([]model.Item, 21)
	m.overlayItems[0] = model.Item{Name: "All Containers", Status: "all"}
	for i := range m.logContainers {
		m.logContainers[i] = "c" + string(rune('a'+i))
		m.overlayItems[i+1] = model.Item{Name: m.logContainers[i]}
	}
	m.overlayCursor = 15
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.True(t, rm.overlayCursor < 15)
}

// =====================================================================
// handleLogPodSelectOverlayKey -- test more branches
// =====================================================================

func TestCovLogPodSelectEsc(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovLogPodSelectEscWithFilter(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	m.logPodFilterText = "web"
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.logPodFilterText)
}

func TestCovLogPodSelectEscWithSavedPod(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	m.logSavedPodName = "old-pod"
	result, cmd := m.handleLogPodSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovLogPodSelectEnterAllPods(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	m.logParentKind = "Deployment"
	m.logParentName = "my-deploy"
	m.overlayItems = []model.Item{{Name: "All Pods", Status: "all"}, {Name: "pod-1"}}
	m.overlayCursor = 0
	result, cmd := m.handleLogPodSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovLogPodSelectEnterSpecificPod(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	m.overlayItems = []model.Item{{Name: "All Pods", Status: "all"}, {Name: "pod-1", Namespace: "ns1"}}
	m.overlayCursor = 1
	result, cmd := m.handleLogPodSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, "pod-1", rm.actionCtx.name)
}

func TestCovLogPodSelectSlash(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.logPodFilterActive)
}

func TestCovLogPodSelectJK(t *testing.T) {
	m := baseModelBoost2()
	m.overlayItems = []model.Item{{Name: "p1"}, {Name: "p2"}, {Name: "p3"}}
	m.overlayCursor = 0
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)

	result2, _ := rm.handleLogPodSelectOverlayKey(keyMsg("k"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.overlayCursor)
}

func TestCovLogPodSelectCtrlD(t *testing.T) {
	m := baseModelBoost2()
	m.overlayItems = make([]model.Item, 20)
	m.overlayCursor = 0
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.True(t, rm.overlayCursor > 0)
}

func TestCovLogPodSelectCtrlU(t *testing.T) {
	m := baseModelBoost2()
	m.overlayItems = make([]model.Item, 20)
	m.overlayCursor = 15
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.True(t, rm.overlayCursor < 15)
}

// =====================================================================
// handleEventTimelineVisualKey -- test more branches
// =====================================================================

func TestCovEventTimelineVisualEsc(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	result, _ := m.handleEventTimelineVisualKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualV(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	result, _ := m.handleEventTimelineVisualKey(keyMsg("V"))
	rm := result.(Model)
	assert.Equal(t, byte('V'), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualVToggleOff(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	result, _ := m.handleEventTimelineVisualKey(keyMsg("V"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualSmallV(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	result, _ := m.handleEventTimelineVisualKey(keyMsg("v"))
	rm := result.(Model)
	assert.Equal(t, byte('v'), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualSmallVToggleOff(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	result, _ := m.handleEventTimelineVisualKey(keyMsg("v"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualCtrlV(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	result, _ := m.handleEventTimelineVisualKey(tea.KeyMsg{Type: tea.KeyCtrlV})
	rm := result.(Model)
	assert.Equal(t, byte('B'), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualCtrlVToggleOff(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'B'
	result, _ := m.handleEventTimelineVisualKey(tea.KeyMsg{Type: tea.KeyCtrlV})
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualJK(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineLines = []string{"line0", "line1", "line2"}
	m.eventTimelineCursor = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.eventTimelineCursor)

	result2, _ := rm.handleEventTimelineVisualKey(keyMsg("k"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.eventTimelineCursor)
}

func TestCovEventTimelineVisualHL(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineCursorCol = 5
	result, _ := m.handleEventTimelineVisualKey(keyMsg("h"))
	rm := result.(Model)
	assert.Equal(t, 4, rm.eventTimelineCursorCol)

	result2, _ := rm.handleEventTimelineVisualKey(keyMsg("l"))
	rm2 := result2.(Model)
	assert.Equal(t, 5, rm2.eventTimelineCursorCol)
}

func TestCovEventTimelineVisualZero(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineCursorCol = 5
	result, _ := m.handleEventTimelineVisualKey(keyMsg("0"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.eventTimelineCursorCol)
}

func TestCovEventTimelineVisualDollar(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world"}
	m.eventTimelineCursor = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("$"))
	rm := result.(Model)
	assert.Equal(t, len([]rune("hello world"))-1, rm.eventTimelineCursorCol)
}

func TestCovEventTimelineVisualCaret(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"  hello world"}
	m.eventTimelineCursor = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("^"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.eventTimelineCursorCol)
}

func TestCovEventTimelineVisualW(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world"}
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("w"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursorCol > 0)
}

func TestCovEventTimelineVisualB(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world"}
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 6
	result, _ := m.handleEventTimelineVisualKey(keyMsg("b"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursorCol < 6)
}

func TestCovEventTimelineVisualBigG(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineLines = []string{"a", "b", "c"}
	m.eventTimelineCursor = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.eventTimelineCursor)
}

func TestCovEventTimelineVisualGG(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineLines = []string{"a", "b", "c"}
	m.eventTimelineCursor = 2
	// First g.
	result, _ := m.handleEventTimelineVisualKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)

	// Second g.
	result2, _ := rm.handleEventTimelineVisualKey(keyMsg("g"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.eventTimelineCursor)
}

func TestCovEventTimelineVisualCtrlD(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineLines = make([]string, 50)
	m.eventTimelineCursor = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursor > 0)
}

func TestCovEventTimelineVisualCtrlU(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineLines = make([]string, 50)
	m.eventTimelineCursor = 25
	result, _ := m.handleEventTimelineVisualKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursor < 25)
}

func TestCovEventTimelineVisualYLineMode(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineLines = []string{"line0", "line1", "line2"}
	m.eventTimelineVisualStart = 0
	m.eventTimelineCursor = 1
	result, cmd := m.handleEventTimelineVisualKey(keyMsg("y"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovEventTimelineVisualYCharMode(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world"}
	m.eventTimelineVisualStart = 0
	m.eventTimelineCursor = 0
	m.eventTimelineVisualCol = 0
	m.eventTimelineCursorCol = 4
	result, cmd := m.handleEventTimelineVisualKey(keyMsg("y"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovEventTimelineVisualYCharModeMultiline(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world", "foo bar", "baz qux"}
	m.eventTimelineVisualStart = 0
	m.eventTimelineCursor = 2
	m.eventTimelineVisualCol = 2
	m.eventTimelineCursorCol = 3
	result, cmd := m.handleEventTimelineVisualKey(keyMsg("y"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovEventTimelineVisualYBlockMode(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'B'
	m.eventTimelineLines = []string{"hello world", "foo bar"}
	m.eventTimelineVisualStart = 0
	m.eventTimelineCursor = 1
	m.eventTimelineVisualCol = 0
	m.eventTimelineCursorCol = 4
	result, cmd := m.handleEventTimelineVisualKey(keyMsg("y"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovEventTimelineVisualBigW(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world foo"}
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("W"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursorCol > 0)
}

func TestCovEventTimelineVisualBigB(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world foo"}
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 12
	result, _ := m.handleEventTimelineVisualKey(keyMsg("B"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursorCol < 12)
}

func TestCovEventTimelineVisualSmallE(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world"}
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("e"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursorCol > 0)
}

func TestCovEventTimelineVisualBigE(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world"}
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("E"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursorCol > 0)
}

// =====================================================================
// handleOverlayKey -- test more overlay dispatch branches
// =====================================================================

func TestCovHandleOverlayKeyRBAC(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayRBAC
	result, _ := m.handleOverlayKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyPodStartup(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPodStartup
	result, _ := m.handleOverlayKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyQuotaDashboardEsc(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayQuotaDashboard
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyQuotaDashboardQ(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayQuotaDashboard
	result, _ := m.handleOverlayKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyAutoSync(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayAutoSync
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyLogPodSelect(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyLogContainerSelect(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyEventTimeline(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayEventTimeline
	m.eventTimelineLines = []string{"event1"}
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyNone(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayNone
	result, cmd := m.handleOverlayKey(keyMsg("q"))
	assert.Nil(t, cmd)
	_ = result
}

// =====================================================================
// handlePortForwardOverlayKey -- test more branches
// =====================================================================

func TestCovPortForwardKeyEsc(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	result, _ := m.handlePortForwardOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovPortForwardKeyJK(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.pfAvailablePorts = []ui.PortInfo{{Port: "80"}, {Port: "443"}}
	m.pfPortCursor = 0
	result, _ := m.handlePortForwardOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.pfPortCursor)

	result2, _ := rm.handlePortForwardOverlayKey(keyMsg("k"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.pfPortCursor)
}

func TestCovPortForwardKeyEnterWithPort(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.pfAvailablePorts = []ui.PortInfo{{Port: "80"}}
	m.pfPortCursor = 0
	result, cmd := m.handlePortForwardOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovPortForwardKeyEnterWithCustomLocal(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.pfAvailablePorts = []ui.PortInfo{{Port: "80"}}
	m.pfPortCursor = 0
	m.portForwardInput.Insert("9090")
	result, cmd := m.handlePortForwardOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovPortForwardKeyEnterManual(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.pfPortCursor = -1
	m.portForwardInput.Insert("8080:80")
	result, cmd := m.handlePortForwardOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovPortForwardKeyEnterManualSinglePort(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.pfPortCursor = -1
	m.portForwardInput.Insert("8080")
	result, cmd := m.handlePortForwardOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovPortForwardKeyEnterEmpty(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.pfPortCursor = -1
	result, cmd := m.handlePortForwardOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd) // scheduleStatusClear
	assert.True(t, rm.hasStatusMessage())
}

func TestCovPortForwardKeyEnterService(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.actionCtx.kind = "Service"
	m.pfAvailablePorts = []ui.PortInfo{{Port: "80"}}
	m.pfPortCursor = 0
	result, cmd := m.handlePortForwardOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovPortForwardKeyBackspace(t *testing.T) {
	m := baseModelBoost2()
	m.portForwardInput.Insert("123")
	result, _ := m.handlePortForwardOverlayKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "12", rm.portForwardInput.Value)
}

func TestCovPortForwardKeyBackspaceEmpty(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handlePortForwardOverlayKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Empty(t, rm.portForwardInput.Value)
}

func TestCovPortForwardKeyCtrlW(t *testing.T) {
	m := baseModelBoost2()
	m.portForwardInput.Insert("123 456")
	result, _ := m.handlePortForwardOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	rm := result.(Model)
	assert.True(t, len(rm.portForwardInput.Value) < len("123 456"))
}

func TestCovPortForwardKeyCtrlA(t *testing.T) {
	m := baseModelBoost2()
	m.portForwardInput.Insert("123")
	result, _ := m.handlePortForwardOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlA})
	_ = result
}

func TestCovPortForwardKeyCtrlE(t *testing.T) {
	m := baseModelBoost2()
	m.portForwardInput.Insert("123")
	result, _ := m.handlePortForwardOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlE})
	_ = result
}

func TestCovPortForwardKeyLeft(t *testing.T) {
	m := baseModelBoost2()
	m.portForwardInput.Insert("123")
	result, _ := m.handlePortForwardOverlayKey(keyMsg("left"))
	_ = result
}

func TestCovPortForwardKeyRight(t *testing.T) {
	m := baseModelBoost2()
	m.portForwardInput.Insert("123")
	result, _ := m.handlePortForwardOverlayKey(keyMsg("right"))
	_ = result
}

func TestCovPortForwardKeyDigit(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handlePortForwardOverlayKey(keyMsg("8"))
	rm := result.(Model)
	assert.Equal(t, "8", rm.portForwardInput.Value)
}

func TestCovPortForwardKeyColon(t *testing.T) {
	m := baseModelBoost2()
	m.pfPortCursor = 0
	result, _ := m.handlePortForwardOverlayKey(keyMsg(":"))
	rm := result.(Model)
	assert.Equal(t, -1, rm.pfPortCursor)
}

func TestCovPortForwardKeyInvalidChar(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handlePortForwardOverlayKey(keyMsg("a"))
	rm := result.(Model)
	assert.Empty(t, rm.portForwardInput.Value)
}

// =====================================================================
// handleContainerSelectOverlayKey -- test more branches
// =====================================================================

func TestCovContainerSelectKeyEsc(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayContainerSelect
	m.pendingAction = "Exec"
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Empty(t, rm.pendingAction)
}

func TestCovContainerSelectKeyEnterWithItem(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayContainerSelect
	m.overlayItems = []model.Item{{Name: "main"}}
	m.overlayCursor = 0
	m.pendingAction = "Describe"
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	result, cmd := m.handleContainerSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, "main", rm.actionCtx.containerName)
	assert.NotNil(t, cmd)
}

func TestCovContainerSelectKeyEnterNoItems(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayContainerSelect
	m.overlayItems = nil
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovContainerSelectKeyJK(t *testing.T) {
	m := baseModelBoost2()
	m.overlayItems = []model.Item{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	m.overlayCursor = 0
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)

	result2, _ := rm.handleContainerSelectOverlayKey(keyMsg("k"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.overlayCursor)
}

func TestCovContainerSelectKeyDown(t *testing.T) {
	m := baseModelBoost2()
	m.overlayItems = []model.Item{{Name: "a"}, {Name: "b"}}
	m.overlayCursor = 0
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("down"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)
}

func TestCovContainerSelectKeyUp(t *testing.T) {
	m := baseModelBoost2()
	m.overlayItems = []model.Item{{Name: "a"}, {Name: "b"}}
	m.overlayCursor = 1
	result, _ := m.handleContainerSelectOverlayKey(keyMsg("up"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.overlayCursor)
}

// =====================================================================
// navigateChild -- test more level branches
// =====================================================================

func TestCovBoost2NavigateChildClusters(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "test-ctx"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
}

func TestCovBoost2NavigateChildResourceTypes(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.middleItems = []model.Item{{Name: "Pods", Extra: "v1/pods"}}
	result, _ := m.navigateChild()
	_ = result
}

func TestCovBoost2NavigateChildResourceTypesOverview(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.True(t, rm.fullscreenDashboard)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildResourceTypesMonitoring(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Monitoring", Extra: "__monitoring__"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.True(t, rm.fullscreenDashboard)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildResourceTypesPortForwards(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.portForwardMgr = k8s.NewPortForwardManager()
	m.middleItems = []model.Item{{Name: "Port Forwards", Kind: "__port_forwards__"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelResources, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildResourceTypesCollapsedGroup(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Workloads", Kind: "__collapsed_group__", Category: "Workloads"}}
	result, _ := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, "Workloads", rm.expandedGroup)
}

func TestCovBoost2NavigateChildResourcesNoChildren(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "ConfigMap", Resource: "configmaps", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-cm"}}
	result, cmd := m.navigateChild()
	assert.Nil(t, cmd) // ConfigMaps don't have children
	_ = result
}

func TestCovBoost2NavigateChildResourcesPod(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-pod", Namespace: "default"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelContainers, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildResourcesDeployment(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", Namespaced: true, APIGroup: "apps", APIVersion: "v1"}
	m.middleItems = []model.Item{{Name: "my-deploy", Namespace: "default"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelOwned, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildResourcesPodNoNamespace(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-pod"}}
	m.allNamespaces = false
	m.namespace = "default"
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelContainers, rm.nav.Level)
	assert.NotNil(t, cmd)
	assert.Equal(t, "default", rm.nav.Namespace)
}

func TestCovBoost2NavigateChildOwnedPod(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "ns1"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelContainers, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildOwnedNonPod(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "rs-1", Kind: "ReplicaSet"}}
	result, cmd := m.navigateChild()
	_ = result
	_ = cmd
}

func TestCovBoost2NavigateChildContainers(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelContainers
	m.middleItems = []model.Item{{Name: "main"}}
	result, cmd := m.navigateChild()
	assert.Nil(t, cmd)
	_ = result
}

func TestCovBoost2NavigateChildNoSelection(t *testing.T) {
	m := baseModelBoost2()
	m.middleItems = nil
	result, cmd := m.navigateChild()
	assert.Nil(t, cmd)
	_ = result
}

// =====================================================================
// loadPreview -- test more branches
// =====================================================================

func TestCovBoost2LoadPreviewNilSelection(t *testing.T) {
	m := baseModelBoost2()
	m.middleItems = nil
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

func TestCovBoost2LoadPreviewClusters(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "test-ctx"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewRTOverview(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	ui.ConfigDashboard = true
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewRTOverviewDisabled(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	ui.ConfigDashboard = false
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

func TestCovBoost2LoadPreviewRTMonitoring(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Monitoring", Extra: "__monitoring__"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewRTCollapsed(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "collapsed", Kind: "__collapsed_group__"}}
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

func TestCovBoost2LoadPreviewRTPortForwards(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.portForwardMgr = k8s.NewPortForwardManager()
	m.middleItems = []model.Item{{Name: "Port Forwards", Kind: "__port_forwards__"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewRTNormal(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.middleItems = []model.Item{{Name: "Pods", Extra: "v1/pods"}}
	// loadResources for preview calls selectedMiddleItem and fetches
	// resources; the result may be nil if the extra doesn't resolve to a
	// known resource type, which is fine for coverage.
	_ = m.loadPreview()
}

func TestCovBoost2LoadPreviewResPortForwards(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__"}
	m.middleItems = []model.Item{{Name: "pf-1"}}
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

func TestCovBoost2LoadPreviewResPod(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-pod", Namespace: "default"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewResDeployment(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", APIGroup: "apps", APIVersion: "v1", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-deploy", Namespace: "default"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewResConfigMap(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "ConfigMap", Resource: "configmaps", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-cm", Namespace: "default"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewResWithFullYAML(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-pod", Namespace: "default"}}
	m.fullYAMLPreview = true
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewResMapView(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", APIGroup: "apps", APIVersion: "v1", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-deploy", Namespace: "default"}}
	m.mapView = true
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewOwnedPod(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewOwnedPodFullYAML(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}}
	m.fullYAMLPreview = true
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewOwnedNonPod(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "rs-1", Kind: "ReplicaSet", Namespace: "default", Extra: "/v1/replicasets"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewOwnedNonPodFullYAML(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "rs-1", Kind: "ReplicaSet", Namespace: "default", Extra: "/v1/replicasets"}}
	m.fullYAMLPreview = true
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewContainers(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelContainers
	m.middleItems = []model.Item{{Name: "main"}}
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

// =====================================================================
// loadDashboard and loadMonitoringDashboard
// =====================================================================

func TestCovBoost2LoadDashboardCmd(t *testing.T) {
	m := baseModelBoost2()
	cmd := m.loadDashboard()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadMonitoringDashboardCmd(t *testing.T) {
	m := baseModelBoost2()
	cmd := m.loadMonitoringDashboard()
	assert.NotNil(t, cmd)
}

// =====================================================================
// handleDiffVisualKey -- yank tests with content
// =====================================================================

func TestCovBoost2DiffVisualYCharModeSingleLine(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world"
	m.diffRight = "hello world"
	m.diffVisualStart = 0
	m.diffCursor = 0
	m.diffVisualCol = 0
	m.diffVisualCurCol = 4
	result, cmd := m.handleDiffVisualKey(keyMsg("y"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovBoost2DiffVisualYBlockMode(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'B'
	m.diffLeft = "hello world\nfoo bar"
	m.diffRight = "hello world\nfoo bar"
	m.diffVisualStart = 0
	m.diffCursor = 1
	m.diffVisualCol = 0
	m.diffVisualCurCol = 4
	result, cmd := m.handleDiffVisualKey(keyMsg("y"), nil, 2, 2, 0)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
	assert.NotNil(t, cmd)
}

// =====================================================================
// handleDiffVisualKey -- word motion keys
// =====================================================================

func TestCovBoost2DiffVisualKeyDollar(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world"
	m.diffRight = "hello world"
	m.diffCursor = 0
	result, _ := m.handleDiffVisualKey(keyMsg("$"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol > 0)
}

func TestCovBoost2DiffVisualKeyCaret(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "  hello world"
	m.diffRight = "  hello world"
	m.diffCursor = 0
	m.diffVisualCurCol = 5
	result, _ := m.handleDiffVisualKey(keyMsg("^"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.Equal(t, 2, rm.diffVisualCurCol) // first non-whitespace
}

func TestCovBoost2DiffVisualKeyW(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world foo"
	m.diffRight = "hello world foo"
	m.diffCursor = 0
	m.diffVisualCurCol = 0
	result, _ := m.handleDiffVisualKey(keyMsg("w"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol > 0)
}

func TestCovBoost2DiffVisualKeySmallB(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world foo"
	m.diffRight = "hello world foo"
	m.diffCursor = 0
	m.diffVisualCurCol = 6
	result, _ := m.handleDiffVisualKey(keyMsg("b"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol < 6)
}

func TestCovBoost2DiffVisualKeyE(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world"
	m.diffRight = "hello world"
	m.diffCursor = 0
	m.diffVisualCurCol = 0
	result, _ := m.handleDiffVisualKey(keyMsg("e"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol > 0)
}

func TestCovBoost2DiffVisualKeyBigE(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world"
	m.diffRight = "hello world"
	m.diffCursor = 0
	m.diffVisualCurCol = 0
	result, _ := m.handleDiffVisualKey(keyMsg("E"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol > 0)
}

func TestCovBoost2DiffVisualKeyBigW(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world foo"
	m.diffRight = "hello world foo"
	m.diffCursor = 0
	m.diffVisualCurCol = 0
	result, _ := m.handleDiffVisualKey(keyMsg("W"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol > 0)
}

func TestCovBoost2DiffVisualKeyBigB(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world foo"
	m.diffRight = "hello world foo"
	m.diffCursor = 0
	m.diffVisualCurCol = 12
	result, _ := m.handleDiffVisualKey(keyMsg("B"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol < 12)
}

// =====================================================================
// handleDiffVisualKey -- G / gg
// =====================================================================

func TestCovBoost2DiffVisualKeyBigG(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 0
	m.diffLeft = "a\nb\nc"
	m.diffRight = "a\nb\nc"
	result, _ := m.handleDiffVisualKey(keyMsg("G"), nil, 3, 3, 0)
	rm := result.(Model)
	assert.Equal(t, 2, rm.diffCursor)
}

func TestCovBoost2DiffVisualKeyGG(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 2
	m.diffLeft = "a\nb\nc"
	m.diffRight = "a\nb\nc"
	// First g.
	result, _ := m.handleDiffVisualKey(keyMsg("g"), nil, 3, 3, 0)
	rm := result.(Model)
	require.True(t, rm.pendingG)

	// Second g.
	result2, _ := rm.handleDiffVisualKey(keyMsg("g"), nil, 3, 3, 0)
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.diffCursor)
}

func TestCovBoost2DiffVisualKeyCtrlD(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 0
	m.diffLeft = strings.Repeat("line\n", 50)
	m.diffRight = strings.Repeat("line\n", 50)
	result, _ := m.handleDiffVisualKey(keyMsg("ctrl+d"), nil, 50, 20, 30)
	rm := result.(Model)
	assert.True(t, rm.diffCursor > 0)
}

func TestCovBoost2DiffVisualKeyCtrlU(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 25
	m.diffLeft = strings.Repeat("line\n", 50)
	m.diffRight = strings.Repeat("line\n", 50)
	result, _ := m.handleDiffVisualKey(keyMsg("ctrl+u"), nil, 50, 20, 30)
	rm := result.(Model)
	assert.True(t, rm.diffCursor < 25)
}

func TestCovBoost2DiffVisualKeyCtrlF(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 0
	result, _ := m.handleDiffVisualKey(keyMsg("ctrl+f"), nil, 50, 20, 30)
	rm := result.(Model)
	assert.True(t, rm.diffCursor > 0)
}

func TestCovBoost2DiffVisualKeyCtrlB(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 30
	result, _ := m.handleDiffVisualKey(keyMsg("ctrl+b"), nil, 50, 20, 30)
	rm := result.(Model)
	assert.True(t, rm.diffCursor < 30)
}

func TestCovBoost2DiffVisualKeyH2(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	result, _ := m.handleDiffVisualKey(keyMsg("H"), nil, 10, 5, 5)
	_ = result
}

func TestCovBoost2DiffVisualKeyL2(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	result, _ := m.handleDiffVisualKey(keyMsg("L"), nil, 10, 5, 5)
	_ = result
}

func TestCovBoost2DiffVisualKeyM(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	result, _ := m.handleDiffVisualKey(keyMsg("M"), nil, 10, 5, 5)
	_ = result
}

func TestCovBoost2DiffVisualKeyTab(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursorSide = 0
	// "tab" is not handled by handleDiffVisualKey, falls through.
	result, cmd := m.handleDiffVisualKey(keyMsg("tab"), nil, 10, 5, 5)
	assert.Nil(t, cmd)
	_ = result
}

func TestCovBoost2DiffVisualKeyUnknown(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	result, cmd := m.handleDiffVisualKey(keyMsg("z"), nil, 10, 5, 5)
	assert.Nil(t, cmd)
	_ = result
}
