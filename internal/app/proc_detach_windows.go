//go:build windows

package app

import "os/exec"

func detachFromTerminal(*exec.Cmd) {}
