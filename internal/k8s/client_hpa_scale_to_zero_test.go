package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/model"
)

// At 0 replicas Kubernetes resolves minAvailable to 0, so Available stays True.
func TestBuildResourceItem_DeploymentScaledToZero_NotFailed(t *testing.T) {
	c := &Client{}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": "web", "namespace": "default"},
		"spec":       map[string]any{"replicas": int64(0)},
		"status": map[string]any{
			"replicas":      int64(0),
			"readyReplicas": int64(0),
			"conditions": []any{
				map[string]any{"type": "Progressing", "status": "True", "reason": "NewReplicaSetAvailable"},
				map[string]any{"type": "Available", "status": "True", "reason": "MinimumReplicasAvailable"},
			},
		},
	}}
	rt := &model.ResourceTypeEntry{Kind: "Deployment", Namespaced: true}

	ti := c.buildResourceItem(obj, rt)

	assert.Equal(t, "Available", ti.Status)
	assert.Equal(t, "0/0", ti.Ready)
}

// StatefulSets do not populate status.conditions in normal operation.
func TestBuildResourceItem_StatefulSetScaledToZero_NotFailed(t *testing.T) {
	c := &Client{}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "StatefulSet",
		"metadata":   map[string]any{"name": "db", "namespace": "default"},
		"spec":       map[string]any{"replicas": int64(0)},
		"status": map[string]any{
			"replicas":      int64(0),
			"readyReplicas": int64(0),
		},
	}}
	rt := &model.ResourceTypeEntry{Kind: "StatefulSet", Namespaced: true}

	ti := c.buildResourceItem(obj, rt)

	assert.Empty(t, ti.Status)
	assert.Equal(t, "0/0", ti.Ready)
}

func TestBuildResourceItem_ReplicaSetScaledToZero_NotFailed(t *testing.T) {
	c := &Client{}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "ReplicaSet",
		"metadata":   map[string]any{"name": "web-abc123", "namespace": "default"},
		"spec":       map[string]any{"replicas": int64(0)},
		"status": map[string]any{
			"replicas":      int64(0),
			"readyReplicas": int64(0),
		},
	}}
	rt := &model.ResourceTypeEntry{Kind: "ReplicaSet", Namespaced: true}

	ti := c.buildResourceItem(obj, rt)

	assert.Empty(t, ti.Status)
	assert.Equal(t, "0/0", ti.Ready)
}
