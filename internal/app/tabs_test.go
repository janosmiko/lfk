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

// TestSortMiddleItemsDeterministicForEqualPrimaryKeys is a regression guard
// for the watch-mode flicker bug: items with identical primary sort keys
// (e.g. a Helm release "traefik" deployed to two namespaces) were
// previously left in arbitrary order because the comparator returned
// "equal" and the only stabilizer was sort.SliceStable relying on input
// order — but k8s API refreshes can return items in different orders each
// call, so the list would visibly jump on every watch tick.
//
// Fix: make the comparator total by adding a (Namespace, Name, Kind)
// tiebreaker chain that is always ascending, regardless of the primary
// sort direction, so identical primary keys have a deterministic order
// across refreshes.
func TestSortMiddleItemsDeterministicForEqualPrimaryKeys(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status", "Namespace"}
	defer func() { ui.ActiveSortableColumns = nil }()

	// Two items with identical Name and different Namespace. The sort must
	// produce the same result regardless of which order they arrive in.
	aFirst := []model.Item{
		{Name: "traefik", Namespace: "prod"},
		{Name: "traefik", Namespace: "dev"},
	}
	bFirst := []model.Item{
		{Name: "traefik", Namespace: "dev"},
		{Name: "traefik", Namespace: "prod"},
	}

	sortCopy := func(items []model.Item) []model.Item {
		cp := append([]model.Item(nil), items...)
		m := Model{
			nav:            model.NavigationState{Level: model.LevelResources},
			sortColumnName: "Name", sortAscending: true,
			middleItems: cp,
		}
		m.sortMiddleItems()
		return m.middleItems
	}

	gotA := sortCopy(aFirst)
	gotB := sortCopy(bFirst)

	require.Len(t, gotA, 2)
	require.Len(t, gotB, 2)
	assert.Equal(t, "dev", gotA[0].Namespace,
		"input order A: tiebreaker must put dev before prod")
	assert.Equal(t, "prod", gotA[1].Namespace)
	assert.Equal(t, "dev", gotB[0].Namespace,
		"input order B: tiebreaker must put dev before prod")
	assert.Equal(t, "prod", gotB[1].Namespace)
	assert.Equal(t, gotA, gotB,
		"sort output must be deterministic regardless of input order")
}

// TestSortMiddleItemsTiebreakerIgnoresDescFlag verifies that when the
// primary sort is descending, the tiebreaker chain still runs ascending.
// Flipping the tiebreaker with the primary direction would re-introduce
// flicker when sorting in reverse.
func TestSortMiddleItemsTiebreakerIgnoresDescFlag(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status", "Namespace"}
	defer func() { ui.ActiveSortableColumns = nil }()

	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources},
		sortColumnName: "Name", sortAscending: false,
		middleItems: []model.Item{
			{Name: "traefik", Namespace: "prod"},
			{Name: "alpha", Namespace: "dev"},
			{Name: "traefik", Namespace: "dev"},
		},
	}
	m.sortMiddleItems()

	require.Len(t, m.middleItems, 3)
	// Name desc: traefik rows first (two), then alpha. Within the two
	// traefik rows, namespace ASC tiebreaker means dev before prod.
	assert.Equal(t, "traefik", m.middleItems[0].Name)
	assert.Equal(t, "dev", m.middleItems[0].Namespace)
	assert.Equal(t, "traefik", m.middleItems[1].Name)
	assert.Equal(t, "prod", m.middleItems[1].Namespace)
	assert.Equal(t, "alpha", m.middleItems[2].Name)
}

// TestSortMiddleItemsByColumnValueDeterministic is the same guard for
// sorting by an arbitrary extra column (e.g. a Revision column on a Helm
// release). Two releases with the same revision number should still have
// a deterministic namespace-based order.
func TestSortMiddleItemsByColumnValueDeterministic(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name", "Revision"}
	defer func() { ui.ActiveSortableColumns = nil }()

	sortCopy := func(items []model.Item) []model.Item {
		cp := append([]model.Item(nil), items...)
		m := Model{
			nav:            model.NavigationState{Level: model.LevelResources},
			sortColumnName: "Revision", sortAscending: true,
			middleItems: cp,
		}
		m.sortMiddleItems()
		return m.middleItems
	}

	rows := []model.Item{
		{Name: "traefik", Namespace: "prod", Columns: []model.KeyValue{{Key: "Revision", Value: "5"}}},
		{Name: "traefik", Namespace: "dev", Columns: []model.KeyValue{{Key: "Revision", Value: "5"}}},
	}
	gotA := sortCopy(rows)

	// Reverse input order.
	rowsB := []model.Item{rows[1], rows[0]}
	gotB := sortCopy(rowsB)

	assert.Equal(t, "dev", gotA[0].Namespace)
	assert.Equal(t, gotA, gotB,
		"column-value sort output must be deterministic regardless of input order")
}

