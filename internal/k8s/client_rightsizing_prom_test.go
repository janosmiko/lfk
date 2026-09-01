package k8s

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/model"
)

// --- buildPromContainerQuery ---

func TestBuildPromContainerQuery_StrategyToPromQL(t *testing.T) {
	maxQ := buildPromContainerQuery("default", "Deployment", "frontend", model.StrategyPromMax1D, "cpu")
	assert.Contains(t, maxQ, "max_over_time", "1d-max strategy must use max_over_time")
	assert.Contains(t, maxQ, "[1d:", "1d window in subquery range")
	assert.Contains(t, maxQ, "container_cpu_usage_seconds_total", "CPU query targets cpu metric")
	assert.Contains(t, maxQ, `namespace="default"`, "namespace label filter")
	assert.Contains(t, maxQ, "frontend", "workload name in pod regex")

	avgQ := buildPromContainerQuery("default", "Deployment", "frontend", model.StrategyPromAvg1D, "cpu")
	assert.Contains(t, avgQ, "avg_over_time", "1d-avg strategy must use avg_over_time")
	assert.Contains(t, avgQ, "[1d:")

	p95Q := buildPromContainerQuery("default", "Deployment", "frontend", model.StrategyPromP957D, "cpu")
	assert.Contains(t, p95Q, "quantile_over_time(0.95", "p95 strategy uses quantile_over_time")
	assert.Contains(t, p95Q, "[7d:", "7d window in subquery range")

	memQ := buildPromContainerQuery("default", "Deployment", "frontend", model.StrategyPromMax1D, "memory")
	assert.Contains(t, memQ, "container_memory_working_set_bytes", "memory query targets memory metric")
	assert.NotContains(t, memQ, "rate(", "memory uses gauge, no rate() wrapper")
}

func TestBuildPromContainerQuery_PodRegexEscapesSpecialChars(t *testing.T) {
	q := buildPromContainerQuery("default", "Deployment", "app.v1", model.StrategyPromMax1D, "cpu")
	assert.Contains(t, q, `app\.v1`, "regex metacharacters must be escaped")
	assert.Contains(t, q, "pod=~`", "the matcher uses a raw PromQL literal so `\\.` survives unprocessed")
}

// --- parsePrometheusContainerResponse ---

func TestParsePrometheusContainerResponse_CPUValuesInCores(t *testing.T) {
	// Prometheus returns CPU rates in cores (e.g. "0.05" = 50m). The
	// parser must convert to millicores so SnapCPUMilliToCanonical can
	// snap to "50m".
	body := []byte(`{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{"metric": {"container": "app"}, "value": [1700000000, "0.08"]},
				{"metric": {"container": "sidecar"}, "value": [1700000000, "0.012"]}
			]
		}
	}`)
	got, err := parsePrometheusContainerResponse(body)
	require.NoError(t, err)
	assert.InDelta(t, 0.08, got["app"], 0.0001, "raw cores preserved")
	assert.InDelta(t, 0.012, got["sidecar"], 0.0001)
}

func TestParsePrometheusContainerResponse_BadJSONErrors(t *testing.T) {
	_, err := parsePrometheusContainerResponse([]byte("not json"))
	assert.Error(t, err)
}

func TestParsePrometheusContainerResponse_NonSuccessStatusErrors(t *testing.T) {
	_, err := parsePrometheusContainerResponse([]byte(`{"status":"error"}`))
	assert.Error(t, err)
}

// --- GetRightsizing with Prometheus strategies ---

func newPromTestClient(t *testing.T, pods []*corev1.Pod, dep *appsv1.Deployment, stub func(ctx context.Context, contextName, query string) ([]byte, error)) *Client {
	t.Helper()

	objs := make([]runtime.Object, 0, 1+len(pods))
	objs = append(objs, dep)
	for _, p := range pods {
		objs = append(objs, p)
	}
	cs := fake.NewSimpleClientset(objs...)

	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		vpaGVRForTest(): "VerticalPodAutoscalerList",
		{Group: "autoscaling.k8s.io", Version: "v1beta2", Resource: "verticalpodautoscalers"}: "VerticalPodAutoscalerList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:                       "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1", Resource: "pods"}:                            "PodMetricsList",
	})

	c := NewTestClient(cs, dyn)
	c.testPromQuery = stub

	// Register Prometheus config so the strategy is considered available.
	prevCfg := model.ConfigMonitoring
	t.Cleanup(func() { model.ConfigMonitoring = prevCfg })
	model.ConfigMonitoring = map[string]model.MonitoringConfig{
		"test-ctx": {
			Prometheus: &model.MonitoringEndpoint{
				Namespaces: []string{"monitoring"},
				Services:   []string{"prometheus"},
				Port:       "9090",
			},
		},
	}
	return c
}

