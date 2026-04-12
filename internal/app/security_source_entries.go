// Package app — security_source_entries.go
package app

import (
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// buildSecuritySourceEntries builds the Security category entries from the
// Manager's currently registered and available sources. Called by the
// SecuritySourcesFn hook installed in NewModel.
//
// A source is included only when the per-source availability probe (run
// on cluster entry via loadSecurityAvailability) has confirmed its
// dependencies are reachable. This prevents dead entries like "Trivy (0)"
// from appearing in clusters where Trivy Operator isn't installed.
//
// The availability map starts empty at cluster entry; the probe runs
// asynchronously and the message handler rebuilds middleItems once the
// results arrive.
func buildSecuritySourceEntries(mgr *security.Manager, availability map[string]bool) []model.SecuritySourceEntry {
	if mgr == nil {
		return nil
	}
	displayByName := map[string]struct {
		display string
		icon    model.Icon
	}{
		"heuristic":      {"Heuristic", model.Icon{Unicode: "◉", Simple: "[He]", Emoji: "🛡️", NerdFont: "\U000f0483"}},
		"trivy-operator": {"Trivy", model.Icon{Unicode: "◈", Simple: "[Tr]", Emoji: "🔍", NerdFont: "\U000f0483"}},
		"policy-report":  {"Kyverno", model.Icon{Unicode: "◇", Simple: "[Ky]", Emoji: "📋", NerdFont: "\U000f0483"}},
		"kube-bench":     {"CIS", model.Icon{Unicode: "◆", Simple: "[CI]", Emoji: "📝", NerdFont: "\U000f0483"}},
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
			meta.icon = model.Icon{Unicode: "●", Simple: "[Se]", Emoji: "🔒", NerdFont: "\U000f0483"}
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
