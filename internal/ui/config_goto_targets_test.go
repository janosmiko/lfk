package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_GotoTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `goto_targets:
  gA:
    kind: ApplicationSet
    group: argoproj.io
    name: AppSets
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	ConfigGotoTargets = nil
	LoadConfig(path)
	got, ok := ConfigGotoTargets["gA"]
	if !ok {
		t.Fatalf("goto_targets not wired; got %+v", ConfigGotoTargets)
	}
	if got.Kind != "ApplicationSet" || got.Group != "argoproj.io" || got.Name != "AppSets" {
		t.Fatalf("goto_targets value wrong: %+v", got)
	}
}

// TestLoadConfig_GotoTargets_InvalidSkipped verifies that invalid goto_targets
// entries are silently skipped while valid siblings are kept.
func TestLoadConfig_GotoTargets_InvalidSkipped(t *testing.T) {
	cases := []struct {
		name        string
		yamlBody    string
		validChord  string // the one valid entry that must survive
		validKind   string
		rejectChord string // an invalid chord that must NOT appear
	}{
		{
			name: "chord_too_short",
			yamlBody: `goto_targets:
  g:
    kind: Pod
  gA:
    kind: ApplicationSet
`,
			validChord:  "gA",
			validKind:   "ApplicationSet",
			rejectChord: "g",
		},
		{
			name: "chord_too_long",
			yamlBody: `goto_targets:
  gAA:
    kind: Pod
  gA:
    kind: ApplicationSet
`,
			validChord:  "gA",
			validKind:   "ApplicationSet",
			rejectChord: "gAA",
		},
		{
			name: "wrong_prefix",
			yamlBody: `goto_targets:
  xA:
    kind: Pod
  gA:
    kind: ApplicationSet
`,
			validChord:  "gA",
			validKind:   "ApplicationSet",
			rejectChord: "xA",
		},
		{
			// goto_targets are held to the same reachability rule as the
			// built-in bindings, so a modified key is a valid chord tail.
			name: "modified_key_accepted",
			yamlBody: `goto_targets:
  gctrl+p:
    kind: ApplicationSet
  gpp:
    kind: Pod
`,
			validChord:  "gctrl+p",
			validKind:   "ApplicationSet",
			rejectChord: "gpp",
		},
		{
			name: "empty_kind",
			yamlBody: `goto_targets:
  gB:
    kind: ""
  gA:
    kind: ApplicationSet
`,
			validChord:  "gA",
			validKind:   "ApplicationSet",
			rejectChord: "gB",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yamlBody), 0o600); err != nil {
				t.Fatal(err)
			}
			ConfigGotoTargets = nil
			LoadConfig(path)

			if _, bad := ConfigGotoTargets[tc.rejectChord]; bad {
				t.Errorf("invalid chord %q should have been skipped, but was accepted", tc.rejectChord)
			}
			got, ok := ConfigGotoTargets[tc.validChord]
			if !ok {
				t.Fatalf("valid chord %q missing from ConfigGotoTargets; got %+v", tc.validChord, ConfigGotoTargets)
			}
			if got.Kind != tc.validKind {
				t.Errorf("chord %q: want kind %q, got %q", tc.validChord, tc.validKind, got.Kind)
			}
		})
	}
}

// TestLoadConfig_GotoTargets_CustomJumpTop is the regression test for the
// keybinding-merge ordering bug: a custom jump_top prefix must be used when
// validating goto_targets chords in the same config.
func TestLoadConfig_GotoTargets_CustomJumpTop(t *testing.T) {
	t.Cleanup(snapshotAllConfigGlobals(t))

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// Custom jump_top = "f"; chord "fA" should be valid; "gA" (old default) should not.
	yaml := `keybindings:
  jump_top: "f"
goto_targets:
  fA:
    kind: ApplicationSet
    group: argoproj.io
  gA:
    kind: Pod
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	ConfigGotoTargets = nil
	LoadConfig(path)

	got, ok := ConfigGotoTargets["fA"]
	if !ok {
		t.Fatalf("chord fA (matching custom jump_top=f) should be valid; got %+v", ConfigGotoTargets)
	}
	if got.Kind != "ApplicationSet" {
		t.Errorf("chord fA: want kind ApplicationSet, got %q", got.Kind)
	}
	if _, bad := ConfigGotoTargets["gA"]; bad {
		t.Error("chord gA should have been rejected because jump_top was changed to f")
	}
}