// TestSortMiddleItemsPrimaryAwareTiebreakerName exercises the
// primary-aware tiebreaker chain when the primary column is Name. With
// Name as primary, the chain becomes (Namespace, Age, Kind, Extra) —
// Name itself is skipped because it cannot discriminate rows with equal
// primary keys. The test fixes Name to a single value across all rows so
// the tiebreaker fully determines the output order, and varies Namespace
// and Age to exercise both levels of the fallback.
func TestSortMiddleItemsPrimaryAwareTiebreakerName(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name", "Age", "Namespace", "Status"}
	defer func() { ui.ActiveSortableColumns = nil }()

	now := time.Now()
	// All items share Name="traefik". Order after sort must be:
	//   1. dev / newest (Namespace ASC first, Age newest-first within ns)
	//   2. dev / older
	//   3. prod / newest
	//   4. prod / older
	items := []model.Item{
		{Name: "traefik", Namespace: "prod", CreatedAt: now.Add(-10 * time.Hour)},
		{Name: "traefik", Namespace: "dev", CreatedAt: now.Add(-2 * time.Hour)},
		{Name: "traefik", Namespace: "prod", CreatedAt: now.Add(-1 * time.Hour)},
		{Name: "traefik", Namespace: "dev", CreatedAt: now.Add(-5 * time.Hour)},
	}
	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources},
		sortColumnName: "Name", sortAscending: true,
		middleItems: items,
	}
	m.sortMiddleItems()

	require.Len(t, m.middleItems, 4)
	assert.Equal(t, "dev", m.middleItems[0].Namespace, "Namespace is secondary")
	assert.Equal(t, 2*time.Hour, now.Sub(m.middleItems[0].CreatedAt), "Age (newest) is tertiary within dev")
	assert.Equal(t, "dev", m.middleItems[1].Namespace)
	assert.Equal(t, 5*time.Hour, now.Sub(m.middleItems[1].CreatedAt))
	assert.Equal(t, "prod", m.middleItems[2].Namespace)
	assert.Equal(t, 1*time.Hour, now.Sub(m.middleItems[2].CreatedAt), "Age (newest) is tertiary within prod")
	assert.Equal(t, "prod", m.middleItems[3].Namespace)
	assert.Equal(t, 10*time.Hour, now.Sub(m.middleItems[3].CreatedAt))
}

// TestSortMiddleItemsPrimaryAwareTiebreakerNamespace exercises the
// chain when the primary column is Namespace: (Name, Age, Kind, Extra).
// Namespace is skipped from the tiebreaker because the primary already
// discriminates it. Rows with equal Namespace must fall through to Name,
// then to Age.
func TestSortMiddleItemsPrimaryAwareTiebreakerNamespace(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name", "Age", "Namespace", "Status"}
	defer func() { ui.ActiveSortableColumns = nil }()

	now := time.Now()
	// All items share Namespace="default". Expected order:
	//   1. alpha (Name ASC first)
	//   2. bravo / newest (Age within equal Name)
	//   3. bravo / older
	items := []model.Item{
		{Name: "bravo", Namespace: "default", CreatedAt: now.Add(-5 * time.Hour)},
		{Name: "alpha", Namespace: "default", CreatedAt: now.Add(-10 * time.Hour)},
		{Name: "bravo", Namespace: "default", CreatedAt: now.Add(-1 * time.Hour)},
	}
	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources},
		sortColumnName: "Namespace", sortAscending: true,
		middleItems: items,
	}
	m.sortMiddleItems()

	require.Len(t, m.middleItems, 3)
	assert.Equal(t, "alpha", m.middleItems[0].Name, "Name is secondary when Namespace is primary")
	assert.Equal(t, "bravo", m.middleItems[1].Name)
	assert.Equal(t, 1*time.Hour, now.Sub(m.middleItems[1].CreatedAt), "Age (newest) is tertiary within equal Name")
	assert.Equal(t, "bravo", m.middleItems[2].Name)
	assert.Equal(t, 5*time.Hour, now.Sub(m.middleItems[2].CreatedAt))
}

