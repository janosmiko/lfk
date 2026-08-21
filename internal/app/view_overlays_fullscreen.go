package app

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) renderOverlayContentLater() (string, int, int, bool) {
	switch m.overlay {
	case overlayLocalClusters:
		w, h := min(100, m.width-10), min(20, m.height-6)
		state := m.buildLocalClusterOverlayState()
		state.Width, state.Height = w, h
		switch m.localClusterState.screen {
		case localClusterScreenList:
			return ui.RenderLocalClusterOverlay(state), w, h, true
		case localClusterScreenDeleteConfirm:
			return ui.RenderLocalClusterDeleteConfirm(state, m.buildLocalClusterDeleteConfirmView()), w, h, true
		}
		return ui.RenderLocalClusterWizard(state, m.buildLocalClusterWizardView()), w, h, true
	case overlayTrafficCapture:
		c, w, h := m.renderOverlayTrafficCapture()
		return c, w, h, true
	case overlayCopyFormat:
		c, w, h := m.renderOverlayCopyFormat()
		return c, w, h, true
	case overlayCopyField:
		c, w, h := m.renderOverlayCopyField()
		return c, w, h, true
	case overlayTaintEditor, overlayTaintPresets:
		c, w, h := m.renderTaintOverlay()
		return c, w, h, true
	case overlayLogTopGroupBy:
		c, w, h := m.renderLogTopGroupByOverlay()
		return c, w, h, true
	case overlayLogTopProfile:
		c, w, h := m.renderLogTopProfileOverlay()
		return c, w, h, true
	case overlayLogTopColumns:
		c, w, h := m.renderLogTopColumnsOverlay()
		return c, w, h, true
	}
	return "", 0, 0, false
}

func (m Model) renderOverlayCanISubject(background string) string {
	canIBg := m.renderCanIOverlay(background)
	w, h := min(80, m.width-10), min(20, m.height-6)
	content := renderCanISubjectOverlay(m, w-4)
	content = ui.FillLinesBg(content, w-4, ui.SurfaceBg)
	overlay := ui.BoxHeight(ui.BoxWidth(ui.OverlayStyle, w), h).Render(content)
	return ui.PlaceOverlay(m.width, m.height, overlay, canIBg)
}

func (m Model) renderOverlayFullscreen(background string) string {
	var overlay string
	switch m.overlay {
	case overlaySecretEditor:
		overlay = ui.RenderSecretEditorOverlay(
			m.secretData, m.secretCursor, m.secretRevealed, m.secretAllRevealed,
			m.secretEditing,
			m.secretEditKey.Value, m.secretEditKey.Cursor,
			m.secretEditValue.Value, m.secretEditValue.Cursor,
			m.secretEditColumn,
			m.editorSearch.query.Value, m.editorSearch.active,
			m.editorSearch.selected, m.editorSearch.formatActive, m.editorSearch.formatCursor,
			m.editorSearch.editValueScroll,
			m.width, m.height,
		)
	case overlayConfigMapEditor:
		overlay = ui.RenderConfigMapEditorOverlay(
			m.configMapData, m.configMapCursor,
			m.configMapEditing,
			m.configMapEditKey.Value, m.configMapEditKey.Cursor,
			m.configMapEditValue.Value, m.configMapEditValue.Cursor,
			m.configMapEditColumn,
			m.editorSearch.query.Value, m.editorSearch.active,
			m.editorSearch.selected, m.editorSearch.formatActive, m.editorSearch.formatCursor,
			m.editorSearch.editValueScroll,
			m.width, m.height,
		)
	case overlayRightsizing:
		overlay = ui.RenderRightsizingOverlay(
			m.rightsizing.data,
			m.rightsizing.loading,
			m.rightsizing.err,
			m.rightsizing.scroll,
			m.width, m.height,
		)
	case overlayRollback:
		overlay = renderRollbackOverlay(m)
	case overlayHelmRollback:
		overlay = renderHelmRollbackOverlay(m)
	case overlayHelmHistory:
		overlay = renderHelmHistoryOverlay(m)
	case overlayLabelEditor:
		overlay = ui.RenderLabelEditorOverlay(
			m.labelData, m.labelCursor, m.labelTab,
			m.labelEditing,
			m.labelEditKey.Value, m.labelEditKey.Cursor,
			m.labelEditValue.Value, m.labelEditValue.Cursor,
			m.labelEditColumn,
			m.editorSearch.query.Value, m.editorSearch.active,
			m.editorSearch.selected, m.editorSearch.formatActive, m.editorSearch.formatCursor,
			m.editorSearch.editValueScroll,
			m.width, m.height,
		)
	case overlayAutoSync:
		overlay = renderAutoSyncOverlay(m)
	default:
		return background
	}
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}

