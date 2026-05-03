package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// clusterColorForActiveContext returns the colour name assigned to the
// currently-active context, or "" when the user is at the cluster picker
// (no active context) or the context has no colour assigned. Callers
// (renderTitleBar, cluster picker) treat "" as "no tint".
func (m Model) clusterColorForActiveContext() string {
	if m.nav.Level == model.LevelClusters {
		return ""
	}
	if m.nav.Context == "" {
		return ""
	}
	return m.clusterColors[m.nav.Context]
}

// clusterColorPickerNoneIndex is the cursor index of the "None / clear"
// row in the picker — always one past the last named colour.
func clusterColorPickerNoneIndex() int { return len(ui.ClusterColorNames) }

// handleKeyClusterColorPicker opens the cluster-color overlay over the
// highlighted row in the cluster picker. Only valid at Level=Clusters; at
// any other level the keypress is silently ignored so users who stash
// Ctrl+L muscle memory don't get confused by half-applied state inside a
// context. Pre-seeds the cursor on the cluster's current color (or the
// "None" row when the cluster has no color set yet).
func (m Model) handleKeyClusterColorPicker() (tea.Model, tea.Cmd) {
	if m.nav.Level != model.LevelClusters {
		return m, nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	m.overlay = overlayClusterColor
	m.clusterColorOverlayContext = sel.Name
	m.clusterColorOverlayCursor = clusterColorPickerNoneIndex()
	if current, ok := m.clusterColors[sel.Name]; ok {
		for i, c := range ui.ClusterColorNames {
			if c == current {
				m.clusterColorOverlayCursor = i
				break
			}
		}
	}
	return m, nil
}

// handleClusterColorOverlayKey services key events while the picker is
// open. Up/Down move the cursor with wrap-around, Enter applies the
// highlighted color (deleting the entry when the cursor is on the "None"
// row), Esc cancels without persisting. Other keys are ignored so the
// overlay can't be dismissed by accident.
func (m Model) handleClusterColorOverlayKey(key string) (tea.Model, tea.Cmd) {
	rows := clusterColorPickerNoneIndex() + 1
	switch key {
	case "down", "j":
		m.clusterColorOverlayCursor = (m.clusterColorOverlayCursor + 1) % rows
		return m, nil
	case "up", "k":
		m.clusterColorOverlayCursor = (m.clusterColorOverlayCursor - 1 + rows) % rows
		return m, nil
	case "esc":
		m.overlay = overlayNone
		m.clusterColorOverlayContext = ""
		return m, nil
	case "enter":
		mdl, hadErr := m.applyClusterColorSelection()
		if hadErr {
			return mdl, scheduleStatusClear()
		}
		return mdl, nil
	}
	return m, nil
}

// applyClusterColorSelection writes the cursor's color to the in-memory
// map, persists to disk, and closes the overlay. The "None" row deletes
// the entry rather than writing an empty string so loadClusterColors's
// validation never sees a sentinel value. Returns hadErr=true when the
// persistence step failed so the caller can schedule a status-clear for
// the user-visible error message.
func (m Model) applyClusterColorSelection() (Model, bool) {
	ctx := m.clusterColorOverlayContext
	if ctx == "" {
		// Defensive: should never happen because handleKeyClusterColorPicker
		// only opens the overlay when a row is highlighted.
		m.overlay = overlayNone
		return m, false
	}
	if m.clusterColors == nil {
		m.clusterColors = make(map[string]string)
	}
	var newColor string
	if m.clusterColorOverlayCursor == clusterColorPickerNoneIndex() {
		delete(m.clusterColors, ctx)
	} else if m.clusterColorOverlayCursor >= 0 && m.clusterColorOverlayCursor < len(ui.ClusterColorNames) {
		newColor = ui.ClusterColorNames[m.clusterColorOverlayCursor]
		m.clusterColors[ctx] = newColor
	}
	// Stamp the row in m.middleItems by index so the swatch updates
	// immediately, without waiting for the next loadContexts roundtrip.
	// Mirrors the pattern in handleKeyReadOnlyToggle: writing through a
	// transient pointer from selectedMiddleItem could miss the cached
	// slice on the fallback path.
	for i := range m.middleItems {
		if m.middleItems[i].Name == ctx {
			m.middleItems[i].ClusterColor = newColor
			break
		}
	}
	saveErr := saveClusterColors(m.clusterColors)
	if saveErr != nil {
		// Persistence failure shouldn't trap the user inside the overlay —
		// the in-memory change still took effect for the session and
		// loadClusterColors's graceful-empty fallback covers the next run.
		logger.Warn("Failed to persist cluster color", "context", ctx, "error", saveErr)
		m.setStatusMessage("Failed to save cluster color: "+saveErr.Error(), true)
	}
	m.overlay = overlayNone
	m.clusterColorOverlayContext = ""
	return m, saveErr != nil
}
