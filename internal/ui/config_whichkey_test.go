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

func TestLoadConfig_WhichKeyLeaderDelay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := "which_key_leader_delay_ms: 5000\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	ConfigWhichKeyLeaderDelayMs = 300
	LoadConfig(path)
	if ConfigWhichKeyLeaderDelayMs != 2000 {
		t.Errorf("which_key_leader_delay_ms = %d, want clamped to 2000", ConfigWhichKeyLeaderDelayMs)
	}
}

func TestLoadConfig_WhichKeyLeaderDelayDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("which_key_enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ConfigWhichKeyLeaderDelayMs = 300
	LoadConfig(path)
	if ConfigWhichKeyLeaderDelayMs != 300 {
		t.Errorf("unset which_key_leader_delay_ms = %d, want 300 retained", ConfigWhichKeyLeaderDelayMs)
	}
}