func makeDepFixture() (*appsv1.Deployment, []*corev1.Pod) {
	dep := &appsv1.Deployment{
		Name: "frontend", Namespace: "default",
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "frontend"}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("128Mi")},
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("512Mi")},
					},
				}}},
			},
		},
	}
	pod := &corev1.Pod{
		Name: "frontend-a", Namespace: "default", Labels: map[string]string{"app": "frontend"},
	}
	return dep, []*corev1.Pod{pod}
}

func TestGetRightsizing_PrometheusMax1D(t *testing.T) {
	dep, pods := makeDepFixture()
	calls := 0
	stub := func(_ context.Context, _ string, query string) ([]byte, error) {
		calls++
		// CPU returns 0.08 cores (= 80m); Memory returns ~200Mi worth of bytes.
		if strings.Contains(query, "container_cpu_usage_seconds_total") {
			return []byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"container":"app"},"value":[1700000000,"0.08"]}]}}`), nil
		}
		// 200 * 1024 * 1024 = 209715200
		return []byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"container":"app"},"value":[1700000000,"209715200"]}]}}`), nil
	}
	c := newPromTestClient(t, pods, dep, stub)

	// Explicit 1.2 headroom keeps the legacy expected values stable
	// (the new picker default is 1.25, so without this the snap math
	// would land on different canonical values).
	out, err := c.GetRightsizing(t.Context(), "test-ctx", "default", "Deployment", "frontend", model.StrategyPromMax1D, 1.2)
	require.NoError(t, err)
	assert.Equal(t, model.StrategyPromMax1D, out.Strategy)
	assert.Equal(t, "1d-max", out.Source)
	assert.Equal(t, "1d", out.Window, "Prometheus window plumbed into Window field")
	require.Len(t, out.Containers, 1)
	rec := out.Containers[0]
	// 80m * 1.2 = 96m → snap up to 100m
	assert.Equal(t, "100m", rec.CPU.RecommendedRequest)
	// 200Mi * 1.2 = 240Mi
	assert.Equal(t, "240Mi", rec.Mem.RecommendedRequest)
	assert.Equal(t, 2, calls, "expects one CPU + one memory PromQL call")
}

func TestGetRightsizing_PrometheusAvg1D(t *testing.T) {
	dep, pods := makeDepFixture()
	stub := func(_ context.Context, _ string, query string) ([]byte, error) {
		assert.Contains(t, query, "avg_over_time", "avg strategy must use avg_over_time")
		assert.Contains(t, query, "[1d:")
		if strings.Contains(query, "container_cpu_usage_seconds_total") {
			return []byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"container":"app"},"value":[1700000000,"0.04"]}]}}`), nil
		}
		// 100Mi worth
		return []byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"container":"app"},"value":[1700000000,"104857600"]}]}}`), nil
	}
	c := newPromTestClient(t, pods, dep, stub)

	out, err := c.GetRightsizing(t.Context(), "test-ctx", "default", "Deployment", "frontend", model.StrategyPromAvg1D, 1.2)
	require.NoError(t, err)
	assert.Equal(t, model.StrategyPromAvg1D, out.Strategy)
	assert.Equal(t, "1d", out.Window)
	// 40m * 1.2 = 48m → snap up to 50m
	assert.Equal(t, "50m", out.Containers[0].CPU.RecommendedRequest)
	assert.Equal(t, "120Mi", out.Containers[0].Mem.RecommendedRequest)
}

func TestGetRightsizing_PrometheusP957D(t *testing.T) {
	dep, pods := makeDepFixture()
	stub := func(_ context.Context, _ string, query string) ([]byte, error) {
		assert.Contains(t, query, "quantile_over_time(0.95")
		assert.Contains(t, query, "[7d:")
		if strings.Contains(query, "container_cpu_usage_seconds_total") {
			return []byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"container":"app"},"value":[1700000000,"0.05"]}]}}`), nil
		}
		// 256Mi worth
		return []byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"container":"app"},"value":[1700000000,"268435456"]}]}}`), nil
	}
	c := newPromTestClient(t, pods, dep, stub)

	out, err := c.GetRightsizing(t.Context(), "test-ctx", "default", "Deployment", "frontend", model.StrategyPromP957D, 1.2)
	require.NoError(t, err)
	assert.Equal(t, model.StrategyPromP957D, out.Strategy)
	assert.Equal(t, "7d", out.Window)
	// 50m * 1.2 = 60m
	assert.Equal(t, "60m", out.Containers[0].CPU.RecommendedRequest)
	// 256 * 1.2 = 307.2 → 308Mi
	assert.Equal(t, "308Mi", out.Containers[0].Mem.RecommendedRequest)
}

