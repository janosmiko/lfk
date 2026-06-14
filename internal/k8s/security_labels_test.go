package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestResourceLabels(t *testing.T) {
	dep := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "cilium",
			"namespace": "kube-system",
			"labels":    map[string]any{"k8s-app": "cilium"},
		},
	}}
	c := newFakeClient(nil, newFakeDynClientWith(nil, dep))

	t.Run("resolves workload labels", func(t *testing.T) {
		got := c.ResourceLabels(t.Context(), "ctx", "kube-system", "Deployment", "cilium")
		assert.Equal(t, map[string]string{"k8s-app": "cilium"}, got)
	})

	t.Run("unmapped kind -> nil", func(t *testing.T) {
		assert.Nil(t, c.ResourceLabels(t.Context(), "ctx", "kube-system", "ClusterRole", "admin"))
	})

	t.Run("empty name -> nil", func(t *testing.T) {
		assert.Nil(t, c.ResourceLabels(t.Context(), "ctx", "kube-system", "Deployment", ""))
	})

	t.Run("not found -> nil", func(t *testing.T) {
		assert.Nil(t, c.ResourceLabels(t.Context(), "ctx", "kube-system", "Deployment", "ghost"))
	})

	t.Run("empty namespace -> nil (mapped kinds are namespaced)", func(t *testing.T) {
		assert.Nil(t, c.ResourceLabels(t.Context(), "ctx", "", "Deployment", "cilium"))
	})
}
