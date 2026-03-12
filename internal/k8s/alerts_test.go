package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAlertMatchesResource(t *testing.T) {
	t.Run("matches by pod name", func(t *testing.T) {
		labels := map[string]string{
			"namespace": "default",
			"pod":       "nginx-abc123",
		}
		assert.True(t, alertMatchesResource(labels, "default", "nginx-abc123", "pod"))
	})

	t.Run("matches by deployment name", func(t *testing.T) {
		labels := map[string]string{
			"namespace":  "production",
			"deployment": "api-server",
		}
		assert.True(t, alertMatchesResource(labels, "production", "api-server", "deployment"))
	})

	t.Run("matches pod prefix for deployment", func(t *testing.T) {
		labels := map[string]string{
			"namespace": "default",
			"pod":       "nginx-7b8d6c5f4-x9k2q",
		}
		assert.True(t, alertMatchesResource(labels, "default", "nginx", "deployment"))
	})

	t.Run("matches by statefulset name", func(t *testing.T) {
		labels := map[string]string{
			"namespace":   "default",
			"statefulset": "mysql",
		}
		assert.True(t, alertMatchesResource(labels, "default", "mysql", "statefulset"))
	})

	t.Run("matches by service name", func(t *testing.T) {
		labels := map[string]string{
			"namespace": "default",
			"service":   "my-svc",
		}
		assert.True(t, alertMatchesResource(labels, "default", "my-svc", "service"))
	})

	t.Run("matches by kind label", func(t *testing.T) {
		labels := map[string]string{
			"namespace": "default",
			"cronjob":   "backup-job",
		}
		assert.True(t, alertMatchesResource(labels, "default", "backup-job", "cronjob"))
	})

	t.Run("does not match wrong namespace", func(t *testing.T) {
		labels := map[string]string{
			"namespace": "staging",
			"pod":       "nginx",
		}
		assert.False(t, alertMatchesResource(labels, "production", "nginx", "pod"))
	})

	t.Run("does not match wrong resource name", func(t *testing.T) {
		labels := map[string]string{
			"namespace": "default",
			"pod":       "redis",
		}
		assert.False(t, alertMatchesResource(labels, "default", "nginx", "pod"))
	})

	t.Run("does not match partial prefix that is not a dash-separated prefix", func(t *testing.T) {
		labels := map[string]string{
			"namespace": "default",
			"pod":       "nginxplus-abc",
		}
		// "nginx" is not a prefix of "nginxplus-abc" with dash separator
		assert.False(t, alertMatchesResource(labels, "default", "nginx", "pod"))
	})

	t.Run("empty labels do not match", func(t *testing.T) {
		labels := map[string]string{
			"namespace": "default",
		}
		assert.False(t, alertMatchesResource(labels, "default", "nginx", "pod"))
	})
}

func TestParseAndFilterAlerts(t *testing.T) {
	t.Run("parses valid response", func(t *testing.T) {
		data := []byte(`{
			"status": "success",
			"data": {
				"alerts": [
					{
						"labels": {
							"alertname": "HighMemory",
							"namespace": "default",
							"pod": "nginx-abc",
							"severity": "warning"
						},
						"annotations": {
							"summary": "Memory usage high",
							"description": "Pod nginx-abc using >80% memory",
							"grafana_url": "https://grafana.example.com/d/abc"
						},
						"state": "firing",
						"activeAt": "2025-01-01T00:00:00Z"
					},
					{
						"labels": {
							"alertname": "OtherAlert",
							"namespace": "staging",
							"pod": "redis"
						},
						"annotations": {},
						"state": "pending",
						"activeAt": "2025-01-02T00:00:00Z"
					}
				]
			}
		}`)

		alerts, err := parseAndFilterAlerts(data, "default", "nginx", "deployment")
		assert.NoError(t, err)
		assert.Len(t, alerts, 1)
		assert.Equal(t, "HighMemory", alerts[0].Name)
		assert.Equal(t, "firing", alerts[0].State)
		assert.Equal(t, "warning", alerts[0].Severity)
		assert.Equal(t, "Memory usage high", alerts[0].Summary)
		assert.Equal(t, "Pod nginx-abc using >80% memory", alerts[0].Description)
		assert.Equal(t, "https://grafana.example.com/d/abc", alerts[0].GrafanaURL)
	})

	t.Run("returns empty for no matching alerts", func(t *testing.T) {
		data := []byte(`{
			"status": "success",
			"data": {
				"alerts": [
					{
						"labels": {"alertname": "Other", "namespace": "other", "pod": "x"},
						"annotations": {},
						"state": "firing",
						"activeAt": "2025-01-01T00:00:00Z"
					}
				]
			}
		}`)

		alerts, err := parseAndFilterAlerts(data, "default", "nginx", "pod")
		assert.NoError(t, err)
		assert.Empty(t, alerts)
	})

	t.Run("returns error for non-success status", func(t *testing.T) {
		data := []byte(`{"status": "error", "data": {"alerts": []}}`)
		_, err := parseAndFilterAlerts(data, "default", "nginx", "pod")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "status: error")
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		_, err := parseAndFilterAlerts([]byte(`{invalid`), "default", "nginx", "pod")
		assert.Error(t, err)
	})

	t.Run("tries grafana url annotation fallbacks", func(t *testing.T) {
		data := []byte(`{
			"status": "success",
			"data": {
				"alerts": [{
					"labels": {"alertname": "A", "namespace": "ns", "pod": "p"},
					"annotations": {"dashboard_url": "https://dash.example.com"},
					"state": "firing",
					"activeAt": "2025-01-01T00:00:00Z"
				}]
			}
		}`)

		alerts, err := parseAndFilterAlerts(data, "ns", "p", "pod")
		assert.NoError(t, err)
		assert.Len(t, alerts, 1)
		assert.Equal(t, "https://dash.example.com", alerts[0].GrafanaURL)
	})
}
