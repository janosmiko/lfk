package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActionsForKind_OffersExportTemplate: every kind can be turned into a
// template, so the entry is appended to every menu rather than listed per kind.
func TestActionsForKind_OffersExportTemplate(t *testing.T) {
	kinds := []string{
		"Pod", "Deployment", "Service", "Node", "PersistentVolumeClaim",
		"Job", "ConfigMap", "Application", "HelmRelease", "UnknownKind",
	}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			var found bool
			for _, a := range ActionsForKind(kind) {
				if a.Label == ActionLabelExportTemplate {
					found = true
					assert.Equal(t, "x", a.Key)
					assert.NotEmpty(t, a.Description)
				}
			}
			require.True(t, found, "%s menu is missing %q", kind, ActionLabelExportTemplate)
		})
	}
}

// TestActionsForContainer_OffersExportTemplate: resolveTemplateSource
// resolves LevelContainers to the parent Pod's manifest, so the container
// menu must offer the same Export Template entry as every other kind.
func TestActionsForContainer_OffersExportTemplate(t *testing.T) {
	var found bool
	for _, a := range ActionsForContainer() {
		if a.Label == ActionLabelExportTemplate {
			found = true
			assert.Equal(t, "x", a.Key)
			assert.NotEmpty(t, a.Description)
		}
	}
	require.True(t, found, "container menu is missing %q", ActionLabelExportTemplate)
}

// TestActionsForKind_ExportTemplateKeyIsFree guards the appended hotkey against
// a per-kind entry that already uses it.
func TestActionsForKind_ExportTemplateKeyIsFree(t *testing.T) {
	kinds := []string{
		"Pod", "Deployment", "Service", "Node", "PersistentVolumeClaim",
		"Job", "ConfigMap", "Application", "HelmRelease", "Certificate",
		"NodeClaim", "Revision", "NetworkPolicy", "HorizontalPodAutoscaler",
		"UnknownKind",
	}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			seen := map[string]string{}
			for _, a := range ActionsForKind(kind) {
				if a.Key == "" {
					continue
				}
				prev, dup := seen[a.Key]
				assert.False(t, dup, "hotkey %q reused by %q and %q", a.Key, prev, a.Label)
				seen[a.Key] = a.Label
			}
		})
	}
}
