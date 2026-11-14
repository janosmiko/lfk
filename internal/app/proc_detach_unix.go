//go:build !windows

package app

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// detachFromTerminal puts the child in its own session so it cannot open
// lfk's controlling tty, and makes a kill reach the whole process group.
func detachFromTerminal(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
