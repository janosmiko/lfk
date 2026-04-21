package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// clampPreviewScroll prevents scrolling past the preview content.
// Only details+events scroll; pinned header (children) and footer (resource usage) are excluded.
func (m *Model) clampPreviewScroll() {
	// Compute the right column width exactly as the View function does.
	usable := m.width - 6
	rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
	innerW := rightW - 2

	// Compute the column height exactly as the View function does.
	colHeight := m.height - 4 // title + status bar + column borders
	if colHeight < 3 {
		colHeight = 3
	}
	if len(m.tabs) > 1 {
		colHeight-- // tab bar
	}

	// Compute footer lines (must match renderRightColumn).
	footerLines := 0
	if !m.fullYAMLPreview && m.metricsContent != "" {
		sep := ui.DimStyle.Render(strings.Repeat("\u2500", innerW))
		footer := sep + "\n" + m.metricsContent
		footerLines = strings.Count(footer, "\n") + 1
	}

	// Compute scrollable viewport height (must match renderRightColumn).
	scrollableH := colHeight - footerLines
	if scrollableH < 3 {
		scrollableH = 3
	}

	if m.hasSplitPreview() {
		childrenHeight := (scrollableH - 2) / 3
		if childrenHeight < 2 {
			childrenHeight = 2
		}
		childLabel := strings.ToUpper(m.ownedChildKindLabel())
		pinnedHeader := m.withSessionColumnsForKind(m.rightColumnKind(), func() string {
			return ui.RenderTable(childLabel, m.rightItems, -1, innerW, childrenHeight, m.loading, m.spinner.View(), "", false)
		})
		pinnedHeader += "\n" + ui.DimStyle.Render(strings.Repeat("\u2500", innerW))
		pinnedHeaderLines := strings.Count(pinnedHeader, "\n") + 1
		scrollableH -= pinnedHeaderLines
		if scrollableH < 3 {
			scrollableH = 3
		}
	}

	// Get the scrollable content line count.
	// Use the actual scrollable height (not 10000) to avoid inflated YAML fill.
	measureH := scrollableH * 3 // enough headroom for scroll, but not inflated
	if measureH < 200 {
		measureH = 200
	}
	var totalLines int
	if m.hasSplitPreview() {
		fullContent := m.renderDetailsOnly(innerW, measureH)
		totalLines = strings.Count(fullContent, "\n") + 1
	} else {
		fullContent := m.renderRightColumnContent(innerW, measureH)
		totalLines = strings.Count(fullContent, "\n") + 1
	}

	// Include events in the scrollable content (they scroll with details).
	if !m.fullYAMLPreview && m.previewEventsContent != "" {
		totalLines += 1 + strings.Count(m.previewEventsContent, "\n") + 1 // separator + event lines
	}

	maxScroll := totalLines - scrollableH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.previewScroll > maxScroll {
		m.previewScroll = maxScroll
	}
}

func (m Model) renderRightColumn(width, height int) string {
	// Build the pinned footer: only resource usage (metrics) is pinned.
	var footerParts []string
	if !m.fullYAMLPreview && m.metricsContent != "" {
		footerParts = append(footerParts,
			ui.DimStyle.Render(strings.Repeat("\u2500", width)),
			m.metricsContent)
	}

	// Reserve height for the pinned footer.
	footerLines := 0
	footer := ""
	if len(footerParts) > 0 {
		footer = strings.Join(footerParts, "\n")
		footerLines = strings.Count(footer, "\n") + 1
	}
	contentHeight := height - footerLines
	if contentHeight < 3 {
		contentHeight = 3
	}

	// Pin children table at the top when in split preview mode.
	pinnedHeader := ""
	pinnedHeaderLines := 0
	if m.hasSplitPreview() {
		childrenHeight := (contentHeight - 2) / 3
		if childrenHeight < 2 {
			childrenHeight = 2
		}
		childLabel := strings.ToUpper(m.ownedChildKindLabel())
		pinnedHeader = m.withSessionColumnsForKind(m.rightColumnKind(), func() string {
			return ui.RenderTable(childLabel, m.rightItems, -1, width, childrenHeight, m.loading, m.spinner.View(), "", false)
		})
		pinnedHeader += "\n" + ui.DimStyle.Render(strings.Repeat("\u2500", width))
		pinnedHeaderLines = strings.Count(pinnedHeader, "\n") + 1
		contentHeight -= pinnedHeaderLines
		if contentHeight < 3 {
			contentHeight = 3
		}
	}

	// Render the scrollable content (details only when split preview, full content otherwise).
	renderHeight := contentHeight + m.previewScroll
	var result string
	if m.hasSplitPreview() {
		result = m.renderDetailsOnly(width, renderHeight)
	} else {
		result = m.renderRightColumnContent(width, renderHeight)
	}

	// Append events to scrollable content (events scroll with the details).
	if !m.fullYAMLPreview && m.previewEventsContent != "" {
		result += "\n" + ui.DimStyle.Render(strings.Repeat("\u2500", width)) + "\n" + m.previewEventsContent
	}

	// Apply preview scroll to the scrollable content only.
	lines := strings.Split(result, "\n")
	if m.previewScroll > 0 {
		if m.previewScroll >= len(lines) {
			m.previewScroll = len(lines) - 1
		}
		if m.previewScroll > 0 {
			lines = lines[m.previewScroll:]
		}
	}
	// Truncate to contentHeight so footer always has room.
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}
	// Pad to contentHeight so footer is pinned to the bottom of the pane.
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	result = strings.Join(lines, "\n")

	// Assemble: pinned header (children) + scrollable content + pinned footer (metrics).
	if pinnedHeader != "" {
		result = pinnedHeader + "\n" + result
	}
	if footer != "" {
		result += "\n" + footer
	}

	return result
}

