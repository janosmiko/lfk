package app

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// =====================================================================
// restoreSession / restoreSingleTabSession / restoreMultiTabSession
// =====================================================================

func TestFinal2RestoreSessionSingleTab(t *testing.T) {
	m := baseFinalModel()
	m.pendingSession = &SessionState{
		Context:   "test-ctx",
		Namespace: "default",
	}
	m.sessionRestored = false
	contexts := []model.Item{{Name: "test-ctx"}, {Name: "other-ctx"}}
	result, cmd := m.restoreSession(contexts)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.sessionRestored)
}

func TestFinal2RestoreSingleTabSessionContextNotFound(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{Context: "nonexistent"}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, _ := m.restoreSingleTabSession(sess, contexts)
	_ = result.(Model)
}

func TestFinal2RestoreSingleTabSessionWithResourceType(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		Context:      "test-ctx",
		Namespace:    "default",
		ResourceType: "/v1/pods",
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, cmd := m.restoreSingleTabSession(sess, contexts)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, model.LevelResources, rm.nav.Level)
}

func TestFinal2RestoreSingleTabSessionWithResourceName(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		Context:      "test-ctx",
		Namespace:    "default",
		ResourceType: "/v1/pods",
		ResourceName: "my-pod",
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, cmd := m.restoreSingleTabSession(sess, contexts)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "my-pod", rm.pendingTarget)
}

func TestFinal2RestoreSingleTabSessionNoResourceType(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		Context:   "test-ctx",
		Namespace: "default",
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, cmd := m.restoreSingleTabSession(sess, contexts)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
}

func TestFinal2RestoreSingleTabSessionAllNamespaces(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		Context:       "test-ctx",
		AllNamespaces: true,
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, _ := m.restoreSingleTabSession(sess, contexts)
	rm := result.(Model)
	assert.True(t, rm.allNamespaces)
}

func TestFinal2RestoreSingleTabSessionSelectedNS(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		Context:            "test-ctx",
		Namespace:          "ns1",
		SelectedNamespaces: []string{"ns1", "ns2"},
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, _ := m.restoreSingleTabSession(sess, contexts)
	rm := result.(Model)
	assert.True(t, rm.selectedNamespaces["ns1"])
	assert.True(t, rm.selectedNamespaces["ns2"])
}

func TestFinal2RestoreMultiTabSession(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		ActiveTab: 0,
		Tabs: []SessionTab{
			{Context: "test-ctx", Namespace: "default", ResourceType: "/v1/pods"},
			{Context: "test-ctx", Namespace: "kube-system"},
		},
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, cmd := m.restoreMultiTabSession(sess, contexts)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, 2, len(rm.tabs))
	assert.Equal(t, 0, rm.activeTab)
}

func TestFinal2RestoreMultiTabSessionInvalidActiveTab(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		ActiveTab: 5,
		Tabs: []SessionTab{
			{Context: "test-ctx", Namespace: "default"},
		},
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, _ := m.restoreMultiTabSession(sess, contexts)
	rm := result.(Model)
	assert.Equal(t, 0, rm.activeTab)
}

func TestFinal2RestoreMultiTabSessionContextNotFound(t *testing.T) {
	m := baseFinalModel()
	sess := &SessionState{
		ActiveTab: 0,
		Tabs: []SessionTab{
			{Context: "nonexistent"},
		},
	}
	contexts := []model.Item{{Name: "test-ctx"}}
	result, _ := m.restoreMultiTabSession(sess, contexts)
	_ = result.(Model)
}

func TestFinal2RestoreSessionMultiTab(t *testing.T) {
	m := baseFinalModel()
	m.pendingSession = &SessionState{
		ActiveTab: 0,
		Tabs: []SessionTab{
			{Context: "test-ctx", Namespace: "default"},
		},
	}
	m.sessionRestored = false
	contexts := []model.Item{{Name: "test-ctx"}}
	result, _ := m.restoreSession(contexts)
	rm := result.(Model)
	assert.True(t, rm.sessionRestored)
}

// =====================================================================
// Update: more message types
// =====================================================================

