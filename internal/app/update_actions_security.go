// Package app — update_actions_security.go
package app

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/security"
	"github.com/janosmiko/lfk/internal/ui"
)

// executeActionSecurityFindings handles the "Security Findings" action.
// Surfaces the per-resource finding count as a status message.
//
// The action is intentionally lightweight: it does not navigate or open
// an overlay. Users who want the full breakdown drill into Security from
// the sidebar; this action is a glance-level "is there anything to look
// at on this resource?" indicator.
func (m Model) executeActionSecurityFindings() Model {
	if m.securityIndex == nil {
		m.setStatusMessage("Security findings still loading…", false)
		return m
	}
	// Count exactly what the SEC row badge shows: the selected item's own
	// findings merged with its owner resources' (owner:N) findings. A bare
	// actionCtx-ref query would miss owner-attributed findings (e.g. trivy
	// CVEs on the Deployment that the Pod's badge surfaces), making the action
	// and the badge disagree. Fall back to the actionCtx ref when no row is
	// selected.
	kind, name := m.actionCtx.kind, m.actionCtx.name
	var counts security.SeverityCounts
	if sel := m.selectedMiddleItem(); sel != nil {
		counts = ui.MergedSecurityCounts(m.securityIndex, sel)
		kind, name = sel.Kind, sel.Name
	} else {
		counts = m.securityIndex.For(security.ResourceRef{
			Namespace: m.actionCtx.namespace,
			Kind:      m.actionCtx.kind,
			Name:      m.actionCtx.name,
		})
	}
	total := counts.Total()
	if total == 0 {
		m.setStatusMessage(fmt.Sprintf("No security findings on %s/%s", kind, name), false)
		return m
	}
	parts := []string{}
	if counts.Critical > 0 {
		parts = append(parts, fmt.Sprintf("%d critical", counts.Critical))
	}
	if counts.High > 0 {
		parts = append(parts, fmt.Sprintf("%d high", counts.High))
	}
	if counts.Medium > 0 {
		parts = append(parts, fmt.Sprintf("%d medium", counts.Medium))
	}
	if counts.Low > 0 {
		parts = append(parts, fmt.Sprintf("%d low", counts.Low))
	}
	summary := summariseSeverityParts(parts)
	if summary == "" {
		m.setStatusMessage(fmt.Sprintf("%d security findings on %s/%s — open Security category for details",
			total, kind, name), false)
		return m
	}
	m.setStatusMessage(fmt.Sprintf("%d security findings (%s) on %s/%s — open Security category for details",
		total, summary, kind, name), false)
	return m
}

// summariseSeverityParts joins severity-count fragments with commas.
// Returns "" for an empty slice so callers can omit the parens block
// without a special case.
func summariseSeverityParts(parts []string) string {
	return strings.Join(parts, ", ")
}
