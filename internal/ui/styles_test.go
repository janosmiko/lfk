package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

// --- AgeStyle ---

func TestAgeStyle(t *testing.T) {
	// Helper to extract a comparable foreground color key from a style.
	fgKey := func(s lipgloss.Style) string {
		fg := s.GetForeground()
		r, g, b, a := fg.RGBA()
		return fmt.Sprintf("%d:%d:%d:%d", r, g, b, a)
	}

	dimFg := fgKey(DimStyle)
	cyanFg := fgKey(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)))
	greenFg := fgKey(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)))
	borderFg := fgKey(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)))

	tests := []struct {
		name       string
		age        string
		expectedFg string
		desc       string
	}{
		// Empty returns DimStyle.
		{"empty string", "", dimFg, "dim"},

		// Seconds: very new -> cyan.
		{"5 seconds", "5s", cyanFg, "cyan"},
		{"30 seconds", "30s", cyanFg, "cyan"},

		// Minutes: very new -> cyan.
		{"1 minute", "1m", cyanFg, "cyan"},
		{"59 minutes", "59m", cyanFg, "cyan"},

		// Hours < 24: recent -> green.
		{"1 hour", "1h", greenFg, "green"},
		{"12 hours", "12h", greenFg, "green"},
		{"23 hours", "23h", greenFg, "green"},

		// Hours >= 24: dim.
		{"24 hours", "24h", dimFg, "dim"},
		{"48 hours", "48h", dimFg, "dim"},

		// Days <= 7: dim.
		{"1 day", "1d", dimFg, "dim"},
		{"7 days", "7d", dimFg, "dim"},

		// Days > 7: extra dim (border color).
		{"8 days", "8d", borderFg, "border"},
		{"30 days", "30d", borderFg, "border"},
		{"365 days", "365d", borderFg, "border"},

		// Years: old -> border.
		{"1 year", "1y", borderFg, "border"},

		// Parse error returns dim.
		{"invalid number", "xm", dimFg, "dim"},
		{"no number", "m", dimFg, "dim"},

		// Unknown unit returns dim.
		{"unknown unit", "5x", dimFg, "dim"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := AgeStyle(tt.age)
			got := fgKey(style)
			assert.Equal(t, tt.expectedFg, got, "age=%q expected %s style", tt.age, tt.desc)
		})
	}
}

// --- ConditionStyle ---

func TestConditionStyle(t *testing.T) {
	fgKey := func(s lipgloss.Style) string {
		fg := s.GetForeground()
		r, g, b, a := fg.RGBA()
		return fmt.Sprintf("%d:%d:%d:%d", r, g, b, a)
	}
	red := fgKey(StatusFailed)
	green := fgKey(StatusRunning)
	amber := fgKey(StatusWarning)
	blue := fgKey(StatusProgressing)
	dim := fgKey(DimStyle)

	tests := []struct {
		condType string
		status   string
		want     string
		desc     string
	}{
		// ArgoCD application conditions are status-less; the type encodes severity.
		{"ComparisonError", "", red, "argocd error"},
		{"InvalidSpecError", "", red, "argocd error"},
		{"SharedResourceWarning", "", amber, "argocd warning"},
		{"OrphanedResourceWarning", "", amber, "argocd warning"},

		// external-secrets / cert-manager / Flux readiness conditions.
		{"SecretSynced", "True", green, "good when True"},
		{"Ready", "True", green, "good when True"},
		{"Ready", "False", red, "bad when False"},

		// cert-manager Issuing: True is in-progress, False is the normal idle state.
		{"Issuing", "True", blue, "in progress"},
		{"Issuing", "False", dim, "idle is neutral, not an error"},

		// Negative-polarity types invert: False is healthy.
		{"Degraded", "True", red, "failing"},
		{"Degraded", "False", green, "not degraded = healthy"},
		{"Stalled", "True", red, "flux stalled (curated)"},

		// Node pressure conditions (heuristic): True is bad.
		{"MemoryPressure", "True", red, "pressure is bad"},
		{"MemoryPressure", "False", green, "no pressure = healthy"},

		// Warning suffix wins regardless of status.
		{"DeprecationWarning", "True", amber, "warning suffix"},

		// Unknown status is always neutral.
		{"Ready", "Unknown", dim, "unknown is neutral"},
		{"ComparisonError", "Unknown", dim, "unknown is neutral"},

		// Unrecognized informational type: True in-progress, False neutral.
		{"CustomFlag", "True", blue, "info True"},
		{"CustomFlag", "False", dim, "info False"},

		// Negated single-token types must NOT match a ready keyword by substring
		// (token-prefix matching): "Unbound"/"Incomplete" fall through to info.
		{"Unbound", "True", blue, "not classified as ready (no substring inversion)"},
		{"Incomplete", "True", blue, "not classified as ready (no substring inversion)"},
		{"Unavailable", "True", blue, "not classified as ready (no substring inversion)"},
		// Multi-token positives still match the relevant token.
		{"ContainersReady", "False", red, "ready token matched, False = problem"},
		{"JobComplete", "True", green, "complete token matched, True = good"},
	}
	for _, tt := range tests {
		t.Run(tt.condType+"/"+tt.status, func(t *testing.T) {
			got := fgKey(ConditionStyle(tt.condType, tt.status))
			assert.Equal(t, tt.want, got, "%s %q: expected %s", tt.condType, tt.status, tt.desc)
		})
	}
}

// --- statusSeverity: Established ---

// "Established" is the healthy terminal state of a CRD (Established: True
// condition), so it must classify as running-green in the built-in Status
// column and rank as healthy in status rollups.
func TestStatusSeverity_Established(t *testing.T) {
	assert.Equal(t, sevRunning, statusSeverity("Established"))
	assert.Equal(t, StatusSeverityRank("Running"), StatusSeverityRank("Established"))
}

