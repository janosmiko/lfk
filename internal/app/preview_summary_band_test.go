package app

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func argoAppItem(health, sync string) model.Item {
	return model.Item{
		Kind: "Application",
		Columns: []model.KeyValue{
			{Key: "Health", Value: health},
			{Key: "Sync Status", Value: sync},
		},
	}
}

// resourceTypeModel builds a Model parked on the resource-type list with the
// cursor on a single hovered type row, and the given resources previewed in the
// children pane (rightItems).
func resourceTypeModel(typeRow model.Item, preview []model.Item) Model {
	return Model{
		nav:         model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{typeRow},
		rightItems:  preview,
	}
}

func TestPreviewSummaryBand_ArgoApplications(t *testing.T) {
	m := resourceTypeModel(
		model.Item{Kind: "Application", Name: "Applications"},
		[]model.Item{argoAppItem("Healthy", "Synced"), argoAppItem("Degraded", "OutOfSync")},
	)

	plain := ansi.Strip(m.previewSummaryBand(70))
	assert.Contains(t, plain, "Applications")
	assert.Contains(t, plain, "Health")
	assert.Contains(t, plain, "Degraded")
	assert.Contains(t, plain, "Sync")
	assert.Contains(t, plain, "OutOfSync")
}

func TestPreviewSummaryBand_CountOnlyForStatuslessKind(t *testing.T) {
	// Statusless kinds still get a band — just the resource count, no status bar.
	m := resourceTypeModel(
		model.Item{Kind: "ConfigMap", Name: "ConfigMaps"},
		[]model.Item{{Kind: "ConfigMap", Name: "a"}, {Kind: "ConfigMap", Name: "b"}},
	)
	plain := ansi.Strip(m.previewSummaryBand(70))
	assert.Contains(t, plain, "2 ConfigMaps")
	assert.NotContains(t, plain, "Status")
}

func TestPreviewSummaryBand_NodeReadiness(t *testing.T) {
	m := resourceTypeModel(
		model.Item{Kind: "Node", Name: "Nodes"},
		[]model.Item{{Kind: "Node", Status: "Ready"}, {Kind: "Node", Status: "NotReady"}},
	)
	plain := ansi.Strip(m.previewSummaryBand(70))
	assert.Contains(t, plain, "Nodes")
	assert.Contains(t, plain, "Ready")
	assert.Contains(t, plain, "NotReady")
}

func TestPreviewSummaryBand_EmptyForSyntheticTypeRow(t *testing.T) {
	// Security source / dashboard / port-forward rows carry a "__"-prefixed
	// Kind and must not get a status rollup even if rightItems are present.
	m := resourceTypeModel(
		model.Item{Kind: model.SecurityLoaderKind, Name: "Security"},
		[]model.Item{{Kind: "Pod", Status: "Running"}},
	)
	assert.Empty(t, m.previewSummaryBand(70))
}

func TestPreviewSummaryBand_EmptyBeforePreviewLoads(t *testing.T) {
	m := resourceTypeModel(model.Item{Kind: "Application", Name: "Applications"}, nil)
	assert.Empty(t, m.previewSummaryBand(70), "no band until the children preview has loaded")
}

func TestPreviewSummaryBand_HiddenAfterDrillingIn(t *testing.T) {
	// Once drilled into the resource list (LevelResources), the band is gone —
	// it is a resource-type-list affordance only.
	m := Model{
		nav:         model.NavigationState{Level: model.LevelResources, ResourceType: model.ResourceTypeEntry{Kind: "Application"}},
		middleItems: []model.Item{argoAppItem("Healthy", "Synced")},
		rightItems:  []model.Item{argoAppItem("Healthy", "Synced")},
	}
	assert.Empty(t, m.previewSummaryBand(70))
}
