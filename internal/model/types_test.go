package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ResourceTypeEntry.CanList ---

func TestResourceTypeEntry_CanList(t *testing.T) {
	cases := []struct {
		name  string
		verbs []string
		want  bool
	}{
		{"empty verbs treated as listable (pseudo-resources)", nil, true},
		{"full verb set", []string{"get", "list", "watch", "create", "update", "patch", "delete"}, true},
		{"only list", []string{"list"}, true},
		{"review API: create-only", []string{"create"}, false},
		{"no list verb", []string{"get", "watch"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := ResourceTypeEntry{Verbs: tc.verbs}
			assert.Equal(t, tc.want, e.CanList())
		})
	}
}

// --- ResourceTypeEntry.ResourceRef ---

func TestResourceRef(t *testing.T) {
	tests := []struct {
		name     string
		entry    ResourceTypeEntry
		expected string
	}{
		{
			"core v1 resource",
			ResourceTypeEntry{APIGroup: "", APIVersion: "v1", Resource: "pods"},
			"/v1/pods",
		},
		{
			"apps group",
			ResourceTypeEntry{APIGroup: "apps", APIVersion: "v1", Resource: "deployments"},
			"apps/v1/deployments",
		},
		{
			"networking group",
			ResourceTypeEntry{APIGroup: "networking.k8s.io", APIVersion: "v1", Resource: "ingresses"},
			"networking.k8s.io/v1/ingresses",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.entry.ResourceRef())
		})
	}
}

// --- ActionsForKind ---

func TestActionsForKind(t *testing.T) {
	t.Run("Pod actions", func(t *testing.T) {
		actions := ActionsForKind("Pod")
		labels := actionLabels(actions)
		assert.Contains(t, labels, "Logs")
		assert.Contains(t, labels, "Exec")
		assert.Contains(t, labels, "Delete")
		assert.Contains(t, labels, "Port Forward")
		assert.Contains(t, labels, "Debug")
	})

	t.Run("Deployment actions", func(t *testing.T) {
		actions := ActionsForKind("Deployment")
		labels := actionLabels(actions)
		assert.Contains(t, labels, "Scale")
		assert.Contains(t, labels, "Restart")
		assert.Contains(t, labels, "Delete")
	})

	t.Run("Node actions", func(t *testing.T) {
		actions := ActionsForKind("Node")
		labels := actionLabels(actions)
		assert.Contains(t, labels, "Cordon")
		assert.Contains(t, labels, "Uncordon")
		assert.Contains(t, labels, "Drain")
		assert.NotContains(t, labels, "Delete")
	})

	t.Run("default actions", func(t *testing.T) {
		actions := ActionsForKind("UnknownKind")
		labels := actionLabels(actions)
		assert.Contains(t, labels, "Describe")
		assert.Contains(t, labels, "Edit")
		assert.Contains(t, labels, "Delete")
	})

	t.Run("HelmRelease actions", func(t *testing.T) {
		actions := ActionsForKind("HelmRelease")
		labels := actionLabels(actions)
		assert.Contains(t, labels, "Describe")
		assert.Contains(t, labels, "Delete")
		assert.Contains(t, labels, "History")
		assert.Contains(t, labels, "Rollback")
		assert.NotContains(t, labels, "Edit")
	})

	t.Run("Application actions", func(t *testing.T) {
		actions := ActionsForKind("Application")
		labels := actionLabels(actions)
		assert.Contains(t, labels, "Sync")
		assert.Contains(t, labels, "Terminate Sync")
		assert.Contains(t, labels, "Refresh")
	})

	t.Run("Certificate actions", func(t *testing.T) {
		actions := ActionsForKind("Certificate")
		labels := actionLabels(actions)
		assert.Contains(t, labels, "Describe")
		assert.Contains(t, labels, "Edit")
		assert.Contains(t, labels, "Delete")
		assert.Contains(t, labels, "Events")
	})

	t.Run("Issuer actions", func(t *testing.T) {
		actions := ActionsForKind("Issuer")
		labels := actionLabels(actions)
		assert.Contains(t, labels, "Describe")
		assert.Contains(t, labels, "Edit")
		assert.Contains(t, labels, "Delete")
		assert.Contains(t, labels, "Events")
	})

	t.Run("Order actions", func(t *testing.T) {
		actions := ActionsForKind("Order")
		labels := actionLabels(actions)
		assert.Contains(t, labels, "Describe")
		assert.Contains(t, labels, "Delete")
		assert.Contains(t, labels, "Events")
		assert.NotContains(t, labels, "Edit")
	})

	t.Run("NetworkPolicy actions", func(t *testing.T) {
		actions := ActionsForKind("NetworkPolicy")
		labels := actionLabels(actions)
		assert.Contains(t, labels, "Visualize")
		assert.Contains(t, labels, "Describe")
		assert.Contains(t, labels, "Edit")
		assert.Contains(t, labels, "Delete")
		assert.Contains(t, labels, "Events")
		assert.Contains(t, labels, "Permissions")
	})
}

