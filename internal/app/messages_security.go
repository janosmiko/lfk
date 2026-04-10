// Package app — messages_security.go
package app

// securityAvailabilityLoadedMsg is sent after a per-source availability
// probe completes. The availability map has one entry per registered
// source (true = IsAvailable succeeded; false = error or not installed).
type securityAvailabilityLoadedMsg struct {
	context      string
	availability map[string]bool
}