func TestGetRightsizing_PrometheusQueryFailureLeavesSuggestionEmpty(t *testing.T) {
	// Network/HTTP errors from the proxy must not blow up the request —
	// the SUGGESTION column simply renders dashes for that container.
	dep, pods := makeDepFixture()
	stub := func(_ context.Context, _ string, _ string) ([]byte, error) {
		return nil, assert.AnError
	}
	c := newPromTestClient(t, pods, dep, stub)

	out, err := c.GetRightsizing(t.Context(), "test-ctx", "default", "Deployment", "frontend", model.StrategyPromMax1D, model.DefaultRightsizingHeadroom)
	require.NoError(t, err, "Prom failure must be soft — strategy still selected")
	require.Len(t, out.Containers, 1)
	assert.Empty(t, out.Containers[0].CPU.RecommendedRequest)
	assert.Empty(t, out.Containers[0].Mem.RecommendedRequest)
}

// --- podsRegexForWorkload ---

func TestPodsRegexForWorkload_MatchesControllerPodShapes(t *testing.T) {
	cases := []struct {
		kind   string
		name   string
		match  []string
		reject []string
	}{
		{
			kind:  "Deployment",
			name:  "frontend",
			match: []string{"frontend-7d9fkp2xz4-b5tqm", "frontend-b-jkl29"},
			reject: []string{
				"frontend",
				"frontend-7d9fkp2xz4",
				"frontend-api-7d9fkp2xz4-b5tqm",
				"frontend-canary-7d9fkp2xz4-b5tqm",
			},
		},
		{
			kind:   "DaemonSet",
			name:   "node-agent",
			match:  []string{"node-agent-k4n2p"},
			reject: []string{"node-agent", "node-agent-7d9fkp2xz4-b5tqm"},
		},
		{
			kind:   "StatefulSet",
			name:   "postgres",
			match:  []string{"postgres-0", "postgres-12"},
			reject: []string{"postgres", "postgres-k4n2p"},
		},
		{
			kind:   "Job",
			name:   "backup",
			match:  []string{"backup-k4n2p", "backup-3-k4n2p"},
			reject: []string{"backup", "backup-k4n2p-x5tqm"},
		},
		{
			kind: "CronJob",
			name: "nightly",
			// The second form is a CronJob whose job template sets
			// completionMode: Indexed, which inserts the index.
			match:  []string{"nightly-29358720-k4n2p", "nightly-29358720-3-k4n2p"},
			reject: []string{"nightly", "nightly-k4n2p"},
		},
		{
			kind:   "Pod",
			name:   "standalone",
			match:  []string{"standalone"},
			reject: []string{"standalone-k4n2p"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			re, err := regexp.Compile(podsRegexForWorkload(tc.kind, tc.name))
			require.NoError(t, err, "generated regex must compile")
			for _, p := range tc.match {
				assert.True(t, re.MatchString(p), "%s should match", p)
			}
			for _, p := range tc.reject {
				assert.False(t, re.MatchString(p), "%s should not match", p)
			}
		})
	}
}

func TestPodsRegexForWorkload_SurvivesRollout(t *testing.T) {
	// Applying a recommendation rolls the workload, so every pod name
	// changes and a selector pinned to the old names loses the window.
	re := regexp.MustCompile(podsRegexForWorkload("Deployment", "frontend"))
	assert.True(t, re.MatchString("frontend-5x9dkm2t7b-q4wzn"), "pod from the revision before the edit")
	assert.True(t, re.MatchString("frontend-7fkp2xz4d9-m5tqb"), "pod from the revision after the edit")
}

func TestPodsRegexForWorkload_EscapesDotsInWorkloadName(t *testing.T) {
	// Deployment names are DNS subdomains, so a dot is legal and would
	// otherwise match any character in that position.
	re := regexp.MustCompile(podsRegexForWorkload("Deployment", "app.v1"))
	assert.True(t, re.MatchString("app.v1-7d9fkp2xz4-b5tqm"))
	assert.False(t, re.MatchString("appxv1-7d9fkp2xz4-b5tqm"), "dot must not act as a wildcard")
}

func TestPodsRegexForWorkload_UnsupportedKindMatchesNothing(t *testing.T) {
	assert.Empty(t, podsRegexForWorkload("ReplicaSet", "frontend"),
		"an unknown kind must not fall back to a namespace-wide selector")
}

func TestBuildPromContainerQuery_NotPinnedToCurrentPodNames(t *testing.T) {
	q := buildPromContainerQuery("default", "Deployment", "frontend", model.StrategyPromP957D, "cpu")
	assert.Contains(t, q, "[7d:", "7d window in subquery range")

	matcher := regexp.MustCompile("pod=~`([^`]+)`").FindStringSubmatch(q)
	require.Len(t, matcher, 2, "query must carry a pod matcher in a raw literal")
	re, err := regexp.Compile(matcher[1])
	require.NoError(t, err)
	assert.True(t, re.MatchString("frontend-5x9dkm2t7b-q4wzn"), "pod from the revision before the edit")
	assert.True(t, re.MatchString("frontend-7fkp2xz4d9-m5tqb"), "pod from the revision after the edit")
}

func TestBuildPromContainerQuery_EmptyForUnsupportedKind(t *testing.T) {
	assert.Empty(t, buildPromContainerQuery("default", "ReplicaSet", "frontend", model.StrategyPromP957D, "cpu"))
}
