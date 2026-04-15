package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/model"
)

// --- parsePodMetrics ---

func TestParsePodMetrics(t *testing.T) {
	t.Run("single container with CPU and memory", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "my-pod",
					"namespace": "default",
				},
				"containers": []interface{}{
					map[string]interface{}{
						"name": "app",
						"usage": map[string]interface{}{
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
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "multi-pod",
					"namespace": "kube-system",
				},
				"containers": []interface{}{
					map[string]interface{}{
						"name": "app",
						"usage": map[string]interface{}{
							"cpu":    "100m",
							"memory": "64Mi",
						},
					},
					map[string]interface{}{
						"name": "sidecar",
						"usage": map[string]interface{}{
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
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
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
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "pod",
					"namespace": "ns",
				},
				"containers": []interface{}{
					"not-a-map",
					map[string]interface{}{
						"name": "app",
						"usage": map[string]interface{}{
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
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "pod",
					"namespace": "ns",
				},
				"containers": []interface{}{
					map[string]interface{}{
						"name": "no-usage",
					},
					map[string]interface{}{
						"name": "with-usage",
						"usage": map[string]interface{}{
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
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "pod",
					"namespace": "ns",
				},
				"containers": []interface{}{
					map[string]interface{}{
						"name": "app",
						"usage": map[string]interface{}{
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
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "pod",
					"namespace": "ns",
				},
				"containers": []interface{}{
					map[string]interface{}{
						"name": "app",
						"usage": map[string]interface{}{
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
