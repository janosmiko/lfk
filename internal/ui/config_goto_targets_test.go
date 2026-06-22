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
