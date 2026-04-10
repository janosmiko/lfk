package app

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
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

func TestPush2UpdateStatusMessageExpiredMsg(t *testing.T) {
	m := basePush80v2Model()
	m.setStatusMessage("temp msg", false)
	m.statusMessageExp = time.Now().Add(-1 * time.Second) // simulate genuine expiration
	result, cmd := m.Update(statusMessageExpiredMsg{})
	rm := result.(Model)
	assert.Empty(t, rm.statusMessage)
	assert.Nil(t, cmd)
}

func TestPush3ViewWithStatusMessage(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeExplorer
	m.setStatusMessage("test message", false)
	result := m.View()
	assert.NotEmpty(t, result)
}

func TestCov80PortForwardItemsEmpty(t *testing.T) {
	m := basePush80Model()
	m.portForwardMgr = k8s.NewPortForwardManager()
	items := m.portForwardItems()
	assert.Empty(t, items)
}

func TestCov80TabLabels(t *testing.T) {
	m := basePush80Model()
	m.tabs = []TabState{
		{nav: model.NavigationState{Context: "prod"}},
		{nav: model.NavigationState{Context: "dev", ResourceType: model.ResourceTypeEntry{DisplayName: "Pods"}}},
		{nav: model.NavigationState{}},
	}
	m.nav.Context = "test-ctx"
	m.activeTab = 0
	labels := m.tabLabels()
	require.Len(t, labels, 3)
	assert.Equal(t, "test-ctx", labels[0]) // active tab updated
	assert.Contains(t, labels[1], "dev/Pods")
	assert.Equal(t, "clusters", labels[2])
}

func TestCov80TabAtX(t *testing.T) {
	m := basePush80Model()
	m.tabs = []TabState{
		{nav: model.NavigationState{Context: "ctx-1"}},
		{nav: model.NavigationState{Context: "ctx-2"}},
	}
	m.nav.Context = "ctx-1"
	m.activeTab = 0
	// First tab starts at pos 1.
	assert.Equal(t, 0, m.tabAtX(1))
	// Past all tabs.
	assert.Equal(t, -1, m.tabAtX(200))
}

func TestCov80SortMiddleItemsByName(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.sortColumnName = "Name"
	m.sortAscending = true
	ui.ActiveSortableColumns = []string{"Name"}
	m.middleItems = []model.Item{
		{Name: "zebra"},
		{Name: "alpha"},
		{Name: "middle"},
	}
	m.sortMiddleItems()
	assert.Equal(t, "alpha", m.middleItems[0].Name)
	assert.Equal(t, "zebra", m.middleItems[2].Name)
}

func TestCov80SortMiddleItemsByStatus(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.sortColumnName = "Status"
	m.sortAscending = true
	ui.ActiveSortableColumns = []string{"Status"}
	m.middleItems = []model.Item{
		{Name: "a", Status: "Failed"},
		{Name: "b", Status: "Running"},
		{Name: "c", Status: "Pending"},
	}
	m.sortMiddleItems()
	assert.Equal(t, "Running", m.middleItems[0].Status)
}

func TestCov80SortMiddleItemsByAge(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.sortColumnName = "Age"
	m.sortAscending = true
	ui.ActiveSortableColumns = []string{"Age"}
	now := time.Now()
	m.middleItems = []model.Item{
		{Name: "old", CreatedAt: now.Add(-10 * time.Hour)},
		{Name: "new", CreatedAt: now.Add(-1 * time.Hour)},
		{Name: "zero"},
	}
	m.sortMiddleItems()
	// Newest first is ascending for Age.
	assert.Equal(t, "new", m.middleItems[0].Name)
}

func TestCov80SortMiddleItemsByRestarts(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.sortColumnName = "Restarts"
	m.sortAscending = true
	ui.ActiveSortableColumns = []string{"Restarts"}
	m.middleItems = []model.Item{
		{Name: "a", Restarts: "5"},
		{Name: "b", Restarts: "1"},
		{Name: "c", Restarts: "10"},
	}
	m.sortMiddleItems()
	assert.Equal(t, "b", m.middleItems[0].Name)
}

