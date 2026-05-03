package app

import (
	"strings"

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

// filteredClusterColorNames returns the colour-name list filtered by the
// current overlay filter input. Filter is a case-insensitive substring
// match. Empty filter returns all names. The "None" row is appended by
// the caller — it stays anchored at the bottom regardless of the
// filter so users can always reach the clear-action.
func (m Model) filteredClusterColorNames() []string {
	q := strings.ToLower(strings.TrimSpace(m.clusterColorFilter.Value))
	if q == "" {
		out := make([]string, len(ui.ClusterColorNames))
		copy(out, ui.ClusterColorNames)
		return out
	}
	out := make([]string, 0, len(ui.ClusterColorNames))
	for _, n := range ui.ClusterColorNames {
		if strings.Contains(n, q) {
			out = append(out, n)
		}
	}
	return out
}

// clusterColorOverlayRowCount returns the total number of rows the
// overlay will render: filtered colours plus the "None" row.
func (m Model) clusterColorOverlayRowCount() int {
	return len(m.filteredClusterColorNames()) + 1
}

// clusterColorOverlayNoneIndex returns the cursor index of the "None"
// row in the *currently filtered* view (always one past the last
// matching colour).
func (m Model) clusterColorOverlayNoneIndex() int {
	return len(m.filteredClusterColorNames())
}

// handleKeyClusterColorPicker opens the cluster-color overlay over the
// highlighted row in the cluster picker. Only valid at Level=Clusters;
// at any other level the keypress is silently ignored so users who
// stash the muscle memory don't get confused by half-applied state
// inside a context. Pre-seeds the cursor on the cluster's current
// colour (or the "None" row when the cluster has no colour set yet),
// and clears any leftover filter from a previous open.
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
	m.clusterColorFilter.Clear()
	m.clusterColorFilterMode = false
	m.clusterColorOverlayCursor = m.clusterColorOverlayNoneIndex()
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
// open. Splits between filter-input mode (typing into the / filter)
// and normal mode (navigation / selection). The hint bar sits on the
// status bar via overlayHintBarSelector — no inline hints in the
// overlay box itself.
func (m Model) handleClusterColorOverlayKey(key string) (tea.Model, tea.Cmd) {
	if m.clusterColorFilterMode {
		return m.handleClusterColorOverlayFilterKey(key), nil
	}
	rows := m.clusterColorOverlayRowCount()
	switch key {
	case "down", "j":
		if rows > 0 {
			m.clusterColorOverlayCursor = (m.clusterColorOverlayCursor + 1) % rows
		}
		return m, nil
	case "up", "k":
		if rows > 0 {
			m.clusterColorOverlayCursor = (m.clusterColorOverlayCursor - 1 + rows) % rows
		}
		return m, nil
	case "/":
		m.clusterColorFilterMode = true
		return m, nil
	case "esc":
		// First Esc clears an active filter; second Esc closes the overlay
		// (mirrors the colorscheme overlay pattern so muscle memory carries
		// across pickers).
		if m.clusterColorFilter.Value != "" {
			m.clusterColorFilter.Clear()
			m.clusterColorOverlayCursor = m.clusterColorOverlayNoneIndex()
			return m, nil
		}
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

// handleClusterColorOverlayFilterKey processes keystrokes while the
// user is typing into the / filter input. Enter / Esc exit the filter
// mode (Enter keeps the current filter, Esc clears it); other keys
// edit the buffer via the shared FilterInput helper.
func (m Model) handleClusterColorOverlayFilterKey(key string) Model {
	action := handleFilterKey(&m.clusterColorFilter, key)
	switch action {
	case filterContinue, filterNavigate:
		// Reset cursor to the first row of the new filtered view so the
		// highlight doesn't land on a stale index when the list shrinks.
		m.clusterColorOverlayCursor = 0
	case filterAccept:
		m.clusterColorFilterMode = false
	case filterEscape:
		m.clusterColorFilter.Clear()
		m.clusterColorFilterMode = false
		m.clusterColorOverlayCursor = m.clusterColorOverlayNoneIndex()
	case filterClose:
		m.clusterColorFilter.Clear()
		m.clusterColorFilterMode = false
		m.overlay = overlayNone
		m.clusterColorOverlayContext = ""
	}
	return m
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
	filtered := m.filteredClusterColorNames()
	noneIdx := len(filtered)
	var newColor string
	if m.clusterColorOverlayCursor == noneIdx {
		delete(m.clusterColors, ctx)
	} else if m.clusterColorOverlayCursor >= 0 && m.clusterColorOverlayCursor < len(filtered) {
		newColor = filtered[m.clusterColorOverlayCursor]
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
	m.clusterColorFilter.Clear()
	m.clusterColorFilterMode = false
	return m, saveErr != nil
}
