package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// ActiveSecurityIndex is set by the app layer before rendering the explorer
// table. When non-nil and ActiveSecurityAvailable is true, each eligible row
// is decorated with a severity badge summarising its findings. Clusters
// without any security source available leave this nil / false so rendering
// is identical to the pre-security behaviour.
var ActiveSecurityIndex *security.FindingIndex

// ActiveSecurityAvailable gates whether the security badge is shown. It is
// set from Model.securityAvailable by the app layer. When false the badge
// must not be rendered even if an index happens to be populated.
var ActiveSecurityAvailable bool

// Severity badge symbols (monochrome width = 1 each).
const (
	securityBadgeCritical = "\u25cf" // ● filled circle
	securityBadgeHigh     = "\u25d0" // ◐ half circle
	securityBadgeLowMed   = "\u25cb" // ○ empty circle
)

// securityBadgeFor returns a styled severity badge for the given resource,
// or an empty string when idx is nil or the resource has zero findings.
// The badge format is "<symbol><total>" (e.g. "●3"), with color driven by
// the highest severity in the bucket.
func securityBadgeFor(idx *security.FindingIndex, ref security.ResourceRef) string {
	if idx == nil {
		return ""
	}
	counts := idx.For(ref)
	if counts.Total() == 0 {
		return ""
	}
	sym, style := securityBadgeSymbolStyle(counts.Highest())
	if sym == "" {
		return ""
	}
	return style.Render(sym + strconv.Itoa(counts.Total()))
}

// securityBadgePlain returns the plain (ANSI-free) text that
// securityBadgeFor would produce for the given counts. Exported only to the
// package for width math and test assertions.
func securityBadgePlain(counts security.SeverityCounts) string {
	total := counts.Total()
	if total == 0 {
		return ""
	}
	sym, _ := securityBadgeSymbolStyle(counts.Highest())
	if sym == "" {
		return ""
	}
	return sym + strconv.Itoa(total)
}

// securityBadgeSymbolStyle maps a severity to a (symbol, style) pair. The
// mapping is intentionally narrow: Medium and Low share the low-priority
// ○ glyph because the explorer row badge is a glance-level indicator, not a
// precise tier. Callers drill into the H dashboard for the full breakdown.
func securityBadgeSymbolStyle(sev security.Severity) (string, lipgloss.Style) {
	switch sev {
	case security.SeverityCritical:
		return securityBadgeCritical, StatusFailed
	case security.SeverityHigh:
		return securityBadgeHigh, DeprecationStyle
	case security.SeverityMedium, security.SeverityLow:
		return securityBadgeLowMed, StatusProgressing
	default:
		return "", lipgloss.NewStyle()
	}
}

// itemSecurityRef builds a ResourceRef from a model.Item using the fields the
// index keys on. Container is intentionally left empty (see ResourceRef.Key).
func itemSecurityRef(item *model.Item) security.ResourceRef {
	if item == nil {
		return security.ResourceRef{}
	}
	return security.ResourceRef{
		Namespace: item.Namespace,
		Kind:      item.Kind,
		Name:      item.Name,
	}
}

// securityBadgeForItem returns the styled badge for an item, honoring the
// ActiveSecurityAvailable gate. Returns empty string when gated off or the
// item has no matching findings. Intended for use by row formatters.
//
// For items with owner references (e.g., Pods owned by Deployments),
// findings for the owner are included so that trivy-operator findings
// (which reference the Deployment, not individual Pods) surface on
// Pod rows too.
func securityBadgeForItem(item *model.Item) string {
	if !ActiveSecurityAvailable || ActiveSecurityIndex == nil || item == nil {
		return ""
	}
	counts := itemSecurityCounts(item)
	if counts.Total() == 0 {
		return ""
	}
	sym, style := securityBadgeSymbolStyle(counts.Highest())
	if sym == "" {
		return ""
	}
	return style.Render(sym + strconv.Itoa(counts.Total()))
}

// securityBadgePlainForItem returns the plain text badge for an item, used
// when computing the width budget for the name column so the styled badge
// slots in alongside the resource name without clipping.
func securityBadgePlainForItem(item *model.Item) string {
	if !ActiveSecurityAvailable || ActiveSecurityIndex == nil || item == nil {
		return ""
	}
	counts := itemSecurityCounts(item)
	return securityBadgePlain(counts)
}

// itemSecurityCounts returns the merged SeverityCounts for an item,
// combining findings for the item itself with findings for its owner
// resources (extracted from owner:N columns).
func itemSecurityCounts(item *model.Item) security.SeverityCounts {
	counts := ActiveSecurityIndex.For(itemSecurityRef(item))
	// Also check owner references so trivy findings (which reference the
	// Deployment, not the Pod) show on Pod rows.
	for _, col := range item.Columns {
		if len(col.Key) < 6 || col.Key[:6] != "owner:" {
			continue
		}
		parts := strings.SplitN(col.Value, "||", 3)
		if len(parts) != 3 {
			continue
		}
		ownerKind, ownerName := parts[1], parts[2]
		ownerRef := security.ResourceRef{
			Namespace: item.Namespace,
			Kind:      ownerKind,
			Name:      ownerName,
		}
		oc := ActiveSecurityIndex.For(ownerRef)
		counts.Critical += oc.Critical
		counts.High += oc.High
		counts.Medium += oc.Medium
		counts.Low += oc.Low
	}
	return counts
}
