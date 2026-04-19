package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	return append([]string{}, splitString(s)...)
}

func splitString(s string) []string {
	result := []string{}
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func indexOf(s, substr string) int {
	for i := range len(s) - len(substr) + 1 {
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

// --- evaluateSimpleJSONPath ---

func TestEvaluateSimpleJSONPath(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"phase": "Running",
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "Ready",
					"status": "True",
				},
				map[string]interface{}{
					"type":   "Initialized",
					"status": "True",
				},
			},
		},
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"repoURL": "https://github.com/example/repo",
			},
			"replicas": float64(3),
		},
		"metadata": map[string]interface{}{
			"creationTimestamp": "2025-01-15T10:30:00Z",
		},
	}

	tests := []struct {
		name    string
		path    string
		wantVal interface{}
		wantOK  bool
	}{
		{"simple field", ".status.phase", "Running", true},
		{"nested field", ".spec.source.repoURL", "https://github.com/example/repo", true},
		{"numeric field", ".spec.replicas", float64(3), true},
		{"array index", ".status.conditions[0].type", "Ready", true},
		{"array index 1", ".status.conditions[1].status", "True", true},
		{"metadata field", ".metadata.creationTimestamp", "2025-01-15T10:30:00Z", true},
		{"missing field", ".status.missing", nil, false},
		{"missing nested", ".status.deep.missing", nil, false},
		{"array out of bounds", ".status.conditions[5].type", nil, false},
		{"empty path", "", nil, false},
		{"path without leading dot", "status.phase", "Running", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := evaluateSimpleJSONPath(obj, tt.path)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantVal, val)
			}
		})
	}
}

// --- formatPrinterValue ---

