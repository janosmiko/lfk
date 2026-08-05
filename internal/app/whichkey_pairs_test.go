package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// Guards for wkAction.Pair — the "both halves of a bidirectional pair are
// offered together, or neither is" rule. See the Pair field's own comment for
// why the pairing is declared rather than inferred.

// wkPairScenarios is the model set the both-or-neither sweep runs each catalog
// against. Viewers reuse wkViewerScenarios (which already carries the severity
// floor and ceiling, the two states the rule exists for); the explorer gets its
// own level walk, because it is deliberately absent from
// whichKeyViewerCatalogs and would otherwise be swept by nothing.
func wkPairScenarios(t *testing.T, mode viewMode) []struct {
	name  string
	build func() Model
} {
	t.Helper()
	type scenario = struct {
		name  string
		build func() Model
	}
	if mode != modeExplorer {
		return wkViewerScenarios(t, mode)
	}
	levels := map[string]model.Level{
		"clusters":   model.LevelClusters,
		"types":      model.LevelResourceTypes,
		"resources":  model.LevelResources,
		"owned":      model.LevelOwned,
		"containers": model.LevelContainers,
	}
	out := make([]scenario, 0, len(levels))
	for name, lvl := range levels {
		out = append(out, scenario{
			name: name,
			build: func() Model {
				m := whichKeyTestModel()
				m.nav.Level = lvl
				return m
			},
		})
	}
	return out
}

// TestWhichKeyCatalogs_PairDeclarationsAreWellFormed checks the declaration
// itself before anything reads it: a Pair name with one entry is a typo that
// would make the sweep below vacuously true, and a name with three is not a
// pair.
func TestWhichKeyCatalogs_PairDeclarationsAreWellFormed(t *testing.T) {
	for _, mc := range whichKeyCatalogList {
		t.Run(mc.name, func(t *testing.T) {
			counts := map[string]int{}
			for _, e := range mc.catalog.entries() {
				if e.Pair != "" {
					counts[e.Pair]++
				}
			}
			for name, n := range counts {
				if n != 2 {
					t.Errorf("pair %q has %d entries; a bidirectional pair is exactly two halves", name, n)
				}
			}
		})
	}
}

// TestWhichKeyCatalogs_BidirectionalPairsAppearTogether is the forcing function
// behind the USER DECISION recorded on wkAction.Pair. Re-adding a clamp to one
// half of a pair fails here: at the value's limit the twin is still offered and
// the clamped half is not, which is precisely the asymmetry that made
// kb.SeverityDown undiscoverable.
func TestWhichKeyCatalogs_BidirectionalPairsAppearTogether(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	for _, mc := range whichKeyCatalogList {
		declared := map[string]int{}
		for _, e := range mc.catalog.entries() {
			if e.Pair != "" {
				declared[e.Pair]++
			}
		}
		if len(declared) == 0 {
			continue
		}
		t.Run(mc.name, func(t *testing.T) {
			for _, sc := range wkPairScenarios(t, mc.mode) {
				t.Run(sc.name, func(t *testing.T) {
					m := sc.build()
					offered := map[string][]string{}
					for _, e := range m.availableWhichKeyActions() {
						if e.Pair != "" {
							offered[e.Pair] = append(offered[e.Pair], e.Label)
						}
					}
					for name, want := range declared {
						got := offered[name]
						if len(got) != 0 && len(got) != want {
							t.Errorf("pair %q is half-offered here (%v); show both halves at a limit or hide both — "+
								"a half that only appears once the user has found its twin cannot be discovered",
								name, got)
						}
					}
				})
			}
		})
	}
}
