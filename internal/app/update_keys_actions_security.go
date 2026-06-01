// Package app — update_keys_actions_security.go
// Security-related explorer action handlers. Extracted from
// update_keys_actions.go to keep that file under the revive
// file-length-limit.
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// handleExplorerSecurityViewKeys dispatches keys that are only meaningful on a
// security view. It runs before the general explorer action keys so it can
// shadow global bindings that make no sense on synthetic finding rows. It
// currently owns the show-ignored toggle (kb.SecurityIgnoreToggle), which
// reuses LabelEditor's "i" — the same context-gated reuse ClusterColorPicker
// applies to the Logs key at the cluster picker. Off a security view it
// returns handled=false with the model untouched, so the key keeps its normal
// global meaning everywhere else.
func (m Model) handleExplorerSecurityViewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if !onSecurityView(&m) {
		return m, nil, false
	}
	if msg.String() == ui.ActiveKeybindings.SecurityIgnoreToggle {
		return m.handleExplorerActionKeySecurityIgnoreToggle()
	}
	return m, nil, false
}

// handleExplorerActionKeySecurityIgnoreToggle flips the show/hide-ignored
// state, propagates it to the k8s client (which decides whether
// groupFindings filters ignored entries), invalidates the manager cache
// so the next list call re-emits the filtered set, and refreshes the
// current level so the user sees the result immediately.
func (m Model) handleExplorerActionKeySecurityIgnoreToggle() (tea.Model, tea.Cmd, bool) {
	m.showSecurityIgnored = !m.showSecurityIgnored
	if m.client != nil {
		m.client.SetShowIgnored(m.showSecurityIgnored)
	}
	if m.securityManager != nil {
		m.securityManager.Invalidate()
	}
	if m.showSecurityIgnored {
		m.setStatusMessage("Showing ignored findings", false)
	} else {
		m.setStatusMessage("Hiding ignored findings", false)
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear()), true
}

// handleExplorerActionKeySecurityBadgeToggle shows/hides the per-resource SEC
// row badge. It is a pure view toggle — no fetch, no cache bust — so the
// Security dashboard and source probing are unaffected. The view layer reads
// hideSecurityBadges into ui.ActiveSecurityBadgesHidden on the next render.
func (m Model) handleExplorerActionKeySecurityBadgeToggle() (tea.Model, tea.Cmd, bool) {
	m.hideSecurityBadges = !m.hideSecurityBadges
	if m.hideSecurityBadges {
		m.setStatusMessage("Hiding security badges", false)
	} else {
		m.setStatusMessage("Showing security badges", false)
	}
	return m, scheduleStatusClear(), true
}
