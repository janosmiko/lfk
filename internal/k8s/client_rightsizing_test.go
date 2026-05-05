package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/model"
)

func TestGetRightsizing_PodCurrentSpecExtracted(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
				},
			},
		},
	}
	cs := fake.NewSimpleClientset(pod)
	c := NewTestClient(cs, nil)
	out, err := c.GetRightsizing(context.Background(), "test-ctx", "default", "Pod", "pod-a", model.StrategySnapshot, model.DefaultRightsizingHeadroom)
	assert.NoError(t, err)
	assert.NotNil(t, out)
	assert.Equal(t, "snapshot", out.Source, "snapshot strategy yields snapshot label")
	assert.Equal(t, model.StrategySnapshot, out.Strategy)
	assert.Equal(t, 1, out.PodCount)
	assert.Len(t, out.Containers, 1)
	rec := out.Containers[0]
	assert.Equal(t, "app", rec.Name)
	assert.Equal(t, "100m", rec.CPU.CurrentRequest)
	assert.Equal(t, "500m", rec.CPU.CurrentLimit)
	assert.Equal(t, "256Mi", rec.Mem.CurrentRequest)
	assert.Equal(t, "512Mi", rec.Mem.CurrentLimit)
	assert.Empty(t, rec.CPU.RecommendedRequest, "no metrics → no rec yet")
	assert.Empty(t, rec.Mem.RecommendedRequest)
}

func TestGetRightsizing_UnsupportedKindErrors(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := NewTestClient(cs, nil)
	_, err := c.GetRightsizing(context.Background(), "test-ctx", "default", "ConfigMap", "x", model.StrategySnapshot, model.DefaultRightsizingHeadroom)
	assert.Error(t, err, "kinds outside in-scope set must error")
}

func vpaGVRForTest() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "autoscaling.k8s.io",
		Version:  "v1",
		Resource: "verticalpodautoscalers",
	}
}

//nolint:unparam // ns is "default" across all current callers but is the natural API shape — keep for future test cases that need a different namespace.
func makeVPAFixture(ns, name, targetKind, targetName string, recs []map[string]any) *unstructured.Unstructured {
	// Convert []map[string]any -> []any so unstructured.DeepCopy
	// (used by the fake client's tracker) can walk it.
	recsAny := make([]any, 0, len(recs))
	for _, r := range recs {
		recsAny = append(recsAny, r)
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "autoscaling.k8s.io/v1",
		"kind":       "VerticalPodAutoscaler",
		"metadata":   map[string]any{"name": name, "namespace": ns},
		"spec": map[string]any{
			"targetRef": map[string]any{"kind": targetKind, "name": targetName},
		},
		"status": map[string]any{
			"recommendation": map[string]any{
				"containerRecommendations": recsAny,
			},
		},
	}}
}

func TestGetRightsizing_VPAMatchesPopulatesRecommendations(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-aaa", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "app",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			}},
		},
	}
	cs := fake.NewSimpleClientset(pod)
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		vpaGVRForTest(): "VerticalPodAutoscalerList",
		// Register v1beta2 too so the fake doesn't panic on the
		// fallthrough when v1 doesn't match. In a real cluster the
		// dynamic client returns NoMatchError on missing GVRs, which
		// findVPA silently `continue`s.
		{Group: "autoscaling.k8s.io", Version: "v1beta2", Resource: "verticalpodautoscalers"}: "VerticalPodAutoscalerList",
		// Metrics GVRs registered so the metrics-server fallback's
		// dynamic List doesn't panic. No PodMetrics objects are
		// loaded here — VPA already covers all containers.
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}: "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1", Resource: "pods"}:      "PodMetricsList",
	}, makeVPAFixture("default", "vpa-frontend", "Pod", "frontend-aaa", []map[string]any{
		{
			"containerName": "app",
			"target":        map[string]any{"cpu": "60m", "memory": "200Mi"},
			"lowerBound":    map[string]any{"cpu": "50m", "memory": "180Mi"},
			"upperBound":    map[string]any{"cpu": "250m", "memory": "400Mi"},
		},
	}))
	c := NewTestClient(cs, dyn)

	// headroom = 1.0 keeps the VPA target verbatim so the existing
	// expected values (60m, 300m, 200Mi, 400Mi) still hold. The non-1.0
	// case is covered by TestGetRightsizing_VPAHeadroomMultipliesTarget.
	out, err := c.GetRightsizing(context.Background(), "test-ctx", "default", "Pod", "frontend-aaa", model.StrategyVPA, 1.0)
	assert.NoError(t, err)
	assert.Equal(t, "VPA", out.Source)
	assert.Equal(t, model.StrategyVPA, out.Strategy)
	assert.Len(t, out.Containers, 1)
	rec := out.Containers[0]
	assert.Equal(t, "60m", rec.CPU.RecommendedRequest)
	// Spec ratio 1:5 → 60m × 5 = 300m
	assert.Equal(t, "300m", rec.CPU.RecommendedLimit)
	assert.Equal(t, "200Mi", rec.Mem.RecommendedRequest)
	// Mem ratio 256Mi:512Mi = 1:2 → 200Mi × 2 = 400Mi (covers the
	// MilliValue()/1000 → SnapMemBytesToCanonical path)
	assert.Equal(t, "400Mi", rec.Mem.RecommendedLimit)
	assert.Equal(t, "50m", rec.CPU.LowerBound)
	assert.Equal(t, "250m", rec.CPU.UpperBound)
}

