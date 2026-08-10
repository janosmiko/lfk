package k8s

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// --- safeProxyGetRaw ---

// TestSafeProxyGetRaw_LogsRecoveredPanicWithStack guards TASK-865 finding 3:
// safeProxyGetRaw's recover silently converted a panic into an error with no
// log line and no stack, so a genuine client-go panic on a real cluster
// would vanish without a trace. The recover stays broad (every caller here
// already runs only on real-cluster paths — queryPrometheusMetric turns
// demo queries away before ever reaching this function, so a demo-scoped
// recover would never fire), but it must now log the recovered value and a
// stack trace before returning the error.
func TestSafeProxyGetRaw_LogsRecoveredPanicWithStack(t *testing.T) {
	buf := &bytes.Buffer{}
	orig := logger.Logger
	logger.Logger = slog.New(slog.NewTextHandler(buf, nil))
	defer func() { logger.Logger = orig }()

	cs := k8sfake.NewClientset()
	cs.PrependProxyReactor("services", func(_ k8stesting.Action) (bool, restclient.ResponseWrapper, error) {
		panic("boom: proxy exploded")
	})

	_, err := safeProxyGetRaw(t.Context(), cs, "monitoring", "prometheus", "9090", "/api/v1/query", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom: proxy exploded")

	logged := buf.String()
	assert.Contains(t, logged, "boom: proxy exploded", "expected the recovered panic value to be logged")
	assert.Contains(t, logged, "goroutine", "expected a stack trace to be logged alongside the recovered panic")
}

// TestSafeProxyGetRaw_RedactsRecoveredPanicValue guards against a client-go
// panic value carrying a bearer token or similar secret straight into the
// log and the returned error -- the panic value comes from a live cluster
// call and is not trusted content, same as any other log/error source.
func TestSafeProxyGetRaw_RedactsRecoveredPanicValue(t *testing.T) {
	buf := &bytes.Buffer{}
	orig := logger.Logger
	logger.Logger = slog.New(slog.NewTextHandler(buf, nil))
	defer func() { logger.Logger = orig }()

	const secret = "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	cs := k8sfake.NewClientset()
	cs.PrependProxyReactor("services", func(_ k8stesting.Action) (bool, restclient.ResponseWrapper, error) {
		panic(secret)
	})

	_, err := safeProxyGetRaw(t.Context(), cs, "monitoring", "prometheus", "9090", "/api/v1/query", nil)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret, "returned error must not carry the raw panic value")
	assert.Contains(t, err.Error(), "[REDACTED", "returned error should show the redacted panic value")

	logged := buf.String()
	assert.NotContains(t, logged, secret, "log line must not carry the raw panic value")
	assert.Contains(t, logged, "[REDACTED", "log line should show the redacted panic value")
}

// --- parsePodMetrics ---

