package app

import (
	"path/filepath"
	"testing"
)

// TestPortableStateDir verifies that every app-package state path honors
// LFK_STATE_DIR, so a portable install (for example Scoop's persist
// directory) keeps all state files under one redirectable root.
func TestPortableStateDir(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", stateDir)
	t.Setenv("XDG_STATE_HOME", "")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"session", sessionFilePath(), filepath.Join(stateDir, "session.yaml")},
		{"sort-memory", sortMemoryFilePath(), filepath.Join(stateDir, "sort_memory.yaml")},
		{"column-prefs", columnPrefsFilePath(), filepath.Join(stateDir, "column_prefs.yaml")},
		{"pinned", pinnedFilePath(), filepath.Join(stateDir, "pinned.yaml")},
		{"bookmarks", bookmarksFilePath(), filepath.Join(stateDir, "bookmarks.yaml")},
		{"cluster-colors", clusterColorsFilePath(), filepath.Join(stateDir, "cluster-colors.yaml")},
		{"portforwards", portForwardStatePath(), filepath.Join(stateDir, "portforwards.yaml")},
		{"local-clusters", localClusterStateFilePath(), filepath.Join(stateDir, "local-clusters.yaml")},
		{"kubetris", highScoreFilePath(), filepath.Join(stateDir, "kubetris-highscore")},
		{"history", historyFilePathFor("history"), filepath.Join(stateDir, "history")},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}
