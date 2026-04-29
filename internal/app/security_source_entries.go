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
// All registered sources are shown immediately so the Security category
// appears as soon as a cluster is selected (heuristic is always
// registered when a k8s client exists). Sources that the availability
// probe later confirms are unavailable (e.g., Trivy Operator not
// installed) are removed on the next sidebar rebuild triggered by
// handleSecurityAvailabilityLoaded.
//
// When the availability map is non-empty (probe has completed), only
// available sources are shown.
func buildSecuritySourceEntries(mgr *security.Manager, availability map[string]bool) []model.SecuritySourceEntry {
	if mgr == nil {
		return nil
	}
	probeCompleted := len(availability) > 0
	displayByName := map[string]struct {
		display string
		icon    model.Icon
	}{
		"heuristic":      {"Heuristic", model.Icon{Unicode: "◉", Simple: "[He]", Emoji: "🛡️", NerdFont: "\U000f0483"}},
		"trivy-operator": {"Trivy", model.Icon{Unicode: "◈", Simple: "[Tr]", Emoji: "🔍", NerdFont: "\U000f0483"}},
		"policy-report":  {"Kyverno", model.Icon{Unicode: "◇", Simple: "[Ky]", Emoji: "📋", NerdFont: "\U000f0483"}},
		"falco":          {"Falco", model.Icon{Unicode: "◎", Simple: "[Fa]", Emoji: "🦅", NerdFont: "\U000f0483"}},
		"kube-bench":     {"CIS", model.Icon{Unicode: "◆", Simple: "[CI]", Emoji: "📝", NerdFont: "\U000f0483"}},
	}
	var entries []model.SecuritySourceEntry
	for _, src := range mgr.Sources() {
		// After the probe completes, filter to available sources only.
		if probeCompleted && !availability[src.Name()] {
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
			Count:       -1,
		})
	}
	return entries
}
