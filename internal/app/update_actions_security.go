// Package app — update_actions_security.go
package app

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/security"
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
	ref := security.ResourceRef{
		Namespace: m.actionCtx.namespace,
		Kind:      m.actionCtx.kind,
		Name:      m.actionCtx.name,
	}
	counts := m.securityIndex.For(ref)
	total := counts.Total()
	if total == 0 {
		m.setStatusMessage(fmt.Sprintf("No security findings on %s/%s", m.actionCtx.kind, m.actionCtx.name), false)
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
	m.setStatusMessage(fmt.Sprintf("%d security findings (%s) on %s/%s — open Security category for details",
		total, summary, m.actionCtx.kind, m.actionCtx.name), false)
	return m
}

// summariseSeverityParts joins severity-count fragments with commas.
// Returns "" for an empty slice so callers can omit the parens block
// without a special case.
func summariseSeverityParts(parts []string) string {
	return strings.Join(parts, ", ")
}