func TestCov80SortMiddleItemsByReady(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.sortColumnName = "Ready"
	m.sortAscending = true
	ui.ActiveSortableColumns = []string{"Ready"}
	m.middleItems = []model.Item{
		{Name: "a", Ready: "3/3"},
		{Name: "b", Ready: "1/3"},
		{Name: "c", Ready: "2/3"},
	}
	m.sortMiddleItems()
	assert.Equal(t, "b", m.middleItems[0].Name)
}

func TestCov80SortMiddleItemsByNamespace(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.sortColumnName = "Namespace"
	m.sortAscending = true
	ui.ActiveSortableColumns = []string{"Namespace"}
	m.middleItems = []model.Item{
		{Name: "a", Namespace: "zeta"},
		{Name: "b", Namespace: "alpha"},
	}
	m.sortMiddleItems()
	assert.Equal(t, "alpha", m.middleItems[0].Namespace)
}

func TestCov80SortMiddleItemsDescending(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.sortColumnName = "Name"
	m.sortAscending = false
	ui.ActiveSortableColumns = []string{"Name"}
	m.middleItems = []model.Item{
		{Name: "alpha"},
		{Name: "zebra"},
	}
	m.sortMiddleItems()
	assert.Equal(t, "zebra", m.middleItems[0].Name)
}

func TestCov80SortMiddleItemsAtResourceTypes(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.sortColumnName = "Name"
	m.sortAscending = true
	ui.ActiveSortableColumns = []string{"Name"}
	m.middleItems = []model.Item{{Name: "b"}, {Name: "a"}}
	m.sortMiddleItems()
	// Should not sort at LevelResourceTypes.
	assert.Equal(t, "b", m.middleItems[0].Name)
}

func TestCov80SortMiddleItemsExtraColumn(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.sortColumnName = "Image"
	m.sortAscending = true
	ui.ActiveSortableColumns = []string{"Image"}
	m.middleItems = []model.Item{
		{Name: "a", Columns: []model.KeyValue{{Key: "Image", Value: "nginx:latest"}}},
		{Name: "b", Columns: []model.KeyValue{{Key: "Image", Value: "alpine:3.18"}}},
	}
	m.sortMiddleItems()
	assert.Equal(t, "b", m.middleItems[0].Name)
}

func TestCov80ItemIndexFromDisplayLine(t *testing.T) {
	m := basePush80Model()
	m.middleItems = []model.Item{
		{Name: "pod-1"},
		{Name: "pod-2"},
		{Name: "pod-3"},
	}
	idx := m.itemIndexFromDisplayLine(0)
	assert.Equal(t, 0, idx)
	idx = m.itemIndexFromDisplayLine(1)
	assert.Equal(t, 1, idx)
	idx = m.itemIndexFromDisplayLine(100)
	assert.Equal(t, -1, idx)
}

func TestCov80ItemIndexFromDisplayLineWithCategories(t *testing.T) {
	m := basePush80Model()
	// Use LevelResources (no category filtering in visibleMiddleItems).
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{
		{Name: "pods", Category: "Core"},
		{Name: "svc", Category: "Core"},
		{Name: "deploy", Category: "Workloads"},
	}
	// Exercise the function -- it will walk through categories and count
	// separator/header lines, hitting various branches.
	// Line 0 = category header "Core", line 1 = pods (0), line 2 = svc (1),
	// line 3 = separator, line 4 = category header "Workloads", line 5 = deploy (2).
	idx := m.itemIndexFromDisplayLine(1)
	assert.Equal(t, 0, idx)
	assert.Equal(t, -1, m.itemIndexFromDisplayLine(200))
}

func TestCov80SanitizeError(t *testing.T) {
	m := basePush80Model()
	m.width = 80
	err := fmt.Errorf("line1\nline2\n\nline4")
	s := m.sanitizeError(err)
	assert.NotContains(t, s, "\n")
}