// hasSplitPreview returns true when the right column shows children + details (split view).
func (m Model) hasSplitPreview() bool {
	if m.fullYAMLPreview || m.mapView {
		return false
	}
	if m.nav.Level == model.LevelResources && (m.resourceTypeHasChildren() || m.nav.ResourceType.Kind == "Pod") && len(m.rightItems) > 0 {
		return true
	}
	if m.nav.Level == model.LevelOwned {
		sel := m.selectedMiddleItem()
		if sel != nil && sel.Kind == "Pod" && len(m.rightItems) > 0 {
			return true
		}
	}
	return false
}

// renderDetailsOnly renders the details portion (without children table) for the right column.
// The returned string contains a "DETAILS" header line followed by the summary
// body, and fits within the requested height (body capped at height-1 to
// reserve one line for the header).
func (m Model) renderDetailsOnly(width, height int) string {
	sel := m.selectedMiddleItem()
	detailsHeader := ui.DimStyle.Bold(true).Render("DETAILS")
	bodyHeight := height - 1
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	var bottomContent string
	if sel != nil && len(sel.Columns) > 0 {
		bottomContent = ui.RenderResourceSummary(sel, "", width, bodyHeight)
	} else {
		yaml := m.previewYAML
		if yaml == "" {
			yaml = m.yamlContent
		}
		if yaml != "" {
			bottomContent = ui.RenderYAMLContent(yaml, width, bodyHeight)
		} else {
			bottomContent = ui.DimStyle.Render("No details available")
		}
	}
	return detailsHeader + "\n" + bottomContent
}

func (m Model) renderRightColumnContent(width, height int) string {
	// Resource map view: show relationship tree.
	if m.mapView && m.nav.Level >= model.LevelResources {
		if m.resourceTree == nil {
			return ui.DimStyle.Render("Loading resource tree...")
		}
		return ui.RenderResourceTree(m.resourceTree, width, height)
	}

	// Full YAML preview mode (Shift+P): show only YAML, no children.
	if m.fullYAMLPreview && m.nav.Level >= model.LevelResources && m.nav.Level != model.LevelContainers {
		return m.renderFullYAMLPreview(width, height)
	}

	switch m.nav.Level {
	case model.LevelResourceTypes:
		return m.renderRightResourceTypes(width, height)
	case model.LevelClusters:
		return m.renderRightClusters(width, height)
	case model.LevelResources:
		return m.renderRightResources(width, height)
	case model.LevelOwned:
		return m.renderRightOwned(width, height)
	case model.LevelContainers:
		if sel := m.selectedMiddleItem(); sel != nil {
			return ui.RenderContainerDetail(sel, width, height)
		}
	}

	return m.renderRightDefault(width, height)
}

func (m Model) renderFullYAMLPreview(width, height int) string {
	yaml := m.previewYAML
	if yaml == "" {
		yaml = m.yamlContent
	}
	if yaml == "" {
		return ui.DimStyle.Render("Loading YAML...")
	}
	return ui.RenderYAMLContent(yaml, width, height)
}

