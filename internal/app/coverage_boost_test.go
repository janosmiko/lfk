package app

import (
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// baseModelCov returns a minimal Model for coverage tests.
func baseModelCov() Model {
	return Model{
		nav:            model.NavigationState{Level: model.LevelResources},
		tabs:           []TabState{{}},
		selectedItems:  make(map[string]bool),
		cursorMemory:   make(map[string]int),
		itemCache:      make(map[string][]model.Item),
		discoveredCRDs: make(map[string][]model.ResourceTypeEntry),
		width:          80,
		height:         40,
		execMu:         &sync.Mutex{},
	}
}

// =====================================================================
// Target 1: commands.go bulk action functions (all 0%)
// =====================================================================

func TestCovBulkDeleteResourcesReturnsCmd(t *testing.T) {
	m := baseModelCov()
	m.bulkItems = []model.Item{
		{Name: "pod-1", Namespace: "default"},
		{Name: "pod-2"},
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		resourceType: model.ResourceTypeEntry{Resource: "pods", Kind: "Pod"},
	}
	m.namespace = "fallback-ns"
	cmd := m.bulkDeleteResources()
	assert.NotNil(t, cmd)
}

func TestCovBulkDeleteResourcesEmpty(t *testing.T) {
	m := baseModelCov()
	m.bulkItems = nil
	m.actionCtx = actionContext{context: "ctx", resourceType: model.ResourceTypeEntry{Resource: "pods"}}
	cmd := m.bulkDeleteResources()
	assert.NotNil(t, cmd)
}

func TestCovBulkScaleResources(t *testing.T) {
	m := baseModelCov()
	m.bulkItems = []model.Item{{Name: "deploy-1", Namespace: "ns1"}, {Name: "deploy-2"}}
	m.actionCtx = actionContext{context: "ctx"}
	m.namespace = "default"
	assert.NotNil(t, m.bulkScaleResources(3))
	assert.NotNil(t, m.bulkScaleResources(0))
}

func TestCovBulkRestartResources(t *testing.T) {
	m := baseModelCov()
	m.bulkItems = []model.Item{{Name: "deploy-1", Namespace: "ns1"}, {Name: "deploy-2"}}
	m.actionCtx = actionContext{context: "ctx"}
	m.namespace = "default"
	assert.NotNil(t, m.bulkRestartResources())
}

func TestCovBulkRestartResourcesNone(t *testing.T) {
	m := baseModelCov()
	m.actionCtx = actionContext{context: "ctx"}
	m.namespace = "default"
	assert.NotNil(t, m.bulkRestartResources())
}

func TestCovBatchPatchLabels(t *testing.T) {
	tests := []struct {
		name         string
		remove       bool
		isAnnotation bool
	}{
		{"add label", false, false},
		{"remove label", true, false},
		{"add annotation", false, true},
		{"remove annotation", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := baseModelCov()
			m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "ns1"}, {Name: "pod-2"}}
			m.actionCtx = actionContext{
				context:      "ctx",
				resourceType: model.ResourceTypeEntry{APIGroup: "apps", APIVersion: "v1", Resource: "deployments"},
			}
			m.namespace = "default"
			assert.NotNil(t, m.batchPatchLabels("env", "prod", tt.remove, tt.isAnnotation))
		})
	}
}

func TestCovBulkForceDeleteResources(t *testing.T) {
	m := baseModelCov()
	m.bulkItems = []model.Item{{Name: "stuck-pod", Namespace: "ns1"}}
	m.actionCtx = actionContext{context: "ctx", resourceType: model.ResourceTypeEntry{Resource: "pods", Namespaced: true}}
	m.namespace = "default"
	assert.NotNil(t, m.bulkForceDeleteResources())
}

func TestCovLoadPodsForAction(t *testing.T) {
	m := baseModelCov()
	m.actionCtx = actionContext{context: "ctx", kind: "Deployment", name: "my-deploy"}
	m.namespace = "default"
	assert.NotNil(t, m.loadPodsForAction())
}

func TestCovLoadContainersForAction(t *testing.T) {
	m := baseModelCov()
	m.actionCtx = actionContext{context: "ctx", kind: "Pod", name: "my-pod"}
	m.namespace = "default"
	assert.NotNil(t, m.loadContainersForAction())
}

func TestCovLoadContainersForLogFilter(t *testing.T) {
	m := baseModelCov()
	m.actionCtx = actionContext{context: "ctx", kind: "Pod", name: "my-pod"}
	m.namespace = "default"
	assert.NotNil(t, m.loadContainersForLogFilter())
}

func TestCovCopyYAMLContainersLevel(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "my-pod"
	assert.NotNil(t, m.copyYAMLToClipboard())
}

func TestCovCopyYAMLOwnedPod(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "my-pod", Kind: "Pod", Namespace: "ns1"}}
	assert.NotNil(t, m.copyYAMLToClipboard())
}

func TestCovCopyYAMLOwnedNonPodUnknown(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "my-rs", Kind: "ReplicaSet", Extra: "/v1/replicasets"}}
	assert.NotNil(t, m.copyYAMLToClipboard())
}

func TestCovCopyYAMLResourcesWithNamespace(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "nginx", Namespace: "web"}}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	assert.NotNil(t, m.copyYAMLToClipboard())
}

