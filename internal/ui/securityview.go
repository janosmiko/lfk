// Package ui security view: pure-function rendering for the Security dashboard.
// The caller (internal/app) owns the state; this file only produces strings.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/security"
)

// SecurityViewState is the complete state needed to render the dashboard.
// It is populated by internal/app from security.Manager results and key events.
type SecurityViewState struct {
	// Data.
	Findings            []security.Finding
	AvailableCategories []security.Category
	ActiveCategory      security.Category

	// Interaction.
	Cursor         int
	Scroll         int
	ShowDetail     bool
	Filter         string
	FilterFocus    bool
	ResourceFilter *security.ResourceRef

	// Status.
	Loading   bool
	LastError error
	LastFetch time.Time
}

// severityCounts returns severity bucket counts across all findings (not just
// the active category).
func (s SecurityViewState) severityCounts() map[security.Severity]int {
	m := map[security.Severity]int{}
	for _, f := range s.Findings {
		m[f.Severity]++
	}
	return m
}

// visibleFindings returns findings filtered to the active category and,
// if set, the resource filter. Text filter is also applied if non-empty.
func (s SecurityViewState) visibleFindings() []security.Finding {
	if len(s.Findings) == 0 {
		return nil
	}
	out := make([]security.Finding, 0, len(s.Findings))
	for _, f := range s.Findings {
		if s.ActiveCategory != "" && f.Category != s.ActiveCategory {
			continue
		}
		if s.ResourceFilter != nil && f.Resource.Key() != s.ResourceFilter.Key() {
			continue
		}
		if s.Filter != "" && !matchFilter(f, s.Filter) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// VisibleFindings returns findings for the active category/filter for callers
// outside this package (internal/app needs this for cursor bounds).
func (s SecurityViewState) VisibleFindings() []security.Finding {
	return s.visibleFindings()
}

// matchFilter performs a case-insensitive substring match.
func matchFilter(f security.Finding, filter string) bool {
	needle := lowerAscii(filter)
	haystacks := []string{f.Title, f.Summary, f.Source, f.Resource.Name, f.Resource.Namespace}
	for _, h := range haystacks {
		if containsLower(h, needle) {
			return true
		}
	}
	return false
}

// lowerAscii lowercases ASCII only — avoids importing unicode.
func lowerAscii(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func containsLower(h, needleLower string) bool {
	if needleLower == "" {
		return true
	}
	hl := lowerAscii(h)
	for i := 0; i+len(needleLower) <= len(hl); i++ {
		if hl[i:i+len(needleLower)] == needleLower {
			return true
		}
	}
	return false
}

// updatedAgo returns a human-readable "12s ago" style string.
func (s SecurityViewState) updatedAgo() string {
	if s.LastFetch.IsZero() {
		return "never"
	}
	d := time.Since(s.LastFetch).Round(time.Second)
	return fmt.Sprintf("%s ago", d)
}

// -----------------------------------------------------------------------------
// F2 — severity tiles
// -----------------------------------------------------------------------------

// renderSeverityTiles returns a single-line tile row with severity counts.
// The tiles use existing theme styles so colors respect the user's theme.
// When width < 40 the function falls back to a compact single-line list.
func renderSeverityTiles(state SecurityViewState, width int) string {
	counts := state.severityCounts()
	critical := counts[security.SeverityCritical]
	high := counts[security.SeverityHigh]
	medium := counts[security.SeverityMedium]
	low := counts[security.SeverityLow]

	if width < 40 {
		return fmt.Sprintf("CRIT %d  HIGH %d  MED %d  LOW %d", critical, high, medium, low)
	}

	parts := []string{
		styleCritical(fmt.Sprintf(" CRIT %3d ", critical)),
		styleHigh(fmt.Sprintf(" HIGH %3d ", high)),
		styleMedium(fmt.Sprintf(" MED  %3d ", medium)),
		styleLow(fmt.Sprintf(" LOW  %3d ", low)),
	}
	return strings.Join(parts, "  ")
}

// styleCritical/High/Medium/Low are small wrappers that use existing theme
// styles. They are defined once here so all severity rendering goes through
// one place.
//
// Note: There is no dedicated StatusWarning style in styles.go, so HIGH uses
// DeprecationStyle which is foregrounded with ColorWarning (orange). CRIT uses
// StatusFailed (red error), MED uses StatusProgressing (blue primary), and
// LOW uses StatusRunning (green secondary).
func styleCritical(s string) string { return StatusFailed.Render(s) }
func styleHigh(s string) string     { return DeprecationStyle.Render(s) }
func styleMedium(s string) string   { return StatusProgressing.Render(s) }
func styleLow(s string) string      { return StatusRunning.Render(s) }

// -----------------------------------------------------------------------------
// F3 — category tab strip
// -----------------------------------------------------------------------------

// categoryLabel returns the short display label for a category.
func categoryLabel(c security.Category) string {
	switch c {
	case security.CategoryVuln:
		return "Vulns"
	case security.CategoryMisconfig:
		return "Misconf"
	case security.CategoryPolicy:
		return "Policies"
	case security.CategoryCompliance:
		return "CIS"
	}
	return string(c)
}

// renderTabStrip returns the category tab row, or empty if there's one or zero
// available categories.
func renderTabStrip(state SecurityViewState) string {
	if len(state.AvailableCategories) <= 1 {
		return ""
	}
	perCat := map[security.Category]int{}
	for _, f := range state.Findings {
		perCat[f.Category]++
	}
	var parts []string
	for _, c := range state.AvailableCategories {
		label := fmt.Sprintf("%s %d", categoryLabel(c), perCat[c])
		if c == state.ActiveCategory {
			parts = append(parts, "["+label+"]")
		} else {
			parts = append(parts, " "+label+" ")
		}
	}
	return strings.Join(parts, "  ")
}

// -----------------------------------------------------------------------------
// F4 — findings table
// -----------------------------------------------------------------------------

// kindShort returns the abbreviated kind name used in the table.
func kindShort(kind string) string {
	switch kind {
	case "Deployment":
		return "deploy"
	case "StatefulSet":
		return "sts"
	case "DaemonSet":
		return "ds"
	case "ReplicaSet":
		return "rs"
	case "Job":
		return "job"
	case "CronJob":
		return "cron"
	case "Pod":
		return "pod"
	}
	return kind
}

func severityShort(s security.Severity) string {
	switch s {
	case security.SeverityCritical:
		return "CRIT"
	case security.SeverityHigh:
		return "HIGH"
	case security.SeverityMedium:
		return "MED"
	case security.SeverityLow:
		return "LOW"
	}
	return "?"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

// renderFindingsTable produces the body table. maxRows is the viewport height
// for findings rows (excluding the header row).
func renderFindingsTable(state SecurityViewState, width, maxRows int) string {
	visible := state.visibleFindings()
	if len(visible) == 0 {
		return DimStyle.Render("  No security findings")
	}

	showSource := width >= 80
	showNamespace := width >= 60

	var b strings.Builder
	if showSource {
		b.WriteString("  SEV   RESOURCE                 SOURCE        TITLE\n")
	} else {
		b.WriteString("  SEV   RESOURCE                 TITLE\n")
	}

	start := state.Scroll
	end := start + maxRows
	if end > len(visible) {
		end = len(visible)
	}

	for i := start; i < end; i++ {
		f := visible[i]
		marker := "  "
		if i == state.Cursor {
			marker = "> "
		}
		resource := fmt.Sprintf("%s/%s", kindShort(f.Resource.Kind), f.Resource.Name)
		if showNamespace {
			resource += fmt.Sprintf("  (%s)", f.Resource.Namespace)
		}
		var row string
		if showSource {
			row = fmt.Sprintf("%s%-5s %-24s %-12s %s",
				marker, severityShort(f.Severity), truncate(resource, 24),
				truncate(f.Source, 12), truncate(f.Title, 40))
		} else {
			titleMax := width - 32
			if titleMax < 10 {
				titleMax = 10
			}
			row = fmt.Sprintf("%s%-5s %-24s %s",
				marker, severityShort(f.Severity), truncate(resource, 24),
				truncate(f.Title, titleMax))
		}
		b.WriteString(row)
		b.WriteString("\n")
	}
	return b.String()
}

// -----------------------------------------------------------------------------
// F5 — details pane + status overlay
// -----------------------------------------------------------------------------

// renderDetailsPane returns the detail panel for the currently selected finding,
// or an empty string if ShowDetail is false.
func renderDetailsPane(state SecurityViewState, width int) string {
	if !state.ShowDetail {
		return ""
	}
	visible := state.visibleFindings()
	if len(visible) == 0 || state.Cursor >= len(visible) {
		return ""
	}
	f := visible[state.Cursor]

	headerWidth := width - 11
	if headerWidth < 0 {
		headerWidth = 0
	}

	var b strings.Builder
	b.WriteString("─ Details ")
	b.WriteString(strings.Repeat("─", headerWidth))
	b.WriteString("\n")
	fmt.Fprintf(&b, " %s    %s\n", f.Title, severityShort(f.Severity))
	fmt.Fprintf(&b, " Source:   %s\n", f.Source)
	if f.Resource.Name != "" {
		fmt.Fprintf(&b, " Resource: %s/%s/%s\n",
			f.Resource.Namespace, kindShort(f.Resource.Kind), f.Resource.Name)
	}
	if f.Summary != "" {
		b.WriteString(" Summary:  ")
		b.WriteString(f.Summary)
		b.WriteString("\n")
	}
	if f.Details != "" {
		b.WriteString("\n")
		b.WriteString(f.Details)
		b.WriteString("\n")
	}
	if len(f.References) > 0 {
		b.WriteString("\n References:\n")
		for _, r := range f.References {
			b.WriteString("  - ")
			b.WriteString(r)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// renderSecurityStatusOverlay returns a status message for loading, error, and
// no-sources states. Returns empty string when the dashboard should render normally.
func renderSecurityStatusOverlay(state SecurityViewState) string {
	if state.Loading {
		return DimStyle.Render("  Loading security findings...")
	}
	if state.LastError != nil {
		return fmt.Sprintf("  Error: %s\n  Press r to retry", state.LastError.Error())
	}
	if len(state.AvailableCategories) == 0 {
		return DimStyle.Render("  No security sources available.\n  Install Trivy Operator to enable vulnerability scanning.")
	}
	return ""
}

// -----------------------------------------------------------------------------
// F6 — main composition
// -----------------------------------------------------------------------------

// RenderSecurityDashboard composes the header, tiles, tab strip, findings
// table, and details pane into a single string sized to (width, height).
// This is the only exported render function for the security view.
func RenderSecurityDashboard(state SecurityViewState, width, height int) string {
	if overlay := renderSecurityStatusOverlay(state); overlay != "" && len(state.Findings) == 0 {
		return overlay
	}

	var b strings.Builder

	// Header.
	header := "Security Dashboard"
	if state.ResourceFilter != nil {
		header = "Security: " + state.ResourceFilter.Key() + "    [C] clear filter"
	}
	b.WriteString(header)
	b.WriteString("    ")
	b.WriteString(DimStyle.Render("updated "))
	b.WriteString(DimStyle.Render(state.updatedAgo()))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", width))
	b.WriteString("\n")

	// Tiles.
	b.WriteString(renderSeverityTiles(state, width))
	b.WriteString("\n\n")

	// Tab strip.
	if tabs := renderTabStrip(state); tabs != "" {
		b.WriteString(tabs)
		b.WriteString("\n")
	}
	b.WriteString(strings.Repeat("─", width))
	b.WriteString("\n")

	// Table — reserve rows for header/details/hint.
	reserved := 8 // header + tiles + separators
	if state.ShowDetail {
		reserved += 8
	}
	maxRows := height - reserved
	if maxRows < 1 {
		maxRows = 1
	}
	b.WriteString(renderFindingsTable(state, width, maxRows))

	// Details pane.
	if state.ShowDetail {
		b.WriteString("\n")
		b.WriteString(renderDetailsPane(state, width))
	}

	// Hint bar.
	b.WriteString("\n")
	hint := "[Tab]tab [Enter]det [r]refresh [J/K]cursor [F]fullscreen"
	if state.ResourceFilter != nil {
		hint = "[Tab]tab [Enter]det [r]refresh [C]clear [J/K]cursor [F]fullscreen"
	}
	b.WriteString(DimStyle.Render(hint))

	return b.String()
}
