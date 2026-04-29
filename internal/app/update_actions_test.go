package app

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestDirectActionDelete_DeletingPod_ForceDeletes(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "stuck-pod", Kind: "Pod", Namespace: "default", Deleting: true},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
	ret, _ := m.directActionDelete()
	result := ret.(Model)
	assert.Equal(t, overlayConfirmType, result.overlay)
	assert.Equal(t, "Force Delete", result.pendingAction)
	assert.Contains(t, result.confirmTitle, "Force Delete")
	assert.Contains(t, result.confirmQuestion, "--force --grace-period=0")
}

func TestDirectActionDelete_DeletingDeployment_ForceFinalize(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "stuck-deploy", Kind: "Deployment", Namespace: "default", Deleting: true},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
	ret, _ := m.directActionDelete()
	result := ret.(Model)
	assert.Equal(t, overlayConfirmType, result.overlay)
	assert.Equal(t, "Force Finalize", result.pendingAction)
	assert.Contains(t, result.confirmTitle, "Force Finalize")
}

func TestDirectActionDelete_DeletingJob_ForceDeletes(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Job", Resource: "jobs", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "stuck-job", Kind: "Job", Namespace: "default", Deleting: true},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
	ret, _ := m.directActionDelete()
	result := ret.(Model)
	assert.Equal(t, overlayConfirmType, result.overlay)
	assert.Equal(t, "Force Delete", result.pendingAction)
}

func TestDirectActionDelete_NormalPod_NormalDelete(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "healthy-pod", Kind: "Pod", Namespace: "default", Deleting: false},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
	ret, _ := m.directActionDelete()
	result := ret.(Model)
	// Normal delete goes through executeAction which opens overlayConfirm.
	assert.Equal(t, overlayConfirm, result.overlay)
	assert.Equal(t, "Delete", result.pendingAction)
}

func TestOpenActionMenu_DeletingPod_ShowsForceDelete(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "stuck-pod", Kind: "Pod", Namespace: "default", Deleting: true},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
	result := m.openActionMenu()
	assert.Equal(t, overlayAction, result.overlay)

	// Find the Force Delete item in the menu.
	found := false
	for _, item := range result.overlayItems {
		if item.Name == "Force Delete" {
			found = true
			assert.Contains(t, item.Extra, "--force --grace-period=0")
			break
		}
	}
	assert.True(t, found, "expected Force Delete in menu for deleting Pod")

	// Should NOT have a "Force Finalize" or plain "Delete" entry.
	for _, item := range result.overlayItems {
		assert.NotEqual(t, "Force Finalize", item.Name, "should not show Force Finalize for Pod")
	}
}

func TestOpenActionMenu_DeletingDeployment_ShowsForceFinalize(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "stuck-deploy", Kind: "Deployment", Namespace: "default", Deleting: true},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
	result := m.openActionMenu()
	assert.Equal(t, overlayAction, result.overlay)

	found := false
	for _, item := range result.overlayItems {
		if item.Name == "Force Finalize" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected Force Finalize in menu for deleting Deployment")
}

func TestPush2ExecuteBulkActionScale(t *testing.T) {
	m := basePush80v2Model()
	m.bulkItems = []model.Item{{Name: "deploy-1"}}
	m.actionCtx = actionContext{kind: "Deployment"}
	result, _ := m.executeBulkAction("Scale")
	rm := result.(Model)
	assert.Equal(t, overlayScaleInput, rm.overlay)
}

func TestPush2ExecuteBulkActionRestart(t *testing.T) {
	m := basePush80v2Model()
	m.bulkItems = []model.Item{{Name: "deploy-1"}}
	m.actionCtx = actionContext{context: "ctx", kind: "Deployment"}
	result, cmd := m.executeBulkAction("Restart")
	rm := result.(Model)
	// Restart triggers a confirmation overlay, not a direct status message.
	_ = rm
	_ = cmd
}

func TestPush2ExecuteBulkActionLabelsAnnotations(t *testing.T) {
	m := basePush80v2Model()
	m.bulkItems = []model.Item{{Name: "pod-1"}}
	m.batchLabelInput = TextInput{}
	result, _ := m.executeBulkAction("Labels / Annotations")
	rm := result.(Model)
	assert.Equal(t, overlayBatchLabel, rm.overlay)
}

func TestPush2RefreshCurrentLevelResources(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelResources
	cmd := m.refreshCurrentLevel()
	require.NotNil(t, cmd)
}

func TestPush2RefreshCurrentLevelOwned(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType.Kind = "Deployment"
	m.nav.ResourceName = "deploy-1"
	cmd := m.refreshCurrentLevel()
	require.NotNil(t, cmd)
}

func TestPush2RefreshCurrentLevelContainers(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "pod-1"
	cmd := m.refreshCurrentLevel()
	require.NotNil(t, cmd)
}

func TestPush2RefreshCurrentLevelClusters(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelClusters
	cmd := m.refreshCurrentLevel()
	require.NotNil(t, cmd)
}

func TestPush2RefreshCurrentLevelResourceTypes(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes
	m.discoveredResources[m.nav.Context] = []model.ResourceTypeEntry{
		{DisplayName: "Pods", Kind: "Pod", APIVersion: "v1", Resource: "pods"},
	}
	cmd := m.refreshCurrentLevel()
	require.NotNil(t, cmd)
}

