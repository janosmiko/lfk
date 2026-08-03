package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// executeOrphansCommand handles the :orphans builtin. No-arg form opens the
// cluster-wide orphan overlay. Kind-arg form (`:orphans pods`, `:orphans
// secrets`, etc.) jumps to that resource list with the orphan filter preset
// active. `kind` is matched case-insensitively against a fixed alias table so
// users can type the singular, plural, or short form interchangeably.
func (m Model) executeOrphansCommand(arg string) (tea.Model, tea.Cmd) {
	if m.isUnionSentinel() {
		m.setStatusMessage("Orphans requires a single cluster", true)
		return m, scheduleStatusClear()
	}
	if arg == "" {
		mt, cmd := m.openOrphansOverlay()
		return mt, cmd
	}

	kind, ok := orphanKindFromAlias(arg)
	if !ok {
		m.setStatusMessage(fmt.Sprintf(
			"unknown kind: %s (try pods/secrets/configmaps/services/pvcs/hpas/pdbs/netpols/roles/clusterroles/rolebindings/clusterrolebindings)",
			arg), true)
		return m, scheduleStatusClear()
	}

	// Jump to the kind's list, then activate the orphan filter preset
	// and fire cmdLoadOrphans for the active namespace.
	mt, jumpCmd := m.executeResourceJump(kind.resource)
	mModel, ok := mt.(Model)
	if !ok {
		return mt, jumpCmd
	}
	key := orphanCacheKey{kubeContext: mModel.nav.Context, namespace: mModel.orphanCacheNamespace()}
	presets := builtinFilterPresetsWithOrphans(kind.lfkKind, mModel.orphanCache, key)
	preset := findOrphanPreset(presets, kind.lfkKind)
	if preset == nil {
		return mModel, jumpCmd
	}
	p := *preset
	mModel.activeFilterPreset = &p
	// When the orphan cache is cold, the matcher is initially empty
	// and the list will paint as "no matches" until DetectOrphans
	// returns. Surface a status message so the user knows a scan is
	// running instead of staring at an empty filtered list — the
	// matcher rebuilds itself the moment the cache slot is populated
	// (see orphanMatcher's lazy memoization).
	if mModel.orphanCache[key] == nil {
		mModel.setStatusMessage(fmt.Sprintf("Scanning for %s orphans...", kind.lfkKind), false)
	}
	loadCmd := (&mModel).cmdLoadOrphans(key)
	return mModel, tea.Batch(jumpCmd, loadCmd, scheduleStatusClear())
}

// orphanKindAlias maps a user-supplied alias to the lfk model Kind name and
// the plural resource name used by executeResourceJump.
type orphanKindAlias struct {
	lfkKind  string // model Kind ("Pod", "Secret", "ConfigMap", "Service")
	resource string // input to executeResourceJump ("pods", "secrets", ...)
}

// orphanKindFromAlias resolves a user-typed kind token to an orphanKindAlias.
// Matching is case-insensitive and accepts singular, plural, and short forms.
// Coverage matches the orphan detector — every kind the cluster overlay
// surfaces also has a per-list `:orphans <kind>` entry point.
func orphanKindFromAlias(s string) (orphanKindAlias, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "pod", "pods", "po":
		return orphanKindAlias{"Pod", "pods"}, true
	case "secret", "secrets", "sec":
		return orphanKindAlias{"Secret", "secrets"}, true
	case "configmap", "configmaps", "cm":
		return orphanKindAlias{"ConfigMap", "configmaps"}, true
	case "service", "services", "svc":
		return orphanKindAlias{"Service", "services"}, true
	case "pvc", "pvcs", "persistentvolumeclaim", "persistentvolumeclaims":
		return orphanKindAlias{"PersistentVolumeClaim", "persistentvolumeclaims"}, true
	case "hpa", "hpas", "horizontalpodautoscaler", "horizontalpodautoscalers":
		return orphanKindAlias{"HorizontalPodAutoscaler", "horizontalpodautoscalers"}, true
	case "pdb", "pdbs", "poddisruptionbudget", "poddisruptionbudgets":
		return orphanKindAlias{"PodDisruptionBudget", "poddisruptionbudgets"}, true
	case "netpol", "netpols", "networkpolicy", "networkpolicies":
		return orphanKindAlias{"NetworkPolicy", "networkpolicies"}, true
	case "role", "roles":
		return orphanKindAlias{"Role", "roles"}, true
	case "clusterrole", "clusterroles":
		return orphanKindAlias{"ClusterRole", "clusterroles"}, true
	case "rolebinding", "rolebindings", "rb":
		return orphanKindAlias{"RoleBinding", "rolebindings"}, true
	case "clusterrolebinding", "clusterrolebindings", "crb":
		return orphanKindAlias{"ClusterRoleBinding", "clusterrolebindings"}, true
	}
	return orphanKindAlias{}, false
}

// findOrphanPreset returns a pointer to the first preset whose Name matches
// the canonical orphan preset name for the given Kind, or nil if not found.
func findOrphanPreset(presets []FilterPreset, kind string) *FilterPreset {
	want := orphanPresetNameForKind(kind)
	for i := range presets {
		if presets[i].Name == want {
			return &presets[i]
		}
	}
	return nil
}

// orphanPresetNameForKind returns the preset Name used by the orphan filter
// preset for the given Kind. The names must match those defined in filters.go.
func orphanPresetNameForKind(kind string) string {
	switch kind {
	case "Pod":
		return "Orphans"
	case "Service":
		return "No Endpoints"
	case "Secret", "ConfigMap":
		return "Unmounted"
	case "PersistentVolumeClaim":
		return "Unused"
	case "HorizontalPodAutoscaler",
		"PodDisruptionBudget",
		"NetworkPolicy",
		"RoleBinding",
		"ClusterRoleBinding":
		return "Dangling"
	case "Role", "ClusterRole":
		return "Unbound"
	}
	return "Unmounted"
}
