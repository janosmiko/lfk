package ui

// GotoTargetEntry is one user-defined goto target from the config file.
// Task 3 populates ConfigGotoTargets; until then the map is always empty.
type GotoTargetEntry struct {
	Kind  string `json:"kind" yaml:"kind"`
	Group string `json:"group" yaml:"group"`
	Name  string `json:"name" yaml:"name"`
}

// ConfigGotoTargets is populated by applyConfigOptions (Task 3) from the
// goto_targets config section. Keys are full chords (e.g. "gx").
var ConfigGotoTargets map[string]GotoTargetEntry