func TestCovExportResourceOwnedPod(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "my-pod", Kind: "Pod"}}
	assert.NotNil(t, m.exportResourceToFile())
}

func TestCovExportResourceOwnedUnknown(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "my-rs", Kind: "UnknownKind"}}
	assert.NotNil(t, m.exportResourceToFile())
}

func TestCovExportResourceContainers(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "my-pod"
	assert.NotNil(t, m.exportResourceToFile())
}

func TestCovExportResourceResources(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "nginx", Namespace: "web"}}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	assert.NotNil(t, m.exportResourceToFile())
}

// =====================================================================
// Target 2: commands_load.go partially covered functions
// =====================================================================

func TestCovLoadResourceTypesWithCRDs(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.discoveredCRDs["ctx"] = []model.ResourceTypeEntry{
		{Kind: "Application", Resource: "applications", APIGroup: "argoproj.io"},
	}
	assert.NotNil(t, m.loadResourceTypes())
}

func TestCovLoadResourceTypesNoCRDs(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	assert.NotNil(t, m.loadResourceTypes())
}

func TestCovLoadResourcesForPreviewNil(t *testing.T) {
	m := baseModelCov()
	m.middleItems = nil
	assert.Nil(t, m.loadResources(true))
}

func TestCovLoadResourcesForPreviewNoRT(t *testing.T) {
	m := baseModelCov()
	m.middleItems = []model.Item{{Name: "item-1", Extra: "nonexistent/v1/foos"}}
	m.nav.Context = "ctx"
	assert.Nil(t, m.loadResources(true))
}

func TestCovLoadResourcesNormal(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	m.namespace = "default"
	assert.NotNil(t, m.loadResources(false))
}

func TestCovLoadYAMLBranches(t *testing.T) {
	tests := []struct {
		name  string
		level model.Level
		items []model.Item
		want  bool
	}{
		{"resources", model.LevelResources, []model.Item{{Name: "nginx", Namespace: "default"}}, true},
		{"resources_empty", model.LevelResources, nil, false},
		{"owned_pod", model.LevelOwned, []model.Item{{Name: "my-pod", Kind: "Pod"}}, true},
		{"owned_unknown", model.LevelOwned, []model.Item{{Name: "my-rs", Kind: "UnknownKind"}}, true},
		{"owned_empty", model.LevelOwned, nil, false},
		{"containers", model.LevelContainers, nil, true},
		{"clusters", model.LevelClusters, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := baseModelCov()
			m.nav.Level = tt.level
			m.nav.Context = "ctx"
			m.nav.OwnedName = "pod-name"
			m.middleItems = tt.items
			m.namespace = "default"
			m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments"}
			cmd := m.loadYAML()
			if tt.want {
				assert.NotNil(t, cmd)
			} else {
				assert.Nil(t, cmd)
			}
		})
	}
}

func TestCovLoadMetricsBranches(t *testing.T) {
	tests := []struct {
		name string
		kind string
		want bool
	}{
		{"Pod", "Pod", true},
		{"Deployment", "Deployment", true},
		{"StatefulSet", "StatefulSet", true},
		{"DaemonSet", "DaemonSet", true},
		{"ConfigMap", "ConfigMap", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := baseModelCov()
			m.nav.ResourceType = model.ResourceTypeEntry{Kind: tt.kind}
			m.middleItems = []model.Item{{Name: "item-1"}}
			m.nav.Context = "ctx"
			m.namespace = "default"
			cmd := m.loadMetrics()
			if tt.want {
				assert.NotNil(t, cmd)
			} else {
				assert.Nil(t, cmd)
			}
		})
	}
}

func TestCovLoadMetricsNil(t *testing.T) {
	m := baseModelCov()
	m.middleItems = nil
	assert.Nil(t, m.loadMetrics())
}

func TestCovLoadMetricsOwnedLevel(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}
	m.middleItems = []model.Item{{Name: "my-pod", Kind: "Pod", Namespace: "ns1"}}
	m.nav.Context = "ctx"
	assert.NotNil(t, m.loadMetrics())
}

func TestCovLoadPreviewEventsNil(t *testing.T) {
	m := baseModelCov()
	m.middleItems = nil
	assert.Nil(t, m.loadPreviewEvents())
}

func TestCovLoadPreviewEventsNormal(t *testing.T) {
	m := baseModelCov()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.middleItems = []model.Item{{Name: "my-pod", Namespace: "ns1"}}
	m.nav.Context = "ctx"
	assert.NotNil(t, m.loadPreviewEvents())
}

func TestCovLoadPreviewEventsOwned(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}
	m.middleItems = []model.Item{{Name: "my-pod", Kind: "Pod"}}
	m.nav.Context = "ctx"
	assert.NotNil(t, m.loadPreviewEvents())
}

func TestCovLoadContainersBranches(t *testing.T) {
	m := baseModelCov()
	m.middleItems = nil
	assert.Nil(t, m.loadContainers(true), "forPreview nil selected")

	m2 := baseModelCov()
	m2.nav.Context = "ctx"
	m2.nav.OwnedName = "my-pod"
	m2.nav.Namespace = "from-nav"
	assert.NotNil(t, m2.loadContainers(false), "not preview with nav namespace")

	m3 := baseModelCov()
	m3.nav.Context = "ctx"
	m3.middleItems = []model.Item{{Name: "my-pod", Namespace: "ns1"}}
	assert.NotNil(t, m3.loadContainers(true), "forPreview with item")
}

