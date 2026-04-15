package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// --- parseAllAlerts ---

func TestParseAllAlerts(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		namespace string
		wantLen   int
		wantErr   string
		check     func(t *testing.T, alerts []AlertInfo)
	}{
		{
			name: "returns all alerts when namespace is empty",
			data: `{
				"status": "success",
				"data": {
					"alerts": [
						{
							"labels": {"alertname": "A1", "namespace": "ns1", "severity": "warning"},
							"annotations": {"summary": "alert one"},
							"state": "firing",
							"activeAt": "2025-01-01T00:00:00Z"
						},
						{
							"labels": {"alertname": "A2", "namespace": "ns2", "severity": "critical"},
							"annotations": {"summary": "alert two"},
							"state": "pending",
							"activeAt": "2025-01-02T00:00:00Z"
						}
					]
				}
			}`,
			namespace: "",
			wantLen:   2,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "A1", alerts[0].Name)
				assert.Equal(t, "firing", alerts[0].State)
				assert.Equal(t, "warning", alerts[0].Severity)
				assert.Equal(t, "alert one", alerts[0].Summary)
				assert.Equal(t, "A2", alerts[1].Name)
				assert.Equal(t, "pending", alerts[1].State)
				assert.Equal(t, "critical", alerts[1].Severity)
			},
		},
		{
			name: "filters by namespace when specified",
			data: `{
				"status": "success",
				"data": {
					"alerts": [
						{
							"labels": {"alertname": "A1", "namespace": "ns1"},
							"annotations": {},
							"state": "firing",
							"activeAt": "2025-01-01T00:00:00Z"
						},
						{
							"labels": {"alertname": "A2", "namespace": "ns2"},
							"annotations": {},
							"state": "firing",
							"activeAt": "2025-01-02T00:00:00Z"
						}
					]
				}
			}`,
			namespace: "ns1",
			wantLen:   1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "A1", alerts[0].Name)
			},
		},
		{
			name: "includes alerts without namespace label when namespace filter is set",
			data: `{
				"status": "success",
				"data": {
					"alerts": [
						{
							"labels": {"alertname": "ClusterWide"},
							"annotations": {},
							"state": "firing",
							"activeAt": "2025-01-01T00:00:00Z"
						}
					]
				}
			}`,
			namespace: "default",
			wantLen:   1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "ClusterWide", alerts[0].Name)
			},
		},
		{
			name: "extracts grafana_url annotation",
			data: `{
				"status": "success",
				"data": {
					"alerts": [{
						"labels": {"alertname": "X"},
						"annotations": {"grafana_url": "https://grafana.example.com/d/abc"},
						"state": "firing",
						"activeAt": "2025-01-01T00:00:00Z"
					}]
				}
			}`,
			namespace: "",
			wantLen:   1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "https://grafana.example.com/d/abc", alerts[0].GrafanaURL)
			},
		},
		{
			name: "falls back to dashboard_url annotation",
			data: `{
				"status": "success",
				"data": {
					"alerts": [{
						"labels": {"alertname": "X"},
						"annotations": {"dashboard_url": "https://dash.example.com"},
						"state": "firing",
						"activeAt": "2025-01-01T00:00:00Z"
					}]
				}
			}`,
			namespace: "",
			wantLen:   1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "https://dash.example.com", alerts[0].GrafanaURL)
			},
		},
		{
			name: "falls back to runbook_url annotation",
			data: `{
				"status": "success",
				"data": {
					"alerts": [{
						"labels": {"alertname": "X"},
						"annotations": {"runbook_url": "https://runbook.example.com"},
						"state": "firing",
						"activeAt": "2025-01-01T00:00:00Z"
					}]
				}
			}`,
			namespace: "",
			wantLen:   1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "https://runbook.example.com", alerts[0].GrafanaURL)
			},
		},
		{
			name: "no annotations leaves GrafanaURL empty",
			data: `{
				"status": "success",
				"data": {
					"alerts": [{
						"labels": {"alertname": "X"},
						"annotations": {},
						"state": "firing",
						"activeAt": "2025-01-01T00:00:00Z"
					}]
				}
			}`,
			namespace: "",
			wantLen:   1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Empty(t, alerts[0].GrafanaURL)
			},
		},
		{
			name:      "returns error for non-success status",
			data:      `{"status": "error", "data": {"alerts": []}}`,
			namespace: "",
			wantErr:   "status: error",
		},
		{
			name:      "returns error for invalid JSON",
			data:      `{invalid`,
			namespace: "",
			wantErr:   "unmarshal",
		},
		{
			name: "empty alerts array returns empty slice",
			data: `{
				"status": "success",
				"data": {"alerts": []}
			}`,
			namespace: "",
			wantLen:   0,
		},
		{
			name: "extracts description annotation",
			data: `{
				"status": "success",
				"data": {
					"alerts": [{
						"labels": {"alertname": "A", "severity": "info"},
						"annotations": {"description": "detailed info here"},
						"state": "pending",
						"activeAt": "2025-06-15T10:30:00Z"
					}]
				}
			}`,
			namespace: "",
			wantLen:   1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "detailed info here", alerts[0].Description)
				assert.Equal(t, "info", alerts[0].Severity)
				assert.False(t, alerts[0].Since.IsZero())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alerts, err := parseAllAlerts([]byte(tt.data), tt.namespace)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Len(t, alerts, tt.wantLen)
			if tt.check != nil {
				tt.check(t, alerts)
			}
		})
	}
}

