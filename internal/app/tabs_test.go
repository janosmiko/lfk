package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- pushLeft / popLeft ---

func TestPushLeftPopLeft(t *testing.T) {
	m := Model{
		leftItems: []model.Item{{Name: "ctx-1"}, {Name: "ctx-2"}},
		middleItems: []model.Item{
			{Name: "Pods"}, {Name: "Deployments"},
		},
	}

	m.pushLeft()
	assert.Equal(t, "Pods", m.leftItems[0].Name)
	assert.Len(t, m.leftItemsHistory, 1)
	assert.Equal(t, "ctx-1", m.leftItemsHistory[0][0].Name)

	// Push again.
	m.middleItems = []model.Item{{Name: "pod-1"}}
	m.pushLeft()
	assert.Equal(t, "pod-1", m.leftItems[0].Name)
	assert.Len(t, m.leftItemsHistory, 2)

	// Pop restores.
	m.popLeft()
	assert.Equal(t, "Pods", m.leftItems[0].Name)
	assert.Len(t, m.leftItemsHistory, 1)

	m.popLeft()
	assert.Equal(t, "ctx-1", m.leftItems[0].Name)
	assert.Len(t, m.leftItemsHistory, 0)

	// Pop on empty sets leftItems to nil.
	m.popLeft()
	assert.Nil(t, m.leftItems)
}

// --- clearRight ---

func TestClearRight(t *testing.T) {
	m := Model{
		rightItems:           []model.Item{{Name: "container-1"}},
		yamlContent:          "apiVersion: v1",
		previewYAML:          "preview yaml",
		metricsContent:       "metrics",
		previewEventsContent: "events",
		mapView:              true,
	}

	m.clearRight()

	assert.Nil(t, m.rightItems)
	assert.Empty(t, m.yamlContent)
	assert.Empty(t, m.previewYAML)
	assert.Empty(t, m.metricsContent)
	assert.Empty(t, m.previewEventsContent)
	assert.False(t, m.mapView)
}

// --- effectiveNamespace ---

func TestEffectiveNamespace(t *testing.T) {
	tests := []struct {
		name               string
		namespace          string
		allNamespaces      bool
		selectedNamespaces map[string]bool
		expected           string
	}{
		{
			name:      "single namespace",
			namespace: "default",
			expected:  "default",
		},
		{
			name:          "allNamespaces returns empty",
			namespace:     "default",
			allNamespaces: true,
			expected:      "",
		},
		{
			name:               "multiple selected returns empty",
			namespace:          "default",
			selectedNamespaces: map[string]bool{"ns-1": true, "ns-2": true},
			expected:           "",
		},
		{
			name:               "single selected returns that namespace",
			namespace:          "default",
			selectedNamespaces: map[string]bool{"production": true},
			expected:           "production",
		},
		{
			name:      "empty namespace",
			namespace: "",
			expected:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				namespace:          tt.namespace,
				allNamespaces:      tt.allNamespaces,
				selectedNamespaces: tt.selectedNamespaces,
			}
			assert.Equal(t, tt.expected, m.effectiveNamespace())
		})
	}
}

// --- sortModeName ---

func TestSortModeName(t *testing.T) {
	// Set up sortable columns so sortModeName can reference them.
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status"}
	defer func() { ui.ActiveSortableColumns = nil }()

	m := Model{sortColumnName: "Name", sortAscending: true}
	assert.Contains(t, m.sortModeName(), "Name")

	m.sortColumnName = "Age"
	assert.Contains(t, m.sortModeName(), "Age")

	m.sortColumnName = "" // empty falls back to default
	assert.Contains(t, m.sortModeName(), "Name")
}

// --- sortMiddleItems ---

func TestSortMiddleItemsByName(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status"}
	defer func() { ui.ActiveSortableColumns = nil }()
	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources},
		sortColumnName: "Name", sortAscending: true,
		middleItems: []model.Item{
			{Name: "charlie"},
			{Name: "alpha"},
			{Name: "bravo"},
		},
	}
	m.sortMiddleItems()
	assert.Equal(t, "alpha", m.middleItems[0].Name)
	assert.Equal(t, "bravo", m.middleItems[1].Name)
	assert.Equal(t, "charlie", m.middleItems[2].Name)
}

