// Package app — messages_security.go
package app

import "github.com/janosmiko/lfk/internal/security"

// securityAvailabilityLoadedMsg is sent after a per-source availability
// probe completes. The availability map has one entry per registered
// source (true = IsAvailable succeeded; false = error or not installed).
type securityAvailabilityLoadedMsg struct {
	context      string
	availability map[string]bool
}

// securityFindingsScannedMsg is sent when a single security source
// finishes scanning findings in the background. Each available source
// gets its own tracked bgtask ("Scan Heuristic findings", etc.) so the
// user can see per-source progress in the :tasks overlay.
type securityFindingsScannedMsg struct {
	context   string
	namespace string
	source    string
	findings  []security.Finding
	err       error
}
