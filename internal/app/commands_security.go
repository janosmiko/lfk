// Package app — commands_security.go
// Async commands for the security feature: source availability probe and
// per-source finding fetches. Each command is wrapped in trackBgTask so it
// shows up in the bg-tasks overlay and titlebar spinner.
package app

import (
	"context"
	"maps"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// invalidateSecurityCache busts the security manager's FetchAll cache so
// the next list call re-pulls findings from every registered source. Safe
// to call when no manager is wired (no-op).
func (m Model) invalidateSecurityCache() {
	if m.securityManager != nil {
		m.securityManager.Invalidate()
	}
}

// maybeProbeSecurityOnFocus lazily probes security source availability the
// first time the user focuses the Security category for the active context.
// Probing is deferred — it does NOT run at cluster open — so a cluster the
// user never inspects for security never pays the probe's API calls. On EKS
// those calls go through the kubeconfig's aws exec-credential plugin, which
// surfaces "SSO session expired" noise when the session has lapsed even
// though the foreground views work off a cached token. The per-context guard
// stops a re-probe on every cursor move; refreshSecuritySources clears it on
// context switch so each cluster is probed once on first Security focus.
func (m *Model) maybeProbeSecurityOnFocus() tea.Cmd {
	if m.nav.Level != model.LevelResourceTypes {
		return nil
	}
	sel := m.selectedMiddleItem()
	focused := m.expandedGroup == "Security" || (sel != nil && sel.Category == "Security")
	if !focused {
		return nil
	}
	ctx := m.nav.Context
	if ctx == "" && m.client != nil {
		ctx = m.client.CurrentContext()
	}
	if m.securityProbedContext == ctx {
		return nil
	}
	m.securityProbedContext = ctx
	return m.loadSecurityAvailability()
}

// eagerSecurityScanEnabled gates the cluster-open background findings scan.
// Enabled now that the scheduler lost-wakeup jam is fixed: at cluster open the
// SEC badges populate from a background, throttled, low-priority scan without
// the user navigating to the Security category. Set to false to fall back to
// the lazy-on-focus behavior (scan only when Security is opened). A var (not
// const) so tests can exercise both paths.
var eagerSecurityScanEnabled = true

// maybeEagerSecurityScan kicks off a background findings fetch at cluster open
// so the SEC badges populate without the user navigating to the Security
// category. It runs only when source availability is already known from the
// on-disk cache (seeded into securityAvailabilityByName by
// refreshSecuritySources, which also sets the manager's availability hint), so
// it performs NO IsAvailable probe: a previously-inspected cluster pays just
// the Fetch cost, and an as-yet-uninspected one (empty cache) no-ops here and
// stays fully lazy — preserving the EKS aws-credential-plugin behavior the
// lazy probe protects. loadSecurityFindings already returns nil when no source
// is available, so this is a thin, intent-revealing wrapper.
func (m Model) maybeEagerSecurityScan() tea.Cmd {
	if !eagerSecurityScanEnabled {
		return nil
	}
	return m.loadSecurityFindings()
}

// loadSecurityAvailability probes IsAvailable on every registered source
// for the active cluster and returns a securityAvailabilityLoadedMsg with
// the per-source result. Each source's probe runs in its own goroutine
// with an independent 3s timeout so a slow CRD discovery in one source
// does not delay or starve the others — the original sequential loop
// shared a single 3s budget across all sources, which the inline comment
// claimed was per-source but in practice was not. Returns nil when no
// manager is wired (e.g., NewModel ran without a kubeconfig).
func (m Model) loadSecurityAvailability() tea.Cmd {
	if m.securityManager == nil {
		return nil
	}
	mgr := m.securityManager
	kctx := m.nav.Context
	if kctx == "" && m.client != nil {
		kctx = m.client.CurrentContext()
	}
	return m.trackBgTask(
		scheduler.KindSecurityScan,
		"Probing security sources",
		bgtaskTarget(kctx, ""),
		func() tea.Msg {
			sources := mgr.Sources()
			byName := make(map[string]bool, len(sources))
			var mu sync.Mutex
			var wg sync.WaitGroup
			for _, s := range sources {
				wg.Add(1)
				go func(s security.SecuritySource) {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					ok, err := s.IsAvailable(ctx, kctx)
					if err != nil {
						// Transient failure (timeout, RBAC blip, API
						// server hiccup): skip the source so the
						// handler's merge keeps the previous-known
						// availability rather than wiping a
						// previously-good source off the sidebar on
						// shift+r. Sources are responsible for
						// returning (false, nil) when they have a
						// definitive "not installed" signal (NotFound
						// on the probe target) — those still update.
						logger.Info("Security source availability probe failed",
							"source", s.Name(), "context", kctx, "error", err)
						return
					}
					mu.Lock()
					byName[s.Name()] = ok
					mu.Unlock()
				}(s)
			}
			wg.Wait()
			return securityAvailabilityLoadedMsg{context: kctx, availability: byName}
		},
	)
}

// updateSecurityAvailabilityLoaded merges a probe result into the Model.
// Stale messages from a prior context are discarded. The hook state is
// refreshed so the next sidebar render filters to available sources only.
// Returns the next command to fire — a background findings fetch that
// populates the SEC badge index when at least one source is available.
func (m Model) updateSecurityAvailabilityLoaded(msg securityAvailabilityLoadedMsg) (Model, tea.Cmd) {
	if msg.context != m.nav.Context && m.nav.Context != "" {
		return m, nil
	}
	if m.securityAvailabilityByName == nil {
		m.securityAvailabilityByName = make(map[string]bool)
	}
	maps.Copy(m.securityAvailabilityByName, msg.availability)
	setSecurityHookState(m.securityManager, m.securityAvailabilityByName)
	// Publish the per-source availability hint to the manager so the
	// next FetchAll skips its own IsAvailable probe — without this the
	// manager re-fires N IsAvailable list calls on every navigation,
	// doubling API load on slow / throttled clusters and triggering
	// client-side rate limits that produce a perpetual "Scanning ..."
	// state in the right pane.
	if m.securityManager != nil {
		m.securityManager.SetAvailability(msg.context, m.securityAvailabilityByName)
	}
	// Persist so the next session's sidebar populates from the cache
	// instead of flashing the loader. Best-effort: a write failure
	// doesn't affect the live session.
	if err := updateSecurityAvailabilityCacheForContext(m.client, msg.context, m.securityAvailabilityByName); err != nil {
		logger.Warn("Failed to persist security availability cache", "context", msg.context, "error", err)
	}
	cmds := []tea.Cmd{m.loadSecurityFindings()}
	// Rebuild the sidebar so the "(probing sources...)" loader entry is
	// replaced with the now-known source entries. The hook state changed
	// in setSecurityHookState above, but middleItems was last set when
	// the loader was emitted and won't refresh until something dispatches
	// a fresh BuildSidebarItems pass — which loadResourceTypes does via
	// its resourceTypesMsg.
	if m.nav.Level == model.LevelResourceTypes {
		cmds = append(cmds, m.loadResourceTypes())
	}
	return m, tea.Batch(cmds...)
}

// loadSecurityFindings pulls findings across every available source for
// the active (context, namespace) and returns them as a
// securityFindingsLoadedMsg. The fetch uses the manager's internal cache
// so subsequent calls within refreshTTL return immediately. Returns nil
// when no manager is wired or no source is available.
func (m Model) loadSecurityFindings() tea.Cmd {
	if m.securityManager == nil || !m.anySecurityAvailable() {
		return nil
	}
	mgr := m.securityManager
	kctx := m.nav.Context
	if kctx == "" && m.client != nil {
		kctx = m.client.CurrentContext()
	}
	// Use the same namespace key as the list and preview fetches
	// (loadResources / loadSecurityAffectedResources both use
	// effectiveNamespace) so all three share one coalesced FetchAll scan
	// instead of keying the manager cache differently and forcing a
	// redundant full scan for the badge index.
	ns := m.effectiveNamespace()
	// KindSecurityScan keeps this background badge scan at PriorityLow so it
	// yields to the user's foreground resource lists, and gives it its own
	// dedupe namespace in the registry (distinct from real KindResourceList
	// loads). The new scheduler aging guarantees Low still makes progress.
	return m.trackBgTask(
		scheduler.KindSecurityScan,
		"Loading security findings",
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			res, err := mgr.FetchAll(ctx, kctx, ns)
			if err != nil {
				// Caller ctx cancelled / preempted / 30s budget elapsed. The
				// detached scan keeps running and caches its result, so a later
				// call rebuilds the index. Emit nothing rather than forwarding
				// an empty result that would wipe the existing badge index.
				return nil
			}
			return securityFindingsLoadedMsg{
				context:   kctx,
				namespace: ns,
				findings:  res.Findings,
				errors:    res.Errors,
			}
		},
	)
}