func TestFinal2UpdateResourcesLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	m.requestGen = 1
	items := []model.Item{{Name: "pod-1", Namespace: "default"}}
	result, cmd := m.Update(resourcesLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, items, rm.middleItems)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateResourcesLoadedMsgStale(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 2
	result, cmd := m.Update(resourcesLoadedMsg{items: nil, gen: 1})
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinal2UpdateResourcesLoadedMsgForPreview(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	items := []model.Item{{Name: "child-1"}}
	result, cmd := m.Update(resourcesLoadedMsg{items: items, gen: 1, forPreview: true})
	rm := result.(Model)
	assert.Equal(t, items, rm.rightItems)
	assert.Nil(t, cmd)
}

func TestFinal2UpdateResourcesLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, cmd := m.Update(resourcesLoadedMsg{err: assert.AnError, gen: 1})
	rm := result.(Model)
	assert.NotNil(t, rm.err)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateResourcesLoadedMsgCanceled(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, cmd := m.Update(resourcesLoadedMsg{err: context.Canceled, gen: 1})
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinal2UpdateResourcesLoadedWithPendingTarget(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	m.pendingTarget = "pod-2"
	items := []model.Item{{Name: "pod-1"}, {Name: "pod-2"}, {Name: "pod-3"}}
	result, _ := m.Update(resourcesLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.Empty(t, rm.pendingTarget)
}

func TestFinal2UpdateOwnedLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	items := []model.Item{{Name: "rs-1"}}
	result, cmd := m.Update(ownedLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, items, rm.middleItems)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateOwnedLoadedMsgForPreview(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	items := []model.Item{{Name: "rs-1"}}
	result, cmd := m.Update(ownedLoadedMsg{items: items, gen: 1, forPreview: true})
	rm := result.(Model)
	assert.Equal(t, items, rm.rightItems)
	assert.Nil(t, cmd)
}

func TestFinal2UpdateOwnedLoadedMsgStale(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 2
	result, cmd := m.Update(ownedLoadedMsg{gen: 1})
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinal2UpdateOwnedLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, cmd := m.Update(ownedLoadedMsg{err: assert.AnError, gen: 1})
	rm := result.(Model)
	assert.NotNil(t, rm.err)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateContainersLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	items := []model.Item{{Name: "nginx"}}
	result, cmd := m.Update(containersLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, items, rm.middleItems)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateContainersLoadedMsgForPreview(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	items := []model.Item{{Name: "nginx"}}
	result, cmd := m.Update(containersLoadedMsg{items: items, gen: 1, forPreview: true})
	rm := result.(Model)
	assert.Equal(t, items, rm.rightItems)
	assert.Nil(t, cmd)
}

func TestFinal2UpdateContainersLoadedMsgStale(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 2
	result, cmd := m.Update(containersLoadedMsg{gen: 1})
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinal2UpdateNamespacesLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	m.allNamespaces = true
	items := []model.Item{{Name: "default"}, {Name: "kube-system"}}
	result, cmd := m.Update(namespacesLoadedMsg{items: items})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, 3, len(rm.overlayItems)) // "All Namespaces" + 2 items
	assert.Nil(t, cmd)
}

func TestFinal2UpdateNamespacesLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(namespacesLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.NotNil(t, rm.err)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateYAMLLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, _ := m.Update(yamlLoadedMsg{content: "apiVersion: v1\nkind: Pod"})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.NotEmpty(t, rm.yamlContent)
}

func TestFinal2UpdateYAMLLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(yamlLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.NotNil(t, rm.err)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdatePreviewYAMLLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, _ := m.Update(previewYAMLLoadedMsg{content: "key: value", gen: 1})
	rm := result.(Model)
	assert.NotEmpty(t, rm.previewYAML)
}

func TestFinal2UpdatePreviewYAMLLoadedMsgStale(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 2
	result, cmd := m.Update(previewYAMLLoadedMsg{content: "key: value", gen: 1})
	rm := result.(Model)
	assert.Empty(t, rm.previewYAML)
	assert.Nil(t, cmd)
}

func TestFinal2UpdatePreviewYAMLLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, _ := m.Update(previewYAMLLoadedMsg{err: assert.AnError, gen: 1})
	rm := result.(Model)
	assert.Empty(t, rm.previewYAML)
}

func TestFinal2UpdateContainerPortsLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(containerPortsLoadedMsg{
		ports: []k8s.ContainerPort{{ContainerPort: 8080, Name: "http", Protocol: "TCP"}},
	})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, overlayPortForward, rm.overlay)
	assert.Equal(t, 1, len(rm.pfAvailablePorts))
	assert.Nil(t, cmd)
}

func TestFinal2UpdateContainerPortsLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(containerPortsLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, overlayPortForward, rm.overlay)
	assert.Nil(t, rm.pfAvailablePorts)
	assert.Nil(t, cmd)
}

