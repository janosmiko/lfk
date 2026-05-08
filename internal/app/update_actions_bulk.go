package app

import (
	"fmt"

	"github.com/janosmiko/lfk/internal/model"
)

// openBulkSelectionMenu builds and shows the action overlay for multi-selected items.
// Extracted from openActionMenu to keep that function's cyclomatic complexity under 30.
func (m Model) openBulkSelectionMenu() Model {
	selectedList := m.selectedItemsList()
	if len(selectedList) == 0 {
		return m
	}
	// Validate the resource kind BEFORE flipping bulk-mode state. An
	// empty kind (selection spanned heterogeneous kinds, or the kind
	// resolver couldn't classify) used to leave m.bulkMode=true and
	// m.bulkItems set even though the function returned early —
	// downstream key handlers would then think a bulk operation was
	// in progress for an empty action menu.
	kind := m.selectedResourceKind()
	if kind == "" {
		return m
	}
	m.bulkMode = true
	m.bulkItems = selectedList
	m.actionCtx = m.buildActionCtx(&selectedList[0], kind)

	actions := model.ActionsForBulk(kind)
	// Filter out actions that don't apply to the selected resource kind.
	if !model.IsScaleableKind(kind) || !model.IsRestartableKind(kind) {
		filtered := actions[:0]
		for _, a := range actions {
			if a.Label == "Scale" && !model.IsScaleableKind(kind) {
				continue
			}
			if a.Label == "Restart" && !model.IsRestartableKind(kind) {
				continue
			}
			filtered = append(filtered, a)
		}
		actions = filtered
	}
	var items []model.Item
	for _, a := range actions {
		if m.readOnly && isMutatingAction(a.Label) {
			continue
		}
		items = append(items, model.Item{
			Name:   a.Label,
			Extra:  fmt.Sprintf("%s (%d items)", a.Description, len(selectedList)),
			Status: a.Key,
		})
	}

	m.overlay = overlayAction
	m.overlayItems = items
	m.overlayCursor = 0
	return m
}
