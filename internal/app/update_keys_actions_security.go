// Package app — update_keys_actions_security.go
// Security-related explorer action handlers. Extracted from
// update_keys_actions.go to keep that file under the revive
// file-length-limit.
package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

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
