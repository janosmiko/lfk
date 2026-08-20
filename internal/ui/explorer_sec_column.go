package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

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

// ActiveSecurityBadgesHidden suppresses the SEC row badge when the user has
// toggled it off (kb.SecurityBadgeToggle). Set from Model.hideSecurityBadges
// by the app layer each render. Independent of ActiveSecurityAvailable so the
// Security dashboard and source probing keep working while badges are hidden.
var ActiveSecurityBadgesHidden bool

// Severity badge symbols (monochrome width = 1 each).
const (
	securityBadgeCritical = "\u25cf" // ● filled circle
	securityBadgeHigh     = "\u25d0" // ◐ half circle
	securityBadgeLowMed   = "\u25cb" // ○ empty circle
)

// securityBadgeStyled renders the styled badge for the given counts, or an
// empty string when there are no findings. It is the single styling path
// shared by the resource-ref and model.Item entry points.
func securityBadgeStyled(counts security.SeverityCounts) string {
	plain := securityBadgePlain(counts)
	if plain == "" {
		return ""
	}
	_, style := securityBadgeSymbolStyle(counts.Highest())
	return style.Render(plain)
}

// securityBadgePlain returns the plain (ANSI-free) badge text for the given
// counts. It is the single source of truth for the badge string used by both
// the styled renderers and the name-column width math.
//
// The badge shows only the worst-severity count (e.g. "●3" = 3 criticals), so
// a red "●" badge can never be misread as a total across all severities. The
// full per-tier breakdown lives in the Security dashboard.
func securityBadgePlain(counts security.SeverityCounts) string {
	worst := counts.HighestCount()
	if worst == 0 {
		return ""
	}
	sym, _ := securityBadgeSymbolStyle(counts.Highest())
	if sym == "" {
		return ""
	}
	return sym + strconv.Itoa(worst)
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
	if ActiveSecurityBadgesHidden || !ActiveSecurityAvailable || ActiveSecurityIndex == nil || item == nil {
		return ""
	}
	return securityBadgeStyled(itemSecurityCounts(item))
}

// securityBadgePlainForItem returns the plain text badge for an item, used
// when computing the width budget for the name column so the styled badge
// slots in alongside the resource name without clipping.
func securityBadgePlainForItem(item *model.Item) string {
	if ActiveSecurityBadgesHidden || !ActiveSecurityAvailable || ActiveSecurityIndex == nil || item == nil {
		return ""
	}
	counts := itemSecurityCounts(item)
	return securityBadgePlain(counts)
}

// itemSecurityCounts returns the merged SeverityCounts for an item against the
// active render-time index. Thin wrapper over MergedSecurityCounts.
func itemSecurityCounts(item *model.Item) security.SeverityCounts {
	return MergedSecurityCounts(ActiveSecurityIndex, item)
}

// MergedSecurityCounts returns an item's own findings combined with its owner
// resources' findings (from owner:N columns) — the exact aggregation the SEC
// row badge shows (e.g. a Pod row includes its Deployment's trivy findings).
// Exported so app-layer surfaces such as the "Security Findings" action report
// the same numbers as the badge instead of a bare per-ref count. Returns zero
// counts when idx or item is nil.
func MergedSecurityCounts(idx *security.FindingIndex, item *model.Item) security.SeverityCounts {
	if idx == nil || item == nil {
		return security.SeverityCounts{}
	}
	var counts security.SeverityCounts
	for _, ref := range SecurityRefsForItem(item) {
		c := idx.For(ref)
		counts.Critical += c.Critical
		counts.High += c.High
		counts.Medium += c.Medium
		counts.Low += c.Low
	}
	return counts
}

// SecurityRefsForItem returns the ResourceRefs the SEC badge aggregates over
// for an item: the item's own ref followed by one ref per owner:N column
// (value "APIVersion||Kind||Name", owners share the item's namespace).
// Malformed owner columns are skipped. Returns nil for a nil item. Exported
// so the "Security Findings" action filters its per-resource list by the
// exact same set the badge counts.
func SecurityRefsForItem(item *model.Item) []security.ResourceRef {
	if item == nil {
		return nil
	}
	refs := []security.ResourceRef{itemSecurityRef(item)}
	for _, col := range item.Columns {
		if len(col.Key) < 6 || col.Key[:6] != "owner:" {
			continue
		}
		parts := strings.SplitN(col.Value, "||", 3)
		if len(parts) != 3 {
			continue
		}
		refs = append(refs, security.ResourceRef{
			Namespace: item.Namespace,
			Kind:      parts[1],
			Name:      parts[2],
		})
	}
	return refs
}
