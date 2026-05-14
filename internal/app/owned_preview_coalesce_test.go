package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	clientfake "k8s.io/client-go/kubernetes/fake"
)

// TestDeploymentChildren_NotCoalescedByMainListRefresh exercises the scheduler
// coalescing path that previously dropped owned-children loads. Both the main
// resource list ("List Deployments") and the owned-pods preview ("List
// Deployment children") run with Kind=ResourceList against the same
// context+namespace+gen, so before the Sig fix they shared a coalesce
// signature: the watch-tick refresh of the main list would silently drop the
// queued children load (and vice versa), leaving the right-pane children pane
// stuck empty.
//
// This test verifies the chain at LevelResources hovering a Deployment runs to
// completion: loadOwned returns an ownedLoadedMsg with the pod, and feeding it
// back through updateOwnedLoaded populates rightItems so renderRightResources
// renders the split preview.
func TestDeploymentChildren_NotCoalescedByMainListRefresh(t *testing.T) {
	rs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "ReplicaSet",
		"metadata": map[string]any{
			"name":      "argo-workflows-server-abc",
			"namespace": "argo",
			"ownerReferences": []any{
				map[string]any{"kind": "Deployment", "name": "argo-workflows-server"},
			},
		},
	}}
	pod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":              "argo-workflows-server-abc-xyz",
			"namespace":         "argo",
			"creationTimestamp": "2026-01-01T00:00:00Z",
			"ownerReferences": []any{
				map[string]any{"kind": "ReplicaSet", "name": "argo-workflows-server-abc"},
			},
		},
		"spec":   map[string]any{"containers": []any{map[string]any{"name": "main"}}},
		"status": map[string]any{"phase": "Running"},
	}}

	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "apps", Version: "v1", Resource: "replicasets"}:          "ReplicaSetList",
		{Group: "", Version: "v1", Resource: "pods"}:                     "PodList",
		{Group: "", Version: "v1", Resource: "events"}:                   "EventList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:  "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}: "NodeMetricsList",
	}
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, rs, pod)
	cs := clientfake.NewClientset()

	m := baseModelCov()
	m.client = k8s.NewTestClient(cs, dyn)
	m.nav.Context = "test-ctx"
	m.namespace = ""
	m.allNamespaces = true

	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Deployment",
		Resource:   "deployments",
		APIGroup:   "apps",
		APIVersion: "v1",
		Namespaced: true,
	}
	m.middleItems = []model.Item{
		{Name: "argo-workflows-server", Namespace: "argo", Kind: "Deployment"},
	}
	m.setCursor(0)
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	require.True(t, m.resourceTypeHasChildren())

	// Run loadOwned and confirm it produces the pod (not nil, not ErrCoalesced).
	ownedCmd := m.loadOwned(true)
	require.NotNil(t, ownedCmd)

	msg := ownedCmd()
	require.NotNil(t, msg, "ownedCmd returned nil — the scheduler coalesced or context-switched this submission")

	owned, ok := msg.(ownedLoadedMsg)
	require.True(t, ok, "expected ownedLoadedMsg, got %T", msg)
	require.NoError(t, owned.err)
	require.Len(t, owned.items, 1)
	assert.Equal(t, "argo-workflows-server-abc-xyz", owned.items[0].Name)

	newModel, _ := m.updateOwnedLoaded(owned)
	m2 := newModel.(Model)
	require.Len(t, m2.rightItems, 1)
}