func (m Model) renderRightResourceTypes(width, height int) string {
	sel := m.selectedMiddleItem()
	if sel != nil && sel.Extra == "__overview__" {
		if m.dashboardPreview == "" {
			return ui.DimStyle.Render(m.spinner.View() + " Loading cluster dashboard...")
		}
		return m.dashboardPreview
	}
	if sel != nil && sel.Extra == "__monitoring__" {
		if m.monitoringPreview == "" {
			return ui.DimStyle.Render(m.spinner.View() + " Loading monitoring dashboard...")
		}
		return m.monitoringPreview
	}
	// Security sources: show spinner while findings load instead of
	// "No resources found".
	if sel != nil && strings.HasPrefix(sel.Kind, "__security_") && sel.Kind != "__security_finding__" {
		if len(m.rightItems) == 0 {
			return ui.DimStyle.Render(m.spinner.View() + " Scanning " + sel.Name + " findings...")
		}
		return m.renderRightDefault(width, height)
	}
	return m.renderRightDefault(width, height)
}

func (m Model) renderRightClusters(width, height int) string {
	// Discovery for the hovered context is orthogonal to m.loading — it
	// runs in its own background task. While it is in flight we keep
	// rightItems empty (see loadPreviewClusters) so the user sees a plain
	// loader instead of a seeded placeholder list.
	discovering := false
	if sel := m.selectedMiddleItem(); sel != nil {
		discovering = m.discoveringContexts[sel.Name]
	}
	if len(m.rightItems) == 0 {
		if m.loading || discovering {
			return ui.DimStyle.Render(m.spinner.View() + " Loading...")
		}
		return ui.DimStyle.Render("No resource types found")
	}
	return ui.RenderColumn("RESOURCE TYPE", m.rightItems, -1, width, height, false, m.loading, m.spinner.View(), "")
}

func (m Model) renderRightResources(width, height int) string {
	sel := m.selectedMiddleItem()
	if sel != nil && sel.Kind == "__security_finding__" {
		return ui.RenderFindingDetails(*sel, width, height)
	}
	// Security finding groups: show affected resources as a table (top)
	// with group details (bottom), same layout as Deployment > Pods.
	if sel != nil && sel.Kind == "__security_finding_group__" {
		if len(m.rightItems) > 0 {
			return m.renderSecurityGroupSplitPreview(sel, width, height)
		}
		if m.previewLoading {
			return ui.DimStyle.Render(m.spinner.View() + " Loading affected resources...")
		}
		return ui.RenderFindingGroupDetails(*sel, nil, width, height)
	}
	if (m.resourceTypeHasChildren() || m.nav.ResourceType.Kind == "Pod") && len(m.rightItems) > 0 {
		return m.renderSplitPreview(width, height)
	}
	// No children to render (either the kind has no children, or a child-ful
	// kind happens to have zero matching items like a Service with no pods
	// behind its selector). Prefer rendering the selected item's details so
	// the right pane stays stable — watch-tick refreshes set previewLoading
	// on every interval, and branching on it here would flash a spinner and
	// clear the details every tick.
	if m.nav.ResourceType.Kind != "Pod" {
		if sel != nil && len(sel.Columns) > 0 {
			return ui.RenderResourceSummary(sel, "", width, height)
		}
		// No item selected yet (e.g., initial load before the list arrives):
		// show the spinner instead of "No preview".
		if m.loading || m.previewLoading {
			return ui.DimStyle.Render(m.spinner.View() + " Loading...")
		}
		return m.renderFallbackYAML(width, height)
	}
	return m.renderRightDefault(width, height)
}

func (m Model) renderRightOwned(width, height int) string {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m.renderRightDefault(width, height)
	}
	// Security affected resources: show finding details for this resource.
	if sel.Kind == "__security_affected_resource__" {
		return ui.RenderAffectedResourceDetails(*sel, width, height)
	}
	if sel.Kind == "Pod" && len(m.rightItems) > 0 {
		return m.renderSplitPreview(width, height)
	}
	if sel.Kind != "Pod" {
		if len(sel.Columns) > 0 {
			return ui.RenderResourceSummary(sel, "", width, height)
		}
		return m.renderFallbackYAML(width, height)
	}
	return m.renderRightDefault(width, height)
}

func (m Model) renderRightDefault(width, height int) string {
	if len(m.rightItems) == 0 {
		if m.loading || m.previewLoading {
			return ui.DimStyle.Render(m.spinner.View() + " Loading...")
		}
		return ui.DimStyle.Render("No resources found")
	}
	return m.withSessionColumnsForKind(m.rightColumnKind(), func() string {
		return ui.RenderTable(strings.ToUpper(m.ownedChildKindLabel()), m.rightItems, -1, width, height, m.loading, m.spinner.View(), "", false)
	})
}

