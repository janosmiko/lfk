// Package app — commands_security.go
package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/model"
)

// loadSecurityAvailability probes each registered source's IsAvailable
// and returns a securityAvailabilityLoadedMsg with a per-source map.
// Used by the SEC column and Security category to decide what to show.
//
// The probe is tracked through the bgtasks Registry so it surfaces in the
// background tasks overlay and titlebar spinner alongside other long-running
// cluster queries. Each source's IsAvailable call can take up to 3 s, so
// users deserve to see the probe in flight.
func (m Model) loadSecurityAvailability() tea.Cmd {
	if m.securityManager == nil {
		return nil
	}
	mgr := m.securityManager
	kctx := m.nav.Context
	return m.trackBgTask(
		bgtasks.KindResourceList,
		"Security availability",
		bgtaskTarget(kctx, ""),
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			byName := make(map[string]bool)
			for _, s := range mgr.Sources() {
				ok, _ := s.IsAvailable(ctx, kctx)
				byName[s.Name()] = ok
			}
			return securityAvailabilityLoadedMsg{context: kctx, availability: byName}
		},
	)
}

// handleSecurityAvailabilityLoaded merges a per-source availability
// probe result into the Model. Stale messages (from a prior context)
// are discarded. After updating the map, the package-level hook state
// is refreshed and the sidebar columns are rebuilt so the Security
// category entries become visible (or disappear) on the next render.
//
// Which column holds the resource-types list depends on the current
// navigation level: at LevelResourceTypes it's middleItems; at
// LevelResources it's leftItems (pushLeft was called when the user
// navigated into a specific resource type). The probe usually arrives
// while the user is on one of these two levels — most commonly at
// LevelResources on cold start because the session restore lands them
// there before the probe finishes. Rebuilding only middleItems missed
// that case entirely, which is why Security entries never appeared
// when the app restarted into a resource list.
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
	// on the next BuildSidebarItems call.
	setSecurityHookState(m.securityManager, m.securityAvailabilityByName)

	rebuilt := m.rebuildSidebarForCurrentContext()

	switch m.nav.Level {
	case model.LevelResourceTypes:
		// The sidebar lives in middleItems. Update both the live view and
		// the cached copy so cursor save/restore keeps working.
		m.middleItems = rebuilt
		m.itemCache[m.navKey()] = m.middleItems
		m.clampCursor()
	case model.LevelResources, model.LevelOwned, model.LevelContainers:
		// The resource-types list was pushed onto the left column chain when
		// the user drilled deeper. Rebuild leftItems so navigating back with
		// `h` immediately shows the Security category. We also drop the
		// cached resource-types middleItems (keyed by context only) so a
		// later navigateParent rebuilds from the fresh leftItems instead of
		// serving a stale cached copy from before the probe completed.
		m.leftItems = rebuilt
		delete(m.itemCache, m.nav.Context)
	}
	return m
}

// rebuildSidebarForCurrentContext produces a fresh sidebar item list for the
// currently-selected cluster context. Callers decide which column to place
// the result into. When the context has no discovered resources yet (cold
// start before API discovery returns), the seed list is used so core K8s
// resources still render.
func (m Model) rebuildSidebarForCurrentContext() []model.Item {
	if discovered, ok := m.discoveredResources[m.nav.Context]; ok && len(discovered) > 0 {
		return model.BuildSidebarItems(discovered)
	}
	return model.BuildSidebarItems(model.SeedResources())
}
