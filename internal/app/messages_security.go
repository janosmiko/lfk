package app

import (
	"github.com/janosmiko/lfk/internal/security"
)

// securityFindingsLoadedMsg is sent when Manager.FetchAll completes
// (successfully or with partial failures).
type securityFindingsLoadedMsg struct {
	context   string
	namespace string
	result    security.FetchResult
}

// securityFetchErrorMsg is sent when Manager.FetchAll returns a fatal error.
// In the current implementation partial failures don't trigger this — only
// unexpected manager-level errors do.
type securityFetchErrorMsg struct {
	context string
	err     error
}

// securityAvailabilityLoadedMsg is sent after Manager.AnyAvailable completes.
// It is used to gate the SEC column and per-resource H key.
type securityAvailabilityLoadedMsg struct {
	context   string
	available bool
}
