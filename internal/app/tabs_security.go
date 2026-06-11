// Package app — tabs_security.go
// Per-tab persistence and restoration of the security-dashboard state.
// Extracted from tabs.go so the file stays under the revive
// file-length-limit.
package app

import "github.com/janosmiko/lfk/internal/security"

// saveSecurityStateToTab copies the model's per-tab security state into t.
// The map share their underlying storage with m via pointer copy; that's
// fine because mutations go through maps.Copy on a fresh map in
// updateSecurityAvailabilityLoaded. If that ever changes, this assignment
// must deep-copy too.
func (m *Model) saveSecurityStateToTab(t *TabState) {
	t.securityManager = m.securityManager
	t.securityAvailabilityByName = m.securityAvailabilityByName
	t.securityIndex = m.securityIndex
	t.securityActiveGroup = m.securityActiveGroup
	t.securityActiveSource = m.securityActiveSource
	// Copy: unlike the maps above, slices invite in-place appends; sharing
	// the backing array would let later model mutations corrupt the tab.
	t.securityResourceFilter = append([]security.ResourceRef(nil), m.securityResourceFilter...)
	t.showSecurityIgnored = m.showSecurityIgnored
}

// loadSecurityStateFromTab restores the tab's security state into the
// model and re-publishes the global hook state so the next sidebar
// render reflects this tab's cluster (rather than whatever tab was
// active when refreshSecuritySources last wrote to setSecurityHookState).
// It also mirrors the per-tab toggle on the shared k8s client so an
// active fetch sees this tab's preference, not the previous tab's.
func (m *Model) loadSecurityStateFromTab(t *TabState) {
	m.securityManager = t.securityManager
	m.securityAvailabilityByName = t.securityAvailabilityByName
	m.securityIndex = t.securityIndex
	m.securityActiveGroup = t.securityActiveGroup
	m.securityActiveSource = t.securityActiveSource
	m.securityResourceFilter = append([]security.ResourceRef(nil), t.securityResourceFilter...)
	m.showSecurityIgnored = t.showSecurityIgnored
	setSecurityHookState(m.securityManager, m.securityAvailabilityByName)
	if m.client == nil {
		return
	}
	m.client.SetShowIgnored(m.showSecurityIgnored)
	if m.securityIgnores != nil {
		m.client.SetIgnoreChecker(newModelIgnoreChecker(m.securityIgnores, m.nav.Context))
	}
}