func TestActionsForKind_ScaleableKinds(t *testing.T) {
	for _, kind := range []string{"Deployment", "StatefulSet", "ReplicaSet"} {
		t.Run(kind+" has Scale", func(t *testing.T) {
			actions := ActionsForKind(kind)
			labels := actionLabels(actions)
			assert.Contains(t, labels, "Scale", "%s should have Scale action", kind)
		})
	}

	for _, kind := range []string{"Pod", "DaemonSet", "Service", "ConfigMap"} {
		t.Run(kind+" no Scale", func(t *testing.T) {
			actions := ActionsForKind(kind)
			labels := actionLabels(actions)
			assert.NotContains(t, labels, "Scale", "%s should not have Scale action", kind)
		})
	}
}

func TestActionsForKind_RestartableKinds(t *testing.T) {
	for _, kind := range []string{"Deployment", "StatefulSet", "DaemonSet"} {
		t.Run(kind+" has Restart", func(t *testing.T) {
			actions := ActionsForKind(kind)
			labels := actionLabels(actions)
			assert.Contains(t, labels, "Restart", "%s should have Restart action", kind)
		})
	}
}

// --- IsScaleableKind / IsRestartableKind ---

func TestIsScaleableKind(t *testing.T) {
	assert.True(t, IsScaleableKind("Deployment"))
	assert.True(t, IsScaleableKind("StatefulSet"))
	assert.True(t, IsScaleableKind("ReplicaSet"))
	assert.False(t, IsScaleableKind("DaemonSet"))
	assert.False(t, IsScaleableKind("Pod"))
	assert.False(t, IsScaleableKind("Service"))
	assert.False(t, IsScaleableKind(""))
}

func TestIsRestartableKind(t *testing.T) {
	assert.True(t, IsRestartableKind("Deployment"))
	assert.True(t, IsRestartableKind("StatefulSet"))
	assert.True(t, IsRestartableKind("DaemonSet"))
	assert.False(t, IsRestartableKind("ReplicaSet"))
	assert.False(t, IsRestartableKind("Pod"))
	assert.False(t, IsRestartableKind("Service"))
	assert.False(t, IsRestartableKind(""))
}

func TestActionsForContainer(t *testing.T) {
	actions := ActionsForContainer()
	assert.NotEmpty(t, actions)
	labels := actionLabels(actions)
	assert.Contains(t, labels, "Logs")
	assert.Contains(t, labels, "Exec")
	assert.Contains(t, labels, "Debug")
}

func TestActionsForBulk(t *testing.T) {
	actions := ActionsForBulk("")
	assert.NotEmpty(t, actions)
	labels := actionLabels(actions)
	assert.Contains(t, labels, "Delete")
	assert.Contains(t, labels, "Scale")
	assert.Contains(t, labels, "Restart")
	// Generic bulk should NOT include ArgoCD actions.
	assert.NotContains(t, labels, "Sync")
	assert.NotContains(t, labels, "Refresh")
}

func TestActionsForBulkApplication(t *testing.T) {
	actions := ActionsForBulk("Application")
	labels := actionLabels(actions)
	assert.Contains(t, labels, "Sync", "Application bulk should include Sync")
	assert.Contains(t, labels, "Sync (Apply Only)", "Application bulk should include Sync (Apply Only)")
	assert.Contains(t, labels, "Refresh", "Application bulk should include Refresh")
	// Should still have generic bulk actions.
	assert.Contains(t, labels, "Delete")
	// ArgoCD actions should appear before generic actions.
	assert.Equal(t, "Sync", actions[0].Label, "Sync should be first")
	assert.Equal(t, "Sync (Apply Only)", actions[1].Label, "Sync (Apply Only) should be second")
	assert.Equal(t, "Refresh", actions[2].Label, "Refresh should be third")
}

func TestActionsForPortForward(t *testing.T) {
	actions := ActionsForPortForward()
	require.Len(t, actions, 4, "should return exactly 4 port forward actions")

	tests := []struct {
		name        string
		wantLabel   string
		wantKey     string
		wantDescNot string
	}{
		{"stop action", "Stop", "s", ""},
		{"restart action", "Restart", "r", ""},
		{"remove action", "Remove", "D", ""},
		{"open in browser action", "Open in Browser", "O", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for _, a := range actions {
				if a.Label == tt.wantLabel {
					found = true
					assert.Equal(t, tt.wantKey, a.Key)
					assert.NotEmpty(t, a.Description)
					break
				}
			}
			assert.True(t, found, "expected action with label %q", tt.wantLabel)
		})
	}

	// Verify unique keys across all actions.
	keys := make(map[string]bool)
	for _, a := range actions {
		assert.False(t, keys[a.Key], "duplicate key %q", a.Key)
		keys[a.Key] = true
	}
}