func TestParsePodMetrics(t *testing.T) {
	t.Run("single container with CPU and memory", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name":      "my-pod",
					"namespace": "default",
				},
				"containers": []any{
					map[string]any{
						"name": "app",
						"usage": map[string]any{
							"cpu":    "250m",
							"memory": "128Mi",
						},
					},
				},
			},
		}

		metrics, err := parsePodMetrics(obj)
		require.NoError(t, err)
		assert.Equal(t, "my-pod", metrics.Name)
		assert.Equal(t, "default", metrics.Namespace)
		assert.Equal(t, int64(250), metrics.CPU)
		assert.Equal(t, int64(128*1024*1024), metrics.Memory)
	})

	t.Run("multiple containers summed", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name":      "multi-pod",
					"namespace": "kube-system",
				},
				"containers": []any{
					map[string]any{
						"name": "app",
						"usage": map[string]any{
							"cpu":    "100m",
							"memory": "64Mi",
						},
					},
					map[string]any{
						"name": "sidecar",
						"usage": map[string]any{
							"cpu":    "50m",
							"memory": "32Mi",
						},
					},
				},
			},
		}

		metrics, err := parsePodMetrics(obj)
		require.NoError(t, err)
		assert.Equal(t, int64(150), metrics.CPU)
		assert.Equal(t, int64(96*1024*1024), metrics.Memory)
	})

	t.Run("no containers returns error", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": "empty-pod",
				},
			},
		}

		_, err := parsePodMetrics(obj)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no containers")
	})

	t.Run("container entry not a map is skipped", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name":      "pod",
					"namespace": "ns",
				},
				"containers": []any{
					"not-a-map",
					map[string]any{
						"name": "app",
						"usage": map[string]any{
							"cpu": "100m",
						},
					},
				},
			},
		}

		metrics, err := parsePodMetrics(obj)
		require.NoError(t, err)
		assert.Equal(t, int64(100), metrics.CPU)
	})

	t.Run("container without usage is skipped", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name":      "pod",
					"namespace": "ns",
				},
				"containers": []any{
					map[string]any{
						"name": "no-usage",
					},
					map[string]any{
						"name": "with-usage",
						"usage": map[string]any{
							"memory": "256Mi",
						},
					},
				},
			},
		}

		metrics, err := parsePodMetrics(obj)
		require.NoError(t, err)
		assert.Equal(t, int64(0), metrics.CPU)
		assert.Equal(t, int64(256*1024*1024), metrics.Memory)
	})

	t.Run("container with only CPU no memory", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name":      "pod",
					"namespace": "ns",
				},
				"containers": []any{
					map[string]any{
						"name": "app",
						"usage": map[string]any{
							"cpu": "500m",
						},
					},
				},
			},
		}

		metrics, err := parsePodMetrics(obj)
		require.NoError(t, err)
		assert.Equal(t, int64(500), metrics.CPU)
		assert.Equal(t, int64(0), metrics.Memory)
	})

	t.Run("unparseable quantity is ignored", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name":      "pod",
					"namespace": "ns",
				},
				"containers": []any{
					map[string]any{
						"name": "app",
						"usage": map[string]any{
							"cpu":    "not-a-quantity",
							"memory": "also-invalid",
						},
					},
				},
			},
		}

		metrics, err := parsePodMetrics(obj)
		require.NoError(t, err)
		assert.Equal(t, int64(0), metrics.CPU)
		assert.Equal(t, int64(0), metrics.Memory)
	})
}

// --- parsePrometheusNodeResponse ---

