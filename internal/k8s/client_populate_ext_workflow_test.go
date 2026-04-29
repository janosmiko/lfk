package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// --- populateArgoWorkflow ---

func TestPopulateArgoWorkflow(t *testing.T) {
	t.Run("nil status returns early", func(t *testing.T) {
		ti := &model.Item{Name: "my-wf"}
		populateArgoWorkflow(ti, nil)
		assert.Empty(t, ti.Columns)
		assert.Empty(t, ti.Conditions)
	})

	t.Run("empty status returns no columns", func(t *testing.T) {
		ti := &model.Item{Name: "my-wf"}
		populateArgoWorkflow(ti, map[string]any{})
		assert.Empty(t, ti.Columns)
	})

	t.Run("progress field is extracted", func(t *testing.T) {
		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"progress": "2/5",
		}
		populateArgoWorkflow(ti, status)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "2/5", colMap["Progress"])
	})

	t.Run("duration from startedAt and finishedAt", func(t *testing.T) {
		started := time.Now().Add(-5 * time.Minute)
		finished := started.Add(3 * time.Minute)

		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"startedAt":  started.Format(time.RFC3339),
			"finishedAt": finished.Format(time.RFC3339),
		}
		populateArgoWorkflow(ti, status)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "3m0s", colMap["Duration"])
	})

	t.Run("duration from startedAt only uses now", func(t *testing.T) {
		started := time.Now().Add(-2 * time.Minute)

		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"startedAt": started.Format(time.RFC3339),
		}
		populateArgoWorkflow(ti, status)

		colMap := columnsToMap(ti.Columns)
		dur, ok := colMap["Duration"]
		require.True(t, ok, "Duration column should exist")
		assert.NotEmpty(t, dur)
	})

	t.Run("message field is extracted", func(t *testing.T) {
		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"message": "workflow failed at step X",
		}
		populateArgoWorkflow(ti, status)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "workflow failed at step X", colMap["Message"])
	})

	t.Run("conditions are extracted", func(t *testing.T) {
		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"conditions": []any{
				map[string]any{
					"type":    "SpecWarning",
					"status":  "True",
					"message": "deprecated feature used",
				},
				map[string]any{
					"type":    "PodRunning",
					"status":  "True",
					"message": "",
				},
			},
		}
		populateArgoWorkflow(ti, status)

		require.Len(t, ti.Conditions, 2)
		assert.Equal(t, "SpecWarning", ti.Conditions[0].Type)
		assert.Equal(t, "True", ti.Conditions[0].Status)
		assert.Equal(t, "deprecated feature used", ti.Conditions[0].Message)
		assert.Equal(t, "PodRunning", ti.Conditions[1].Type)
	})

	t.Run("conditions skips non-map entries", func(t *testing.T) {
		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"conditions": []any{
				"not-a-map",
				map[string]any{
					"type":   "PodRunning",
					"status": "True",
				},
			},
		}
		populateArgoWorkflow(ti, status)

		require.Len(t, ti.Conditions, 1)
		assert.Equal(t, "PodRunning", ti.Conditions[0].Type)
	})

	t.Run("conditions with empty type is skipped", func(t *testing.T) {
		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"conditions": []any{
				map[string]any{
					"type":   "",
					"status": "True",
				},
			},
		}
		populateArgoWorkflow(ti, status)

		assert.Empty(t, ti.Conditions)
	})

	t.Run("workflow nodes are walked in BFS order", func(t *testing.T) {
		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"nodes": map[string]any{
				"root-id": map[string]any{
					"name":        "my-wf",
					"displayName": "my-wf",
					"phase":       "Succeeded",
					"children":    []any{"step-1", "step-2"},
				},
				"step-1": map[string]any{
					"name":        "my-wf.step-1",
					"displayName": "build",
					"phase":       "Succeeded",
					"children":    []any{"step-1a"},
				},
				"step-2": map[string]any{
					"name":        "my-wf.step-2",
					"displayName": "deploy",
					"phase":       "Running",
					"message":     "deploying",
				},
				"step-1a": map[string]any{
					"name":        "my-wf.step-1.sub",
					"displayName": "compile",
					"phase":       "Succeeded",
				},
			},
		}
		populateArgoWorkflow(ti, status)

		// Expect steps in BFS order: step-1 (build), step-2 (deploy), step-1a (compile).
		// Root node (my-wf) should be skipped.
		stepColumns := make([]model.KeyValue, 0)
		for _, col := range ti.Columns {
			if len(col.Key) > 5 && col.Key[:5] == "step:" {
				stepColumns = append(stepColumns, col)
			}
		}
		require.Len(t, stepColumns, 3)
		assert.Equal(t, "step:build", stepColumns[0].Key)
		assert.Equal(t, "Succeeded", stepColumns[0].Value)
		assert.Equal(t, "step:deploy", stepColumns[1].Key)
		assert.Equal(t, "Running: deploying", stepColumns[1].Value)
		assert.Equal(t, "step:compile", stepColumns[2].Key)
		assert.Equal(t, "Succeeded", stepColumns[2].Value)
	})

	t.Run("node without displayName falls back to name", func(t *testing.T) {
		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"nodes": map[string]any{
				"root-id": map[string]any{
					"name":     "my-wf",
					"phase":    "Succeeded",
					"children": []any{"step-1"},
				},
				"step-1": map[string]any{
					"name":  "my-wf.step-1",
					"phase": "Succeeded",
				},
			},
		}
		populateArgoWorkflow(ti, status)

		stepColumns := make([]model.KeyValue, 0)
		for _, col := range ti.Columns {
			if len(col.Key) > 5 && col.Key[:5] == "step:" {
				stepColumns = append(stepColumns, col)
			}
		}
		require.Len(t, stepColumns, 1)
		assert.Equal(t, "step:my-wf.step-1", stepColumns[0].Key)
	})

	t.Run("non-map node entries are skipped", func(t *testing.T) {
		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"nodes": map[string]any{
				"root-id": map[string]any{
					"name":     "my-wf",
					"phase":    "Succeeded",
					"children": []any{"step-1"},
				},
				"step-1":   "not-a-map",
				"step-bad": 42,
			},
		}
		populateArgoWorkflow(ti, status)

		// No steps should be added since step-1 is not a map.
		stepColumns := make([]model.KeyValue, 0)
		for _, col := range ti.Columns {
			if len(col.Key) > 5 && col.Key[:5] == "step:" {
				stepColumns = append(stepColumns, col)
			}
		}
		assert.Empty(t, stepColumns)
	})

	t.Run("non-string children entries are skipped", func(t *testing.T) {
		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"nodes": map[string]any{
				"root-id": map[string]any{
					"name":     "my-wf",
					"phase":    "Succeeded",
					"children": []any{42, "step-1"},
				},
				"step-1": map[string]any{
					"name":        "my-wf.step-1",
					"displayName": "build",
					"phase":       "Running",
				},
			},
		}
		populateArgoWorkflow(ti, status)

		stepColumns := make([]model.KeyValue, 0)
		for _, col := range ti.Columns {
			if len(col.Key) > 5 && col.Key[:5] == "step:" {
				stepColumns = append(stepColumns, col)
			}
		}
		require.Len(t, stepColumns, 1)
		assert.Equal(t, "step:build", stepColumns[0].Key)
	})

	t.Run("all fields together", func(t *testing.T) {
		started := time.Now().Add(-10 * time.Minute)
		finished := started.Add(8 * time.Minute)

		ti := &model.Item{Name: "full-wf"}
		status := map[string]any{
			"progress":   "5/5",
			"startedAt":  started.Format(time.RFC3339),
			"finishedAt": finished.Format(time.RFC3339),
			"message":    "workflow completed",
			"conditions": []any{
				map[string]any{
					"type":   "Completed",
					"status": "True",
				},
			},
			"nodes": map[string]any{
				"root": map[string]any{
					"name":     "full-wf",
					"phase":    "Succeeded",
					"children": []any{"s1"},
				},
				"s1": map[string]any{
					"displayName": "final-step",
					"phase":       "Succeeded",
				},
			},
		}
		populateArgoWorkflow(ti, status)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "5/5", colMap["Progress"])
		assert.Equal(t, "8m0s", colMap["Duration"])
		assert.Equal(t, "workflow completed", colMap["Message"])
		assert.Equal(t, "Succeeded", colMap["step:final-step"])

		require.Len(t, ti.Conditions, 1)
		assert.Equal(t, "Completed", ti.Conditions[0].Type)
	})

	t.Run("invalid startedAt is ignored", func(t *testing.T) {
		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"startedAt": "not-a-date",
		}
		populateArgoWorkflow(ti, status)

		colMap := columnsToMap(ti.Columns)
		_, hasDuration := colMap["Duration"]
		assert.False(t, hasDuration)
	})

	t.Run("invalid finishedAt uses now", func(t *testing.T) {
		started := time.Now().Add(-1 * time.Minute)
		ti := &model.Item{Name: "my-wf"}
		status := map[string]any{
			"startedAt":  started.Format(time.RFC3339),
			"finishedAt": "not-a-date",
		}
		populateArgoWorkflow(ti, status)

		colMap := columnsToMap(ti.Columns)
		dur, ok := colMap["Duration"]
		require.True(t, ok)
		assert.NotEmpty(t, dur)
	})
}
