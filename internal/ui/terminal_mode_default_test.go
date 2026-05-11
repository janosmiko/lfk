package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// resolveTerminalMode is the testable kernel behind applyConfigOptions'
// terminal-mode handling. The Windows branch must downgrade an explicit
// `terminal: pty` to exec because pty is unreachable there (creack/pty
// has no Windows backend) — letting the option through would just trap
// users in a state where every interactive action fails.
func TestResolveTerminalMode(t *testing.T) {
	cases := []struct {
		name        string
		configValue string
		goos        string
		currentMode string
		wantMode    string
		wantWarn    bool
	}{
		{
			name:        "empty config keeps current default",
			configValue: "",
			goos:        "linux",
			currentMode: TerminalModePTY,
			wantMode:    TerminalModePTY,
		},
		{
			name:        "pty on linux is honoured",
			configValue: "pty",
			goos:        "linux",
			currentMode: TerminalModeExec,
			wantMode:    TerminalModePTY,
		},
		{
			name:        "pty on darwin is honoured",
			configValue: "pty",
			goos:        "darwin",
			currentMode: TerminalModeExec,
			wantMode:    TerminalModePTY,
		},
		{
			name:        "pty on windows is rejected and falls back to exec",
			configValue: "pty",
			goos:        "windows",
			currentMode: TerminalModeExec,
			wantMode:    TerminalModeExec,
			wantWarn:    true,
		},
		{
			name:        "exec on windows is honoured",
			configValue: "exec",
			goos:        "windows",
			currentMode: TerminalModeExec,
			wantMode:    TerminalModeExec,
		},
		{
			name:        "mux on windows is honoured (user can opt in if they run tmux/zellij)",
			configValue: "mux",
			goos:        "windows",
			currentMode: TerminalModeExec,
			wantMode:    TerminalModeMux,
		},
		{
			name:        "case-insensitive normalisation",
			configValue: "PTY",
			goos:        "linux",
			currentMode: TerminalModeExec,
			wantMode:    TerminalModePTY,
		},
		{
			name:        "leading and trailing whitespace is trimmed (no spurious warning)",
			configValue: "  pty\n",
			goos:        "linux",
			currentMode: TerminalModeExec,
			wantMode:    TerminalModePTY,
		},
		{
			name:        "whitespace-only value treated as empty",
			configValue: "   ",
			goos:        "linux",
			currentMode: TerminalModePTY,
			wantMode:    TerminalModePTY,
		},
		{
			name:        "unrecognised value keeps current with warning",
			configValue: "garbage",
			goos:        "linux",
			currentMode: TerminalModePTY,
			wantMode:    TerminalModePTY,
			wantWarn:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode, warning := resolveTerminalMode(tc.configValue, tc.goos, tc.currentMode)
			assert.Equal(t, tc.wantMode, mode)
			if tc.wantWarn {
				assert.NotEmpty(t, warning, "expected a fallback warning")
			} else {
				assert.Empty(t, warning, "no warning expected for an honoured value")
			}
		})
	}
}

// resolveTerminalMode must not echo the raw config value into its
// warning string — global log-redaction policy. Even though the
// `terminal:` field is structurally an enum, a misplaced secret in
// that key would otherwise land in the log stream verbatim. Both
// warning-producing branches must hold the line: the unrecognised
// fallback (any value the switch doesn't match) and the Windows
// pty-downgrade. The downgrade path's warning is currently a fixed
// string by construction, but pinning the policy in a test prevents
// a future refactor from quietly weakening it.
func TestResolveTerminalMode_WarningDoesNotLeakRawValue(t *testing.T) {
	const sentinel = "super-secret-token-do-not-leak"

	t.Run("unrecognised value branch", func(t *testing.T) {
		_, warning := resolveTerminalMode(sentinel, "linux", TerminalModePTY)
		assert.NotEmpty(t, warning, "an unrecognised value must produce a warning")
		assert.NotContains(t, warning, sentinel,
			"warning string must not embed the raw configValue (log redaction policy)")
	})

	t.Run("windows pty downgrade branch", func(t *testing.T) {
		// Raw value normalises to "pty" (TrimSpace + ToLower) so the
		// Windows downgrade branch fires; the original carries a tab
		// and mixed case that the static warning never produces. A
		// future refactor that interpolates configValue into the
		// warning would re-introduce the tab and fail NotContains.
		raw := "\t  PtY  \t"
		_, warning := resolveTerminalMode(raw, "windows", TerminalModeExec)
		assert.NotEmpty(t, warning, "windows pty downgrade must produce a warning")
		assert.NotContains(t, warning, raw,
			"windows downgrade warning must not embed the raw configValue")
		assert.NotContains(t, warning, "\t",
			"windows downgrade warning must not echo whitespace from the raw input")
	})
}

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
