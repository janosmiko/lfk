package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/model"
)

var draGVRs = map[schema.GroupVersionResource]string{
	{Group: "resource.k8s.io", Version: "v1", Resource: "resourceclaims"}:         "ResourceClaimList",
	{Group: "resource.k8s.io", Version: "v1", Resource: "resourceclaimtemplates"}: "ResourceClaimTemplateList",
	{Group: "resource.k8s.io", Version: "v1", Resource: "resourceslices"}:         "ResourceSliceList",
	{Group: "resource.k8s.io", Version: "v1", Resource: "deviceclasses"}:          "DeviceClassList",
}

func newResourceClaim(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "resource.k8s.io/v1",
		"kind":       "ResourceClaim",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"status": map[string]any{
			"allocation": map[string]any{
				"devices": map[string]any{
					"results": []any{
						map[string]any{"driver": "gpu.example.com", "pool": "pool-a", "device": "gpu-0"},
					},
				},
			},
		},
	}}
}

func newResourceClaimTemplate(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "resource.k8s.io/v1",
		"kind":       "ResourceClaimTemplate",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"spec": map[string]any{
			"spec": map[string]any{
				"devices": map[string]any{
					"requests": []any{map[string]any{"name": "gpu"}},
				},
			},
		},
	}}
}

func newResourceSlice(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "resource.k8s.io/v1",
		"kind":       "ResourceSlice",
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"driver":   "gpu.example.com",
			"nodeName": "node-1",
			"devices":  []any{map[string]any{"name": "gpu-0"}},
		},
	}}
}

func newDeviceClass(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "resource.k8s.io/v1",
		"kind":       "DeviceClass",
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"selectors": []any{map[string]any{"cel": map[string]any{"expression": "true"}}},
		},
	}}
}

func TestGetResources_ResourceClaim(t *testing.T) {
	dc := newFakeDynClientWith(draGVRs, newResourceClaim("gpu-claim", "default"))
	c := newFakeClient(nil, dc)

	rt := model.ResourceTypeEntry{
		APIGroup: "resource.k8s.io", APIVersion: "v1", Resource: "resourceclaims", Kind: "ResourceClaim", Namespaced: true,
	}
	items, err := c.GetResources(t.Context(), "", "default", rt)
	require.NoError(t, err)
	require.Len(t, items, 1)

	colMap := columnsToMap(items[0].Columns)
	assert.Equal(t, "gpu.example.com", colMap["Driver"])
	assert.Equal(t, "pool-a", colMap["Pool"])
	assert.Equal(t, "gpu-0", colMap["Device"])
}

func TestGetResources_ResourceClaimTemplate(t *testing.T) {
	dc := newFakeDynClientWith(draGVRs, newResourceClaimTemplate("gpu-template", "default"))
	c := newFakeClient(nil, dc)

	rt := model.ResourceTypeEntry{
		APIGroup: "resource.k8s.io", APIVersion: "v1", Resource: "resourceclaimtemplates", Kind: "ResourceClaimTemplate", Namespaced: true,
	}
	items, err := c.GetResources(t.Context(), "", "default", rt)
	require.NoError(t, err)
	require.Len(t, items, 1)

	colMap := columnsToMap(items[0].Columns)
	assert.Equal(t, "1", colMap["Devices"])
}

func TestGetResources_ResourceSlice(t *testing.T) {
	dc := newFakeDynClientWith(draGVRs, newResourceSlice("node-1-slice"))
	c := newFakeClient(nil, dc)

	rt := model.ResourceTypeEntry{
		APIGroup: "resource.k8s.io", APIVersion: "v1", Resource: "resourceslices", Kind: "ResourceSlice", Namespaced: false,
	}
	items, err := c.GetResources(t.Context(), "", "", rt)
	require.NoError(t, err)
	require.Len(t, items, 1)

	colMap := columnsToMap(items[0].Columns)
	assert.Equal(t, "gpu.example.com", colMap["Driver"])
	assert.Equal(t, "node-1", colMap["Node or Pool"])
	assert.Equal(t, "1", colMap["Devices"])
}

func TestGetResources_DeviceClass(t *testing.T) {
	dc := newFakeDynClientWith(draGVRs, newDeviceClass("gpu-class"))
	c := newFakeClient(nil, dc)

	rt := model.ResourceTypeEntry{
		APIGroup: "resource.k8s.io", APIVersion: "v1", Resource: "deviceclasses", Kind: "DeviceClass", Namespaced: false,
	}
	items, err := c.GetResources(t.Context(), "", "", rt)
	require.NoError(t, err)
	require.Len(t, items, 1)

	colMap := columnsToMap(items[0].Columns)
	assert.Equal(t, "1", colMap["Selectors"])
}

func TestGetResources_Pod_ResourceClaimsColumn(t *testing.T) {
	pod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": "gpu-pod", "namespace": "default"},
		"spec": map[string]any{
			"resourceClaims": []any{
				map[string]any{"name": "gpu", "resourceClaimTemplateName": "gpu-template"},
			},
		},
		"status": map[string]any{
			"resourceClaimStatuses": []any{
				map[string]any{"name": "gpu", "resourceClaimName": "gpu-pod-gpu"},
			},
		},
	}}
	dc := newFakeDynClient(pod)
	c := newFakeClient(nil, dc)

	rt := model.ResourceTypeEntry{APIGroup: "", APIVersion: "v1", Resource: "pods", Kind: "Pod", Namespaced: true}
	items, err := c.GetResources(t.Context(), "", "default", rt)
	require.NoError(t, err)
	require.Len(t, items, 1)

	colMap := columnsToMap(items[0].Columns)
	assert.Equal(t, "gpu-pod-gpu", colMap["Resource Claims"])
}