// renderSecurityGroupSplitPreview renders a split view for a security finding
// group: affected resources table on top, group details on the bottom — the
// same layout as Deployment > Pods.
func (m Model) renderSecurityGroupSplitPreview(sel *model.Item, width, height int) string {
	childrenHeight := (height - 2) / 3
	if childrenHeight < 2 {
		childrenHeight = 2
	}
	detailsHeight := height - childrenHeight - 2
	if detailsHeight < 1 {
		detailsHeight = 1
	}

	childrenContent := ui.RenderTable("AFFECTED RESOURCES", m.rightItems, -1, width, childrenHeight, m.loading, m.spinner.View(), "", false)
	separator := ui.DimStyle.Render(strings.Repeat("\u2500", width))
	detailsContent := ui.RenderFindingGroupDetails(*sel, nil, width, detailsHeight)

	return childrenContent + "\n" + separator + "\n" + detailsContent
}

// renderSplitPreview renders the right column as a split: top children table, bottom details.
func (m Model) renderSplitPreview(width, height int) string {
	childrenHeight := (height - 2) / 3 // -2 for separator + details header
	if childrenHeight < 2 {
		childrenHeight = 2 // at least header + 1 row
	}
	detailsHeight := height - childrenHeight - 2 // separator + details header
	if detailsHeight < 1 {
		detailsHeight = 1
	}

	// Render children as table (same format as middle column). Scope the
	// session column config to the child kind so pod/container configs
	// stay independent even when the middle column shows pods.
	childLabel := strings.ToUpper(m.ownedChildKindLabel())
	childrenContent := m.withSessionColumnsForKind(m.rightColumnKind(), func() string {
		return ui.RenderTable(childLabel, m.rightItems, -1, width, childrenHeight, m.loading, m.spinner.View(), "", false)
	})

	// Separator line.
	separator := ui.DimStyle.Render(strings.Repeat("\u2500", width))

	// Render details summary in bottom portion.
	sel := m.selectedMiddleItem()
	detailsHeader := ui.DimStyle.Bold(true).Render("DETAILS")
	var bottomContent string
	if sel != nil && len(sel.Columns) > 0 {
		bottomContent = ui.RenderResourceSummary(sel, "", width, detailsHeight)
	} else {
		// Fall back to YAML if no detail columns are available.
		yaml := m.previewYAML
		if yaml == "" {
			yaml = m.yamlContent
		}
		if yaml != "" {
			bottomContent = ui.RenderYAMLContent(yaml, width, detailsHeight)
		} else {
			bottomContent = ui.DimStyle.Render("No details available")
		}
	}

	return childrenContent + "\n" + separator + "\n" + detailsHeader + "\n" + bottomContent
}

// renderFallbackYAML renders YAML content when no detail columns are available for a resource.
func (m Model) renderFallbackYAML(width, height int) string {
	yaml := m.previewYAML
	if yaml == "" {
		yaml = m.yamlContent
	}
	if yaml != "" {
		return ui.RenderYAMLContent(yaml, width, height)
	}
	return ui.DimStyle.Render("No preview")
}

func (m Model) resourceTypeHasChildren() bool {
	return kindHasOwnedChildren(m.nav.ResourceType.Kind)
}

// kindHasOwnedChildren reports whether a given Kubernetes resource kind can
// have child/owned resources that GetOwnedResources knows how to fetch.
// This is used both at LevelResources (to decide whether right-arrow navigates
// into owned view) and at LevelOwned (to allow nested drill-down, e.g.,
// ArgoCD Application -> Deployment -> Pods).
func kindHasOwnedChildren(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob",
		"Service", "Application", "HelmRelease", "Kustomization", "Node":
		return true
	default:
		return false
	}
}

// ownedItemKindLabel returns the label for the items shown in the middle column at LevelOwned.
// This reflects what the owned items *are* (e.g., Pods owned by a Deployment).
func (m Model) ownedItemKindLabel() string {
	switch m.nav.ResourceType.Kind {
	case "CronJob":
		return "Job"
	case "Application", "HelmRelease":
		return "Resource"
	case "Pod":
		return "Container"
	case "Node":
		return "Pod"
	default:
		return "Pod"
	}
}

// ownedChildKindLabel returns the label for the children of the selected owned item,
// shown in the right column (e.g., Containers within a selected Pod).
func (m Model) ownedChildKindLabel() string {
	// At the owned level, if the selected item is a Pod, right column shows containers.
	if m.nav.Level == model.LevelOwned {
		sel := m.selectedMiddleItem()
		if sel != nil && sel.Kind == "Pod" {
			return "Container"
		}
	}
	switch m.nav.ResourceType.Kind {
	case "CronJob":
		return "Job"
	case "Application", "HelmRelease":
		return "Resource"
	case "Pod":
		return "Container"
	case "Node":
		return "Pod"
	default:
		return "NAME"
	}
}
