// Package app — update_actions_hidden.go
// Action-menu entries for hiding/showing a resource type in the sidebar.
// Hiding is offered from the action menu (x) at the resource-types level
// rather than a dedicated hotkey; the state is per-cluster-context (or named
// union set) and mirrors the pin feature (see pinned.go / handleKeyPinGroup).
// Hidden types disappear from the sidebar unless the reveal toggle (Shift+H /
// ToggleRare) is on, in which case they render dimmed so the user can find
// and un-hide them.
package app

import (
	"fmt"
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// Action labels for the resource-type action menu. Kept as constants so the
// menu builder and executeAction's dispatch switch on the same literal.
const (
	actionLabelPinType   = "Pin"
	actionLabelUnpinType = "Unpin"
	actionLabelHideType  = "Hide"
	actionLabelShowType  = "Show"
)

// hideMenuChip is the in-menu single-letter activator for the hide/show
// action. Hardcoded because there is no global hide keybinding to echo (hiding
// is offered only through this menu) and it does not collide with the
// overlay's j/k navigation. The pin/unpin entry reuses the configured
// PinGroup key so the chip stays in sync with the global binding.
const hideMenuChip = "h"

// openResourceTypeActionMenu builds the action menu shown when the user
// presses the action-menu key at the resource-types level. It offers Pin/Unpin
// and Hide/Show toggles for the resource type under the cursor, in the same
// label + description + key-chip shape as the resource action menu. Returns the
// model unchanged (no overlay) when the selection is not a real resource type
// (dashboard pseudo-items, collapsed-group headers).
func (m Model) openResourceTypeActionMenu() Model {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m
	}
	if sel.Kind == "__collapsed_group__" || sel.Category == "Dashboards" {
		return m
	}
	key := model.PinKeyFromRef(sel.Extra)
	if key == "" {
		return m
	}

	pinLabel, pinDesc := actionLabelPinType, "Pin this resource type to the top of the sidebar"
	if m.isTypePinned(key) {
		pinLabel, pinDesc = actionLabelUnpinType, "Remove this resource type from the Pinned section"
	}
	hideLabel, hideDesc := actionLabelHideType, "Hide this resource type from the sidebar"
	if m.isTypeHidden(key) {
		hideLabel, hideDesc = actionLabelShowType, "Show this hidden resource type again"
	}

	m.overlay = overlayAction
	m.overlayItems = []model.Item{
		{Name: pinLabel, Extra: pinDesc, Status: ui.ActiveKeybindings.PinGroup},
		{Name: hideLabel, Extra: hideDesc, Status: hideMenuChip},
	}
	m.overlayCursor = 0
	return m
}

// isTypeHidden reports whether the version-agnostic type key is hidden in the
// active scope, reading the persisted state directly rather than the
// model.HiddenTypes global so the menu label is correct even if the global is
// momentarily stale.
func (m Model) isTypeHidden(key string) bool {
	if m.hiddenState == nil {
		return false
	}
	if m.isUnionSentinel() && m.unionSetName != "" {
		return slices.Contains(m.hiddenState.UnionSets[m.unionSetName], key)
	}
	return slices.Contains(m.hiddenState.Contexts[m.nav.Context], key)
}

// isTypePinned reports whether the type key is pinned in the active scope. It
// reads the per-context / per-union-set state that handleKeyPinGroup toggles,
// so the menu label predicts what selecting the entry will do (config-level
// pins are a separate, file-managed layer and are intentionally not consulted
// here).
func (m Model) isTypePinned(key string) bool {
	if m.pinnedState == nil {
		return false
	}
	if m.isUnionSentinel() && m.unionSetName != "" {
		return slices.Contains(m.pinnedState.UnionSets[m.unionSetName], key)
	}
	return slices.Contains(m.pinnedState.Contexts[m.nav.Context], key)
}

// toggleHiddenResourceType hides or shows the resource type under the cursor,
// persists the change, and rebuilds the sidebar. Mirrors handleKeyPinGroup:
// scoped per-context (or per-union-set when a named set is active), with an
// in-memory rollback if the disk write fails.
func (m Model) toggleHiddenResourceType() (tea.Model, tea.Cmd) {
	if m.nav.Level != model.LevelResourceTypes {
		return m, nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	if sel.Kind == "__collapsed_group__" || sel.Category == "Dashboards" {
		m.setStatusMessage("Select a resource type to hide", true)
		return m, scheduleStatusClear()
	}
	key := model.PinKeyFromRef(sel.Extra)
	if key == "" {
		m.setStatusMessage("This item cannot be hidden", true)
		return m, scheduleStatusClear()
	}

	// Capture the row and its next sibling before the rebuild: when hiding
	// with reveal off the row vanishes, so the cursor follows to the sibling;
	// otherwise it stays on the (now dimmed or restored) row.
	selCopy := *sel
	nextSibling := m.nextResourceTypeCursorItem()
	// A pinned type that gets hidden disappears from the Pinned section too;
	// call that out so the type doesn't seem to vanish for no reason.
	wasPinned := sel.Category == "Pinned"

	hiddenNow := false
	var undo func()
	scopeLabel := ""
	switch {
	case m.isUnionSentinel() && m.unionSetName != "":
		hiddenNow = toggleHiddenUnionSetType(m.hiddenState, m.unionSetName, key)
		undo = func() { _ = toggleHiddenUnionSetType(m.hiddenState, m.unionSetName, key) }
		scopeLabel = " for union set " + m.unionSetName
	case m.isUnionSentinel():
		m.setStatusMessage("Hiding in union mode requires a named union set", true)
		return m, scheduleStatusClear()
	default:
		hiddenNow = toggleHiddenType(m.hiddenState, m.nav.Context, key)
		undo = func() { _ = toggleHiddenType(m.hiddenState, m.nav.Context, key) }
	}

	if err := saveHiddenTypesState(m.hiddenState); err != nil {
		undo()
		m.setStatusMessage(fmt.Sprintf("Failed to save hidden types: %v", err), true)
		return m, scheduleStatusClear()
	}

	m.applyHiddenTypes()

	// Re-sort the sidebar now so the cursor can follow; the async
	// loadResourceTypes refresh below rebuilds the same list and preserves
	// this position (setMiddleItems does not reset the cursor).
	discoveryCtx := m.nav.Context
	if m.isUnionSentinel() && len(m.unionContexts) > 0 {
		discoveryCtx = m.unionContexts[0]
	}
	if entries := m.discoveredResources[discoveryCtx]; len(entries) > 0 {
		m.setMiddleItems(model.BuildSidebarItems(entries))
		m.syncExpandedGroup()
	}

	// When reveal is off, hiding removes the row; follow to the next sibling.
	// Otherwise (showing, or hiding while revealed) the row remains in place.
	if hiddenNow && !model.ShowRareResources {
		m.focusMiddleItem(nextSibling)
	} else {
		m.focusMiddleItem(&selCopy)
	}
	m.clampCursor()

	if hiddenNow {
		pinNote := ""
		if wasPinned {
			pinNote = ", removed from Pinned"
		}
		m.setStatusMessage(fmt.Sprintf("Hidden%s: %s (%s to reveal%s)", scopeLabel, sel.Name, ui.ActiveKeybindings.ToggleRare, pinNote), false)
	} else {
		m.setStatusMessage(fmt.Sprintf("Shown%s: %s", scopeLabel, sel.Name), false)
	}
	return m, tea.Batch(m.loadResourceTypes(), scheduleStatusClear())
}
