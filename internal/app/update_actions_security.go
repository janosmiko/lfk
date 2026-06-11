// Package app — update_actions_security.go
package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
	"github.com/janosmiko/lfk/internal/ui"
)

// executeActionSecurityFindings handles the "Security Findings" action.
// Opens the per-resource findings view: a LevelResources list of finding
// groups from every source, filtered to the selected resource plus its
// owners — the exact set the SEC row badge aggregates over (a bare per-ref
// query would miss owner-attributed findings, e.g. trivy CVEs on the
// Deployment that the Pod's badge surfaces). The teleport records jump
// history so JumpBack returns to the originating list.
//
// Fallbacks keep the old status-message behavior: index still loading, zero
// findings (nothing to list), and union-sentinel mode (the per-resource view
// filters one cluster's scan, which the sentinel context cannot resolve).
func (m Model) executeActionSecurityFindings() (Model, tea.Cmd) {
	if m.securityIndex == nil {
		m.setStatusMessage("Security findings still loading…", false)
		return m, scheduleStatusClear()
	}
	kind, name := m.actionCtx.kind, m.actionCtx.name
	var counts security.SeverityCounts
	var refs []security.ResourceRef
	if sel := m.selectedMiddleItem(); sel != nil {
		counts = ui.MergedSecurityCounts(m.securityIndex, sel)
		kind, name = sel.Kind, sel.Name
		refs = ui.SecurityRefsForItem(sel)
	} else {
		ref := security.ResourceRef{
			Namespace: m.actionCtx.namespace,
			Kind:      m.actionCtx.kind,
			Name:      m.actionCtx.name,
		}
		counts = m.securityIndex.For(ref)
		refs = []security.ResourceRef{ref}
	}
	total := counts.Total()
	if total == 0 {
		m.setStatusMessage(fmt.Sprintf("No security findings on %s/%s", kind, name), false)
		return m, scheduleStatusClear()
	}
	if m.isUnionSentinel() {
		m.setStatusMessage(securityCountsSummary(total, counts, kind, name), false)
		return m, scheduleStatusClear()
	}
	return m.openSecurityFindingsForResource(refs, kind, name)
}

// openSecurityFindingsForResource teleports to the per-resource findings
// view. The synthetic ResourceTypeEntry encodes the primary ref in Resource
// so navKey-based caches (cursor memory, item cache) stay distinct per
// resource; the actual filter set travels in m.securityResourceFilter.
func (m Model) openSecurityFindingsForResource(refs []security.ResourceRef, kind, name string) (Model, tea.Cmd) {
	m.saveCursor()
	// Record the origin BEFORE any nav mutation so JumpBack restores it.
	m.pushJumpHistory()
	m.unwindToResourcesLevel()
	m.nav.ResourceType = model.ResourceTypeEntry{
		DisplayName: "Findings: " + kind + "/" + name,
		Kind:        model.SecurityResourceFindingsKind,
		APIGroup:    model.SecurityVirtualAPIGroup,
		APIVersion:  "v1",
		Resource:    "findings-resource/" + refs[0].Key(),
		Namespaced:  true,
	}
	m.securityResourceFilter = refs
	m.securityActiveGroup = ""
	m.securityActiveSource = ""
	m.filterText = ""
	m.clearRight()
	m.setMiddleItems(nil)
	m.setCursor(0)
	m.saveCurrentSession()
	m.loading = true
	return m, m.loadResources(false)
}

// unwindToResourcesLevel pops the left-pane stack back to the depth of
// LevelResources so a teleport from a deeper level (pod under a workload,
// container list) leaves the Esc cascade consistent. Each descend performed
// exactly one pushLeft: LevelOwned sits one push below LevelResources plus
// one per nested owned drill (ownedParentStack entry); LevelContainers adds
// one more unless it was entered directly from a Pod list (where the drill
// skipped LevelOwned).
func (m *Model) unwindToResourcesLevel() {
	switch m.nav.Level {
	case model.LevelOwned:
		for range m.ownedParentStack {
			m.popLeft()
		}
		m.popLeft()
	case model.LevelContainers:
		for range m.ownedParentStack {
			m.popLeft()
		}
		if m.nav.ResourceType.Kind != "Pod" {
			m.popLeft()
		}
		m.popLeft()
	}
	m.ownedParentStack = nil
	m.nav.Level = model.LevelResources
	m.nav.ResourceName = ""
	m.nav.OwnedName = ""
}

// securityCountsSummary renders the "<n> security findings (<breakdown>) on
// kind/name" status line used when the action cannot open the filtered view.
func securityCountsSummary(total int, counts security.SeverityCounts, kind, name string) string {
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
		return fmt.Sprintf("%d security findings on %s/%s — open Security category for details",
			total, kind, name)
	}
	return fmt.Sprintf("%d security findings (%s) on %s/%s — open Security category for details",
		total, summary, kind, name)
}

// summariseSeverityParts joins severity-count fragments with commas.
// Returns "" for an empty slice so callers can omit the parens block
// without a special case.
func summariseSeverityParts(parts []string) string {
	return strings.Join(parts, ", ")
}