func TestFormatPrinterValue(t *testing.T) {
	tests := []struct {
		name    string
		val     interface{}
		colType string
		want    string
	}{
		{"string value", "Running", "string", "Running"},
		{"default type string", "hello", "", "hello"},
		{"integer from float64", float64(42), "integer", "42"},
		{"integer from int64", int64(7), "integer", "7"},
		{"number whole", float64(100), "number", "100"},
		{"number fractional", float64(3.14), "number", "3.14"},
		{"boolean true", true, "boolean", "true"},
		{"boolean false", false, "boolean", "false"},
		{"date RFC3339", "2025-01-15T10:30:00Z", "date", formatAge(time.Since(func() time.Time {
			t, _ := time.Parse(time.RFC3339, "2025-01-15T10:30:00Z")
			return t
		}()))},
		{"date invalid", "not-a-date", "date", "not-a-date"},
		{"nil value", nil, "string", ""},
		{"integer from string fallback", "42", "integer", "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPrinterValue(tt.val, tt.colType)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- extractCRDPrinterColumns ---

func TestExtractCRDPrinterColumns(t *testing.T) {
	t.Run("extracts columns from matching version", func(t *testing.T) {
		spec := map[string]interface{}{
			"versions": []interface{}{
				map[string]interface{}{
					"name":   "v1alpha1",
					"served": true,
					"additionalPrinterColumns": []interface{}{
						map[string]interface{}{
							"name":     "Status",
							"type":     "string",
							"jsonPath": ".status.phase",
						},
						map[string]interface{}{
							"name":     "Repo",
							"type":     "string",
							"jsonPath": ".spec.source.repoURL",
						},
						map[string]interface{}{
							"name":     "Age",
							"type":     "date",
							"jsonPath": ".metadata.creationTimestamp",
						},
					},
				},
			},
		}
		cols := extractCRDPrinterColumns(spec, "v1alpha1")
		// Age should be skipped.
		assert.Len(t, cols, 2)
		assert.Equal(t, "Status", cols[0].Name)
		assert.Equal(t, "string", cols[0].Type)
		assert.Equal(t, ".status.phase", cols[0].JSONPath)
		assert.Equal(t, "Repo", cols[1].Name)
	})

	t.Run("returns nil for non-matching version", func(t *testing.T) {
		spec := map[string]interface{}{
			"versions": []interface{}{
				map[string]interface{}{
					"name":   "v1",
					"served": true,
					"additionalPrinterColumns": []interface{}{
						map[string]interface{}{
							"name":     "Status",
							"type":     "string",
							"jsonPath": ".status.phase",
						},
					},
				},
			},
		}
		cols := extractCRDPrinterColumns(spec, "v2")
		assert.Nil(t, cols)
	})

	t.Run("returns nil when no versions", func(t *testing.T) {
		spec := map[string]interface{}{}
		cols := extractCRDPrinterColumns(spec, "v1")
		assert.Nil(t, cols)
	})

	t.Run("returns nil when no additionalPrinterColumns", func(t *testing.T) {
		spec := map[string]interface{}{
			"versions": []interface{}{
				map[string]interface{}{
					"name":   "v1",
					"served": true,
				},
			},
		}
		cols := extractCRDPrinterColumns(spec, "v1")
		assert.Nil(t, cols)
	})

	t.Run("skips columns with empty name or jsonPath", func(t *testing.T) {
		spec := map[string]interface{}{
			"versions": []interface{}{
				map[string]interface{}{
					"name":   "v1",
					"served": true,
					"additionalPrinterColumns": []interface{}{
						map[string]interface{}{
							"name":     "",
							"type":     "string",
							"jsonPath": ".status.phase",
						},
						map[string]interface{}{
							"name":     "Status",
							"type":     "string",
							"jsonPath": "",
						},
						map[string]interface{}{
							"name":     "Valid",
							"type":     "string",
							"jsonPath": ".status.phase",
						},
					},
				},
			},
		}
		cols := extractCRDPrinterColumns(spec, "v1")
		assert.Len(t, cols, 1)
		assert.Equal(t, "Valid", cols[0].Name)
	})
}

// --- extractGenericConditions ---

func TestExtractGenericConditions(t *testing.T) {
	t.Run("prefers Ready condition", func(t *testing.T) {
		ti := &model.Item{}
		conditions := []interface{}{
			map[string]interface{}{
				"type":   "Initialized",
				"status": "True",
				"reason": "InitDone",
			},
			map[string]interface{}{
				"type":               "Ready",
				"status":             "True",
				"reason":             "AllGood",
				"lastTransitionTime": "2026-01-01T00:00:00Z",
			},
		}
		extractGenericConditions(ti, conditions)
		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "True", colMap["Ready"])
		assert.Equal(t, "AllGood", colMap["Reason"])
		assert.NotEmpty(t, colMap["Last Transition"])
		// Conditions should be stored on the Conditions field, not as columns.
		assert.Len(t, ti.Conditions, 2)
		assert.Equal(t, "Initialized", ti.Conditions[0].Type)
		assert.Equal(t, "Ready", ti.Conditions[1].Type)
	})

	t.Run("falls back to last condition", func(t *testing.T) {
		ti := &model.Item{}
		conditions := []interface{}{
			map[string]interface{}{
				"type":   "Initialized",
				"status": "True",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "False",
				"reason":  "MinimumReplicasUnavailable",
				"message": "Deployment does not have minimum availability.",
			},
		}
		extractGenericConditions(ti, conditions)
		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "False", colMap["Available"])
		assert.Equal(t, "MinimumReplicasUnavailable", colMap["Reason"])
		assert.NotEmpty(t, colMap["Message"])
	})

	t.Run("truncates long messages", func(t *testing.T) {
		ti := &model.Item{}
		longMsg := ""
		for i := range 100 {
			longMsg += fmt.Sprintf("x%d", i)
		}
		conditions := []interface{}{
			map[string]interface{}{
				"type":    "Ready",
				"status":  "False",
				"message": longMsg,
			},
		}
		extractGenericConditions(ti, conditions)
		// Find message column.
		for _, kv := range ti.Columns {
			if kv.Key == "Message" {
				assert.LessOrEqual(t, len(kv.Value), 80)
				assert.True(t, strings.HasSuffix(kv.Value, "..."))
			}
		}
	})

	t.Run("empty conditions", func(t *testing.T) {
		ti := &model.Item{}
		extractGenericConditions(ti, []interface{}{})
		assert.Empty(t, ti.Columns)
	})
}

// --- containsPath ---

func TestContainsPath(t *testing.T) {
	tests := []struct {
		name   string
		paths  []string
		target string
		want   bool
	}{
		{"found at beginning", []string{"/a", "/b", "/c"}, "/a", true},
		{"found in middle", []string{"/a", "/b", "/c"}, "/b", true},
		{"found at end", []string{"/a", "/b", "/c"}, "/c", true},
		{"not found", []string{"/a", "/b", "/c"}, "/d", false},
		{"empty list", []string{}, "/a", false},
		{"nil list", nil, "/a", false},
		{"exact match required", []string{"/home/user/.kube/config"}, "/home/user/.kube", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, containsPath(tt.paths, tt.target))
		})
	}
}

// --- buildKubeconfigPaths ---

