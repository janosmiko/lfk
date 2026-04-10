package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// --- TopLevelResourceTypes ---

func TestTopLevelResourceTypes(t *testing.T) {
	cats := TopLevelResourceTypes()
	assert.NotEmpty(t, cats)

	// Verify some expected categories exist.
	catNames := make(map[string]bool)
	for _, cat := range cats {
		catNames[cat.Name] = true
		// The Security category is dynamically populated from
		// SecuritySourcesFn; when the hook is unset (as in this test)
		// the category is present but empty.
		if cat.Name == "Security" {
			continue
		}
		assert.NotEmpty(t, cat.Types, "category %s should have types", cat.Name)
		for _, rt := range cat.Types {
			assert.NotEmpty(t, rt.DisplayName)
			assert.NotEmpty(t, rt.Kind)
			assert.NotEmpty(t, rt.Resource)
			assert.NotEmpty(t, rt.Icon)
		}
	}
	assert.True(t, catNames["Workloads"])
	assert.True(t, catNames["Config"])
	assert.True(t, catNames["Networking"])
	assert.True(t, catNames["Storage"])
	assert.True(t, catNames["Cluster"])
	assert.True(t, catNames["Security"])
}

// --- FlattenedResourceTypes ---

func TestFlattenedResourceTypes(t *testing.T) {
	items := FlattenedResourceTypes()
	assert.NotEmpty(t, items)

	// First item should be Cluster Dashboard.
	assert.Equal(t, "Cluster", items[0].Name)
	assert.Equal(t, "__overview__", items[0].Kind)

	// Should not contain ArgoCD (hasArgo=false by default).
	for _, item := range items {
		assert.NotEqual(t, "argoproj.io", item.Category, "ArgoCD should be hidden by default")
	}

	// Should not contain gateway.networking.k8s.io.
	for _, item := range items {
		assert.NotEqual(t, "gateway.networking.k8s.io", item.Category, "gateway.networking.k8s.io should be hidden by default")
	}

	// Helm should be present.
	hasHelm := false
	for _, item := range items {
		if item.Category == "Helm" {
			hasHelm = true
			break
		}
	}
	assert.True(t, hasHelm, "Helm should be present by default")

	// CRD-dependent entries (VPA) should not be present by default.
	for _, item := range items {
		assert.NotEqual(t, "VerticalPodAutoscaler", item.Kind,
			"VPA should be hidden by default (no CRD discovery)")
	}

	// cert-manager.io should not be present by default.
	for _, item := range items {
		assert.NotEqual(t, "cert-manager.io", item.Category, "cert-manager.io should be hidden by default")
	}

	// longhorn.io should not be present by default.
	for _, item := range items {
		assert.NotEqual(t, "longhorn.io", item.Category, "longhorn.io should be hidden by default")
	}

	// networking.istio.io should not be present by default.
	for _, item := range items {
		assert.NotEqual(t, "networking.istio.io", item.Category, "networking.istio.io should be hidden by default")
	}

	// cloud.google.com should not be present by default.
	for _, item := range items {
		assert.NotEqual(t, "cloud.google.com", item.Category, "cloud.google.com should be hidden by default")
	}

	// vpcresources.k8s.aws should not be present by default.
	for _, item := range items {
		assert.NotEqual(t, "vpcresources.k8s.aws", item.Category, "vpcresources.k8s.aws should be hidden by default")
	}
}

// --- FlattenedResourceTypesFiltered ---

