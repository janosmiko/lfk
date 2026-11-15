//go:build !windows

package app

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetachFromTerminal_CancelOnFinishedProcessIsDone(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "true")
	detachFromTerminal(cmd)
	require.NoError(t, cmd.Run())
	assert.ErrorIs(t, cmd.Cancel(), os.ErrProcessDone)
}
