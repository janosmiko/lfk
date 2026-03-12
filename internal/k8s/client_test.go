package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- formatAge ---

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		d        time.Duration
		expected string
	}{
		{"zero", 0, "0s"},
		{"seconds", 30 * time.Second, "30s"},
		{"just under a minute", 59 * time.Second, "59s"},
		{"one minute", time.Minute, "1m"},
		{"minutes", 45 * time.Minute, "45m"},
		{"just under an hour", 59 * time.Minute, "59m"},
		{"one hour", time.Hour, "1h"},
		{"hours", 12 * time.Hour, "12h"},
		{"just under a day", 23 * time.Hour, "23h"},
		{"one day", 24 * time.Hour, "1d"},
		{"days", 30 * 24 * time.Hour, "30d"},
		{"364 days", 364 * 24 * time.Hour, "364d"},
		{"one year", 365 * 24 * time.Hour, "1y"},
		{"two years", 730 * 24 * time.Hour, "2y"},
		{"mixed (1h30m shows as 1h)", 90 * time.Minute, "1h"},
		{"mixed (1d12h shows as 1d)", 36 * time.Hour, "1d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatAge(tt.d))
		})
	}
}

// --- getInt ---

func TestGetInt(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]interface{}
		key      string
		expected int64
	}{
		{"int64 value", map[string]interface{}{"count": int64(42)}, "count", 42},
		{"float64 value", map[string]interface{}{"count": float64(99.9)}, "count", 99},
		{"missing key", map[string]interface{}{"other": int64(1)}, "count", 0},
		{"nil map", nil, "count", 0},
		{"wrong type string", map[string]interface{}{"count": "hello"}, "count", 0},
		{"wrong type bool", map[string]interface{}{"count": true}, "count", 0},
		{"zero int64", map[string]interface{}{"count": int64(0)}, "count", 0},
		{"negative int64", map[string]interface{}{"count": int64(-5)}, "count", -5},
		{"negative float64", map[string]interface{}{"count": float64(-3.7)}, "count", -3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, getInt(tt.m, tt.key))
		})
	}
}

// --- parseEventTimestamp ---

func TestParseEventTimestamp(t *testing.T) {
	t.Run("valid RFC3339", func(t *testing.T) {
		obj := map[string]interface{}{
			"lastTimestamp": "2024-01-15T10:30:00Z",
		}
		result := parseEventTimestamp(obj, "lastTimestamp")
		assert.False(t, result.IsZero())
		assert.Equal(t, 2024, result.Year())
		assert.Equal(t, time.January, result.Month())
		assert.Equal(t, 15, result.Day())
	})

	t.Run("valid RFC3339Nano", func(t *testing.T) {
		obj := map[string]interface{}{
			"eventTime": "2024-01-15T10:30:00.123456789Z",
		}
		result := parseEventTimestamp(obj, "eventTime")
		assert.False(t, result.IsZero())
	})

	t.Run("missing field", func(t *testing.T) {
		obj := map[string]interface{}{}
		result := parseEventTimestamp(obj, "lastTimestamp")
		assert.True(t, result.IsZero())
	})

	t.Run("nil value", func(t *testing.T) {
		obj := map[string]interface{}{"lastTimestamp": nil}
		result := parseEventTimestamp(obj, "lastTimestamp")
		assert.True(t, result.IsZero())
	})

	t.Run("empty string", func(t *testing.T) {
		obj := map[string]interface{}{"lastTimestamp": ""}
		result := parseEventTimestamp(obj, "lastTimestamp")
		assert.True(t, result.IsZero())
	})

	t.Run("invalid format", func(t *testing.T) {
		obj := map[string]interface{}{"lastTimestamp": "not-a-date"}
		result := parseEventTimestamp(obj, "lastTimestamp")
		assert.True(t, result.IsZero())
	})

	t.Run("non-string type", func(t *testing.T) {
		obj := map[string]interface{}{"lastTimestamp": 12345}
		result := parseEventTimestamp(obj, "lastTimestamp")
		assert.True(t, result.IsZero())
	})
}

// --- extractStatus ---

