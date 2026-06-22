package ui

import "strings"

// GotoTargetEntry is one user-defined goto target from the config file.
type GotoTargetEntry struct {
	Kind  string `json:"kind" yaml:"kind"`
	Group string `json:"group" yaml:"group"`
	Name  string `json:"name" yaml:"name"`
}

// ConfigGotoTargets is populated by applyGotoTargets (called from LoadConfig
// after the keybindings merge) from the goto_targets config section. Keys are
// full chords (e.g. "gx").
// Invalid entries (wrong chord format or missing kind) are silently skipped.
var ConfigGotoTargets map[string]GotoTargetEntry

// applyGotoTargets validates and loads goto_targets from cfg into ConfigGotoTargets.
// jumpTopPrefix is the already-merged jump_top keybinding (pass kb.JumpTop from
// LoadConfig so custom jump_top values are validated against the correct prefix).
// A chord must be exactly 2 runes, start with jumpTopPrefix, and have a non-empty
// kind; invalid entries are silently skipped.
func applyGotoTargets(cfg configFile, jumpTopPrefix string) {
	if len(cfg.GotoTargets) == 0 {
		return
	}
	valid := make(map[string]GotoTargetEntry, len(cfg.GotoTargets))
	for chord, t := range cfg.GotoTargets {
		if len([]rune(chord)) != 2 || !strings.HasPrefix(chord, jumpTopPrefix) || t.Kind == "" {
			continue
		}
		valid[chord] = t
	}
	ConfigGotoTargets = valid
}
