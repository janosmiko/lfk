package app

import (
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

const whichKeyPrefsFileName = "whichkey_prefs.yaml"

// WhichKeyPrefsState is the on-disk schema for the which-key panel's entry
// order, written to the state directory rather than to config.yaml: config is
// user-authored, this is the app recording what the user last did.
//
// Grouped is that last runtime choice (the leader key's toggle). ConfigDefault
// is the which_key_grouped value in force when the choice was made — see
// loadWhichKeyGrouping for what it buys.
//
// Both fields are pointers so an absent one is distinguishable from false: a
// hand-written or half-written file must degrade to "no preference", never to
// "the user chose key order".
type WhichKeyPrefsState struct {
	Grouped       *bool `json:"grouped,omitempty"        yaml:"grouped,omitempty"`
	ConfigDefault *bool `json:"config_default,omitempty" yaml:"config_default,omitempty"`
}

// whichKeyPrefsFilePath returns the path to the which-key prefs state file.
func whichKeyPrefsFilePath() string {
	return stateFilePath(whichKeyPrefsFileName)
}

// loadWhichKeyPrefs reads the state file, returning a zero value (every field
// nil, i.e. "no preference") when it is missing, unreadable, or corrupt. A
// user-visible on-disk schema has to survive being edited by hand, so nothing
// here is ever fatal.
func loadWhichKeyPrefs() WhichKeyPrefsState {
	return loadStateFile[WhichKeyPrefsState](whichKeyPrefsFileName)
}

// loadWhichKeyGrouping resolves the startup entry order.
//
// PRECEDENCE: the persisted runtime choice outranks which_key_grouped. The
// config key is documented as the STARTUP default, and a default is what
// applies until the user says otherwise; pressing the leader key twice is
// saying otherwise, and it would be strange for the panel to forget that on
// every launch (the whole point of persisting it).
//
// The stale-state escape: a default only deserves to be outvoted while it is
// still the default the user was overriding. ConfigDefault records the value
// in force at toggle time, so editing which_key_grouped afterwards makes the
// recorded baseline stop matching and retires the choice — the newer edit is
// the newer intent. Absent baseline (a hand-written file) skips the check
// rather than guessing.
//
// Residual edge: writing which_key_grouped with the value it already had
// effectively (e.g. `true`, which is also the built-in default) changes
// nothing to compare against, so the persisted choice stands. Toggling with
// the leader key overwrites it, and deleting the state file clears it — both
// documented in docs/keybindings.md.
func loadWhichKeyGrouping() wkGrouping {
	s := loadWhichKeyPrefs()
	if s.Grouped == nil {
		return wkGroupDefault
	}
	if s.ConfigDefault != nil && *s.ConfigDefault != ui.ConfigWhichKeyGrouped {
		return wkGroupDefault
	}
	if *s.Grouped {
		return wkGroupOn
	}
	return wkGroupOff
}

// saveWhichKeyGrouping records the runtime choice. Best-effort: losing a UI
// preference is never worth failing a keypress over, so every error is logged
// and swallowed. Called from the single Bubble Tea Update goroutine.
func saveWhichKeyGrouping(grouped bool) {
	cfg := ui.ConfigWhichKeyGrouped
	if err := saveStateFile(whichKeyPrefsFileName, WhichKeyPrefsState{Grouped: &grouped, ConfigDefault: &cfg}); err != nil {
		logger.Error("Failed to persist which-key prefs", "error", err)
	}
}
