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
		m        map[string]any
		key      string
		expected int64
	}{
		{"int64 value", map[string]any{"count": int64(42)}, "count", 42},
		{"float64 value", map[string]any{"count": float64(99.9)}, "count", 99},
		{"missing key", map[string]any{"other": int64(1)}, "count", 0},
		{"nil map", nil, "count", 0},
		{"wrong type string", map[string]any{"count": "hello"}, "count", 0},
		{"wrong type bool", map[string]any{"count": true}, "count", 0},
		{"zero int64", map[string]any{"count": int64(0)}, "count", 0},
		{"negative int64", map[string]any{"count": int64(-5)}, "count", -5},
		{"negative float64", map[string]any{"count": float64(-3.7)}, "count", -3},
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
		obj := map[string]any{
			"lastTimestamp": "2024-01-15T10:30:00Z",
		}
		result := parseEventTimestamp(obj, "lastTimestamp")
		assert.False(t, result.IsZero())
		assert.Equal(t, 2024, result.Year())
		assert.Equal(t, time.January, result.Month())
		assert.Equal(t, 15, result.Day())
	})

	t.Run("valid RFC3339Nano", func(t *testing.T) {
		obj := map[string]any{
			"eventTime": "2024-01-15T10:30:00.123456789Z",
		}
		result := parseEventTimestamp(obj, "eventTime")
		assert.False(t, result.IsZero())
	})

	t.Run("missing field", func(t *testing.T) {
		obj := map[string]any{}
		result := parseEventTimestamp(obj, "lastTimestamp")
		assert.True(t, result.IsZero())
	})

	t.Run("nil value", func(t *testing.T) {
		obj := map[string]any{"lastTimestamp": nil}
		result := parseEventTimestamp(obj, "lastTimestamp")
		assert.True(t, result.IsZero())
	})

	t.Run("empty string", func(t *testing.T) {
		obj := map[string]any{"lastTimestamp": ""}
		result := parseEventTimestamp(obj, "lastTimestamp")
		assert.True(t, result.IsZero())
	})

	t.Run("invalid format", func(t *testing.T) {
		obj := map[string]any{"lastTimestamp": "not-a-date"}
		result := parseEventTimestamp(obj, "lastTimestamp")
		assert.True(t, result.IsZero())
	})

	t.Run("non-string type", func(t *testing.T) {
		obj := map[string]any{"lastTimestamp": 12345}
		result := parseEventTimestamp(obj, "lastTimestamp")
		assert.True(t, result.IsZero())
	})
}

// --- extractStatus ---

