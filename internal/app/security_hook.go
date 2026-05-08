// Package app — security_hook.go
// Package-level state read by model.SecuritySourcesFn on every sidebar
// render. We cannot take &m.field for the closure because Bubbletea passes
// Model by value, which leaves the hook pointing at a stale Model after
// the first Update cycle (Trivy entries persist across cluster switches,
// heuristic counts go stale). The Model and refreshSecuritySources publish
// the current cluster's manager and availability map under a mutex.
package app

import (
	"sync"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

var (
	securityHookMu           sync.RWMutex
	securityHookManager      *security.Manager
	securityHookAvailability map[string]bool
)

// setSecurityHookState publishes the currently-active manager and
// availability map so SecuritySourcesFn reads them on the next render.
// Safe to call from any goroutine. Pass nil to clear.
func setSecurityHookState(mgr *security.Manager, avail map[string]bool) {
	securityHookMu.Lock()
	defer securityHookMu.Unlock()
	securityHookManager = mgr
	securityHookAvailability = avail
}

// currentSecurityHookState returns a snapshot of the current hook state.
// Callers must not mutate the returned map.
func currentSecurityHookState() (*security.Manager, map[string]bool) {
	securityHookMu.RLock()
	defer securityHookMu.RUnlock()
	return securityHookManager, securityHookAvailability
}

// installSecuritySourcesHook wires model.SecuritySourcesFn to read from the
// package-level hook state. Called once at startup. The closure consults
// currentSecurityHookState on every call so live cluster switches surface
// without re-installing the hook.
func installSecuritySourcesHook() {
	model.SecuritySourcesFn = func() []model.SecuritySourceEntry {
		mgr, avail := currentSecurityHookState()
		if mgr == nil {
			return nil
		}
		return buildSecuritySourceEntries(mgr, avail)
	}
}