func TestExtractStatus(t *testing.T) {
	t.Run("phase field", func(t *testing.T) {
		obj := map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Running",
			},
		}
		assert.Equal(t, "Running", extractStatus(obj))
	})

	t.Run("ArgoCD health+sync", func(t *testing.T) {
		obj := map[string]interface{}{
			"status": map[string]interface{}{
				"health": map[string]interface{}{
					"status": "Healthy",
				},
				"sync": map[string]interface{}{
					"status": "Synced",
				},
			},
		}
		assert.Equal(t, "Healthy/Synced", extractStatus(obj))
	})

	t.Run("ArgoCD health only", func(t *testing.T) {
		obj := map[string]interface{}{
			"status": map[string]interface{}{
				"health": map[string]interface{}{
					"status": "Degraded",
				},
			},
		}
		assert.Equal(t, "Degraded", extractStatus(obj))
	})

	t.Run("conditions with Available", func(t *testing.T) {
		obj := map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Progressing",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "Available",
						"status": "True",
					},
				},
			},
		}
		assert.Equal(t, "Available", extractStatus(obj))
	})

	t.Run("conditions fallback to last", func(t *testing.T) {
		obj := map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Initialized",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "Ready",
						"status": "False",
					},
				},
			},
		}
		assert.Equal(t, "Ready", extractStatus(obj))
	})

	t.Run("Available condition with False status", func(t *testing.T) {
		obj := map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Available",
						"status": "False",
					},
					map[string]interface{}{
						"type":   "Progressing",
						"status": "True",
					},
				},
			},
		}
		// Available is False, so falls back to last condition type.
		assert.Equal(t, "Progressing", extractStatus(obj))
	})

	t.Run("no status field", func(t *testing.T) {
		obj := map[string]interface{}{
			"metadata": map[string]interface{}{},
		}
		assert.Equal(t, "", extractStatus(obj))
	})

	t.Run("status is not a map", func(t *testing.T) {
		obj := map[string]interface{}{
			"status": "something",
		}
		assert.Equal(t, "", extractStatus(obj))
	})

	t.Run("empty status map", func(t *testing.T) {
		obj := map[string]interface{}{
			"status": map[string]interface{}{},
		}
		assert.Equal(t, "", extractStatus(obj))
	})

	t.Run("empty conditions array", func(t *testing.T) {
		obj := map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{},
			},
		}
		assert.Equal(t, "", extractStatus(obj))
	})
}

// --- reorderYAMLFields ---

func TestReorderYAMLFields(t *testing.T) {
	t.Run("standard kubernetes YAML", func(t *testing.T) {
		input := `status:
  phase: Running
spec:
  replicas: 3
metadata:
  name: my-deploy
kind: Deployment
apiVersion: apps/v1`
		result := reorderYAMLFields(input)
		lines := splitLines(result)
		// apiVersion should come first.
		assert.Equal(t, "apiVersion: apps/v1", lines[0])
		// kind second.
		assert.Equal(t, "kind: Deployment", lines[1])
		// metadata third.
		assert.Equal(t, "metadata:", lines[2])
		assert.Equal(t, "  name: my-deploy", lines[3])
	})

	t.Run("preserves nested content", func(t *testing.T) {
		input := `kind: Pod
apiVersion: v1
metadata:
  name: test
  labels:
    app: test
spec:
  containers:
    - name: nginx`
		result := reorderYAMLFields(input)
		assert.Contains(t, result, "apiVersion: v1")
		assert.Contains(t, result, "  labels:")
		assert.Contains(t, result, "    app: test")
		assert.Contains(t, result, "    - name: nginx")
	})

	t.Run("non-priority fields stay in order", func(t *testing.T) {
		input := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
custom1:
  key: val1
custom2:
  key: val2
data:
  foo: bar`
		result := reorderYAMLFields(input)
		// data has priority 5, should come before custom fields.
		dataIdx := indexOf(result, "data:")
		custom1Idx := indexOf(result, "custom1:")
		assert.Less(t, dataIdx, custom1Idx)
	})

	t.Run("empty input", func(t *testing.T) {
		assert.Equal(t, "", reorderYAMLFields(""))
	})

	t.Run("already ordered", func(t *testing.T) {
		input := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value`
		result := reorderYAMLFields(input)
		assert.Equal(t, input, result)
	})
}

func splitLines(s string) []string {
	var lines []string
	for _, l := range splitString(s) {
		lines = append(lines, l)
	}
	return lines
}

