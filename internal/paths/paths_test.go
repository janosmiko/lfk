package paths

import (
	"path/filepath"
	"testing"
)

// TestDirResolution exercises the precedence of each directory accessor:
// LFK_*_DIR (verbatim) > XDG_*_HOME (with an "lfk" component appended) > OS default.
func TestDirResolution(t *testing.T) {
	home := t.TempDir()

	tests := []struct {
		name   string
		fn     func() (string, error)
		lfkVar string
		xdgVar string
		lfkVal string
		xdgVal string
		want   string
	}{
		{
			name: "config: LFK_CONFIG_DIR wins, used verbatim",
			fn:   ConfigDir, lfkVar: "LFK_CONFIG_DIR", xdgVar: "XDG_CONFIG_HOME",
			lfkVal: "/portable/cfg", xdgVal: "/xdg/cfg", want: "/portable/cfg",
		},
		{
			name: "config: XDG_CONFIG_HOME, lfk appended",
			fn:   ConfigDir, lfkVar: "LFK_CONFIG_DIR", xdgVar: "XDG_CONFIG_HOME",
			lfkVal: "", xdgVal: "/xdg/cfg", want: filepath.Join("/xdg/cfg", "lfk"),
		},
		{
			name: "config: OS default",
			fn:   ConfigDir, lfkVar: "LFK_CONFIG_DIR", xdgVar: "XDG_CONFIG_HOME",
			lfkVal: "", xdgVal: "", want: filepath.Join(home, ".config", "lfk"),
		},
		{
			name: "state: LFK_STATE_DIR wins, used verbatim",
			fn:   StateDir, lfkVar: "LFK_STATE_DIR", xdgVar: "XDG_STATE_HOME",
			lfkVal: "/portable/state", xdgVal: "/xdg/state", want: "/portable/state",
		},
		{
			name: "state: XDG_STATE_HOME, lfk appended",
			fn:   StateDir, lfkVar: "LFK_STATE_DIR", xdgVar: "XDG_STATE_HOME",
			lfkVal: "", xdgVal: "/xdg/state", want: filepath.Join("/xdg/state", "lfk"),
		},
		{
			name: "state: OS default",
			fn:   StateDir, lfkVar: "LFK_STATE_DIR", xdgVar: "XDG_STATE_HOME",
			lfkVal: "", xdgVal: "", want: filepath.Join(home, ".local", "state", "lfk"),
		},
		{
			name: "data: LFK_DATA_DIR wins, used verbatim",
			fn:   DataDir, lfkVar: "LFK_DATA_DIR", xdgVar: "XDG_DATA_HOME",
			lfkVal: "/portable/data", xdgVal: "/xdg/data", want: "/portable/data",
		},
		{
			name: "data: XDG_DATA_HOME, lfk appended",
			fn:   DataDir, lfkVar: "LFK_DATA_DIR", xdgVar: "XDG_DATA_HOME",
			lfkVal: "", xdgVal: "/xdg/data", want: filepath.Join("/xdg/data", "lfk"),
		},
		{
			name: "data: OS default",
			fn:   DataDir, lfkVar: "LFK_DATA_DIR", xdgVar: "XDG_DATA_HOME",
			lfkVal: "", xdgVal: "", want: filepath.Join(home, ".local", "share", "lfk"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set HOME and USERPROFILE so os.UserHomeDir() is deterministic on
			// both Unix and Windows. Explicit empty values clear any LFK_*/XDG_*
			// inherited from the developer's environment, keeping the test hermetic.
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv(tc.lfkVar, tc.lfkVal)
			t.Setenv(tc.xdgVar, tc.xdgVal)

			got, err := tc.fn()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
