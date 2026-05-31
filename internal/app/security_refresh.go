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
	mgr := security.NewManager()
	if m.client != nil {
		kctx := m.nav.Context
		if kctx == "" {
			kctx = m.client.CurrentContext()
		}
		kc := m.client.RawClientsetForContext(kctx)
		dc := m.client.RawDynamicForContext(kctx)
		if kc != nil {
			mgr.Register(heuristic.NewWithClient(kc))
			mgr.Register(falco.NewWithClient(kc))
		}
		if dc != nil {
			mgr.Register(trivyop.NewWithDynamic(dc))
			mgr.Register(policyreport.NewWithDynamic(dc))
			mgr.Register(kubescape.NewWithDynamic(dc))
		}
		// Gatekeeper needs both clientsets — Discovery() to enumerate
		// the dynamically-generated Constraint CRDs and the dynamic
		// client to list each kind's instances.
		if kc != nil && dc != nil {
			mgr.Register(gatekeeper.NewWithClients(kc, dc))
		}
	}
	m.securityManager = mgr
	// Reuse the kctx resolved above so the cache lookup honours the
	// current-context fallback. Passing m.nav.Context directly would skip
	// the cache when nav.Context is unset (first render before nav is
	// hydrated), which forces a full probe even though the disk cache is
	// keyed by the real cluster host.
	resolvedCtx := m.nav.Context
	if resolvedCtx == "" && m.client != nil {
		resolvedCtx = m.client.CurrentContext()
	}
	// Seed availability from the per-host disk cache so the sidebar
	// shows real entries immediately on subsequent runs. A nil/empty
	// result triggers the loader entry until the live probe completes.
	if cached := loadSecurityAvailabilityCacheForContext(m.client, resolvedCtx); len(cached) > 0 {
		m.securityAvailabilityByName = cached
	} else {
		m.securityAvailabilityByName = make(map[string]bool)
	}
	if m.client != nil {
		m.client.SetSecurityManager(mgr)
		if m.securityIgnores != nil {
			m.client.SetIgnoreChecker(&modelIgnoreChecker{
				state: m.securityIgnores,
				ctx:   resolvedCtx,
			})
		}
		m.client.SetShowIgnored(m.showSecurityIgnored)
	}
	// Publish to the hook state so SecuritySourcesFn reads the new data.
	setSecurityHookState(m.securityManager, m.securityAvailabilityByName)
}
