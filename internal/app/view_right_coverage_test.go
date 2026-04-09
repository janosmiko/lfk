package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- renderRightColumnContent: various modes ---

func TestRenderRightColumnContentClusters(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level: model.LevelClusters,
		},
		rightItems: []model.Item{
			{Name: "Pods", Category: "Workloads"},
			{Name: "Deployments", Category: "Workloads"},
		},
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "Pods")
}

func TestRenderRightColumnContentClustersEmpty(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelClusters},
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "No resource types found")
}

func TestRenderRightColumnContentClustersLoading(t *testing.T) {
	m := Model{
		nav:     model.NavigationState{Level: model.LevelClusters},
		loading: true,
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "Loading")
}

func TestRenderRightColumnContentResourceTypesOverview(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{
			{Name: "Cluster Dashboard", Extra: "__overview__"},
		},
		dashboardPreview: "Node: 3",
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "Node: 3")
}

func TestRenderRightColumnContentResourceTypesOverviewLoading(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{
			{Name: "Cluster Dashboard", Extra: "__overview__"},
		},
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "Loading cluster dashboard")
}

func TestRenderRightColumnContentResourceTypesMonitoring(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{
			{Name: "Monitoring", Extra: "__monitoring__"},
		},
		monitoringPreview: "CPU: 45%",
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "CPU: 45%")
}

func TestRenderRightColumnContentResourceTypesMonitoringLoading(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{
			{Name: "Monitoring", Extra: "__monitoring__"},
		},
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "Loading monitoring dashboard")
}

func TestRenderRightResourceTypesSecurityBranch(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{
			{Name: "Security", Extra: "__security__"},
		},
		securityView: ui.SecurityViewState{Loading: true},
	}
	out := m.renderRightResourceTypes(100, 20)
	assert.Contains(t, out, "Loading security findings")
}

func TestRenderRightColumnContentResourceTypesSecurity(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{
			{Name: "Security", Extra: "__security__"},
		},
		securityView: ui.SecurityViewState{Loading: true},
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "Loading security findings")
}

func TestRenderRightColumnContentFullYAMLPreview(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
		},
		fullYAMLPreview: true,
		previewYAML:     "apiVersion: v1\nkind: ConfigMap",
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "apiVersion")
}

func TestRenderRightColumnContentFullYAMLPreviewLoading(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
		},
		fullYAMLPreview: true,
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "Loading YAML")
}

func TestRenderRightColumnContentContainers(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelContainers,
			ResourceType: model.ResourceTypeEntry{Kind: "Pod"},
		},
		middleItems: []model.Item{
			{
				Name: "nginx",
				Kind: "Container",
				Columns: []model.KeyValue{
					{Key: "Image", Value: "nginx:latest"},
				},
			},
		},
	}
	result := m.renderRightColumnContent(80, 20)
	assert.NotEmpty(t, result)
}

func TestRenderRightColumnContentNoResourcesFallbackYAML(t *testing.T) {
	// ConfigMap with no middle items and no YAML: falls through to "No preview".
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
		},
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "No preview")
}

func TestRenderRightColumnContentNoRightItemsLoading(t *testing.T) {
	// A kind with children (Deployment) with no rightItems falls through
	// to the generic "no right items" path showing "No resources found".
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
		},
		loading: true,
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "Loading")
}

// --- renderRightColumn with events and metrics ---

func TestRenderRightColumnWithEvents(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
		},
		middleItems: []model.Item{
			{
				Name: "cm-1",
				Kind: "ConfigMap",
				Columns: []model.KeyValue{
					{Key: "Data", Value: "2"},
				},
			},
		},
		previewEventsContent: "Event: Normal Created",
	}
	result := m.renderRightColumn(80, 20)
	assert.NotEmpty(t, result)
}

func TestRenderRightColumnWithMetrics(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
		},
		middleItems: []model.Item{
			{
				Name: "cm-1",
				Kind: "ConfigMap",
				Columns: []model.KeyValue{
					{Key: "Data", Value: "2"},
				},
			},
		},
		metricsContent: "CPU: 100m MEM: 256Mi",
	}
	result := m.renderRightColumn(80, 20)
	assert.NotEmpty(t, result)
}

// --- renderDetailsOnly ---

func TestRenderDetailsOnlyWithColumns(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Pod"},
		},
		middleItems: []model.Item{
			{
				Name: "pod-1",
				Columns: []model.KeyValue{
					{Key: "Status", Value: "Running"},
					{Key: "Node", Value: "worker-1"},
				},
			},
		},
	}
	result := m.renderDetailsOnly(80, 20)
	assert.Contains(t, result, "DETAILS")
	assert.Contains(t, result, "pod-1", "summary should include resource name as NAME row")
}

func TestRenderDetailsOnlyWithYAML(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
		},
		middleItems: []model.Item{{Name: "cm-1"}},
		previewYAML: "apiVersion: v1\nkind: ConfigMap",
	}
	result := m.renderDetailsOnly(80, 20)
	assert.Contains(t, result, "DETAILS")
	assert.Contains(t, result, "apiVersion")
}

func TestRenderDetailsOnlyNoData(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
		},
		middleItems: []model.Item{{Name: "cm-1"}},
	}
	result := m.renderDetailsOnly(80, 20)
	assert.Contains(t, result, "No details available")
}

func TestRenderDetailsOnlyNoSelection(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Pod"},
		},
		middleItems: []model.Item{},
	}
	result := m.renderDetailsOnly(80, 20)
	assert.Contains(t, result, "DETAILS", "header should still show DETAILS when nothing selected")
}

// --- renderRightColumnContent: owned level ---

func TestRenderRightColumnContentOwnedPodWithColumns(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelOwned,
			ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
		},
		middleItems: []model.Item{
			{
				Name: "my-rs",
				Kind: "ReplicaSet",
				Columns: []model.KeyValue{
					{Key: "Ready", Value: "3/3"},
				},
			},
		},
	}
	result := m.renderRightColumnContent(80, 20)
	assert.NotEmpty(t, result)
}

func TestRenderRightColumnContentMapView(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
		},
		mapView: true,
	}
	result := m.renderRightColumnContent(80, 20)
	assert.Contains(t, result, "Loading resource tree")
}
