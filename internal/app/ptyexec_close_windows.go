//go:build windows

package app

// isBenignPTYCloseError is a no-op on Windows: PTY mode is unsupported there
// (pty.StartWithSize fails before the reader goroutine runs), so there is no
// pty-close error to special-case. io.EOF handling in the caller suffices.
func isBenignPTYCloseError(error) bool { return false }
