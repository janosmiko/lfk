package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func TestSortActionMenuItems(t *testing.T) {
	tests := []struct {
		name  string
		items []model.Item
		want  []string // expected Name order
	}{
		{
			name: "case-insensitive alphabetical by key",
			items: []model.Item{
				{Name: "Exec", Status: "s"},
				{Name: "Delete", Status: "D"},
				{Name: "Attach", Status: "A"},
			},
			want: []string{"Attach", "Delete", "Exec"},
		},
		{
			name: "same letter sorts lowercase before uppercase",
			items: []model.Item{
				{Name: "Logs", Status: "L"},
				{Name: "Tail Logs", Status: "l"},
			},
			want: []string{"Tail Logs", "Logs"},
		},
		{
			name: "empty keys sort last keeping their relative order",
			items: []model.Item{
				{Name: "Custom B", Status: ""},
				{Name: "Exec", Status: "s"},
				{Name: "Custom A", Status: ""},
				{Name: "Attach", Status: "A"},
			},
			want: []string{"Attach", "Exec", "Custom B", "Custom A"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortActionMenuItems(tt.items)
			got := make([]string, len(tt.items))
			for i, it := range tt.items {
				got[i] = it.Name
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// actionMenuKeysSorted asserts the overlay items are in hotkey order:
// case-insensitive alphabetical, lowercase first on a case tie, empty last.
func assertActionMenuSorted(t *testing.T, items []model.Item) {
	t.Helper()
	require.NotEmpty(t, items)
	for i := 1; i < len(items); i++ {
		prev, cur := items[i-1].Status, items[i].Status
		assert.LessOrEqual(t, compareActionMenuKeys(prev, cur), 0,
			"items[%d] key %q should not sort after items[%d] key %q", i-1, prev, i, cur)
	}
}

func TestOpenResourceActionMenuSortedByHotkey(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}}
	m.setCursor(0)
	rm := m.openActionMenu()
	assert.Equal(t, overlayAction, rm.overlay)
	assertActionMenuSorted(t, rm.overlayItems)
	// Pod menu: "Attach" (A) is the lowest hotkey, so it must lead.
	assert.Equal(t, "Attach", rm.overlayItems[0].Name)
}

func TestOpenBulkSelectionMenuSortedByHotkey(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{
		{Name: "pod-1", Kind: "Pod", Namespace: "default"},
		{Name: "pod-2", Kind: "Pod", Namespace: "default"},
	}
	m.selectedItems[selectionKey(m.middleItems[0])] = true
	m.selectedItems[selectionKey(m.middleItems[1])] = true
	rm := m.openActionMenu()
	assert.Equal(t, overlayAction, rm.overlay)
	assert.True(t, rm.bulkMode)
	assertActionMenuSorted(t, rm.overlayItems)
}

func TestOpenSecurityActionMenuSortedByHotkey(t *testing.T) {
	m := basePush80Model()
	m.securityIgnores = &SecurityIgnoreState{Contexts: map[string][]SecurityIgnoreRule{}}
	m.middleItems = []model.Item{{
		Name:  "finding-group",
		Kind:  "__security_finding_group__",
		Extra: "group-key",
	}}
	m.setCursor(0)
	rm, ok := m.openSecurityActionMenuIfApplicable()
	require.True(t, ok)
	assert.Equal(t, overlayAction, rm.overlay)
	assertActionMenuSorted(t, rm.overlayItems)
}
