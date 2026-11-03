package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) renderRightResourceTypes(width, height int) string {
	sel := m.selectedMiddleItem()
	if sel != nil && sel.Extra == "__overview__" {
		if m.isUnionSentinel() {
			return m.renderUnionDashboardMembers(width, height)
		}
		if m.dashboardPreview == "" {
			return ui.DimStyle.Render(m.spinner.View() + " Loading cluster dashboard...")
		}
		return m.dashboardPreview
	}
	if sel != nil && sel.Extra == "__monitoring__" {
		if m.isUnionSentinel() {
			return m.renderUnionDashboardMembers(width, height)
		}
		if m.monitoringPreview == "" {
			return ui.DimStyle.Render(m.spinner.View() + " Loading monitoring dashboard...")
		}
		return m.monitoringPreview
	}
	// Security sources: show a scanning spinner while the findings
	// preview load is actually in flight. Once it completes, an empty
	// rightItems means "this source returned zero findings" or "the
	// fetch errored out", and the spinner would loop forever — so fall
	// through to renderRightDefault (which says "No resources found")
	// in that case.
	if sel != nil && strings.HasPrefix(sel.Kind, "__security_") && sel.Kind != "__security_finding__" {
		if len(m.rightItems) == 0 && m.previewLoading {
			return ui.DimStyle.Render(m.spinner.View() + " Scanning " + sel.Name + " findings...")
		}
		return m.renderRightDefault(width, height)
	}
	return m.renderRightDefault(width, height)
}

func (m Model) renderUnionDashboardMembers(width, height int) string {
	if len(m.rightItems) == 0 {
		if m.loading || m.previewLoading {
			return ui.DimStyle.Render(m.spinner.View() + " Loading contexts...")
		}
		return ui.DimStyle.Render("No union contexts found")
	}
	return ui.RenderColumn("CONTEXT", m.rightItems, -1, width, height, false, m.loading, m.spinner.View(), "")
}

func (m Model) renderRightClusters(width, height int) string {
	// Discovery for the hovered context is orthogonal to m.loading — it
	// runs in its own background task. While it is in flight we keep
	// rightItems empty (see loadPreviewClusters) so the user sees a plain
	// loader instead of a seeded placeholder list.
	discovering := false
	header := "RESOURCE TYPE"
	if sel := m.selectedMiddleItem(); sel != nil {
		if isUnionSetItem(sel) {
			header = "CONTEXT"
		} else {
			discovering = m.discoveringContexts[sel.Name]
		}
	}
	if len(m.rightItems) == 0 {
		if m.loading || discovering {
			return ui.DimStyle.Render(m.spinner.View() + " Loading...")
		}
		return ui.DimStyle.Render("No resource types found")
	}
	return ui.RenderColumn(header, m.rightItems, -1, width, height, false, m.loading, m.spinner.View(), "")
}

func (m Model) renderRightResources(width, height int) string {
	if isUnionDashboardResourceKind(m.nav.ResourceType.Kind) {
		return m.renderUnionDashboardMemberPreview()
	}
	sel := m.selectedMiddleItem()
	if sel != nil && sel.Kind == "__security_finding__" {
		return ui.RenderFindingDetails(*sel, width, height)
	}
	// Security finding groups with affected rows render via the split-preview
	// machinery in renderRightColumn (pinned affected table + scrolling
	// details, the same shape as Deployment > Pods), so this path is only
	// reached before the affected resources have loaded.
	if sel != nil && sel.Kind == "__security_finding_group__" {
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
		if sel := m.selectedMiddleItem(); sel != nil && (sel.Name != "" || m.secretDataCachedFor(sel)) {
			// Loaded YAML beats the minimal identity-summary fallback.
			if len(sel.Columns) == 0 && !m.secretDataCachedFor(sel) &&
				(m.previewYAML != "" || m.yamlView.content != "") {
				return m.renderFallbackYAML(width, height)
			}
			return ui.RenderResourceSummary(sel, "", width, height)
		}
		// No item selected yet (e.g., initial load before the list arrives):
		// show the spinner instead of "No preview".
		if m.loading || m.previewLoading {
			return ui.DimStyle.Render(m.spinner.View() + " Loading...")
		}
		if m.err != nil {
			return ui.DimStyle.Render("Unable to load resources")
		}
		return m.renderFallbackYAML(width, height)
	}
	return m.renderPreviewWithoutChildren(sel, width, height)
}

// renderPreviewWithoutChildren renders the right pane for a selected item whose
// children have not arrived. previewLoading alone cannot answer whether they
// ever will: the silent watch-tick refresh of the list clears it every interval
// while the hovered item's child preview is still on the wire, so the pane read
// an empty rightItems as "this pod has no containers" and flashed "No resources
// found" once a second. The item's own details are the stable answer, and this
// is what the non-Pod branch above has always done.
func (m Model) renderPreviewWithoutChildren(sel *model.Item, width, height int) string {
	if sel != nil && len(sel.Columns) > 0 {
		return ui.RenderResourceSummary(sel, "", width, height)
	}
	return m.renderRightDefault(width, height)
}

func (m Model) renderUnionDashboardMemberPreview() string {
	mode, _ := unionDashboardModeFromKind(m.nav.ResourceType.Kind)
	if mode == unionDashboardMonitoring {
		if m.monitoringPreview == "" {
			return ui.DimStyle.Render(m.spinner.View() + " Loading monitoring dashboard...")
		}
		return m.monitoringPreview
	}
	if m.dashboardPreview == "" {
		return ui.DimStyle.Render(m.spinner.View() + " Loading cluster dashboard...")
	}
	return m.dashboardPreview
}

func (m Model) renderRightOwned(width, height int) string {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m.renderRightDefault(width, height)
	}
	// Security affected resources: render finding context for this resource.
	if sel.Kind == "__security_affected_resource__" {
		return ui.RenderAffectedResourceDetails(*sel, width, height)
	}
	if sel.Kind == "Pod" && len(m.rightItems) > 0 {
		return m.renderSplitPreview(width, height)
	}
	if sel.Kind != "Pod" {
		// Same precedence as renderRightResources: YAML first, then summary.
		if len(sel.Columns) == 0 && (m.previewYAML != "" || m.yamlView.content != "") {
			return m.renderFallbackYAML(width, height)
		}
		return ui.RenderResourceSummary(sel, "", width, height)
	}
	return m.renderPreviewWithoutChildren(sel, width, height)
}

func (m Model) renderRightDefault(width, height int) string {
	if len(m.rightItems) == 0 {
		if m.loading || m.previewLoading {
			return ui.DimStyle.Render(m.spinner.View() + " Loading...")
		}
		if m.err != nil {
			return ui.DimStyle.Render("Unable to load resources")
		}
		return ui.DimStyle.Render("No resources found")
	}
	return m.withSessionColumnsForKind(m.rightColumnKind(), func() string {
		return ui.RenderTable(strings.ToUpper(m.ownedChildKindLabel()), m.rightItems, -1, width, height, m.loading, m.spinner.View(), "", false)
	})
}

// renderSplitPreview renders the right column as a split: top children table, bottom details.
func (m Model) renderSplitPreview(width, height int) string {
	childrenHeight := max(
		// -2 for separator + details header
		(height-2)/3,
		// at least header + 1 row
		2)
	detailsHeight := max(
		// separator + details header
		height-childrenHeight-2, 1)

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
			yaml = m.yamlView.content
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
		yaml = m.yamlView.content
	}
	if yaml != "" {
		return ui.RenderYAMLContent(yaml, width, height)
	}
	return ui.DimStyle.Render("No preview")
}