func TestCov80SanitizeErrorShortWidth(t *testing.T) {
	m := basePush80Model()
	m.width = 20
	err := fmt.Errorf("this is a very long error message that should be truncated at some point")
	s := m.sanitizeError(err)
	assert.True(t, len(s) <= 43) // maxLen = max(40, 20-20) = 40, +3 for "..."
}

func TestCov80SanitizeMessage(t *testing.T) {
	m := basePush80Model()
	m.width = 80
	s := m.sanitizeMessage("line1\nline2")
	assert.NotContains(t, s, "\n")
}

func TestCov80SanitizeMessageTruncation(t *testing.T) {
	m := basePush80Model()
	m.width = 20
	long := ""
	for range 100 {
		long += "x"
	}
	s := m.sanitizeMessage(long)
	assert.True(t, len(s) <= 43)
}

func TestCov80FullErrorMessage(t *testing.T) {
	err := fmt.Errorf("err\n  with\n  spaces  ")
	s := fullErrorMessage(err)
	assert.NotContains(t, s, "\n")
	assert.NotContains(t, s, "  ")
}

func TestCov80SetStatusMessage(t *testing.T) {
	m := basePush80Model()
	m.setStatusMessage("test info", false)
	assert.Equal(t, "test info", m.statusMessage)
	assert.False(t, m.statusMessageErr)

	m.setStatusMessage("test error", true)
	assert.True(t, m.statusMessageErr)
}

func TestCov80SetErrorFromErr(t *testing.T) {
	m := basePush80Model()
	m.setErrorFromErr("prefix: ", fmt.Errorf("something went wrong"))
	assert.True(t, m.statusMessageErr)
	assert.Contains(t, m.statusMessage, "prefix:")
}

func TestCov80HasStatusMessage(t *testing.T) {
	m := basePush80Model()
	assert.False(t, m.hasStatusMessage())
	m.setStatusMessage("test", false)
	assert.True(t, m.hasStatusMessage())
}

func TestCov80AddLogEntryOverflow(t *testing.T) {
	m := basePush80Model()
	for range 600 {
		m.addLogEntry("INF", "msg")
	}
	assert.LessOrEqual(t, len(m.errorLog), 500)
}

func TestCov80SelectedResourceKind(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType.Kind = "Pod"
	assert.Equal(t, "Pod", m.selectedResourceKind())

	m.nav.Level = model.LevelContainers
	assert.Equal(t, "Container", m.selectedResourceKind())

	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod"}}
	m.setCursor(0)
	assert.Equal(t, "Pod", m.selectedResourceKind())
}

func TestCov80EffectiveNamespace(t *testing.T) {
	m := basePush80Model()
	assert.Equal(t, "default", m.effectiveNamespace())

	m.allNamespaces = true
	assert.Equal(t, "", m.effectiveNamespace())

	m.allNamespaces = false
	m.selectedNamespaces = map[string]bool{"ns1": true, "ns2": true}
	assert.Equal(t, "", m.effectiveNamespace())

	m.selectedNamespaces = map[string]bool{"ns1": true}
	assert.Equal(t, "ns1", m.effectiveNamespace())
}

func TestCov80ActionNamespace(t *testing.T) {
	m := basePush80Model()
	m.actionCtx = actionContext{namespace: "action-ns"}
	assert.Equal(t, "action-ns", m.actionNamespace())

	m.actionCtx.namespace = ""
	assert.Equal(t, "default", m.actionNamespace())
}

func TestCov80GetPortForwardID(t *testing.T) {
	m := basePush80Model()
	cols := []model.KeyValue{
		{Key: "ID", Value: "42"},
		{Key: "Status", Value: "running"},
	}
	assert.Equal(t, 42, m.getPortForwardID(cols))

	assert.Equal(t, 0, m.getPortForwardID(nil))
	assert.Equal(t, 0, m.getPortForwardID([]model.KeyValue{{Key: "ID", Value: "bad"}}))
}

func TestCov80ParseReadyRatioInvalid(t *testing.T) {
	assert.Equal(t, float64(0), parseReadyRatio("noslash"))
	assert.Equal(t, float64(0), parseReadyRatio("0/0"))
	assert.InDelta(t, 0.5, parseReadyRatio("1/2"), 0.01)
}

