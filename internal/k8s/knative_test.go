package k8s

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestActivateKnativeRevision(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		knativeRevisionGVR: "RevisionList",
		knativeServiceGVR:  "ServiceList",
	}
	rev := &unstructured.Unstructured{}
	rev.SetUnstructuredContent(map[string]any{
		"apiVersion": "serving.knative.dev/v1",
		"kind":       "Revision",
		"metadata": map[string]any{
			"name":      "my-svc-00002",
			"namespace": "default",
			"labels":    map[string]any{knativeServiceLabel: "my-svc"},
		},
	})
	svc := &unstructured.Unstructured{}
	svc.SetUnstructuredContent(map[string]any{
		"apiVersion": "serving.knative.dev/v1",
		"kind":       "Service",
		"metadata":   map[string]any{"name": "my-svc", "namespace": "default"},
		"spec": map[string]any{
			"traffic": []any{
				map[string]any{"revisionName": "my-svc-00001", "percent": int64(50)},
				map[string]any{"revisionName": "my-svc-00002", "percent": int64(50)},
			},
		},
	})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, rev, svc)
	c := newFakeClient(nil, dyn)

	parent, err := c.ActivateKnativeRevision("test-ctx", "default", "my-svc-00002")
	require.NoError(t, err)
	assert.Equal(t, "my-svc", parent, "must return the resolved parent Service name")

	// Verify the Service's spec.traffic was replaced with a single
	// 100% entry pointing at the activated Revision. JSON merge-patch
	// treats arrays as atomic, which is the intended semantic.
	got, err := dyn.Resource(knativeServiceGVR).Namespace("default").Get(t.Context(), "my-svc", metav1.GetOptions{})
	require.NoError(t, err)
	traffic, found, err := unstructured.NestedSlice(got.Object, "spec", "traffic")
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, traffic, 1, "traffic should collapse to a single Revision entry")
	entry, ok := traffic[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "my-svc-00002", entry["revisionName"])
	// JSON unmarshals numeric literals into float64 through the fake
	// client; compare via float to avoid spurious type-mismatch failures.
	pct, _ := json.Marshal(entry["percent"])
	assert.Equal(t, "100", string(pct))
	assert.Equal(t, false, entry["latestRevision"],
		"Activate pins traffic explicitly — latestRevision must NOT be true")
}

// TestActivateKnativeRevision_OrphanRevision exercises the (rare) case
// where the Revision exists but carries no serving.knative.dev/service
// label. The function should surface a clear error naming the missing
// label rather than blindly attempting a patch with an empty parent.
func TestActivateKnativeRevision_OrphanRevision(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		knativeRevisionGVR: "RevisionList",
		knativeServiceGVR:  "ServiceList",
	}
	rev := &unstructured.Unstructured{}
	rev.SetUnstructuredContent(map[string]any{
		"apiVersion": "serving.knative.dev/v1",
		"kind":       "Revision",
		// no serving.knative.dev/service label
		"metadata": map[string]any{"name": "orphan", "namespace": "default"},
	})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, rev)
	c := newFakeClient(nil, dyn)

	_, err := c.ActivateKnativeRevision("test-ctx", "default", "orphan")
	require.Error(t, err)
	assert.Contains(t, err.Error(), knativeServiceLabel,
		"error must name the missing label so the user knows what's wrong")
}

func TestActivateKnativeRevision_RevisionNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		knativeRevisionGVR: "RevisionList",
		knativeServiceGVR:  "ServiceList",
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs)
	c := newFakeClient(nil, dyn)

	_, err := c.ActivateKnativeRevision("test-ctx", "default", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting Revision")
}