func TestSortMiddleItemsByAge(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status"}
	defer func() { ui.ActiveSortableColumns = nil }()
	now := time.Now()
	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources},
		sortColumnName: "Age", sortAscending: true,
		middleItems: []model.Item{
			{Name: "old", CreatedAt: now.Add(-10 * time.Hour)},
			{Name: "new", CreatedAt: now.Add(-1 * time.Hour)},
			{Name: "no-time"},
		},
	}
	m.sortMiddleItems()
	// Newest first, then items with zero time at the end.
	assert.Equal(t, "new", m.middleItems[0].Name)
	assert.Equal(t, "old", m.middleItems[1].Name)
	assert.Equal(t, "no-time", m.middleItems[2].Name)
}

func TestSortMiddleItemsByStatus(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status"}
	defer func() { ui.ActiveSortableColumns = nil }()
	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources},
		sortColumnName: "Status", sortAscending: true,
		middleItems: []model.Item{
			{Name: "err-pod", Status: "CrashLoopBackOff"},
			{Name: "run-pod", Status: "Running"},
			{Name: "pend-pod", Status: "Pending"},
		},
	}
	m.sortMiddleItems()
	assert.Equal(t, "run-pod", m.middleItems[0].Name)
	assert.Equal(t, "pend-pod", m.middleItems[1].Name)
	assert.Equal(t, "err-pod", m.middleItems[2].Name)
}

func TestSortMiddleItemsSkipsResourceTypes(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name"}
	defer func() { ui.ActiveSortableColumns = nil }()
	m := Model{
		nav:            model.NavigationState{Level: model.LevelResourceTypes},
		sortColumnName: "Name", sortAscending: true,
		middleItems: []model.Item{
			{Name: "charlie"},
			{Name: "alpha"},
		},
	}
	m.sortMiddleItems()
	// Should not sort at LevelResourceTypes.
	assert.Equal(t, "charlie", m.middleItems[0].Name)
	assert.Equal(t, "alpha", m.middleItems[1].Name)
}

// --- sanitizeError ---

func TestSanitizeError(t *testing.T) {
	m := Model{width: 100}

	t.Run("strips newlines and carriage returns", func(t *testing.T) {
		err := errors.New("line1\nline2\r\nline3")
		result := m.sanitizeError(err)
		assert.NotContains(t, result, "\n")
		assert.NotContains(t, result, "\r")
	})

	t.Run("collapses multiple spaces", func(t *testing.T) {
		err := errors.New("too   many    spaces")
		result := m.sanitizeError(err)
		assert.NotContains(t, result, "  ")
	})

	t.Run("truncates long messages", func(t *testing.T) {
		longMsg := strings.Repeat("x", 200)
		err := errors.New(longMsg)
		result := m.sanitizeError(err)
		assert.True(t, len(result) <= m.width-20)
		assert.True(t, len(result) > 0)
	})

	t.Run("minimum maxLen is 40", func(t *testing.T) {
		small := Model{width: 10}
		longMsg := strings.Repeat("x", 100)
		err := errors.New(longMsg)
		result := small.sanitizeError(err)
		assert.True(t, len(result) <= 40)
	})
}

// --- sanitizeMessage ---

func TestSanitizeMessage(t *testing.T) {
	m := Model{width: 100}

	t.Run("strips newlines", func(t *testing.T) {
		result := m.sanitizeMessage("line1\nline2\r\nline3")
		assert.NotContains(t, result, "\n")
		assert.NotContains(t, result, "\r")
	})

	t.Run("truncates long messages", func(t *testing.T) {
		longMsg := strings.Repeat("x", 200)
		result := m.sanitizeMessage(longMsg)
		assert.True(t, len(result) <= m.width-6)
	})
}

// --- setStatusMessage / hasStatusMessage ---

func TestSetStatusMessage(t *testing.T) {
	m := Model{}

	m.setStatusMessage("test message", false)
	assert.Equal(t, "test message", m.statusMessage)
	assert.False(t, m.statusMessageErr)
	assert.True(t, m.hasStatusMessage())
	assert.Len(t, m.errorLog, 1)
	assert.Equal(t, "INF", m.errorLog[0].Level)

	m.setStatusMessage("error message", true)
	assert.True(t, m.statusMessageErr)
	assert.Len(t, m.errorLog, 2)
	assert.Equal(t, "ERR", m.errorLog[1].Level)
}

func TestHasStatusMessageExpired(t *testing.T) {
	m := Model{
		statusMessage:    "expired",
		statusMessageExp: time.Now().Add(-1 * time.Second),
	}
	assert.False(t, m.hasStatusMessage())
}

// --- addLogEntry ---

