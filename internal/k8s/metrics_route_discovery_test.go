package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/model"
)

// TestPrometheusAvailable covers the route decision's input: lfk must send pod
// and node metrics to Prometheus when discovery found one, not only when the
// user wrote a monitoring config.
func TestPrometheusAvailable(t *testing.T) {
	setConfig := func(t *testing.T, cfg map[string]model.MonitoringConfig) {
		t.Helper()
		orig := model.ConfigMonitoring
		model.ConfigMonitoring = cfg
		t.Cleanup(func() { model.ConfigMonitoring = orig })
	}

	t.Run("true when discovery found a Prometheus and no config exists", func(t *testing.T) {
		resetMonitoringDiscoveryCache()
		setConfig(t, nil)
		cs := k8sfake.NewClientset(monitoringSvc("monitoring", "vmselect-vmks", "vmselect", port("http", 8481)))

		assert.True(t, newFakeClient(cs, nil).prometheusAvailable(t.Context(), "ctx"))
	})

	t.Run("false when the cluster has no monitoring stack", func(t *testing.T) {
		resetMonitoringDiscoveryCache()
		setConfig(t, nil)
		cs := k8sfake.NewClientset(monitoringSvc("default", "web", "nginx", port("http", 80)))

		assert.False(t, newFakeClient(cs, nil).prometheusAvailable(t.Context(), "ctx"))
	})

	t.Run("true when the config names a prometheus endpoint", func(t *testing.T) {
		resetMonitoringDiscoveryCache()
		setConfig(t, map[string]model.MonitoringConfig{
			"_global": {Prometheus: &model.MonitoringEndpoint{Services: []string{"my-prom"}}},
		})
		cs := k8sfake.NewClientset()

		assert.True(t, newFakeClient(cs, nil).prometheusAvailable(t.Context(), "ctx"))
	})

	t.Run("false in demo mode", func(t *testing.T) {
		resetMonitoringDiscoveryCache()
		setConfig(t, nil)
		cs := k8sfake.NewClientset(monitoringSvc("monitoring", "vmsingle-vmks", "vmsingle", port("http", 8428)))
		c := newFakeClient(cs, nil)
		c.demo = true

		assert.False(t, c.prometheusAvailable(t.Context(), "ctx"))
	})
}

// TestDiscoveredPrometheusPicksTheRoute ties prometheusAvailable to the route
// order, which is the behaviour a user sees as CPU and MEM columns filling in.
func TestDiscoveredPrometheusPicksTheRoute(t *testing.T) {
	resetMonitoringDiscoveryCache()
	orig := model.ConfigMonitoring
	model.ConfigMonitoring = nil
	t.Cleanup(func() { model.ConfigMonitoring = orig })

	cs := k8sfake.NewClientset(monitoringSvc("monitoring", "vmsingle-vmks", "vmsingle", port("http", 8428)))
	c := newFakeClient(cs, nil)

	routes := selectNodeMetricsRoutes("", false, c.prometheusAvailable(t.Context(), "ctx"))

	assert.Equal(t, []nodeMetricsRoute{nodeMetricsRoutePrometheus, nodeMetricsRouteAPI}, routes)
}

// TestNodeUptimeEnabledByDiscovery covers the Uptime column, which stayed
// hidden because it read the same config-only signal.
func TestNodeUptimeEnabledByDiscovery(t *testing.T) {
	resetMonitoringDiscoveryCache()
	orig := model.ConfigMonitoring
	model.ConfigMonitoring = nil
	t.Cleanup(func() { model.ConfigMonitoring = orig })

	withProm := k8sfake.NewClientset(monitoringSvc("monitoring", "vmsingle-vmks", "vmsingle", port("http", 8428)))
	assert.True(t, newFakeClient(withProm, nil).nodeUptimeQueryEnabled(t.Context(), "ctx"))

	resetMonitoringDiscoveryCache()
	without := k8sfake.NewClientset()
	assert.False(t, newFakeClient(without, nil).nodeUptimeQueryEnabled(t.Context(), "ctx"))
}