func TestCov80CompareNumeric(t *testing.T) {
	assert.True(t, compareNumeric("1", "2"))
	assert.False(t, compareNumeric("10", "2"))
	assert.True(t, compareNumeric("abc", "1")) // "abc" parses as 0
}

func TestCov80StatusPriority(t *testing.T) {
	assert.Equal(t, 0, statusPriority("Running"))
	assert.Equal(t, 1, statusPriority("Pending"))
	assert.Equal(t, 2, statusPriority("Failed"))
	assert.Equal(t, 3, statusPriority("Unknown"))
}

func TestCov80SortModeName(t *testing.T) {
	m := basePush80Model()
	m.sortColumnName = ""
	assert.Contains(t, m.sortModeName(), "Name")

	m.sortColumnName = "Status"
	m.sortAscending = true
	assert.Contains(t, m.sortModeName(), "Status")
	assert.Contains(t, m.sortModeName(), "\u2191")

	m.sortAscending = false
	assert.Contains(t, m.sortModeName(), "\u2193")
}

func TestCov80GetColumnValue(t *testing.T) {
	item := model.Item{
		Columns: []model.KeyValue{
			{Key: "Image", Value: "nginx:latest"},
			{Key: "Node", Value: "worker-1"},
		},
	}
	assert.Equal(t, "nginx:latest", getColumnValue(item, "Image"))
	assert.Equal(t, "", getColumnValue(item, "NonExistent"))
}

func TestCov80CopyMapStringIntNil(t *testing.T) {
	c := copyMapStringInt(nil)
	assert.NotNil(t, c)
	assert.Empty(t, c)
}

func TestCov80CopyMapStringIntNonNil(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}
	c := copyMapStringInt(m)
	assert.Equal(t, 1, c["a"])
	c["a"] = 99
	assert.Equal(t, 1, m["a"]) // original unchanged
}

func TestCov80CopyMapStringBoolNil(t *testing.T) {
	c := copyMapStringBool(nil)
	assert.NotNil(t, c)
	assert.Empty(t, c)
}

func TestCov80CopyItemCacheNil(t *testing.T) {
	c := copyItemCache(nil)
	assert.NotNil(t, c)
	assert.Empty(t, c)
}

func TestCov80CopyItemCacheNonNil(t *testing.T) {
	m := map[string][]model.Item{
		"key": {{Name: "a"}, {Name: "b"}},
	}
	c := copyItemCache(m)
	assert.Len(t, c["key"], 2)
	c["key"][0].Name = "changed"
	assert.Equal(t, "a", m["key"][0].Name) // original unchanged
}

func TestCovTabAtXSingleTab(t *testing.T) {
	m := baseModelActions()
	m.tabs = []TabState{{}}
	// With a single tab, tabLabels returns a single label.
	// tabAtX should find tab 0 at x=1 area.
	tab := m.tabAtX(1)
	assert.GreaterOrEqual(t, tab, 0)
}

func TestCovTabAtXNegative(t *testing.T) {
	m := baseModelActions()
	m.tabs = []TabState{{}}
	tab := m.tabAtX(200)
	assert.Equal(t, -1, tab)
}

func TestCovTabAtXMultipleTabs(t *testing.T) {
	m := baseModelActions()
	m.tabs = []TabState{
		{nav: model.NavigationState{Context: "ctx1"}},
		{nav: model.NavigationState{Context: "ctx2"}},
	}
	// First tab starts at x=1. Tabwidth = label + 2
	tab := m.tabAtX(1)
	assert.GreaterOrEqual(t, tab, 0)
}

func TestCovTabAtXOutOfBounds(t *testing.T) {
	m := baseModelActions()
	m.tabs = []TabState{{}, {}}
	tab := m.tabAtX(200)
	assert.Equal(t, -1, tab)
}

