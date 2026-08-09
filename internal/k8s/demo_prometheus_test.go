package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s/demo"
	"github.com/janosmiko/lfk/internal/model"
)

// withGlobalPrometheusConfig registers a `_global` monitoring config
// carrying a Prometheus block, mirroring a real user's
// ~/.config/lfk/config.yaml. Callers exercising a demo Client against this
// config reproduce the --demo startup crash: demo mode has no real cluster
// this config could plausibly point at, but the code must not learn that by
// panicking.
func withGlobalPrometheusConfig(t *testing.T) {
	t.Helper()
	prev := model.ConfigMonitoring
	t.Cleanup(func() { model.ConfigMonitoring = prev })
	model.ConfigMonitoring = map[string]model.MonitoringConfig{
		"_global": {
			Prometheus: &model.MonitoringEndpoint{
				Namespaces: []string{"monitoring"},
				Services:   []string{"prometheus"},
				Port:       "9090",
			},
			Alertmanager: &model.MonitoringEndpoint{
				Namespaces: []string{"monitoring"},
				Services:   []string{"alertmanager"},
				Port:       "9093",
			},
		},
	}
}

// TestNewDemoClient_GetAllNodeMetrics_PrometheusConfigured_NoPanic is the
// regression guard for the --demo startup crash on a machine with a real
// lfk config carrying a `_global` Prometheus block: resolveNodeMetricsConfig
// picks up the global config and routes node metrics through Prometheus
// first, and the demo backend's fake clientset panics inside ProxyGet
// itself. Node metrics must instead fall back to the metrics-api route,
// which the demo backend seeds real data for.
func TestNewDemoClient_GetAllNodeMetrics_PrometheusConfigured_NoPanic(t *testing.T) {
	withGlobalPrometheusConfig(t)

	c, err := NewDemoClient()
	require.NoError(t, err)
	t.Cleanup(c.Shutdown)

	metrics, err := c.GetAllNodeMetrics(t.Context(), c.CurrentContext())
	require.NoError(t, err, "must fall back to metrics-api, not error out")
	assert.Contains(t, metrics, demo.NodeControlPlane)
	assert.Contains(t, metrics, demo.NodeWorker1)
}

// TestNewDemoClient_GetAllPodMetrics_PrometheusConfigured_NoPanic mirrors
// TestNewDemoClient_GetAllNodeMetrics_PrometheusConfigured_NoPanic for the
// pod-metrics route, which shares the same Prometheus-first routing.
func TestNewDemoClient_GetAllPodMetrics_PrometheusConfigured_NoPanic(t *testing.T) {
	withGlobalPrometheusConfig(t)

	c, err := NewDemoClient()
	require.NoError(t, err)
	t.Cleanup(c.Shutdown)

	metrics, err := c.GetAllPodMetrics(t.Context(), c.CurrentContext(), demo.NamespaceDemo)
	require.NoError(t, err, "must fall back to metrics-api, not error out")
	assert.NotEmpty(t, metrics)
}

// TestNewDemoClient_GetNodeUptimes_PrometheusConfigured_NoPanic covers the
// node-uptime path, which shares queryPrometheusMetric with node CPU/mem
// metrics but has no metrics-api fallback (uptime is Prometheus-only) -- the
// important assertion here is the absence of a panic, not a populated
// result.
func TestNewDemoClient_GetNodeUptimes_PrometheusConfigured_NoPanic(t *testing.T) {
	withGlobalPrometheusConfig(t)

	c, err := NewDemoClient()
	require.NoError(t, err)
	t.Cleanup(c.Shutdown)

	uptimes, err := c.GetNodeUptimes(t.Context(), c.CurrentContext())
	assert.Error(t, err, "demo mode has no Prometheus to answer this query")
	assert.True(t, uptimes.Empty())
}

// TestNewDemoClient_GetActiveAlerts_PrometheusConfigured_NoPanic and
// TestNewDemoClient_GetAllActiveAlerts_PrometheusConfigured_NoPanic cover
// alerts.go's separate ProxyGet implementation, reachable from the Cluster
// Dashboard the same way node metrics are.
func TestNewDemoClient_GetActiveAlerts_PrometheusConfigured_NoPanic(t *testing.T) {
	withGlobalPrometheusConfig(t)

	c, err := NewDemoClient()
	require.NoError(t, err)
	t.Cleanup(c.Shutdown)

	alerts, err := c.GetActiveAlerts(t.Context(), c.CurrentContext(), demo.NamespaceDemo, "web", "Deployment")
	assert.Error(t, err)
	assert.Empty(t, alerts)
}

func TestNewDemoClient_GetAllActiveAlerts_PrometheusConfigured_NoPanic(t *testing.T) {
	withGlobalPrometheusConfig(t)

	c, err := NewDemoClient()
	require.NoError(t, err)
	t.Cleanup(c.Shutdown)

	alerts, err := c.GetAllActiveAlerts(t.Context(), c.CurrentContext(), "")
	assert.Error(t, err)
	assert.Empty(t, alerts)
}

// TestClient_RunPrometheusQuery_DemoMode_NoPanic covers the right-sizing
// Prometheus strategies' shared query path directly: GetRightsizing needs a
// real workload fixture to reach it, but runPrometheusQuery is the single
// choke point every Prometheus strategy funnels through.
func TestClient_RunPrometheusQuery_DemoMode_NoPanic(t *testing.T) {
	c, err := NewDemoClient()
	require.NoError(t, err)
	t.Cleanup(c.Shutdown)

	data, err := c.runPrometheusQuery(t.Context(), c.CurrentContext(), "up")
	assert.ErrorIs(t, err, errPrometheusUnavailableDemo)
	assert.Nil(t, data)
}