func TestCovResolveOwnedResourceType(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.discoveredCRDs["ctx"] = nil

	_, ok := m.resolveOwnedResourceType(nil)
	assert.False(t, ok, "nil item")

	_, ok = m.resolveOwnedResourceType(&model.Item{Kind: "CompletelyUnknown"})
	assert.False(t, ok, "unknown kind")

	rt, ok := m.resolveOwnedResourceType(&model.Item{Kind: "CustomThing", Extra: "custom.io/v1"})
	assert.True(t, ok, "group/version fallback")
	assert.Equal(t, "custom.io", rt.APIGroup)
	assert.Equal(t, "v1", rt.APIVersion)
	assert.Equal(t, "customthings", rt.Resource)
}

func TestCovResolveNamespace(t *testing.T) {
	m := baseModelCov()
	m.nav.Namespace = "nav-ns"
	m.namespace = "model-ns"
	assert.Equal(t, "nav-ns", m.resolveNamespace())

	m.nav.Namespace = ""
	assert.Equal(t, "model-ns", m.resolveNamespace())
}

// =====================================================================
// Target 3: update_explain.go key handling (all 0%)
// =====================================================================

func TestCovHandleExplainKeyHelp(t *testing.T) {
	m := baseModelCov()
	m.mode = modeExplain
	result, cmd := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	assert.Equal(t, modeHelp, result.(Model).mode)
	assert.Nil(t, cmd)
}

func TestCovHandleExplainKeyQuit(t *testing.T) {
	m := baseModelCov()
	m.mode = modeExplain
	result, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.Equal(t, modeExplorer, result.(Model).mode)
}

func TestCovHandleExplainKeyEscRoot(t *testing.T) {
	m := baseModelCov()
	m.mode = modeExplain
	m.explainPath = ""
	result, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, modeExplorer, result.(Model).mode)
}

// TestCovHandleExplainKeyEscNested and EscSingle removed: those paths call
// execKubectlExplain which dereferences m.client (nil in unit tests).

func TestCovHandleExplainKeySlash(t *testing.T) {
	m := baseModelCov()
	result, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	assert.True(t, result.(Model).explainSearchActive)
}

func TestCovHandleExplainKeyJK(t *testing.T) {
	m := baseModelCov()
	m.explainFields = []model.ExplainField{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	m.explainCursor = 0
	r, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, r.(Model).explainCursor)

	m.explainCursor = 2
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 1, r.(Model).explainCursor)
}

func TestCovHandleExplainKeyGG(t *testing.T) {
	m := baseModelCov()
	m.explainFields = []model.ExplainField{{Name: "a"}, {Name: "b"}}

	r, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	assert.Equal(t, 1, r.(Model).explainCursor)

	m.explainCursor = 1
	m.pendingG = true
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	assert.Equal(t, 0, r.(Model).explainCursor)

	m.pendingG = false
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	assert.True(t, r.(Model).pendingG)
}

func TestCovHandleExplainKeyDrillPrimitive(t *testing.T) {
	// Only test the non-drillable (primitive) branch; drillable calls execKubectlExplain
	// which dereferences m.client (nil in unit tests).
	m := baseModelCov()
	m.explainFields = []model.ExplainField{{Name: "name", Type: "string"}}
	r, cmd := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	assert.True(t, r.(Model).statusMessageErr)
	assert.NotNil(t, cmd)

	// Also test enter/l with no fields (out-of-bounds cursor).
	m.explainFields = nil
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, r)
}

func TestCovHandleExplainKeyBackRoot(t *testing.T) {
	// Only test the root-level exit path; nested path calls execKubectlExplain
	// which dereferences m.client (nil in unit tests).
	m := baseModelCov()
	m.explainPath = ""
	r, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	assert.Equal(t, modeExplorer, r.(Model).mode)
}

// TestCovHandleExplainKeyRecursive removed: 'r' calls execKubectlExplainRecursive
// which dereferences m.client (nil in unit tests).

