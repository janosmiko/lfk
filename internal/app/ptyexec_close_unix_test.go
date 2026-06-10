//go:build !windows

package app

import (
	"errors"
	"io"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsBenignPTYCloseError(t *testing.T) {
	// Linux returns EIO wrapped in *os.PathError once the child exits.
	wrapped := &os.PathError{Op: "read", Path: "/dev/ptmx", Err: syscall.EIO}
	assert.True(t, isBenignPTYCloseError(wrapped), "wrapped EIO is a clean pty close")

	assert.False(t, isBenignPTYCloseError(io.EOF), "io.EOF is handled separately, not here")
	assert.False(t, isBenignPTYCloseError(errors.New("boom")), "real errors must still log")
}
