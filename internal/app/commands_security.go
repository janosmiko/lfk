// Package app — commands_security.go
package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
)

// loadSecurityAvailability probes each registered source's IsAvailable
// and returns a securityAvailabilityLoadedMsg with a per-source map.
// Used by the SEC column and Security category to decide what to show.
func (m Model) loadSecurityAvailability() tea.Cmd {
	if m.securityManager == nil {
		return nil
	}
	mgr := m.securityManager
	kctx := m.nav.Context
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		byName := make(map[string]bool)
		for _, s := range mgr.Sources() {
			ok, _ := s.IsAvailable(ctx, kctx)
			byName[s.Name()] = ok
		}
		return securityAvailabilityLoadedMsg{context: kctx, availability: byName}
	}
}

// handleSecurityAvailabilityLoaded merges a per-source availability
// probe result into the Model. Stale messages (from a prior context)
// are discarded. After updating the map, the package-level hook state
// is refreshed and the middle-column items are rebuilt so the Security
// category entries become visible (or disappear) on the next render.
func (m Model) handleSecurityAvailabilityLoaded(msg securityAvailabilityLoadedMsg) Model {
	if msg.context != m.nav.Context && m.nav.Context != "" {
		return m
	}
	if m.securityAvailabilityByName == nil {
		m.securityAvailabilityByName = make(map[string]bool)
	}
	for k, v := range msg.availability {
		m.securityAvailabilityByName[k] = v
	}
	// Publish the updated availability map so SecuritySourcesFn reads it
	// on the next TopLevelResourceTypes call.
	setSecurityHookState(m.securityManager, m.securityAvailabilityByName)
	// Rebuild middleItems if we're at LevelResourceTypes so the newly-
	// available Security entries appear immediately. The old cached
	// middleItems were built with an empty availability map and don't
	// include the Security entries.
	if m.nav.Level == model.LevelResourceTypes {
		if crds, ok := m.discoveredCRDs[m.nav.Context]; ok && len(crds) > 0 {
			m.middleItems = model.MergeWithCRDs(crds)
		} else {
			m.middleItems = model.FlattenedResourceTypes()
		}
		m.itemCache[m.navKey()] = m.middleItems
		m.clampCursor()
	}
	return m
}