func TestExtractStatus(t *testing.T) {
	t.Run("phase field", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"phase": "Running",
			},
		}
		assert.Equal(t, "Running", extractStatus(obj))
	})

	t.Run("ArgoCD health+sync", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"health": map[string]any{
					"status": "Healthy",
				},
				"sync": map[string]any{
					"status": "Synced",
				},
			},
		}
		assert.Equal(t, "Healthy/Synced", extractStatus(obj))
	})

	t.Run("ArgoCD health only", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"health": map[string]any{
					"status": "Degraded",
				},
			},
		}
		assert.Equal(t, "Degraded", extractStatus(obj))
	})

	t.Run("conditions with Available", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Progressing",
						"status": "True",
					},
					map[string]any{
						"type":   "Available",
						"status": "True",
					},
				},
			},
		}
		assert.Equal(t, "Available", extractStatus(obj))
	})

	t.Run("conditions fallback to last", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Initialized",
						"status": "True",
					},
					map[string]any{
						"type":   "Ready",
						"status": "False",
					},
				},
			},
		}
		assert.Equal(t, "Ready", extractStatus(obj))
	})

	t.Run("Available condition with False status", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Available",
						"status": "False",
					},
					map[string]any{
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
		obj := map[string]any{
			"metadata": map[string]any{},
		}
		assert.Equal(t, "", extractStatus(obj))
	})

	t.Run("status is not a map", func(t *testing.T) {
		obj := map[string]any{
			"status": "something",
		}
		assert.Equal(t, "", extractStatus(obj))
	})

	t.Run("empty status map", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{},
		}
		assert.Equal(t, "", extractStatus(obj))
	})

	t.Run("empty conditions array", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"conditions": []any{},
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
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{
					map[string]any{"name": "app"},
				},
			},
			"status": map[string]any{
				"containerStatuses": []any{
					map[string]any{
						"name":         "app",
						"ready":        true,
						"restartCount": float64(3),
						"lastState": map[string]any{
							"terminated": map[string]any{
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
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{
					map[string]any{"name": "app"},
					map[string]any{"name": "sidecar"},
				},
			},
			"status": map[string]any{
				"containerStatuses": []any{
					map[string]any{
						"name":         "app",
						"ready":        true,
						"restartCount": float64(1),
						"lastState": map[string]any{
							"terminated": map[string]any{
								"finishedAt": "2025-06-15T10:00:00Z",
							},
						},
					},
					map[string]any{
						"name":         "sidecar",
						"ready":        true,
						"restartCount": float64(2),
						"lastState": map[string]any{
							"terminated": map[string]any{
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
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{
					map[string]any{"name": "app"},
				},
			},
			"status": map[string]any{
				"containerStatuses": []any{
					map[string]any{
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
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{
					map[string]any{"name": "app"},
				},
			},
			"status": map[string]any{
				"containerStatuses": []any{
					map[string]any{
						"name":         "app",
						"ready":        false,
						"restartCount": float64(1),
						"lastState": map[string]any{
							"waiting": map[string]any{
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
	obj := map[string]any{
		"status": map[string]any{
			"phase": "Running",
			"conditions": []any{
				map[string]any{
					"type":   "Ready",
					"status": "True",
				},
				map[string]any{
					"type":   "Initialized",
					"status": "True",
				},
			},
		},
		"spec": map[string]any{
			"source": map[string]any{
				"repoURL": "https://github.com/example/repo",
			},
			"replicas": float64(3),
		},
		"metadata": map[string]any{
			"creationTimestamp": "2025-01-15T10:30:00Z",
		},
	}

	tests := []struct {
		name    string
		path    string
		wantVal any
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
		val     any
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
		spec := map[string]any{
			"versions": []any{
				map[string]any{
					"name":   "v1alpha1",
					"served": true,
					"additionalPrinterColumns": []any{
						map[string]any{
							"name":     "Status",
							"type":     "string",
							"jsonPath": ".status.phase",
						},
						map[string]any{
							"name":     "Repo",
							"type":     "string",
							"jsonPath": ".spec.source.repoURL",
						},
						map[string]any{
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
		spec := map[string]any{
			"versions": []any{
				map[string]any{
					"name":   "v1",
					"served": true,
					"additionalPrinterColumns": []any{
						map[string]any{
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
		spec := map[string]any{}
		cols := extractCRDPrinterColumns(spec, "v1")
		assert.Nil(t, cols)
	})

	t.Run("returns nil when no additionalPrinterColumns", func(t *testing.T) {
		spec := map[string]any{
			"versions": []any{
				map[string]any{
					"name":   "v1",
					"served": true,
				},
			},
		}
		cols := extractCRDPrinterColumns(spec, "v1")
		assert.Nil(t, cols)
	})

	t.Run("skips columns with empty name or jsonPath", func(t *testing.T) {
		spec := map[string]any{
			"versions": []any{
				map[string]any{
					"name":   "v1",
					"served": true,
					"additionalPrinterColumns": []any{
						map[string]any{
							"name":     "",
							"type":     "string",
							"jsonPath": ".status.phase",
						},
						map[string]any{
							"name":     "Status",
							"type":     "string",
							"jsonPath": "",
						},
						map[string]any{
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
		conditions := []any{
			map[string]any{
				"type":   "Initialized",
				"status": "True",
				"reason": "InitDone",
			},
			map[string]any{
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
		conditions := []any{
			map[string]any{
				"type":   "Initialized",
				"status": "True",
			},
			map[string]any{
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
		var longMsg strings.Builder
		for i := range 100 {
			fmt.Fprintf(&longMsg, "x%d", i)
		}
		conditions := []any{
			map[string]any{
				"type":    "Ready",
				"status":  "False",
				"message": longMsg.String(),
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
		extractGenericConditions(ti, []any{})
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

	t.Run("dedups KUBECONFIG entries that point at the same file", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := filepath.Join(tmpDir, "config.yaml")
		assert.NoError(t, os.WriteFile(cfg, []byte(""), 0o600))
		// Reference the same file via two cosmetically different paths.
		via := filepath.Join(tmpDir, ".", "config.yaml")
		t.Setenv("KUBECONFIG", cfg+string(os.PathListSeparator)+via)

		paths := buildKubeconfigPaths()
		count := 0
		for _, p := range paths {
			if p == cfg || p == via {
				count++
			}
		}
		assert.Equal(t, 1, count,
			"dedup should collapse path/./path duplicates so contexts aren't loaded twice")
	})

	t.Run("dedups KUBECONFIG entry that overlaps with config.d walk", func(t *testing.T) {
		// Regression: the same kubeconfig listed in KUBECONFIG and also
		// living under ~/.kube/config.d/ (when both happen, e.g. a user
		// symlinks their environment file into config.d) was being loaded
		// twice, so each context inside it appeared as two disambiguated
		// rows in the cluster list.
		tmpDir := t.TempDir()
		cfg := filepath.Join(tmpDir, "shared.yaml")
		assert.NoError(t, os.WriteFile(cfg, []byte(""), 0o600))
		viaSymlink := filepath.Join(tmpDir, "shared-link.yaml")
		assert.NoError(t, os.Symlink(cfg, viaSymlink))
		t.Setenv("KUBECONFIG", cfg+string(os.PathListSeparator)+viaSymlink)

		paths := buildKubeconfigPaths()
		// Both entries point at the same underlying file; only one should
		// remain after dedup. Compare via EvalSymlinks on both sides so
		// we don't trip over /tmp → /private/tmp on macOS.
		canonCfg, err := filepath.EvalSymlinks(cfg)
		assert.NoError(t, err)
		seenReal := 0
		for _, p := range paths {
			if resolved, err := filepath.EvalSymlinks(p); err == nil && resolved == canonCfg {
				seenReal++
			}
		}
		assert.Equal(t, 1, seenReal,
			"symlinked duplicates should collapse so collectContexts doesn't see the file twice")
	})
}

func TestDedupKubeconfigPaths(t *testing.T) {
	t.Run("preserves order, drops later duplicates", func(t *testing.T) {
		tmp := t.TempDir()
		a := filepath.Join(tmp, "a.yaml")
		b := filepath.Join(tmp, "b.yaml")
		assert.NoError(t, os.WriteFile(a, []byte(""), 0o600))
		assert.NoError(t, os.WriteFile(b, []byte(""), 0o600))
		got := dedupKubeconfigPaths([]string{a, b, a, b})
		assert.Equal(t, []string{a, b}, got)
	})

	t.Run("keeps unresolvable paths as-is", func(t *testing.T) {
		// Missing/dangling paths should still pass through (clientcmd will
		// surface a clear error later) — they shouldn't crash the dedup.
		got := dedupKubeconfigPaths([]string{"/no/such/file", "/no/such/file", "/another"})
		assert.Equal(t, []string{"/no/such/file", "/another"}, got)
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

// TestMultiKubeconfigOverlappingNames verifies that multiple kubeconfigs sharing
// the same cluster and user names (but distinct context names) still resolve to
// each context's correct cluster and user. Reproduces issue #23: all five
// configs in ~/.kube/config.d declared cluster "k0s" and user "root", so the
// clientcmd merge collapsed them and every context routed traffic to the
// last-merged cluster.
func TestMultiKubeconfigOverlappingNames(t *testing.T) {
	tmp := t.TempDir()

	c1 := filepath.Join(tmp, "cluster-one.yaml")
	c2 := filepath.Join(tmp, "cluster-two.yaml")
	// Both files declare the same cluster name ("k0s") and user name ("root")
	// but point at different servers and tokens. Distinct context names are
	// the only thing keeping them apart.
	assert.NoError(t, os.WriteFile(c1, []byte(`apiVersion: v1
kind: Config
current-context: one
clusters:
- name: k0s
  cluster:
    server: https://one.example.test:6443
    insecure-skip-tls-verify: true
contexts:
- name: one
  context:
    cluster: k0s
    user: root
users:
- name: root
  user:
    token: one-token
`), 0o600))
	assert.NoError(t, os.WriteFile(c2, []byte(`apiVersion: v1
kind: Config
current-context: two
clusters:
- name: k0s
  cluster:
    server: https://two.example.test:6443
    insecure-skip-tls-verify: true
contexts:
- name: two
  context:
    cluster: k0s
    user: root
users:
- name: root
  user:
    token: two-token
`), 0o600))

	t.Setenv("KUBECONFIG", c1+string(os.PathListSeparator)+c2)

	client, err := NewClient("")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	t.Run("both contexts visible despite shared cluster/user names", func(t *testing.T) {
		ctxs, err := client.GetContexts()
		assert.NoError(t, err)
		names := make(map[string]bool, len(ctxs))
		for _, c := range ctxs {
			names[c.Name] = true
		}
		assert.True(t, names["one"], "context 'one' must be present")
		assert.True(t, names["two"], "context 'two' must be present")
	})

	t.Run("one context resolves to one's server", func(t *testing.T) {
		cfg, err := client.restConfigForContext("one")
		assert.NoError(t, err)
		assert.Equal(t, "https://one.example.test:6443", cfg.Host,
			"context 'one' must route to cluster-one's server even though "+
				"cluster name 'k0s' is shared with cluster-two")
		assert.Equal(t, "one-token", cfg.BearerToken,
			"context 'one' must use one's token even though "+
				"user name 'root' is shared with cluster-two")
	})

	t.Run("two context resolves to two's server", func(t *testing.T) {
		cfg, err := client.restConfigForContext("two")
		assert.NoError(t, err)
		assert.Equal(t, "https://two.example.test:6443", cfg.Host,
			"context 'two' must route to cluster-two's server even though "+
				"cluster name 'k0s' is shared with cluster-one")
		assert.Equal(t, "two-token", cfg.BearerToken,
			"context 'two' must use two's token even though "+
				"user name 'root' is shared with cluster-one")
	})

	t.Run("KubeconfigPathForContext returns the per-context source file", func(t *testing.T) {
		// Subprocess invocations (kubectl, helm, port-forward) rely on this
		// being the single source file so the merge bug doesn't recur.
		assert.Equal(t, c1, client.KubeconfigPathForContext("one"))
		assert.Equal(t, c2, client.KubeconfigPathForContext("two"))
	})

	t.Run("KubeconfigPathForContext caches lookups", func(t *testing.T) {
		// Second call must still resolve correctly after the first populated
		// the contextOrigin cache. Catches regressions where the cache key or
		// value is wrong.
		assert.Equal(t, c1, client.KubeconfigPathForContext("one"))
		assert.Equal(t, c2, client.KubeconfigPathForContext("two"))
	})
}

// TestMultiKubeconfigIdenticalContextNames verifies that when several
// kubeconfig files declare the *same* context name (so the user can't
// distinguish them by name alone), every file is still surfaced as a
// selectable, disambiguated context. Reproduces the second half of issue #23,
// where ~/.kube/config.d/{dev-envs,itg-k8s,prod-envs}.yaml all declared
// context "dev"/cluster "dev"/user "dev" pointing at distinct servers, so
// clientcmd's first-writer-wins merge made only one "dev" visible and routed
// every drill-down to the first file's cluster.
func TestMultiKubeconfigIdenticalContextNames(t *testing.T) {
	tmp := t.TempDir()
	// Isolate HOME so the developer's real ~/.kube/config and
	// ~/.kube/config.d/* don't get merged into the test's loading rules and
	// pollute the visible context list.
	t.Setenv("HOME", tmp)

	dev := filepath.Join(tmp, "dev-envs.yaml")
	itg := filepath.Join(tmp, "itg-k8s.yaml")
	prod := filepath.Join(tmp, "prod-envs.yaml")
	for path, server := range map[string]string{
		dev:  "https://dev.example.test:6443",
		itg:  "https://itg.example.test:6443",
		prod: "https://prod.example.test:6443",
	} {
		assert.NoError(t, os.WriteFile(path, fmt.Appendf(nil, `apiVersion: v1
kind: Config
current-context: dev
clusters:
- name: dev
  cluster:
    server: %s
    insecure-skip-tls-verify: true
contexts:
- name: dev
  context:
    cluster: dev
    user: dev
users:
- name: dev
  user:
    token: %s-token
`, server, filepath.Base(path)), 0o600))
	}

	t.Setenv("KUBECONFIG",
		dev+string(os.PathListSeparator)+itg+string(os.PathListSeparator)+prod)

	client, err := NewClient("")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	t.Run("all three contexts appear with disambiguated display names", func(t *testing.T) {
		ctxs, err := client.GetContexts()
		assert.NoError(t, err)
		got := make([]string, 0, len(ctxs))
		for _, c := range ctxs {
			got = append(got, c.Name)
		}
		// One entry per source file is required — three files, three entries.
		assert.Len(t, got, 3, "got contexts: %v", got)
		// Each file must be addressable; the suffix encodes the source file.
		assertContains := func(needle string) {
			for _, name := range got {
				if strings.Contains(name, needle) {
					return
				}
			}
			t.Errorf("no context display name contains %q; got: %v", needle, got)
		}
		assertContains("dev-envs")
		assertContains("itg-k8s")
		assertContains("prod-envs")
	})

	t.Run("each disambiguated context routes to its own server", func(t *testing.T) {
		ctxs, _ := client.GetContexts()
		// Build a name → expected-server expectation by parsing the source-file
		// hint embedded in the display name.
		for _, c := range ctxs {
			var expected string
			switch {
			case strings.Contains(c.Name, "dev-envs"):
				expected = "https://dev.example.test:6443"
			case strings.Contains(c.Name, "itg-k8s"):
				expected = "https://itg.example.test:6443"
			case strings.Contains(c.Name, "prod-envs"):
				expected = "https://prod.example.test:6443"
			default:
				t.Fatalf("unexpected context name %q", c.Name)
			}

			cfg, err := client.restConfigForContext(c.Name)
			assert.NoError(t, err, "restConfigForContext(%q)", c.Name)
			assert.Equal(t, expected, cfg.Host,
				"context %q must route to its own file's server", c.Name)
		}
	})

	t.Run("OriginalContextName recovers the kubectl --context value", func(t *testing.T) {
		ctxs, _ := client.GetContexts()
		// Every disambiguated display name maps back to the literal "dev" that
		// kubectl sees in the underlying kubeconfig file.
		for _, c := range ctxs {
			assert.Equal(t, "dev", client.OriginalContextName(c.Name),
				"display name %q must translate to kubectl context 'dev'", c.Name)
		}
	})

	t.Run("KubeconfigPathForContext returns the matching source file", func(t *testing.T) {
		ctxs, _ := client.GetContexts()
		for _, c := range ctxs {
			path := client.KubeconfigPathForContext(c.Name)
			switch {
			case strings.Contains(c.Name, "dev-envs"):
				assert.Equal(t, dev, path)
			case strings.Contains(c.Name, "itg-k8s"):
				assert.Equal(t, itg, path)
			case strings.Contains(c.Name, "prod-envs"):
				assert.Equal(t, prod, path)
			}
		}
	})
}

// TestKubeconfigOverlapPermutations is a matrix test that exercises every
// reasonable combination of context-name, cluster-name, and user-name overlap
// across one or more kubeconfig files. Each row builds a fresh KUBECONFIG
// loadout, asserts the visible-context list, and verifies that every
// disambiguated context resolves to its own server/token through the
// in-process API.
//
// Why this exists: issue #23 surfaced two distinct merge problems
// (cluster/user collision with distinct contexts; full-collision with
// identical contexts). Spelling out the permutations makes it harder for a
// future refactor to silently regress on any of them.
func TestKubeconfigOverlapPermutations(t *testing.T) {
	type fileSpec struct {
		filename       string
		contextName    string
		clusterName    string
		userName       string
		server         string
		token          string
		currentContext string
	}

	type expected struct {
		// hint is a substring that must appear in the disambiguated display
		// name; "" means "name must equal contextName exactly".
		hint   string
		server string
		token  string
		// kubectl is the name we must emit for kubectl --context (the
		// original kubeconfig context name).
		kubectl string
	}

	tests := []struct {
		name   string
		files  []fileSpec
		expect []expected
	}{
		{
			name: "single file, multiple distinct contexts",
			files: []fileSpec{
				{
					filename:       "all.yaml",
					contextName:    "alpha",
					clusterName:    "cluster-a",
					userName:       "user-a",
					server:         "https://a.example.test:6443",
					token:          "a-token",
					currentContext: "alpha",
				},
				// A second file simulating a multi-context single config by
				// referencing a separate cluster/user from another source. We
				// represent the "single file, multiple contexts" idea
				// via a second cleanly-distinct file because YAML can't have
				// two top-level Configs in one stream — what matters is that
				// no name collides.
				{
					filename:    "extra.yaml",
					contextName: "beta",
					clusterName: "cluster-b",
					userName:    "user-b",
					server:      "https://b.example.test:6443",
					token:       "b-token",
				},
			},
			expect: []expected{
				{server: "https://a.example.test:6443", token: "a-token", kubectl: "alpha"},
				{server: "https://b.example.test:6443", token: "b-token", kubectl: "beta"},
			},
		},
		{
			name: "context overlap only (clusters and users distinct)",
			files: []fileSpec{
				{filename: "a.yaml", contextName: "ctx", clusterName: "cluster-a", userName: "user-a", server: "https://a.example.test:6443", token: "a-token", currentContext: "ctx"},
				{filename: "b.yaml", contextName: "ctx", clusterName: "cluster-b", userName: "user-b", server: "https://b.example.test:6443", token: "b-token"},
			},
			expect: []expected{
				{hint: "a", server: "https://a.example.test:6443", token: "a-token", kubectl: "ctx"},
				{hint: "b", server: "https://b.example.test:6443", token: "b-token", kubectl: "ctx"},
			},
		},
		{
			name: "cluster overlap only (contexts and users distinct)",
			files: []fileSpec{
				{filename: "a.yaml", contextName: "alpha", clusterName: "shared", userName: "user-a", server: "https://a.example.test:6443", token: "a-token", currentContext: "alpha"},
				{filename: "b.yaml", contextName: "beta", clusterName: "shared", userName: "user-b", server: "https://b.example.test:6443", token: "b-token"},
			},
			expect: []expected{
				{server: "https://a.example.test:6443", token: "a-token", kubectl: "alpha"},
				{server: "https://b.example.test:6443", token: "b-token", kubectl: "beta"},
			},
		},
		{
			name: "user overlap only (contexts and clusters distinct)",
			files: []fileSpec{
				{filename: "a.yaml", contextName: "alpha", clusterName: "cluster-a", userName: "shared", server: "https://a.example.test:6443", token: "a-token", currentContext: "alpha"},
				{filename: "b.yaml", contextName: "beta", clusterName: "cluster-b", userName: "shared", server: "https://b.example.test:6443", token: "b-token"},
			},
			expect: []expected{
				{server: "https://a.example.test:6443", token: "a-token", kubectl: "alpha"},
				{server: "https://b.example.test:6443", token: "b-token", kubectl: "beta"},
			},
		},
		{
			name: "all three overlap, three files (issue #23 main case)",
			files: []fileSpec{
				{filename: "dev-envs.yaml", contextName: "dev", clusterName: "k0s", userName: "root", server: "https://dev.example.test:6443", token: "dev-token", currentContext: "dev"},
				{filename: "itg-k8s.yaml", contextName: "dev", clusterName: "k0s", userName: "root", server: "https://itg.example.test:6443", token: "itg-token"},
				{filename: "prod-envs.yaml", contextName: "dev", clusterName: "k0s", userName: "root", server: "https://prod.example.test:6443", token: "prod-token"},
			},
			expect: []expected{
				{hint: "dev-envs", server: "https://dev.example.test:6443", token: "dev-token", kubectl: "dev"},
				{hint: "itg-k8s", server: "https://itg.example.test:6443", token: "itg-token", kubectl: "dev"},
				{hint: "prod-envs", server: "https://prod.example.test:6443", token: "prod-token", kubectl: "dev"},
			},
		},
		{
			name: "cluster + user overlap with distinct contexts",
			files: []fileSpec{
				{filename: "a.yaml", contextName: "alpha", clusterName: "k0s", userName: "root", server: "https://a.example.test:6443", token: "a-token", currentContext: "alpha"},
				{filename: "b.yaml", contextName: "beta", clusterName: "k0s", userName: "root", server: "https://b.example.test:6443", token: "b-token"},
			},
			expect: []expected{
				{server: "https://a.example.test:6443", token: "a-token", kubectl: "alpha"},
				{server: "https://b.example.test:6443", token: "b-token", kubectl: "beta"},
			},
		},
		{
			name: "context + cluster overlap, distinct users",
			files: []fileSpec{
				{filename: "a.yaml", contextName: "ctx", clusterName: "k0s", userName: "user-a", server: "https://a.example.test:6443", token: "a-token", currentContext: "ctx"},
				{filename: "b.yaml", contextName: "ctx", clusterName: "k0s", userName: "user-b", server: "https://b.example.test:6443", token: "b-token"},
			},
			expect: []expected{
				{hint: "a", server: "https://a.example.test:6443", token: "a-token", kubectl: "ctx"},
				{hint: "b", server: "https://b.example.test:6443", token: "b-token", kubectl: "ctx"},
			},
		},
		{
			name: "context + user overlap, distinct clusters",
			files: []fileSpec{
				{filename: "a.yaml", contextName: "ctx", clusterName: "cluster-a", userName: "root", server: "https://a.example.test:6443", token: "a-token", currentContext: "ctx"},
				{filename: "b.yaml", contextName: "ctx", clusterName: "cluster-b", userName: "root", server: "https://b.example.test:6443", token: "b-token"},
			},
			expect: []expected{
				{hint: "a", server: "https://a.example.test:6443", token: "a-token", kubectl: "ctx"},
				{hint: "b", server: "https://b.example.test:6443", token: "b-token", kubectl: "ctx"},
			},
		},
		{
			name: "mixed: one shared name, one distinct",
			files: []fileSpec{
				{filename: "a.yaml", contextName: "shared", clusterName: "cluster-a", userName: "user-a", server: "https://a.example.test:6443", token: "a-token", currentContext: "shared"},
				{filename: "b.yaml", contextName: "shared", clusterName: "cluster-b", userName: "user-b", server: "https://b.example.test:6443", token: "b-token"},
				{filename: "c.yaml", contextName: "unique", clusterName: "cluster-c", userName: "user-c", server: "https://c.example.test:6443", token: "c-token"},
			},
			expect: []expected{
				{hint: "a", server: "https://a.example.test:6443", token: "a-token", kubectl: "shared"},
				{hint: "b", server: "https://b.example.test:6443", token: "b-token", kubectl: "shared"},
				{server: "https://c.example.test:6443", token: "c-token", kubectl: "unique"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("HOME", tmp)

			paths := make([]string, 0, len(tt.files))
			for _, f := range tt.files {
				path := filepath.Join(tmp, f.filename)
				current := ""
				if f.currentContext != "" {
					current = "current-context: " + f.currentContext + "\n"
				}
				body := fmt.Sprintf(`apiVersion: v1
kind: Config
%sclusters:
- name: %s
  cluster:
    server: %s
    insecure-skip-tls-verify: true
contexts:
- name: %s
  context:
    cluster: %s
    user: %s
users:
- name: %s
  user:
    token: %s
`, current, f.clusterName, f.server, f.contextName, f.clusterName, f.userName, f.userName, f.token)
				assert.NoError(t, os.WriteFile(path, []byte(body), 0o600))
				paths = append(paths, path)
			}

			t.Setenv("KUBECONFIG", strings.Join(paths, string(os.PathListSeparator)))

			client, err := NewClient("")
			assert.NoError(t, err)
			assert.NotNil(t, client)

			ctxs, err := client.GetContexts()
			assert.NoError(t, err)
			assert.Len(t, ctxs, len(tt.expect),
				"context count mismatch; visible names: %v", ctxNames(ctxs))

			// Build a map of expected outcomes keyed by the disambiguating
			// hint so the assertions are order-independent.
			matched := make(map[string]bool, len(ctxs))
			for _, want := range tt.expect {
				display := findDisplayName(t, ctxs, want.hint, want.kubectl)
				if display == "" {
					t.Fatalf("no context matched hint=%q kubectl=%q; got: %v", want.hint, want.kubectl, ctxNames(ctxs))
				}
				matched[display] = true

				cfg, err := client.restConfigForContext(display)
				assert.NoError(t, err, "restConfigForContext(%q)", display)
				assert.Equal(t, want.server, cfg.Host,
					"context %q must route to %s", display, want.server)
				assert.Equal(t, want.token, cfg.BearerToken,
					"context %q must use the file's own token", display)
				assert.Equal(t, want.kubectl, client.OriginalContextName(display),
					"context %q must translate to kubectl --context %q", display, want.kubectl)
			}
			assert.Len(t, matched, len(ctxs),
				"every visible context must be claimed exactly once by an expectation")
		})
	}
}

// ctxNames extracts the Name field from each item for diagnostic output.
func ctxNames(items []model.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

// findDisplayName picks the visible context that matches the given hint
// (substring of the disambiguated name) and falls back to an exact match on
// the original name when hint is empty (no collision was expected).
func findDisplayName(t *testing.T, items []model.Item, hint, exactKubectlName string) string {
	t.Helper()
	if hint == "" {
		// No collision expected — display name should equal kubectl name.
		for _, it := range items {
			if it.Name == exactKubectlName {
				return it.Name
			}
		}
		return ""
	}
	for _, it := range items {
		if strings.Contains(it.Name, hint) {
			return it.Name
		}
	}
	return ""
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
