package app

import (
	"maps"
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

func (m *Model) cursor() int {
	return m.cursors[m.nav.Level]
}

func (m *Model) setCursor(v int) {
	m.cursors[m.nav.Level] = v
}

// clampCursor ensures the cursor is within bounds for visible (filtered) middleItems.
func (m *Model) clampCursor() {
	c := max(m.cursor(), 0)
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

// setMiddleItems replaces the middleItems slice and bumps the row-cache rev.
// Every path that swaps middleItems (full replace, filter-and-replace,
// append-and-replace, nil-out) must go through this helper so the
// TableRenderer fingerprint invalidates. Slice reassignment alone is not
// enough — itemsPtr is only a fast-path. Rev is the authoritative signal.
func (m *Model) setMiddleItems(items []model.Item) {
	m.middleItems = items
	m.middleItemsRev++
}

// carryOverMetricsColumns copies metrics columns (CPU, CPU/R, CPU/L, MEM, MEM/R, MEM/L)
// from existing middle items to new items by matching on name+namespace.
// This prevents blinking during watch mode refreshes while metrics load async.
// Only carries over if actual usage data exists (CPU/MEM have real values).
func (m *Model) carryOverMetricsColumns(newItems []model.Item) {
	carryOverMetricsColumnsFrom(m.middleItems, newItems)
}

// carryOverMetricsColumnsFrom is the source-explicit core of
// carryOverMetricsColumns. The right-pane list preview carries from its own
// previous items / the drill-in cache instead of middleItems (issue #408:
// preview fetches return unenriched items, so without the carry-over the
// preview's column set flips against the drilled list on every refetch).
func carryOverMetricsColumnsFrom(oldItems, newItems []model.Item) {
	carryOverBootedAt(oldItems, newItems)
	metricsKeys := map[string]bool{
		"CPU": true, "CPU/R": true, "CPU/L": true,
		"MEM": true, "MEM/R": true, "MEM/L": true,
		"CPU%": true, "MEM%": true, "Uptime": true,
	}
	// Build lookup from old items. Carry over whatever metrics columns the
	// item already had so the column set stays visually stable across watch
	// ticks -- including "n/a" placeholders set by the node enrichment path
	// when metrics-server returned nothing. The previous hasUsage gate
	// dropped the carryover whenever every value was empty/zero, which made
	// the metrics columns flicker out and back in on each refresh.
	// The key includes the cluster: in union mode the same namespace+name can
	// exist in several clusters and must not share carried columns.
	type itemKey struct{ cluster, ns, name string }
	oldMetrics := make(map[itemKey][]model.KeyValue)
	for _, item := range oldItems {
		var cols []model.KeyValue
		for _, kv := range item.Columns {
			if metricsKeys[kv.Key] {
				cols = append(cols, kv)
			}
		}
		if len(cols) > 0 {
			oldMetrics[itemKey{item.ClusterName, item.Namespace, item.Name}] = cols
		}
	}
	if len(oldMetrics) == 0 {
		return
	}
	// Apply to new items: prepend carried-over metrics columns while keeping
	// the raw request/limit columns (CPU Req, CPU Lim, Mem Req, Mem Lim) so
	// that podMetricsEnrichedMsg can still read them to compute percentages.
	for i := range newItems {
		key := itemKey{newItems[i].ClusterName, newItems[i].Namespace, newItems[i].Name}
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

// carryOverServiceEndpointColumns copies the lazily-fetched Service
// rollup columns ("Backing Endpoints" + the multi-line "Endpoints"
// block) from the existing middleItems into the freshly-loaded ones,
// matched on name+namespace.
//
// Without this carry-over, every watch-tick refresh would replace the
// whole middleItems slice with new objects whose Columns don't yet
// have the rollup, and the table would render once without those rows
// before the async fetch lands ~100ms later — a visible flash and
// layout jump in the right pane. The carry-over keeps the previous
// values in place so the next render is identical to the prior one
// until the fresh fetch updates the columns from
// updatePreviewServiceEndpointsLoaded.
//
// Same reasoning as carryOverMetricsColumns above.
func (m *Model) carryOverServiceEndpointColumns(newItems []model.Item) {
	carryOverServiceEndpointColumnsFrom(m.middleItems, newItems)
}

// carryOverServiceEndpointColumnsFrom is the source-explicit core of
// carryOverServiceEndpointColumns, shared with the right-pane list preview
// (see carryOverMetricsColumnsFrom).
func carryOverServiceEndpointColumnsFrom(oldItems, newItems []model.Item) {
	endpointKeys := map[string]bool{
		"Backing Endpoints": true,
		"Endpoints":         true,
	}
	// Cluster-qualified key: see carryOverMetricsColumnsFrom.
	type itemKey struct{ cluster, ns, name string }
	old := make(map[itemKey][]model.KeyValue)
	for _, item := range oldItems {
		var cols []model.KeyValue
		for _, kv := range item.Columns {
			if endpointKeys[kv.Key] {
				cols = append(cols, kv)
			}
		}
		if len(cols) > 0 {
			old[itemKey{item.ClusterName, item.Namespace, item.Name}] = cols
		}
	}
	if len(old) == 0 {
		return
	}
	for i := range newItems {
		key := itemKey{newItems[i].ClusterName, newItems[i].Namespace, newItems[i].Name}
		cols, ok := old[key]
		if !ok {
			continue
		}
		// Drop any rollup columns that arrived on the new item (the
		// populator only writes "Type / Cluster IP / ..." for Services,
		// so this loop is a no-op today — but keeping it makes the
		// helper safe if the populator ever starts writing them too).
		var kept []model.KeyValue
		for _, kv := range newItems[i].Columns {
			if !endpointKeys[kv.Key] {
				kept = append(kept, kv)
			}
		}
		merged := make([]model.KeyValue, 0, len(kept)+len(cols))
		merged = append(merged, kept...)
		merged = append(merged, cols...)
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
	if m.mode == modeDescribe && m.describeView.content != "" {
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

// middleColumnRef returns the ui.ResourceRef identifying the resource
// currently rendered in the middle column. Used to resolve view configs
// keyed by GVR or Kind. At LevelOwned/LevelContainers the parent's GVR
// does not match the rendered items, so only Kind is populated (matching
// the kind returned by middleColumnKind). Resolution then falls back to
// Kind-only lookup. At shallower levels the full GVR is returned so views
// keyed by `<group>/<version>/<resource>` resolve.
func (m *Model) middleColumnRef() ui.ResourceRef {
	if m.nav.Level == model.LevelOwned || m.nav.Level == model.LevelContainers {
		if len(m.middleItems) > 0 && m.middleItems[0].Kind != "" {
			return ui.ResourceRef{Kind: m.middleItems[0].Kind}
		}
		return ui.ResourceRef{Kind: m.nav.ResourceType.Kind}
	}
	return ui.ResourceRef{
		Group:    m.nav.ResourceType.APIGroup,
		Version:  m.nav.ResourceType.APIVersion,
		Resource: m.nav.ResourceType.Resource,
		Kind:     m.nav.ResourceType.Kind,
	}
}

// viewRefForKind returns a ResourceRef suitable for resolving view config
// (HiddenBuiltinsForView / ColumnsForKind / ResolveView) for the given
// kind. When kind matches nav.ResourceType.Kind, the full GVR is included
// so views keyed by GVR resolve. Otherwise (LevelOwned/LevelContainers
// where rendered kind diverges from nav.ResourceType) a Kind-only ref is
// returned so resolution falls back to Kind lookup.
func (m *Model) viewRefForKind(kind string) ui.ResourceRef {
	if kind != "" && strings.EqualFold(kind, m.nav.ResourceType.Kind) {
		return ui.ResourceRef{
			Group:    m.nav.ResourceType.APIGroup,
			Version:  m.nav.ResourceType.APIVersion,
			Resource: m.nav.ResourceType.Resource,
			Kind:     m.nav.ResourceType.Kind,
		}
	}
	return ui.ResourceRef{Kind: kind}
}

// columnMemoryKey scopes the per-kind column session maps (sessionColumns,
// hiddenBuiltinColumns, columnOrder) to the current cluster context, so the
// same kind can show different columns in different clusters — mirroring sort
// memory. Both the render path (applySessionColumnsForKind) and the
// column-toggle overlay must route map access through this so reads and
// writes share one key.
func (m *Model) columnMemoryKey(kind string) string {
	return m.nav.Context + "\x00" + kind
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

// savedFilter captures a list's committed filter so it can be recalled exactly
// when the user returns to that level. broad mirrors m.filterBroadMode (the Tab
// toggle that also matches column values) so a broad filter doesn't come back
// as a plain name filter.
type savedFilter struct {
	text  string
	broad bool
}

// copyMapStringSavedFilter deep copies a map[string]savedFilter. A nil input
// yields a non-nil empty map so callers can write into it without a nil check.
func copyMapStringSavedFilter(m map[string]savedFilter) map[string]savedFilter {
	c := make(map[string]savedFilter, len(m))
	maps.Copy(c, m)
	return c
}

// saveLevelFilter persists the committed filter for the current navigation path
// so it can be restored when the user returns to this list. An empty filter
// deletes any prior entry so a later visit starts clean rather than restoring a
// phantom filter. Must be called BEFORE a level change clears m.filterText.
func (m *Model) saveLevelFilter() {
	key := m.navKey()
	if m.filterText == "" {
		delete(m.filterMemory, key)
		return
	}
	if m.filterMemory == nil {
		m.filterMemory = make(map[string]savedFilter)
	}
	m.filterMemory[key] = savedFilter{text: m.filterText, broad: m.filterBroadMode}
}

// resetFilterForTypeSwitch remembers the current list's committed filter for
// back-nav restore, then clears all live filter/search state so the next list
// starts clean. Every path landing on a different resource list must call it
// BEFORE mutating m.nav — saveLevelFilter keys off the old position (TASK-839).
func (m *Model) resetFilterForTypeSwitch() {
	m.saveLevelFilter()
	m.filterText = ""
	m.filterInput.Clear()
	m.filterActive = false
	m.filterBroadMode = false
	m.activeFilterPreset = nil
	m.unfilteredMiddleItems = nil
	m.searchInput.Clear()
}

// restoreLevelFilter applies the saved filter for the current navigation path,
// or clears the live filter if none was saved (so a sibling list never inherits
// another list's filter). Must be called AFTER the destination level is set.
// It deliberately does NOT touch m.filterMemory and is only called on explicit
// navigation transitions — never on data-refresh paths — so a live filter the
// user is still typing is never clobbered.
func (m *Model) restoreLevelFilter() {
	if f, ok := m.filterMemory[m.navKey()]; ok {
		m.filterText = f.text
		m.filterInput.Set(f.text)
		m.filterBroadMode = f.broad
	} else {
		m.filterText = ""
		m.filterInput.Clear()
		m.filterBroadMode = false
	}
	m.filterActive = false
}

// selectedMiddleItem returns the currently selected item in the middle column,
// taking into account any active filter.
func (m *Model) selectedMiddleItem() *model.Item {
	visible := m.visibleMiddleItems()
	c := m.cursor()
	if c >= 0 && c < len(visible) {
		// Return a pointer to the item in middleItems (not the filtered copy).
		// ClusterName is included in the match so union-mode rows that share
		// Name+Kind+Extra+Namespace across clusters still resolve to the exact
		// row under the cursor. In non-union mode ClusterName is "" on all
		// items, so this is a no-op.
		target := visible[c]
		for i := range m.middleItems {
			if m.middleItems[i].Name == target.Name &&
				m.middleItems[i].Kind == target.Kind &&
				m.middleItems[i].Extra == target.Extra &&
				m.middleItems[i].Namespace == target.Namespace &&
				m.middleItems[i].ClusterName == target.ClusterName {
				return &m.middleItems[i]
			}
		}
		// Fallback: return the filtered item directly.
		return &visible[c]
	}
	return nil
}

// nextResourceTypeCursorItem returns a copy of the resource-type item the
// cursor should follow to after the selected item is pinned/unpinned and the
// sidebar re-sorts: the next real resource type below the cursor, falling back
// to the previous one when the selection is the last item. Returns nil when no
// sibling resource type exists. Headers and collapsed-group placeholders (whose
// Extra has no version segment) are skipped.
func (m *Model) nextResourceTypeCursorItem() *model.Item {
	visible := m.visibleMiddleItems()
	c := m.cursor()
	isType := func(it model.Item) bool {
		return it.Kind != "__collapsed_group__" && model.PinKeyFromRef(it.Extra) != ""
	}
	for i := c + 1; i < len(visible); i++ {
		if isType(visible[i]) {
			it := visible[i]
			return &it
		}
	}
	for i := c - 1; i >= 0; i-- {
		if isType(visible[i]) {
			it := visible[i]
			return &it
		}
	}
	return nil
}

// focusMiddleItem moves the cursor onto the item matching target (by
// Name/Kind/Extra) in the current visible list. No-op when target is nil or
// the item is no longer present.
func (m *Model) focusMiddleItem(target *model.Item) {
	if target == nil {
		return
	}
	visible := m.visibleMiddleItems()
	for i := range visible {
		if visible[i].Name == target.Name &&
			visible[i].Kind == target.Kind &&
			visible[i].Extra == target.Extra {
			m.setCursor(i)
			return
		}
	}
}

// selectionKey generates a unique key for an item used in the selectedItems
// map. It delegates to model.Item.SelectionKey so the app's selection store
// and the ui renderer's selected-row check share one key derivation and
// cannot drift (a drift that previously hid the multi-select marker in
// union view).
func selectionKey(item model.Item) string {
	return item.SelectionKey()
}

func (m *Model) isSelected(item model.Item) bool {
	return m.selectedItems[selectionKey(item)]
}

func (m *Model) toggleSelection(item model.Item) {
	key := selectionKey(item)
	if m.selectedItems[key] {
		delete(m.selectedItems, key)
	} else {
		m.selectedItems[key] = true
	}
	m.selectionRev++
}

func (m *Model) clearSelection() {
	m.selectedItems = make(map[string]bool)
	m.selectionAnchor = -1
	m.selectionRev++
}

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
