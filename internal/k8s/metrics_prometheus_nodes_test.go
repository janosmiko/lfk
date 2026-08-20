package k8s

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/model"
)

// --- parseNodeUptimeVector ---

func TestParseNodeUptimeVector(t *testing.T) {
	t.Run("node label lands in byName", func(t *testing.T) {
		data := `{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"node":"node-1"},"value":[1700000000,"3600"]}
		]}}`
		result, err := parseNodeUptimeVector([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, map[string]float64{"node-1": 3600}, result.byName)
		assert.Empty(t, result.byAddr)
	})

	t.Run("instance host:port strips port and lands in byAddr", func(t *testing.T) {
		data := `{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"instance":"10.0.1.5:9100"},"value":[1700000000,"120"]}
		]}}`
		result, err := parseNodeUptimeVector([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, map[string]float64{"10.0.1.5": 120}, result.byAddr)
		assert.Empty(t, result.byName)
	})

	t.Run("instance IPv6 host:port strips port", func(t *testing.T) {
		data := `{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"instance":"[::1]:9100"},"value":[1700000000,"42"]}
		]}}`
		result, err := parseNodeUptimeVector([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, map[string]float64{"::1": 42}, result.byAddr)
	})

	t.Run("instance without port kept verbatim", func(t *testing.T) {
		data := `{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"instance":"worker-1"},"value":[1700000000,"55"]}
		]}}`
		result, err := parseNodeUptimeVector([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, map[string]float64{"worker-1": 55}, result.byAddr)
	})

	t.Run("both node and instance labels populate both maps with the same value", func(t *testing.T) {
		data := `{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"node":"node-1","instance":"10.0.1.5:9100"},"value":[1700000000,"99"]}
		]}}`
		result, err := parseNodeUptimeVector([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, map[string]float64{"node-1": 99}, result.byName)
		assert.Equal(t, map[string]float64{"10.0.1.5": 99}, result.byAddr)
	})

	t.Run("kubernetes_node fallback label lands in byName", func(t *testing.T) {
		data := `{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"kubernetes_node":"alt-node"},"value":[1700000000,"10"]}
		]}}`
		result, err := parseNodeUptimeVector([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, map[string]float64{"alt-node": 10}, result.byName)
		assert.Empty(t, result.byAddr)
	})

	for _, label := range []string{"nodename", "host"} {
		t.Run(label+" fallback label lands in byAddr, not byName", func(t *testing.T) {
			data := `{"status":"success","data":{"resultType":"vector","result":[
				{"metric":{"` + label + `":"alt-node"},"value":[1700000000,"10"]}
			]}}`
			result, err := parseNodeUptimeVector([]byte(data))
			require.NoError(t, err)
			assert.Equal(t, map[string]float64{"alt-node": 10}, result.byAddr)
			assert.Empty(t, result.byName)
		})
	}

	// Regression guard for defect #4: a node's name colliding with another
	// node's stripped instance address must not overwrite either value --
	// each keyspace stays independent.
	t.Run("node name colliding with another series' instance address stays independent", func(t *testing.T) {
		data := `{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"node":"10.0.1.5"},"value":[1700000000,"100"]},
			{"metric":{"instance":"10.0.1.5:9100"},"value":[1700000000,"9999"]}
		]}}`
		result, err := parseNodeUptimeVector([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, map[string]float64{"10.0.1.5": 100}, result.byName)
		assert.Equal(t, map[string]float64{"10.0.1.5": 9999}, result.byAddr)
	})

	t.Run("empty label values are skipped", func(t *testing.T) {
		data := `{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"node":"","instance":""},"value":[1700000000,"10"]}
		]}}`
		result, err := parseNodeUptimeVector([]byte(data))
		require.NoError(t, err)
		assert.Empty(t, result.byName)
		assert.Empty(t, result.byAddr)
	})

	t.Run("unparseable sample value is skipped", func(t *testing.T) {
		data := `{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"node":"node-1"},"value":[1700000000,"not-a-number"]}
		]}}`
		result, err := parseNodeUptimeVector([]byte(data))
		require.NoError(t, err)
		assert.Empty(t, result.byName)
	})

	t.Run("negative value is skipped", func(t *testing.T) {
		data := `{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"node":"node-1"},"value":[1700000000,"-5"]}
		]}}`
		result, err := parseNodeUptimeVector([]byte(data))
		require.NoError(t, err)
		assert.Empty(t, result.byName)
	})

	for _, val := range []string{"NaN", "+Inf", "Inf", "-Inf"} {
		t.Run(val+" value is skipped", func(t *testing.T) {
			data := `{"status":"success","data":{"resultType":"vector","result":[
				{"metric":{"node":"node-1"},"value":[1700000000,"` + val + `"]}
			]}}`
			result, err := parseNodeUptimeVector([]byte(data))
			require.NoError(t, err)
			assert.Empty(t, result.byName, "%s must be skipped, not rendered as a bogus uptime", val)
		})
	}

	t.Run("non-success status returns error", func(t *testing.T) {
		data := `{"status":"error","errorType":"bad_data","error":"invalid query"}`
		_, err := parseNodeUptimeVector([]byte(data))
		assert.Error(t, err)
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		_, err := parseNodeUptimeVector([]byte(`{not-valid-json`))
		assert.Error(t, err)
	})
}

// --- nodeUptimeQueryEnabled ---

func TestNodeUptimeQueryEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]model.MonitoringConfig
		ctx  string
		want bool
	}{
		{
			name: "nil config",
			cfg:  nil,
			ctx:  "any-context",
			want: false,
		},
		{
			name: "node_metrics prometheus",
			cfg: map[string]model.MonitoringConfig{
				"my-ctx": {NodeMetrics: "prometheus"},
			},
			ctx:  "my-ctx",
			want: true,
		},
		{
			name: "explicit prometheus endpoint block",
			cfg: map[string]model.MonitoringConfig{
				"my-ctx": {Prometheus: &model.MonitoringEndpoint{Namespaces: []string{"monitoring"}}},
			},
			ctx:  "my-ctx",
			want: true,
		},
		{
			name: "node_metrics metrics-api with no prometheus block",
			cfg: map[string]model.MonitoringConfig{
				"my-ctx": {NodeMetrics: "metrics-api"},
			},
			ctx:  "my-ctx",
			want: false,
		},
		{
			name: "_global fallback with prometheus",
			cfg: map[string]model.MonitoringConfig{
				"_global": {NodeMetrics: "prometheus"},
			},
			ctx:  "unrelated-ctx",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := model.ConfigMonitoring
			t.Cleanup(func() { model.ConfigMonitoring = prev })
			model.ConfigMonitoring = tt.cfg

			assert.Equal(t, tt.want, nodeUptimeQueryEnabled(tt.ctx))
		})
	}
}

// --- GetNodeUptimes ---

func TestGetNodeUptimes_DisabledReturnsEmpty(t *testing.T) {
	prev := model.ConfigMonitoring
	t.Cleanup(func() { model.ConfigMonitoring = prev })
	model.ConfigMonitoring = nil

	c := &Client{}
	result, err := c.GetNodeUptimes(t.Context(), "any-context")
	require.NoError(t, err)
	assert.True(t, result.Empty())
}

// fakeProxyResponse implements rest.ResponseWrapper for injecting a canned
// Prometheus proxy response through a fake clientset's ProxyReactor chain.
type fakeProxyResponse struct {
	body []byte
	err  error
}

func (f fakeProxyResponse) DoRaw(context.Context) ([]byte, error) { return f.body, f.err }

func (f fakeProxyResponse) Stream(context.Context) (io.ReadCloser, error) {
	return nil, nil
}

func TestGetNodeUptimes_EnabledQueriesPrometheus(t *testing.T) {
	prev := model.ConfigMonitoring
	t.Cleanup(func() { model.ConfigMonitoring = prev })
	model.ConfigMonitoring = map[string]model.MonitoringConfig{
		"test-ctx": {
			Prometheus: &model.MonitoringEndpoint{
				Namespaces: []string{"monitoring"},
				Services:   []string{"prometheus"},
				Port:       "9090",
			},
		},
	}

	cs := fake.NewSimpleClientset(&corev1.Service{
		Name: "prometheus", Namespace: "monitoring",
	})
	cs.PrependProxyReactor("services", func(clienttesting.Action) (bool, restclient.ResponseWrapper, error) {
		body := []byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"node":"node-1"},"value":[1700000000,"7200"]}
		]}}`)
		return true, fakeProxyResponse{body: body}, nil
	})

	c := NewTestClient(cs, nil)
	result, err := c.GetNodeUptimes(t.Context(), "test-ctx")
	require.NoError(t, err)
	require.Contains(t, result.ByName, "node-1")
	assert.Equal(t, 2*time.Hour, result.ByName["node-1"])
	assert.Empty(t, result.ByAddr)
}

func TestGetNodeUptimes_QueryErrorIsWrapped(t *testing.T) {
	prev := model.ConfigMonitoring
	t.Cleanup(func() { model.ConfigMonitoring = prev })
	model.ConfigMonitoring = map[string]model.MonitoringConfig{
		"test-ctx": {
			Prometheus: &model.MonitoringEndpoint{
				Namespaces: []string{"monitoring"},
				Services:   []string{"prometheus"},
				Port:       "9090",
			},
		},
	}

	cs := fake.NewSimpleClientset()
	cs.PrependProxyReactor("services", func(clienttesting.Action) (bool, restclient.ResponseWrapper, error) {
		return true, fakeProxyResponse{err: assert.AnError}, nil
	})

	c := NewTestClient(cs, nil)
	result, err := c.GetNodeUptimes(t.Context(), "test-ctx")
	require.Error(t, err)
	assert.True(t, result.Empty())
}

// TestGetNodeUptimes_NameAddrKeyspacesStayIndependent is the end-to-end
// regression guard for defect #4: a node whose name collides with another
// node's instance address must not overwrite either value once GetNodeUptimes
// converts seconds to durations.
func TestGetNodeUptimes_NameAddrKeyspacesStayIndependent(t *testing.T) {
	prev := model.ConfigMonitoring
	t.Cleanup(func() { model.ConfigMonitoring = prev })
	model.ConfigMonitoring = map[string]model.MonitoringConfig{
		"test-ctx": {
			Prometheus: &model.MonitoringEndpoint{
				Namespaces: []string{"monitoring"},
				Services:   []string{"prometheus"},
				Port:       "9090",
			},
		},
	}

	cs := fake.NewSimpleClientset(&corev1.Service{
		Name: "prometheus", Namespace: "monitoring",
	})
	cs.PrependProxyReactor("services", func(clienttesting.Action) (bool, restclient.ResponseWrapper, error) {
		body := []byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"node":"10.0.1.5"},"value":[1700000000,"3600"]},
			{"metric":{"instance":"10.0.1.5:9100"},"value":[1700000000,"7200"]}
		]}}`)
		return true, fakeProxyResponse{body: body}, nil
	})

	c := NewTestClient(cs, nil)
	result, err := c.GetNodeUptimes(t.Context(), "test-ctx")
	require.NoError(t, err)
	require.Contains(t, result.ByName, "10.0.1.5")
	require.Contains(t, result.ByAddr, "10.0.1.5")
	assert.Equal(t, time.Hour, result.ByName["10.0.1.5"])
	assert.Equal(t, 2*time.Hour, result.ByAddr["10.0.1.5"])
}
