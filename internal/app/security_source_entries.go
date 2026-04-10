// Package app — security_source_entries.go
package app

import (
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// buildSecuritySourceEntries builds the Security category entries from the
// Manager's currently registered sources. Called by the SecuritySourcesFn
// hook installed in NewModel.
//
// All registered sources are shown regardless of the availability map.
// Sources whose external dependencies are missing (e.g., Trivy Operator
// CRDs not installed) will return errors at fetch time, which surface
// naturally in the findings list and the error log. Filtering them out
// at registration time would hide discoverable features from the user
// (e.g., "Trivy is an option I could enable"). The availability map is
// kept for the action menu gate and for diagnostic logging.
func buildSecuritySourceEntries(mgr *security.Manager, _ map[string]bool) []model.SecuritySourceEntry {
	if mgr == nil {
		return nil
	}
	displayByName := map[string]struct {
		display string
		icon    string
	}{
		"heuristic":      {"Heuristic", "◉"},
		"trivy-operator": {"Trivy", "◈"},
		"policy-report":  {"Kyverno", "◇"},
		"kube-bench":     {"CIS", "◆"},
	}
	idx := mgr.Index()
	var entries []model.SecuritySourceEntry
	for _, src := range mgr.Sources() {
		meta, known := displayByName[src.Name()]
		if !known {
			meta.display = src.Name()
			meta.icon = "●"
		}
		entries = append(entries, model.SecuritySourceEntry{
			DisplayName: meta.display,
			SourceName:  src.Name(),
			Icon:        meta.icon,
			Count:       idx.CountBySource(src.Name()),
		})
	}
	return entries
}
