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
