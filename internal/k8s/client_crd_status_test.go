package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/model"
)

// --- statusPhase / StatusFromPhase (issue #536) ---

func TestStatusPhase(t *testing.T) {
	got, ok := statusPhase(map[string]any{"status": map[string]any{"phase": "Jumping"}})
	assert.True(t, ok)
	assert.Equal(t, "Jumping", got)

	_, ok = statusPhase(map[string]any{"status": map[string]any{"phase": ""}})
	assert.False(t, ok, "empty phase is not a signal")

	_, ok = statusPhase(map[string]any{"status": map[string]any{"conditions": []any{}}})
	assert.False(t, ok, "no phase key")

	_, ok = statusPhase(map[string]any{})
	assert.False(t, ok, "no status")
}

// TestBuildResourceItem_StatusFromPhase reproduces issue #536: a generic CRD
// exposing .status.phase both as the built-in Status and as a Phase printer
// column. The duplicate Phase column is suppressed, but StatusFromPhase is set
// so the list summary can still roll up by phase.
func TestBuildResourceItem_StatusFromPhase(t *testing.T) {
	c := &Client{}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Chore",
		"metadata":   map[string]any{"name": "mow-lawn", "namespace": "default"},
		"status": map[string]any{
			"phase": "Jumping",
			"conditions": []any{
				map[string]any{"type": "Jumping", "status": "True"},
			},
		},
	}}
	rt := &model.ResourceTypeEntry{
		Kind:       "Chore",
		Namespaced: true,
		PrinterColumns: []model.PrinterColumn{
			{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
		},
	}

	ti := c.buildResourceItem(obj, rt)

	assert.Equal(t, "Jumping", ti.Status)
	assert.True(t, ti.StatusFromPhase, "Status derived from .status.phase must be flagged")
	assert.Empty(t, ti.ColumnValue("Phase"), "duplicate Phase column stays suppressed")
}

// TestBuildResourceItem_StatusFromPhase_Deleting documents the call-order
// interaction with deletion: applyDeletionStatus overwrites Status to
// "Terminating" after StatusFromPhase is set, but populatePrinterColumns runs
// later still, so a declared Phase printer column no longer matches the
// (now "Terminating") Status and survives the dedup. The summary then rolls up
// by that surviving column, not the StatusFromPhase fallback.
func TestBuildResourceItem_StatusFromPhase_Deleting(t *testing.T) {
	c := &Client{}
	now := metav1.Now()
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Chore",
		"metadata": map[string]any{
			"name":              "mow-lawn",
			"namespace":         "default",
			"deletionTimestamp": now.Format(time.RFC3339),
		},
		"status": map[string]any{"phase": "Jumping"},
	}}
	rt := &model.ResourceTypeEntry{
		Kind:       "Chore",
		Namespaced: true,
		PrinterColumns: []model.PrinterColumn{
			{Name: "Phase", Type: "string", JSONPath: ".status.phase"},
		},
	}

	ti := c.buildResourceItem(obj, rt)

	assert.Equal(t, "Terminating", ti.Status)
	assert.True(t, ti.StatusFromPhase)
	assert.Equal(t, "Jumping", ti.ColumnValue("Phase"),
		"Phase column survives dedup once Status is overridden to Terminating")
}

// TestBuildResourceItem_StatusNotFromPhase guards the inverse: a resource whose
// Status comes from conditions (not phase) must leave StatusFromPhase unset.
func TestBuildResourceItem_StatusNotFromPhase(t *testing.T) {
	c := &Client{}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata":   map[string]any{"name": "w1"},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "True"},
			},
		},
	}}
	rt := &model.ResourceTypeEntry{Kind: "Widget"}

	ti := c.buildResourceItem(obj, rt)

	require.Equal(t, "Ready", ti.Status)
	assert.False(t, ti.StatusFromPhase)
}

// --- extractStatus: additional branch coverage ---

func TestExtractStatus_NegativeConditionPrefersTrueCondition(t *testing.T) {
	t.Run("last condition is Failed:False, prefer True condition", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "JobCreated",
						"status": "True",
					},
					map[string]any{
						"type":   "Failed",
						"status": "False",
					},
				},
			},
		}
		// Failed is a negative condition type and has status False,
		// so it should prefer the True condition "JobCreated".
		assert.Equal(t, "JobCreated", extractStatus(obj))
	})

	t.Run("last condition is Error:False, prefer True condition", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Reconciling",
						"status": "True",
					},
					map[string]any{
						"type":   "InternalError",
						"status": "False",
					},
				},
			},
		}
		assert.Equal(t, "Reconciling", extractStatus(obj))
	})

	t.Run("last condition is Degraded:False, prefer True condition", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Healthy",
						"status": "True",
					},
					map[string]any{
						"type":   "Degraded",
						"status": "False",
					},
				},
			},
		}
		assert.Equal(t, "Healthy", extractStatus(obj))
	})

	t.Run("last condition is negative but no True condition, return last type", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Initialized",
						"status": "False",
					},
					map[string]any{
						"type":   "Failed",
						"status": "False",
					},
				},
			},
		}
		// No True condition exists, falls back to lastType.
		assert.Equal(t, "Failed", extractStatus(obj))
	})

	t.Run("last condition is non-negative, return lastType", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "JobCreated",
						"status": "True",
					},
					map[string]any{
						"type":   "Progressing",
						"status": "False",
					},
				},
			},
		}
		// "Progressing" is not a negative type, so use lastType.
		assert.Equal(t, "Progressing", extractStatus(obj))
	})

	t.Run("ArgoCD health with sync that has no status key", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"health": map[string]any{
					"status": "Healthy",
				},
				"sync": map[string]any{
					"revision": "abc123",
				},
			},
		}
		// sync map exists but has no "status" key, falls back to health only.
		assert.Equal(t, "Healthy", extractStatus(obj))
	})

	t.Run("health map with no status key", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"health": map[string]any{
					"message": "degraded",
				},
			},
		}
		// health map exists but no "status" key, returns empty.
		assert.Equal(t, "", extractStatus(obj))
	})

	t.Run("conditions with Ready:True returns immediately", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"status": "True",
					},
					map[string]any{
						"type":   "Failed",
						"status": "True",
					},
				},
			},
		}
		// "Ready" with "True" returns immediately.
		assert.Equal(t, "Ready", extractStatus(obj))
	})
}