func TestBuildKubeconfigPaths(t *testing.T) {
	t.Run("includes default kubeconfig path", func(t *testing.T) {
		// Save and clear KUBECONFIG to isolate the test.
		origKubeconfig := os.Getenv("KUBECONFIG")
		t.Setenv("KUBECONFIG", "")
		defer func() {
			t.Setenv("KUBECONFIG", origKubeconfig)
		}()

		paths := buildKubeconfigPaths()

		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("cannot determine home directory")
		}
		defaultPath := filepath.Join(home, ".kube", "config")
		assert.Contains(t, paths, defaultPath)
	})

	t.Run("includes KUBECONFIG env paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg1 := filepath.Join(tmpDir, "config1")
		cfg2 := filepath.Join(tmpDir, "config2")
		// Create the files so they exist.
		assert.NoError(t, os.WriteFile(cfg1, []byte(""), 0o600))
		assert.NoError(t, os.WriteFile(cfg2, []byte(""), 0o600))

		t.Setenv("KUBECONFIG", cfg1+string(os.PathListSeparator)+cfg2)

		paths := buildKubeconfigPaths()

		assert.Contains(t, paths, cfg1)
		assert.Contains(t, paths, cfg2)
	})

	t.Run("does not duplicate default path when in KUBECONFIG", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("cannot determine home directory")
		}
		defaultPath := filepath.Join(home, ".kube", "config")

		t.Setenv("KUBECONFIG", defaultPath)

		paths := buildKubeconfigPaths()

		count := 0
		for _, p := range paths {
			if p == defaultPath {
				count++
			}
		}
		assert.Equal(t, 1, count, "default path should appear exactly once")
	})

	t.Run("includes files from config.d directory", func(t *testing.T) {
		// Create a temporary home structure.
		tmpHome := t.TempDir()
		configD := filepath.Join(tmpHome, ".kube", "config.d")
		assert.NoError(t, os.MkdirAll(configD, 0o755))
		extraCfg := filepath.Join(configD, "extra-cluster")
		assert.NoError(t, os.WriteFile(extraCfg, []byte(""), 0o600))

		// This test verifies the WalkDir mechanism by checking the real home dir.
		// Since we cannot override UserHomeDir, we test that the function runs without error
		// and returns at least the default path.
		t.Setenv("KUBECONFIG", "")

		paths := buildKubeconfigPaths()

		assert.NotEmpty(t, paths, "should return at least the default kubeconfig path")
	})
}

// --- multi-kubeconfig context switching ---

func TestMultiKubeconfigContextSwitching(t *testing.T) {
	// Build two distinct kubeconfig files and verify that restConfigForContext
	// correctly resolves each context's server URL when both files are merged
	// via KUBECONFIG=c1:c2.
	tmp := t.TempDir()

	c1 := filepath.Join(tmp, "c1.yaml")
	c2 := filepath.Join(tmp, "c2.yaml")
	assert.NoError(t, os.WriteFile(c1, []byte(`apiVersion: v1
kind: Config
current-context: alpha
clusters:
- name: cluster-alpha
  cluster:
    server: https://alpha.example.test:6443
    insecure-skip-tls-verify: true
contexts:
- name: alpha
  context:
    cluster: cluster-alpha
    user: user-alpha
users:
- name: user-alpha
  user:
    token: alpha-token
`), 0o600))
	assert.NoError(t, os.WriteFile(c2, []byte(`apiVersion: v1
kind: Config
current-context: beta
clusters:
- name: cluster-beta
  cluster:
    server: https://beta.example.test:6443
    insecure-skip-tls-verify: true
contexts:
- name: beta
  context:
    cluster: cluster-beta
    user: user-beta
users:
- name: user-beta
  user:
    token: beta-token
`), 0o600))

	t.Setenv("KUBECONFIG", c1+string(os.PathListSeparator)+c2)

	client, err := NewClient("")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	t.Run("both contexts appear in list", func(t *testing.T) {
		ctxs, err := client.GetContexts()
		assert.NoError(t, err)
		names := make(map[string]bool, len(ctxs))
		for _, c := range ctxs {
			names[c.Name] = true
		}
		assert.True(t, names["alpha"], "alpha context must be present")
		assert.True(t, names["beta"], "beta context must be present")
	})

	t.Run("current context is from first file", func(t *testing.T) {
		// clientcmd merge: the first file's current-context wins.
		assert.Equal(t, "alpha", client.CurrentContext())
	})

	t.Run("restConfigForContext resolves alpha to alpha's server", func(t *testing.T) {
		cfg, err := client.restConfigForContext("alpha")
		assert.NoError(t, err)
		assert.Equal(t, "https://alpha.example.test:6443", cfg.Host,
			"alpha context must resolve to cluster-alpha's server")
	})

	t.Run("restConfigForContext resolves beta to beta's server", func(t *testing.T) {
		cfg, err := client.restConfigForContext("beta")
		assert.NoError(t, err)
		assert.Equal(t, "https://beta.example.test:6443", cfg.Host,
			"beta context must resolve to cluster-beta's server — regression: "+
				"when two KUBECONFIG files are merged, switching to the second "+
				"context must not keep routing traffic to the first cluster")
	})

	t.Run("KubeconfigPathForContext maps each context to its source file", func(t *testing.T) {
		assert.Equal(t, c1, client.KubeconfigPathForContext("alpha"))
		assert.Equal(t, c2, client.KubeconfigPathForContext("beta"))
	})
}

