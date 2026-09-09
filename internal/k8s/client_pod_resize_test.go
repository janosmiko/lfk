package k8s

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	k8stesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/model"
)

func TestResizePodResources_PatchBody(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":      "my-pod",
				"namespace": "default",
			},
		},
	}
	dc := newFakeDynClient(pod)

	var captured k8stesting.PatchAction
	dc.PrependReactor("patch", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		captured = action.(k8stesting.PatchAction)
		return true, pod, nil
	})
	c := newFakeClient(nil, dc)

	specs := []model.ContainerResources{
		{Name: "web", CPURequest: "200m", CPULimit: "1", MemRequest: "256Mi"},
	}
	err := c.ResizePodResources(t.Context(), "", "default", "my-pod", specs)
	require.NoError(t, err)
	require.NotNil(t, captured)

	assert.Equal(t, "resize", captured.GetSubresource())
	assert.Equal(t, k8stypes.StrategicMergePatchType, captured.GetPatchType())

	var body map[string]any
	require.NoError(t, json.Unmarshal(captured.GetPatch(), &body))
	expected := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name": "web",
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu": "1",
						},
						"requests": map[string]any{
							"cpu":    "200m",
							"memory": "256Mi",
						},
					},
				},
			},
		},
	}
	assert.Equal(t, expected, body)
}

func TestResizePodResources_OmitsEmptyFields(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":      "my-pod",
				"namespace": "default",
			},
		},
	}
	dc := newFakeDynClient(pod)

	var captured k8stesting.PatchAction
	dc.PrependReactor("patch", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		captured = action.(k8stesting.PatchAction)
		return true, pod, nil
	})
	c := newFakeClient(nil, dc)

	specs := []model.ContainerResources{
		{Name: "web", CPURequest: "200m"},
	}
	err := c.ResizePodResources(t.Context(), "", "default", "my-pod", specs)
	require.NoError(t, err)
	require.NotNil(t, captured)

	var body map[string]any
	require.NoError(t, json.Unmarshal(captured.GetPatch(), &body))
	expected := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name": "web",
					"resources": map[string]any{
						"requests": map[string]any{
							"cpu": "200m",
						},
					},
				},
			},
		},
	}
	assert.Equal(t, expected, body)
}
