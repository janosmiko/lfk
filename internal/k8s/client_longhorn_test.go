package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/model"
)

var longhornNodeRT = model.ResourceTypeEntry{
	APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "nodes", Kind: "Node", Namespaced: true,
}

var longhornNodeGVR = schema.GroupVersionResource{Group: "longhorn.io", Version: "v1beta2", Resource: "nodes"}

func newLonghornNode(name string, allowScheduling, evictionRequested bool) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "longhorn.io/v1beta2",
			"kind":       "Node",
			"metadata": map[string]any{
				"name":      name,
				"namespace": "longhorn-system",
			},
			"spec": map[string]any{
				"allowScheduling":   allowScheduling,
				"evictionRequested": evictionRequested,
			},
		},
	}
}

func newLonghornReplica(name, nodeID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "longhorn.io/v1beta2",
			"kind":       "Replica",
			"metadata": map[string]any{
				"name":      name,
				"namespace": "longhorn-system",
			},
			"spec": map[string]any{"nodeID": nodeID},
		},
	}
}

// TestGetResources_LonghornNode_ReplicaCountColumn verifies the Longhorn Nodes
// list gets a REPLICAS column counting longhorn.io/replicas by spec.nodeID.
func TestGetResources_LonghornNode_ReplicaCountColumn(t *testing.T) {
	dc := newFakeDynClient(
		newLonghornNode("node-a", true, false),
		newLonghornNode("node-b", true, false),
		newLonghornNode("node-c", true, false), // no replicas -> "0"
		newLonghornReplica("r1", "node-a"),
		newLonghornReplica("r2", "node-a"),
		newLonghornReplica("r3", "node-b"),
	)
	c := newFakeClient(nil, dc)

	items, err := c.GetResources(t.Context(), "", "longhorn-system", longhornNodeRT)
	require.NoError(t, err)

	got := map[string]string{}
	for _, it := range items {
		for _, kv := range it.Columns {
			if kv.Key == "REPLICAS" {
				got[it.Name] = kv.Value
			}
		}
	}
	assert.Equal(t, "2", got["node-a"])
	assert.Equal(t, "1", got["node-b"])
	assert.Equal(t, "0", got["node-c"])
}

// TestWithLonghornReplicaCounts_DoesNotMutateInput guards against the
// informer-cache path accumulating duplicate REPLICAS columns: the function
// must not mutate the (memoized) input items, and repeated calls must each
// yield exactly one REPLICAS column.
func TestWithLonghornReplicaCounts_DoesNotMutateInput(t *testing.T) {
	dc := newFakeDynClient(newLonghornReplica("r1", "node-a"))
	c := newFakeClient(nil, dc)
	items := []model.Item{{Name: "node-a"}}

	out1 := c.withLonghornReplicaCounts(t.Context(), "", "longhorn-system", longhornNodeRT, items)
	out2 := c.withLonghornReplicaCounts(t.Context(), "", "longhorn-system", longhornNodeRT, items)

	assert.Empty(t, items[0].Columns, "input items must not be mutated")
	for _, out := range [][]model.Item{out1, out2} {
		count := 0
		for _, kv := range out[0].Columns {
			if kv.Key == LonghornReplicaColumn {
				count++
			}
		}
		assert.Equal(t, 1, count, "each call must produce exactly one REPLICAS column")
	}
}

// TestGetResources_NonLonghorn_NoReplicaColumn ensures the REPLICAS column is
// scoped to Longhorn nodes and does not leak onto other resource types.
func TestGetResources_NonLonghorn_NoReplicaColumn(t *testing.T) {
	pod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{"name": "p1", "namespace": "default"},
	}}
	dc := newFakeDynClient(pod)
	c := newFakeClient(nil, dc)

	rt := model.ResourceTypeEntry{APIGroup: "", APIVersion: "v1", Resource: "pods", Kind: "Pod", Namespaced: true}
	items, err := c.GetResources(t.Context(), "", "default", rt)
	require.NoError(t, err)
	require.Len(t, items, 1)
	for _, kv := range items[0].Columns {
		assert.NotEqual(t, "REPLICAS", kv.Key)
	}
}

// TestForceDeleteLonghornNode verifies the node is removed. Scheduling is
// disabled first (the validating webhook rejects deletion of a still
// schedulable node), then the node is deleted.
func TestForceDeleteLonghornNode(t *testing.T) {
	dc := newFakeDynClient(newLonghornNode("node1", true, false))
	c := newFakeClient(nil, dc)

	err := c.ForceDeleteLonghornNode(t.Context(), "", "longhorn-system", longhornNodeRT, "node1")
	require.NoError(t, err)

	_, getErr := dc.Resource(longhornNodeGVR).Namespace("longhorn-system").
		Get(t.Context(), "node1", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(getErr), "node should be deleted, got %v", getErr)
}

// TestSetLonghornNodeEviction_Enable verifies that requesting eviction sets
// both evictionRequested=true and allowScheduling=false (Longhorn refuses to
// evict a node that is still schedulable).
func TestSetLonghornNodeEviction_Enable(t *testing.T) {
	dc := newFakeDynClient(newLonghornNode("node1", true, false))
	c := newFakeClient(nil, dc)

	err := c.SetLonghornNodeEviction(t.Context(), "", "longhorn-system", longhornNodeRT, "node1", true)
	require.NoError(t, err)

	obj, getErr := dc.Resource(longhornNodeGVR).Namespace("longhorn-system").
		Get(t.Context(), "node1", metav1.GetOptions{})
	require.NoError(t, getErr)

	evict, evictFound, evictErr := unstructured.NestedBool(obj.Object, "spec", "evictionRequested")
	sched, schedFound, schedErr := unstructured.NestedBool(obj.Object, "spec", "allowScheduling")
	require.NoError(t, evictErr)
	require.NoError(t, schedErr)
	require.True(t, evictFound, "spec.evictionRequested should exist")
	require.True(t, schedFound, "spec.allowScheduling should exist")
	assert.True(t, evict, "evictionRequested must be true")
	assert.False(t, sched, "allowScheduling must be false during eviction")
}

// TestSetLonghornNodeEviction_Cancel verifies that cancelling eviction clears
// evictionRequested without re-enabling scheduling.
func TestSetLonghornNodeEviction_Cancel(t *testing.T) {
	dc := newFakeDynClient(newLonghornNode("node1", false, true))
	c := newFakeClient(nil, dc)

	err := c.SetLonghornNodeEviction(t.Context(), "", "longhorn-system", longhornNodeRT, "node1", false)
	require.NoError(t, err)

	obj, getErr := dc.Resource(longhornNodeGVR).Namespace("longhorn-system").
		Get(t.Context(), "node1", metav1.GetOptions{})
	require.NoError(t, getErr)

	evict, evictFound, evictErr := unstructured.NestedBool(obj.Object, "spec", "evictionRequested")
	sched, schedFound, schedErr := unstructured.NestedBool(obj.Object, "spec", "allowScheduling")
	require.NoError(t, evictErr)
	require.NoError(t, schedErr)
	require.True(t, evictFound, "spec.evictionRequested should exist")
	require.True(t, schedFound, "spec.allowScheduling should exist")
	assert.False(t, evict, "evictionRequested must be false after cancel")
	assert.False(t, sched, "allowScheduling must remain disabled after cancel")
}