// --- statusSeverity: free-form phrases ---

// Operators that break the CamelCase-phase convention (e.g. CloudNativePG's
// "Cluster in healthy state", "Failing over") must still color by severity in
// status columns and summary bars, not fall through to the gray unknown bucket
// (issue #536 follow-up). Classification is word-based on the unknown-status
// fallback path; exact matches above keep precedence.
func TestStatusSeverity_FreeFormPhrases(t *testing.T) {
	tests := []struct {
		status string
		want   statusSev
	}{
		// CloudNativePG cluster phases.
		{"Cluster in healthy state", sevRunning},
		{"Failing over", sevFailed},
		{"Failing over to replica", sevFailed},
		{"Setting up primary", sevProgressing},
		{"Creating a new replica", sevProgressing},
		{"Waiting for the instances to become active", sevProgressing},
		{"Upgrading cluster", sevProgressing},
		{"Switchover in progress", sevProgressing},
		{"Primary instance is being restarted in-place", sevProgressing},
		// Worst word wins within a phrase.
		{"Healthy but degraded", sevFailed},
		// Single unknown words classify too.
		{"Unhealthy", sevFailed},
		// Negated positives are amber, mirroring the exact-match "NotReady".
		{"Not ready", sevProgressing},
		{"Not healthy", sevProgressing},
		{"Cluster not ready", sevProgressing},
		// Negation only flips the word it precedes.
		{"Ready or not", sevRunning},
		// No recognized word stays unknown (gray).
		{"Jumping", sevUnknown},
		{"Some bespoke text", sevUnknown},
		// Exact matches keep precedence over word scanning.
		{"Failed", sevFailed},
		{"Running", sevRunning},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			assert.Equal(t, tt.want, statusSeverity(tt.status))
		})
	}
}

// --- FillLinesBg ---

// TestFillLinesBgReestablishesBgAfterShortReset guards issue #293's recurrence
// in the dashboard events column. lipgloss/reflow emit the parameterless SGR
// reset (ESC[m) at word-wrap boundaries, not the full ESC[0m. FillLinesBg must
// re-apply the background after both, or column padding following a
// wrap-induced reset renders with the terminal default (a black "tear").
func TestFillLinesBgReestablishesBgAfterShortReset(t *testing.T) {
	origProfile := lipgloss.DefaultRenderer().ColorProfile()
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(origProfile) })

	// Use an explicit color (not the theme-dependent BaseBg) so the test does
	// not depend on global theme state another test may have mutated.
	bg := lipgloss.Color("#1a1a2e")

	// Derive the exact bg sequence FillLinesBg injects, and guard against a
	// vacuous pass: a downgraded color profile would emit no sequence.
	sample := lipgloss.NewStyle().Background(bg).Render("X")
	bgSeq, _, _ := strings.Cut(sample, "X")
	if bgSeq == "" {
		t.Fatal("color profile must emit a background sequence for this test to be meaningful")
	}

	// A styled span closed by the parameterless reset, followed by spaces, and
	// already at full width so no trailing fill is appended — the wrap boundary
	// lipgloss produces (interior column padding follows the reset). The old
	// code left these spaces un-backgrounded.
	content := "\x1b[38;2;1;2;3mhello\x1b[m     "
	got := FillLinesBg(content, 10, bg)

	assert.Contains(t, got, "\x1b[m"+bgSeq,
		"background must be re-established immediately after the parameterless reset")
	assert.NotContains(t, got, "\x1b[m ",
		"no un-backgrounded padding may follow a parameterless reset")
}

// --- StatusSortRank ---

// StatusSortRank drives the Status column sort. It orders healthy-first
// (matching the column's long-standing ascending order) and, unlike the old
// hand-maintained table in internal/app, derives every bucket from
// statusSeverity — so a status the coloring understands can never fall into an
// unranked catch-all and sort by name instead.
func TestStatusSortRank(t *testing.T) {
	tests := []struct {
		status string
		want   int
	}{
		// 0: healthy.
		{"Running", 0},
		{"Active", 0},
		{"Ready", 0},
		{"Healthy/Synced", 0},
		{"Established", 0},
		// 1: in progress, including the statuses the old table never ranked.
		{"Pending", 1},
		{"ContainerCreating", 1},
		{"Terminating", 1},
		{"NotReady", 1},
		{"Unknown", 1},
		{"Suspended", 1},
		// 2: failed.
		{"Failed", 2},
		{"CrashLoopBackOff", 2},
		{"OOMKilled", 2},
		{"Degraded/OutOfSync", 2},
		// 3: terminal success — completed rows are noise, so they sit below
		// anything still running or broken instead of burying it.
		{"Succeeded", 3},
		{"Completed", 3},
		{"Superseded", 3},
		// 4: no signal.
		{"", 4},
		{"Normal", 4},
		{"SomeRandomStatus", 4},
	}
	for _, tt := range tests {
		name := tt.status
		if name == "" {
			name = "empty string"
		}
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, StatusSortRank(tt.status))
		})
	}
}

// Free-form CRD phrases must bucket through the same word fallback the
// coloring uses, so an operator's "Cluster in healthy state" sorts with the
// healthy rows rather than dropping into the no-signal bucket.
func TestStatusSortRank_FreeFormPhrases(t *testing.T) {
	assert.Equal(t, StatusSortRank("Running"), StatusSortRank("Cluster in healthy state"))
	assert.Equal(t, StatusSortRank("Failed"), StatusSortRank("Failing over"))
	assert.Equal(t, StatusSortRank("Pending"), StatusSortRank("Setting up primary"))
}