func TestGetRightsizing_VPATargetMismatchFallsThrough(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-aaa", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	}
	cs := fake.NewSimpleClientset(pod)
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		vpaGVRForTest(): "VerticalPodAutoscalerList",
		// Register v1beta2 too so the fake doesn't panic on the
		// fallthrough when v1 doesn't match. In a real cluster the
		// dynamic client returns NoMatchError on missing GVRs, which
		// findVPA silently `continue`s.
		{Group: "autoscaling.k8s.io", Version: "v1beta2", Resource: "verticalpodautoscalers"}: "VerticalPodAutoscalerList",
		// Metrics GVRs registered so the metrics-server fallback's
		// dynamic List doesn't panic. No PodMetrics objects loaded —
		// the fallback finds no data and the source stays "estimated".
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}: "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1", Resource: "pods"}:      "PodMetricsList",
	}, makeVPAFixture("default", "vpa-other", "Deployment", "different", nil))
	c := NewTestClient(cs, dyn)

	// VPA strategy with no matching VPA: the request still uses the
	// VPA dispatch (no recommendations layered) and falls through to the
	// metrics-server "live Usage" reading. Source label reflects the
	// requested strategy because it didn't escalate to a different one.
	out, err := c.GetRightsizing(context.Background(), "test-ctx", "default", "Pod", "frontend-aaa", model.StrategyVPA, model.DefaultRightsizingHeadroom)
	assert.NoError(t, err)
	assert.Equal(t, model.StrategyVPA, out.Strategy, "requested strategy is preserved")
}

func TestGetRightsizing_MetricsFallbackMaxAggregation(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "frontend"}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
					},
				}}},
			},
		},
	}
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-a", Namespace: "default", Labels: map[string]string{"app": "frontend"}},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-b", Namespace: "default", Labels: map[string]string{"app": "frontend"}},
	}
	cs := fake.NewSimpleClientset(dep, pod1, pod2)

	metricsGVR := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		metricsGVR:      "PodMetricsList",
		vpaGVRForTest(): "VerticalPodAutoscalerList",
		// Register v1beta2 too so the fake doesn't panic on the
		// fallthrough when v1 doesn't match.
		{Group: "autoscaling.k8s.io", Version: "v1beta2", Resource: "verticalpodautoscalers"}: "VerticalPodAutoscalerList",
		// Register the v1 metrics GVR so the GVR-fallthrough loop in
		// applyMetricsRecommendations doesn't panic if it ever falls
		// through (the v1beta1 list succeeds first in this test).
		{Group: "metrics.k8s.io", Version: "v1", Resource: "pods"}: "PodMetricsList",
	})

	// Direct tracker.Create — Add() goes through UnsafeGuessKindToResource
	// which maps "PodMetrics" → "podmetricses" (kinds ending in 's' get
	// '-es' appended). Real metrics.k8s.io publishes under "pods", so we
	// have to bypass the heuristic and tell the tracker the exact GVR.
	pm1 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "metrics.k8s.io/v1beta1", "kind": "PodMetrics",
		"metadata":   map[string]any{"name": "frontend-a", "namespace": "default"},
		"containers": []any{map[string]any{"name": "app", "usage": map[string]any{"cpu": "50m"}}},
	}}
	pm2 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "metrics.k8s.io/v1beta1", "kind": "PodMetrics",
		"metadata":   map[string]any{"name": "frontend-b", "namespace": "default"},
		"containers": []any{map[string]any{"name": "app", "usage": map[string]any{"cpu": "80m"}}},
	}}
	if err := dyn.Tracker().Create(metricsGVR, pm1, "default"); err != nil {
		t.Fatalf("seeding pm1: %v", err)
	}
	if err := dyn.Tracker().Create(metricsGVR, pm2, "default"); err != nil {
		t.Fatalf("seeding pm2: %v", err)
	}
	c := NewTestClient(cs, dyn)

	// Explicit 1.2 headroom keeps the legacy expected values stable
	// (the new picker default is 1.25, so without this the snap math
	// would land on different canonical values).
	out, err := c.GetRightsizing(context.Background(), "test-ctx", "default", "Deployment", "frontend", model.StrategySnapshot, 1.2)
	assert.NoError(t, err)
	assert.Equal(t, "snapshot", out.Source)
	assert.Equal(t, model.StrategySnapshot, out.Strategy)
	assert.Equal(t, 2, out.PodCount)
	assert.Len(t, out.Containers, 1)
	rec := out.Containers[0]
	// max(50, 80) = 80m; × 1.2 = 96m; snap up to 100m
	assert.Equal(t, "100m", rec.CPU.RecommendedRequest)
	// Spec ratio 1:5 → 100m × 5 = 500m
	assert.Equal(t, "500m", rec.CPU.RecommendedLimit)
	// Usage column populated from the same max-aggregated reading
	// (snapped to nearest 10m for canonical display).
	assert.Equal(t, "80m", rec.CPU.Usage, "Usage surfaces the observed peak even when recommendations are derived from it")
}

