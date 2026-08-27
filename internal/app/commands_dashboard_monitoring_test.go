package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestComposeMonitoringSearchHint(t *testing.T) {
	setConfig := func(t *testing.T, cfg map[string]model.MonitoringConfig) {
		t.Helper()
		orig := model.ConfigMonitoring
		model.ConfigMonitoring = cfg
		t.Cleanup(func() { model.ConfigMonitoring = orig })
	}

	t.Run("names the discovery labels when discovery runs", func(t *testing.T) {
		setConfig(t, nil)

		out := stripANSI(composeMonitoring("ctx", nil, "connection refused"))

		assert.Contains(t, out, "connection refused")
		assert.Contains(t, out, "vmselect")
	})

	t.Run("does not claim discovery ran when the config names both roles", func(t *testing.T) {
		setConfig(t, map[string]model.MonitoringConfig{
			"_global": {
				Prometheus:   &model.MonitoringEndpoint{Services: []string{"my-prom"}},
				Alertmanager: &model.MonitoringEndpoint{Services: []string{"my-am"}},
			},
		})

		out := stripANSI(composeMonitoring("ctx", nil, "connection refused"))

		assert.Contains(t, out, "my-prom")
		assert.NotContains(t, out, "vmselect")
	})
}
