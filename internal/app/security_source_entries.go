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
// While the availability probe is still in flight (availability map empty),
// the function returns a single loader-sentinel entry so the sidebar
// shows a stable "(probing sources...)" row until we know which sources
// are actually installed in the cluster. The previous behaviour eagerly
// listed every registered source (Trivy, Heuristic, Falco, Kyverno) and
// then "shrunk" the list once the probe landed — confusing users on
// clusters where only Heuristic was available.
//
// Once the probe completes (availability map non-empty), only sources the
// probe confirmed available are shown.
func buildSecuritySourceEntries(mgr *security.Manager, availability map[string]bool) []model.SecuritySourceEntry {
	if mgr == nil {
		return nil
	}
	// No registered sources (e.g. every source disabled per-config): keep the
	// Security category empty rather than showing a loader that never resolves.
	if len(mgr.Sources()) == 0 {
		return nil
	}
	if len(availability) == 0 {
		// Probe in flight — surface a single non-navigable loader entry
		// so users see "(probing security sources...)" instead of an
		// empty Security category that pops in mid-navigation. The
		// sentinel SourceName "" is recognised by injectSecuritySourceItems
		// which builds the loader Item with SecurityLoaderKind.
		return []model.SecuritySourceEntry{{
			DisplayName: "(probing sources...)",
			SourceName:  "",
			Icon:        model.Icon{Unicode: "…", Simple: "[..]", Emoji: "⌛", NerdFont: "\U000f04bd"},
			Count:       -1,
		}}
	}
	displayByName := map[string]struct {
		display string
		icon    model.Icon
	}{
		"heuristic":      {"Heuristic", model.Icon{Unicode: "◉", Simple: "[He]", Emoji: "🧠", NerdFont: "\U000f0483"}},
		"advisor":        {"Advisor", model.Icon{Unicode: "◌", Simple: "[Ad]", Emoji: "💡", NerdFont: "\U000f0483"}},
		"rbac":           {"RBAC", model.Icon{Unicode: "◒", Simple: "[Rb]", Emoji: "🔑", NerdFont: "\U000f0483"}},
		"trivy-operator": {"Trivy", model.Icon{Unicode: "◈", Simple: "[Tr]", Emoji: "🔍", NerdFont: "\U000f0483"}},
		"policy-report":  {"Kyverno", model.Icon{Unicode: "◇", Simple: "[Ky]", Emoji: "📃", NerdFont: "\U000f0483"}},
		"falco":          {"Falco", model.Icon{Unicode: "◎", Simple: "[Fa]", Emoji: "🦅", NerdFont: "\U000f0483"}},
		"kube-bench":     {"CIS", model.Icon{Unicode: "◆", Simple: "[CI]", Emoji: "✅", NerdFont: "\U000f0483"}},
		"gatekeeper":     {"Gatekeeper", model.Icon{Unicode: "◔", Simple: "[Gk]", Emoji: "🚦", NerdFont: "\U000f0483"}},
		"kubescape":      {"Kubescape", model.Icon{Unicode: "◐", Simple: "[Ks]", Emoji: "🔬", NerdFont: "\U000f0483"}},
	}
	var entries []model.SecuritySourceEntry
	for _, src := range mgr.Sources() {
		if !availability[src.Name()] {
			continue
		}
		meta, known := displayByName[src.Name()]
		if !known {
			meta.display = src.Name()
			meta.icon = model.Icon{Unicode: "●", Simple: "[Se]", Emoji: "🔎", NerdFont: "\U000f0483"}
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