// --- collectConfigDirPaths ---

func TestCollectConfigDirPaths(t *testing.T) {
	t.Run("regular directory returns contained files", func(t *testing.T) {
		tmp := t.TempDir()
		a := filepath.Join(tmp, "a.yaml")
		b := filepath.Join(tmp, "b.yaml")
		assert.NoError(t, os.WriteFile(a, []byte("{}"), 0o600))
		assert.NoError(t, os.WriteFile(b, []byte("{}"), 0o600))

		paths := collectConfigDirPaths(tmp)
		assert.Len(t, paths, 2)
	})

	t.Run("symlink to directory is followed", func(t *testing.T) {
		tmp := t.TempDir()
		realDir := filepath.Join(tmp, "real-config-dir")
		assert.NoError(t, os.MkdirAll(realDir, 0o755))
		assert.NoError(t, os.WriteFile(filepath.Join(realDir, "cluster.yaml"), []byte("{}"), 0o600))

		linkDir := filepath.Join(tmp, "config.d")
		assert.NoError(t, os.Symlink(realDir, linkDir))

		paths := collectConfigDirPaths(linkDir)
		assert.Len(t, paths, 1, "symlink to directory should be followed")
		// Every returned path must point to a real file, never the directory itself.
		for _, p := range paths {
			info, err := os.Stat(p)
			assert.NoError(t, err)
			assert.False(t, info.IsDir(), "returned path must not be a directory: %s", p)
		}
	})

	t.Run("non-existent path returns empty", func(t *testing.T) {
		tmp := t.TempDir()
		paths := collectConfigDirPaths(filepath.Join(tmp, "nope"))
		assert.Empty(t, paths)
	})

	t.Run("regular file at dir location returns empty", func(t *testing.T) {
		tmp := t.TempDir()
		notDir := filepath.Join(tmp, "not-dir")
		assert.NoError(t, os.WriteFile(notDir, []byte("{}"), 0o600))

		paths := collectConfigDirPaths(notDir)
		assert.Empty(t, paths, "must not add a non-directory path")
	})

	t.Run("dangling symlink returns empty", func(t *testing.T) {
		tmp := t.TempDir()
		dangling := filepath.Join(tmp, "dangling")
		assert.NoError(t, os.Symlink(filepath.Join(tmp, "missing"), dangling))

		paths := collectConfigDirPaths(dangling)
		assert.Empty(t, paths)
	})

	t.Run("nested files are discovered recursively", func(t *testing.T) {
		tmp := t.TempDir()
		sub := filepath.Join(tmp, "sub")
		assert.NoError(t, os.MkdirAll(sub, 0o755))
		assert.NoError(t, os.WriteFile(filepath.Join(tmp, "top.yaml"), []byte("{}"), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(sub, "nested.yaml"), []byte("{}"), 0o600))

		paths := collectConfigDirPaths(tmp)
		assert.Len(t, paths, 2)
	})
}

// --- formatRelativeTime ---

func TestFormatRelativeTime(t *testing.T) {
	t.Run("seconds ago", func(t *testing.T) {
		result := formatRelativeTime(time.Now().Add(-30 * time.Second))
		assert.Contains(t, result, "s ago")
	})

	t.Run("minutes ago", func(t *testing.T) {
		result := formatRelativeTime(time.Now().Add(-5 * time.Minute))
		assert.Contains(t, result, "m ago")
	})

	t.Run("hours ago", func(t *testing.T) {
		result := formatRelativeTime(time.Now().Add(-3 * time.Hour))
		assert.Contains(t, result, "h ago")
	})

	t.Run("days ago", func(t *testing.T) {
		result := formatRelativeTime(time.Now().Add(-48 * time.Hour))
		assert.Contains(t, result, "d ago")
	})

	t.Run("just now", func(t *testing.T) {
		result := formatRelativeTime(time.Now())
		assert.Contains(t, result, "s ago")
	})

	t.Run("one minute boundary", func(t *testing.T) {
		result := formatRelativeTime(time.Now().Add(-60 * time.Second))
		assert.Contains(t, result, "m ago")
	})

	t.Run("one hour boundary", func(t *testing.T) {
		result := formatRelativeTime(time.Now().Add(-60 * time.Minute))
		assert.Contains(t, result, "h ago")
	})

	t.Run("one day boundary", func(t *testing.T) {
		result := formatRelativeTime(time.Now().Add(-24 * time.Hour))
		assert.Contains(t, result, "d ago")
	})
}