func TestCovJumpToSlotNotFound(t *testing.T) {
	m := baseModelCov()
	m.bookmarks = nil
	ret, cmd := m.jumpToSlot("a")
	result := ret.(Model)
	assert.True(t, result.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovPortForwardItemsNoManager(t *testing.T) {
	m := baseModelCov()
	m.portForwardMgr = k8s.NewPortForwardManager()
	items := m.portForwardItems()
	assert.Empty(t, items)
}

func TestCovSelectedResourceKind(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}

	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	assert.Equal(t, "Pod", m.selectedResourceKind())

	m.nav.Level = model.LevelContainers
	assert.Equal(t, "Container", m.selectedResourceKind())

	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod"}}
	m.setCursor(0)
	assert.Equal(t, "Pod", m.selectedResourceKind())

	m.nav.Level = model.LevelClusters
	assert.Empty(t, m.selectedResourceKind())
}

func TestCovEffectiveNamespace(t *testing.T) {
	m := baseModelCov()
	m.namespace = "default"
	assert.Equal(t, "default", m.effectiveNamespace())

	m.allNamespaces = true
	assert.Empty(t, m.effectiveNamespace())

	m.allNamespaces = false
	m.selectedNamespaces = map[string]bool{"ns1": true, "ns2": true}
	assert.Empty(t, m.effectiveNamespace())

	m.selectedNamespaces = map[string]bool{"ns1": true}
	assert.Equal(t, "ns1", m.effectiveNamespace())
}

func TestCovSortModeName(t *testing.T) {
	m := baseModelCov()
	assert.Contains(t, m.sortModeName(), "Name")

	m.sortColumnName = "Age"
	m.sortAscending = true
	assert.Contains(t, m.sortModeName(), "Age")
	assert.Contains(t, m.sortModeName(), "\u2191")

	m.sortAscending = false
	assert.Contains(t, m.sortModeName(), "\u2193")
}

func TestCovSanitizeError(t *testing.T) {
	m := baseModelCov()
	m.width = 80

	err := assert.AnError
	result := m.sanitizeError(err)
	assert.NotEmpty(t, result)
}

func TestCovSanitizeMessage(t *testing.T) {
	m := baseModelCov()
	m.width = 80

	assert.Equal(t, "hello world", m.sanitizeMessage("hello\nworld"))

	// Long message truncation.
	m.width = 20
	long := "this is a very long message that exceeds the width"
	result := m.sanitizeMessage(long)
	assert.True(t, len(result) <= 43) // maxLen=40 with 3 chars for "..."
}

func TestCovSetStatusMessage(t *testing.T) {
	m := baseModelCov()
	m.setStatusMessage("test message", false)
	assert.Equal(t, "test message", m.statusMessage)
	assert.False(t, m.statusMessageErr)
	assert.Len(t, m.errorLog, 1)

	m.setStatusMessage("error message", true)
	assert.Equal(t, "error message", m.statusMessage)
	assert.True(t, m.statusMessageErr)
	assert.Len(t, m.errorLog, 2)
}

func TestCovSetErrorFromErr(t *testing.T) {
	m := baseModelCov()
	m.width = 80
	m.setErrorFromErr("Failed: ", assert.AnError)
	assert.True(t, m.statusMessageErr)
	assert.Contains(t, m.statusMessage, "Failed: ")
	assert.Len(t, m.errorLog, 1)
}

func TestCovHasStatusMessage(t *testing.T) {
	m := baseModelCov()
	assert.False(t, m.hasStatusMessage())

	m.setStatusMessage("test", false)
	assert.True(t, m.hasStatusMessage())
}

func TestCovFullErrorMessage(t *testing.T) {
	result := fullErrorMessage(assert.AnError)
	assert.NotEmpty(t, result)
	assert.NotContains(t, result, "\n")
}

func TestCovAddLogEntry(t *testing.T) {
	m := baseModelCov()
	m.addLogEntry("INF", "test log entry")
	assert.Len(t, m.errorLog, 1)
	assert.Equal(t, "INF", m.errorLog[0].Level)
}

func TestCovTabLabels(t *testing.T) {
	m := baseModelCov()
	m.tabs = []TabState{{nav: model.NavigationState{Context: "prod", ResourceType: model.ResourceTypeEntry{DisplayName: "Pods"}}}}
	m.nav.Context = "prod"
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods"}
	labels := m.tabLabels()
	assert.Len(t, labels, 1)
	assert.Contains(t, labels[0], "prod")
}

func TestCovTabLabelsEmpty(t *testing.T) {
	m := baseModelCov()
	m.tabs = []TabState{{}}
	labels := m.tabLabels()
	assert.Equal(t, "clusters", labels[0])
}

func TestCovGetPortForwardID(t *testing.T) {
	m := baseModelCov()
	cols := []model.KeyValue{
		{Key: "ID", Value: "42"},
		{Key: "Local", Value: "8080"},
	}
	assert.Equal(t, 42, m.getPortForwardID(cols))

	// No ID column.
	assert.Equal(t, 0, m.getPortForwardID([]model.KeyValue{{Key: "Local", Value: "8080"}}))

	// Invalid ID.
	assert.Equal(t, 0, m.getPortForwardID([]model.KeyValue{{Key: "ID", Value: "abc"}}))
}

func TestCovCompareReady(t *testing.T) {
	assert.True(t, compareReady("0/1", "1/1"))
	assert.False(t, compareReady("1/1", "0/1"))
	assert.False(t, compareReady("1/1", "1/1"))
}

func TestCovParseReadyRatio(t *testing.T) {
	assert.InDelta(t, 0.5, parseReadyRatio("1/2"), 0.01)
	assert.InDelta(t, 0.0, parseReadyRatio("0/1"), 0.01)
	assert.InDelta(t, 0.0, parseReadyRatio("0/0"), 0.01)
	assert.InDelta(t, 0.0, parseReadyRatio("invalid"), 0.01)
}

func TestCovCompareNumeric(t *testing.T) {
	assert.True(t, compareNumeric("1", "2"))
	assert.False(t, compareNumeric("5", "3"))
	assert.False(t, compareNumeric("abc", "def"))
}

func TestCovStatusPriority(t *testing.T) {
	assert.Equal(t, 0, statusPriority("Running"))
	assert.Equal(t, 0, statusPriority("Active"))
	assert.Equal(t, 1, statusPriority("Pending"))
	assert.Equal(t, 2, statusPriority("Failed"))
	assert.Equal(t, 2, statusPriority("CrashLoopBackOff"))
	assert.Equal(t, 3, statusPriority("Unknown"))
}

func TestCovGetColumnValue(t *testing.T) {
	item := model.Item{
		Columns: []model.KeyValue{{Key: "IP", Value: "10.0.0.1"}, {Key: "Port", Value: "8080"}},
	}
	assert.Equal(t, "10.0.0.1", getColumnValue(item, "IP"))
	assert.Equal(t, "8080", getColumnValue(item, "Port"))
	assert.Empty(t, getColumnValue(item, "Missing"))
}

func TestCovSortMiddleItems(t *testing.T) {
	// Save and restore global state.
	origCols := ui.ActiveSortableColumns
	t.Cleanup(func() { ui.ActiveSortableColumns = origCols })
	ui.ActiveSortableColumns = []string{"Name"}

	m := Model{
		nav:    model.NavigationState{Level: model.LevelResources},
		tabs:   []TabState{{}},
		execMu: &sync.Mutex{},
	}

	m.middleItems = []model.Item{
		{Name: "charlie"},
		{Name: "alpha"},
		{Name: "bravo"},
	}
	m.sortColumnName = "Name"
	m.sortAscending = true

	m.sortMiddleItems()
	assert.Equal(t, "alpha", m.middleItems[0].Name)
	assert.Equal(t, "bravo", m.middleItems[1].Name)
	assert.Equal(t, "charlie", m.middleItems[2].Name)
}

func TestCovSortMiddleItemsSkipsResourceTypes(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "c"},
		{Name: "a"},
		{Name: "b"},
	}
	m.sortColumnName = "Name"
	m.sortMiddleItems()
	// Should not be sorted.
	assert.Equal(t, "c", m.middleItems[0].Name)
}

