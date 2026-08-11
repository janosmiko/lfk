package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- RenderRBACOverlay ---

func TestRenderRBACOverlay(t *testing.T) {
	t.Run("basic rendering with mixed results", func(t *testing.T) {
		results := []RBACCheckEntry{
			{Verb: "get", Allowed: true},
			{Verb: "create", Allowed: false},
			{Verb: "delete", Allowed: true},
		}
		result := RenderRBACOverlay(results, "pods")
		assert.Contains(t, result, "RBAC Permissions: pods")
		assert.Contains(t, result, "get")
		assert.Contains(t, result, "create")
		assert.Contains(t, result, "delete")
		// Check mark for allowed.
		assert.Contains(t, result, "\u2713")
		// Cross mark for denied.
		assert.Contains(t, result, "\u2717")
	})

	t.Run("empty results shows title", func(t *testing.T) {
		result := RenderRBACOverlay(nil, "secrets")
		assert.Contains(t, result, "RBAC Permissions: secrets")
		// Hint moved to status bar.
		assert.NotContains(t, result, "Press any key to close")
	})

	t.Run("all allowed", func(t *testing.T) {
		results := []RBACCheckEntry{
			{Verb: "get", Allowed: true},
			{Verb: "list", Allowed: true},
		}
		result := RenderRBACOverlay(results, "configmaps")
		assert.Contains(t, result, "\u2713")
		assert.NotContains(t, result, "\u2717")
	})
}