func TestFlattenedResourceTypesFiltered(t *testing.T) {
	t.Run("nil availableGroups shows only core categories", func(t *testing.T) {
		items := FlattenedResourceTypesFiltered(nil)
		for _, item := range items {
			assert.NotEqual(t, "argoproj.io", item.Category)
			assert.NotEqual(t, "gateway.networking.k8s.io", item.Category)
			assert.NotEqual(t, "kustomize.toolkit.fluxcd.io", item.Category)
			assert.NotEqual(t, "helm.toolkit.fluxcd.io", item.Category)
			assert.NotEqual(t, "source.toolkit.fluxcd.io", item.Category)
			assert.NotEqual(t, "notification.toolkit.fluxcd.io", item.Category)
			assert.NotEqual(t, "image.toolkit.fluxcd.io", item.Category)
		}
	})

	t.Run("ArgoCD enabled", func(t *testing.T) {
		items := FlattenedResourceTypesFiltered(map[string]bool{"argoproj.io": true})
		hasArgo := false
		for _, item := range items {
			if item.Category == "argoproj.io" {
				hasArgo = true
				break
			}
		}
		assert.True(t, hasArgo)
	})

	t.Run("Gateway API enabled", func(t *testing.T) {
		items := FlattenedResourceTypesFiltered(map[string]bool{"gateway.networking.k8s.io": true})
		hasGW := false
		for _, item := range items {
			if item.Category == "gateway.networking.k8s.io" {
				hasGW = true
				break
			}
		}
		assert.True(t, hasGW)
	})

	t.Run("Flux enabled", func(t *testing.T) {
		items := FlattenedResourceTypesFiltered(map[string]bool{
			"kustomize.toolkit.fluxcd.io":    true,
			"helm.toolkit.fluxcd.io":         true,
			"source.toolkit.fluxcd.io":       true,
			"notification.toolkit.fluxcd.io": true,
			"image.toolkit.fluxcd.io":        true,
		})
		hasFlux := false
		for _, item := range items {
			if item.Category == "kustomize.toolkit.fluxcd.io" {
				hasFlux = true
				break
			}
		}
		assert.True(t, hasFlux)
	})

	t.Run("cert-manager enabled", func(t *testing.T) {
		groups := map[string]bool{
			"cert-manager.io":                     true,
			"acme.cert-manager.io":                true,
			"cert-manager.io/certificates":        true,
			"cert-manager.io/issuers":             true,
			"cert-manager.io/clusterissuers":      true,
			"cert-manager.io/certificaterequests": true,
			"acme.cert-manager.io/orders":         true,
			"acme.cert-manager.io/challenges":     true,
		}
		items := FlattenedResourceTypesFiltered(groups)
		hasCM := false
		for _, item := range items {
			if item.Category == "cert-manager.io" {
				hasCM = true
				break
			}
		}
		assert.True(t, hasCM)
	})

	t.Run("Longhorn enabled", func(t *testing.T) {
		items := FlattenedResourceTypesFiltered(map[string]bool{"longhorn.io": true})
		hasLH := false
		for _, item := range items {
			if item.Category == "longhorn.io" {
				hasLH = true
				break
			}
		}
		assert.True(t, hasLH)
	})

	t.Run("Istio enabled", func(t *testing.T) {
		items := FlattenedResourceTypesFiltered(map[string]bool{
			"networking.istio.io": true,
			"security.istio.io":   true,
			"telemetry.istio.io":  true,
		})
		hasIstio := false
		for _, item := range items {
			if item.Category == "networking.istio.io" {
				hasIstio = true
				break
			}
		}
		assert.True(t, hasIstio)
	})

	t.Run("GKE enabled", func(t *testing.T) {
		items := FlattenedResourceTypesFiltered(map[string]bool{
			"cloud.google.com":  true,
			"networking.gke.io": true,
		})
		hasGKE := false
		for _, item := range items {
			if item.Category == "cloud.google.com" {
				hasGKE = true
				break
			}
		}
		assert.True(t, hasGKE)
	})

	t.Run("EKS enabled", func(t *testing.T) {
		items := FlattenedResourceTypesFiltered(map[string]bool{
			"vpcresources.k8s.aws":  true,
			"crd.k8s.amazonaws.com": true,
		})
		hasEKS := false
		for _, item := range items {
			if item.Category == "vpcresources.k8s.aws" {
				hasEKS = true
				break
			}
		}
		assert.True(t, hasEKS)
	})

	t.Run("AKS enabled", func(t *testing.T) {
		items := FlattenedResourceTypesFiltered(map[string]bool{
			"aadpodidentity.k8s.io":           true,
			"infrastructure.cluster.x-k8s.io": true,
		})
		hasAKS := false
		for _, item := range items {
			if item.Category == "aadpodidentity.k8s.io" {
				hasAKS = true
				break
			}
		}
		assert.True(t, hasAKS)
	})

	t.Run("Helm always present as core category", func(t *testing.T) {
		items := FlattenedResourceTypesFiltered(nil)
		hasHelm := false
		for _, item := range items {
			if item.Category == "Helm" {
				hasHelm = true
				break
			}
		}
		assert.True(t, hasHelm, "Helm is a core category and should always be present")
	})

	t.Run("CRD-dependent entries hidden when availableGroups is nil", func(t *testing.T) {
		items := FlattenedResourceTypesFiltered(nil)
		for _, item := range items {
			assert.NotEqual(t, "VerticalPodAutoscaler", item.Kind,
				"VPA should be hidden when availableGroups is nil")
		}
	})

	t.Run("CRD-dependent entries shown when API group available", func(t *testing.T) {
		groups := map[string]bool{"autoscaling.k8s.io/verticalpodautoscalers": true}
		items := FlattenedResourceTypesFiltered(groups)
		hasVPA := false
		for _, item := range items {
			if item.Kind == "VerticalPodAutoscaler" {
				hasVPA = true
				break
			}
		}
		assert.True(t, hasVPA, "VPA should be shown when autoscaling.k8s.io is available")
	})
}

