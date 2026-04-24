package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- ownedItemKindLabel ---

func TestOwnedItemKindLabel(t *testing.T) {
	tests := []struct {
		kind     string
		expected string
	}{
		{"CronJob", "Job"},
		{"Application", "Resource"},
		{"HelmRelease", "Resource"},
		{"Pod", "Container"},
		{"Node", "Pod"},
		{"Deployment", "Pod"},
		{"StatefulSet", "Pod"},
		{"DaemonSet", "Pod"},
		{"SomeCRD", "Pod"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			m := Model{
				nav: model.NavigationState{
					ResourceType: model.ResourceTypeEntry{Kind: tt.kind},
				},
			}
			assert.Equal(t, tt.expected, m.ownedItemKindLabel())
		})
	}
}

// --- ownedChildKindLabel ---

func TestOwnedChildKindLabel(t *testing.T) {
	t.Run("CronJob returns Job", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "CronJob"},
			},
		}
		assert.Equal(t, "Job", m.ownedChildKindLabel())
	})

	t.Run("Application returns Resource", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Application"},
			},
		}
		assert.Equal(t, "Resource", m.ownedChildKindLabel())
	})

	t.Run("Pod returns Container", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Pod"},
			},
		}
		assert.Equal(t, "Container", m.ownedChildKindLabel())
	})

	t.Run("Node returns Pod", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Node"},
			},
		}
		assert.Equal(t, "Pod", m.ownedChildKindLabel())
	})

	t.Run("Deployment defaults to NAME", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
			},
		}
		assert.Equal(t, "NAME", m.ownedChildKindLabel())
	})

	t.Run("LevelOwned with Pod item returns Container", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelOwned,
				ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
			},
			middleItems: []model.Item{
				{Name: "my-pod", Kind: "Pod"},
			},
		}
		m.setCursor(0)
		assert.Equal(t, "Container", m.ownedChildKindLabel())
	})

	t.Run("LevelOwned with non-Pod item uses resource type kind", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelOwned,
				ResourceType: model.ResourceTypeEntry{Kind: "CronJob"},
			},
			middleItems: []model.Item{
				{Name: "my-job", Kind: "Job"},
			},
		}
		m.setCursor(0)
		assert.Equal(t, "Job", m.ownedChildKindLabel())
	})
}

// --- kindHasOwnedChildren ---

func TestKindHasOwnedChildren(t *testing.T) {
	hasChildren := []string{
		"Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob",
		"Service", "Application", "HelmRelease", "Kustomization", "Node",
		// PVC is listed here so the right-pane preview can lazily show
		// which pods reference the selected PVC. The previous eager
		// "Used By" column in GetResources issued one pod list call per
		// PVC (N+1) — on large namespaces the PVC list took 6+ seconds
		// to arrive.
		"PersistentVolumeClaim",
	}
	for _, kind := range hasChildren {
		t.Run(kind+"_true", func(t *testing.T) {
			assert.True(t, kindHasOwnedChildren(kind))
		})
	}

	noChildren := []string{
		"ConfigMap", "Secret", "Namespace",
		"ServiceAccount", "Ingress", "NetworkPolicy", "SomeCRD",
	}
	for _, kind := range noChildren {
		t.Run(kind+"_false", func(t *testing.T) {
			assert.False(t, kindHasOwnedChildren(kind))
		})
	}
}

// --- resourceTypeHasChildren ---

func TestResourceTypeHasChildren(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
		},
	}
	assert.True(t, m.resourceTypeHasChildren())

	m.nav.ResourceType.Kind = "ConfigMap"
	assert.False(t, m.resourceTypeHasChildren())
}

// --- hasSplitPreview ---

func TestHasSplitPreview(t *testing.T) {
	t.Run("full YAML preview disables split", func(t *testing.T) {
		m := Model{
			fullYAMLPreview: true,
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
			},
			rightItems: []model.Item{{Name: "container"}},
		}
		assert.False(t, m.hasSplitPreview())
	})

	t.Run("map view disables split", func(t *testing.T) {
		m := Model{
			mapView: true,
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
			},
			rightItems: []model.Item{{Name: "container"}},
		}
		assert.False(t, m.hasSplitPreview())
	})

	t.Run("deployment with right items enables split", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
			},
			rightItems: []model.Item{{Name: "pod-1"}},
		}
		assert.True(t, m.hasSplitPreview())
	})

	t.Run("deployment without right items disables split", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
			},
		}
		assert.False(t, m.hasSplitPreview())
	})

	t.Run("Pod at LevelResources with right items enables split", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Pod"},
			},
			rightItems: []model.Item{{Name: "container"}},
		}
		assert.True(t, m.hasSplitPreview())
	})

	t.Run("ConfigMap with right items does not enable split", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
			},
			rightItems: []model.Item{{Name: "data"}},
		}
		assert.False(t, m.hasSplitPreview())
	})
}

// --- logViewHeight ---

func TestLogViewHeight(t *testing.T) {
	tests := []struct {
		name     string
		height   int
		expected int
	}{
		{"normal height", 40, 38},
		{"minimum height", 4, 3},
		{"very small", 1, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{height: tt.height}
			assert.Equal(t, tt.expected, m.logViewHeight())
		})
	}
}

// --- logContentHeight ---

func TestLogContentHeight(t *testing.T) {
	t.Run("normal height single tab", func(t *testing.T) {
		m := Model{height: 40, tabs: []TabState{{}}}
		assert.Equal(t, 35, m.logContentHeight())
	})

	t.Run("normal height multiple tabs", func(t *testing.T) {
		m := Model{height: 40, tabs: []TabState{{}, {}}}
		assert.Equal(t, 34, m.logContentHeight())
	})

	t.Run("very small height", func(t *testing.T) {
		m := Model{height: 3, tabs: []TabState{{}}}
		assert.Equal(t, 1, m.logContentHeight())
	})
}

func TestCovMiddleColumnHeader(t *testing.T) {
	tests := []struct {
		level  model.Level
		kind   string
		expect string
	}{
		{model.LevelClusters, "", "KUBECONFIG"},
		{model.LevelResourceTypes, "", "RESOURCE TYPE"},
		{model.LevelResources, "Pod", "POD"},
		{model.LevelContainers, "", "CONTAINER"},
		{99, "", ""},
	}
	for _, tt := range tests {
		m := Model{nav: model.NavigationState{Level: tt.level, ResourceType: model.ResourceTypeEntry{Kind: tt.kind}}}
		assert.Equal(t, tt.expect, m.middleColumnHeader())
	}
}

func TestCovMiddleColumnHeaderOwned(t *testing.T) {
	for _, kind := range []string{"CronJob", "Application", "Pod", "Node", "Deployment"} {
		m := Model{nav: model.NavigationState{Level: model.LevelOwned, ResourceType: model.ResourceTypeEntry{Kind: kind}}}
		assert.NotEmpty(t, m.middleColumnHeader())
	}
}

func TestCovLeftColumnHeader(t *testing.T) {
	tests := []struct {
		level  model.Level
		dn     string
		expect string
	}{
		{model.LevelClusters, "", ""},
		{model.LevelResourceTypes, "", "KUBECONFIG"},
		{model.LevelResources, "", "RESOURCE TYPE"},
		{model.LevelOwned, "Deployments", "DEPLOYMENTS"},
		{model.LevelContainers, "Pods", "PODS"},
		{99, "", ""},
	}
	for _, tt := range tests {
		m := Model{nav: model.NavigationState{Level: tt.level, ResourceType: model.ResourceTypeEntry{DisplayName: tt.dn}}}
		assert.Equal(t, tt.expect, m.leftColumnHeader())
	}
}