func TestFinalUpdateStatusClearMsg(t *testing.T) {
	m := baseFinalModel()
	m.setStatusMessage("test message", false)
	m.statusMessageExp = time.Now().Add(-1 * time.Second) // simulate genuine expiration
	result, _ := m.Update(statusMessageExpiredMsg{})
	rm := result.(Model)
	assert.Empty(t, rm.statusMessage)
}

func TestFinalJumpToSlotNotFound(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.jumpToSlot("z")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "not set")
}

func TestCovCompareResourceValues(t *testing.T) {
	result := compareResourceValues("100m", "200m", "CPU")
	assert.True(t, result) // 100m < 200m
}

func TestCovCompareResourceValuesCPUPrefix(t *testing.T) {
	result := compareResourceValues("500m", "1", "CPU(%)")
	assert.True(t, result) // 500m < 1
}

func TestCovCompareResourceValuesMemory(t *testing.T) {
	result := compareResourceValues("100Mi", "200Mi", "Memory")
	assert.True(t, result)
}

func TestCovCompareResourceValuesEqual(t *testing.T) {
	result := compareResourceValues("100m", "100m", "CPU")
	assert.False(t, result) // equal, not less
}

func TestSortMiddleItemsResourceQuantities(t *testing.T) {
	origCols := ui.ActiveSortableColumns
	t.Cleanup(func() { ui.ActiveSortableColumns = origCols })
	ui.ActiveSortableColumns = []string{"Capacity"}

	m := Model{
		nav:    model.NavigationState{Level: model.LevelResources},
		tabs:   []TabState{{}},
		execMu: &sync.Mutex{},
	}

	m.middleItems = []model.Item{
		{Name: "pv-a", Columns: []model.KeyValue{{Key: "Capacity", Value: "10Gi"}}},
		{Name: "pv-b", Columns: []model.KeyValue{{Key: "Capacity", Value: "50Gi"}}},
		{Name: "pv-c", Columns: []model.KeyValue{{Key: "Capacity", Value: "5Gi"}}},
	}
	m.sortColumnName = "Capacity"
	m.sortAscending = true

	m.sortMiddleItems()
	assert.Equal(t, "pv-c", m.middleItems[0].Name) // 5Gi
	assert.Equal(t, "pv-a", m.middleItems[1].Name) // 10Gi
	assert.Equal(t, "pv-b", m.middleItems[2].Name) // 50Gi
}