func TestAddLogEntry(t *testing.T) {
	m := Model{}

	m.addLogEntry("INF", "test info")
	assert.Len(t, m.errorLog, 1)
	assert.Equal(t, "INF", m.errorLog[0].Level)
	assert.Equal(t, "test info", m.errorLog[0].Message)

	// Test cap at 500.
	for range 510 {
		m.addLogEntry("DBG", "entry")
	}
	assert.Len(t, m.errorLog, 500)
}

// --- tabLabels ---

func TestTabLabels(t *testing.T) {
	t.Run("single tab with context", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Context:      "prod",
				ResourceType: model.ResourceTypeEntry{DisplayName: "Pods"},
			},
			tabs:      []TabState{{nav: model.NavigationState{Context: "prod"}}},
			activeTab: 0,
		}
		labels := m.tabLabels()
		assert.Len(t, labels, 1)
		assert.Equal(t, "prod/Pods", labels[0])
	})

	t.Run("multiple tabs", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Context: "dev"},
			tabs: []TabState{
				{nav: model.NavigationState{Context: "prod", ResourceType: model.ResourceTypeEntry{DisplayName: "Pods"}}},
				{nav: model.NavigationState{Context: "dev"}},
			},
			activeTab: 1,
		}
		labels := m.tabLabels()
		assert.Len(t, labels, 2)
		assert.Equal(t, "prod/Pods", labels[0])
		assert.Equal(t, "dev", labels[1]) // active tab uses live model state
	})

	t.Run("empty context shows clusters", func(t *testing.T) {
		m := Model{
			nav:       model.NavigationState{},
			tabs:      []TabState{{nav: model.NavigationState{}}},
			activeTab: 0,
		}
		labels := m.tabLabels()
		assert.Equal(t, "clusters", labels[0])
	})
}

// --- getPortForwardID ---

func TestGetPortForwardID(t *testing.T) {
	m := Model{}

	t.Run("extracts ID from columns", func(t *testing.T) {
		columns := []model.KeyValue{
			{Key: "Local", Value: "8080"},
			{Key: "ID", Value: "42"},
			{Key: "Remote", Value: "80"},
		}
		assert.Equal(t, 42, m.getPortForwardID(columns))
	})

	t.Run("returns 0 when no ID column", func(t *testing.T) {
		columns := []model.KeyValue{
			{Key: "Local", Value: "8080"},
		}
		assert.Equal(t, 0, m.getPortForwardID(columns))
	})

	t.Run("returns 0 for invalid ID", func(t *testing.T) {
		columns := []model.KeyValue{
			{Key: "ID", Value: "not-a-number"},
		}
		assert.Equal(t, 0, m.getPortForwardID(columns))
	})

	t.Run("returns 0 for empty columns", func(t *testing.T) {
		assert.Equal(t, 0, m.getPortForwardID(nil))
	})
}

// --- selectedResourceKind ---

func TestSelectedResourceKind(t *testing.T) {
	t.Run("LevelResources returns resource type kind", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
			},
		}
		assert.Equal(t, "Deployment", m.selectedResourceKind())
	})

	t.Run("LevelContainers returns Container", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelContainers},
		}
		assert.Equal(t, "Container", m.selectedResourceKind())
	})

	t.Run("LevelOwned returns selected item kind", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelOwned},
			middleItems: []model.Item{
				{Name: "rs-1", Kind: "ReplicaSet"},
			},
		}
		m.setCursor(0)
		assert.Equal(t, "ReplicaSet", m.selectedResourceKind())
	})

	t.Run("LevelOwned no selection returns empty", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelOwned},
		}
		assert.Empty(t, m.selectedResourceKind())
	})

	t.Run("LevelClusters returns empty", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelClusters},
		}
		assert.Empty(t, m.selectedResourceKind())
	})
}

// --- setErrorFromErr ---

func TestSetErrorFromErr(t *testing.T) {
	m := Model{width: 100}

	m.setErrorFromErr("fetch error: ", errors.New("connection\nrefused"))

	assert.True(t, m.statusMessageErr)
	assert.Contains(t, m.statusMessage, "fetch error:")
	assert.NotContains(t, m.statusMessage, "\n")
	assert.Len(t, m.errorLog, 1)
	assert.Equal(t, "ERR", m.errorLog[0].Level)
	assert.Contains(t, m.errorLog[0].Message, "fetch error: connection refused")
}

// --- setStatusMessage log capping ---

