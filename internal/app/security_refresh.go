// Package app — security_refresh.go
// Builds a fresh security.Manager populated with the four built-in source
// adapters for the active cluster context and wires it into k8s.Client +
// the package-level hook state. Called from NewModel and on context switch.
package app

import (
	"github.com/janosmiko/lfk/internal/security"
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
func (m *Model) refreshSecuritySources() {
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
		return
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
			register("heuristic", heuristic.NewWithClient(kc))
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
		// Stale-while-revalidate: paint SEC badges instantly from the
		// last session's findings while the eager scan revalidates in the
		// background (maybeEagerSecurityScan, fired at cluster open, replaces
		// the index when fresh results land). Keyed by the effective
		// namespace so the seed matches what the scan will fetch. A miss
		// (no cache / expired) just leaves the index nil until the scan
		// completes — the pre-cache behavior.
		if cachedFindings := loadSecurityFindingsCacheForContext(
			m.client, resolvedCtx, m.effectiveNamespace(), securityFindingsCacheTTL,
		); cachedFindings != nil {
			m.securityIndex = security.BuildFindingIndex(cachedFindings)
		}
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
}