func TestGetRightsizing_MetricsWindowPlumbedThrough(t *testing.T) {
	// PodMetrics carries a `window` field with the snapshot duration
	// (e.g. "30s"). Plumbing it through to model.Rightsizing.Window
	// lets the renderer show the user what window backs the
	// estimated recommendation.
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "frontend"}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
					},
				}}},
			},
		},
	}
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-a", Namespace: "default", Labels: map[string]string{"app": "frontend"}},
	}
	cs := fake.NewSimpleClientset(dep, pod1)

	metricsGVR := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		metricsGVR:      "PodMetricsList",
		vpaGVRForTest(): "VerticalPodAutoscalerList",
		{Group: "autoscaling.k8s.io", Version: "v1beta2", Resource: "verticalpodautoscalers"}: "VerticalPodAutoscalerList",
		{Group: "metrics.k8s.io", Version: "v1", Resource: "pods"}:                            "PodMetricsList",
	})
	pm := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "metrics.k8s.io/v1beta1", "kind": "PodMetrics",
		"metadata":   map[string]any{"name": "frontend-a", "namespace": "default"},
		"window":     "30s",
		"containers": []any{map[string]any{"name": "app", "usage": map[string]any{"cpu": "50m"}}},
	}}
	if err := dyn.Tracker().Create(metricsGVR, pm, "default"); err != nil {
		t.Fatalf("seeding pm: %v", err)
	}
	c := NewTestClient(cs, dyn)

	out, err := c.GetRightsizing(context.Background(), "test-ctx", "default", "Deployment", "frontend", model.StrategySnapshot, model.DefaultRightsizingHeadroom)
	assert.NoError(t, err)
	assert.Equal(t, "30s", out.Window, "metrics-server PodMetrics 'window' field should be plumbed to model.Rightsizing.Window")
}

// silence unused import lint if dynamic isn't directly referenced
var _ dynamic.Interface = (*dynamicfake.FakeDynamicClient)(nil)

// --- Headroom parameter plumbing ---