func TestSetStatusMessageLogCapping(t *testing.T) {
	m := Model{}
	for range 210 {
		m.setStatusMessage("msg", false)
	}
	assert.Len(t, m.errorLog, 200)
}

// --- cloneCurrentTab ---

func TestCloneCurrentTab(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "prod",
		},
		leftItems:        []model.Item{{Name: "ctx-1"}},
		middleItems:      []model.Item{{Name: "pod-1"}},
		rightItems:       []model.Item{{Name: "container-1"}},
		leftItemsHistory: [][]model.Item{{{Name: "hist-item"}}},
		cursorMemory:     map[string]int{"key1": 5},
		itemCache:        map[string][]model.Item{"key2": {{Name: "cached"}}},
		selectedItems:    map[string]bool{"ns/pod": true},
		selectionAnchor:  3,
		namespace:        "default",
		filterText:       "nginx",
		expandedGroup:    "Workloads",
	}

	clone := m.cloneCurrentTab()

	assert.Equal(t, m.nav, clone.nav)
	assert.Equal(t, m.namespace, clone.namespace)
	assert.Equal(t, m.filterText, clone.filterText)
	assert.Equal(t, m.expandedGroup, clone.expandedGroup)
	assert.Len(t, clone.leftItems, 1)
	assert.Len(t, clone.middleItems, 1)
	assert.Len(t, clone.rightItems, 1)
	assert.Len(t, clone.leftItemsHistory, 1)
	assert.True(t, clone.selectedItems["ns/pod"])

	// Verify deep copy: modifying clone should not affect original.
	clone.leftItems[0].Name = "modified"
	assert.Equal(t, "ctx-1", m.leftItems[0].Name)

	clone.cursorMemory["key1"] = 99
	assert.Equal(t, 5, m.cursorMemory["key1"])

	// Visual mode is not cloned.
	assert.False(t, clone.logVisualMode)
}

// --- actionNamespace ---

func TestActionNamespace(t *testing.T) {
	t.Run("prefers action context namespace", func(t *testing.T) {
		m := Model{
			actionCtx: actionContext{namespace: "action-ns"},
			namespace: "default",
		}
		assert.Equal(t, "action-ns", m.actionNamespace())
	})

	t.Run("falls back to resolveNamespace", func(t *testing.T) {
		m := Model{
			actionCtx: actionContext{namespace: ""},
			namespace: "fallback-ns",
		}
		assert.Equal(t, "fallback-ns", m.actionNamespace())
	})

	t.Run("nav namespace overrides model namespace", func(t *testing.T) {
		m := Model{
			actionCtx: actionContext{namespace: ""},
			nav:       model.NavigationState{Namespace: "nav-ns"},
			namespace: "model-ns",
		}
		assert.Equal(t, "nav-ns", m.actionNamespace())
	})
}

// --- saveCurrentTab ---

func TestSaveCurrentTab(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "prod",
		},
		tabs:               []TabState{{}},
		activeTab:          0,
		leftItems:          []model.Item{{Name: "ctx"}},
		middleItems:        []model.Item{{Name: "pod"}},
		rightItems:         []model.Item{{Name: "container"}},
		leftItemsHistory:   [][]model.Item{{{Name: "hist"}}},
		cursorMemory:       map[string]int{"k": 1},
		itemCache:          map[string][]model.Item{"k": {{Name: "cached"}}},
		namespace:          "default",
		filterText:         "test",
		watchMode:          true,
		selectedItems:      map[string]bool{"x": true},
		selectionAnchor:    2,
		yamlCollapsed:      map[string]bool{"sec1": true},
		selectedNamespaces: map[string]bool{"ns": true},
		errorLog: []ui.ErrorLogEntry{
			{Message: "test", Level: "INF"},
		},
	}

	m.saveCurrentTab()
	tab := m.tabs[0]

	assert.Equal(t, "prod", tab.nav.Context)
	assert.Len(t, tab.leftItems, 1)
	assert.Len(t, tab.middleItems, 1)
	assert.Len(t, tab.rightItems, 1)
	assert.Len(t, tab.leftItemsHistory, 1)
	assert.Equal(t, "default", tab.namespace)
	assert.Equal(t, "test", tab.filterText)
	assert.True(t, tab.watchMode)
	assert.True(t, tab.selectedItems["x"])
	assert.Equal(t, 2, tab.selectionAnchor)

	// Verify deep copy: modifying tab fields should not affect model.
	tab.leftItems[0].Name = "modified"
	assert.Equal(t, "ctx", m.leftItems[0].Name)
}
