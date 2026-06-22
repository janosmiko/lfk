package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/model"
)

var hpaGVR = schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"}

func hpaResourceType() model.ResourceTypeEntry {
	return model.ResourceTypeEntry{APIGroup: "autoscaling", APIVersion: "v2", Resource: "horizontalpodautoscalers", Namespaced: true}
}

func hpaUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "autoscaling/v2",
		"kind":       "HorizontalPodAutoscaler",
		"metadata":   map[string]any{"name": "web", "namespace": "default"},
		"spec": map[string]any{
			"minReplicas":    int64(2),
			"maxReplicas":    int64(5),
			"scaleTargetRef": map[string]any{"kind": "Deployment", "name": "web"},
		},
	}}
}

func TestPatchHPAScale(t *testing.T) {
	dc := newFakeDynClientWith(map[schema.GroupVersionResource]string{hpaGVR: "HorizontalPodAutoscalerList"}, hpaUnstructured())
	c := newFakeClient(nil, dc)

	err := c.PatchHPAScale("", "default", hpaResourceType(), "web", 3, 10)
	require.NoError(t, err)

	got, err := dc.Resource(hpaGVR).Namespace("default").Get(t.Context(), "web", metav1.GetOptions{})
	require.NoError(t, err)
	spec, ok := got.Object["spec"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 3, spec["minReplicas"])
	assert.EqualValues(t, 10, spec["maxReplicas"])
}

func TestPatchHPAScale_InvalidContext(t *testing.T) {
	c := newTestClient(t)
	err := c.PatchHPAScale("nonexistent-context", "default", hpaResourceType(), "web", 1, 3)
	require.Error(t, err)
}