// TestSortMiddleItemsPrimaryAwareTiebreakerAge exercises the chain when
// the primary column is Age: (Name, Namespace, Kind, Extra). Age is
// skipped. Rows with identical CreatedAt must fall through to Name, then
// Namespace.
func TestSortMiddleItemsPrimaryAwareTiebreakerAge(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name", "Age", "Namespace", "Status"}
	defer func() { ui.ActiveSortableColumns = nil }()

	sharedTime := time.Now().Add(-3 * time.Hour)
	items := []model.Item{
		{Name: "bravo", Namespace: "prod", CreatedAt: sharedTime},
		{Name: "alpha", Namespace: "prod", CreatedAt: sharedTime},
		{Name: "bravo", Namespace: "dev", CreatedAt: sharedTime},
	}
	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources},
		sortColumnName: "Age", sortAscending: true,
		middleItems: items,
	}
	m.sortMiddleItems()

	require.Len(t, m.middleItems, 3)
	assert.Equal(t, "alpha", m.middleItems[0].Name, "Name is secondary when Age is primary")
	assert.Equal(t, "bravo", m.middleItems[1].Name)
	assert.Equal(t, "dev", m.middleItems[1].Namespace, "Namespace is tertiary within equal Name")
	assert.Equal(t, "bravo", m.middleItems[2].Name)
	assert.Equal(t, "prod", m.middleItems[2].Namespace)
}