// --- MergeWithCRDs ---

func TestMergeWithCRDs(t *testing.T) { //nolint:gocyclo
	t.Run("no CRDs hides CRD-dependent entries", func(t *testing.T) {
		items := MergeWithCRDs(nil)
		assert.NotEmpty(t, items)
		assert.Equal(t, "Cluster", items[0].Name)
		for _, item := range items {
			assert.NotEqual(t, "VerticalPodAutoscaler", item.Kind,
				"VPA should be hidden when no CRDs are discovered")
		}
	})

	t.Run("VPA shown when autoscaling.k8s.io CRDs present", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "Verticalpodautoscalers", Kind: "VerticalPodAutoscaler", APIGroup: "autoscaling.k8s.io", APIVersion: "v1", Resource: "verticalpodautoscalers", Icon: "⧫", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		hasVPA := false
		for _, item := range items {
			if item.Kind == "VerticalPodAutoscaler" {
				hasVPA = true
				break
			}
		}
		assert.True(t, hasVPA, "VPA should be shown when autoscaling.k8s.io CRDs are discovered")
	})

	t.Run("ArgoCD CRDs enable ArgoCD category", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "Workflows", Kind: "Workflow", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "workflows", Icon: "⎈", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		hasArgo := false
		for _, item := range items {
			if item.Category == "argoproj.io" {
				hasArgo = true
				break
			}
		}
		assert.True(t, hasArgo)
	})

	t.Run("Gateway API CRDs enable gateway.networking.k8s.io category", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "Custom GW", Kind: "CustomGateway", APIGroup: "gateway.networking.k8s.io", APIVersion: "v1", Resource: "customgateways", Icon: "⇶", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		hasGW := false
		for _, item := range items {
			if item.Category == "gateway.networking.k8s.io" {
				hasGW = true
				break
			}
		}
		assert.True(t, hasGW)
	})

	t.Run("Longhorn CRDs enable longhorn.io category", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "Volumes", Kind: "Volume", APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "volumes", Icon: "⬡", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		hasLH := false
		for _, item := range items {
			if item.Category == "longhorn.io" {
				hasLH = true
				break
			}
		}
		assert.True(t, hasLH)
	})

	t.Run("Istio CRDs enable networking.istio.io category", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "VirtualServices", Kind: "VirtualService", APIGroup: "networking.istio.io", APIVersion: "v1", Resource: "virtualservices", Icon: "⎈", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		hasIstio := false
		for _, item := range items {
			if item.Category == "networking.istio.io" {
				hasIstio = true
				break
			}
		}
		assert.True(t, hasIstio)
	})

	t.Run("Istio security CRDs enable security.istio.io category", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "PeerAuthentications", Kind: "PeerAuthentication", APIGroup: "security.istio.io", APIVersion: "v1", Resource: "peerauthentications", Icon: "⎈", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		hasIstio := false
		for _, item := range items {
			if item.Category == "security.istio.io" {
				hasIstio = true
				break
			}
		}
		assert.True(t, hasIstio)
	})

	t.Run("GKE CRDs enable cloud.google.com category", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "BackendConfigs", Kind: "BackendConfig", APIGroup: "cloud.google.com", APIVersion: "v1", Resource: "backendconfigs", Icon: "☁", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		hasGKE := false
		for _, item := range items {
			if item.Category == "cloud.google.com" {
				hasGKE = true
				break
			}
		}
		assert.True(t, hasGKE)
	})

	t.Run("EKS CRDs enable vpcresources.k8s.aws category", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "SecurityGroupPolicies", Kind: "SecurityGroupPolicy", APIGroup: "vpcresources.k8s.aws", APIVersion: "v1beta1", Resource: "securitygrouppolicies", Icon: "☁", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		hasEKS := false
		for _, item := range items {
			if item.Category == "vpcresources.k8s.aws" {
				hasEKS = true
				break
			}
		}
		assert.True(t, hasEKS)
	})

	t.Run("longhorn.io CRDs grouped under longhorn.io category", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "SystemBackups", Kind: "SystemBackup", APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "systembackups", Icon: "⬡", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		for _, item := range items {
			if item.Kind == "SystemBackup" {
				assert.Equal(t, "longhorn.io", item.Category,
					"longhorn.io CRDs should use longhorn.io category")
				return
			}
		}
		t.Fatal("SystemBackup CRD should be present")
	})

	t.Run("Istio CRDs grouped under networking.istio.io category", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "WasmPlugins", Kind: "WasmPlugin", APIGroup: "networking.istio.io", APIVersion: "v1", Resource: "wasmplugins", Icon: "⎈", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		for _, item := range items {
			if item.Kind == "WasmPlugin" {
				assert.Equal(t, "networking.istio.io", item.Category,
					"networking.istio.io CRDs should use networking.istio.io category")
				return
			}
		}
		t.Fatal("WasmPlugin CRD should be present")
	})

	t.Run("duplicate CRDs filtered out", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			// Same as built-in pods.
			{DisplayName: "Pods", Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Icon: "⬤", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		podCount := 0
		for _, item := range items {
			if item.Kind == "Pod" {
				podCount++
			}
		}
		assert.Equal(t, 1, podCount, "duplicate Pod should be filtered out")
	})

	t.Run("argoproj.io CRDs grouped under ArgoCD category", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			// Built-in ArgoCD entries will be filtered as duplicates; this extra
			// argoproj.io CRD should appear under "argoproj.io", not a raw API group.
			{DisplayName: "WorkflowEventBindings", Kind: "WorkflowEventBinding", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "workfloweventbindings", Icon: "⎈", Namespaced: true},
		}
		items := MergeWithCRDs(crds)

		// Verify the discovered CRD has the "argoproj.io" category.
		crdFound := false
		for _, item := range items {
			if item.Kind == "WorkflowEventBinding" {
				crdFound = true
				assert.Equal(t, "argoproj.io", item.Category,
					"argoproj.io CRDs should use ArgoCD category, not raw API group")
				break
			}
		}
		assert.True(t, crdFound, "WorkflowEventBinding CRD should be present")

		// Verify the discovered CRD appears right after the built-in argoproj.io entries,
		// not at the very end of the list.
		lastArgoCDIdx := -1
		crdIdx := -1
		for i, item := range items {
			if item.Category == "argoproj.io" {
				lastArgoCDIdx = i
			}
			if item.Kind == "WorkflowEventBinding" {
				crdIdx = i
			}
		}
		assert.Greater(t, crdIdx, 0, "WorkflowEventBinding should be in the list")
		// The discovered CRD should be among the argoproj.io items (lastArgoCDIdx should
		// be >= crdIdx since it's inserted after built-in entries).
		assert.Equal(t, lastArgoCDIdx, crdIdx,
			"WorkflowEventBinding should be the last argoproj.io item (inserted after built-in entries)")
	})

	t.Run("gateway.networking.k8s.io CRDs grouped under gateway.networking.k8s.io category", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "Custom GW", Kind: "CustomGateway", APIGroup: "gateway.networking.k8s.io", APIVersion: "v1", Resource: "customgateways", Icon: "⇶", Namespaced: true},
		}
		items := MergeWithCRDs(crds)

		for _, item := range items {
			if item.Kind == "CustomGateway" {
				assert.Equal(t, "gateway.networking.k8s.io", item.Category,
					"gateway.networking.k8s.io CRDs should use gateway.networking.k8s.io category")
				break
			}
		}
	})

	t.Run("custom CRDs appended", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "MyResources", Kind: "MyResource", APIGroup: "custom.io", APIVersion: "v1", Resource: "myresources", Icon: "★", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		found := false
		for _, item := range items {
			if item.Kind == "MyResource" {
				found = true
				assert.Equal(t, "custom.io", item.Category)
				break
			}
		}
		assert.True(t, found, "custom CRD should be appended")
	})

	t.Run("deprecated CRD propagates Deprecated flag to item", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{
				DisplayName:    "Ingresses",
				Kind:           "Ingress",
				APIGroup:       "extensions",
				APIVersion:     "v1beta1",
				Resource:       "ingresses",
				Icon:           "⧫",
				Namespaced:     true,
				Deprecated:     true,
				DeprecationMsg: "Ingress extensions/v1beta1 removed in 1.22, use networking.k8s.io/v1",
			},
		}
		items := MergeWithCRDs(crds)
		found := false
		for _, item := range items {
			if item.Kind == "Ingress" && item.Category == "extensions" {
				found = true
				assert.True(t, item.Deprecated, "deprecated CRD item should have Deprecated=true")
				break
			}
		}
		assert.True(t, found, "deprecated CRD should be present")
	})

	t.Run("non-deprecated CRD has Deprecated=false", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{DisplayName: "Widgets", Kind: "Widget", APIGroup: "custom.io", APIVersion: "v1", Resource: "widgets", Icon: "⧫", Namespaced: true},
		}
		items := MergeWithCRDs(crds)
		for _, item := range items {
			if item.Kind == "Widget" {
				assert.False(t, item.Deprecated, "non-deprecated CRD should have Deprecated=false")
				return
			}
		}
		t.Fatal("Widget CRD should be present")
	})
}