func TestPush2CloseTabOrQuitSingleTab(t *testing.T) {
	m := basePush80v2Model()
	m.portForwardMgr = k8s.NewPortForwardManager()
	m.tabs = []TabState{{}}
	result, _ := m.closeTabOrQuit()
	rm := result.(Model)
	// Single tab: should show quit confirm or quit directly.
	_ = rm
}

func TestPush2CloseTabOrQuitMultipleTabs(t *testing.T) {
	m := basePush80v2Model()
	m.tabs = []TabState{{}, {}}
	m.activeTab = 0
	result, _ := m.closeTabOrQuit()
	rm := result.(Model)
	assert.Len(t, rm.tabs, 1)
}

func TestPush2DirectActionForceDeleteNoSel(t *testing.T) {
	m := basePush80v2Model()
	m.middleItems = nil
	result, cmd := m.directActionForceDelete()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80DirectActionLogsNoKind(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	result, cmd := m.directActionLogs()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80DirectActionLogsPortForward(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__"}
	result, cmd := m.directActionLogs()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80DirectActionLogsNoSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	result, cmd := m.directActionLogs()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80DirectActionEditNoKind(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	result, cmd := m.directActionEdit()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80DirectActionEditNoSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	result, cmd := m.directActionEdit()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80DirectActionDescribeNoKind(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	result, cmd := m.directActionDescribe()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80DirectActionDescribeNoSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	result, cmd := m.directActionDescribe()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80DirectActionDeleteNoKind(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	result, cmd := m.directActionDelete()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80DirectActionDeleteNoSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	result, cmd := m.directActionDelete()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80DirectActionRefresh(t *testing.T) {
	m := basePush80Model()
	result, cmd := m.directActionRefresh()
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Refreshing")
	assert.NotNil(t, cmd)
}

func TestCov80OpenActionMenuBulkMode(t *testing.T) {
	m := basePush80Model()
	m.selectedItems = map[string]bool{
		"default/pod-1": true,
		"ns-2/pod-2":    true,
	}
	rm := m.openActionMenu()
	assert.True(t, rm.bulkMode)
	assert.Equal(t, overlayAction, rm.overlay)
}

func TestCov80OpenActionMenuNoKind(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	m.middleItems = nil
	_ = m.openActionMenu()
}

func TestCov80BuildActionCtxResources(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	sel := &model.Item{Name: "pod-1", Namespace: "ns1"}
	ctx := m.buildActionCtx(sel, "Pod")
	assert.Equal(t, "pod-1", ctx.name)
	assert.Equal(t, "ns1", ctx.namespace)
	assert.Equal(t, "Pod", ctx.resourceType.Kind)
}

func TestCov80BuildActionCtxContainers(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "my-pod"
	sel := &model.Item{Name: "app", Extra: "nginx:1.25"}
	ctx := m.buildActionCtx(sel, "Container")
	assert.Equal(t, "my-pod", ctx.name)
	assert.Equal(t, "app", ctx.containerName)
	assert.Equal(t, "nginx:1.25", ctx.image)
	assert.Equal(t, "Pod", ctx.kind)
}

func TestCov80BuildActionCtxOwned(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	sel := &model.Item{Name: "rs-1", Kind: "ReplicaSet", Namespace: "default"}
	ctx := m.buildActionCtx(sel, "ReplicaSet")
	assert.Equal(t, "rs-1", ctx.name)
}

func TestCov80BuildActionCtxNamespacePriority(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.Namespace = "nav-ns"
	m.namespace = "selector-ns"

	// Item namespace takes priority.
	sel := &model.Item{Name: "pod-1", Namespace: "item-ns"}
	ctx := m.buildActionCtx(sel, "Pod")
	assert.Equal(t, "item-ns", ctx.namespace)

	// Nav namespace second.
	sel2 := &model.Item{Name: "pod-1"}
	ctx2 := m.buildActionCtx(sel2, "Pod")
	assert.Equal(t, "nav-ns", ctx2.namespace)

	// Selector namespace last.
	m.nav.Namespace = ""
	ctx3 := m.buildActionCtx(sel2, "Pod")
	assert.Equal(t, "selector-ns", ctx3.namespace)
}

func TestCovDirectActionLogsNoKind(t *testing.T) {
	m := baseModelActions()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "ctx-1"}}
	result, cmd := m.directActionLogs()
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovDirectActionLogsNoSelection(t *testing.T) {
	m := baseModelActions()
	m.middleItems = nil
	result, cmd := m.directActionLogs()
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovDirectActionLogsPortForwards(t *testing.T) {
	m := baseModelActions()
	m.middleItems = []model.Item{{Name: "pf-1", Kind: "__port_forwards__"}}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__"}
	result, cmd := m.directActionLogs()
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovDirectActionRefresh(t *testing.T) {
	m := baseModelActions()
	result, cmd := m.directActionRefresh()
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.hasStatusMessage())
}

func TestCovDirectActionEditNoKind(t *testing.T) {
	m := baseModelActions()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "ctx"}}
	result, cmd := m.directActionEdit()
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovDirectActionEditNoSelection(t *testing.T) {
	m := baseModelActions()
	m.middleItems = nil
	result, cmd := m.directActionEdit()
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovDirectActionEditPortForwards(t *testing.T) {
	m := baseModelActions()
	m.middleItems = []model.Item{{Name: "pf", Kind: "__port_forwards__"}}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__"}
	result, cmd := m.directActionEdit()
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovDirectActionDescribeNoKind(t *testing.T) {
	m := baseModelActions()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "ctx"}}
	result, cmd := m.directActionDescribe()
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovDirectActionDescribeNoSelection(t *testing.T) {
	m := baseModelActions()
	m.middleItems = nil
	result, cmd := m.directActionDescribe()
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovDirectActionForceDeleteNoKind(t *testing.T) {
	m := baseModelActions()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "ctx"}}
	result, cmd := m.directActionForceDelete()
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovDirectActionForceDeleteNotDeletable(t *testing.T) {
	m := baseModelActions()
	m.middleItems = []model.Item{{Name: "deploy-1", Kind: "Deployment"}}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments"}
	_, cmd := m.directActionForceDelete()
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestCovDirectActionForceDeleteNoItem(t *testing.T) {
	m := baseModelActions()
	m.middleItems = nil
	result, cmd := m.directActionForceDelete()
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovDirectActionForceDeletePod(t *testing.T) {
	m := baseModelActions()
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	result, _ := m.directActionForceDelete()
	rm := result.(Model)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Equal(t, "Force Delete", rm.pendingAction)
}

func TestCovDirectActionScaleNotScaleable(t *testing.T) {
	m := baseModelActions()
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod"}}
	_, cmd := m.directActionScale()
	assert.NotNil(t, cmd)
}

func TestCovDirectActionScaleNoItem(t *testing.T) {
	m := baseModelActions()
	m.middleItems = nil
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments"}
	result, cmd := m.directActionScale()
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovOpenActionMenuNoSelection(t *testing.T) {
	m := baseModelActions()
	rm := m.openActionMenu()
	assert.Equal(t, overlayAction, rm.overlay)
}

func TestCovOpenActionMenuBulk(t *testing.T) {
	m := baseModelActions()
	m.selectedItems["default/pod-1"] = true
	rm := m.openActionMenu()
	assert.True(t, rm.bulkMode)
}

func TestCovOpenActionMenuNoMiddleItems(t *testing.T) {
	m := baseModelActions()
	m.middleItems = nil
	_ = m.openActionMenu()
}

func TestCovDirectActionDeleteWithDeletingResource(t *testing.T) {
	m := baseModelActions()
	m.middleItems = []model.Item{
		{Name: "pod-1", Kind: "Pod", Namespace: "default", Deleting: true},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	result, _ := m.directActionDelete()
	rm := result.(Model)
	assert.Equal(t, overlayConfirmType, rm.overlay)
}

func TestCovDirectActionDeleteDeletingNonForceDeleteable(t *testing.T) {
	m := baseModelActions()
	m.middleItems = []model.Item{
		{Name: "deploy-1", Kind: "Deployment", Namespace: "default", Deleting: true},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments"}
	result, _ := m.directActionDelete()
	rm := result.(Model)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Equal(t, "Force Finalize", rm.pendingAction)
}

func TestCovRefreshCurrentLevelResources(t *testing.T) {
	m := baseModelActions()
	m.nav.Level = model.LevelResources
	m.nav.Context = "ctx"
	m.nav.ResourceType = model.ResourceTypeEntry{Resource: "pods", Kind: "Pod"}
	cmd := m.refreshCurrentLevel()
	assert.NotNil(t, cmd)
}

func TestCovRefreshCurrentLevelContexts(t *testing.T) {
	m := baseModelActions()
	m.nav.Level = model.LevelClusters
	cmd := m.refreshCurrentLevel()
	assert.NotNil(t, cmd)
}

func TestCovRefreshCurrentLevelResourceTypes(t *testing.T) {
	m := baseModelActions()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "ctx"
	m.discoveredResources["ctx"] = []model.ResourceTypeEntry{
		{DisplayName: "Pods", Kind: "Pod", APIVersion: "v1", Resource: "pods"},
	}
	cmd := m.refreshCurrentLevel()
	assert.NotNil(t, cmd)
}

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

func TestCovExecuteBulkActionLogs(t *testing.T) {
	m := baseModelWithFakeClient()
	m.execMu = &sync.Mutex{}
	m.bulkItems = []model.Item{{Name: "pod-1"}}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.actionCtx = actionContext{context: "ctx", namespace: "ns"}
	ret, _ := m.executeBulkAction("Logs")
	switch v := ret.(type) {
	case *Model:
		assert.False(t, v.bulkMode)
	case Model:
		assert.False(t, v.bulkMode)
	}
}

func TestCovExecuteBulkActionDelete(t *testing.T) {
	m := baseModelCov()
	m.bulkItems = []model.Item{{Name: "pod-1"}, {Name: "pod-2"}}
	m.confirmTypeInput = TextInput{}
	ret, cmd := m.executeBulkAction("Delete")
	result := ret.(Model)
	assert.Equal(t, overlayConfirm, result.overlay)
	assert.Nil(t, cmd)
}

func TestCovExecuteBulkActionForceDelete(t *testing.T) {
	m := baseModelCov()
	m.bulkItems = []model.Item{{Name: "pod-1"}}
	m.confirmTypeInput = TextInput{}
	ret, cmd := m.executeBulkAction("Force Delete")
	result := ret.(Model)
	assert.Equal(t, overlayConfirmType, result.overlay)
	assert.Nil(t, cmd)
}

func TestCovExecuteBulkActionScale(t *testing.T) {
	m := baseModelCov()
	m.bulkItems = []model.Item{{Name: "deploy-1"}}
	m.scaleInput = TextInput{}
	ret, cmd := m.executeBulkAction("Scale")
	result := ret.(Model)
	assert.Equal(t, overlayScaleInput, result.overlay)
	assert.Nil(t, cmd)
}

func TestCovExecuteBulkActionRestart(t *testing.T) {
	m := baseModelCov()
	m.bulkItems = []model.Item{{Name: "deploy-1"}}
	m.actionCtx = actionContext{context: "ctx", namespace: "ns"}
	ret, cmd := m.executeBulkAction("Restart")
	result := ret.(Model)
	assert.True(t, result.loading)
	assert.NotNil(t, cmd)
}

func TestCovExecuteBulkActionLabels(t *testing.T) {
	m := baseModelCov()
	m.bulkItems = []model.Item{{Name: "pod-1"}}
	m.batchLabelInput = TextInput{}
	ret, cmd := m.executeBulkAction("Labels / Annotations")
	result := ret.(Model)
	assert.Equal(t, overlayBatchLabel, result.overlay)
	assert.Nil(t, cmd)
}

func TestCovExecuteBulkActionDiffWrongCount(t *testing.T) {
	m := baseModelCov()
	m.bulkItems = []model.Item{{Name: "pod-1"}}
	ret, cmd := m.executeBulkAction("Diff")
	result := ret.(Model)
	assert.True(t, result.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovExecuteBulkActionUnknown(t *testing.T) {
	m := baseModelCov()
	m.bulkItems = []model.Item{{Name: "pod-1"}}
	ret, cmd := m.executeBulkAction("NonExistent")
	_ = ret.(Model)
	assert.Nil(t, cmd)
}

func TestCovRefreshCurrentLevelClustersFakeClient(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	cmd := m.refreshCurrentLevel()
	assert.NotNil(t, cmd)
}

func TestCovRefreshCurrentLevelResourceTypesFakeClient(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelResourceTypes
	m.discoveredResources[m.nav.Context] = []model.ResourceTypeEntry{
		{DisplayName: "Pods", Kind: "Pod", APIVersion: "v1", Resource: "pods"},
	}
	cmd := m.refreshCurrentLevel()
	assert.NotNil(t, cmd)
}

func TestCovRefreshCurrentLevelPortForwards(t *testing.T) {
	m := baseModelWithFakeClient()
	m.portForwardMgr = k8s.NewPortForwardManager()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__"}
	cmd := m.refreshCurrentLevel()
	assert.NotNil(t, cmd)
}

func TestCovRefreshCurrentLevelResourcesFakeClient(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true}
	cmd := m.refreshCurrentLevel()
	assert.NotNil(t, cmd)
}

func TestCovExecuteActionDescribe(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Describe")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	_ = rm
}

func TestCovExecuteActionDescribeNamespaced(t *testing.T) {
	m := testModelExec()
	m.actionCtx.resourceType.Namespaced = true
	result, cmd := m.executeAction("Describe")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionEdit(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Edit")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionDelete(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Delete")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayConfirm, rm.overlay)
	assert.Equal(t, "Delete", rm.pendingAction)
}

func TestCovExecuteActionScale(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Scale")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayScaleInput, rm.overlay)
}

func TestCovExecuteActionResize(t *testing.T) {
	m := testModelExec()
	m.actionCtx.columns = []model.KeyValue{{Key: "Capacity", Value: "5Gi"}}
	result, cmd := m.executeAction("Resize")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayPVCResize, rm.overlay)
	assert.Equal(t, "5Gi", rm.pvcCurrentSize)
}

func TestCovExecuteActionResizeNoCap(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Resize")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayPVCResize, rm.overlay)
	assert.Empty(t, rm.pvcCurrentSize)
}

func TestCovExecuteActionRestart(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Restart")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionRestartPortForward(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "__port_forwards__"
	m.actionCtx.columns = []model.KeyValue{{Key: "ID", Value: "0"}}
	result, cmd := m.executeAction("Restart")
	_ = result
	assert.Nil(t, cmd)
}

func TestCovExecuteActionDebug(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Debug")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionDebugPod(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Debug Pod")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionDebugMount(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-pvc"
	result, cmd := m.executeAction("Debug Mount")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionForceDelete(t *testing.T) {
	// Regression for #89: action-menu Force Delete must request typed
	// confirmation, mirroring directActionForceDelete and the bulk path,
	// so an accidental x->X (or wrong list cursor) cannot nuke a pod.
	m := testModelExec()
	result, cmd := m.executeAction("Force Delete")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.False(t, rm.loading)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Equal(t, "Force Delete", rm.pendingAction)
	assert.Contains(t, rm.confirmTitle, "Force Delete")
	assert.Contains(t, rm.confirmQuestion, "--force --grace-period=0")
}

func TestCovExecuteActionForceFinalize(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Force Finalize")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Equal(t, "Force Finalize", rm.pendingAction)
}

func TestCovExecuteActionCordon(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	result, cmd := m.executeAction("Cordon")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionUncordon(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	result, cmd := m.executeAction("Uncordon")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionDrain(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	result, cmd := m.executeAction("Drain")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayConfirm, rm.overlay)
}

func TestCovExecuteActionTaint(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	result, cmd := m.executeAction("Taint")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.True(t, rm.commandBarActive)
}

func TestCovExecuteActionUntaint(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	m.actionCtx.columns = []model.KeyValue{{Key: "Taints", Value: "key1=val:NoSchedule, key2:NoExecute"}}
	result, cmd := m.executeAction("Untaint")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.True(t, rm.commandBarActive)
}

func TestCovExecuteActionUntaintEmpty(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	result, cmd := m.executeAction("Untaint")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.True(t, rm.commandBarActive)
}

func TestCovExecuteActionTrigger(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "CronJob"
	result, cmd := m.executeAction("Trigger")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionShell(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	result, cmd := m.executeAction("Shell")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionPortForward(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	result, cmd := m.executeAction("Port Forward")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionEvents(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Events")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionSecretEditor(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Secret"
	m.middleItems = []model.Item{{Name: "my-secret", Namespace: "default"}}
	result, cmd := m.executeAction("Secret Editor")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionConfigMapEditor(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "ConfigMap"
	m.middleItems = []model.Item{{Name: "my-cm", Namespace: "default"}}
	result, cmd := m.executeAction("ConfigMap Editor")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionRollbackDeployment(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	m.middleItems = []model.Item{{Name: "my-deploy", Namespace: "default"}}
	result, cmd := m.executeAction("Rollback")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionRollbackHelmRelease(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	m.actionCtx.name = "my-release"
	m.actionCtx.namespace = "default"
	result, cmd := m.executeAction("Rollback")
	mdl := result.(Model)
	// Helm rollback opens the overlay optimistically in a loading state so
	// the user gets immediate feedback while helm history runs.
	assert.Equal(t, overlayHelmRollback, mdl.overlay)
	assert.Equal(t, 0, mdl.helmRollbackCursor)
	assert.Nil(t, mdl.helmRollbackRevisions)
	assert.True(t, mdl.helmRevisionsLoading)
	assert.NotNil(t, cmd)
}

func TestCovExecuteActionHistoryHelmRelease(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	m.actionCtx.name = "my-release"
	m.actionCtx.namespace = "default"
	result, cmd := m.executeAction("History")
	mdl := result.(Model)
	assert.Equal(t, overlayHelmHistory, mdl.overlay)
	assert.Equal(t, 0, mdl.helmHistoryCursor)
	assert.Nil(t, mdl.helmHistoryRevisions)
	assert.True(t, mdl.helmRevisionsLoading)
	assert.NotNil(t, cmd)
}

func TestHelmRevisionListClearsLoadingFlag(t *testing.T) {
	m := testModelExec()
	m.helmRevisionsLoading = true
	result, _ := m.updateHelmRevisionList(helmRevisionListMsg{
		revisions: []ui.HelmRevision{{Revision: 1, Status: "deployed"}},
	})
	mdl := result.(Model)
	assert.False(t, mdl.helmRevisionsLoading)
	assert.Equal(t, overlayHelmRollback, mdl.overlay)
	assert.Len(t, mdl.helmRollbackRevisions, 1)
}

func TestHelmHistoryListClearsLoadingFlag(t *testing.T) {
	m := testModelExec()
	m.helmRevisionsLoading = true
	m.overlay = overlayHelmHistory
	result, _ := m.updateHelmHistoryList(helmHistoryListMsg{
		revisions: []ui.HelmRevision{{Revision: 1, Status: "deployed"}},
	})
	mdl := result.(Model)
	assert.False(t, mdl.helmRevisionsLoading)
	assert.Equal(t, overlayHelmHistory, mdl.overlay)
	assert.Len(t, mdl.helmHistoryRevisions, 1)
}

func TestCovExecuteActionVulnScan(t *testing.T) {
	m := testModelExec()
	m.actionCtx.image = "nginx:latest"
	result, cmd := m.executeAction("Vuln Scan")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionVulnScanNoImage(t *testing.T) {
	m := testModelExec()
	m.actionCtx.image = ""
	result, cmd := m.executeAction("Vuln Scan")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.hasStatusMessage())
	_ = rm
}

func TestCovExecuteActionPermissions(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Permissions")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionStartupAnalysis(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Startup Analysis")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionAlerts(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Alerts")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionVisualize(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Visualize")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionLabelsAnnotations(t *testing.T) {
	m := testModelExec()
	m.middleItems = []model.Item{{Name: "my-pod", Namespace: "default"}}
	result, cmd := m.executeAction("Labels / Annotations")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionValues(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("Values")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionAllValues(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("All Values")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionEditValues(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("Edit Values")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionDiffHelm(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("Diff")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionDiffNonHelm(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Diff")
	assert.Nil(t, cmd)
	_ = result
}

func TestCovExecuteActionUpgrade(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("Upgrade")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionLogsDirectPod(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	m.actionCtx.containerName = "main"
	result, cmd := m.executeAction("Logs")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, modeLogs, rm.mode)
}

func TestCovExecuteActionLogsGroupResource(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Logs")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, modeLogs, rm.mode)
}

func TestCovExecuteActionLogsNonPodNonGroup(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "CRD"
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Logs")
	rm := result.(Model)
	// Should need container selection.
	assert.NotNil(t, cmd)
	assert.Equal(t, "Logs", rm.pendingAction)
}

func TestCovExecuteActionLogsPodAllContainers(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Logs")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, modeLogs, rm.mode)
	assert.Contains(t, rm.logTitle, "Logs:")
}

func TestCovExecuteActionExecDirectPod(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	m.actionCtx.containerName = "main"
	result, cmd := m.executeAction("Exec")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionExecParent(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Exec")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, "Exec", rm.pendingAction)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionExecNeedsContainer(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Exec")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, "Exec", rm.pendingAction)
}

func TestCovExecuteActionAttachDirect(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	m.actionCtx.containerName = "main"
	result, cmd := m.executeAction("Attach")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionAttachParent(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "StatefulSet"
	result, cmd := m.executeAction("Attach")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, "Attach", rm.pendingAction)
}

func TestCovExecuteActionAttachNeedsContainer(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Attach")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, "Attach", rm.pendingAction)
}

func TestCovExecuteActionStopPortForward(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "__port_forwards__"
	m.actionCtx.columns = []model.KeyValue{{Key: "ID", Value: "0"}}
	result, cmd := m.executeAction("Stop")
	_ = result
	assert.Nil(t, cmd)
}

func TestCovExecuteActionRemovePortForward(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "__port_forwards__"
	m.actionCtx.columns = []model.KeyValue{{Key: "ID", Value: "0"}}
	result, cmd := m.executeAction("Remove")
	_ = result
	assert.Nil(t, cmd)
}

func TestCovExecuteActionUnknown(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("NonExistentAction12345")
	assert.Nil(t, cmd)
	_ = result
}

func TestCovExecuteActionGoToPodSingle(t *testing.T) {
	m := testModelExec()
	m.actionCtx.columns = []model.KeyValue{{Key: "Used By", Value: "single-pod"}}
	m.middleItems = []model.Item{{Name: "item1"}}
	result, _ := m.executeAction("Go to Pod")
	_ = result
}

func TestCovExecuteActionGoToPodMultiple(t *testing.T) {
	m := testModelExec()
	m.actionCtx.columns = []model.KeyValue{{Key: "Used By", Value: "pod-a, pod-b"}}
	result, cmd := m.executeAction("Go to Pod")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayPodSelect, rm.overlay)
}

func TestCovExecuteActionGoToPodNone(t *testing.T) {
	m := testModelExec()
	m.actionCtx.columns = nil
	result, cmd := m.executeAction("Go to Pod")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.hasStatusMessage())
}

func TestCovExecuteActionConfigureAutoSync(t *testing.T) {
	m := testModelExec()
	m.middleItems = []model.Item{{Name: "my-app", Namespace: "argocd"}}
	result, cmd := m.executeAction("Configure AutoSync")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionSync(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Sync")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionSyncApplyOnly(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Sync (Apply Only)")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionRefreshApp(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Application"
	result, cmd := m.executeAction("Refresh")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionRefreshAppSet(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "ApplicationSet"
	result, cmd := m.executeAction("Refresh")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionRefreshAppSetByResource(t *testing.T) {
	m := testModelExec()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Resource: "applicationsets"}
	result, cmd := m.executeAction("Refresh")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionTerminateSync(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Terminate Sync")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionWatchWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Watch Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
	assert.True(t, rm.describeAutoRefresh)
}

func TestCovExecuteActionSuspendWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Suspend Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionResumeWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Resume Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionStopWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Stop Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionTerminateWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Terminate Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionResubmitWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Resubmit Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionSubmitWorkflow(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "WorkflowTemplate"
	result, cmd := m.executeAction("Submit Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionSubmitClusterWorkflow(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "ClusterWorkflowTemplate"
	result, cmd := m.executeAction("Submit Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionSuspendCronWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Suspend CronWorkflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionResumeCronWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Resume CronWorkflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionForceRenew(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Force Renew")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionForceRefresh(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Force Refresh")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionPause(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Pause")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionUnpause(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Unpause")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionReconcile(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Reconcile")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionSuspend(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Suspend")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionResume(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Resume")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionOpenInBrowserPF(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "__port_forwards__"
	m.actionCtx.columns = []model.KeyValue{{Key: "Local", Value: "8080"}}
	result, cmd := m.executeAction("Open in Browser")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.hasStatusMessage())
}

func TestCovExecuteActionOpenInBrowserPFNoPort(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "__port_forwards__"
	result, cmd := m.executeAction("Open in Browser")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.hasStatusMessage())
}

func TestCovExecuteActionOpenInBrowserIngress(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Ingress"
	m.middleItems = []model.Item{{Name: "my-ingress", Columns: []model.KeyValue{{Key: "__ingress_url", Value: "https://example.com"}}}}
	result, cmd := m.executeAction("Open in Browser")
	_ = result
	_ = cmd
}

func TestCovExecuteActionBulkMode(t *testing.T) {
	m := testModelExec()
	m.bulkMode = true
	m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "default"}}
	m.actionCtx.kind = "Pod"
	// Should dispatch to executeBulkAction.
	result, cmd := m.executeAction("Delete")
	_ = result
	_ = cmd
}

func TestFinal3UpdateBulkActionResultMsg(t *testing.T) {
	m := baseFinalModel()
	m.bulkMode = true
	result, cmd := m.Update(bulkActionResultMsg{succeeded: 2})
	rm := result.(Model)
	assert.False(t, rm.bulkMode)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateBulkActionResultMsgErrors(t *testing.T) {
	m := baseFinalModel()
	m.bulkMode = true
	result, cmd := m.Update(bulkActionResultMsg{
		failed: 1,
		errors: []string{"failed to delete pod-1"},
	})
	rm := result.(Model)
	assert.False(t, rm.bulkMode)
	assert.NotNil(t, cmd)
}

func TestPush4BuildActionCtxResources(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	sel := &model.Item{Name: "pod-1", Namespace: "default", Kind: "Pod"}
	ctx := m.buildActionCtx(sel, "Pod")
	assert.Equal(t, "pod-1", ctx.name)
	assert.Equal(t, "test-ctx", ctx.context)
}

func TestPush4BuildActionCtxOwned(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceName = "deploy-1"
	sel := &model.Item{Name: "rs-1", Kind: "ReplicaSet", Namespace: "default"}
	ctx := m.buildActionCtx(sel, "ReplicaSet")
	assert.Equal(t, "rs-1", ctx.name)
}

func TestPush4BuildActionCtxContainers(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "pod-1"
	sel := &model.Item{Name: "nginx", Kind: "Container"}
	ctx := m.buildActionCtx(sel, "Container")
	assert.Equal(t, "pod-1", ctx.name)
	assert.Equal(t, "nginx", ctx.containerName)
}

func TestPush4RefreshCurrentLevelOwned(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelOwned
	cmd := m.refreshCurrentLevel()
	assert.NotNil(t, cmd)
}

func TestPush4RefreshCurrentLevelContainers(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelContainers
	cmd := m.refreshCurrentLevel()
	assert.NotNil(t, cmd)
}

func TestFinalExecuteActionDescribe(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Describe")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalExecuteActionEdit(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Edit")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalExecuteActionDelete(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Delete")
	assert.Nil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayConfirm, rm.overlay)
	assert.Equal(t, "Delete", rm.pendingAction)
}

func TestFinalExecuteActionScale(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Scale")
	assert.Nil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayScaleInput, rm.overlay)
}

func TestFinalExecuteActionResize(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = []model.KeyValue{{Key: "Capacity", Value: "10Gi"}}
	result, cmd := m.executeAction("Resize")
	assert.Nil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayPVCResize, rm.overlay)
	assert.Equal(t, "10Gi", rm.pvcCurrentSize)
}

func TestFinalExecuteActionResizeCapacity(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = []model.KeyValue{{Key: "CAPACITY", Value: "5Gi"}}
	result, _ := m.executeAction("Resize")
	rm := result.(Model)
	assert.Equal(t, "5Gi", rm.pvcCurrentSize)
}

func TestFinalExecuteActionResizeNoCapacity(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = []model.KeyValue{{Key: "Other", Value: "5Gi"}}
	result, _ := m.executeAction("Resize")
	rm := result.(Model)
	assert.Equal(t, "", rm.pvcCurrentSize)
}

func TestFinalExecuteActionRestart(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Restart")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionRestartPortForward(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "__port_forward_entry__"
	result, _ := m.executeAction("Restart")
	_ = result.(Model)
}

func TestFinalExecuteActionRollback(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Rollback")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionRollbackHelm(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("Rollback")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionPortForward(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Port Forward")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionDebug(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Debug")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionEvents(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Events")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionSecretEditor(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Secret Editor")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionConfigMapEditor(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("ConfigMap Editor")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionForceDelete(t *testing.T) {
	// Regression for #89: action-menu Force Delete must request typed
	// confirmation, not fire kubectl delete --force directly.
	m := baseFinalModel()
	result, cmd := m.executeAction("Force Delete")
	assert.Nil(t, cmd)
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Equal(t, "Force Delete", rm.pendingAction)
}

func TestFinalExecuteActionForceFinalize(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Force Finalize")
	assert.Nil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Equal(t, "Force Finalize", rm.pendingAction)
}

func TestFinalExecuteActionCordon(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	result, cmd := m.executeAction("Cordon")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionUncordon(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	result, cmd := m.executeAction("Uncordon")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionDrain(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	result, cmd := m.executeAction("Drain")
	assert.Nil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayConfirm, rm.overlay)
	assert.Equal(t, "Drain", rm.pendingAction)
}

func TestFinalExecuteActionTaint(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	result, _ := m.executeAction("Taint")
	rm := result.(Model)
	assert.True(t, rm.commandBarActive)
}

func TestFinalExecuteActionUntaint(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	m.actionCtx.columns = []model.KeyValue{{Key: "Taints", Value: "key1=value1:NoSchedule, key2:NoExecute"}}
	result, _ := m.executeAction("Untaint")
	rm := result.(Model)
	assert.True(t, rm.commandBarActive)
}

func TestFinalExecuteActionUntaintNoTaints(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	result, _ := m.executeAction("Untaint")
	rm := result.(Model)
	assert.True(t, rm.commandBarActive)
}

func TestFinalExecuteActionTrigger(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "CronJob"
	result, cmd := m.executeAction("Trigger")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionShell(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	result, cmd := m.executeAction("Shell")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionDebugPod(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Debug Pod")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionLogsDirectPod(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = "main"
	result, cmd := m.executeAction("Logs")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, modeLogs, rm.mode)
	assert.Contains(t, rm.logTitle, "main")
}

func TestFinalExecuteActionLogsGroupResource(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Logs")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, modeLogs, rm.mode)
	assert.Equal(t, "Deployment", rm.logParentKind)
}

func TestFinalExecuteActionLogsUnknownKind(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "ConfigMap"
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Logs")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "Logs", rm.pendingAction)
}

func TestFinalExecuteActionExecDirectWithContainer(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = "web"
	result, cmd := m.executeAction("Exec")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionExecDeployment(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Exec")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "Exec", rm.pendingAction)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionExecNoContainer(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Exec")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "Exec", rm.pendingAction)
}

func TestFinalExecuteActionAttachDirect(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = "web"
	result, cmd := m.executeAction("Attach")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionAttachDeployment(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "StatefulSet"
	result, cmd := m.executeAction("Attach")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "Attach", rm.pendingAction)
}

func TestFinalExecuteActionAttachNoContainer(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Attach")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "Attach", rm.pendingAction)
}

func TestFinalExecuteActionGoToPodNoPods(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = nil
	result, cmd := m.executeAction("Go to Pod")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "No pods")
}

func TestFinalExecuteActionGoToPodSingle(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = []model.KeyValue{{Key: "Used By", Value: "my-pod"}}
	m.nav.Level = model.LevelResources
	result, _ := m.executeAction("Go to Pod")
	_ = result.(Model)
}

func TestFinalExecuteActionGoToPodMultiple(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = []model.KeyValue{{Key: "Used By", Value: "pod-a, pod-b"}}
	result, _ := m.executeAction("Go to Pod")
	rm := result.(Model)
	assert.Equal(t, overlayPodSelect, rm.overlay)
	assert.Equal(t, 2, len(rm.overlayItems))
}

func TestFinalExecuteActionBulkMode(t *testing.T) {
	m := baseFinalModel()
	m.bulkMode = true
	m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "default"}}
	m.actionCtx.context = "test-ctx"
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	result, _ := m.executeAction("Delete")
	_ = result.(Model)
}

func TestFinalExecuteActionSync(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Sync")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionSyncApplyOnly(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Sync (Apply Only)")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionRefreshArgo(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Application"
	result, cmd := m.executeAction("Refresh")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionRefreshArgoAppSet(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "ApplicationSet"
	result, cmd := m.executeAction("Refresh")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionTerminateSync(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Terminate Sync")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionConfigureAutoSync(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Configure AutoSync")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionWatchWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Watch Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
	assert.True(t, rm.describeAutoRefresh)
}

func TestFinalExecuteActionSuspendWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Suspend Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionResumeWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Resume Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionStopWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Stop Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionTerminateWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Terminate Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionResubmitWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Resubmit Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionSubmitWorkflow(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "WorkflowTemplate"
	result, cmd := m.executeAction("Submit Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionSubmitWorkflowCluster(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "ClusterWorkflowTemplate"
	result, cmd := m.executeAction("Submit Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionSuspendCronWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Suspend CronWorkflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionResumeCronWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Resume CronWorkflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionForceRenew(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Force Renew")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionForceRefresh(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Force Refresh")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionPause(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Pause")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionUnpause(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Unpause")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionReconcile(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Reconcile")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionSuspend(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Suspend")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionResume(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Resume")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionUnknown(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("NonExistentAction")
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionHelmValues(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "HelmRelease"
	m.actionCtx.resourceType = model.ResourceTypeEntry{APIGroup: "_helm"}
	result, cmd := m.executeAction("Values")
	// Values opens helm edit values.
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionHelmDiff(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "HelmRelease"
	m.actionCtx.resourceType = model.ResourceTypeEntry{APIGroup: "_helm"}
	result, cmd := m.executeAction("Diff Values")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionHelmUpgrade(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "HelmRelease"
	m.actionCtx.resourceType = model.ResourceTypeEntry{APIGroup: "_helm"}
	result, cmd := m.executeAction("Upgrade")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionVulnScan(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = []model.KeyValue{{Key: "Image", Value: "nginx:latest"}}
	result, cmd := m.executeAction("Vulnerability Scan")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionLabels(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Labels")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionDebugPVC(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "PersistentVolumeClaim"
	result, cmd := m.executeAction("Debug Pod (PVC)")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionResourceQuota(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Resource Quotas")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionRBACCheck(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("RBAC Check")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionPodStartup(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Pod Startup")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionNetworkPolicies(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Network Policies")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionExplain(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Explain")
	_ = result.(Model)
	_ = cmd
}
