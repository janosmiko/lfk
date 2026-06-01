package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// saveRateGlobals snapshots and restores the package-level rate-limit config so
// tests don't leak state into each other.
func saveRateGlobals(t *testing.T) {
	t.Helper()
	origQPS, origBurst := ConfigK8sClientQPS, ConfigK8sClientBurst
	origCQPS, origCBurst := ConfigClusterK8sClientQPS, ConfigClusterK8sClientBurst
	t.Cleanup(func() {
		ConfigK8sClientQPS, ConfigK8sClientBurst = origQPS, origBurst
		ConfigClusterK8sClientQPS, ConfigClusterK8sClientBurst = origCQPS, origCBurst
	})
	ConfigK8sClientQPS, ConfigK8sClientBurst = 50, 100
	ConfigClusterK8sClientQPS = map[string]int{}
	ConfigClusterK8sClientBurst = map[string]int{}
}

func TestResolveK8sClientRate(t *testing.T) {
	t.Run("default global", func(t *testing.T) {
		saveRateGlobals(t)
		qps, burst := ResolveK8sClientRate("any")
		assert.Equal(t, float32(50), qps)
		assert.Equal(t, 100, burst)
	})

	t.Run("global override", func(t *testing.T) {
		saveRateGlobals(t)
		ConfigK8sClientQPS, ConfigK8sClientBurst = 80, 160
		qps, burst := ResolveK8sClientRate("any")
		assert.Equal(t, float32(80), qps)
		assert.Equal(t, 160, burst)
	})

	t.Run("per-cluster overrides global", func(t *testing.T) {
		saveRateGlobals(t)
		ConfigK8sClientQPS, ConfigK8sClientBurst = 50, 100
		ConfigClusterK8sClientQPS = map[string]int{"prod": 200}
		ConfigClusterK8sClientBurst = map[string]int{"prod": 400}

		qps, burst := ResolveK8sClientRate("prod")
		assert.Equal(t, float32(200), qps, "per-cluster QPS wins over global")
		assert.Equal(t, 400, burst, "per-cluster Burst wins over global")

		qps, burst = ResolveK8sClientRate("dev")
		assert.Equal(t, float32(50), qps, "other contexts fall back to global")
		assert.Equal(t, 100, burst)
	})

	t.Run("partial per-cluster override keeps global for the other field", func(t *testing.T) {
		saveRateGlobals(t)
		ConfigK8sClientQPS, ConfigK8sClientBurst = 50, 100
		ConfigClusterK8sClientQPS = map[string]int{"prod": 200} // only QPS overridden

		qps, burst := ResolveK8sClientRate("prod")
		assert.Equal(t, float32(200), qps)
		assert.Equal(t, 100, burst, "Burst falls back to global when not overridden")
	})
}