// --- FindResourceTypeByKind ---

func TestFindResourceTypeByKind(t *testing.T) {
	t.Run("found built-in", func(t *testing.T) {
		rt, ok := FindResourceTypeByKind("Pod", nil)
		assert.True(t, ok)
		assert.Equal(t, "Pod", rt.Kind)
		assert.Equal(t, "pods", rt.Resource)
	})

	t.Run("found in CRDs", func(t *testing.T) {
		crds := []ResourceTypeEntry{
			{Kind: "MyThing", DisplayName: "MyThings", Resource: "mythings"},
		}
		rt, ok := FindResourceTypeByKind("MyThing", crds)
		assert.True(t, ok)
		assert.Equal(t, "MyThing", rt.Kind)
	})

	t.Run("not found", func(t *testing.T) {
		_, ok := FindResourceTypeByKind("NonExistent", nil)
		assert.False(t, ok)
	})
}

// --- FindResourceType ---

func TestFindResourceType(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		rt, ok := FindResourceType("/v1/pods")
		assert.True(t, ok)
		assert.Equal(t, "Pod", rt.Kind)
	})

	t.Run("not found", func(t *testing.T) {
		_, ok := FindResourceType("nonexistent/v1/things")
		assert.False(t, ok)
	})
}

// --- FindResourceTypeIn ---

