// Package app — security_refresh.go
// Builds a fresh security.Manager populated with the four built-in source
// adapters for the active cluster context and wires it into k8s.Client +
// the package-level hook state. Called from NewModel and on context switch.
package app

import (
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/security"
	"github.com/janosmiko/lfk/internal/security/advisor"
	"github.com/janosmiko/lfk/internal/security/falco"
	"github.com/janosmiko/lfk/internal/security/gatekeeper"
	"github.com/janosmiko/lfk/internal/security/heuristic"
	"github.com/janosmiko/lfk/internal/security/kubescape"
	"github.com/janosmiko/lfk/internal/security/policyreport"
	"github.com/janosmiko/lfk/internal/security/trivyop"
	"github.com/janosmiko/lfk/internal/ui"
)

// refreshSecuritySources rebuilds the security manager's source list for the
// currently active cluster. The manager is replaced wholesale so stale
// caches and per-context state from a prior cluster cannot linger. The
// fresh manager is wired into the k8s.Client and published to the
// package-level hook state so model.SecuritySourcesFn picks it up on the
// next sidebar render.
// The returned command, when non-nil, reads the per-host findings cache off the
// Update goroutine and emits securityFindingsSeedMsg to paint the SEC badges
// (stale-while-revalidate). Callers must dispatch it; NewModel stores it for
// Init to fire. Reading that cache inline here used to freeze the UI for seconds
// on clusters with a large findings cache.
func (m *Model) refreshSecuritySources() tea.Cmd {
	// The manager is rebuilt below, so any prior probe for this slot is moot.
	// Clearing the guard lets maybeProbeSecurityOnFocus re-probe the new (or
	// re-selected) context the next time the user focuses the Security
	// category. Without this, switching back to a previously-probed context
	// would leave the rebuilt manager's availability empty (perpetual loader).
	m.securityProbedContext = ""
	// Resolve the context once so the enable check, source gating, and the
	// disk-cache lookup below all agree (m.nav.Context is empty on the first
	// render before nav is hydrated; fall back to the client's current one).
	resolvedCtx := m.nav.Context
	if resolvedCtx == "" && m.client != nil {
		resolvedCtx = m.client.CurrentContext()
	}
	// Honour the global / per-cluster enable toggle. When disabled, tear down
	// the manager and hook state so the Security category, SEC badge, and all
	// probing stay off for this context.
	if !ui.ResolveSecurityEnabled(resolvedCtx) {
		m.securityManager = nil
		m.securityIndex = nil
		m.securityAvailabilityByName = nil
		if m.client != nil {
			m.client.SetSecurityManager(nil)
		}
		setSecurityHookState(nil, nil)
		return nil
	}
	mgr := security.NewManager()
	if m.client != nil {
		// Security sources get throttled clients (dedicated lower QPS/Burst)
		// so background finding scans run at a low rate and never drain the
		// foreground's API budget.
		kc := m.client.RawClientsetForContextThrottled(resolvedCtx)
		dc := m.client.RawDynamicForContextThrottled(resolvedCtx)
		// register adds src only when its per-source toggle is enabled for
		// this context (per-cluster override > global > enabled by default).
		register := func(name string, src security.SecuritySource) {
			if ui.ResolveSecuritySourceEnabled(resolvedCtx, name) {
				mgr.Register(src)
			}
		}
		if kc != nil {
			h := heuristic.NewWithClient(kc)
			h.SetSecretEnvPatterns(ui.ConfigSecuritySecretEnvInclude, ui.ConfigSecuritySecretEnvExclude)
			register("heuristic", h)
			register("advisor", advisor.NewWithClient(kc))
			register("falco", falco.NewWithClient(kc))
		}
		if dc != nil {
			register("trivy-operator", trivyop.NewWithDynamic(dc))
			register("policy-report", policyreport.NewWithDynamic(dc))
			register("kubescape", kubescape.NewWithDynamic(dc))
		}
		// Gatekeeper needs both clientsets — Discovery() to enumerate
		// the dynamically-generated Constraint CRDs and the dynamic
		// client to list each kind's instances.
		if kc != nil && dc != nil {
			register("gatekeeper", gatekeeper.NewWithClients(kc, dc))
		}
	}
	m.securityManager = mgr
	// Seed availability from the per-host disk cache so the sidebar
	// shows real entries immediately on subsequent runs. A nil/empty
	// result triggers the loader entry until the live probe completes.
	seedFindings := false
	if cached := loadSecurityAvailabilityCacheForContext(m.client, resolvedCtx); len(cached) > 0 {
		m.securityAvailabilityByName = cached
		// Publish the cached availability as the manager's hint so an eager
		// findings fetch (maybeEagerSecurityScan, fired at cluster open)
		// skips the per-source IsAvailable probe — that probe is the exact
		// EKS aws-credential-plugin call the lazy design avoids. With the
		// hint set, a previously-inspected cluster scans straight from Fetch
		// with no new probe; an as-yet-uninspected cluster has no cache and
		// stays fully lazy.
		mgr.SetAvailability(resolvedCtx, cached)
		// A cached availability means this cluster was inspected before, so its
		// findings cache is worth reading for the stale-while-revalidate badge
		// seed. The read itself is deferred to securityFindingsSeedCmd (below)
		// because the cache file can be tens of MB.
		seedFindings = true
	} else {
		m.securityAvailabilityByName = make(map[string]bool)
	}
	if m.client != nil {
		m.client.SetSecurityManager(mgr)
		if m.securityIgnores != nil {
			m.client.SetIgnoreChecker(newModelIgnoreChecker(m.securityIgnores, resolvedCtx))
		}
		m.client.SetShowIgnored(m.showSecurityIgnored)
	}
	// Publish to the hook state so SecuritySourcesFn reads the new data.
	setSecurityHookState(m.securityManager, m.securityAvailabilityByName)
	if !seedFindings {
		return nil
	}
	return m.securityFindingsSeedCmd(resolvedCtx, m.effectiveNamespace())
}

// securityFindingsSeedCmd reads the per-host findings cache and builds the SEC
// badge index OFF the Update goroutine, delivering the result as a
// securityFindingsSeedMsg. The per-host cache file can be tens of MB on
// clusters with many findings; decoding it inline (the previous behavior in
// refreshSecuritySources) froze the UI for seconds at startup and on
// context/tab switch. Returns nil when no client is wired. A cache miss yields
// a nil message, which Bubble Tea drops.
func (m *Model) securityFindingsSeedCmd(resolvedCtx, namespace string) tea.Cmd {
	client := m.client
	if client == nil {
		return nil
	}
	return func() tea.Msg {
		// Reading a large cache file tokenizes the whole thing, briefly
		// allocating well beyond the live retained index — even on a TTL miss,
		// where nothing is returned. Hand those transient pages back to the OS
		// now rather than waiting on the slow background scavenger (issue #387).
		// Deferred so it covers both the miss and hit paths; runs off the Update
		// goroutine, so the GC pause is invisible to the UI.
		if securityFindingsCacheFileSizeForContext(client, resolvedCtx) > securityFindingsCacheReleaseThreshold {
			defer debug.FreeOSMemory()
		}
		cached := loadSecurityFindingsCacheForContext(client, resolvedCtx, namespace, securityFindingsCacheTTL)
		if cached == nil {
			return nil
		}
		return securityFindingsSeedMsg{
			context:   resolvedCtx,
			namespace: namespace,
			index:     security.BuildFindingIndex(cached),
		}
	}
}
