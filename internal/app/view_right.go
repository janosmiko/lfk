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
// Only details+events scroll. Pinned header (children) and footer (resource usage) are excluded.
func (m *Model) clampPreviewScroll() {
	// This runs in the Update path (preview wheel/keys) and renders right-pane
	// content below (the split pinned header and the measurement render), all
	// with cursor=-1. Park the persistent pane-scroll globals around those
	// renders: RenderTable/RenderColumn would otherwise reset ActiveMiddleScroll
	// / ActiveLeftScroll to 0 (VimScrollOff with cursor=-1), making the pane to
	// the left visibly jump (issue #398). ActiveMiddleLineMap is saved too:
	// RenderTable rebuilds it (from the right pane's items, here) under the
	// same >= 0 gate. The View path applies this guard around its right-pane
	// render in viewExplorerThreeCol.
	savedMiddleScroll, savedLeftScroll := ui.ActiveMiddleScroll, ui.ActiveLeftScroll
	savedLineMap := ui.ActiveMiddleLineMap
	ui.ActiveMiddleScroll, ui.ActiveLeftScroll = -1, -1
	defer func() {
		ui.ActiveMiddleScroll, ui.ActiveLeftScroll = savedMiddleScroll, savedLeftScroll
		ui.ActiveMiddleLineMap = savedLineMap
	}()

	// The fullscreen dashboard reuses previewScroll but renders entirely
	// different content (cluster overview / monitoring), so bound it against
	// that content instead of the right-column preview.
	if m.fullscreenDashboard {
		m.clampDashboardScroll()
		return
	}
	// Compute the right column width exactly as the View function does.
	_, _, rightW := m.explorerColumnWidths()
	innerW := rightW - 2

	// Compute the column height exactly as the View function does.
	colHeight := max(
		// title + status bar + column borders
		m.height-4, 3)
	if len(m.tabs) > 1 {
		colHeight-- // tab bar
	}

	// The live-log preview occupies the full right pane (early-return in
	// renderRightColumn). Clamp its dedicated fromBottom offset against the
	// physical line count so the view cannot scroll past the oldest line.
	// Uses the same PreviewLogPhysicalCount helper as RenderLogPreview so
	// clamp and render always agree on the total line count.
	if m.fullLogPreview {
		bodyHeight := max(colHeight-1, 1)
		total := ui.PreviewLogPhysicalCount(m.previewLog.lines, innerW)
		maxFromBottom := max(total-bodyHeight, 0)
		if m.previewLog.fromBottom > maxFromBottom {
			m.previewLog.fromBottom = maxFromBottom
		}
		if m.previewLog.fromBottom < 0 {
			m.previewLog.fromBottom = 0
		}
		return
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
	// selName invalidates the memo when the middle selection changes while
	// every layout dimension stays equal — e.g. two security finding groups
	// with the same affected count but different description lengths.
	selName string
}

// measureScrollableLines returns the line count of the scrollable right-pane
// content, memoized on previewMeasureKey. The expensive measurement renders the
// content once at a height tall enough to hold all of it (so a long list or YAML
// document can scroll to the end — see the issue where the right pane froze
// ~halfway down a few-hundred-item list because the height was capped). While
// the user scrolls, the key is unchanged so this is O(1). It recomputes only
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
	if sel := m.selectedMiddleItem(); sel != nil {
		key.selName = sel.Name
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
	// rollup. Real resource types carry their Kubernetes Kind.
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
	// Live-log preview mode: the right pane is fully occupied by the log tail.
	// Return early before footer and split-preview processing, mirroring the
	// fullYAMLPreview early-return that lives inside renderRightColumnContent.
	if m.fullLogPreview {
		return ui.RenderLogPreview(m.previewLog.lines, m.previewLog.err, width, height, m.previewLogPodLabel(), m.previewLog.fromBottom)
	}

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
	// Lists have no events. Their scroll is already applied above.
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
// windowed render in renderRightColumn is gated on it. Every other case
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
	if m.fullYAMLPreview || m.mapView || m.fullLogPreview {
		return false
	}
	if m.nav.Level == model.LevelResources && (m.resourceTypeHasChildren() || m.nav.ResourceType.Kind == "Pod") && len(m.rightItems) > 0 {
		return true
	}
	// Security finding groups split the same way: affected resources pinned
	// on top, group details scrolling below — so previewScroll never pushes
	// the table out of view.
	if m.isSecurityGroupSplit() {
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

// isSecurityGroupSplit reports whether the right pane previews a security
// finding group with its affected resources loaded.
func (m Model) isSecurityGroupSplit() bool {
	if m.nav.Level != model.LevelResources || len(m.rightItems) == 0 {
		return false
	}
	sel := m.selectedMiddleItem()
	return sel != nil && sel.Kind == "__security_finding_group__"
}

// renderDetailsOnly renders the details portion (without children table) for the right column.
// The returned string contains a "DETAILS" header line followed by the summary
// body, and fits within the requested height (body capped at height-1 to
// reserve one line for the header).
func (m Model) renderDetailsOnly(width, height int) string {
	sel := m.selectedMiddleItem()
	// Security finding groups carry their own title line. No DETAILS header.
	if sel != nil && sel.Kind == "__security_finding_group__" {
		return ui.RenderFindingGroupDetails(*sel, nil, width, height)
	}
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
// Security finding groups list affected resources of arbitrary kinds at this
// level, so they get their own label instead of the Pod fallback.
func (m Model) ownedItemKindLabel() string {
	if strings.HasPrefix(m.nav.ResourceType.Kind, "__security_") {
		return "Affected Resource"
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
		return "Pod"
	}
}

// ownedChildKindLabel returns the label for the children of the selected owned item,
// shown in the right column (e.g., Containers within a selected Pod).
func (m Model) ownedChildKindLabel() string {
	// Security finding groups pin their affected-resources table on top.
	if m.isSecurityGroupSplit() {
		return "Affected Resources"
	}
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
