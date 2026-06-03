package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestNodeReadyStatus(t *testing.T) {
	ready := map[string]any{
		"conditions": []any{
			map[string]any{"type": "MemoryPressure", "status": "False"},
			map[string]any{"type": "Ready", "status": "True"},
		},
	}
	assert.Equal(t, "Ready", nodeReadyStatus(ready))

	notReady := map[string]any{
		"conditions": []any{
			map[string]any{"type": "Ready", "status": "False"},
		},
	}
	assert.Equal(t, "NotReady", nodeReadyStatus(notReady))

	// No Ready condition and nil status leave the status untouched.
	assert.Equal(t, "", nodeReadyStatus(map[string]any{"conditions": []any{}}))
	assert.Equal(t, "", nodeReadyStatus(nil))
}

func TestPopulateNamespaceDetails(t *testing.T) {
	ti := &model.Item{Kind: "Namespace"}
	populateNamespaceDetails(ti, map[string]any{"phase": "Active"})
	assert.Equal(t, "Active", ti.Status)

	term := &model.Item{Kind: "Namespace"}
	populateNamespaceDetails(term, map[string]any{"phase": "Terminating"})
	assert.Equal(t, "Terminating", term.Status)

	// Nil/empty status leaves Status untouched.
	empty := &model.Item{Kind: "Namespace"}
	populateNamespaceDetails(empty, nil)
	assert.Equal(t, "", empty.Status)
}
