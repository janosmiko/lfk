package k8s

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sfake "k8s.io/client-go/kubernetes/fake"
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