func TestGetRightsizing_HeadroomZeroDefaults(t *testing.T) {
	// Callers that don't yet know about headroom (or pass a zero value
	// by accident) must NOT trip a divide-by-zero / multiply-by-zero
	// footgun. The dispatcher snaps headroom == 0 to
	// model.DefaultRightsizingHeadroom so the recommendation column is
	// always populated with a sane number.
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "frontend"}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
					},
				}}},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-a", Namespace: "default", Labels: map[string]string{"app": "frontend"}},
	}
	cs := fake.NewSimpleClientset(dep, pod)

	metricsGVR := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		metricsGVR:      "PodMetricsList",
		vpaGVRForTest(): "VerticalPodAutoscalerList",
		{Group: "autoscaling.k8s.io", Version: "v1beta2", Resource: "verticalpodautoscalers"}: "VerticalPodAutoscalerList",
		{Group: "metrics.k8s.io", Version: "v1", Resource: "pods"}:                            "PodMetricsList",
	})
	pm := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "metrics.k8s.io/v1beta1", "kind": "PodMetrics",
		"metadata":   map[string]any{"name": "frontend-a", "namespace": "default"},
		"containers": []any{map[string]any{"name": "app", "usage": map[string]any{"cpu": "80m"}}},
	}}
	if err := dyn.Tracker().Create(metricsGVR, pm, "default"); err != nil {
		t.Fatalf("seeding pm: %v", err)
	}
	c := NewTestClient(cs, dyn)

	out, err := c.GetRightsizing(context.Background(), "test-ctx", "default", "Deployment", "frontend", model.StrategySnapshot, 0)
	assert.NoError(t, err)
	require.Len(t, out.Containers, 1)
	// 80m * 1.25 = 100m → snap up to 100m. (1.25 is the default headroom
	// when 0 is passed.)
	assert.Equal(t, "100m", out.Containers[0].CPU.RecommendedRequest)
	assert.InDelta(t, model.DefaultRightsizingHeadroom, out.Headroom, 1e-9,
		"out.Headroom must echo the effective multiplier (default when 0 passed)")
}

func TestGetRightsizing_HeadroomCustomMultiplier(t *testing.T) {
	// A non-default headroom flows through to the recommendation math —
	// at 1.5, the snapshot recommendation is the observed peak times 1.5
	// (snapped to canonical units), and the Headroom field round-trips.
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "frontend"}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
					},
				}}},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-a", Namespace: "default", Labels: map[string]string{"app": "frontend"}},
	}
	cs := fake.NewSimpleClientset(dep, pod)

	metricsGVR := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		metricsGVR:      "PodMetricsList",
		vpaGVRForTest(): "VerticalPodAutoscalerList",
		{Group: "autoscaling.k8s.io", Version: "v1beta2", Resource: "verticalpodautoscalers"}: "VerticalPodAutoscalerList",
		{Group: "metrics.k8s.io", Version: "v1", Resource: "pods"}:                            "PodMetricsList",
	})
	pm := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "metrics.k8s.io/v1beta1", "kind": "PodMetrics",
		"metadata":   map[string]any{"name": "frontend-a", "namespace": "default"},
		"containers": []any{map[string]any{"name": "app", "usage": map[string]any{"cpu": "80m"}}},
	}}
	if err := dyn.Tracker().Create(metricsGVR, pm, "default"); err != nil {
		t.Fatalf("seeding pm: %v", err)
	}
	c := NewTestClient(cs, dyn)

	out, err := c.GetRightsizing(context.Background(), "test-ctx", "default", "Deployment", "frontend", model.StrategySnapshot, 1.5)
	assert.NoError(t, err)
	require.Len(t, out.Containers, 1)
	// 80m * 1.5 = 120m → snap up to 120m
	assert.Equal(t, "120m", out.Containers[0].CPU.RecommendedRequest)
	assert.InDelta(t, 1.5, out.Headroom, 1e-9, "out.Headroom must echo the effective multiplier")
}

func TestGetRightsizing_VPAHeadroomMultipliesTarget(t *testing.T) {
	// The VPA strategy multiplies the recommender's target by the
	// headroom factor. At 1.0, the recommendation is the raw VPA target;
	// at 1.5, it's target × 1.5 (snapped to canonical units).
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-aaa", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "app",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			}},
		},
	}
	cs := fake.NewSimpleClientset(pod)
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		vpaGVRForTest(): "VerticalPodAutoscalerList",
		{Group: "autoscaling.k8s.io", Version: "v1beta2", Resource: "verticalpodautoscalers"}: "VerticalPodAutoscalerList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:                       "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1", Resource: "pods"}:                            "PodMetricsList",
	}, makeVPAFixture("default", "vpa-frontend", "Pod", "frontend-aaa", []map[string]any{
		{
			"containerName": "app",
			"target":        map[string]any{"cpu": "60m", "memory": "200Mi"},
		},
	}))
	c := NewTestClient(cs, dyn)

	// At headroom 1.5: cpu 60m × 1.5 = 90m; memory 200Mi × 1.5 = 300Mi
	out, err := c.GetRightsizing(context.Background(), "test-ctx", "default", "Pod", "frontend-aaa", model.StrategyVPA, 1.5)
	assert.NoError(t, err)
	require.Len(t, out.Containers, 1)
	assert.Equal(t, "90m", out.Containers[0].CPU.RecommendedRequest, "VPA target × 1.5 headroom")
	assert.Equal(t, "300Mi", out.Containers[0].Mem.RecommendedRequest, "VPA memory target × 1.5 headroom")
	assert.InDelta(t, 1.5, out.Headroom, 1e-9)
}