func TestParsePrometheusNodeResponse(t *testing.T) {
	t.Run("valid response with node label", func(t *testing.T) {
		data := `{
			"status": "success",
			"data": {
				"resultType": "vector",
				"result": [
					{
						"metric": {"node": "node-1"},
						"value": [1234567890, "500.5"]
					},
					{
						"metric": {"node": "node-2"},
						"value": [1234567890, "300.25"]
					}
				]
			}
		}`

		result, err := parsePrometheusNodeResponse([]byte(data))
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.InDelta(t, 500.5, result["node-1"], 0.01)
		assert.InDelta(t, 300.25, result["node-2"], 0.01)
	})

	t.Run("fallback to instance label", func(t *testing.T) {
		data := `{
			"status": "success",
			"data": {
				"resultType": "vector",
				"result": [
					{
						"metric": {"instance": "worker-1"},
						"value": [1234567890, "100"]
					}
				]
			}
		}`

		result, err := parsePrometheusNodeResponse([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, 100.0, result["worker-1"])
	})

	t.Run("fallback to kubernetes_node label", func(t *testing.T) {
		data := `{
			"status": "success",
			"data": {
				"resultType": "vector",
				"result": [
					{
						"metric": {"kubernetes_node": "k8s-node-1"},
						"value": [1234567890, "42"]
					}
				]
			}
		}`

		result, err := parsePrometheusNodeResponse([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, 42.0, result["k8s-node-1"])
	})

	t.Run("fallback to nodename label", func(t *testing.T) {
		data := `{
			"status": "success",
			"data": {
				"resultType": "vector",
				"result": [
					{
						"metric": {"nodename": "my-node"},
						"value": [1234567890, "77"]
					}
				]
			}
		}`

		result, err := parsePrometheusNodeResponse([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, 77.0, result["my-node"])
	})

	t.Run("fallback to host label", func(t *testing.T) {
		data := `{
			"status": "success",
			"data": {
				"resultType": "vector",
				"result": [
					{
						"metric": {"host": "host-1"},
						"value": [1234567890, "55"]
					}
				]
			}
		}`

		result, err := parsePrometheusNodeResponse([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, 55.0, result["host-1"])
	})

	t.Run("no node label at all is skipped", func(t *testing.T) {
		data := `{
			"status": "success",
			"data": {
				"resultType": "vector",
				"result": [
					{
						"metric": {"unknown_label": "val"},
						"value": [1234567890, "100"]
					}
				]
			}
		}`

		result, err := parsePrometheusNodeResponse([]byte(data))
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("error status returns error", func(t *testing.T) {
		data := `{
			"status": "error",
			"errorType": "bad_data",
			"error": "invalid query"
		}`

		_, err := parsePrometheusNodeResponse([]byte(data))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		_, err := parsePrometheusNodeResponse([]byte(`{not-valid-json`))
		assert.Error(t, err)
	})

	t.Run("empty result set", func(t *testing.T) {
		data := `{
			"status": "success",
			"data": {
				"resultType": "vector",
				"result": []
			}
		}`

		result, err := parsePrometheusNodeResponse([]byte(data))
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("value array too short is skipped", func(t *testing.T) {
		data := `{
			"status": "success",
			"data": {
				"resultType": "vector",
				"result": [
					{
						"metric": {"node": "node-1"},
						"value": [1234567890]
					}
				]
			}
		}`

		result, err := parsePrometheusNodeResponse([]byte(data))
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("unparseable value string is skipped", func(t *testing.T) {
		// The value[1] is a valid JSON string but not a float.
		data := `{
			"status": "success",
			"data": {
				"resultType": "vector",
				"result": [
					{
						"metric": {"node": "node-1"},
						"value": [1234567890, "not-a-number"]
					}
				]
			}
		}`

		result, err := parsePrometheusNodeResponse([]byte(data))
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("value[1] not a JSON string is skipped", func(t *testing.T) {
		// value[1] is a raw number instead of a JSON string.
		data := `{
			"status": "success",
			"data": {
				"resultType": "vector",
				"result": [
					{
						"metric": {"node": "node-1"},
						"value": [1234567890, 100.5]
					}
				]
			}
		}`

		result, err := parsePrometheusNodeResponse([]byte(data))
		require.NoError(t, err)
		// json.Unmarshal into string will fail for a raw number.
		assert.Empty(t, result)
	})
}

// --- resolveNodeMetricsConfig ---

func TestResolveNodeMetricsConfig(t *testing.T) {
	t.Run("nil config returns empty", func(t *testing.T) {
		origCfg := model.ConfigMonitoring
		model.ConfigMonitoring = nil
		defer func() { model.ConfigMonitoring = origCfg }()

		nm, hp := resolveNodeMetricsConfig("any-context")
		assert.Equal(t, "", nm)
		assert.False(t, hp)
	})

	t.Run("exact context match", func(t *testing.T) {
		origCfg := model.ConfigMonitoring
		model.ConfigMonitoring = map[string]model.MonitoringConfig{
			"my-ctx": {
				NodeMetrics: "prometheus",
				Prometheus: &model.MonitoringEndpoint{
					Namespaces: []string{"monitoring"},
					Services:   []string{"prometheus"},
				},
			},
		}
		defer func() { model.ConfigMonitoring = origCfg }()

		nm, hp := resolveNodeMetricsConfig("my-ctx")
		assert.Equal(t, "prometheus", nm)
		assert.True(t, hp)
	})

	t.Run("falls back to _global config", func(t *testing.T) {
		origCfg := model.ConfigMonitoring
		model.ConfigMonitoring = map[string]model.MonitoringConfig{
			"_global": {
				NodeMetrics: "metrics-api",
			},
		}
		defer func() { model.ConfigMonitoring = origCfg }()

		nm, hp := resolveNodeMetricsConfig("unknown-ctx")
		assert.Equal(t, "metrics-api", nm)
		assert.False(t, hp)
	})

	t.Run("no matching context and no default", func(t *testing.T) {
		origCfg := model.ConfigMonitoring
		model.ConfigMonitoring = map[string]model.MonitoringConfig{
			"other-ctx": {NodeMetrics: "prometheus"},
		}
		defer func() { model.ConfigMonitoring = origCfg }()

		nm, hp := resolveNodeMetricsConfig("unrelated")
		assert.Equal(t, "", nm)
		assert.False(t, hp)
	})
}

// Regression guard: a `_global` MonitoringConfig with a Prometheus block
// is a common shared default. Before the routing fallback, any cluster
// without its own per-context entry was hard-routed to Prometheus, and
// metrics-server-only clusters (e.g. EKS without kube-prometheus-stack)
// silently rendered n/a everywhere when the Prometheus probe failed.
// The fallback restores metrics-api as the safety net.
func TestSelectNodeMetricsRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		nodeMetrics   string
		hasPrometheus bool
		want          []nodeMetricsRoute
	}{
		{
			name:        "nothing configured tries metrics-api only",
			nodeMetrics: "", hasPrometheus: false,
			want: []nodeMetricsRoute{nodeMetricsRouteAPI},
		},
		{
			name:        "implicit prometheus (e.g. _global) falls back to metrics-api",
			nodeMetrics: "", hasPrometheus: true,
			want: []nodeMetricsRoute{nodeMetricsRoutePrometheus, nodeMetricsRouteAPI},
		},
		{
			name:        "explicit prometheus falls back to metrics-api",
			nodeMetrics: "prometheus", hasPrometheus: true,
			want: []nodeMetricsRoute{nodeMetricsRoutePrometheus, nodeMetricsRouteAPI},
		},
		{
			name:        "explicit metrics-api with prometheus available falls back to prometheus",
			nodeMetrics: "metrics-api", hasPrometheus: true,
			want: []nodeMetricsRoute{nodeMetricsRouteAPI, nodeMetricsRoutePrometheus},
		},
		{
			name:        "explicit metrics-api without prometheus does not attempt prometheus fallback",
			nodeMetrics: "metrics-api", hasPrometheus: false,
			want: []nodeMetricsRoute{nodeMetricsRouteAPI},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := selectNodeMetricsRoutes(tt.nodeMetrics, tt.hasPrometheus)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNodeMetricsRouteString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "metrics-api", nodeMetricsRouteAPI.String())
	assert.Equal(t, "prometheus", nodeMetricsRoutePrometheus.String())
}

func TestParsePodMetricsByContainer(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "pod-x",
				"namespace": "ns",
			},
			"containers": []any{
				map[string]any{
					"name": "app",
					"usage": map[string]any{
						"cpu":    "120m",
						"memory": "200Mi",
					},
				},
				map[string]any{
					"name": "proxy",
					"usage": map[string]any{
						"cpu":    "30m",
						"memory": "64Mi",
					},
				},
			},
		},
	}

	out, err := parsePodMetricsByContainer(obj)
	assert.NoError(t, err)
	assert.Len(t, out, 2, "two containers reported")
	assert.Equal(t, int64(120), out["app"].CPUMilli)
	assert.Equal(t, int64(200*1024*1024), out["app"].MemBytes)
	assert.Equal(t, int64(30), out["proxy"].CPUMilli)
	assert.Equal(t, int64(64*1024*1024), out["proxy"].MemBytes)
}

func TestParsePodMetricsByContainer_NoContainers(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "p", "namespace": "ns"},
	}}
	_, err := parsePodMetricsByContainer(obj)
	assert.Error(t, err, "no containers field is an error so callers can distinguish from zero-usage")
}
