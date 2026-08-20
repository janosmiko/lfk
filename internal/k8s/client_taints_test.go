package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/model"
)

func taintedNode() *corev1.Node {
	added := metav1.NewTime(time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))
	return &corev1.Node{
		Name: "worker-1",
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
				{Key: "maintenance", Effect: corev1.TaintEffectNoExecute, TimeAdded: &added},
			},
		},
	}
}

func TestGetNodeTaints(t *testing.T) {
	cs := k8sfake.NewClientset(taintedNode())
	c := newFakeClient(cs, nil)

	taints, err := c.GetNodeTaints(t.Context(), "", "worker-1")
	require.NoError(t, err)
	assert.Equal(t, []model.Taint{
		{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"},
		{Key: "maintenance", Effect: "NoExecute"},
	}, taints)
}

func TestGetNodeTaints_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)
	_, err := c.GetNodeTaints(t.Context(), "", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting node")
}

func TestUpdateNodeTaints_RemoveAndAdd(t *testing.T) {
	cs := k8sfake.NewClientset(taintedNode())
	c := newFakeClient(cs, nil)

	err := c.UpdateNodeTaints(t.Context(), "", "worker-1",
		[]model.Taint{{Key: "dedicated", Effect: "NoSchedule"}},
		[]model.Taint{{Key: "team", Value: "ml", Effect: "PreferNoSchedule"}})
	require.NoError(t, err)

	node, err := cs.CoreV1().Nodes().Get(t.Context(), "worker-1", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, node.Spec.Taints, 2)
	assert.Equal(t, "maintenance", node.Spec.Taints[0].Key)
	assert.NotNil(t, node.Spec.Taints[0].TimeAdded, "kept taints preserve TimeAdded")
	assert.Equal(t, corev1.Taint{Key: "team", Value: "ml", Effect: corev1.TaintEffectPreferNoSchedule},
		node.Spec.Taints[1])
}

func TestUpdateNodeTaints_ValueEditViaRemoveAndReAdd(t *testing.T) {
	// The editor's value-edit workflow: remove dedicated=gpu:NoSchedule,
	// re-add dedicated=cpu:NoSchedule. The NEW value must win — reusing
	// the original taint struct here would silently write "gpu" back.
	cs := k8sfake.NewClientset(taintedNode())
	c := newFakeClient(cs, nil)

	err := c.UpdateNodeTaints(t.Context(), "", "worker-1",
		[]model.Taint{{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"}},
		[]model.Taint{{Key: "dedicated", Value: "cpu", Effect: "NoSchedule"}})
	require.NoError(t, err)

	node, err := cs.CoreV1().Nodes().Get(t.Context(), "worker-1", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, node.Spec.Taints, 2)
	assert.Equal(t, "cpu", node.Spec.Taints[1].Value, "re-added taint carries the new value")
}

func TestUpdateNodeTaints_RemoveAll(t *testing.T) {
	cs := k8sfake.NewClientset(taintedNode())
	c := newFakeClient(cs, nil)

	err := c.UpdateNodeTaints(t.Context(), "", "worker-1",
		[]model.Taint{
			{Key: "dedicated", Effect: "NoSchedule"},
			{Key: "maintenance", Effect: "NoExecute"},
		}, nil)
	require.NoError(t, err)

	node, err := cs.CoreV1().Nodes().Get(t.Context(), "worker-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, node.Spec.Taints)
}

func TestUpdateNodeTaints_NotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)
	err := c.UpdateNodeTaints(t.Context(), "", "missing", nil,
		[]model.Taint{{Key: "a", Effect: "NoSchedule"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting node")
}
