package app

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

type pinnedOrderEntry struct {
	key    string
	label  string
	locked bool
}

// pinnedOrderState is the runtime state of the pinned-summaries order overlay.
type pinnedOrderState struct {
	items  []pinnedOrderEntry
	cursor int
}

var overlayPinnedOrderScrollPos int

// pinnedOrderSource returns the persisted keys the overlay reorders: the
// visible defaults while they are showing, else the scope's state slice.
func (m Model) pinnedOrderSource() []string {
	if m.defaultsActiveForScope() {
		return m.seedDefaultPinnedSummaries()
	}
	return m.currentScopeSlice()
}

func (m Model) openPinnedOrder() (Model, tea.Cmd) {
	refuse := func(msg string) (Model, tea.Cmd) {
		m.setStatusMessage(msg, true)
		return m, scheduleStatusClear()
	}
	if m.isUnionSentinel() && m.unionSetName == "" {
		return refuse("Pinning in union mode requires a named union set")
	}
	if defaultPinsDisabled() {
		return refuse("Nothing to reorder")
	}

	discovered := m.defaultSeedDiscovery()
	seen := make(map[string]bool)
	var items []pinnedOrderEntry
	add := func(key string, locked bool) {
		if key == "" || seen[key] || len(items) >= maxPinnedSummaries() {
			return
		}
		seen[key] = true
		label := key
		if e, ok := resolvePinnedSummaryEntry(discovered, key); ok {
			label = model.DisplayNameFor(e)
		}
		items = append(items, pinnedOrderEntry{key: key, label: label, locked: locked})
	}
	for _, k := range ui.ConfigPinnedSummaries {
		add(k, true)
	}
	for _, k := range m.pinnedOrderSource() {
		add(k, false)
	}

	cursor, movable := -1, 0
	for i, e := range items {
		if !e.locked {
			movable++
			if cursor < 0 {
				cursor = i
			}
		}
	}
	if movable < 2 {
		return refuse("Nothing to reorder")
	}
	m.pinnedOrder = pinnedOrderState{items: items, cursor: cursor}
	m.overlay = overlayPinnedOrder
	return m, nil
}

func (m Model) handlePinnedOrderKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	maxIdx := len(m.pinnedOrder.items) - 1
	switch msg.String() {
	case "j", "down":
		m.pinnedOrder.cursor = clampOverlayCursor(m.pinnedOrder.cursor, 1, maxIdx)
	case "k", "up":
		m.pinnedOrder.cursor = clampOverlayCursor(m.pinnedOrder.cursor, -1, maxIdx)
	case "ctrl+d":
		m.pinnedOrder.cursor = clampOverlayCursor(m.pinnedOrder.cursor, 10, maxIdx)
	case "ctrl+u":
		m.pinnedOrder.cursor = clampOverlayCursor(m.pinnedOrder.cursor, -10, maxIdx)
	case "ctrl+f":
		m.pinnedOrder.cursor = clampOverlayCursor(m.pinnedOrder.cursor, 20, maxIdx)
	case "ctrl+b":
		m.pinnedOrder.cursor = clampOverlayCursor(m.pinnedOrder.cursor, -20, maxIdx)
	case "esc", "q":
		m.overlay = overlayNone
		m.pinnedOrder = pinnedOrderState{}
	case "J":
		m.movePinnedOrderRow(1)
	case "K":
		m.movePinnedOrderRow(-1)
	case "enter":
		return m.savePinnedOrder()
	}
	return m, nil
}

func (m *Model) movePinnedOrderRow(delta int) {
	from, to := m.pinnedOrder.cursor, m.pinnedOrder.cursor+delta
	items := m.pinnedOrder.items
	if to < 0 || to >= len(items) || items[from].locked || items[to].locked {
		return
	}
	items = slices.Clone(items)
	items[from], items[to] = items[to], items[from]
	m.pinnedOrder = pinnedOrderState{items: items, cursor: to}
}

// savePinnedOrder persists the new order. Keys the cap cut from the overlay
// stay at the end so reordering never drops a pin.
func (m Model) savePinnedOrder() (tea.Model, tea.Cmd) {
	source := m.pinnedOrderSource()
	shown := make(map[string]bool, len(m.pinnedOrder.items))
	var moved []string
	for _, e := range m.pinnedOrder.items {
		if !e.locked {
			shown[e.key] = true
			moved = append(moved, e.key)
		}
	}
	var before, rest []string
	for _, k := range source {
		if shown[k] {
			before = append(before, k)
		} else if k != "" {
			rest = append(rest, k)
		}
	}
	newOrder := append(slices.Clone(moved), rest...)
	m.overlay = overlayNone
	m.pinnedOrder = pinnedOrderState{}
	if slices.Equal(moved, before) {
		return m, nil
	}

	if m.pinnedSummariesState == nil {
		m.pinnedSummariesState = newPinnedState()
	}
	preSeed := m.currentScopeSlice()
	m.setScopePinnedSummaries(newOrder)
	if err := savePinnedSummariesState(m.pinnedSummariesState); err != nil {
		m.setScopePinnedSummaries(preSeed)
		m.setStatusMessage(fmt.Sprintf("Failed to save pinned summaries: %v", err), true)
		return m, scheduleStatusClear()
	}

	delete(m.dashboardData, m.dashboardPreviewTargetContext())
	m.dashboardPreview = ""
	m.dashboardEventsPreview = ""
	m.setStatusMessage("Pinned summaries reordered", false)
	if reload := m.summaryDashboardReloadCmd(); reload != nil {
		return m, tea.Batch(reload, scheduleStatusClear())
	}
	return m, scheduleStatusClear()
}

func (m Model) renderOverlayPinnedOrder() (string, int, int) {
	overlayW := min(50, m.width-10)
	overlayH := min(20, m.height-6)
	items := make([]ui.OverlayListItem, len(m.pinnedOrder.items))
	for i, e := range m.pinnedOrder.items {
		items[i] = ui.OverlayListItem{Name: e.label}
		if e.locked {
			items[i].Description = "config"
		}
	}
	contentH := max(overlayH-2, 1)
	maxVisible := max(contentH-overlayListChromeFilterable(), 1)
	content := ui.RenderOverlayList(items, ui.OverlayListConfig{
		Title:           "Pinned Summaries Order",
		Cursor:          m.pinnedOrder.cursor,
		ShowDescription: true,
		Scroll:          overlayListScroll(&overlayPinnedOrderScrollPos, m.pinnedOrder.cursor, len(items), maxVisible),
		MaxVisible:      maxVisible,
		EmptyMessage:    "No pinned summaries",
		Height:          contentH,
	}, overlayW-6)
	return content, overlayW, overlayH
}
