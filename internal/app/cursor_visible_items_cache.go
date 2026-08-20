package app

import (
	"unsafe"

	"github.com/janosmiko/lfk/internal/model"
)

// visibleMiddleItemsKey is the memo key for computeVisibleMiddleItems: every
// field that changes its output.
type visibleMiddleItemsKey struct {
	itemsPtr          uintptr
	itemsLen          int
	itemsRev          uint64
	filterText        string
	filterBroadMode   bool
	navLevel          model.Level
	allGroupsExpanded bool
	expandedGroup     string
}

// middleItemsHeaderPtr backstops itemsRev the same way TableRenderer's
// itemsPtr backstops middleRev, against a reassignment that skips setMiddleItems.
func middleItemsHeaderPtr(items []model.Item) uintptr {
	if len(items) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&items[0]))
}

// visibleMiddleItemsCacheEntry must be replaced wholesale, never mutated in
// place: a by-value Model copy shares this pointer.
type visibleMiddleItemsCacheEntry struct {
	key   visibleMiddleItemsKey
	items []model.Item
}

// visibleMiddleItems returns the filtered subset of middleItems when a filter
// is active, or all middleItems otherwise. At LevelResourceTypes, it also
// applies collapsible group logic (accordion behavior). Memoized: callers
// must not mutate the returned slice.
func (m *Model) visibleMiddleItems() []model.Item {
	key := visibleMiddleItemsKey{
		itemsPtr:          middleItemsHeaderPtr(m.middleItems),
		itemsLen:          len(m.middleItems),
		itemsRev:          m.middleItemsRev,
		filterText:        m.filterText,
		filterBroadMode:   m.filterBroadMode,
		navLevel:          m.nav.Level,
		allGroupsExpanded: m.allGroupsExpanded,
		expandedGroup:     m.expandedGroup,
	}
	if c := m.middleItemsVisibleCache; c != nil && c.key == key {
		return c.items
	}
	items := m.computeVisibleMiddleItems()
	m.middleItemsVisibleCache = &visibleMiddleItemsCacheEntry{key: key, items: items}
	return items
}
