package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// On Windows, github.com/creack/pty (the embedded PTY driver behind
// TerminalModePTY) returns ErrUnsupported from StartWithSize, so the
// embedded-terminal path can never succeed. The package-level default
// must therefore be TerminalModeExec on Windows so a first-time user
// with no config file gets a working `Exec` action instead of the
// cryptic "failed to start PTY: unsupported" message that issue #194
// stems from. Linux and macOS keep the embedded PTY default.
func TestDefaultTerminalModeForOS(t *testing.T) {
	cases := []struct {
		goos string
		want string
	}{
		{"windows", TerminalModeExec},
		{"linux", TerminalModePTY},
		{"darwin", TerminalModePTY},
		{"freebsd", TerminalModePTY},
	}
	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			assert.Equal(t, tc.want, defaultTerminalModeForOS(tc.goos))
		})
	}
}