func TestCovHandleExplainKeySearchNextPrev(t *testing.T) {
	m := baseModelCov()
	m.explainSearchQuery = "status"
	m.explainFields = []model.ExplainField{{Name: "spec"}, {Name: "status"}, {Name: "meta"}}

	r, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, 1, r.(Model).explainCursor)

	m2 := baseModelCov()
	m2.explainSearchQuery = "spec"
	m2.explainFields = m.explainFields
	m2.explainCursor = 2
	r, _ = m2.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	assert.Equal(t, 0, r.(Model).explainCursor)

	m3 := baseModelCov()
	m3.explainSearchQuery = "zzz"
	m3.explainFields = m.explainFields
	r, cmd := m3.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.True(t, r.(Model).statusMessageErr)
	assert.NotNil(t, cmd)

	r, cmd = m3.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	assert.True(t, r.(Model).statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestCovHandleExplainKeyPages(t *testing.T) {
	fields := make([]model.ExplainField, 50)
	for i := range fields {
		fields[i] = model.ExplainField{Name: "f"}
	}
	m := baseModelCov()
	m.explainFields = fields

	r, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.Greater(t, r.(Model).explainCursor, 0)

	m.explainCursor = 25
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.Less(t, r.(Model).explainCursor, 25)

	m.explainCursor = 0
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	assert.Greater(t, r.(Model).explainCursor, 0)

	m.explainCursor = 40
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	assert.Less(t, r.(Model).explainCursor, 40)
}

func TestCovHandleExplainSearchKey(t *testing.T) {
	m := baseModelCov()
	m.explainSearchActive = true
	m.explainSearchInput = TextInput{Value: "spec"}
	r, _ := m.handleExplainSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, r.(Model).explainSearchActive)
	assert.Equal(t, "spec", r.(Model).explainSearchQuery)

	m.explainSearchActive = true
	m.explainSearchPrevCursor = 3
	r, _ = m.handleExplainSearchKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, r.(Model).explainSearchActive)
	assert.Equal(t, 3, r.(Model).explainCursor)

	m.explainSearchActive = true
	m.explainSearchInput = TextInput{Value: "sp", Cursor: 2}
	m.explainFields = []model.ExplainField{{Name: "spec"}}
	r, _ = m.handleExplainSearchKey(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "s", r.(Model).explainSearchInput.Value)

	m.explainSearchInput = TextInput{Value: "hello", Cursor: 5}
	r, _ = m.handleExplainSearchKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.NotEqual(t, "hello", r.(Model).explainSearchInput.Value)

	m.explainSearchInput = TextInput{}
	m.explainFields = []model.ExplainField{{Name: "spec"}}
	r, _ = m.handleExplainSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	assert.Equal(t, "s", r.(Model).explainSearchInput.Value)
}

// =====================================================================
// Target 3: update_cani.go key handling (all 0%)
// =====================================================================

func TestCovHandleCanIKeyHelp(t *testing.T) {
	m := baseModelCov()
	m.canIGroups = []model.CanIGroup{{Name: "core"}}
	r, _ := m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	assert.Equal(t, modeHelp, r.(Model).mode)
}

func TestCovHandleCanIKeyToggleAllowed(t *testing.T) {
	m := baseModelCov()
	r, _ := m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.True(t, r.(Model).canIAllowedOnly)
}

func TestCovHandleCanIKeyQuit(t *testing.T) {
	m := baseModelCov()
	m.canISearchQuery = "apps"
	r, _ := m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.Empty(t, r.(Model).canISearchQuery)

	m.canISearchQuery = ""
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.Equal(t, overlayNone, r.(Model).overlay)
}

func TestCovHandleCanIKeyNavigation(t *testing.T) {
	m := baseModelCov()
	m.canIGroups = []model.CanIGroup{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	r, _ := m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, r.(Model).canIGroupCursor)

	m.canIGroupCursor = 2
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 1, r.(Model).canIGroupCursor)

	m.canIGroupCursor = 0
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	assert.Equal(t, 2, r.(Model).canIGroupCursor)

	m.canIGroupCursor = 2
	m.pendingG = true
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	assert.Equal(t, 0, r.(Model).canIGroupCursor)
}

func TestCovHandleCanIKeyScrollResource(t *testing.T) {
	m := baseModelCov()
	m.canIGroups = []model.CanIGroup{{
		Name:      "core",
		Resources: make([]model.CanIResource, 20),
	}}
	r, _ := m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	assert.GreaterOrEqual(t, r.(Model).canIResourceScroll, 0)

	m.canIResourceScroll = 1
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	assert.Equal(t, 0, r.(Model).canIResourceScroll)
}

func TestCovHandleCanIKeyPages(t *testing.T) {
	groups := make([]model.CanIGroup, 50)
	for i := range groups {
		groups[i] = model.CanIGroup{Name: "g"}
	}
	m := baseModelCov()
	m.canIGroups = groups

	r, _ := m.handleCanIKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.Greater(t, r.(Model).canIGroupCursor, 0)

	m.canIGroupCursor = 25
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.Less(t, r.(Model).canIGroupCursor, 25)

	m.canIGroupCursor = 0
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	assert.Greater(t, r.(Model).canIGroupCursor, 0)

	m.canIGroupCursor = 40
	r, _ = m.handleCanIKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	assert.Less(t, r.(Model).canIGroupCursor, 40)
}

func TestCovHandleCanISearchKey(t *testing.T) {
	m := baseModelCov()
	m.canISearchActive = true
	m.canISearchInput = TextInput{Value: "core"}
	r, _ := m.handleCanISearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, "core", r.(Model).canISearchQuery)

	m.canISearchActive = true
	r, _ = m.handleCanISearchKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, r.(Model).canISearchActive)

	m.canISearchActive = true
	m.canISearchInput = TextInput{Value: "co", Cursor: 2}
	r, _ = m.handleCanISearchKey(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "c", r.(Model).canISearchInput.Value)

	m.canISearchInput = TextInput{Value: "hello", Cursor: 5}
	r, _ = m.handleCanISearchKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Empty(t, r.(Model).canISearchInput.Value)

	m.canISearchInput = TextInput{}
	r, _ = m.handleCanISearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.Equal(t, "a", r.(Model).canISearchInput.Value)
}

// =====================================================================
// Target 3: update_bookmarks.go key handling (all 0%)
// =====================================================================

