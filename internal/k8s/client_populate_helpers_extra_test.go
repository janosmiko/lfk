package k8s

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- evaluateSimpleJSONPath: extra branches ---

func TestEvaluateSimpleJSONPath_ExtraBranches(t *testing.T) {
	tests := []struct {
		name    string
		obj     map[string]any
		path    string
		wantVal any
		wantOK  bool
	}{
		{
			name: "array index on non-array returns false",
			obj: map[string]any{
				"items": "not-an-array",
			},
			path:    ".items[0]",
			wantVal: nil,
			wantOK:  false,
		},
		{
			name: "nil intermediate value returns false",
			obj: map[string]any{
				"status": nil,
			},
			path:    ".status.phase",
			wantVal: nil,
			wantOK:  false,
		},
		{
			name: "deeply nested path",
			obj: map[string]any{
				"a": map[string]any{
					"b": map[string]any{
						"c": map[string]any{
							"d": 42,
						},
					},
				},
			},
			path:    ".a.b.c.d",
			wantVal: 42,
			wantOK:  true,
		},
		{
			name: "bracket without valid index treats field as non-indexed",
			obj: map[string]any{
				"items": []any{"a"},
			},
			path:    ".items[]",
			wantVal: []any{"a"},
			wantOK:  true,
		},
		{
			name:    "path with only dot returns false",
			obj:     map[string]any{"x": 1},
			path:    ".",
			wantVal: nil,
			wantOK:  false,
		},
		{
			name: "non-map intermediate value returns false",
			obj: map[string]any{
				"status": "a-string-not-a-map",
			},
			path:    ".status.phase",
			wantVal: nil,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := evaluateSimpleJSONPath(tt.obj, tt.path)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantVal, val)
			}
		})
	}
}

// --- formatPrinterValue: extra branches ---

func TestFormatPrinterValue_ExtraBranches(t *testing.T) {
	tests := []struct {
		name    string
		val     any
		colType string
		want    string
	}{
		{
			name:    "date type with RFC3339Nano",
			val:     time.Now().Add(-2 * time.Hour).Format(time.RFC3339Nano),
			colType: "date",
			want:    "2h",
		},
		{
			name:    "date type with non-string returns formatted",
			val:     12345,
			colType: "date",
			want:    "12345",
		},
		{
			name:    "number type with int64",
			val:     int64(7),
			colType: "number",
			want:    "7",
		},
		{
			name:    "number type with other falls back",
			val:     "three",
			colType: "number",
			want:    "three",
		},
		{
			name:    "integer type with other falls back",
			val:     "seven",
			colType: "integer",
			want:    "seven",
		},
		{
			name:    "boolean type with non-bool falls back",
			val:     "yes",
			colType: "boolean",
			want:    "yes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPrinterValue(tt.val, tt.colType)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- extractGenericConditions missing branches ---

func TestExtractGenericConditions_MissingBranches(t *testing.T) {
	t.Run("no Ready condition falls back to last condition", func(t *testing.T) {
		ti := &model.Item{}
		conditions := []any{
			map[string]any{
				"type":    "Available",
				"status":  "True",
				"reason":  "MinPods",
				"message": "all pods available",
			},
		}

		extractGenericConditions(ti, conditions)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "True", colMap["Available"])
		assert.Equal(t, "MinPods", colMap["Reason"])
		// Message should not appear since status is True.
		_, hasMsg := colMap["Message"]
		assert.False(t, hasMsg)
	})

	t.Run("non-map condition entries are skipped", func(t *testing.T) {
		ti := &model.Item{}
		conditions := []any{
			"not-a-map",
			map[string]any{
				"type":   "Ready",
				"status": "False",
				"reason": "NotReady",
			},
		}

		extractGenericConditions(ti, conditions)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "False", colMap["Ready"])
	})

	t.Run("all non-map conditions produces no output", func(t *testing.T) {
		ti := &model.Item{}
		conditions := []any{
			"not-a-map",
			42,
		}

		extractGenericConditions(ti, conditions)

		assert.Empty(t, ti.Columns)
	})

	t.Run("message shown when status is not True", func(t *testing.T) {
		ti := &model.Item{}
		conditions := []any{
			map[string]any{
				"type":    "Ready",
				"status":  "False",
				"message": "Pod is not ready",
			},
		}

		extractGenericConditions(ti, conditions)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "Pod is not ready", colMap["Message"])
	})

	t.Run("long message is truncated", func(t *testing.T) {
		var longMsg strings.Builder
		for range 100 {
			longMsg.WriteString("x")
		}
		ti := &model.Item{}
		conditions := []any{
			map[string]any{
				"type":    "Ready",
				"status":  "False",
				"message": longMsg.String(),
			},
		}

		extractGenericConditions(ti, conditions)

		colMap := columnsToMap(ti.Columns)
		assert.LessOrEqual(t, len(colMap["Message"]), 80)
		assert.Contains(t, colMap["Message"], "...")
	})

	t.Run("condition with empty type and status produces no type column", func(t *testing.T) {
		ti := &model.Item{}
		conditions := []any{
			map[string]any{
				"reason": "SomeReason",
			},
		}

		extractGenericConditions(ti, conditions)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "SomeReason", colMap["Reason"])
		// No type/status column should be added.
		_, hasType := colMap[""]
		assert.False(t, hasType)
	})
}

// --- extractTemplateResources: missing no-containers-in-spec branch ---

func TestExtractTemplateResources_NoContainersInTemplateSpec(t *testing.T) {
	spec := map[string]any{
		"template": map[string]any{
			"spec": map[string]any{
				// No "containers" key at all.
				"nodeSelector": map[string]any{"zone": "us-east"},
			},
		},
	}

	cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)

	assert.Empty(t, cpuReq)
	assert.Empty(t, cpuLim)
	assert.Empty(t, memReq)
	assert.Empty(t, memLim)
}

// --- populateContainerImages: missing branch when containers is not []interface{} ---

func TestPopulateContainerImages_NonSliceContainers(t *testing.T) {
	ti := &model.Item{}
	spec := map[string]any{
		"template": map[string]any{
			"spec": map[string]any{
				"containers": "not-a-slice",
			},
		},
	}

	populateContainerImages(ti, spec)

	assert.Empty(t, ti.Columns)
}

func TestPopulateContainerImages_NonMapTemplateSpec(t *testing.T) {
	ti := &model.Item{}
	spec := map[string]any{
		"template": map[string]any{
			"spec": "not-a-map",
		},
	}

	populateContainerImages(ti, spec)

	assert.Empty(t, ti.Columns)
}