// --- parseAlertmanagerAlerts ---

func TestParseAlertmanagerAlerts(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		namespace string
		wantLen   int
		wantErr   string
		check     func(t *testing.T, alerts []AlertInfo)
	}{
		{
			name: "parses active alerts",
			data: `[
				{
					"labels": {"alertname": "HighCPU", "namespace": "prod", "severity": "critical"},
					"annotations": {"summary": "CPU usage high", "description": "Pod at 95%"},
					"status": {"state": "active"},
					"startsAt": "2025-01-01T12:00:00Z"
				}
			]`,
			namespace: "",
			wantLen:   1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "HighCPU", alerts[0].Name)
				assert.Equal(t, "firing", alerts[0].State)
				assert.Equal(t, "critical", alerts[0].Severity)
				assert.Equal(t, "CPU usage high", alerts[0].Summary)
				assert.Equal(t, "Pod at 95%", alerts[0].Description)
			},
		},
		{
			name: "unprocessed state maps to pending",
			data: `[
				{
					"labels": {"alertname": "SlowQuery", "namespace": "default"},
					"annotations": {},
					"status": {"state": "unprocessed"},
					"startsAt": "2025-01-01T00:00:00Z"
				}
			]`,
			namespace: "",
			wantLen:   1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "pending", alerts[0].State)
			},
		},
		{
			name: "suppressed alerts are filtered out",
			data: `[
				{
					"labels": {"alertname": "Silenced"},
					"annotations": {},
					"status": {"state": "suppressed"},
					"startsAt": "2025-01-01T00:00:00Z"
				},
				{
					"labels": {"alertname": "Active"},
					"annotations": {},
					"status": {"state": "active"},
					"startsAt": "2025-01-01T00:00:00Z"
				}
			]`,
			namespace: "",
			wantLen:   1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "Active", alerts[0].Name)
			},
		},
		{
			name: "filters by namespace",
			data: `[
				{
					"labels": {"alertname": "A1", "namespace": "ns1"},
					"annotations": {},
					"status": {"state": "active"},
					"startsAt": "2025-01-01T00:00:00Z"
				},
				{
					"labels": {"alertname": "A2", "namespace": "ns2"},
					"annotations": {},
					"status": {"state": "active"},
					"startsAt": "2025-01-01T00:00:00Z"
				}
			]`,
			namespace: "ns1",
			wantLen:   1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "A1", alerts[0].Name)
			},
		},
		{
			name: "includes alerts without namespace label when namespace filter is set",
			data: `[
				{
					"labels": {"alertname": "ClusterAlert"},
					"annotations": {},
					"status": {"state": "active"},
					"startsAt": "2025-01-01T00:00:00Z"
				}
			]`,
			namespace: "default",
			wantLen:   1,
		},
		{
			name: "extracts grafana_url annotation",
			data: `[
				{
					"labels": {"alertname": "X"},
					"annotations": {"grafana_url": "https://g.example.com/d/1"},
					"status": {"state": "active"},
					"startsAt": "2025-01-01T00:00:00Z"
				}
			]`,
			namespace: "",
			wantLen:   1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "https://g.example.com/d/1", alerts[0].GrafanaURL)
			},
		},
		{
			name:    "returns error for invalid JSON",
			data:    `not-json`,
			wantErr: "unmarshal",
		},
		{
			name:      "empty array returns empty slice",
			data:      `[]`,
			namespace: "",
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alerts, err := parseAlertmanagerAlerts([]byte(tt.data), tt.namespace)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Len(t, alerts, tt.wantLen)
			if tt.check != nil {
				tt.check(t, alerts)
			}
		})
	}
}

