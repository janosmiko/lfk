package demo

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// buildPodMetrics and buildNodeMetrics seed the metrics.k8s.io/v1beta1
// PodMetrics/NodeMetrics objects internal/k8s.getPodMetricsFromAPI and
// getNodeMetricsFromAPI read. They are unstructured because no typed Go
// package for metrics.k8s.io is vendored here. internal/k8s reads them as
// unstructured too, so the shape only needs to match what parsePodMetrics
// and getNodeMetricsFromAPI expect. The shape is a top-level
// "containers[].usage" map for pods, a top-level "usage" map for nodes.

func buildPodMetrics() []*unstructured.Unstructured {
	ts := demoEpoch.Add(-30 * time.Second).Format(time.RFC3339)
	return []*unstructured.Unstructured{
		podMetrics(PodWebHealthy1, "45m", "96Mi", ts),
		podMetrics(PodWebHealthy2, "52m", "101Mi", ts),
		podMetrics(PodWebCrashLoop, "5m", "24Mi", ts),
	}
}

func podMetrics(name, cpu, mem, ts string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "metrics.k8s.io/v1beta1",
		"kind":       "PodMetrics",
		"metadata": map[string]any{
			"name":      name,
			"namespace": NamespaceDemo,
		},
		"timestamp": ts,
		"window":    "30s",
		"containers": []any{
			map[string]any{
				"name": "web",
				"usage": map[string]any{
					"cpu":    cpu,
					"memory": mem,
				},
			},
		},
	}}
}

func buildNodeMetrics() []*unstructured.Unstructured {
	ts := demoEpoch.Add(-30 * time.Second).Format(time.RFC3339)
	return []*unstructured.Unstructured{
		nodeMetrics(NodeControlPlane, "620m", "2.1Gi", ts),
		nodeMetrics(NodeWorker1, "1100m", "4.8Gi", ts),
	}
}

func nodeMetrics(name, cpu, mem, ts string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "metrics.k8s.io/v1beta1",
		"kind":       "NodeMetrics",
		"metadata": map[string]any{
			"name": name,
		},
		"timestamp": ts,
		"window":    "30s",
		"usage": map[string]any{
			"cpu":    cpu,
			"memory": mem,
		},
	}}
}