func TestFindResourceTypeIn(t *testing.T) {
	t.Run("found in built-in", func(t *testing.T) {
		rt, ok := FindResourceTypeIn("apps/v1/deployments", nil)
		assert.True(t, ok)
		assert.Equal(t, "Deployment", rt.Kind)
	})

	t.Run("found in additional", func(t *testing.T) {
		additional := []ResourceTypeEntry{
			{Kind: "Custom", APIGroup: "test.io", APIVersion: "v1", Resource: "customs"},
		}
		rt, ok := FindResourceTypeIn("test.io/v1/customs", additional)
		assert.True(t, ok)
		assert.Equal(t, "Custom", rt.Kind)
	})

	t.Run("not found", func(t *testing.T) {
		_, ok := FindResourceTypeIn("nope/v1/nothing", nil)
		assert.False(t, ok)
	})

	t.Run("built-in enriched with PrinterColumns from discovered CRDs", func(t *testing.T) {
		// Simulate a discovered CRD that matches a built-in resource type (e.g. deployments)
		// and carries PrinterColumns from CRD discovery.
		additional := []ResourceTypeEntry{
			{
				Kind:       "Deployment",
				APIGroup:   "apps",
				APIVersion: "v1",
				Resource:   "deployments",
				PrinterColumns: []PrinterColumn{
					{Name: "Ready", Type: "string", JSONPath: ".status.readyReplicas"},
					{Name: "Available", Type: "integer", JSONPath: ".status.availableReplicas"},
				},
			},
		}
		rt, ok := FindResourceTypeIn("apps/v1/deployments", additional)
		require.True(t, ok)
		assert.Equal(t, "Deployment", rt.Kind)
		// The built-in match should be enriched with PrinterColumns from discovered CRDs.
		assert.Len(t, rt.PrinterColumns, 2)
		assert.Equal(t, "Ready", rt.PrinterColumns[0].Name)
		assert.Equal(t, "Available", rt.PrinterColumns[1].Name)
	})

	t.Run("built-in without matching CRD has no PrinterColumns", func(t *testing.T) {
		rt, ok := FindResourceTypeIn("apps/v1/deployments", nil)
		require.True(t, ok)
		assert.Equal(t, "Deployment", rt.Kind)
		assert.Empty(t, rt.PrinterColumns)
	})
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

// --- Cluster group is first in TopLevelResourceTypes ---

func TestClusterGroupIsFirst(t *testing.T) {
	cats := TopLevelResourceTypes()
	require.NotEmpty(t, cats)
	assert.Equal(t, "Cluster", cats[0].Name,
		"Cluster group should be the first category (shown below Dashboards)")
}

func TestClusterGroupOrder(t *testing.T) {
	cats := TopLevelResourceTypes()
	require.Equal(t, "Cluster", cats[0].Name)
	types := cats[0].Types
	require.Len(t, types, 3)
	assert.Equal(t, "Node", types[0].Kind, "first should be Nodes")
	assert.Equal(t, "Namespace", types[1].Kind, "second should be Namespaces")
	assert.Equal(t, "Event", types[2].Kind, "third should be Events")
}

func TestClusterGroupContents(t *testing.T) {
	cats := TopLevelResourceTypes()
	var cluster *ResourceCategory
	for i := range cats {
		if cats[i].Name == "Cluster" {
			cluster = &cats[i]
			break
		}
	}
	require.NotNil(t, cluster, "Cluster group must exist")

	kinds := make(map[string]bool)
	for _, rt := range cluster.Types {
		kinds[rt.Kind] = true
	}
	assert.True(t, kinds["Namespace"], "Cluster group should contain Namespaces")
	assert.True(t, kinds["Event"], "Cluster group should contain Events")
	assert.True(t, kinds["Node"], "Cluster group should contain Nodes")
	assert.False(t, kinds["CustomResourceDefinition"],
		"CRDs should NOT be in Cluster group (moved to API and CRDs)")
	assert.False(t, kinds["APIService"],
		"API Services should NOT be in Cluster group (moved to API and CRDs)")
}

// --- API and CRDs group exists after Helm ---

func TestAPICRDsGroupExistsAfterHelm(t *testing.T) {
	cats := TopLevelResourceTypes()
	helmIdx := -1
	apiIdx := -1
	for i, cat := range cats {
		if cat.Name == "Helm" {
			helmIdx = i
		}
		if cat.Name == "API and CRDs" {
			apiIdx = i
		}
	}
	require.NotEqual(t, -1, helmIdx, "Helm group must exist")
	require.NotEqual(t, -1, apiIdx, "API and CRDs group must exist")
	assert.Equal(t, helmIdx+1, apiIdx,
		"API and CRDs group should be immediately after Helm")
}

func TestAPICRDsGroupContents(t *testing.T) {
	cats := TopLevelResourceTypes()
	var apiCRDs *ResourceCategory
	for i := range cats {
		if cats[i].Name == "API and CRDs" {
			apiCRDs = &cats[i]
			break
		}
	}
	require.NotNil(t, apiCRDs, "API and CRDs group must exist")

	kinds := make(map[string]bool)
	for _, rt := range apiCRDs.Types {
		kinds[rt.Kind] = true
	}
	assert.True(t, kinds["APIService"], "API and CRDs should contain API Services")
	assert.True(t, kinds["CustomResourceDefinition"], "API and CRDs should contain CRDs")
}

func TestAPICRDsIsCoreCategory(t *testing.T) {
	assert.True(t, IsCoreCategory("API and CRDs"),
		"API and CRDs should be a core category")
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

// --- Security virtual API group and dynamic category ---

func TestSecurityVirtualAPIGroupConstant(t *testing.T) {
	assert.Equal(t, "_security", SecurityVirtualAPIGroup)
}

func TestSecuritySourcesFnNilReturnsNothing(t *testing.T) {
	prev := SecuritySourcesFn
	t.Cleanup(func() { SecuritySourcesFn = prev })
	SecuritySourcesFn = nil

	cats := TopLevelResourceTypes()
	var securityCat *ResourceCategory
	for i := range cats {
		if cats[i].Name == "Security" {
			securityCat = &cats[i]
			break
		}
	}
	require.NotNil(t, securityCat, "Security category must exist even when hook is nil")
	assert.Empty(t, securityCat.Types)
}

func TestSecuritySourcesFnReturnsEntries(t *testing.T) {
	prev := SecuritySourcesFn
	t.Cleanup(func() { SecuritySourcesFn = prev })
	SecuritySourcesFn = func() []SecuritySourceEntry {
		return []SecuritySourceEntry{
			{DisplayName: "Trivy", SourceName: "trivy-operator", Icon: "◈", Count: 5},
			{DisplayName: "Heuristic", SourceName: "heuristic", Icon: "◉", Count: 12},
		}
	}

	cats := TopLevelResourceTypes()
	var securityCat *ResourceCategory
	for i := range cats {
		if cats[i].Name == "Security" {
			securityCat = &cats[i]
			break
		}
	}
	require.NotNil(t, securityCat)
	require.Len(t, securityCat.Types, 2)

	assert.Equal(t, "Trivy (5)", securityCat.Types[0].DisplayName)
	assert.Equal(t, "__security_trivy-operator__", securityCat.Types[0].Kind)
	assert.Equal(t, SecurityVirtualAPIGroup, securityCat.Types[0].APIGroup)
	assert.Equal(t, "findings", securityCat.Types[0].Resource)
	assert.False(t, securityCat.Types[0].Namespaced)

	assert.Equal(t, "Heuristic (12)", securityCat.Types[1].DisplayName)
	assert.Equal(t, "__security_heuristic__", securityCat.Types[1].Kind)
}

func TestSecurityIsCoreCategoryAlwaysShown(t *testing.T) {
	assert.True(t, IsCoreCategory("Security"), "Security must be a core category")
}
