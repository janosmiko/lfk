package app

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helm uninstall/upgrade are non-interactive, so they run as background tasks
// (runHelmCmdResult) rather than tea.ExecProcess — the latter suspends the TUI
// and takes over the terminal, which only the editor-driven editHelmValues
// needs. runHelmCmdResult runs the prepared command to completion and maps the
// outcome to an actionResultMsg, never a terminal takeover.
func TestRunHelmCmdResultSuccess(t *testing.T) {
	msg := runHelmCmdResult(exec.Command("true"), "Upgraded my-release", "helm upgrade")
	res, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.NoError(t, res.err)
	assert.Equal(t, "Upgraded my-release", res.message)
}

func TestRunHelmCmdResultFailureWrapsOutput(t *testing.T) {
	// `false` exits non-zero with no output; the error must carry the prefix.
	msg := runHelmCmdResult(exec.Command("false"), "Upgraded my-release", "helm upgrade")
	res, ok := msg.(actionResultMsg)
	require.True(t, ok)
	require.Error(t, res.err)
	assert.Empty(t, res.message, "no success message on failure")
	assert.Contains(t, res.err.Error(), "helm upgrade")
}
