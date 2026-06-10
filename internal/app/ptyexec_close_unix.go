//go:build !windows

package app

import (
	"errors"
	"syscall"
)

// isBenignPTYCloseError reports whether err is the clean end-of-session signal a
// pty master returns once the child exits. On Linux that is EIO (wrapped in an
// *os.PathError), not io.EOF; treating it as benign avoids a spurious "PTY read
// error" log on every normal exec exit.
func isBenignPTYCloseError(err error) bool {
	return errors.Is(err, syscall.EIO)
}
