// Package k8s — informer_watchfail.go
// Give-up policy for informer watches that keep dying on the network, as
// opposed to denyGVR's permanent verdict on an RBAC or API-surface refusal.
package k8s

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/logger"
)

// Lists and watches to one apiserver share a single HTTP/2 connection, so a
// proxy that resets the watch resets the lists beside it, and the reflector
// retries forever (issue #694). Three clears a leader election, not a bad path.
const (
	defaultMaxWatchFailures     = 3
	defaultWatchFailureCooldown = 10 * time.Minute
	// The reflector backs off to at most 30s between retries, so failures
	// from one broken connection land inside two minutes. A wider gap means
	// the watch ran in between, which is a recovery client-go never reports.
	defaultWatchFailureWindow = 2 * time.Minute
)

// blockedLocked reports whether a watch give-up still holds. Caller holds
// state.mu.
func (s *gvrAutoState) blockedLocked() bool {
	return !s.cooldownUntil.IsZero() && time.Now().Before(s.cooldownUntil)
}

// cacheBlocked reports whether the router must send (contextName, gvr)
// straight to a direct list. Covers both give-up kinds: denyGVR's permanent
// RBAC verdict and noteWatchFailure's expiring network cooldown.
func (ic *informerCache) cacheBlocked(contextName string, gvr schema.GroupVersionResource) bool {
	state := ic.getAutoState(contextName, gvr)
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.denied || state.blockedLocked()
}

// noteWatchSuccess clears the failure run once a watch syncs, so only
// consecutive failures count toward the give-up.
func (ic *informerCache) noteWatchSuccess(contextName string, gvr schema.GroupVersionResource) {
	state := ic.getAutoState(contextName, gvr)
	state.mu.Lock()
	state.watchFailures = 0
	state.lastWatchFailure = time.Time{}
	state.mu.Unlock()
}

// noteWatchFailure records a watch error no status check explains. Stopping
// the informer matters more than blocking the cache: while it lives, the
// reflector retries and re-breaks the shared connection.
func (ic *informerCache) noteWatchFailure(contextName string, gvr schema.GroupVersionResource, err error) {
	now := time.Now()
	state := ic.getAutoState(contextName, gvr)
	state.mu.Lock()
	if state.denied || state.blockedLocked() {
		state.mu.Unlock()
		return
	}
	// noteWatchSuccess only fires at the initial sync, so a stale previous
	// failure is the sole evidence that the watch recovered in between.
	if !state.lastWatchFailure.IsZero() && now.Sub(state.lastWatchFailure) > ic.watchFailureWindow {
		state.watchFailures = 0
	}
	state.lastWatchFailure = now
	state.watchFailures++
	trip := state.watchFailures >= ic.maxWatchFailures
	if trip {
		state.watchFailures = 0
		state.lastWatchFailure = time.Time{}
		state.hot = false
		state.promoted = false
		state.cooldownUntil = now.Add(ic.watchFailureCooldown)
	}
	state.mu.Unlock()

	if !trip {
		return
	}
	ic.stopOne(contextName, gvr)

	logger.WarnOnce("informer-watch-unreachable", contextName+"/"+gvr.String(),
		"Watch kept failing; falling back to direct lists for this resource",
		"context", contextName, "gvr", gvr.String(),
		"cooldown", ic.watchFailureCooldown.String(), "error", err)
}

// hasEntry reports whether a live informer exists for (contextName, gvr).
func (ic *informerCache) hasEntry(contextName string, gvr schema.GroupVersionResource) bool {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	_, ok := ic.entries[contextName][gvr]
	return ok
}