// TestLoadConfig_UnreachableGotoChordsDropped covers the half-configurable
// goto chord: handleGotoChord only ever builds jump_top + the next key, so a
// goto_* value that starts with anything else is dead. Before it was dropped
// at load, the g-prefix popup still drew a cell for it.
func TestLoadConfig_UnreachableGotoChordsDropped(t *testing.T) {
	cases := []struct {
		name     string
		yamlBody string
		want     map[string]string // field value after load
	}{
		{
			name: "wrong prefix is dropped, siblings survive",
			yamlBody: `keybindings:
  jump_top: "g"
  goto_pods: "zp"
  goto_nodes: "gn"
`,
			want: map[string]string{"pods": "", "nodes": "gn"},
		},
		{
			name: "prefix alone is dropped: the second key is what selects the target",
			yamlBody: `keybindings:
  jump_top: "g"
  goto_pods: "g"
`,
			want: map[string]string{"pods": ""},
		},
		{
			name: "a custom jump_top makes the matching chord reachable",
			yamlBody: `keybindings:
  jump_top: "z"
  goto_pods: "zp"
  goto_nodes: "gn"
`,
			want: map[string]string{"pods": "zp", "nodes": ""},
		},
		{
			name: "a two-key suffix is dropped: handleGotoChord reads one keypress",
			yamlBody: `keybindings:
  jump_top: "g"
  goto_pods: "gpp"
  goto_nodes: "gn"
`,
			want: map[string]string{"pods": "", "nodes": "gn"},
		},
		{
			name: "a modified key is still one keypress and survives",
			yamlBody: `keybindings:
  jump_top: "g"
  goto_pods: "gctrl+p"
  goto_nodes: "gtab"
`,
			want: map[string]string{"pods": "gctrl+p", "nodes": "gtab"},
		},
		{
			name: "previous_namespace obeys the same rule",
			yamlBody: `keybindings:
  jump_top: "g"
  previous_namespace: "z\\"
`,
			want: map[string]string{"prevns": ""},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yamlBody), 0o600); err != nil {
				t.Fatal(err)
			}
			orig := ActiveKeybindings
			t.Cleanup(func() { ActiveKeybindings = orig })
			LoadConfig(path)

			got := map[string]string{
				"pods":   ActiveKeybindings.GotoPods,
				"nodes":  ActiveKeybindings.GotoNodes,
				"prevns": ActiveKeybindings.PreviousNamespace,
			}
			for field, want := range tc.want {
				if got[field] != want {
					t.Errorf("%s = %q, want %q", field, got[field], want)
				}
			}
		})
	}
}

// TestGotoChordReachable pins the predicate both surfaces share.
func TestGotoChordReachable(t *testing.T) {
	cases := []struct {
		chord, prefix string
		want          bool
	}{
		{"gp", "g", true},
		{"zp", "g", false},
		{"g", "g", false},  // prefix alone: handleGotoChord falls through to gg
		{"", "g", false},   // unset
		{"zp", "z", true},  // custom jump_top
		{"g\\", "g", true}, // previous_namespace default shape

		// The remainder is one KEYPRESS, not one rune. A modified or named key
		// prints as a word and stays reachable; a run of two keys never is.
		{"gAA", "g", false},
		{"gctrl+p", "g", true},
		{"gctrl+alt+y", "g", true},
		{"gctrl++", "g", true}, // the literal "+" key under a modifier
		{"g+", "g", true},
		{"gtab", "g", true},
		{"gpgup", "g", true},
		{"gf12", "g", true},
		{"gspace", "g", true},
		{"gpp", "g", false},      // two letters, no such key name
		{"gctrl+pp", "g", false}, // modifier over two keys
		{"ga+b", "g", false},     // "a" is not a modifier

		// A trailing plus with nothing after it is a typo, not the "+" key.
		// Only "ctrl++" spells a modified "+"; dropping the last component
		// unconditionally used to swallow the modifier and pass these.
		{"gctrl+", "g", false},
		{"gctrl+alt+", "g", false},
		{"gga+", "g", false},
	}
	for _, tc := range cases {
		if got := GotoChordReachable(tc.chord, tc.prefix); got != tc.want {
			t.Errorf("GotoChordReachable(%q, %q) = %v, want %v", tc.chord, tc.prefix, got, tc.want)
		}
	}
}