func TestGetRightsizing_VPAHeadroom1RawTarget(t *testing.T) {
	// At headroom = 1.0 the VPA path returns the raw recommender target.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-aaa", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "app",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
					Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
				},
			}},
		},
	}
	cs := fake.NewSimpleClientset(pod)
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		vpaGVRForTest(): "VerticalPodAutoscalerList",
		{Group: "autoscaling.k8s.io", Version: "v1beta2", Resource: "verticalpodautoscalers"}: "VerticalPodAutoscalerList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:                       "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1", Resource: "pods"}:                            "PodMetricsList",
	}, makeVPAFixture("default", "vpa-frontend", "Pod", "frontend-aaa", []map[string]any{
		{
			"containerName": "app",
			"target":        map[string]any{"cpu": "60m"},
		},
	}))
	c := NewTestClient(cs, dyn)

	out, err := c.GetRightsizing(context.Background(), "test-ctx", "default", "Pod", "frontend-aaa", model.StrategyVPA, 1.0)
	assert.NoError(t, err)
	assert.Equal(t, "60m", out.Containers[0].CPU.RecommendedRequest, "headroom 1.0 returns raw VPA target")
}

// --- AvailableRightsizingStrategies ---

func TestAvailableRightsizingStrategies(t *testing.T) {
	// Save and restore the global ConfigMonitoring so test ordering
	// doesn't leak between cases.
	prevCfg := model.ConfigMonitoring
	t.Cleanup(func() { model.ConfigMonitoring = prevCfg })

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-aaa", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	}
	matchVPA := makeVPAFixture("default", "vpa-frontend", "Pod", "frontend-aaa", nil)
	otherVPA := makeVPAFixture("default", "vpa-other", "Deployment", "different", nil)

	cases := []struct {
		name    string
		vpaObj  *unstructured.Unstructured
		promCfg *model.MonitoringEndpoint
		want    []model.RightsizingStrategy
		comment string
	}{
		{
			name:    "no VPA, no Prometheus -> snapshot only",
			vpaObj:  otherVPA,
			promCfg: nil,
			want:    []model.RightsizingStrategy{model.StrategySnapshot},
			comment: "Snapshot is always available; no other source matches",
		},
		{
			name:    "VPA matches, no Prometheus -> VPA + snapshot",
			vpaObj:  matchVPA,
			promCfg: nil,
			want:    []model.RightsizingStrategy{model.StrategyVPA, model.StrategySnapshot},
		},
		{
			name:    "no VPA, Prometheus configured -> prom strategies + snapshot",
			vpaObj:  otherVPA,
			promCfg: &model.MonitoringEndpoint{Namespaces: []string{"monitoring"}, Services: []string{"prometheus"}, Port: "9090"},
			want: []model.RightsizingStrategy{
				model.StrategyPromMax1D,
				model.StrategyPromAvg1D,
				model.StrategyPromP957D,
				model.StrategySnapshot,
			},
		},
		{
			name:    "VPA matches + Prometheus configured -> all strategies in priority order",
			vpaObj:  matchVPA,
			promCfg: &model.MonitoringEndpoint{Namespaces: []string{"monitoring"}, Services: []string{"prometheus"}, Port: "9090"},
			want: []model.RightsizingStrategy{
				model.StrategyVPA,
				model.StrategyPromMax1D,
				model.StrategyPromAvg1D,
				model.StrategyPromP957D,
				model.StrategySnapshot,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.promCfg != nil {
				model.ConfigMonitoring = map[string]model.MonitoringConfig{
					"test-ctx": {Prometheus: tc.promCfg},
				}
			} else {
				model.ConfigMonitoring = nil
			}

			cs := fake.NewSimpleClientset(pod)
			scheme := runtime.NewScheme()
			dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
				vpaGVRForTest(): "VerticalPodAutoscalerList",
				{Group: "autoscaling.k8s.io", Version: "v1beta2", Resource: "verticalpodautoscalers"}: "VerticalPodAutoscalerList",
			}, tc.vpaObj)
			c := NewTestClient(cs, dyn)

			got := c.AvailableRightsizingStrategies(context.Background(), "test-ctx", "default", "Pod", "frontend-aaa")
			assert.Equal(t, tc.want, got, tc.comment)
		})
	}
}
