// Package app — security_state.go
// Embedded sub-struct of Model holding the security feature's runtime
// state. Kept in a dedicated file to keep app.go under the 800-line cap
// while preserving direct field access (m.securityManager etc.) via Go's
// embedded-struct field promotion.
package app

import (
	"github.com/janosmiko/lfk/internal/security"
)

// securityModelState bundles the Model fields owned by the security
// feature. The manager and availability map are rebuilt by
// refreshSecuritySources on each cluster switch; the ignore state is
// loaded once at startup from the YAML file. securityIndex is the
// per-resource finding lookup table used by the explorer's SEC badge,
// rebuilt whenever a fresh findings fetch lands.
type securityModelState struct {
	securityManager            *security.Manager
	securityAvailabilityByName map[string]bool
	securityIgnores            *SecurityIgnoreState
	securityIndex              *security.FindingIndex
	// securityActiveGroup tracks the finding-group key the user has drilled
	// into at LevelOwned. Set when navigateChildResource transitions from a
	// __security_finding_group__ row; consumed by loadSecurityAffectedResources
	// to know which group's affected resources to fetch.
	securityActiveGroup string
	// securityActiveSource is the source of the drilled finding group (from
	// its __source__ column). The per-resource findings view mixes sources,
	// so the source can't be derived from nav.ResourceType.Kind there;
	// loadSecurityAffectedResources uses this to scope the drill-in fetch.
	securityActiveSource string
	// securityResourceFilter holds the refs (resource + owners) the
	// per-resource findings view (SecurityResourceFindingsKind) is filtered
	// to. Set by the "Security Findings" action; consumed by loadResources.
	securityResourceFilter []security.ResourceRef
	// showSecurityIgnored toggles whether ignored findings surface in the
	// explorer. Off by default; flipped with kb.SecurityIgnoreToggle.
	showSecurityIgnored bool
	// hideSecurityBadges suppresses the per-resource SEC row badge when set.
	// Off by default (badges shown); flipped with kb.SecurityBadgeToggle. A
	// global session preference, not per-tab and not persisted, mirroring
	// showRareResources — it hides only the row badge, leaving the Security
	// dashboard and source probing untouched.
	hideSecurityBadges bool
	// securityProbedContext is the cluster context whose security sources
	// have already been probed this activation. Availability probing is
	// lazy — it runs only when the user focuses the Security category, not
	// eagerly at cluster open — so this guard stops a re-probe on every
	// keystroke while focused. refreshSecuritySources clears it on context
	// switch (the manager is rebuilt), allowing a fresh probe per context.
	securityProbedContext string
}

// anySecurityAvailable reports whether at least one registered source
// returned true from its IsAvailable probe. Used as the gate for the
// SEC badge activation: when no source is available we leave the badge
// off so the explorer renders identically to clusters without security.
func (s securityModelState) anySecurityAvailable() bool {
	for _, ok := range s.securityAvailabilityByName {
		if ok {
			return true
		}
	}
	return false
}
