package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_WhichKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := "which_key_enabled: false\nwhich_key_delay_ms: 5000\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	ConfigWhichKeyEnabled = true
	ConfigWhichKeyDelayMs = 0
	LoadConfig(path)
	if ConfigWhichKeyEnabled {
		t.Error("which_key_enabled=false not wired")
	}
	if ConfigWhichKeyDelayMs != 2000 {
		t.Errorf("which_key_delay_ms = %d, want clamped to 2000", ConfigWhichKeyDelayMs)
	}
}