func TestCovHandleBookmarkOverlayKeyDispatch(t *testing.T) {
	m := baseModelCov()
	m.bookmarkSearchMode = bookmarkModeFilter
	r, _ := m.handleBookmarkOverlayKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)

	m.bookmarkSearchMode = bookmarkModeConfirmDelete
	r, _ = m.handleBookmarkOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)

	m.bookmarkSearchMode = bookmarkModeConfirmDeleteAll
	r, _ = m.handleBookmarkOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)
}

func TestCovHandleBookmarkNormalMode(t *testing.T) {
	filtered := []model.Bookmark{{Name: "a", Slot: "1"}, {Name: "b", Slot: "2"}, {Name: "c", Slot: "3"}}
	m := baseModelCov()
	m.overlay = overlayBookmarks

	r, _ := m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyEscape}, nil)
	assert.Equal(t, overlayNone, r.(Model).overlay)

	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, filtered)
	assert.Equal(t, 1, r.(Model).overlayCursor)

	m.overlayCursor = 1
	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, filtered)
	assert.Equal(t, 0, r.(Model).overlayCursor)

	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}, filtered)
	assert.Equal(t, 2, r.(Model).overlayCursor)

	m.pendingG = true
	m.overlayCursor = 2
	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}, filtered)
	assert.Equal(t, 0, r.(Model).overlayCursor)

	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}}, filtered)
	assert.Equal(t, bookmarkModeConfirmDelete, r.(Model).bookmarkSearchMode)

	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyCtrlX}, filtered)
	assert.Equal(t, bookmarkModeConfirmDeleteAll, r.(Model).bookmarkSearchMode)

	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}, filtered)
	assert.Equal(t, bookmarkModeFilter, r.(Model).bookmarkSearchMode)

	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyCtrlD}, filtered)
	assert.GreaterOrEqual(t, r.(Model).overlayCursor, 0)

	r, _ = m.handleBookmarkNormalMode(tea.KeyMsg{Type: tea.KeyCtrlU}, filtered)
	assert.GreaterOrEqual(t, r.(Model).overlayCursor, 0)
}