func (m Model) renderOverlayColumnToggle() (string, int, int) {
	filtered := m.filteredColumnToggleItems()
	entries := make([]ui.ColumnToggleEntry, len(filtered))
	for i, e := range filtered {
		entries[i] = ui.ColumnToggleEntry{Key: e.key, Visible: e.visible}
	}
	// Pass the overlay box dimensions (not the full screen) so the
	// renderer's maxVisible cap matches what fits inside the box.
	// Otherwise on a tall terminal the renderer emits ~34 lines into a
	// 20-tall box. The box visibly grew on overflow and "shrank" back
	// as the filter narrowed results — looked like the window was
	// resizing.
	overlayW := min(50, m.width-10)
	overlayH := min(20, m.height-6)
	return renderColumnToggleOverlay(m, entries, overlayW, overlayH), overlayW, overlayH
}

func (m Model) renderOverlayFinalizerSearch() (string, int, int) {
	filtered := m.filteredFinalizerResults()
	entries := make([]ui.FinalizerMatchEntry, len(filtered))
	for i, r := range filtered {
		entries[i] = ui.FinalizerMatchEntry{
			Name: r.Name, Namespace: r.Namespace, Kind: r.Kind, Matched: r.Matched, Age: r.Age,
		}
	}
	w := min(m.width-6, m.width*80/100)
	if w < 60 {
		w = min(60, m.width-4)
	}
	h := min(m.height-4, m.height*70/100)
	return ui.RenderFinalizerSearchOverlay(
		entries, m.finalizerSearchCursor, m.finalizerSearchSelected,
		m.finalizerSearchPattern, m.finalizerSearchFilter, m.finalizerSearchFilterActive,
		m.finalizerSearchLoading, w, h,
	), w, h
}