// --- parseAndFilterAlertmanagerAlerts ---

func TestParseAndFilterAlertmanagerAlerts(t *testing.T) {
	tests := []struct {
		name         string
		data         string
		resourceNs   string
		resourceName string
		resourceKind string
		wantLen      int
		wantErr      string
		check        func(t *testing.T, alerts []AlertInfo)
	}{
		{
			name: "filters by resource labels",
			data: `[
				{
					"labels": {"alertname": "PodCrash", "namespace": "default", "pod": "nginx-abc"},
					"annotations": {"summary": "Pod crashing"},
					"status": {"state": "active"},
					"startsAt": "2025-01-01T00:00:00Z"
				},
				{
					"labels": {"alertname": "OtherAlert", "namespace": "default", "pod": "redis"},
					"annotations": {},
					"status": {"state": "active"},
					"startsAt": "2025-01-01T00:00:00Z"
				}
			]`,
			resourceNs:   "default",
			resourceName: "nginx",
			resourceKind: "Deployment",
			wantLen:      1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "PodCrash", alerts[0].Name)
				assert.Equal(t, "firing", alerts[0].State)
			},
		},
		{
			name: "suppressed alerts are excluded",
			data: `[
				{
					"labels": {"alertname": "X", "namespace": "ns", "pod": "my-pod"},
					"annotations": {},
					"status": {"state": "suppressed"},
					"startsAt": "2025-01-01T00:00:00Z"
				}
			]`,
			resourceNs:   "ns",
			resourceName: "my-pod",
			resourceKind: "Pod",
			wantLen:      0,
		},
		{
			name: "unprocessed maps to pending state",
			data: `[
				{
					"labels": {"alertname": "X", "namespace": "ns", "deployment": "app"},
					"annotations": {},
					"status": {"state": "unprocessed"},
					"startsAt": "2025-01-01T00:00:00Z"
				}
			]`,
			resourceNs:   "ns",
			resourceName: "app",
			resourceKind: "Deployment",
			wantLen:      1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "pending", alerts[0].State)
			},
		},
		{
			name: "extracts dashboard_url fallback",
			data: `[
				{
					"labels": {"alertname": "X", "namespace": "ns", "pod": "p"},
					"annotations": {"dashboard_url": "https://dash.example.com"},
					"status": {"state": "active"},
					"startsAt": "2025-01-01T00:00:00Z"
				}
			]`,
			resourceNs:   "ns",
			resourceName: "p",
			resourceKind: "Pod",
			wantLen:      1,
			check: func(t *testing.T, alerts []AlertInfo) {
				t.Helper()
				assert.Equal(t, "https://dash.example.com", alerts[0].GrafanaURL)
			},
		},
		{
			name:         "returns error for invalid JSON",
			data:         `{broken`,
			resourceNs:   "ns",
			resourceName: "x",
			resourceKind: "Pod",
			wantErr:      "unmarshal",
		},
		{
			name:         "no matching resources returns empty",
			data:         `[]`,
			resourceNs:   "ns",
			resourceName: "x",
			resourceKind: "Pod",
			wantLen:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alerts, err := parseAndFilterAlertmanagerAlerts(
				[]byte(tt.data), tt.resourceNs, tt.resourceName, tt.resourceKind,
			)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Len(t, alerts, tt.wantLen)
			if tt.check != nil {
				tt.check(t, alerts)
			}
		})
	}
}

// --- resolveMonitoringEndpoints ---