// updateSecurityFindingsLoaded rebuilds the FindingIndex consulted by
// the SEC badge renderer. Stale messages (different context OR a
// different namespace within the same context) are discarded so the
// index never carries findings from a prior cluster or a prior
// namespace selection. Per-source errors are logged but do not block
// index rebuild — partial success is the common case (e.g., Trivy
// installed but Falco missing).
func (m Model) updateSecurityFindingsLoaded(msg securityFindingsLoadedMsg) Model {
	if m.nav.Context != "" && msg.context != m.nav.Context {
		return m
	}
	// Compare against effectiveNamespace — the value loadSecurityFindings
	// now fetches with — so the guard discards stale messages by the same
	// key the fetch used (matters in all-namespaces / multi-select mode,
	// where effectiveNamespace is "" but m.namespace is not).
	if msg.namespace != m.effectiveNamespace() {
		return m
	}
	for source, err := range msg.errors {
		if err != nil {
			logger.Info("Security source fetch failed", "source", source, "context", msg.context, "error", err)
		}
	}
	m.securityIndex = security.BuildFindingIndex(msg.findings)
	return m
}

// loadSecurityAffectedResources fetches the resources affected by a
// finding group and emits them as an ownedLoadedMsg so the existing
// LevelOwned rendering picks them up. The group key comes from the
// hovered Item's Extra field for preview loads (forPreview=true) or
// from m.securityActiveGroup for drill-in loads. Returns nil when no
// manager is wired, the navigation is not a security source, or no
// group key is resolvable.
func (m Model) loadSecurityAffectedResources(forPreview bool) tea.Cmd {
	if m.securityManager == nil || m.client == nil {
		return nil
	}
	if m.nav.ResourceType.APIGroup != model.SecurityVirtualAPIGroup {
		return nil
	}
	groupKey := m.securityActiveGroup
	if forPreview {
		sel := m.selectedMiddleItem()
		if sel == nil || sel.Kind != "__security_finding_group__" {
			return nil
		}
		groupKey = sel.Extra
	}
	if groupKey == "" {
		return nil
	}
	rt := m.nav.ResourceType
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	gen := m.requestGen
	silent := m.suppressBgtasks
	reqCtx := m.reqCtx
	return m.trackBgTask(
		scheduler.KindResourceList,
		"List affected resources",
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			items, err := m.client.GetSecurityAffectedResources(reqCtx, kctx, ns, rt, groupKey)
			return ownedLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen, silent: silent}
		},
	)
}