// TestSortMiddleItemsPrimaryAwareTiebreakerCustomColumn exercises the
// chain when the primary column is not one of (Name, Namespace, Age) —
// in this case the full identity triple participates in the tiebreaker:
// (Name, Namespace, Age, Kind, Extra). Rows with identical column values
// must fall through to Name, then Namespace, then Age.
func TestSortMiddleItemsPrimaryAwareTiebreakerCustomColumn(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name", "Age", "Namespace", "CPU"}
	defer func() { ui.ActiveSortableColumns = nil }()

	now := time.Now()
	// All items share CPU="100m" so the tiebreaker fully determines order.
	row := func(name, ns string, age time.Duration) model.Item {
		return model.Item{
			Name: name, Namespace: ns, CreatedAt: now.Add(-age),
			Columns: []model.KeyValue{{Key: "CPU", Value: "100m"}},
		}
	}
	items := []model.Item{
		row("bravo", "prod", 1*time.Hour),
		row("alpha", "prod", 5*time.Hour),
		row("alpha", "dev", 10*time.Hour),
		row("alpha", "dev", 2*time.Hour),
	}
	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources},
		sortColumnName: "CPU", sortAscending: true,
		middleItems: items,
	}
	m.sortMiddleItems()

	require.Len(t, m.middleItems, 4)
	// Expected:
	//   alpha / dev / 2h  (Name, Namespace, Age newest)
	//   alpha / dev / 10h
	//   alpha / prod / 5h
	//   bravo / prod / 1h
	assert.Equal(t, "alpha", m.middleItems[0].Name)
	assert.Equal(t, "dev", m.middleItems[0].Namespace)
	assert.Equal(t, 2*time.Hour, now.Sub(m.middleItems[0].CreatedAt))
	assert.Equal(t, "alpha", m.middleItems[1].Name)
	assert.Equal(t, "dev", m.middleItems[1].Namespace)
	assert.Equal(t, 10*time.Hour, now.Sub(m.middleItems[1].CreatedAt))
	assert.Equal(t, "alpha", m.middleItems[2].Name)
	assert.Equal(t, "prod", m.middleItems[2].Namespace)
	assert.Equal(t, "bravo", m.middleItems[3].Name)
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

	t.Run("LevelOwned shows resource name", func(t *testing.T) {
		// Navigated into a Deployment to see its pods. The tab should
		// expose the deployment name so users can tell tabs apart.
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelOwned,
				Context:      "prod",
				ResourceType: model.ResourceTypeEntry{DisplayName: "Deployments"},
				ResourceName: "web-server",
			},
			tabs:      []TabState{{nav: model.NavigationState{}}},
			activeTab: 0,
		}
		labels := m.tabLabels()
		assert.Equal(t, "prod/Deployments/web-server", labels[0])
	})

	t.Run("LevelContainers shows resource and owned name", func(t *testing.T) {
		// Navigated all the way down: deployment -> pod -> containers.
		// The tab should expose both names so the user can tell which pod
		// of which deployment they're looking at.
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelContainers,
				Context:      "prod",
				ResourceType: model.ResourceTypeEntry{DisplayName: "Deployments"},
				ResourceName: "web-server",
				OwnedName:    "web-server-abc-xyz",
			},
			tabs:      []TabState{{nav: model.NavigationState{}}},
			activeTab: 0,
		}
		labels := m.tabLabels()
		assert.Equal(t, "prod/Deployments/web-server/web-server-abc-xyz", labels[0])
	})

	t.Run("discovered resource type without DisplayName uses metadata", func(t *testing.T) {
		// Resource types coming from API discovery have an empty DisplayName
		// (per model.DisplayNameFor). The label must still surface a friendly
		// name by going through the metadata fallback chain.
		m := Model{
			nav: model.NavigationState{
				Level:   model.LevelResources,
				Context: "prod",
				ResourceType: model.ResourceTypeEntry{
					// DisplayName intentionally empty.
					Kind:       "Pod",
					APIGroup:   "",
					APIVersion: "v1",
					Resource:   "pods",
				},
			},
			tabs:      []TabState{{nav: model.NavigationState{}}},
			activeTab: 0,
		}
		labels := m.tabLabels()
		assert.Equal(t, "prod/Pods", labels[0])
	})

	t.Run("discovered resource falls back to Kind when no metadata", func(t *testing.T) {
		// CRD-style resource: no DisplayName, no built-in metadata entry,
		// only Kind. The label should still show the Kind so the tab is
		// distinguishable.
		m := Model{
			nav: model.NavigationState{
				Level:   model.LevelResources,
				Context: "prod",
				ResourceType: model.ResourceTypeEntry{
					Kind:       "MyCustomResource",
					APIGroup:   "example.com",
					APIVersion: "v1",
					Resource:   "mycustomresources",
				},
			},
			tabs:      []TabState{{nav: model.NavigationState{}}},
			activeTab: 0,
		}
		labels := m.tabLabels()
		assert.Equal(t, "prod/MyCustomResource", labels[0])
	})

	t.Run("Pod at LevelContainers does not duplicate name", func(t *testing.T) {
		// navigateChildResource sets both ResourceName AND OwnedName to the
		// same value when entering a Pod (so the containers view knows its
		// parent). The label must not show "ctx/Pods/my-pod/my-pod".
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelContainers,
				Context:      "prod",
				ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"},
				ResourceName: "web-7d8c-abc",
				OwnedName:    "web-7d8c-abc",
			},
			tabs:      []TabState{{nav: model.NavigationState{}}},
			activeTab: 0,
		}
		labels := m.tabLabels()
		assert.Equal(t, "prod/Pods/web-7d8c-abc", labels[0],
			"ResourceName == OwnedName should appear only once")
	})

	t.Run("inactive tab also reflects its saved depth", func(t *testing.T) {
		// Inactive tabs are rendered from saved TabState, not the live model.
		// They should still show the deeper navigation if that's where the
		// tab was last left.
		m := Model{
			nav: model.NavigationState{Context: "dev"},
			tabs: []TabState{
				{nav: model.NavigationState{
					Level:        model.LevelOwned,
					Context:      "prod",
					ResourceType: model.ResourceTypeEntry{DisplayName: "StatefulSets"},
					ResourceName: "db",
				}},
				{nav: model.NavigationState{Context: "dev"}},
			},
			activeTab: 1,
		}
		labels := m.tabLabels()
		assert.Equal(t, "prod/StatefulSets/db", labels[0])
		assert.Equal(t, "dev", labels[1])
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
	// basePush80Model sets m.nav.ResourceType.Kind = "Pod" / Resource = "pods"
	// (no DisplayName), which model.DisplayNameFor resolves to "Pods" via the
	// built-in metadata table.
	assert.Equal(t, "test-ctx/Pods", labels[0])
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
	var long strings.Builder
	for range 100 {
		long.WriteString("x")
	}
	s := m.sanitizeMessage(long.String())
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
