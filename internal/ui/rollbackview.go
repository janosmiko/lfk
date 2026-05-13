package ui

import (
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
)

// HelmRevision represents a single entry from `helm history` output. The
// rollback / history pickers render via the OverlayList-based helpers in
// internal/app; the type stays here so both internal/app and the k8s
// caller share one struct definition.
type HelmRevision struct {
	Revision    int
	Status      string
	Chart       string
	AppVersion  string
	Description string
	Updated     string
}

// FormatAge returns a human-readable age string for a timestamp. Zero time
// renders as "-"; otherwise delegates to k8s.FormatAge so deployment-rollback
// rows match the format used everywhere else in the app (including the year
// suffix once a revision is older than ~12 months).
func FormatAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return k8s.FormatAge(time.Since(t))
}
