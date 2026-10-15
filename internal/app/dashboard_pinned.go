// Package app - dashboard_pinned.go
// Inline pinned-summary rows on the cluster dashboard (issue #525, Task 10):
// each pinned resource type's status rollup renders as regular dashboard
// metric rows directly below the Pods row, using the same bar/label machinery
// as Nodes/Pods - no separate section, no header.
package app

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/janosmiko/lfk/internal/ui"
)

// pinnedRowLabel is the label a pinned kind's first dashboard row uses: its
// resolved display name, falling back to the raw pin key when discovery
// didn't supply one (e.g. a notFound placeholder for a key absent from this
// cluster).
func pinnedRowLabel(r pinnedSummaryResult) string {
	if r.displayName != "" {
		return r.displayName
	}
	return r.key
}

// maxPinnedLabelWidth returns the widest pinned label (kind or "├ <dim>"), so
// every pinned row's bar aligns with Nodes/Pods.
func maxPinnedLabelWidth(data dashboardData) int {
	maxW := 0
	widen := func(s string) {
		if w := lipgloss.Width(s); w > maxW {
			maxW = w
		}
	}
	for _, r := range data.pinnedSummaries {
		if len(r.summary.Bars) < 2 {
			widen(pinnedRowLabel(r))
			continue
		}
		// The kind name gets its own header line, outside the label column.
		for _, bar := range r.summary.Bars {
			widen("├ " + bar.Label)
		}
	}
	return maxW
}

// dashboardPinnedRows renders one dashboard metric row per pinned summary
// dimension, in pin order, directly following the Pods row inside the same
// CLUSTER DASHBOARD block - no separator, no header. A pin whose type is
// absent from this cluster's discovery renders a dim one-line "(not installed
// in this cluster)" note instead of a bar; a failed list call renders
// "(unavailable)".
func dashboardPinnedRows(lines []string, data dashboardData, w dashboardWidths) []string {
	if len(data.pinnedSummaries) == 0 {
		return lines
	}
	// The fan-out delivers sections in arrival order; sort a copy by pin
	// index (never in place - the slice is shared via m.dashboardData).
	results := append([]pinnedSummaryResult(nil), data.pinnedSummaries...)
	sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })

	for _, r := range results {
		lines = append(lines, dashboardPinnedRowLines(r, w)...)
	}
	return lines
}

// dashboardPinnedRowLines renders one pinned kind's row(s): a notFound or
// error placeholder, a single zero-total row, or one row per summary
// dimension.
func dashboardPinnedRowLines(r pinnedSummaryResult, w dashboardWidths) []string {
	label := pinnedRowLabel(r)
	switch {
	case r.notFound:
		return []string{"  " + ui.DimStyle.Bold(true).Render(ui.Truncate(label, w.label)) +
			" " + ui.DimStyle.Render("(not installed in this cluster)")}
	case r.err != nil:
		return []string{"  " + ui.DimStyle.Bold(true).Render(ui.Truncate(label, w.label)) +
			" " + ui.StatusProgressing.Render("(unavailable)")}
	case len(r.summary.Bars) == 0:
		// Zero-total kind (e.g. 0 Jobs): still render a row - an empty bar
		// and a "0" summary, the same shape as every other metric row.
		bar := stackedMetricBar(nil, 0, w.bar)
		return dashboardMetricLines(label, bar, "0", w)
	case len(r.summary.Bars) == 1:
		dim := r.summary.Bars[0]
		bar := stackedMetricBar(pinnedBarSegments(dim), dim.Total, w.bar)
		return dashboardMetricLines(label, bar, pinnedSummaryLegend(dim), w)
	default:
		// A bare second row (e.g. "Sync" under "Applications") reads as an
		// unrelated metric, so group every dimension as a branch under the kind.
		out := []string{ansi.Truncate("  "+ui.HelpKeyStyle.Render(label), w.content, "")}
		last := len(r.summary.Bars) - 1
		for i, dim := range r.summary.Bars {
			branch := "├ "
			if i == last {
				branch = "└ "
			}
			bar := stackedMetricBar(pinnedBarSegments(dim), dim.Total, w.bar)
			rows := dashboardMetricLines(branch+dim.Label, bar, pinnedSummaryLegend(dim), w)
			if i < last && len(rows) > 1 {
				// The legend line starts with plain spaces: swap the branch
				// column's space for a continuation bar.
				rows[1] = "  │" + rows[1][3:]
			}
			out = append(out, rows...)
		}
		return out
	}
}

// pinnedBarSegments converts a summary dimension's worst-first buckets into
// renderStackedBar segments, colored the same way the resource-type preview
// band colors its summary bar (ui.StatusStyle keyed on the bucket value).
func pinnedBarSegments(dim ui.SummaryBar) []struct {
	count int
	style lipgloss.Style
} {
	segments := make([]struct {
		count int
		style lipgloss.Style
	}, len(dim.Buckets))
	for i, b := range dim.Buckets {
		segments[i].count = b.Count
		segments[i].style = ui.StatusStyle(b.Value)
	}
	return segments
}

// pinnedSummaryLegend renders a dimension's bucket counts as a colored legend
// line, e.g. "1 Degraded  1 Healthy" - mirroring ui's unexported
// renderSummaryLegend so pinned rows read identically to the preview band's
// own summary line.
func pinnedSummaryLegend(dim ui.SummaryBar) string {
	parts := make([]string, 0, len(dim.Buckets))
	for _, b := range dim.Buckets {
		parts = append(parts, ui.StatusStyle(b.Value).Render(fmt.Sprintf("%d %s", b.Count, b.Value)))
	}
	return strings.Join(parts, "  ")
}
