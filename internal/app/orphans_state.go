package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/k8s"
)

// orphanKind is the kind chip filter at the top of the orphan overlay.
// Tab cycles through the values in order.
type orphanKind int

const (
	orphanKindAll orphanKind = iota
	orphanKindPod
	orphanKindSecret
	orphanKindConfigMap
	orphanKindService
	orphanKindPVC
	orphanKindHPA
	orphanKindPDB
	orphanKindNetworkPolicy
	// orphanKindRoles bundles Role + ClusterRole into one chip — both
	// answer the "this role isn't bound by anything" question with
	// the same Reason ("no binding"), and the Kind column already
	// distinguishes them in the row. Same logic for orphanKindBindings
	// (RoleBinding + ClusterRoleBinding). Keeping them as separate
	// chips would split a small list across two cycles for no UX gain.
	orphanKindRoles
	orphanKindBindings
	// orphanKindMax must follow the last real kind so cycling logic
	// can use it as the modulo base. Keep it last; any new kind goes
	// above it.
	orphanKindMax
)

// orphanState bundles the orphan-overlay UI state. Lives on Model so the
// overlay survives Tab/Shift+Tab / cursor moves without losing scroll
// position.
//
// strict gates the lenient-only items (LenientOnly=true on OrphanItem)
// — Secrets/ConfigMaps that no live Pod / Ingress / SA references but a
// workload template still does (e.g. a CronJob between firings). When
// strict=true (default) those items are hidden so the overlay shows
// "what's truly unused"; flipping to strict=false surfaces them so the
// user can audit "what's currently idle". The `s` keybinding toggles
// the field; see handleOrphansKey.
type orphanState struct {
	loading      bool
	report       k8s.OrphanReport
	partial      error // non-nil => render banner row
	visibleKind  orphanKind
	cursor       int
	scroll       int
	filter       TextInput
	filterActive bool
	strict       bool
}

// visibleItems returns the orphan rows currently visible in the overlay
// after applying the kind chip, the strict-mode filter (drops
// LenientOnly items when strict=true), and (if active) the search filter.
func (s orphanState) visibleItems() []k8s.OrphanItem {
	var pool []k8s.OrphanItem
	switch s.visibleKind {
	case orphanKindAll:
		pool = append(pool, s.report.Pods...)
		pool = append(pool, s.report.Secrets...)
		pool = append(pool, s.report.ConfigMaps...)
		pool = append(pool, s.report.Services...)
		pool = append(pool, s.report.PVCs...)
		pool = append(pool, s.report.HPAs...)
		pool = append(pool, s.report.PDBs...)
		pool = append(pool, s.report.NetworkPolicies...)
		pool = append(pool, s.report.Roles...)
		pool = append(pool, s.report.ClusterRoles...)
		pool = append(pool, s.report.RoleBindings...)
		pool = append(pool, s.report.ClusterRoleBindings...)
	case orphanKindPod:
		pool = s.report.Pods
	case orphanKindSecret:
		pool = s.report.Secrets
	case orphanKindConfigMap:
		pool = s.report.ConfigMaps
	case orphanKindService:
		pool = s.report.Services
	case orphanKindPVC:
		pool = s.report.PVCs
	case orphanKindHPA:
		pool = s.report.HPAs
	case orphanKindPDB:
		pool = s.report.PDBs
	case orphanKindNetworkPolicy:
		pool = s.report.NetworkPolicies
	case orphanKindRoles:
		pool = append(pool, s.report.Roles...)
		pool = append(pool, s.report.ClusterRoles...)
	case orphanKindBindings:
		pool = append(pool, s.report.RoleBindings...)
		pool = append(pool, s.report.ClusterRoleBindings...)
	}
	if s.strict {
		strict := make([]k8s.OrphanItem, 0, len(pool))
		for _, it := range pool {
			if !it.LenientOnly {
				strict = append(strict, it)
			}
		}
		pool = strict
	}
	if !s.filterActive && s.filter.Value == "" {
		return pool
	}
	q := s.filter.Value
	out := make([]k8s.OrphanItem, 0, len(pool))
	for _, it := range pool {
		if matchesOrphanFilter(it, q) {
			out = append(out, it)
		}
	}
	return out
}

// orphanKindCount returns the count for a given kind chip respecting
// the current strict-mode filter — chips show "Pods (3)" with a count
// that matches what the user will actually see when they Tab to that
// chip.
func (s orphanState) orphanKindCount(k orphanKind) int {
	var pool []k8s.OrphanItem
	switch k {
	case orphanKindPod:
		pool = s.report.Pods
	case orphanKindSecret:
		pool = s.report.Secrets
	case orphanKindConfigMap:
		pool = s.report.ConfigMaps
	case orphanKindService:
		pool = s.report.Services
	case orphanKindPVC:
		pool = s.report.PVCs
	case orphanKindHPA:
		pool = s.report.HPAs
	case orphanKindPDB:
		pool = s.report.PDBs
	case orphanKindNetworkPolicy:
		pool = s.report.NetworkPolicies
	case orphanKindRoles:
		pool = append(pool, s.report.Roles...)
		pool = append(pool, s.report.ClusterRoles...)
	case orphanKindBindings:
		pool = append(pool, s.report.RoleBindings...)
		pool = append(pool, s.report.ClusterRoleBindings...)
	default:
		return s.orphanKindCount(orphanKindPod) +
			s.orphanKindCount(orphanKindSecret) +
			s.orphanKindCount(orphanKindConfigMap) +
			s.orphanKindCount(orphanKindService) +
			s.orphanKindCount(orphanKindPVC) +
			s.orphanKindCount(orphanKindHPA) +
			s.orphanKindCount(orphanKindPDB) +
			s.orphanKindCount(orphanKindNetworkPolicy) +
			s.orphanKindCount(orphanKindRoles) +
			s.orphanKindCount(orphanKindBindings)
	}
	if !s.strict {
		return len(pool)
	}
	n := 0
	for _, it := range pool {
		if !it.LenientOnly {
			n++
		}
	}
	return n
}

func matchesOrphanFilter(it k8s.OrphanItem, query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(it.Namespace), q) ||
		strings.Contains(strings.ToLower(it.Name), q)
}
