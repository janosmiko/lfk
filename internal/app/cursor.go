package app

import (
	"sort"
	"strings"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// parentIndex returns the index of the parent item in leftItems, or -1 if none.
//
// At LevelResources the match uses the ResourceRef (group/version/resource)
// stored on Item.Extra rather than DisplayName. API-discovery-produced
// ResourceTypeEntry values leave DisplayName empty — only pseudo-resources
// (Port Forwards, Helm Releases) and the curated BuiltInMetadata table
// carry one — so matching on DisplayName silently drops the highlight for
// every real-world resource. ResourceRef is populated for every sidebar
// item and is the canonical identity of a resource type.
func (m *Model) parentIndex() int {
	switch m.nav.Level {
	case model.LevelResourceTypes:
		return indexByName(m.leftItems, m.nav.Context)
	case model.LevelResources:
		return indexByExtra(m.leftItems, m.nav.ResourceType.ResourceRef())
	case model.LevelOwned:
		return indexByName(m.leftItems, m.nav.ResourceName)
	case model.LevelContainers:
		return indexByName(m.leftItems, m.nav.OwnedName)
	default:
		return -1
	}
}

func indexByName(items []model.Item, name string) int {
	if name == "" {
		return -1
	}
	for i, item := range items {
		if item.Name == name {
			return i
		}
	}
	return -1
}

func indexByExtra(items []model.Item, extra string) int {
	// A zero-value ResourceTypeEntry produces the sentinel "//", which no
	// real sidebar item carries — reject it explicitly so the linear scan
	// cannot accidentally match a malformed entry.
	if extra == "" || extra == "//" {
		return -1
	}
	for i, item := range items {
		if item.Extra == extra {
			return i
		}
	}
	return -1
}

// cursor returns the cursor position for the current level.
func (m *Model) cursor() int {
	return m.cursors[m.nav.Level]
}

// setCursor sets the cursor for the current level.
func (m *Model) setCursor(v int) {
	m.cursors[m.nav.Level] = v
}

// clampCursor ensures the cursor is within bounds for visible (filtered) middleItems.
func (m *Model) clampCursor() {
	c := m.cursor()
	if c < 0 {
		c = 0
	}
	visible := m.visibleMiddleItems()
	if len(visible) > 0 && c >= len(visible) {
		c = len(visible) - 1
	}
	m.setCursor(c)
}

// cursorItemKey returns a stable identifier for the currently selected visible item.
// Returns empty strings if no item is selected. Kind is included in the
// identity so that resources sharing the same name+namespace+extra (e.g. an
// ArgoCD application that creates a Deployment, Service, ConfigMap and
// ServiceAccount all named "myapp" — all in the same namespace, all with
// extra "/v1" derived from group/version only) can still be told apart when
// the cursor is restored after a refresh.
func (m *Model) cursorItemKey() (name, namespace, extra, kind string) {
	visible := m.visibleMiddleItems()
	c := m.cursor()
	if c >= 0 && c < len(visible) {
		return visible[c].Name, visible[c].Namespace, visible[c].Extra, visible[c].Kind
	}
	return "", "", "", ""
}

// restoreCursorToItem adjusts the cursor to point at the item matching the given
// name/namespace/extra/kind in the current visible items. Falls back to
// clampCursor if the item is no longer in the list.
func (m *Model) restoreCursorToItem(name, namespace, extra, kind string) {
	if name == "" && extra == "" && kind == "" {
		m.clampCursor()
		return
	}
	visible := m.visibleMiddleItems()
	for i, item := range visible {
		if item.Name == name && item.Namespace == namespace && item.Extra == extra && item.Kind == kind {
			m.setCursor(i)
			return
		}
	}
	// Item gone -- keep cursor in bounds.
	m.clampCursor()
}

// carryOverMetricsColumns copies metrics columns (CPU, CPU/R, CPU/L, MEM, MEM/R, MEM/L)
// from existing middle items to new items by matching on name+namespace.
// This prevents blinking during watch mode refreshes while metrics load async.
// Only carries over if actual usage data exists (CPU/MEM have real values).
func (m *Model) carryOverMetricsColumns(newItems []model.Item) {
	metricsKeys := map[string]bool{
		"CPU": true, "CPU/R": true, "CPU/L": true,
		"MEM": true, "MEM/R": true, "MEM/L": true,
		"CPU%": true, "MEM%": true,
	}
	// Build lookup from old items -- only if they have real usage data.
	type itemKey struct{ ns, name string }
	oldMetrics := make(map[itemKey][]model.KeyValue)
	for _, item := range m.middleItems {
		var cols []model.KeyValue
		hasUsage := false
		for _, kv := range item.Columns {
			if metricsKeys[kv.Key] {
				cols = append(cols, kv)
				if (kv.Key == "CPU" || kv.Key == "MEM") && kv.Value != "" && kv.Value != "0" && kv.Value != "0m" && kv.Value != "0B" {
					hasUsage = true
				}
			}
		}
		if hasUsage && len(cols) > 0 {
			oldMetrics[itemKey{item.Namespace, item.Name}] = cols
		}
	}
	if len(oldMetrics) == 0 {
		return
	}
	// Apply to new items: prepend carried-over metrics columns while keeping
	// the raw request/limit columns (CPU Req, CPU Lim, Mem Req, Mem Lim) so
	// that podMetricsEnrichedMsg can still read them to compute percentages.
	for i := range newItems {
		key := itemKey{newItems[i].Namespace, newItems[i].Name}
		cols, ok := oldMetrics[key]
		if !ok {
			continue
		}
		var kept []model.KeyValue
		for _, kv := range newItems[i].Columns {
			if !metricsKeys[kv.Key] {
				kept = append(kept, kv)
			}
		}
		merged := make([]model.KeyValue, 0, len(cols)+len(kept))
		merged = append(merged, cols...)
		merged = append(merged, kept...)
		newItems[i].Columns = merged
	}
}

// clampAllCursors ensures all cursor positions are within bounds after resize.
func (m *Model) clampAllCursors() {
	m.clampCursor()
	// Clamp event timeline cursor on resize.
	if len(m.eventTimelineLines) > 0 {
		if m.eventTimelineCursor >= len(m.eventTimelineLines) {
			m.eventTimelineCursor = len(m.eventTimelineLines) - 1
		}
		m.ensureEventCursorVisible()
	}
	// Clamp describe cursor on resize.
	if m.mode == modeDescribe && m.describeContent != "" {
		m.ensureDescribeCursorVisible()
	}
}

// middleColumnKind returns the lowercased kind that identifies the items
// currently rendered in the middle column. It is used as the key for the
// sessionColumns, hiddenBuiltinColumns, and columnOrder maps so that
// column-visibility changes made while viewing one level do not leak into
// another level that happens to navigate under the same parent
// ResourceType (e.g., container columns must be independent of pod
// columns even though nav.ResourceType stays "Pod" at LevelContainers).
//
// At LevelOwned and LevelContainers the parent's ResourceType.Kind is
// misleading — the middle column shows different kinds (ReplicaSets,
// Containers, etc.), so the method derives the kind from the first
// middleItem. It falls back to nav.ResourceType.Kind when middleItems is
// empty or at shallower levels.
func (m *Model) middleColumnKind() string {
	if m.nav.Level == model.LevelOwned || m.nav.Level == model.LevelContainers {
		if len(m.middleItems) > 0 && m.middleItems[0].Kind != "" {
			return strings.ToLower(m.middleItems[0].Kind)
		}
	}
	return strings.ToLower(m.nav.ResourceType.Kind)
}

// navKey builds a unique key from the current navigation state, used for
// cursor memory and item caching.
func (m *Model) navKey() string {
	parts := []string{m.nav.Context}
	if m.nav.ResourceType.Resource != "" {
		parts = append(parts, m.nav.ResourceType.Resource)
	}
	if m.nav.ResourceName != "" {
		parts = append(parts, m.nav.ResourceName)
	}
	if m.nav.OwnedName != "" {
		parts = append(parts, m.nav.OwnedName)
	}
	return strings.Join(parts, "/")
}

// saveCursor stores the current cursor position keyed by navigation path.
func (m *Model) saveCursor() {
	m.cursorMemory[m.navKey()] = m.cursor()
}

// restoreCursor restores the cursor position from memory for the current
// navigation path, or resets to 0 if no saved position exists.
func (m *Model) restoreCursor() {
	if pos, ok := m.cursorMemory[m.navKey()]; ok {
		m.setCursor(pos)
		m.clampCursor()
		return
	}
	m.setCursor(0)
}

// selectedMiddleItem returns the currently selected item in the middle column,
// taking into account any active filter.
func (m *Model) selectedMiddleItem() *model.Item {
	visible := m.visibleMiddleItems()
	c := m.cursor()
	if c >= 0 && c < len(visible) {
		// Return a pointer to the item in middleItems (not the filtered copy).
		target := visible[c]
		for i := range m.middleItems {
			if m.middleItems[i].Name == target.Name &&
				m.middleItems[i].Kind == target.Kind &&
				m.middleItems[i].Extra == target.Extra &&
				m.middleItems[i].Namespace == target.Namespace {
				return &m.middleItems[i]
			}
		}
		// Fallback: return the filtered item directly.
		return &visible[c]
	}
	return nil
}

// selectionKey generates a unique key for an item used in the selectedItems map.
func selectionKey(item model.Item) string {
	if item.Namespace != "" {
		return item.Namespace + "/" + item.Name
	}
	return item.Name
}

// isSelected returns true if the given item is in the multi-selection set.
func (m *Model) isSelected(item model.Item) bool {
	return m.selectedItems[selectionKey(item)]
}

// toggleSelection toggles the selection state of an item.
func (m *Model) toggleSelection(item model.Item) {
	key := selectionKey(item)
	if m.selectedItems[key] {
		delete(m.selectedItems, key)
	} else {
		m.selectedItems[key] = true
	}
}

// clearSelection removes all items from the multi-selection set and resets the region anchor.
func (m *Model) clearSelection() {
	m.selectedItems = make(map[string]bool)
	m.selectionAnchor = -1
}

// hasSelection returns true if any items are selected.
func (m *Model) hasSelection() bool {
	return len(m.selectedItems) > 0
}

// selectedItemsList returns the list of currently selected items from visibleMiddleItems.
func (m *Model) selectedItemsList() []model.Item {
	var selected []model.Item
	for _, item := range m.visibleMiddleItems() {
		if m.isSelected(item) {
			selected = append(selected, item)
		}
	}
	return selected
}

// visibleMiddleItems returns the filtered subset of middleItems when a filter
// is active, or all middleItems otherwise. At LevelResourceTypes, it also
// applies collapsible group logic (accordion behavior).
func (m *Model) visibleMiddleItems() []model.Item {
	items := m.middleItems

	// Apply text filter first.
	if m.filterText != "" {
		rawQuery := m.filterText

		// First pass: determine which categories match the filter.
		matchedCategories := make(map[string]bool)
		for _, item := range items {
			if item.Category != "" && ui.MatchLine(item.Category, rawQuery) {
				matchedCategories[item.Category] = true
			}
		}

		// Second pass: include items that match by name OR belong to a matched category.
		var filtered []model.Item
		for _, item := range items {
			if item.Category != "" && matchedCategories[item.Category] {
				filtered = append(filtered, item)
				continue
			}
			// Match against name (and namespace/name for namespaced resources).
			searchText := item.Name
			if item.Namespace != "" {
				searchText = item.Namespace + "/" + searchText
			}
			if ui.MatchLine(searchText, rawQuery) {
				filtered = append(filtered, item)
				continue
			}
			// Broad mode: also scan column values (annotations, labels,
			// finalizers, CRD printer columns, custom user columns).
			// Internal-prefix columns stay excluded.
			if m.filterBroadMode {
				for _, kv := range item.Columns {
					if isInternalColumnKey(kv.Key) {
						continue
					}
					if ui.MatchLine(kv.Value, rawQuery) {
						filtered = append(filtered, item)
						break
					}
				}
			}
		}
		items = filtered

		// When in fuzzy mode, sort results by fuzzy score (best matches first).
		mode, query := ui.DetectSearchMode(rawQuery)
		if mode == ui.SearchFuzzy && query != "" {
			type scoredItem struct {
				item  model.Item
				score int
			}
			scored := make([]scoredItem, 0, len(items))
			for _, item := range items {
				s := ui.FuzzyScore(item.Name, query)
				scored = append(scored, scoredItem{item: item, score: s})
			}
			sort.SliceStable(scored, func(i, j int) bool {
				return scored[i].score > scored[j].score
			})
			sortedItems := make([]model.Item, len(scored))
			for i, si := range scored {
				sortedItems[i] = si.item
			}
			items = sortedItems
		}
	}

	// Apply collapsible group logic at LevelResourceTypes. When a text
	// filter is active, skip the collapse step so matched items in
	// non-expanded categories stay visible and navigable — otherwise a
	// filter like "pods" would hide the Pods item inside a collapsed
	// "Workloads" header when some other group happens to be expanded.
	if m.nav.Level == model.LevelResourceTypes && !m.allGroupsExpanded && m.filterText == "" {
		var collapsed []model.Item
		seenCategories := make(map[string]bool)
		for _, item := range items {
			// Items with no category or in the Dashboards group are always shown expanded.
			if item.Category == "" || item.Category == "Dashboards" {
				collapsed = append(collapsed, item)
				continue
			}
			if item.Category == m.expandedGroup {
				// Expanded group: show all items.
				collapsed = append(collapsed, item)
				seenCategories[item.Category] = true
			} else if !seenCategories[item.Category] {
				// Collapsed group: insert a placeholder (header-only, no item line).
				seenCategories[item.Category] = true
				collapsed = append(collapsed, model.Item{
					Name:     item.Category,
					Kind:     "__collapsed_group__",
					Category: item.Category,
				})
			}
		}
		items = collapsed
	}

	return items
}

// categoryCounts returns the number of items in each category from the full
// (unfiltered, uncollapsed) middleItems list. Used for rendering collapsed
// group headers with item counts.
func (m *Model) categoryCounts() map[string]int {
	counts := make(map[string]int)
	for _, item := range m.middleItems {
		if item.Category != "" {
			counts[item.Category]++
		}
	}
	return counts
}

// syncExpandedGroup updates the expanded group to match the category of the
// item currently under the cursor. This is used after cursor jumps (g/G) and
// when navigating back to LevelResourceTypes.
func (m *Model) syncExpandedGroup() {
	if m.nav.Level != model.LevelResourceTypes || m.allGroupsExpanded {
		return
	}
	visible := m.visibleMiddleItems()
	c := m.cursor()
	if c >= len(visible) {
		c = len(visible) - 1
		m.setCursor(c)
	}
	if c >= 0 && c < len(visible) {
		cat := visible[c].Category
		if cat != "" && cat != m.expandedGroup {
			m.expandedGroup = cat
			// Recompute and find the first real item of this category.
			newVisible := m.visibleMiddleItems()
			for i, item := range newVisible {
				if item.Category == cat && item.Kind != "__collapsed_group__" {
					m.setCursor(i)
					return
				}
			}
			m.clampCursor()
		}
	}
}

// filteredExplainRecursiveResults returns recursive search results filtered by the overlay filter input.
func (m *Model) filteredExplainRecursiveResults() []model.ExplainField {
	if m.explainRecursiveFilter.Value == "" {
		return m.explainRecursiveResults
	}
	rawQuery := m.explainRecursiveFilter.Value
	var filtered []model.ExplainField
	for _, f := range m.explainRecursiveResults {
		if ui.MatchLine(f.Name, rawQuery) || ui.MatchLine(f.Path, rawQuery) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// filteredOverlayItems returns overlay items matching the current filter.
//
// Allocates a non-nil empty slice when the filter matches nothing so
// downstream renderers (e.g. RenderNamespaceOverlay) can distinguish
// "filter excluded everything" (empty) from "fetch still in flight"
// (nil). Without the upfront allocation, a no-match filter slipped
// through as nil and the namespace overlay rendered "Loading
// namespaces..." indefinitely.
func (m *Model) filteredOverlayItems() []model.Item {
	if m.overlayFilter.Value == "" {
		return m.overlayItems
	}
	rawQuery := m.overlayFilter.Value
	filtered := []model.Item{}
	for _, item := range m.overlayItems {
		if ui.MatchLine(item.Name, rawQuery) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filteredLogPodItems returns overlay items matching the current log pod filter.
func (m *Model) filteredLogPodItems() []model.Item {
	if m.logPodFilterText == "" {
		return m.overlayItems
	}
	rawQuery := m.logPodFilterText
	var filtered []model.Item
	for _, item := range m.overlayItems {
		if ui.MatchLine(item.Name, rawQuery) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filteredLogContainerItems returns overlay items matching the current log container filter.
func (m *Model) filteredLogContainerItems() []model.Item {
	if m.logContainerFilterText == "" {
		return m.overlayItems
	}
	rawQuery := m.logContainerFilterText
	var filtered []model.Item
	for _, item := range m.overlayItems {
		// Always include the "All Containers" virtual item.
		if item.Status == "all" || ui.MatchLine(item.Name, rawQuery) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
