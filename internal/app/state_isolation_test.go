package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/paths"
)

// TestStateDirsAreSandboxed enforces that the test package's config/state/data
// directories resolve inside the per-package temp sandbox (see TestMain), so no
// test can clobber the developer's real lfk files — e.g. resetting pinned
// resource types. This guards the LFK_*-precedence trap: paths.resolve checks
// LFK_<X>_DIR before XDG_<X>_HOME, so an XDG-only sandbox is bypassed on any
// machine whose environment sets LFK_STATE_DIR/LFK_CONFIG_DIR/LFK_DATA_DIR.
func TestStateDirsAreSandboxed(t *testing.T) {
	const marker = "lfk-app-tests-" // os.MkdirTemp prefix used by TestMain

	cases := []struct {
		name string
		fn   func() (string, error)
	}{
		{"StateDir", paths.StateDir},
		{"ConfigDir", paths.ConfigDir},
		{"DataDir", paths.DataDir},
	}
	for _, c := range cases {
		dir, err := c.fn()
		require.NoError(t, err)
		assert.Truef(t, strings.Contains(dir, marker),
			"%s must resolve inside the test sandbox, got %q", c.name, dir)
	}
}

// TestPinnedStateNeverTouchesRealFiles is the concrete regression for the
// reported bug: pinned.yaml must resolve inside the sandbox, so saving pinned
// types from a test never overwrites the developer's real file.
func TestPinnedStateNeverTouchesRealFiles(t *testing.T) {
	path := pinnedFilePath()
	require.NotEmpty(t, path)
	assert.Contains(t, path, "lfk-app-tests-",
		"pinned.yaml must live in the test sandbox, not the real state dir")
}
