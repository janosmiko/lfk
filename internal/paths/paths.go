// Package paths resolves lfk's config, state, and data directories.
//
// Each directory is resolved with the following precedence:
//
//  1. $LFK_<X>_DIR         — used verbatim as the lfk directory.
//  2. $XDG_<X>_HOME/lfk    — XDG base directory with an "lfk" component appended.
//  3. <home>/<default>/lfk — OS default when neither variable is set.
//
// The LFK_* variables let portable installs (for example Scoop's persist
// directory) redirect lfk's files without disturbing the shared XDG variables.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigDir returns the directory that holds config.yaml.
func ConfigDir() (string, error) {
	return resolve("LFK_CONFIG_DIR", "XDG_CONFIG_HOME", ".config")
}

// StateDir returns the directory that holds runtime state: bookmarks, session,
// pinned groups, input history, cluster colors, port-forward and local-cluster
// state, and packet captures.
func StateDir() (string, error) {
	return resolve("LFK_STATE_DIR", "XDG_STATE_HOME", filepath.Join(".local", "state"))
}

// DataDir returns the directory that holds the application log file.
func DataDir() (string, error) {
	return resolve("LFK_DATA_DIR", "XDG_DATA_HOME", filepath.Join(".local", "share"))
}

// resolve applies the three-step precedence. defaultSubPath is the
// home-relative path used when neither environment variable is set.
func resolve(lfkVar, xdgVar, defaultSubPath string) (string, error) {
	if dir := os.Getenv(lfkVar); dir != "" {
		return dir, nil
	}
	if xdg := os.Getenv(xdgVar); xdg != "" {
		return filepath.Join(xdg, "lfk"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, defaultSubPath, "lfk"), nil
}
