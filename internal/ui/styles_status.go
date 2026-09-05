package ui

import (
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/model"
)

// statusSev classifies a resource status string into a severity bucket. It is
// the single source of truth shared by StatusStyle (which maps it to a color)
// and StatusSeverityRank (which maps it to an ordering), so coloring and
// severity ordering can never drift — and the ordering stays correct in
// no-color mode, where the styles carry no distinguishing foreground.
type statusSev int

const (
	sevUnknown     statusSev = iota // unknown non-empty status -> dimmed
	sevBlank                        // "" -> neutral
	sevDefault                      // literal "default" -> primary
	sevNormal                       // "Normal" event type -> dim
	sevRunning                      // healthy / ready / synced
	sevDone                         // succeeded / completed
	sevProgressing                  // pending / progressing / out-of-sync / warning
	sevFailed                       // failed / degraded / errored
)

func statusSeverity(status string) statusSev {
	switch status {
	case "default":
		return sevDefault
	case "Running", "Active", "Bound", "Available", "Ready",
		"Healthy", "Healthy/Synced", "Synced",
		"Deployed",
		"SecretSynced", "Created", "Updated", "Valid",
		"Established":
		return sevRunning
	case "Succeeded", "Completed",
		"Superseded":
		return sevDone
	case "Pending", "ContainerCreating", "PodInitializing", "Terminating",
		"Waiting", "Init", "NotReady",
		"Progressing", "Progressing/Synced", "Progressing/OutOfSync",
		"Missing", "Suspended", "Unknown", "Reconciling",
		"Ready,SchedulingDisabled", "NotReady,SchedulingDisabled", "SchedulingDisabled",
		"Healthy/OutOfSync", "Missing/OutOfSync", "Suspended/OutOfSync",
		"OutOfSync",
		"Pending-install", "Pending-upgrade", "Pending-rollback", "Uninstalling",
		"Warning":
		return sevProgressing
	case "Normal":
		return sevNormal
	case "Failed", "CrashLoopBackOff", "Error", "ImagePullBackOff", "Terminated",
		"Degraded", "Degraded/Synced", "Degraded/OutOfSync",
		"Missing/Synced",
		"OOMKilled", "ErrImagePull", "CreateContainerConfigError",
		"SecretSyncedError", "SecretMissing", "MissingProviderSecret",
		model.MissingRefStatus,
		"UpdateFailed", "FailedScheduling",
		"InvalidStoreConfiguration", "InvalidProviderConfig", "ValidationFailed":
		return sevFailed
	default:
		if status == "" {
			return sevBlank
		}
		return phraseSeverity(status)
	}
}

// phraseSeverityWords maps lowercase words to a severity for statuses that miss
// the exact-match table above — operators that put free-form phrases in
// .status.phase (e.g. CloudNativePG's "Cluster in healthy state", "Failing
// over") instead of conventional CamelCase values. Word-based, so "Unavailable"
// or "Jumping" never match a fragment.
var phraseSeverityWords = map[string]statusSev{
	"failed": sevFailed, "failing": sevFailed, "error": sevFailed,
	"errored": sevFailed, "degraded": sevFailed, "unhealthy": sevFailed,
	"pending": sevProgressing, "waiting": sevProgressing, "creating": sevProgressing,
	"initializing": sevProgressing, "starting": sevProgressing, "setting": sevProgressing,
	"upgrading": sevProgressing, "provisioning": sevProgressing, "progressing": sevProgressing,
	"progress": sevProgressing, "reconciling": sevProgressing, "restarting": sevProgressing,
	"restarted": sevProgressing, "terminating": sevProgressing, "deleting": sevProgressing,
	"healthy": sevRunning, "ready": sevRunning, "running": sevRunning, "active": sevRunning,
	"succeeded": sevDone, "completed": sevDone,
}

// phraseSeverity classifies an unknown status string by scanning its words
// against phraseSeverityWords. The worst-severity word wins ("Healthy but
// degraded" is failed). A positive word preceded by "not" reads as
// progressing-amber, mirroring the exact-match "NotReady". No recognized word
// yields sevUnknown (gray).
func phraseSeverity(status string) statusSev {
	sev := sevUnknown
	rank := func(s statusSev) int {
		switch s {
		case sevFailed:
			return 3
		case sevProgressing:
			return 2
		case sevRunning, sevDone:
			return 1
		default:
			return 0
		}
	}
	negated := false
	for _, w := range strings.FieldsFunc(strings.ToLower(status), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if w == "not" {
			negated = true
			continue
		}
		if s, ok := phraseSeverityWords[w]; ok {
			if negated && (s == sevRunning || s == sevDone) {
				s = sevProgressing
			}
			if rank(s) > rank(sev) {
				sev = s
			}
		}
		negated = false
	}
	return sev
}

// StatusStyle returns the appropriate style for a resource status string.
func StatusStyle(status string) lipgloss.Style {
	switch statusSeverity(status) {
	case sevDefault:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Background(BaseBg)
	case sevRunning:
		return StatusRunning
	case sevDone:
		return StatusOther
	case sevProgressing:
		return StatusProgressing
	case sevNormal:
		return DimStyle
	case sevFailed:
		return StatusFailed
	case sevBlank:
		return NormalStyle
	default: // sevUnknown
		return StatusOther
	}
}

// StatusSeverityRank orders a status string worst-first (0 = most severe) for
// status rollups and summaries. Derived from statusSeverity so it tracks
// StatusStyle and works regardless of color profile.
func StatusSeverityRank(status string) int {
	switch statusSeverity(status) {
	case sevFailed:
		return 0
	case sevProgressing:
		return 2
	case sevRunning:
		return 3
	default: // done / normal / default / blank / unknown
		return 4
	}
}

// StatusSortRank orders a status string healthy-first (0 = healthy) for the
// Status column sort, where ascending has always put Running at the top.
// Completed work ranks below both live and broken rows so a wall of Succeeded
// cron pods never buries them. Derived from statusSeverity, so every status the
// coloring understands gets a real bucket — the catch-all is reserved for rows
// carrying no signal at all.
func StatusSortRank(status string) int {
	switch statusSeverity(status) {
	case sevRunning:
		return 0
	case sevProgressing:
		return 1
	case sevFailed:
		return 2
	case sevDone:
		return 3
	default: // normal / default / blank / unknown
		return 4
	}
}