// --- IsForceDeleteableKind ---

func TestIsForceDeleteableKind(t *testing.T) {
	tests := []struct {
		label string
		kind  string
		want  bool
	}{
		{"Pod", "Pod", true},
		{"Job", "Job", true},
		{"Deployment", "Deployment", false},
		{"StatefulSet", "StatefulSet", false},
		{"DaemonSet", "DaemonSet", false},
		{"ReplicaSet", "ReplicaSet", false},
		{"Service", "Service", false},
		{"ConfigMap", "ConfigMap", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			assert.Equal(t, tt.want, IsForceDeleteableKind(tt.kind))
		})
	}
}

// --- IsCoreCategory ---

func TestIsCoreCategory(t *testing.T) {
	tests := []struct {
		label    string
		category string
		want     bool
	}{
		{"Dashboards", "Dashboards", true},
		{"Security", "Security", true},
		{"Workloads", "Workloads", true},
		{"Config", "Config", true},
		{"Networking", "Networking", true},
		{"Storage", "Storage", true},
		{"Access Control", "Access Control", true},
		{"Cluster", "Cluster", true},
		{"Helm", "Helm", true},
		{"API and CRDs", "API and CRDs", true},
		{"argoproj.io", "argoproj.io", false},
		{"gateway.networking.k8s.io", "gateway.networking.k8s.io", false},
		{"cert-manager.io", "cert-manager.io", false},
		{"Custom", "Custom", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			assert.Equal(t, tt.want, IsCoreCategory(tt.category))
		})
	}
}

func TestAPICRDsIsCoreCategory(t *testing.T) {
	assert.True(t, IsCoreCategory("API and CRDs"),
		"API and CRDs should be a core category")
}

// TestSecurityIsCoreCategoryAlwaysShown guards against accidental removal of
// the Security entry from CoreCategories. The Security category is dynamically
// populated from SecuritySourcesFn at runtime, but the category itself must
// always be a core category so it renders in the fixed order.
func TestSecurityIsCoreCategoryAlwaysShown(t *testing.T) {
	assert.True(t, IsCoreCategory("Security"), "Security must be a core category")
}

// --- Templates ---

func TestBuiltinTemplates(t *testing.T) {
	templates := BuiltinTemplates()
	assert.NotEmpty(t, templates)
	for _, tmpl := range templates {
		assert.NotEmpty(t, tmpl.Name, "template should have a name")
		assert.NotEmpty(t, tmpl.Description, "template should have a description")
		assert.NotEmpty(t, tmpl.Category, "template should have a category")
		assert.NotEmpty(t, tmpl.YAML, "template should have YAML")
	}
}

func TestBuiltinTemplatesContainNamespace(t *testing.T) {
	// Cluster-scoped resources do not have a namespace field.
	clusterScoped := map[string]bool{
		"Namespace":          true,
		"PersistentVolume":   true,
		"StorageClass":       true,
		"ClusterRole":        true,
		"ClusterRoleBinding": true,
	}
	for _, tmpl := range BuiltinTemplates() {
		if clusterScoped[tmpl.Name] {
			continue
		}
		require.Contains(t, tmpl.YAML, "NAMESPACE",
			"template %s should contain NAMESPACE placeholder", tmpl.Name)
	}
}

// helper
func actionLabels(actions []ActionMenuItem) []string {
	labels := make([]string, len(actions))
	for i, a := range actions {
		labels[i] = a.Label
	}
	return labels
}

func TestBookmarkIsContextAware(t *testing.T) {
	tests := []struct {
		name    string
		context string
		want    bool
	}{
		{name: "empty context is context-free", context: "", want: false},
		{name: "populated context is context-aware", context: "prod-cluster", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := Bookmark{Context: tt.context}
			if got := bm.IsContextAware(); got != tt.want {
				t.Errorf("IsContextAware() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- Security virtual API group ---

func TestSecurityVirtualAPIGroupConstant(t *testing.T) {
	assert.Equal(t, "_security", SecurityVirtualAPIGroup)
}

// --- Item.ColumnValue ---

func TestItemColumnValuePresent(t *testing.T) {
	it := Item{
		Columns: []KeyValue{
			{Key: "Severity", Value: "CRIT"},
			{Key: "Title", Value: "CVE-2024-1234"},
		},
	}
	assert.Equal(t, "CRIT", it.ColumnValue("Severity"))
	assert.Equal(t, "CVE-2024-1234", it.ColumnValue("Title"))
}

func TestItemColumnValueAbsent(t *testing.T) {
	it := Item{Columns: []KeyValue{{Key: "Severity", Value: "HIGH"}}}
	assert.Equal(t, "", it.ColumnValue("Missing"))
}

func TestItemColumnValueEmptyColumns(t *testing.T) {
	it := Item{}
	assert.Equal(t, "", it.ColumnValue("anything"))
}
