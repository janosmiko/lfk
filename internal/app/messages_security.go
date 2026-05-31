// Package app — messages_security.go
package app

import "github.com/janosmiko/lfk/internal/security"

// securityAvailabilityLoadedMsg is sent after a per-source availability
// probe completes. The availability map only includes sources whose probe
// returned without error (true = installed/available; false = definitively
// not installed). Sources whose probe errored are omitted so the handler's
// merge keeps the previous-known availability rather than wiping them.
type securityAvailabilityLoadedMsg struct {
	context      string
	availability map[string]bool
}

// securityFindingsLoadedMsg is sent after a background findings fetch
// returns. The findings span every available source for the given
// (context, namespace) pair. The handler uses them to build the
// FindingIndex consulted by the SEC badge renderer. errors carries
// per-source failures from FetchAll's aggregate result so a fully-failed
// fetch can be distinguished from "no findings" in the handler.
type securityFindingsLoadedMsg struct {
	context   string
	namespace string
	findings  []security.Finding
	errors    map[string]error
}

// securityIgnoresSaveErrMsg is dispatched when the async ignore-state save
// (via saveSecurityIgnoresCmd) fails. Successful saves emit no message — the
// optimistic status set when the user pressed the action stays visible. The
// failure handler swaps in an error status so the user knows their ignore
// rule did not persist.
type securityIgnoresSaveErrMsg struct {
	err error
}
