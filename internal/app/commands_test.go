package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- shellQuote ---

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple word",
			input:    "hello",
			expected: "'hello'",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "''",
		},
		{
			name:     "string with spaces",
			input:    "hello world",
			expected: "'hello world'",
		},
		{
			name:     "string with single quote",
			input:    "it's",
			expected: "'it'\"'\"'s'",
		},
		{
			name:     "string with multiple single quotes",
			input:    "it's a 'test'",
			expected: "'it'\"'\"'s a '\"'\"'test'\"'\"''",
		},
		{
			name:     "string with double quotes",
			input:    `say "hello"`,
			expected: `'say "hello"'`,
		},
		{
			name:     "string with special characters",
			input:    "a$b&c|d;e",
			expected: "'a$b&c|d;e'",
		},
		{
			name:     "string with newline",
			input:    "line1\nline2",
			expected: "'line1\nline2'",
		},
		{
			name:     "string with backslash",
			input:    `path\to\file`,
			expected: `'path\to\file'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shellQuote(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- classifyInput (kubectl detection) ---

func TestClassifyInputKubectl(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		isKubectl bool
	}{
		// Explicit "kubectl" prefix cases.
		{name: "starts with kubectl space", input: "kubectl get pods", isKubectl: true},
		{name: "just kubectl", input: "kubectl", isKubectl: true},
		// Bare subcommand cases -- no longer classified as kubectl.
		{name: "get subcommand", input: "get pods -n kube-system", isKubectl: false},
		{name: "describe subcommand", input: "describe pod my-pod", isKubectl: false},
		{name: "logs subcommand", input: "logs my-pod -f", isKubectl: false},
		{name: "exec subcommand", input: "exec -it my-pod -- bash", isKubectl: false},
		{name: "delete subcommand", input: "delete pod my-pod", isKubectl: false},
		{name: "apply subcommand", input: "apply -f deployment.yaml", isKubectl: false},
		{name: "create subcommand", input: "create namespace test", isKubectl: false},
		{name: "edit subcommand", input: "edit deployment my-deploy", isKubectl: false},
		{name: "patch subcommand", input: "patch svc my-svc -p {}", isKubectl: false},
		{name: "scale subcommand", input: "scale deployment my-deploy --replicas=3", isKubectl: false},
		{name: "rollout subcommand", input: "rollout restart deployment my-deploy", isKubectl: false},
		{name: "top subcommand", input: "top pods", isKubectl: false},
		{name: "label subcommand", input: "label pod my-pod env=prod", isKubectl: false},
		{name: "annotate subcommand", input: "annotate pod my-pod note=test", isKubectl: false},
		{name: "port-forward subcommand", input: "port-forward svc/my-svc 8080:80", isKubectl: false},
		{name: "cp subcommand", input: "cp /tmp/foo my-pod:/tmp/bar", isKubectl: false},
		{name: "cordon subcommand", input: "cordon my-node", isKubectl: false},
		{name: "uncordon subcommand", input: "uncordon my-node", isKubectl: false},
		{name: "drain subcommand", input: "drain my-node", isKubectl: false},
		{name: "taint subcommand", input: "taint node my-node key=val:NoSchedule", isKubectl: false},
		{name: "config subcommand", input: "config view", isKubectl: false},
		{name: "auth subcommand", input: "auth can-i get pods", isKubectl: false},
		{name: "api-resources subcommand", input: "api-resources", isKubectl: false},
		{name: "explain subcommand", input: "explain pod.spec", isKubectl: false},
		{name: "diff subcommand", input: "diff -f deployment.yaml", isKubectl: false},
		// Non-kubectl cases.
		{name: "shell command", input: "echo hello", isKubectl: false},
		{name: "arbitrary command", input: "ls -la /tmp", isKubectl: false},
		{name: "curl command", input: "curl http://example.com", isKubectl: false},
		{name: "helm command", input: "helm list", isKubectl: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyInput(tt.input)
			if tt.isKubectl {
				assert.Equal(t, cmdKubectl, result)
			} else {
				assert.NotEqual(t, cmdKubectl, result)
			}
		})
	}
}

// --- startupTips ---

func TestStartupTipsNotEmpty(t *testing.T) {
	assert.NotEmpty(t, startupTips, "startupTips should not be empty")
}

func TestStartupTipsAllNonEmpty(t *testing.T) {
	for i, tip := range startupTips {
		assert.NotEmpty(t, tip, "tip at index %d should not be empty", i)
	}
}

func TestCov80BulkDeleteWithFakeClient(t *testing.T) {
	m := basePush80Model()
	m.bulkItems = []model.Item{
		{Name: "pod-1", Namespace: "default"},
		{Name: "pod-2", Namespace: "other-ns"},
		{Name: "pod-3"}, // no namespace, should use fallback
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		resourceType: model.ResourceTypeEntry{Resource: "pods", Kind: "Pod", Namespaced: true},
	}
	cmd := m.bulkDeleteResources()
	require.NotNil(t, cmd)
	msg := cmd()
	result, ok := msg.(bulkActionResultMsg)
	require.True(t, ok)
	// Fake client can't find these resources, so they all fail.
	assert.Equal(t, 3, result.failed)
}

func TestExpandGroupedItems_NoGroupedRefs(t *testing.T) {
	items := []model.Item{
		{Name: "pod-1", Namespace: "ns-1"},
		{Name: "pod-2", Namespace: "ns-2"},
	}
	refs := expandGroupedItems(items)
	require.Len(t, refs, 2)
	assert.Equal(t, "pod-1", refs[0].Name)
	assert.Equal(t, "ns-1", refs[0].Namespace)
	assert.Equal(t, "pod-2", refs[1].Name)
	assert.Equal(t, "ns-2", refs[1].Namespace)
}

func TestExpandGroupedItems_WithGroupedRefs(t *testing.T) {
	items := []model.Item{
		{
			Name:      "evt-first",
			Namespace: "ns-1",
			GroupedRefs: []model.GroupedRef{
				{Name: "evt-aaa", Namespace: "ns-1"},
				{Name: "evt-bbb", Namespace: "ns-1"},
				{Name: "evt-ccc", Namespace: "ns-2"},
			},
		},
		{Name: "pod-1", Namespace: "default"},
	}
	refs := expandGroupedItems(items)
	require.Len(t, refs, 4, "grouped event expands to 3 refs + 1 plain item")
	assert.Equal(t, "evt-aaa", refs[0].Name)
	assert.Equal(t, "evt-bbb", refs[1].Name)
	assert.Equal(t, "evt-ccc", refs[2].Name)
	assert.Equal(t, "pod-1", refs[3].Name)
}

func TestExpandGroupedItems_Empty(t *testing.T) {
	refs := expandGroupedItems(nil)
	assert.Empty(t, refs)
}

func TestBulkDeleteExpandsGroupedEvents(t *testing.T) {
	m := basePush80Model()
	m.bulkItems = []model.Item{
		{
			Name:      "evt-first",
			Namespace: "default",
			Kind:      "Event",
			GroupedRefs: []model.GroupedRef{
				{Name: "evt-aaa", Namespace: "default"},
				{Name: "evt-bbb", Namespace: "default"},
			},
		},
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		resourceType: model.ResourceTypeEntry{Resource: "events", Kind: "Event", APIVersion: "v1", Namespaced: true},
	}
	cmd := m.bulkDeleteResources()
	require.NotNil(t, cmd)
	msg := cmd()
	result, ok := msg.(bulkActionResultMsg)
	require.True(t, ok)
	// 2 underlying events should be attempted (both fail since fake client has none).
	assert.Equal(t, 2, result.failed, "should attempt to delete all underlying events, not just the grouped row")
}

func TestCov80BulkScaleWithFakeClient(t *testing.T) {
	m := basePush80Model()
	m.bulkItems = []model.Item{
		{Name: "deploy-1", Namespace: "default"},
		{Name: "deploy-2"},
	}
	m.actionCtx = actionContext{
		context: "test-ctx",
		kind:    "Deployment",
	}
	cmd := m.bulkScaleResources(3)
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(bulkActionResultMsg)
	assert.True(t, ok)
}

func TestCov80BulkScaleToZero(t *testing.T) {
	m := basePush80Model()
	m.bulkItems = []model.Item{{Name: "deploy-1", Namespace: "ns1"}}
	m.actionCtx = actionContext{context: "test-ctx", kind: "Deployment"}
	cmd := m.bulkScaleResources(0)
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(bulkActionResultMsg)
	assert.True(t, ok)
}

func TestCov80BulkRestartWithFakeClient(t *testing.T) {
	m := basePush80Model()
	m.bulkItems = []model.Item{
		{Name: "deploy-1", Namespace: "default"},
		{Name: "deploy-2", Namespace: "ns2"},
	}
	m.actionCtx = actionContext{context: "test-ctx", kind: "Deployment"}
	cmd := m.bulkRestartResources()
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(bulkActionResultMsg)
	assert.True(t, ok)
}

func TestCov80BatchPatchLabelsAdd(t *testing.T) {
	m := basePush80Model()
	m.bulkItems = []model.Item{
		{Name: "pod-1", Namespace: "default"},
		{Name: "pod-2"},
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		resourceType: model.ResourceTypeEntry{Resource: "pods", APIVersion: "v1", Namespaced: true},
	}
	cmd := m.batchPatchLabels("env", "prod", false, false)
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(bulkActionResultMsg)
	assert.True(t, ok)
}

func TestCov80BatchPatchLabelsRemove(t *testing.T) {
	m := basePush80Model()
	m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "default"}}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		resourceType: model.ResourceTypeEntry{Resource: "pods", APIVersion: "v1", Namespaced: true},
	}
	cmd := m.batchPatchLabels("env", "", true, false)
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(bulkActionResultMsg)
	assert.True(t, ok)
}

func TestCov80BatchPatchAnnotations(t *testing.T) {
	m := basePush80Model()
	m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "default"}}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		resourceType: model.ResourceTypeEntry{Resource: "pods", APIVersion: "v1", Namespaced: true},
	}
	cmd := m.batchPatchLabels("note", "value", false, true)
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(bulkActionResultMsg)
	assert.True(t, ok)
}

func TestCov80OpenBulkActionDirectNoSelection(t *testing.T) {
	m := basePush80Model()
	m.selectedItems = make(map[string]bool)
	result, cmd := m.openBulkActionDirect("Delete")
	rm := result.(Model)
	assert.False(t, rm.bulkMode)
	assert.Nil(t, cmd)
}

func TestCov80OpenBulkActionDirectWithSelection(t *testing.T) {
	m := basePush80Model()
	// Selection keys use "namespace/name" format (see selectionKey()).
	m.selectedItems = map[string]bool{
		"default/pod-1": true,
		"ns-2/pod-2":    true,
	}
	result, _ := m.openBulkActionDirect("Delete")
	rm := result.(Model)
	assert.True(t, rm.bulkMode)
}

func TestCov80OpenBulkActionDirectLogs(t *testing.T) {
	m := basePush80Model()
	m.selectedItems = map[string]bool{
		"default/pod-1": true,
	}
	result, _ := m.openBulkActionDirect("Logs")
	// executeBulkAction("Logs") may return *Model.
	switch rm := result.(type) {
	case Model:
		assert.False(t, rm.overlay == overlayAction)
	case *Model:
		assert.False(t, rm.overlay == overlayAction)
	}
}

func TestCov80ExportResourceToFileNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.exportResourceToFile()
	assert.Nil(t, cmd)
}

func TestCov80ExportResourceToFileDefaultLevel(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	cmd := m.exportResourceToFile()
	assert.Nil(t, cmd)
}

func TestCov80CopyYAMLNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.copyYAMLToClipboard()
	assert.Nil(t, cmd)
}

func TestCov80CopyYAMLLevelClusters(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	cmd := m.copyYAMLToClipboard()
	assert.Nil(t, cmd)
}

func TestCov80LoadContainersForAction(t *testing.T) {
	m := basePush80Model()
	m.actionCtx = actionContext{
		context:   "test-ctx",
		name:      "my-pod",
		namespace: "default",
	}
	cmd := m.loadContainersForAction()
	require.NotNil(t, cmd)
}

func TestCov80LoadPodsForAction(t *testing.T) {
	m := basePush80Model()
	m.actionCtx = actionContext{
		context:   "test-ctx",
		name:      "deploy-1",
		namespace: "default",
		kind:      "Deployment",
	}
	cmd := m.loadPodsForAction()
	require.NotNil(t, cmd)
}

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

func TestCovLoadPodsForActionFakeClient(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-deploy", "default", "Deployment", model.ResourceTypeEntry{})
	cmd := m.loadPodsForAction()
	// Just verify command is returned; the GetOwnedResources call needs
	// replicaset list kind registered in the dynamic client.
	assert.NotNil(t, cmd)
}

func TestCovOpenBulkActionDirectNoSelection(t *testing.T) {
	m := baseModelCov()
	m.selectedItems = map[string]bool{}
	ret, cmd := m.openBulkActionDirect("Delete")
	_ = ret.(Model)
	assert.Nil(t, cmd)
}

func TestCovExecuteCommandBarInputEmpty(t *testing.T) {
	m := baseModelWithFakeClient()
	_, cmd := m.executeCommandBarInput("")
	assert.Nil(t, cmd)
}

func TestCovExecuteCommandBarInputShell(t *testing.T) {
	m := baseModelWithFakeClient()
	_, cmd := m.executeCommandBarInput("!echo hello")
	// Shell commands use tea.ExecProcess, so cmd is non-nil.
	assert.NotNil(t, cmd)
}

func TestCovExecuteCommandBarInputKubectl(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"
	m.namespace = "default"
	_, cmd := m.executeCommandBarInput("kubectl version --client")
	// Returns non-nil even if kubectl is not found.
	assert.NotNil(t, cmd)
}