func TestFinal2UpdateContainerPortsLoadedMsgEmpty(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(containerPortsLoadedMsg{ports: nil})
	rm := result.(Model)
	assert.Equal(t, overlayPortForward, rm.overlay)
	assert.Equal(t, -1, rm.pfPortCursor)
	assert.Nil(t, cmd)
}

func TestFinal2UpdateResourceTreeLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	tree := &model.ResourceNode{Name: "root"}
	result, _ := m.Update(resourceTreeLoadedMsg{tree: tree, gen: 1})
	rm := result.(Model)
	assert.Equal(t, tree, rm.resourceTree)
}

func TestFinal2UpdateResourceTreeLoadedMsgStale(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 2
	result, cmd := m.Update(resourceTreeLoadedMsg{gen: 1})
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinal2UpdateResourceTreeLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, cmd := m.Update(resourceTreeLoadedMsg{err: assert.AnError, gen: 1})
	_ = result.(Model)
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update: warning events filtering in resources loaded
// =====================================================================

func TestFinal2UpdateResourcesLoadedWarningEventsOnly(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	m.warningEventsOnly = true
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Event", Resource: "events"}
	items := []model.Item{
		{Name: "evt-1", Status: "Warning"},
		{Name: "evt-2", Status: "Normal"},
		{Name: "evt-3", Status: "Warning"},
	}
	result, _ := m.Update(resourcesLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.Equal(t, 2, len(rm.middleItems))
}

func TestFinal2UpdateResourcesLoadedPreviewWarningFilter(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	m.warningEventsOnly = true
	items := []model.Item{
		{Name: "evt-1", Kind: "Event", Status: "Warning"},
		{Name: "evt-2", Kind: "Event", Status: "Normal"},
	}
	result, _ := m.Update(resourcesLoadedMsg{items: items, gen: 1, forPreview: true})
	rm := result.(Model)
	assert.Equal(t, 1, len(rm.rightItems))
}

func TestFinal2UpdateResourcesLoadedMultiNSFilter(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	m.selectedNamespaces = map[string]bool{"ns1": true, "ns2": true}
	items := []model.Item{
		{Name: "pod-1", Namespace: "ns1"},
		{Name: "pod-2", Namespace: "ns2"},
		{Name: "pod-3", Namespace: "ns3"},
	}
	result, _ := m.Update(resourcesLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.Equal(t, 2, len(rm.middleItems))
}

// portForwardStartedMsg/portForwardUpdateMsg require PortForwardManager -- skipped.

// =====================================================================
// Update: explainRecursiveMsg
// =====================================================================

func TestFinal2UpdateExplainRecursiveMsg(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeExplain
	matches := []model.ExplainField{{Name: "containers", Type: "[]Container"}}
	result, _ := m.Update(explainRecursiveMsg{matches: matches, query: "container"})
	rm := result.(Model)
	assert.Equal(t, 1, len(rm.explainRecursiveResults))
}

func TestFinal2UpdateExplainRecursiveMsgError(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeExplain
	result, cmd := m.Update(explainRecursiveMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update: containerActionLoadedMsg
// =====================================================================

func TestFinal2UpdateContainerSelectMsg(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	items := []model.Item{{Name: "nginx"}, {Name: "sidecar"}}
	result, _ := m.Update(containerSelectMsg{items: items})
	rm := result.(Model)
	assert.Equal(t, overlayContainerSelect, rm.overlay)
}

func TestFinal2UpdateContainerSelectMsgSingle(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	items := []model.Item{{Name: "nginx"}}
	result, _ := m.Update(containerSelectMsg{items: items})
	rm := result.(Model)
	assert.Equal(t, "nginx", rm.actionCtx.containerName)
}

func TestFinal2UpdateContainerSelectMsgError(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	result, cmd := m.Update(containerSelectMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.Empty(t, rm.pendingAction)
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update: podSelectMsg
// =====================================================================

func TestFinal2UpdatePodSelectMsg(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	items := []model.Item{{Name: "pod-1", Kind: "Pod"}, {Name: "pod-2", Kind: "Pod"}}
	result, _ := m.Update(podSelectMsg{items: items})
	rm := result.(Model)
	assert.Equal(t, overlayPodSelect, rm.overlay)
}

func TestFinal2UpdatePodSelectMsgSingle(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	items := []model.Item{{Name: "pod-1", Kind: "Pod"}}
	result, cmd := m.Update(podSelectMsg{items: items})
	rm := result.(Model)
	assert.Equal(t, "pod-1", rm.actionCtx.name)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdatePodSelectMsgNoPods(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	items := []model.Item{{Name: "not-a-pod", Kind: "Service"}}
	result, cmd := m.Update(podSelectMsg{items: items})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "No pods")
	assert.NotNil(t, cmd)
}

func TestFinal2UpdatePodSelectMsgError(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	result, cmd := m.Update(podSelectMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

// =====================================================================
// Additional Update: CRD deeper level
// =====================================================================

func TestFinal2UpdateCRDDiscoveryMsgDeeperLevel(t *testing.T) {
	m := baseFinalModel()
	m.nav.Context = "test-ctx"
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1"}
	entries := []model.ResourceTypeEntry{
		{Kind: "Pod", Resource: "pods", APIVersion: "v1"},
		{Kind: "CustomResource", Resource: "crs", APIGroup: "example.com", APIVersion: "v1"},
	}
	result, _ := m.Update(crdDiscoveryMsg{context: "test-ctx", entries: entries})
	rm := result.(Model)
	assert.Contains(t, rm.discoveredCRDs, "test-ctx")
}

// =====================================================================
// Additional exec tests: functions that need cmd != nil only
// =====================================================================

func TestFinal2ForceDeleteResource(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	cmd := m.forceDeleteResource()
	assert.NotNil(t, cmd)
}

func TestFinal2ForceDeleteResourceNonNamespaced(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Node", Resource: "nodes", Namespaced: false}
	cmd := m.forceDeleteResource()
	assert.NotNil(t, cmd)
}

func TestFinal2RemoveFinalizers(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	cmd := m.removeFinalizers()
	assert.NotNil(t, cmd)
}

func TestFinal2RemoveFinalizersNonNamespaced(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Node", Resource: "nodes", Namespaced: false}
	cmd := m.removeFinalizers()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlNodeCmd(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.name = "node-1"
	m.actionCtx.context = "test-ctx"
	cmd := m.execKubectlNodeCmd("cordon")
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlDrain(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlDrain()
	assert.NotNil(t, cmd)
}

func TestFinal2RollbackHelmRelease(t *testing.T) {
	m := baseFinalModel()
	cmd := m.rollbackHelmRelease(1)
	assert.NotNil(t, cmd)
}

func TestFinal2VulnScanImage(t *testing.T) {
	m := baseFinalModel()
	cmd := m.vulnScanImage("nginx:latest")
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlExplain(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execKubectlExplain("pods", "", "")
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlExplainWithFieldPath(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execKubectlExplain("pods", "v1", "spec.containers")
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlExplainRecursive(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execKubectlExplainRecursive("pods", "", "container")
	assert.NotNil(t, cmd)
}

func TestFinal2ExecCustomAction(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execCustomAction("echo hello")
	assert.NotNil(t, cmd)
}

func TestFinal2HelmDiff(t *testing.T) {
	m := baseFinalModel()
	cmd := m.helmDiff()
	assert.NotNil(t, cmd)
}

func TestFinal2HelmUpgrade(t *testing.T) {
	m := baseFinalModel()
	cmd := m.helmUpgrade()
	assert.NotNil(t, cmd)
}

func TestFinal2EditHelmValues(t *testing.T) {
	m := baseFinalModel()
	cmd := m.editHelmValues()
	assert.NotNil(t, cmd)
}

func TestFinal2UninstallHelmRelease(t *testing.T) {
	m := baseFinalModel()
	cmd := m.uninstallHelmRelease()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlNodeShell(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlNodeShell()
	assert.NotNil(t, cmd)
}

func TestFinal2RunDebugPod(t *testing.T) {
	m := baseFinalModel()
	cmd := m.runDebugPod()
	assert.NotNil(t, cmd)
}

func TestFinal2RunDebugPodWithPVC(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.name = "my-pvc"
	cmd := m.runDebugPodWithPVC()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlExec(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execKubectlExec()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlExecWithContainer(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = "web"
	cmd := m.execKubectlExec()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlAttach(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execKubectlAttach()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlAttachWithContainer(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = "web"
	cmd := m.execKubectlAttach()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlDebug(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execKubectlDebug()
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update: spinner tick
// =====================================================================

func TestFinal2UpdateSpinnerTick(t *testing.T) {
	m := baseFinalModel()
	m.spinner = spinner.New()
	// Send a spinner tick message directly.
	result, _ := m.Update(spinner.TickMsg{})
	_ = result.(Model)
}

// =====================================================================
// Update: more msgs that route to handler functions
// =====================================================================

// portForwardUpdateMsg requires PortForwardManager -- skipped.