func TestCovHandleBookmarkFilterMode(t *testing.T) {
	m := baseModelCov()
	m.bookmarkSearchMode = bookmarkModeFilter

	r, _ := m.handleBookmarkFilterMode(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)

	m.bookmarkSearchMode = bookmarkModeFilter
	m.bookmarkFilter = TextInput{Value: "pr", Cursor: 2}
	r, _ = m.handleBookmarkFilterMode(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "p", r.(Model).bookmarkFilter.Value)

	m.bookmarkFilter = TextInput{}
	r, _ = m.handleBookmarkFilterMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	assert.Equal(t, "p", r.(Model).bookmarkFilter.Value)

	m.bookmarkFilter = TextInput{Value: "hello", Cursor: 5}
	r, _ = m.handleBookmarkFilterMode(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Empty(t, r.(Model).bookmarkFilter.Value)

	r, _ = m.handleBookmarkFilterMode(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)
}

func TestCovHandleBookmarkConfirmDelete(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseModelCov()
	m.bookmarkSearchMode = bookmarkModeConfirmDelete
	m.bookmarks = []model.Bookmark{{Name: "test", Slot: "a"}}
	r, cmd := m.handleBookmarkConfirmDelete(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)
	assert.NotNil(t, cmd)

	r, cmd = m.handleBookmarkConfirmDelete(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)
	assert.NotNil(t, cmd)
}

func TestCovHandleBookmarkConfirmDeleteAll(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := baseModelCov()
	m.bookmarks = []model.Bookmark{{Name: "test", Slot: "a"}}
	r, cmd := m.handleBookmarkConfirmDeleteAll(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)
	assert.NotNil(t, cmd)

	m.bookmarks = []model.Bookmark{{Name: "test", Slot: "a"}}
	r, cmd = m.handleBookmarkConfirmDeleteAll(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, bookmarkModeNormal, r.(Model).bookmarkSearchMode)
	assert.NotNil(t, cmd)
}

// =====================================================================
// Target 4: update.go more Update() branches
// =====================================================================

func TestCovUpdateBulkActionResult(t *testing.T) {
	m := baseModelCov()
	m.bulkMode = true

	r, cmd := m.Update(bulkActionResultMsg{succeeded: 3, failed: 0})
	assert.Contains(t, r.(Model).statusMessage, "3")
	assert.NotNil(t, cmd)

	r, cmd = m.Update(bulkActionResultMsg{succeeded: 2, failed: 1, errors: []string{"err"}})
	assert.True(t, r.(Model).statusMessageErr)
	assert.NotNil(t, cmd)

	r, cmd = m.Update(bulkActionResultMsg{succeeded: 0, failed: 2, errors: []string{"a", "b"}})
	assert.True(t, r.(Model).statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestCovUpdateCommandBarResult(t *testing.T) {
	m := baseModelCov()

	r, _ := m.Update(commandBarResultMsg{output: "pod deleted"})
	assert.Equal(t, modeDescribe, r.(Model).mode)

	r, _ = m.Update(commandBarResultMsg{output: "forbidden", err: assert.AnError})
	assert.Equal(t, modeDescribe, r.(Model).mode)

	r, cmd := m.Update(commandBarResultMsg{err: assert.AnError})
	assert.True(t, r.(Model).statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestCovUpdateContainersLoadedPreview(t *testing.T) {
	m := baseModelCov()
	m.requestGen = 1
	r, cmd := m.Update(containersLoadedMsg{items: []model.Item{{Name: "c1"}}, forPreview: true, gen: 1})
	assert.Len(t, r.(Model).rightItems, 1)
	assert.Nil(t, cmd)
}

func TestCovUpdateContainersLoadedStale(t *testing.T) {
	m := baseModelCov()
	m.requestGen = 2
	_, cmd := m.Update(containersLoadedMsg{gen: 1})
	assert.Nil(t, cmd)
}

func TestCovUpdateNamespacesLoaded(t *testing.T) {
	m := baseModelCov()
	r, _ := m.Update(namespacesLoadedMsg{items: []model.Item{{Name: "default"}, {Name: "kube-system"}}})
	assert.Len(t, r.(Model).overlayItems, 3) // "All Namespaces" + 2

	m.allNamespaces = true
	r, _ = m.Update(namespacesLoadedMsg{items: []model.Item{{Name: "default"}}})
	assert.Equal(t, 0, r.(Model).overlayCursor)
}

func TestCovUpdatePreviewYAMLLoaded(t *testing.T) {
	m := baseModelCov()
	m.requestGen = 5
	r, _ := m.Update(previewYAMLLoadedMsg{content: "apiVersion: v1", gen: 5})
	assert.Contains(t, r.(Model).previewYAML, "apiVersion")

	r, _ = m.Update(previewYAMLLoadedMsg{content: "stale", gen: 3})
	assert.Empty(t, r.(Model).previewYAML)

	r, _ = m.Update(previewYAMLLoadedMsg{err: assert.AnError, gen: 5})
	assert.Empty(t, r.(Model).previewYAML)
}

func TestCovUpdateOwnedLoadedPreview(t *testing.T) {
	m := baseModelCov()
	m.requestGen = 1
	r, cmd := m.Update(ownedLoadedMsg{items: []model.Item{{Name: "pod-1"}}, forPreview: true, gen: 1})
	assert.Len(t, r.(Model).rightItems, 1)
	assert.Nil(t, cmd)

	_, cmd = m.Update(ownedLoadedMsg{gen: 0})
	assert.Nil(t, cmd)
}

func TestCovUpdateResourceTreeLoadedStale(t *testing.T) {
	m := baseModelCov()
	m.requestGen = 5
	_, cmd := m.Update(resourceTreeLoadedMsg{gen: 3})
	assert.Nil(t, cmd)
}

// =====================================================================
// Target 5: view functions
// =====================================================================

func TestCovViewExecTerminalSmall(t *testing.T) {
	m := Model{
		width: 15, height: 8, mode: modeExec, execTitle: "Exec",
		tabs: []TabState{{}}, execMu: &sync.Mutex{},
	}
	assert.NotEmpty(t, m.viewExecTerminal())
}

func TestCovMiddleColumnHeader(t *testing.T) {
	tests := []struct {
		level  model.Level
		kind   string
		expect string
	}{
		{model.LevelClusters, "", "KUBECONFIG"},
		{model.LevelResourceTypes, "", "RESOURCE TYPE"},
		{model.LevelResources, "Pod", "POD"},
		{model.LevelContainers, "", "CONTAINER"},
		{99, "", ""},
	}
	for _, tt := range tests {
		m := Model{nav: model.NavigationState{Level: tt.level, ResourceType: model.ResourceTypeEntry{Kind: tt.kind}}}
		assert.Equal(t, tt.expect, m.middleColumnHeader())
	}
}

func TestCovMiddleColumnHeaderOwned(t *testing.T) {
	for _, kind := range []string{"CronJob", "Application", "Pod", "Node", "Deployment"} {
		m := Model{nav: model.NavigationState{Level: model.LevelOwned, ResourceType: model.ResourceTypeEntry{Kind: kind}}}
		assert.NotEmpty(t, m.middleColumnHeader())
	}
}

func TestCovLeftColumnHeader(t *testing.T) {
	tests := []struct {
		level  model.Level
		dn     string
		expect string
	}{
		{model.LevelClusters, "", ""},
		{model.LevelResourceTypes, "", "KUBECONFIG"},
		{model.LevelResources, "", "RESOURCE TYPE"},
		{model.LevelOwned, "Deployments", "DEPLOYMENTS"},
		{model.LevelContainers, "Pods", "PODS"},
		{99, "", ""},
	}
	for _, tt := range tests {
		m := Model{nav: model.NavigationState{Level: tt.level, ResourceType: model.ResourceTypeEntry{DisplayName: tt.dn}}}
		assert.Equal(t, tt.expect, m.leftColumnHeader())
	}
}

func TestCovViewLogsBasic(t *testing.T) {
	m := Model{
		width: 80, height: 30, tabs: []TabState{{}},
		logLines: []string{"line 1", "line 2"}, logTitle: "Logs: pod",
		logFollow: true, actionCtx: actionContext{kind: "Pod"}, logSearchInput: TextInput{},
	}
	assert.NotEmpty(t, m.viewLogs())
}

func TestCovViewLogsSearch(t *testing.T) {
	m := Model{
		width: 80, height: 30, tabs: []TabState{{}},
		logLines: []string{"line"}, logTitle: "Logs",
		logSearchActive: true, logSearchInput: TextInput{Value: "err"},
		actionCtx: actionContext{kind: "Pod"},
	}
	assert.NotEmpty(t, m.viewLogs())
}

func TestCovViewLogsStatus(t *testing.T) {
	m := Model{
		width: 80, height: 30, tabs: []TabState{{}},
		logLines: []string{"line"}, logTitle: "Logs",
		statusMessage: "Copied", actionCtx: actionContext{kind: "Pod"},
		logSearchInput: TextInput{},
	}
	assert.NotEmpty(t, m.viewLogs())
}

// =====================================================================
// Target 6: filters.go missing branches
// =====================================================================

func TestCovFilterPresetsServiceLBNoIP(t *testing.T) {
	presets := builtinFilterPresets("Service")
	lbNoIP := findPreset(presets, "LB No IP")
	require.NotNil(t, lbNoIP)

	assert.True(t, lbNoIP.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Type", Value: "LoadBalancer"}, {Key: "External-IP", Value: "<pending>"}}}))
	assert.True(t, lbNoIP.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Type", Value: "LoadBalancer"}, {Key: "External-IP", Value: "<none>"}}}))
	assert.True(t, lbNoIP.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Type", Value: "LoadBalancer"}, {Key: "External-IP", Value: ""}}}))
	assert.False(t, lbNoIP.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Type", Value: "LoadBalancer"}, {Key: "External-IP", Value: "1.2.3.4"}}}))
	assert.False(t, lbNoIP.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Type", Value: "ClusterIP"}}}))
}

func TestCovFilterPresetsNodeCordoned(t *testing.T) {
	presets := builtinFilterPresets("Node")
	cordoned := findPreset(presets, "Cordoned")
	require.NotNil(t, cordoned)
	assert.True(t, cordoned.MatchFn(model.Item{Status: "Ready,SchedulingDisabled"}))
	assert.False(t, cordoned.MatchFn(model.Item{Status: "Ready"}))
}

func TestCovFilterPresetsEventWarnings(t *testing.T) {
	presets := builtinFilterPresets("Event")
	warnings := findPreset(presets, "Warnings")
	require.NotNil(t, warnings)
	assert.True(t, warnings.MatchFn(model.Item{Status: "Warning"}))
	assert.False(t, warnings.MatchFn(model.Item{Status: "Normal"}))
}

func TestCovFilterPresetsCertExpiring(t *testing.T) {
	presets := builtinFilterPresets("Certificate")
	expiring := findPreset(presets, "Expiring Soon")
	require.NotNil(t, expiring)

	in15d := time.Now().Add(15 * 24 * time.Hour).Format(time.RFC3339)
	assert.True(t, expiring.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Expires", Value: in15d}}}))

	in60d := time.Now().Add(60 * 24 * time.Hour).Format(time.RFC3339)
	assert.False(t, expiring.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Expires", Value: in60d}}}))
	assert.False(t, expiring.MatchFn(model.Item{}))

	in10d := time.Now().Add(10 * 24 * time.Hour).Format("2006-01-02")
	assert.True(t, expiring.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Not After", Value: in10d}}}))
}

func TestCovBuildConfigMatchFnCombined(t *testing.T) {
	fn := buildConfigMatchFn(ui.ConfigFilterMatch{Status: "error", ReadyNot: true})
	assert.True(t, fn(model.Item{Status: "Error", Ready: "0/1"}))
	assert.False(t, fn(model.Item{Status: "Error", Ready: "1/1"}))
	assert.False(t, fn(model.Item{Status: "Running", Ready: "0/1"}))
}

func TestCovBuildConfigMatchFnColumnNoValue(t *testing.T) {
	fn := buildConfigMatchFn(ui.ConfigFilterMatch{Column: "IP"})
	assert.True(t, fn(model.Item{Columns: []model.KeyValue{{Key: "IP", Value: "10.0.0.1"}}}))
	assert.False(t, fn(model.Item{Columns: []model.KeyValue{{Key: "IP", Value: ""}}}))
}

func TestCovBuildConfigMatchFnRestartsGt(t *testing.T) {
	fn := buildConfigMatchFn(ui.ConfigFilterMatch{RestartsGt: 5})
	assert.True(t, fn(model.Item{Restarts: "10"}))
	assert.False(t, fn(model.Item{Restarts: "3"}))
	assert.False(t, fn(model.Item{Restarts: ""}))
}

// =====================================================================
// Additional: processCanIRules (0%)
// =====================================================================

func TestCovProcessCanIRulesWildcard(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.discoveredCRDs["ctx"] = nil

	m.processCanIRules([]k8s.AccessRule{
		{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"*"}},
	})
	assert.NotEmpty(t, m.canIGroups)
}

func TestCovProcessCanIRulesWildcardAll(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.discoveredCRDs["ctx"] = nil

	m.processCanIRules([]k8s.AccessRule{
		{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get", "list"}},
	})
	assert.NotEmpty(t, m.canIGroups)
}

func TestCovProcessCanIRulesEmpty(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.discoveredCRDs["ctx"] = nil

	m.processCanIRules(nil)
	assert.NotEmpty(t, m.canIGroups)
}

func TestCovProcessCanIRulesWithCRDs(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.discoveredCRDs["ctx"] = []model.ResourceTypeEntry{
		{Kind: "Application", Resource: "applications", APIGroup: "argoproj.io", APIVersion: "v1alpha1"},
	}

	m.processCanIRules([]k8s.AccessRule{
		{APIGroups: []string{"argoproj.io"}, Resources: []string{"applications"}, Verbs: []string{"get", "list"}},
	})

	var found bool
	for _, g := range m.canIGroups {
		if g.Name == "argoproj.io" {
			for _, r := range g.Resources {
				if r.Resource == "applications" {
					found = true
					assert.True(t, r.Verbs["get"])
					assert.True(t, r.Verbs["list"])
				}
			}
		}
	}
	assert.True(t, found, "should find argoproj.io/applications")
}

// =====================================================================
// Additional: explain overlay key handlers
// =====================================================================

func TestCovExplainSearchOverlayNormalNav(t *testing.T) {
	m := baseModelCov()
	m.explainRecursiveResults = []model.ExplainField{{Name: "a", Path: "a"}, {Name: "b", Path: "b"}}

	r, _ := m.handleExplainSearchOverlayNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, r.(Model).explainRecursiveCursor)

	m2 := r.(Model)
	r, _ = m2.handleExplainSearchOverlayNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, r.(Model).explainRecursiveCursor)

	r, _ = m.handleExplainSearchOverlayNormalKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, overlayNone, r.(Model).overlay)

	r, _ = m.handleExplainSearchOverlayNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	assert.True(t, r.(Model).explainRecursiveFilterActive)

	m.pendingG = true
	m.explainRecursiveCursor = 1
	r, _ = m.handleExplainSearchOverlayNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	assert.Equal(t, 0, r.(Model).explainRecursiveCursor)

	m.explainRecursiveCursor = 0
	r, _ = m.handleExplainSearchOverlayNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	assert.Equal(t, 1, r.(Model).explainRecursiveCursor)
}

func TestCovExplainSearchOverlayFilterKey(t *testing.T) {
	m := baseModelCov()
	m.explainRecursiveFilterActive = true
	m.explainRecursiveFilter = TextInput{}

	r, _ := m.handleExplainSearchOverlayFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	assert.Equal(t, "s", r.(Model).explainRecursiveFilter.Value)

	r, _ = m.handleExplainSearchOverlayFilterKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, r.(Model).explainRecursiveFilterActive)

	m.explainRecursiveFilter = TextInput{Value: "test"}
	r, _ = m.handleExplainSearchOverlayFilterKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Empty(t, r.(Model).explainRecursiveFilter.Value)

	m.explainRecursiveFilter = TextInput{Value: "ab", Cursor: 2}
	r, _ = m.handleExplainSearchOverlayFilterKey(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "a", r.(Model).explainRecursiveFilter.Value)

	m.explainRecursiveFilter = TextInput{Value: "hello", Cursor: 5}
	r, _ = m.handleExplainSearchOverlayFilterKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Empty(t, r.(Model).explainRecursiveFilter.Value)
}

// =====================================================================
// Additional: Can-I subject overlay (0%)
// =====================================================================

func TestCovCanISubjectNormalMode(t *testing.T) {
	m := baseModelCov()
	m.overlayItems = []model.Item{{Name: "sa1"}, {Name: "sa2"}}

	r, _ := m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, r.(Model).overlayCursor)

	m.overlayCursor = 1
	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, r.(Model).overlayCursor)

	m.overlay = overlayCanISubject
	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, overlayCanI, r.(Model).overlay)

	m.overlayFilter = TextInput{Value: "admin"}
	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Empty(t, r.(Model).overlayFilter.Value)

	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	assert.True(t, r.(Model).canISubjectFilterMode)

	// Reset filter so filteredOverlayItems returns all items for page scroll tests.
	m.overlayFilter = TextInput{}
	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.GreaterOrEqual(t, r.(Model).overlayCursor, 0)

	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.GreaterOrEqual(t, r.(Model).overlayCursor, 0)

	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyCtrlF})
	assert.GreaterOrEqual(t, r.(Model).overlayCursor, 0)

	r, _ = m.handleCanISubjectNormalMode(tea.KeyMsg{Type: tea.KeyCtrlB})
	assert.GreaterOrEqual(t, r.(Model).overlayCursor, 0)
}

func TestCovCanISubjectFilterMode(t *testing.T) {
	m := baseModelCov()
	m.canISubjectFilterMode = true

	r, _ := m.handleCanISubjectFilterMode(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, r.(Model).canISubjectFilterMode)

	m.overlayFilter = TextInput{Value: "admin"}
	r, _ = m.handleCanISubjectFilterMode(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Empty(t, r.(Model).overlayFilter.Value)

	m.overlayFilter = TextInput{}
	r, _ = m.handleCanISubjectFilterMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.Equal(t, "a", r.(Model).overlayFilter.Value)

	m.overlayFilter = TextInput{Value: "ab", Cursor: 2}
	r, _ = m.handleCanISubjectFilterMode(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "a", r.(Model).overlayFilter.Value)

	m.overlayFilter = TextInput{Value: "hello", Cursor: 5}
	r, _ = m.handleCanISubjectFilterMode(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Empty(t, r.(Model).overlayFilter.Value)
}
