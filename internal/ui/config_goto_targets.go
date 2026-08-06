package ui

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

// GotoChordReachable reports whether a chord can ever be dispatched.
// handleGotoChord (internal/app/whichkey.go) builds the chord it looks up as
// jump_top + msg.String() for ONE keypress, so a chord that does not start with
// jump_top, or whose remainder is not a single keypress, is unreachable however
// it was configured.
//
// The remainder is measured in keypresses, not runes: "gctrl+p", "gtab" and
// "gf1" are all one key after the prefix and stay reachable, while "gAA" is two
// and never is.
func GotoChordReachable(chord, jumpTopPrefix string) bool {
	if !strings.HasPrefix(chord, jumpTopPrefix) {
		return false
	}
	return IsSingleKeypress(strings.TrimPrefix(chord, jumpTopPrefix))
}

// dropUnreachableGotoChords blanks every goto_* chord (and previous_namespace)
// that GotoChordReachable rejects, warning about the ones the user wrote.
//
// Without this a chord like `goto_pods: "zp"` under `jump_top: "g"` was dead —
// and still drawn in the g-prefix popup, which advertises the part after the
// prefix and so rendered a cell no keypress could reach. Blanking is what
// removes it: gotoTargets skips empty chords.
//
// userSet is the raw keybindings block from the config file, where an unset
// field is empty. Only values the user actually wrote are warned about: a
// custom jump_top strands all seventeen built-in defaults at once, and
// seventeen warnings about values nobody typed is noise, not a diagnostic.
// Those defaults were already unreachable — this only stops the popup
// advertising them.
//
// applyGotoTargets calls the same predicate on user-defined goto_targets, so
// both halves of the feature reject the same shapes. Fields are found by yaml
// tag rather than listed, so a new goto_* binding is validated the day it is
// added.
//
// Writes to stderr for the same reason loadConfigFile does: LoadConfig runs
// before logger.Init, so a logger call here would go to io.Discard.
func dropUnreachableGotoChords(kb, userSet *Keybindings) {
	v := reflect.ValueOf(kb).Elem()
	uv := reflect.ValueOf(userSet).Elem()
	for i, f := range reflect.VisibleFields(v.Type()) {
		if f.Type.Kind() != reflect.String {
			continue
		}
		name := f.Tag.Get("yaml")
		if !strings.HasPrefix(name, "goto_") && name != "previous_namespace" {
			continue
		}
		fv := v.Field(i)
		chord := fv.String()
		if chord == "" || GotoChordReachable(chord, kb.JumpTop) {
			continue
		}
		if uv.Field(i).String() != "" {
			fmt.Fprintf(os.Stderr,
				"lfk: keybinding %s: %q must start with jump_top (%q) and add one more key; the chord is unreachable and has been ignored\n",
				name, chord, kb.JumpTop)
		}
		fv.SetString("")
	}
}

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
// A chord must satisfy GotoChordReachable — the same rule the built-in goto
// bindings are held to, so the two halves of the feature cannot drift apart —
// and have a non-empty kind; invalid entries are silently skipped.
func applyGotoTargets(cfg configFile, jumpTopPrefix string) {
	if len(cfg.GotoTargets) == 0 {
		// Reset so a previous non-empty value does not survive a reload that
		// removed goto_targets.
		ConfigGotoTargets = nil
		return
	}
	valid := make(map[string]GotoTargetEntry, len(cfg.GotoTargets))
	for chord, t := range cfg.GotoTargets {
		if !GotoChordReachable(chord, jumpTopPrefix) || t.Kind == "" {
			continue
		}
		valid[chord] = t
	}
	ConfigGotoTargets = valid
}