func TestSortMiddleItemsPlainNumbers(t *testing.T) {
	origCols := ui.ActiveSortableColumns
	t.Cleanup(func() { ui.ActiveSortableColumns = origCols })
	ui.ActiveSortableColumns = []string{"Replicas"}

	m := Model{
		nav:    model.NavigationState{Level: model.LevelResources},
		tabs:   []TabState{{}},
		execMu: &sync.Mutex{},
	}

	m.middleItems = []model.Item{
		{Name: "d-a", Columns: []model.KeyValue{{Key: "Replicas", Value: "10"}}},
		{Name: "d-b", Columns: []model.KeyValue{{Key: "Replicas", Value: "2"}}},
		{Name: "d-c", Columns: []model.KeyValue{{Key: "Replicas", Value: "100"}}},
	}
	m.sortColumnName = "Replicas"
	m.sortAscending = true

	m.sortMiddleItems()
	assert.Equal(t, "d-b", m.middleItems[0].Name) // 2
	assert.Equal(t, "d-a", m.middleItems[1].Name) // 10
	assert.Equal(t, "d-c", m.middleItems[2].Name) // 100
}

func TestSortMiddleItemsMixedMiGi(t *testing.T) {
	origCols := ui.ActiveSortableColumns
	t.Cleanup(func() { ui.ActiveSortableColumns = origCols })
	ui.ActiveSortableColumns = []string{"Storage"}

	m := Model{
		nav:    model.NavigationState{Level: model.LevelResources},
		tabs:   []TabState{{}},
		execMu: &sync.Mutex{},
	}

	m.middleItems = []model.Item{
		{Name: "pv-a", Columns: []model.KeyValue{{Key: "Storage", Value: "512Mi"}}},
		{Name: "pv-b", Columns: []model.KeyValue{{Key: "Storage", Value: "2Gi"}}},
		{Name: "pv-c", Columns: []model.KeyValue{{Key: "Storage", Value: "100Mi"}}},
	}
	m.sortColumnName = "Storage"
	m.sortAscending = true

	m.sortMiddleItems()
	assert.Equal(t, "pv-c", m.middleItems[0].Name) // 100Mi
	assert.Equal(t, "pv-a", m.middleItems[1].Name) // 512Mi
	assert.Equal(t, "pv-b", m.middleItems[2].Name) // 2Gi
}

