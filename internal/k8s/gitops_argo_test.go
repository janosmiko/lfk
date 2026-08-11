package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// argoStatusResourcesToItems: health persistence shapes.
//
// ArgoCD's default resourceHealthSource ("appTree") never writes a "health"
// key into status.resources, so a bare sync status ("Synced") read as a full
// verdict when health is simply unknown. These tests pin the two real
// cluster shapes: health absent (the default) and health present (clusters
// with resourceHealthSource: resourceHealth), so the working path can never
// silently regress while the unknown-health path gets fixed.

// TestArgoStatusResourcesToItems_MissingHealth_IsLabeledUnknown guards the
// bug: with no "health" key, the child's rendered status must say the health
// is unknown, not read as "Synced" (which implies healthy).
func TestArgoStatusResourcesToItems_MissingHealth_IsLabeledUnknown(t *testing.T) {
	resources := []any{
		map[string]any{
			"group": "external-secrets.io", "kind": "ExternalSecret",
			"name": "light-avalara", "namespace": "apps",
			"status": "Synced", "version": "v1",
		},
	}

	items := argoStatusResourcesToItems(resources)

	require.Len(t, items, 1)
	assert.Equal(t, "Unknown/Synced", items[0].Status,
		"missing health must not render as a bare, healthy-looking sync status")
}

// TestArgoStatusResourcesToItems_MissingHealth_ExplainsWhy guards the
// requirement that the details panel can say why health is unknown: a
// clusters running the appTree default. The row itself stays untouched.
func TestArgoStatusResourcesToItems_MissingHealth_ExplainsWhy(t *testing.T) {
	resources := []any{
		map[string]any{
			"group": "external-secrets.io", "kind": "ExternalSecret",
			"name": "light-avalara", "namespace": "apps",
			"status": "Synced", "version": "v1",
		},
	}

	items := argoStatusResourcesToItems(resources)

	require.Len(t, items, 1)
	var found bool
	for _, kv := range items[0].Columns {
		if kv.Key == "Health Message" {
			found = true
			assert.Contains(t, kv.Value, "resourceHealthSource")
		}
	}
	assert.True(t, found, "details panel must explain why health is unknown")
}

// TestArgoStatusResourcesToItems_PresentHealth_Unchanged is the regression
// guard: when ArgoCD does persist health (resourceHealthSource:
// resourceHealth), the existing "Degraded/Synced" form must stay exactly as
// it is, with no explanatory Health Message column added.
func TestArgoStatusResourcesToItems_PresentHealth_Unchanged(t *testing.T) {
	resources := []any{
		map[string]any{
			"group": "apps", "kind": "Deployment",
			"name": "my-deploy", "namespace": "default",
			"status": "Synced", "version": "v1",
			"health": map[string]any{"status": "Degraded"},
		},
	}

	items := argoStatusResourcesToItems(resources)

	require.Len(t, items, 1)
	assert.Equal(t, "Degraded/Synced", items[0].Status)
	for _, kv := range items[0].Columns {
		assert.NotEqual(t, "Health Message", kv.Key,
			"health-present path must not gain the unknown-health explanation")
	}
}