func TestResolveMonitoringEndpoints(t *testing.T) {
	t.Run("returns defaults when ConfigMonitoring is nil", func(t *testing.T) {
		origCfg := model.ConfigMonitoring
		model.ConfigMonitoring = nil
		defer func() { model.ConfigMonitoring = origCfg }()

		promNs, promSvc, promPort, amNs, amSvc, amPort := resolveMonitoringEndpoints("test-ctx")

		assert.Contains(t, promNs, "monitoring")
		assert.Contains(t, promSvc, "prometheus")
		assert.Equal(t, "9090", promPort)
		assert.Contains(t, amNs, "monitoring")
		assert.Contains(t, amSvc, "alertmanager")
		assert.Equal(t, "9093", amPort)
	})

	t.Run("uses context-specific config when available", func(t *testing.T) {
		origCfg := model.ConfigMonitoring
		model.ConfigMonitoring = map[string]model.MonitoringConfig{
			"my-ctx": {
				Prometheus: &model.MonitoringEndpoint{
					Namespaces: []string{"custom-ns"},
					Services:   []string{"custom-prom"},
					Port:       "8080",
				},
				Alertmanager: &model.MonitoringEndpoint{
					Namespaces: []string{"alerting"},
					Services:   []string{"custom-am"},
					Port:       "9999",
				},
			},
		}
		defer func() { model.ConfigMonitoring = origCfg }()

		promNs, promSvc, promPort, amNs, amSvc, amPort := resolveMonitoringEndpoints("my-ctx")

		assert.Equal(t, []string{"custom-ns"}, promNs)
		assert.Equal(t, []string{"custom-prom"}, promSvc)
		assert.Equal(t, "8080", promPort)
		assert.Equal(t, []string{"alerting"}, amNs)
		assert.Equal(t, []string{"custom-am"}, amSvc)
		assert.Equal(t, "9999", amPort)
	})

	t.Run("falls back to _global config when context not found", func(t *testing.T) {
		origCfg := model.ConfigMonitoring
		model.ConfigMonitoring = map[string]model.MonitoringConfig{
			"_global": {
				Prometheus: &model.MonitoringEndpoint{
					Namespaces: []string{"default-mon"},
					Services:   []string{"default-prom"},
					Port:       "7070",
				},
			},
		}
		defer func() { model.ConfigMonitoring = origCfg }()

		promNs, promSvc, promPort, _, _, _ := resolveMonitoringEndpoints("unknown-ctx")

		assert.Equal(t, []string{"default-mon"}, promNs)
		assert.Equal(t, []string{"default-prom"}, promSvc)
		assert.Equal(t, "7070", promPort)
	})

	t.Run("returns defaults when neither context nor default config exists", func(t *testing.T) {
		origCfg := model.ConfigMonitoring
		model.ConfigMonitoring = map[string]model.MonitoringConfig{
			"other-ctx": {},
		}
		defer func() { model.ConfigMonitoring = origCfg }()

		promNs, promSvc, promPort, amNs, amSvc, amPort := resolveMonitoringEndpoints("missing-ctx")

		assert.Contains(t, promNs, "monitoring")
		assert.Contains(t, promSvc, "prometheus")
		assert.Equal(t, "9090", promPort)
		assert.Contains(t, amNs, "monitoring")
		assert.Contains(t, amSvc, "alertmanager")
		assert.Equal(t, "9093", amPort)
	})

	t.Run("partial config only overrides specified fields", func(t *testing.T) {
		origCfg := model.ConfigMonitoring
		model.ConfigMonitoring = map[string]model.MonitoringConfig{
			"partial-ctx": {
				Prometheus: &model.MonitoringEndpoint{
					Port: "1234",
					// Namespaces and Services not set: should keep defaults.
				},
				// Alertmanager not set: should keep defaults.
			},
		}
		defer func() { model.ConfigMonitoring = origCfg }()

		promNs, promSvc, promPort, amNs, amSvc, amPort := resolveMonitoringEndpoints("partial-ctx")

		// Prometheus port overridden.
		assert.Equal(t, "1234", promPort)
		// Prometheus ns and svc remain defaults.
		assert.Contains(t, promNs, "monitoring")
		assert.Contains(t, promSvc, "prometheus")
		// Alertmanager all defaults.
		assert.Contains(t, amNs, "monitoring")
		assert.Contains(t, amSvc, "alertmanager")
		assert.Equal(t, "9093", amPort)
	})
}