func TestCovErrorLogVisibleCount(t *testing.T) {
	m := baseModelCov()
	m.addLogEntry("INF", "log1")
	m.addLogEntry("ERR", "log2")

	visible, maxVisible, maxScroll := m.errorLogVisibleCount()
	assert.Equal(t, 2, visible)
	assert.Greater(t, maxVisible, 0)
	assert.GreaterOrEqual(t, maxScroll, 0)
}

func TestCovErrorLogVisibleCountFullscreen(t *testing.T) {
	m := baseModelCov()
	m.errorLogFullscreen = true
	m.addLogEntry("INF", "log1")

	visible, maxVisible, _ := m.errorLogVisibleCount()
	assert.Equal(t, 1, visible)
	assert.Greater(t, maxVisible, 0)
}

func TestCovUpdateStatusClearMsg(t *testing.T) {
	m := baseModelNav()
	m.setStatusMessage("test", false)
	m.statusMessageExp = time.Now().Add(-1 * time.Second) // simulate genuine expiration
	result, _ := m.Update(statusMessageExpiredMsg{})
	rm := result.(Model)
	assert.False(t, rm.hasStatusMessage())
}

func TestCovUpdateActionResult(t *testing.T) {
	m := baseModelUpdate()
	m.loading = true
	result, cmd := m.Update(actionResultMsg{
		message: "Deleted pod/test-pod",
	})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateActionResultError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(actionResultMsg{
		err: errors.New("delete failed"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateStatusClear(t *testing.T) {
	m := baseModelUpdate()
	m.setStatusMessage("test message", false)
	m.statusMessageExp = time.Now().Add(-1 * time.Second) // simulate genuine expiration
	result, _ := m.Update(statusMessageExpiredMsg{})
	rm := result.(Model)
	assert.False(t, rm.hasStatusMessage())
}

func TestCovUpdateDiffLoadedError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(diffLoadedMsg{
		err: errors.New("helm not found"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
}

func TestCovUpdatePodsForActionError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(podSelectMsg{
		err: errors.New("not found"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateContainersForActionError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(containerSelectMsg{
		err: errors.New("forbidden"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateRollbackDone(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(rollbackDoneMsg{})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateRollbackDoneError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(rollbackDoneMsg{err: errors.New("failed")})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateHelmRollbackDone(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(helmRollbackDoneMsg{})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateHelmRollbackDoneError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(helmRollbackDoneMsg{err: errors.New("failed")})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateTriggerCronJob(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(triggerCronJobMsg{jobName: "manual-job-1"})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateTriggerCronJobError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(triggerCronJobMsg{err: errors.New("quota exceeded")})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdatePortForwardStarted(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(portForwardStartedMsg{localPort: "9090", remotePort: "80"})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdatePortForwardStartedError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(portForwardStartedMsg{err: errors.New("bind failed")})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateLogSaveAll(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(logSaveAllMsg{path: "/tmp/test.log"})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateLogSaveAllError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(logSaveAllMsg{err: errors.New("disk full")})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateExplainLoadedError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(explainLoadedMsg{
		err: errors.New("not found"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
}

func TestCovUpdateExplainRecursiveError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(explainRecursiveMsg{
		err: errors.New("kubectl not found"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
}

func TestCovUpdateFinalizerSearchError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(finalizerSearchResultMsg{
		err: errors.New("timeout"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
}

func TestCovUpdateEventTimelineError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(eventTimelineMsg{
		err: errors.New("forbidden"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}
