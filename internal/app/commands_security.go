// Package app — commands_security.go
package app

import (
	"context"
	"maps"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
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
func (m Model) handleSecurityAvailabilityLoaded(msg securityAvailabilityLoadedMsg) (Model, tea.Cmd) {
	if msg.context != m.nav.Context && m.nav.Context != "" {
		return m, nil
	}
	if m.securityAvailabilityByName == nil {
		m.securityAvailabilityByName = make(map[string]bool)
	}
	maps.Copy(m.securityAvailabilityByName, msg.availability)
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
	// Kick off per-source background scans so findings are pre-warmed
	// and users see "Scan Heuristic findings" / "Scan Trivy findings"
	// in the :tasks overlay.
	return m, m.loadSecurityFindings()
}

// securitySourceDisplayName maps an internal source name to the
// user-facing label shown in the :tasks overlay and sidebar.
func securitySourceDisplayName(name string) string {
	switch name {
	case "heuristic":
		return "Heuristic"
	case "trivy-operator":
		return "Trivy"
	case "policy-report":
		return "Kyverno"
	case "falco":
		return "Falco"
	case "kube-bench":
		return "CIS"
	default:
		return name
	}
}

// loadSecurityFindings launches per-source background scans for all
// available security sources. Each source gets its own tracked bgtask
// ("Scan Heuristic findings", "Scan Trivy findings", etc.) so users see
// individual progress in the :tasks overlay and title-bar spinner.
//
// Called by handleSecurityAvailabilityLoaded after the availability probe
// confirms which sources are reachable. The resulting findings populate
// the Manager's FindingIndex so the SEC column badge and sidebar counts
// reflect actual data, and subsequent navigations into a security source
// get a pre-warmed FetchAll cache.
func (m Model) loadSecurityFindings() tea.Cmd {
	if m.securityManager == nil {
		return nil
	}
	mgr := m.securityManager
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	var cmds []tea.Cmd
	for _, src := range mgr.Sources() {
		if !m.securityAvailabilityByName[src.Name()] {
			continue
		}
		s := src // capture for closure
		display := securitySourceDisplayName(s.Name())
		cmd := m.trackBgTask(
			bgtasks.KindResourceList,
			"Scan "+display+" findings",
			bgtaskTarget(kctx, ns),
			func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				findings, err := s.Fetch(ctx, kctx, ns)
				return securityFindingsScannedMsg{
					context:   kctx,
					namespace: ns,
					source:    s.Name(),
					findings:  findings,
					err:       err,
				}
			},
		)
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// handleSecurityFindingsScanned processes a per-source scan result.
// It aggregates findings into the Manager's FindingIndex so sidebar
// counts and SEC column badges update, and rebuilds the sidebar.
func (m Model) handleSecurityFindingsScanned(msg securityFindingsScannedMsg) Model {
	if msg.context != m.nav.Context && m.nav.Context != "" {
		return m
	}
	if msg.err != nil {
		return m
	}
	// Merge these findings into the manager's index. We rebuild the
	// full index from scratch using all cached per-source findings
	// stored in the Model. This is cheap (O(findings)) and keeps the
	// index consistent across incremental source arrivals.
	if m.securityFindingsBySource == nil {
		m.securityFindingsBySource = make(map[string][]security.Finding)
	}
	m.securityFindingsBySource[msg.source] = msg.findings

	var allFindings []security.Finding
	for _, f := range m.securityFindingsBySource {
		allFindings = append(allFindings, f...)
	}
	m.securityManager.SetIndex(security.BuildFindingIndex(allFindings))

	// Refresh sidebar so the Security source counts update.
	setSecurityHookState(m.securityManager, m.securityAvailabilityByName)
	rebuilt := m.rebuildSidebarForCurrentContext()
	switch m.nav.Level {
	case model.LevelResourceTypes:
		m.middleItems = rebuilt
		m.itemCache[m.navKey()] = m.middleItems
		m.clampCursor()
	case model.LevelResources, model.LevelOwned, model.LevelContainers:
		m.leftItems = rebuilt
		delete(m.itemCache, m.nav.Context)
	}
	return m
}

// loadSecurityAffectedResources fetches the resources affected by a specific
// finding group (identified by groupKey) and returns them as an ownedLoadedMsg
// so the existing owned-level handler places them into middleItems.
func (m Model) loadSecurityAffectedResources(groupKey string) tea.Cmd {
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	rt := m.nav.ResourceType
	gen := m.requestGen
	reqCtx := m.reqCtx
	return m.trackBgTask(
		bgtasks.KindResourceList,
		"List affected resources",
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			items, err := m.client.GetSecurityAffectedResources(reqCtx, kctx, ns, rt, groupKey)
			return ownedLoadedMsg{items: items, err: err, gen: gen}
		},
	)
}

// loadSecurityAffectedResourcesPreview is the preview variant — returns
// items as a resourcesLoadedMsg with forPreview=true so they land in
// rightItems (preview pane) instead of middleItems.
func (m Model) loadSecurityAffectedResourcesPreview(groupKey string) tea.Cmd {
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	rt := m.nav.ResourceType
	gen := m.requestGen
	reqCtx := m.reqCtx
	return m.trackBgTask(
		bgtasks.KindResourceList,
		"Preview affected resources",
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			items, err := m.client.GetSecurityAffectedResources(reqCtx, kctx, ns, rt, groupKey)
			return resourcesLoadedMsg{items: items, err: err, forPreview: true, gen: gen}
		},
	)
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
