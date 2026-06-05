package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// clearRight resets the right column and YAML preview so stale data doesn't linger.
// Every caller of clearRight is a navigation transition that will dispatch a
// new preview load, so we arm previewLoading here to keep the right pane's
// spinner visible during the gap. Without this, navigateParent/navigateChild
// and other transitions briefly render "No resources found".
func (m *Model) clearRight() {
	m.rightItems = nil
	m.yamlView.content = ""
	m.yamlView.sections = nil
	m.previewYAML = ""
	m.previewScroll = 0
	m.previewMeasureLines = 0 // force the scroll measurement to recompute for the new preview
	m.metricsContent = ""
	m.previewEventsContent = ""
	m.metricsData = nil
	m.metricsLoading = false
	m.previewEventsData = nil
	m.resourceTree = nil
	m.mapView = false
	m.previewLoading = true
}

// clampPreviewScroll prevents scrolling past the preview content.
// Only details+events scroll; pinned header (children) and footer (resource usage) are excluded.
func (m *Model) clampPreviewScroll() {
	// The fullscreen dashboard reuses previewScroll but renders entirely
	// different content (cluster overview / monitoring), so bound it against
	// that content instead of the right-column preview.
	if m.fullscreenDashboard {
		m.clampDashboardScroll()
		return
	}
	// Compute the right column width exactly as the View function does.
	usable := m.width - 6
	rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
	innerW := rightW - 2

	// Compute the column height exactly as the View function does.
	colHeight := max(
		// title + status bar + column borders
		m.height-4, 3)
	if len(m.tabs) > 1 {
		colHeight-- // tab bar
	}

	// Compute footer lines (must match renderRightColumn): the metrics rollup
	// and/or the list summary band are pinned to the bottom and excluded from
	// the scrollable area.
	var footerParts []string
	if !m.fullYAMLPreview && m.metricsContent != "" {
		footerParts = append(footerParts, ui.DimStyle.Render(strings.Repeat("\u2500", innerW)), m.metricsContent)
	}
	if band := m.previewSummaryBand(innerW); band != "" {
		footerParts = append(footerParts, ui.DimStyle.Render(strings.Repeat("\u2500", innerW)), band)
	}
	footerLines := 0
	if len(footerParts) > 0 {
		footerLines = strings.Count(strings.Join(footerParts, "\n"), "\n") + 1
	}

	// Compute scrollable viewport height (must match renderRightColumn).
	scrollableH := max(colHeight-footerLines, 3)

	if m.hasSplitPreview() {
		childrenHeight := max((scrollableH-2)/3, 2)
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

	totalLines := m.measureScrollableLines(innerW, scrollableH)
	maxScroll := max(totalLines-scrollableH, 0)
	if m.previewScroll > maxScroll {
		m.previewScroll = maxScroll
	}
}

// previewMeasureKey fingerprints the inputs that determine the scrollable
// right-pane line count, so measureScrollableLines can memoize the result and
// avoid re-rendering a large list on every scroll keystroke.
type previewMeasureKey struct {
	innerW      int
	scrollableH int
	rightLen    int
	level       model.Level
	split       bool
	mapView     bool
	fullYAML    bool
	yamlLen     int
	dashLen     int
	eventsLen   int
}

// measureScrollableLines returns the line count of the scrollable right-pane
// content, memoized on previewMeasureKey. The expensive measurement renders the
// content once at a height tall enough to hold all of it (so a long list or YAML
// document can scroll to the end — see the issue where the right pane froze
// ~halfway down a few-hundred-item list because the height was capped). While
// the user scrolls, the key is unchanged so this is O(1); it recomputes only
// when the content or layout changes.
func (m *Model) measureScrollableLines(innerW, scrollableH int) int {
	yaml := m.previewYAML
	if yaml == "" {
		yaml = m.yamlView.content
	}
	key := previewMeasureKey{
		innerW:      innerW,
		scrollableH: scrollableH,
		rightLen:    len(m.rightItems),
		level:       m.nav.Level,
		split:       m.hasSplitPreview(),
		mapView:     m.mapView,
		fullYAML:    m.fullYAMLPreview,
		yamlLen:     len(yaml),
		dashLen:     len(m.dashboardPreview) + len(m.monitoringPreview),
		eventsLen:   len(m.previewEventsContent),
	}
	if key == m.previewMeasureKey && m.previewMeasureLines > 0 {
		return m.previewMeasureLines
	}

	// Measure at a height large enough to hold any realistic content so the true
	// line count is captured and the pane can scroll to the very end — long
	// resource lists AND long text details (e.g. a ConfigMap with a multi-line
	// data value). Every right-pane renderer only truncates at its height
	// argument, never pads up to it (RenderTable / RenderYAMLContent /
	// RenderResourceSummary / RenderResourceTree / detail renderers), so an
	// oversized measure height never invents phantom lines. This render is
	// memoized on previewMeasureKey, so the O(content) cost is paid once per
	// content change, not per scroll keystroke.
	const measureH = 1 << 20

	var totalLines int
	if key.split {
		totalLines = strings.Count(m.renderDetailsOnly(innerW, measureH), "\n") + 1
	} else {
		totalLines = strings.Count(m.renderRightColumnContent(innerW, measureH), "\n") + 1
	}
	// Events scroll with the details content (excluded in full-YAML mode).
	if !m.fullYAMLPreview && m.previewEventsContent != "" {
		totalLines += 1 + strings.Count(m.previewEventsContent, "\n") + 1 // separator + event lines
	}

	m.previewMeasureKey = key
	m.previewMeasureLines = totalLines
	return totalLines
}

// previewSummaryBand renders the aggregate status band pinned at the bottom of
// the children pane while hovering a resource type in the resource-type list — a
// rollup of that type's resources (e.g. ArgoCD Application health / sync) so the
// list can be assessed without drilling in. It is derived synchronously from the
// already-loaded preview items, so it costs no API calls and cannot block the
// UI. Returns "" unless we are at the resource-type list previewing a real,
// summarisable kind.
func (m Model) previewSummaryBand(width int) string {
	if m.nav.Level != model.LevelResourceTypes {
		return ""
	}
	sel := m.selectedMiddleItem()
	// Synthetic rows (dashboards, security sources, port-forwards, captures,
	// collapsed groups) all carry a "__"-prefixed Kind and have no status
	// rollup; real resource types carry their Kubernetes Kind.
	if sel == nil || strings.HasPrefix(sel.Kind, "__") {
		return ""
	}
	items := m.rightItems
	if len(items) == 0 {
		return ""
	}
	summary := ui.BuildListSummary(sel.Kind, items)
	label := sel.Name
	if label == "" {
		label = sel.Kind
	}
	return ui.RenderListSummary(summary, label, width)
}

func (m Model) renderRightColumn(width, height int) string {
	// Build the pinned footer (bottom of the pane): resource usage (metrics)
	// and the list summary band, each preceded by a separator. They are
	// mutually exclusive in practice (metrics at the resource levels, the band
	// at the resource-type list), but both are handled uniformly here.
	var footerParts []string
	if !m.fullYAMLPreview && m.metricsContent != "" {
		footerParts = append(footerParts,
			ui.DimStyle.Render(strings.Repeat("\u2500", width)),
			m.metricsContent)
	}
	if band := m.previewSummaryBand(width); band != "" {
		footerParts = append(footerParts,
			ui.DimStyle.Render(strings.Repeat("─", width)),
			band)
	}

	// Reserve height for the pinned footer.
	footerLines := 0
	footer := ""
	if len(footerParts) > 0 {
		footer = strings.Join(footerParts, "\n")
		footerLines = strings.Count(footer, "\n") + 1
	}
	contentHeight := max(height-footerLines, 3)

	// Pin children table at the top when in split preview mode. This RenderTable
	// is cursor-less but runs before ActiveRightScroll is set below, and split
	// preview (LevelResources/Owned) is mutually exclusive with the windowed list
	// preview (LevelResourceTypes), so it never picks up the windowing.
	pinnedHeader := ""
	pinnedHeaderLines := 0
	if m.hasSplitPreview() {
		childrenHeight := max((contentHeight-2)/3, 2)
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

	// Render the scrollable content. For the resource-type list preview the
	// content is a plain item table that can be very long, so render it windowed
	// (only the visible rows) via ActiveRightScroll \u2014 each frame is O(viewport)
	// instead of O(scroll position). The output is kept byte-identical to the
	// generic render-then-slice path so windowing is a pure performance change:
	// previewScroll is a display-line offset where line 0 is the table header and
	// line i (i>=1) is rightItems[i-1], so a scrolled view drops the header and
	// starts at item previewScroll-1. We render that window (one extra row to
	// replace the dropped header) and strip the header line. Other content
	// (details, YAML, tree) is small and uses the generic path below.
	listPreview := m.isRightListPreview()
	var result string
	switch {
	case listPreview && m.previewScroll == 0:
		prev := ui.ActiveRightScroll
		ui.ActiveRightScroll = 0
		result = m.renderRightColumnContent(width, contentHeight)
		ui.ActiveRightScroll = prev
	case listPreview:
		prev := ui.ActiveRightScroll
		ui.ActiveRightScroll = m.previewScroll - 1 // line previewScroll == item previewScroll-1
		result = m.renderRightColumnContent(width, contentHeight+1)
		ui.ActiveRightScroll = prev
		if i := strings.IndexByte(result, '\n'); i >= 0 { // drop the header line
			result = result[i+1:]
		} else {
			result = ""
		}
	case m.hasSplitPreview():
		result = m.renderDetailsOnly(width, contentHeight+m.previewScroll)
	default:
		result = m.renderRightColumnContent(width, contentHeight+m.previewScroll)
	}

	// Append events to scrollable content (events scroll with the details).
	// Lists have no events; their scroll is already applied above.
	if !listPreview && !m.fullYAMLPreview && m.previewEventsContent != "" {
		result += "\n" + ui.DimStyle.Render(strings.Repeat("\u2500", width)) + "\n" + m.previewEventsContent
	}

	// Apply preview scroll to the scrollable content. The windowed list path is
	// already positioned at previewScroll, so only the generic path slices here.
	lines := strings.Split(result, "\n")
	if !listPreview && m.previewScroll > 0 {
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

	// Assemble: pinned header (children) + scrollable content + pinned footer
	// (metrics and/or the list summary band).
	if pinnedHeader != "" {
		result = pinnedHeader + "\n" + result
	}
	if footer != "" {
		result += "\n" + footer
	}

	return result
}

// isRightListPreview reports whether the right pane's scrollable content is the
// plain resource list shown while hovering a resource type (renderRightDefault's
// RenderTable over rightItems). Only this path honours ActiveRightScroll, so the
// windowed render in renderRightColumn is gated on it; every other case
// (dashboards, security sources, union members rendered with RenderColumn,
// details/YAML/tree) falls back to the generic render-then-slice path.
func (m Model) isRightListPreview() bool {
	if m.nav.Level != model.LevelResourceTypes || m.mapView || m.fullYAMLPreview {
		return false
	}
	if len(m.rightItems) == 0 || m.isUnionSentinel() {
		return false
	}
	sel := m.selectedMiddleItem()
	if sel == nil || strings.HasPrefix(sel.Kind, "__") {
		return false
	}
	if sel.Extra == "__overview__" || sel.Extra == "__monitoring__" {
		return false
	}
	// Windowing maps previewScroll (a line offset) to an item index, which only
	// holds when each item is exactly one line. Category headers/separators break
	// that 1:1 mapping, so fall back to the generic line-slice path if any item
	// carries a category (instance lists normally don't).
	for i := range m.rightItems {
		if m.rightItems[i].Category != "" {
			return false
		}
	}
	return true
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
	bodyHeight := max(height-1, 1)
	var bottomContent string
	if sel != nil && len(sel.Columns) > 0 {
		bottomContent = ui.RenderResourceSummary(sel, "", width, bodyHeight)
	} else {
		yaml := m.previewYAML
		if yaml == "" {
			yaml = m.yamlView.content
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
		yaml = m.yamlView.content
	}
	if yaml == "" {
		return ui.DimStyle.Render("Loading YAML...")
	}
	return ui.RenderYAMLContent(yaml, width, height)
}

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
	// preview load is actually in flight; once it completes, an empty
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
	// Security finding groups: split view — affected resources table on
	// top + group details on the bottom, the same shape as Deployment > Pods.
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
		if sel := m.selectedMiddleItem(); sel != nil && (len(sel.Columns) > 0 || m.secretDataCachedFor(sel)) {
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
		if m.err != nil {
			return ui.DimStyle.Render("Unable to load resources")
		}
		return ui.DimStyle.Render("No resources found")
	}
	return m.withSessionColumnsForKind(m.rightColumnKind(), func() string {
		return ui.RenderTable(strings.ToUpper(m.ownedChildKindLabel()), m.rightItems, -1, width, height, m.loading, m.spinner.View(), "", false)
	})
}

// renderSecurityGroupSplitPreview renders a split view for a security
// finding group: affected resources table on top, group details on the
// bottom — the same layout as Deployment > Pods.
func (m Model) renderSecurityGroupSplitPreview(sel *model.Item, width, height int) string {
	childrenHeight := max((height-2)/3, 2)
	detailsHeight := max(height-childrenHeight-2, 1)

	childrenContent := ui.RenderTable("AFFECTED RESOURCES", m.rightItems, -1, width, childrenHeight, m.loading, m.spinner.View(), "", false)
	separator := ui.DimStyle.Render(strings.Repeat("─", width))
	detailsContent := ui.RenderFindingGroupDetails(*sel, nil, width, detailsHeight)

	return childrenContent + "\n" + separator + "\n" + detailsContent
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

func (m Model) resourceTypeHasChildren() bool {
	return kindHasOwnedChildren(m.nav.ResourceType.Kind)
}

// kindHasOwnedChildren reports whether a given Kubernetes resource kind can
// have child/owned resources that GetOwnedResources knows how to fetch.
// This is used both at LevelResources (to decide whether right-arrow navigates
// into owned view) and at LevelOwned (to allow nested drill-down, e.g.,
// ArgoCD Application -> Deployment -> Pods).
//
// PersistentVolumeClaim is listed here even though the ownership is
// logically reversed (pods reference PVCs, not the other way around): it
// lets the right-pane preview lazily show which pods are using the
// selected PVC, and it lets right-arrow drill into that pod list. The
// alternative — populating a "Used By" column on every PVC during the
// list fetch — issued one pod-list call per PVC and could take 6+ seconds
// on large namespaces.
func kindHasOwnedChildren(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob",
		"Service", "Application", "HelmRelease", "Kustomization", "Node",
		"PersistentVolumeClaim":
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
