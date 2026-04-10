// Package app — security_source_entries.go
package app

import (
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// buildSecuritySourceEntries builds the Security category entries from the
// Manager's currently registered and available sources. Called by the
// SecuritySourcesFn hook installed in NewModel.
func buildSecuritySourceEntries(mgr *security.Manager, availability map[string]bool) []model.SecuritySourceEntry {
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
		if !availability[src.Name()] {
			continue
		}
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
