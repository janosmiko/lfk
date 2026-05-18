package k8s

import (
	"path/filepath"
	"testing"
)

// TestDefaultCaptureDirHonorsLFKStateDir verifies the capture directory
// follows LFK_STATE_DIR so portable installs keep packet captures together
// with the rest of lfk's state.
func TestDefaultCaptureDirHonorsLFKStateDir(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", stateDir)
	t.Setenv("XDG_STATE_HOME", "")

	got := defaultCaptureDir()
	want := filepath.Join(stateDir, "captures")
	if got != want {
		t.Errorf("defaultCaptureDir() = %q, want %q", got, want)
	}
}