func splitString(s string) []string {
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// --- populateResourceDetails ---

func TestPopulateResourceDetails_LastRestartAt(t *testing.T) {
	t.Run("sets LastRestartAt from container lastState.terminated.finishedAt", func(t *testing.T) {
		obj := map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "app"},
				},
			},
			"status": map[string]interface{}{
				"containerStatuses": []interface{}{
					map[string]interface{}{
						"name":         "app",
						"ready":        true,
						"restartCount": float64(3),
						"lastState": map[string]interface{}{
							"terminated": map[string]interface{}{
								"finishedAt": "2025-06-15T12:30:00Z",
							},
						},
					},
				},
			},
		}
		var ti model.Item
		populateResourceDetails(&ti, obj, "Pod")
		assert.Equal(t, "1/1", ti.Ready)
		assert.Equal(t, "3", ti.Restarts)
		expected, _ := time.Parse(time.RFC3339, "2025-06-15T12:30:00Z")
		assert.Equal(t, expected, ti.LastRestartAt)
	})

	t.Run("picks the most recent LastRestartAt across containers", func(t *testing.T) {
		obj := map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "app"},
					map[string]interface{}{"name": "sidecar"},
				},
			},
			"status": map[string]interface{}{
				"containerStatuses": []interface{}{
					map[string]interface{}{
						"name":         "app",
						"ready":        true,
						"restartCount": float64(1),
						"lastState": map[string]interface{}{
							"terminated": map[string]interface{}{
								"finishedAt": "2025-06-15T10:00:00Z",
							},
						},
					},
					map[string]interface{}{
						"name":         "sidecar",
						"ready":        true,
						"restartCount": float64(2),
						"lastState": map[string]interface{}{
							"terminated": map[string]interface{}{
								"finishedAt": "2025-06-15T14:00:00Z",
							},
						},
					},
				},
			},
		}
		var ti model.Item
		populateResourceDetails(&ti, obj, "Pod")
		expected, _ := time.Parse(time.RFC3339, "2025-06-15T14:00:00Z")
		assert.Equal(t, expected, ti.LastRestartAt,
			"should pick the most recent finishedAt across containers")
	})

	t.Run("zero LastRestartAt when no lastState", func(t *testing.T) {
		obj := map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "app"},
				},
			},
			"status": map[string]interface{}{
				"containerStatuses": []interface{}{
					map[string]interface{}{
						"name":         "app",
						"ready":        true,
						"restartCount": float64(0),
					},
				},
			},
		}
		var ti model.Item
		populateResourceDetails(&ti, obj, "Pod")
		assert.True(t, ti.LastRestartAt.IsZero(),
			"LastRestartAt should be zero when no lastState is present")
	})

	t.Run("zero LastRestartAt when lastState has no terminated", func(t *testing.T) {
		obj := map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "app"},
				},
			},
			"status": map[string]interface{}{
				"containerStatuses": []interface{}{
					map[string]interface{}{
						"name":         "app",
						"ready":        false,
						"restartCount": float64(1),
						"lastState": map[string]interface{}{
							"waiting": map[string]interface{}{
								"reason": "CrashLoopBackOff",
							},
						},
					},
				},
			},
		}
		var ti model.Item
		populateResourceDetails(&ti, obj, "Pod")
		assert.True(t, ti.LastRestartAt.IsZero(),
			"LastRestartAt should be zero when lastState has no terminated block")
	})
}

// --- computeQuotaPercent ---

func TestComputeQuotaPercent(t *testing.T) {
	tests := []struct {
		name    string
		resName string
		hard    string
		used    string
		want    float64
	}{
		{"cpu half used", "cpu", "4", "2", 50},
		{"cpu fully used", "cpu", "2", "2", 100},
		{"cpu zero used", "cpu", "4", "0", 0},
		{"memory quantity strings", "memory", "8Gi", "6Gi", 75},
		{"memory zero", "memory", "4Gi", "0", 0},
		{"pods integer", "pods", "20", "10", 50},
		{"services integer", "services", "5", "5", 100},
		{"zero hard returns 0", "cpu", "0", "0", 0},
		{"cpu millis", "cpu", "1000m", "500m", 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeQuotaPercent(tt.resName, tt.hard, tt.used)
			assert.InDelta(t, tt.want, got, 0.5, "computeQuotaPercent(%s, %s, %s)", tt.resName, tt.hard, tt.used)
		})
	}
}
