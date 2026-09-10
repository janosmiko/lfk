package k8s

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestQuarantineTargets_MatchesOnlySelectorServices(t *testing.T) {
	podLabels := map[string]string{"app": "web", "tier": "backend"}

	cs := k8sfake.NewClientset(
		&corev1.Service{
			Name: "svc-match", Namespace: "default",
			Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "web"}},
		},
		&corev1.Service{
			Name: "svc-nomatch", Namespace: "default",
			Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "other"}},
		},
		&corev1.Service{
			Name: "svc-manual-endpoints", Namespace: "default",
			Spec: corev1.ServiceSpec{Selector: map[string]string{}},
		},
	)
	c := newFakeClient(cs, nil)

	services, keys, err := c.QuarantineTargets(t.Context(), "", "default", podLabels)
	require.NoError(t, err)
	assert.Equal(t, []string{"svc-match"}, services)
	assert.Equal(t, []string{"app"}, keys)
}

func quarantineTestPod() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":      "my-pod",
				"namespace": "default",
				"labels": map[string]any{
					"app":  "web",
					"tier": "backend",
				},
			},
		},
	}
}

func TestQuarantinePod_StripsKeysAndRecordsAnnotation(t *testing.T) {
	dc := newFakeDynClient(quarantineTestPod())
	c := newFakeClient(nil, dc)

	removed, err := c.QuarantinePod(t.Context(), "", "default", "my-pod", []string{"app"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"app": "web"}, removed)

	obj, err := dc.Resource(podsGVR).Namespace("default").Get(t.Context(), "my-pod", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotContains(t, obj.GetLabels(), "app")
	assert.Equal(t, "backend", obj.GetLabels()["tier"])
	assert.Equal(t, `{"app":"web"}`, obj.GetAnnotations()[QuarantinedLabelsAnnotation])
}

func TestQuarantinePod_PatchCarriesResourceVersion(t *testing.T) {
	pod := quarantineTestPod()
	pod.SetResourceVersion("42")
	dc := newFakeDynClient(pod)

	var captured k8stesting.PatchAction
	dc.PrependReactor("patch", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		captured = action.(k8stesting.PatchAction)
		return true, pod, nil
	})
	c := newFakeClient(nil, dc)

	_, err := c.QuarantinePod(t.Context(), "", "default", "my-pod", []string{"app"})
	require.NoError(t, err)
	require.NotNil(t, captured)

	var body map[string]any
	require.NoError(t, json.Unmarshal(captured.GetPatch(), &body))
	metadata, ok := body["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "42", metadata["resourceVersion"])
}

func TestQuarantinePod_ConflictSurfacesError(t *testing.T) {
	dc := newFakeDynClient(quarantineTestPod())
	patchCount := 0
	dc.PrependReactor("patch", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		patchCount++
		return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "pods"}, "my-pod", errors.New("stale resourceVersion"))
	})
	c := newFakeClient(nil, dc)

	_, err := c.QuarantinePod(t.Context(), "", "default", "my-pod", []string{"app"})
	require.Error(t, err)
	assert.True(t, apierrors.IsConflict(err))
	assert.Equal(t, 1, patchCount, "a conflict must not be retried with a second patch")
}

func TestRestorePod_PatchCarriesResourceVersion(t *testing.T) {
	pod := quarantineTestPodWithAnnotation(`{"app":"web"}`)
	pod.SetResourceVersion("99")
	dc := newFakeDynClient(pod)

	var captured k8stesting.PatchAction
	dc.PrependReactor("patch", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		captured = action.(k8stesting.PatchAction)
		return true, pod, nil
	})
	c := newFakeClient(nil, dc)

	_, err := c.RestorePod(t.Context(), "", "default", "my-pod")
	require.NoError(t, err)
	require.NotNil(t, captured)

	var body map[string]any
	require.NoError(t, json.Unmarshal(captured.GetPatch(), &body))
	metadata, ok := body["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "99", metadata["resourceVersion"])
}

func TestRestorePod_RestoresExactPairsAndClearsAnnotation(t *testing.T) {
	dc := newFakeDynClient(quarantineTestPod())
	c := newFakeClient(nil, dc)

	_, err := c.QuarantinePod(t.Context(), "", "default", "my-pod", []string{"app"})
	require.NoError(t, err)

	restored, err := c.RestorePod(t.Context(), "", "default", "my-pod")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"app": "web"}, restored)

	obj, err := dc.Resource(podsGVR).Namespace("default").Get(t.Context(), "my-pod", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"app": "web", "tier": "backend"}, obj.GetLabels())
	assert.NotContains(t, obj.GetAnnotations(), QuarantinedLabelsAnnotation)
}

func TestRestorePod_NoAnnotation_ReturnsErrNotQuarantined(t *testing.T) {
	dc := newFakeDynClient(quarantineTestPod())
	c := newFakeClient(nil, dc)

	_, err := c.RestorePod(t.Context(), "", "default", "my-pod")
	assert.True(t, errors.Is(err, ErrNotQuarantined))
}

func quarantineTestPodWithAnnotation(rawAnnotation string) *unstructured.Unstructured {
	pod := quarantineTestPod()
	pod.Object["metadata"].(map[string]any)["annotations"] = map[string]any{
		QuarantinedLabelsAnnotation: rawAnnotation,
	}
	return pod
}

func hasPatchAction(dc *dynamicfake.FakeDynamicClient) bool {
	for _, action := range dc.Actions() {
		if action.GetVerb() == "patch" {
			return true
		}
	}
	return false
}

func TestRestorePod_RefusesInvalidLabelKey(t *testing.T) {
	dc := newFakeDynClient(quarantineTestPodWithAnnotation(`{"bad key!":"value"}`))
	c := newFakeClient(nil, dc)

	_, err := c.RestorePod(t.Context(), "", "default", "my-pod")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTamperedAnnotation))
	assert.False(t, hasPatchAction(dc), "an invalid key must not reach a patch call")
}

func TestRestorePod_RefusesInvalidLabelValue(t *testing.T) {
	dc := newFakeDynClient(quarantineTestPodWithAnnotation(`{"app":"has spaces"}`))
	c := newFakeClient(nil, dc)

	_, err := c.RestorePod(t.Context(), "", "default", "my-pod")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTamperedAnnotation))
	assert.False(t, hasPatchAction(dc), "an invalid value must not reach a patch call")
}

func TestRestorePod_RefusesOversizedAnnotation(t *testing.T) {
	pairs := make(map[string]string, maxRestoredLabelEntries+1)
	for i := range maxRestoredLabelEntries + 1 {
		pairs[fmt.Sprintf("key-%d", i)] = "value"
	}
	raw, err := json.Marshal(pairs)
	require.NoError(t, err)

	dc := newFakeDynClient(quarantineTestPodWithAnnotation(string(raw)))
	c := newFakeClient(nil, dc)

	_, err = c.RestorePod(t.Context(), "", "default", "my-pod")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTamperedAnnotation))
	assert.False(t, hasPatchAction(dc), "an oversized annotation must not reach a patch call")
}
