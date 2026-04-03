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

// --- isKubectlCommand ---

func TestIsKubectlCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Explicit "kubectl" prefix cases.
		{
			name:     "starts with kubectl space",
			input:    "kubectl get pods",
			expected: true,
		},
		{
			name:     "just kubectl",
			input:    "kubectl",
			expected: true,
		},
		{
			name:     "kubectl with leading whitespace",
			input:    "  kubectl get pods",
			expected: true,
		},
		// Known subcommand cases.
		{
			name:     "get subcommand",
			input:    "get pods -n kube-system",
			expected: true,
		},
		{
			name:     "describe subcommand",
			input:    "describe pod my-pod",
			expected: true,
		},
		{
			name:     "logs subcommand",
			input:    "logs my-pod -f",
			expected: true,
		},
		{
			name:     "exec subcommand",
			input:    "exec -it my-pod -- bash",
			expected: true,
		},
		{
			name:     "delete subcommand",
			input:    "delete pod my-pod",
			expected: true,
		},
		{
			name:     "apply subcommand",
			input:    "apply -f deployment.yaml",
			expected: true,
		},
		{
			name:     "create subcommand",
			input:    "create namespace test",
			expected: true,
		},
		{
			name:     "edit subcommand",
			input:    "edit deployment my-deploy",
			expected: true,
		},
		{
			name:     "patch subcommand",
			input:    "patch svc my-svc -p {}",
			expected: true,
		},
		{
			name:     "scale subcommand",
			input:    "scale deployment my-deploy --replicas=3",
			expected: true,
		},
		{
			name:     "rollout subcommand",
			input:    "rollout restart deployment my-deploy",
			expected: true,
		},
		{
			name:     "top subcommand",
			input:    "top pods",
			expected: true,
		},
		{
			name:     "label subcommand",
			input:    "label pod my-pod env=prod",
			expected: true,
		},
		{
			name:     "annotate subcommand",
			input:    "annotate pod my-pod note=test",
			expected: true,
		},
		{
			name:     "port-forward subcommand",
			input:    "port-forward svc/my-svc 8080:80",
			expected: true,
		},
		{
			name:     "cp subcommand",
			input:    "cp /tmp/foo my-pod:/tmp/bar",
			expected: true,
		},
		{
			name:     "cordon subcommand",
			input:    "cordon my-node",
			expected: true,
		},
		{
			name:     "uncordon subcommand",
			input:    "uncordon my-node",
			expected: true,
		},
		{
			name:     "drain subcommand",
			input:    "drain my-node",
			expected: true,
		},
		{
			name:     "taint subcommand",
			input:    "taint node my-node key=val:NoSchedule",
			expected: true,
		},
		{
			name:     "config subcommand",
			input:    "config view",
			expected: true,
		},
		{
			name:     "auth subcommand",
			input:    "auth can-i get pods",
			expected: true,
		},
		{
			name:     "api-resources subcommand",
			input:    "api-resources",
			expected: true,
		},
		{
			name:     "explain subcommand",
			input:    "explain pod.spec",
			expected: true,
		},
		{
			name:     "diff subcommand",
			input:    "diff -f deployment.yaml",
			expected: true,
		},
		// Non-kubectl cases.
		{
			name:     "shell command",
			input:    "echo hello",
			expected: false,
		},
		{
			name:     "arbitrary command",
			input:    "ls -la /tmp",
			expected: false,
		},
		{
			name:     "curl command",
			input:    "curl http://example.com",
			expected: false,
		},
		{
			name:     "helm command",
			input:    "helm list",
			expected: false,
		},
		{
			name:     "subcommand case insensitive",
			input:    "GET pods",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isKubectlCommand(tt.input)
			assert.Equal(t, tt.expected, result)
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

func TestCovExecuteCommandBarEmpty(t *testing.T) {
	m := baseModelWithFakeClient()
	cmd := m.executeCommandBar("")
	assert.Nil(t, cmd)
}

func TestCovExecuteCommandBarShell(t *testing.T) {
	m := baseModelWithFakeClient()
	cmd := m.executeCommandBar("echo hello")
	msg := execCmd(t, cmd)
	result, ok := msg.(commandBarResultMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
	assert.Contains(t, result.output, "hello")
}

func TestCovExecuteCommandBarKubectl(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"
	m.namespace = "default"
	cmd := m.executeCommandBar("kubectl version --client")
	// Returns non-nil even if kubectl is not found.
	assert.NotNil(t, cmd)
}