// renderCanIOverlay renders the Can-I browser overlay on top of the
// given background. In Who-Can mode the same overlay frame hosts the
// reverse-RBAC view via renderWhoCanInner — same dimensions, same wrap,
// just different content.
func (m Model) renderCanIOverlay(background string) string {
	if m.canIMode == canIModeWhoCan {
		return m.renderWhoCanOverlay(background)
	}
	visibleGroupIdxs := m.canIVisibleGroups()
	groupNames := make([]string, len(visibleGroupIdxs))
	for i, idx := range visibleGroupIdxs {
		name := m.canIGroups[idx].Name
		if name == "" {
			name = "core"
		}
		count := len(m.canIGroups[idx].Resources)
		if m.canIAllowedOnly {
			count = countAllowedResources(m.canIGroups[idx].Resources)
		}
		groupNames[i] = fmt.Sprintf("%s (%d)", name, count)
	}
	var resources []model.CanIResource
	if m.canIGroupCursor >= 0 && m.canIGroupCursor < len(visibleGroupIdxs) {
		resources = m.canIGroups[visibleGroupIdxs[m.canIGroupCursor]].Resources
		if m.canIAllowedOnly {
			resources = filterAllowedResources(resources)
		}
	}
	subjectName := m.canISubjectName
	if subjectName == "" {
		subjectName = "Current User"
	}
	overlayW := min(m.width-4, m.width*90/100)
	overlayH := min(m.height-4, m.height*80/100)
	innerW := overlayW - 4
	innerH := overlayH - 2

	// Search bar shown inside the overlay. Normal hints moved to the main status bar.
	var hintBar string
	if m.canISearchActive {
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.canISearchInput.CursorLeft()) + ui.BarDimStyle.Render("█") + ui.BarNormalStyle.Render(m.canISearchInput.CursorRight())
		hintBar = ui.StatusBarBgStyle.Width(innerW).Render(searchBar)
	} else if m.canISearchQuery != "" {
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.canISearchQuery)
		hintBar = ui.StatusBarBgStyle.Width(innerW).Render(searchBar)
	}

	canIContent := ui.RenderCanIView(
		groupNames, resources,
		m.canIGroupCursor, m.canIGroupScroll,
		subjectName, m.canINamespaces,
		innerW, innerH,
		hintBar,
		m.canIResourceScroll,
		m.nsSelectionNegated,
	)
	// RBAC overlay uses baseBg end-to-end: title (TitleStyle/barBg=baseBg)
	// + column boxes (Active/InactiveColumnStyle/baseBg) + filler. Mixing
	// surfaceBg here would paint a visible "frame" of a different shade
	// around the inner baseBg content — the user reported this.
	canIContent = ui.FillLinesBg(canIContent, overlayW-4, ui.BaseBg)
	overlay := ui.OverlayStyle.
		Background(ui.BaseBg).
		BorderBackground(ui.BaseBg).
		Width(overlayW).Height(overlayH).
		Render(canIContent)
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}

// renderErrorLogOverlay renders the error log overlay on top of the given background.
// In fullscreen mode it replaces the background entirely. In overlay mode it centers on top.
func (m Model) renderErrorLogOverlay(background string) string {
	vp := ui.ErrorLogVisualParams{
		VisualMode:     m.errorLogVisualMode,
		VisualStart:    m.errorLogVisualStart,
		VisualStartCol: m.errorLogVisualStartCol,
		CursorLine:     m.errorLogCursorLine,
		CursorCol:      m.errorLogCursorCol,
	}

	if m.errorLogFullscreen {
		// Fullscreen rendering is handled by viewExplorer via the
		// viewErrorLogFullscreen helper (same pattern as the dashboard
		// fullscreen). The background passed in here is already that
		// composed view, so just return it unchanged.
		return background
	}

	overlayW := min(140, m.width-4)
	overlayH := min(30, m.height-4)
	if overlayW < 10 {
		overlayW = 10
	}
	if overlayH < 3 {
		overlayH = 3
	}

	// OverlayStyle adds 2 border + 2*2 horizontal padding + 2*1 vertical padding,
	// so the inner content area is overlayW-6 wide and overlayH-4 tall. Render
	// only that many lines so lipgloss does not expand the overlay to fit
	// overflowing content.
	innerW := overlayW - 6
	innerH := overlayH - 4
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	content := ui.RenderErrorLogOverlay(m.errorLog, m.errorLogScroll, innerW, innerH, m.showDebugLogs, vp)
	content = clampErrorLogLines(content, innerW, innerH)
	content = ui.FillLinesBg(content, innerW, ui.SurfaceBg)
	overlay := ui.BoxHeight(ui.BoxWidth(ui.OverlayStyle, overlayW), overlayH).Render(content)
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}

// clampErrorLogLines is a defensive safety net that caps the total line count
// at maxH and each line at maxW visual columns (cutting overflow with a
// trailing "~" via ui.Truncate). RenderErrorLogOverlay already wraps messages
// to width and paginates by physical line to fit its viewport, so with
// wrapping enabled this is ordinarily a no-op — it only fires for edge cases
// such as wide (CJK) runes whose visual width exceeds their rune count.
func clampErrorLogLines(content string, maxW, maxH int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > maxH {
		lines = lines[:maxH]
	}
	for i, line := range lines {
		lines[i] = ui.Truncate(line, maxW)
	}
	return strings.Join(lines, "\n")
}
