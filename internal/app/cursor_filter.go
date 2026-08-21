package app

import (
	"sort"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// computeVisibleMiddleItems is the uncached body of visibleMiddleItems (see
// cursor_visible_items_cache.go).
func (m *Model) computeVisibleMiddleItems() []model.Item {
	items := m.middleItems

	// Apply text filter first.
	if m.filterText != "" {
		items = m.applyTextFilter(items)
	}

	// Apply collapsible group logic at LevelResourceTypes. When a text
	// filter is active, skip the collapse step so matched items in
	// non-expanded categories stay visible and navigable — otherwise a
	// filter like "pods" would hide the Pods item inside a collapsed
	// "Workloads" header when some other group happens to be expanded.
	if m.nav.Level == model.LevelResourceTypes && !m.allGroupsExpanded && m.filterText == "" {
		items = m.applyGroupCollapse(items)
	}

	return items
}

// applyTextFilter is the text-filter pass of computeVisibleMiddleItems.
func (m *Model) applyTextFilter(items []model.Item) []model.Item {
	rawQuery := m.filterText

	// Category expansion is gated on broad mode (Tab) and only at
	// LevelResourceTypes where the category bar actually renders.
	// Plain `f ing` matches resource type names only — typing it
	// must not pull in every Networking member just because the
	// category name contains the substring.
	expandByCategory := m.filterBroadMode && m.nav.Level == model.LevelResourceTypes

	// First pass: determine which categories match the filter
	// (only used when expandByCategory is on).
	matchedCategories := make(map[string]bool)
	if expandByCategory {
		for _, item := range items {
			if item.Category != "" && ui.MatchLine(item.Category, rawQuery) {
				matchedCategories[item.Category] = true
			}
		}
	}

	// Second pass: include items that match by name OR belong to a matched category.
	var filtered []model.Item
	for _, item := range items {
		if expandByCategory && item.Category != "" && matchedCategories[item.Category] {
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
		// Internal-prefix columns stay excluded. Outside
		// LevelResourceTypes this is what Tab does — the category
		// branch above is a no-op there.
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
		items = sortItemsByFuzzyScore(items, query, expandByCategory, matchedCategories, m.filterBroadMode)
	}

	return items
}

// sortItemsByFuzzyScore is the fuzzy re-sort inside applyTextFilter.
func sortItemsByFuzzyScore(items []model.Item, query string, expandByCategory bool, matchedCategories map[string]bool, broadMode bool) []model.Item {
	type scoredItem struct {
		item  model.Item
		score int
	}
	scored := make([]scoredItem, 0, len(items))
	for _, item := range items {
		s := fuzzyFieldScore(item, query, expandByCategory, matchedCategories, broadMode)
		scored = append(scored, scoredItem{item: item, score: s})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	sortedItems := make([]model.Item, len(scored))
	for i, si := range scored {
		sortedItems[i] = si.item
	}
	return sortedItems
}

// fuzzyFieldScore mirrors the matching pass above: Category alone for a
// category-expanded item, otherwise the best of namespace/name and (broad
// mode) column values.
func fuzzyFieldScore(item model.Item, query string, expandByCategory bool, matchedCategories map[string]bool, broadMode bool) int {
	if expandByCategory && item.Category != "" && matchedCategories[item.Category] {
		return ui.FuzzyScore(item.Category, query)
	}
	searchText := item.Name
	if item.Namespace != "" {
		searchText = item.Namespace + "/" + searchText
	}
	best := ui.FuzzyScore(searchText, query)
	if broadMode {
		for _, kv := range item.Columns {
			if isInternalColumnKey(kv.Key) {
				continue
			}
			if s := ui.FuzzyScore(kv.Value, query); s > best {
				best = s
			}
		}
	}
	return best
}

// applyGroupCollapse is the accordion collapse pass of computeVisibleMiddleItems.
func (m *Model) applyGroupCollapse(items []model.Item) []model.Item {
	var collapsed []model.Item
	seenCategories := make(map[string]bool)
	for _, item := range items {
		// Items with no category or in the Dashboards group are always shown expanded.
		if item.Category == "" || item.Category == "Dashboards" || item.Category == "Pinned" {
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
	return collapsed
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
	if m.logView.podFilterText == "" {
		return m.overlayItems
	}
	rawQuery := m.logView.podFilterText
	var filtered []model.Item
	for _, item := range m.overlayItems {
		if ui.MatchLine(item.Name, rawQuery) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// filteredLogContainerItems returns overlay items matching the current log container filter.
//
// The "All Containers" virtual row is filtered by name like every other
// entry — keeping it pinned would clutter the narrowed list and break the
// muscle-memory consistency with the namespace and log pod selectors.
// Users can still reach all-containers by clearing the filter.
func (m *Model) filteredLogContainerItems() []model.Item {
	if m.logView.containerFilterText == "" {
		return m.overlayItems
	}
	rawQuery := m.logView.containerFilterText
	filtered := []model.Item{}
	for _, item := range m.overlayItems {
		if ui.MatchLine(item.Name, rawQuery) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
