package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// enterWhoCanMode flips the Can-I overlay into reverse-RBAC mode and
// prepares the resource picker. The resource list is the deduped union
// of all Can-I groups so the user sees the same canonical names as in
// the forward view; the cursor lands on whatever resource was
// highlighted in Can-I (when reachable) so Tab feels like a
// continuation rather than a fresh start.
func (m Model) enterWhoCanMode() (tea.Model, tea.Cmd) {
	m.canIMode = canIModeWhoCan
	m.whoCan.verbCursor = 0 // "get"
	m.whoCan.resourceFilter.Clear()
	m.whoCan.resourceFilterActive = false
	m.whoCan.subjectsScroll = 0
	m.whoCan.subjects = nil

	m.whoCan.resourceList = whoCanCollectResources(m.canIGroups)
	m.whoCan.resourceCursor = 0
	m.whoCan.resourceScroll = 0

	// Pre-position cursor on the Can-I row's resource if it appears in
	// the picker — keeps the user's spatial context across the flip.
	if pre := m.canIResourceUnderCursor(); pre != "" {
		for i, r := range m.whoCan.resourceList {
			if r == pre {
				m.whoCan.resourceCursor = i
				break
			}
		}
	}
	// Pull the viewport down so the pre-positioned cursor is visible
	// instead of forcing the user to scroll to find where they are.
	m.whoCan.resourceScroll = ui.WhoCanScrollForCursor(
		0, m.whoCan.resourceCursor,
		m.whoCanVisibleLines(), len(m.whoCan.resourceList),
	)

	resource := whoCanCurrentResource(m.whoCan.resourceList, m.whoCan.resourceCursor)
	if resource == "" {
		// No resources in the picker (Can-I hadn't loaded any). Leave
		// the subjects panel idle; the user will see the empty list.
		m.whoCan.resource = ""
		return m, nil
	}
	m.whoCan.resource = resource
	m.whoCan.loading = true
	return m, m.loadWhoCan()
}

// canIResourceUnderCursor returns the resource name under the Can-I
// view's group cursor, or "" when nothing is selectable. Used to
// pre-fill the Who-Can resource so Tab carries the user's context
// across the mode flip.
func (m Model) canIResourceUnderCursor() string {
	groups := m.canIVisibleGroups()
	if m.canIGroupCursor < 0 || m.canIGroupCursor >= len(groups) {
		return ""
	}
	resources := m.canIGroups[groups[m.canIGroupCursor]].Resources
	if len(resources) == 0 {
		return ""
	}
	return resources[0].Resource
}

// whoCanCurrentResource returns the resource at the cursor's position
// in the (filtered) list, or "" when the cursor is out of range.
// Wraps the bounds check so callers don't repeat it.
func whoCanCurrentResource(list []string, cursor int) string {
	if cursor < 0 || cursor >= len(list) {
		return ""
	}
	return list[cursor]
}

// whoCanVisibleResources returns the post-filter list the user sees in
// the picker. Always builds from resourceList so a cleared filter
// instantly restores the full set without re-collecting from canIGroups.
func (m Model) whoCanVisibleResources() []string {
	return whoCanFilterResources(m.whoCan.resourceList, m.whoCan.resourceFilter.Value)
}

// handleWhoCanKey services key events while the overlay is in
// reverse-RBAC mode. Filter mode (typing into the resource picker)
// gets its own sub-handler.
func (m Model) handleWhoCanKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.whoCan.resourceFilterActive {
		return m.handleWhoCanFilterKey(msg)
	}
	switch msg.String() {
	case "tab":
		// Tab toggles back to forward Can-I.
		m.canIMode = canIModeForward
		return m, nil
	case "esc", "q":
		m.exitCanIView()
		return m, nil
	case "left", "h":
		return m.whoCanCycleVerb(-1)
	case "right", "l":
		return m.whoCanCycleVerb(+1)
	case "/":
		m.whoCan.resourceFilterActive = true
		m.whoCan.resourceFilter.Clear()
		return m, nil
	case "j", "down":
		return m.whoCanMoveCursor(+1)
	case "k", "up":
		return m.whoCanMoveCursor(-1)
	case "g", "home":
		return m.whoCanJumpCursor(0)
	case "G", "end":
		visible := m.whoCanVisibleResources()
		return m.whoCanJumpCursor(len(visible) - 1)
	case "ctrl+d":
		return m.whoCanMoveCursor(+m.whoCanVisibleLines() / 2)
	case "ctrl+u":
		return m.whoCanMoveCursor(-m.whoCanVisibleLines() / 2)
	case "ctrl+f", "pgdown":
		return m.whoCanMoveCursor(+m.whoCanVisibleLines())
	case "ctrl+b", "pgup":
		return m.whoCanMoveCursor(-m.whoCanVisibleLines())
	case "J":
		// Scroll subjects (right column) without moving the resource
		// cursor. Useful when a query returns more rows than fit.
		m.whoCan.subjectsScroll = whoCanClampSubjectsScroll(
			m.whoCan.subjectsScroll+1, len(m.whoCan.subjects), m.whoCanVisibleLines())
		return m, nil
	case "K":
		m.whoCan.subjectsScroll = max(m.whoCan.subjectsScroll-1, 0)
		return m, nil
	case "A":
		// Toggle namespace scope: empty string ↔ current namespace.
		if m.canINamespaces == nil || (len(m.canINamespaces) == 1 && m.canINamespaces[0] == "") {
			m.canINamespaces = []string{m.namespace}
		} else {
			m.canINamespaces = []string{""}
		}
		if m.whoCan.resource != "" {
			m.whoCan.loading = true
			return m, m.loadWhoCan()
		}
		return m, nil
	}
	return m, nil
}

// whoCanCycleVerb advances/retreats the verb cursor and re-fires the
// query so the user sees the new verb's subjects without needing a
// commit step. Returns no-op at the list edges (no wrap).
func (m Model) whoCanCycleVerb(delta int) (tea.Model, tea.Cmd) {
	next := m.whoCan.verbCursor + delta
	if next < 0 || next >= len(ui.WhoCanVerbs) {
		return m, nil
	}
	m.whoCan.verbCursor = next
	if m.whoCan.resource != "" {
		m.whoCan.loading = true
		return m, m.loadWhoCan()
	}
	return m, nil
}

// whoCanMoveCursor moves the resource cursor by delta within the
// visible (filtered) list, adjusts the scroll offset to keep the
// cursor in view (vim-like: only scroll when the cursor leaves the
// viewport), fires a Who-Can query for the new resource, and clamps
// at the list edges. No-op when the cursor wouldn't change so the
// user doesn't spam the API by mashing keys.
func (m Model) whoCanMoveCursor(delta int) (tea.Model, tea.Cmd) {
	visible := m.whoCanVisibleResources()
	if len(visible) == 0 {
		return m, nil
	}
	next := m.whoCan.resourceCursor + delta
	next = max(next, 0)
	if next >= len(visible) {
		next = len(visible) - 1
	}
	if next == m.whoCan.resourceCursor {
		return m, nil
	}
	m.whoCan.resourceCursor = next
	m.whoCan.resourceScroll = ui.WhoCanScrollForCursor(
		m.whoCan.resourceScroll, m.whoCan.resourceCursor,
		m.whoCanVisibleLines(), len(visible),
	)
	return m.refreshWhoCanForCursor(visible)
}

// whoCanJumpCursor positions the cursor at an absolute index (clamped
// to the visible list), adjusts the scroll, and fires a query for the
// new resource. Powers g/G/Home/End navigation.
func (m Model) whoCanJumpCursor(idx int) (tea.Model, tea.Cmd) {
	visible := m.whoCanVisibleResources()
	if len(visible) == 0 {
		return m, nil
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(visible) {
		idx = len(visible) - 1
	}
	if idx == m.whoCan.resourceCursor {
		return m, nil
	}
	m.whoCan.resourceCursor = idx
	m.whoCan.resourceScroll = ui.WhoCanScrollForCursor(
		m.whoCan.resourceScroll, m.whoCan.resourceCursor,
		m.whoCanVisibleLines(), len(visible),
	)
	return m.refreshWhoCanForCursor(visible)
}

// whoCanVisibleLines returns the number of body rows the resource
// picker / subjects column can show. MUST match the math in
// RenderWhoCanView and renderWhoCanResourcePicker exactly — when this
// is off by even one row, the scroll-for-cursor handler positions the
// cursor at a row the renderer doesn't actually show, and the user
// sees the last resource clipped past the bottom of the viewport.
func (m *Model) whoCanVisibleLines() int {
	overlayH := min(m.height-4, m.height*80/100)
	innerH := overlayH - 2       // OverlayStyle vertical padding
	contentH := max(innerH-4, 3) // -1 header row, -2 column borders, -1 reserved footer
	return max(contentH-1, 1)    // -1 for the picker's own "Resources …" header
}

// whoCanClampSubjectsScroll keeps the subjects-column scroll offset in
// the valid range so J/K can't scroll past the end of the list.
func whoCanClampSubjectsScroll(offset, total, visible int) int {
	maxScroll := max(total-visible, 0)
	if offset < 0 {
		return 0
	}
	if offset > maxScroll {
		return maxScroll
	}
	return offset
}

// refreshWhoCanForCursor commits the resource under the cursor and
// dispatches a fetch — but only when the resource actually changed,
// so j/k that lands on the same row (e.g. duplicate entries) doesn't
// re-fire. Also resets subjectsScroll so the new result starts at the
// top of its panel.
func (m Model) refreshWhoCanForCursor(visible []string) (tea.Model, tea.Cmd) {
	resource := whoCanCurrentResource(visible, m.whoCan.resourceCursor)
	if resource == "" || resource == m.whoCan.resource {
		return m, nil
	}
	m.whoCan.resource = resource
	m.whoCan.subjectsScroll = 0
	m.whoCan.loading = true
	return m, m.loadWhoCan()
}

// handleWhoCanFilterKey processes typing into the resource filter.
// As keystrokes land the visible list narrows and the cursor snaps to
// the top of the new list (with a query fire so the right pane keeps
// up). Enter accepts the filter and exits filter mode; Esc clears it.
func (m Model) handleWhoCanFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := handleFilterKey(&m.whoCan.resourceFilter, msg.String())
	switch action {
	case filterAccept:
		m.whoCan.resourceFilterActive = false
		visible := m.whoCanVisibleResources()
		if len(visible) == 0 {
			return m, nil
		}
		m.whoCan.resourceCursor = 0
		m.whoCan.resourceScroll = 0
		return m.refreshWhoCanForCursor(visible)
	case filterEscape:
		m.whoCan.resourceFilterActive = false
		m.whoCan.resourceFilter.Clear()
		// After clearing the filter the list returns to the full set, so
		// the cursor index that pointed into the narrowed list may now
		// land on a different resource. Refresh subjects against the
		// (now un-narrowed) cursor so the picker highlight and the
		// subjects pane stay in sync — without this, Esc would leave
		// the user looking at the previously-selected resource's
		// subjects while the picker highlights an unrelated row.
		visible := m.whoCanVisibleResources()
		if len(visible) == 0 {
			m.whoCan.resource = ""
			m.whoCan.resourceCursor = 0
			m.whoCan.resourceScroll = 0
			return m, nil
		}
		if m.whoCan.resourceCursor >= len(visible) {
			m.whoCan.resourceCursor = 0
		}
		m.whoCan.resourceScroll = 0
		return m.refreshWhoCanForCursor(visible)
	case filterClose:
		m.whoCan.resourceFilterActive = false
		m.whoCan.resourceFilter.Clear()
		m.exitCanIView()
		return m, nil
	}
	// Live-narrow: typing changed the filter, so re-fire for the new
	// top entry. Reset cursor + scroll to 0 because the previous
	// indices likely point past the end of the narrowed list.
	visible := m.whoCanVisibleResources()
	m.whoCan.resourceCursor = 0
	m.whoCan.resourceScroll = 0
	if len(visible) == 0 {
		return m, nil
	}
	return m.refreshWhoCanForCursor(visible)
}

// loadWhoCan dispatches the WhoCan fetch for the current verb +
// resource + namespace. Fires asynchronously; the result lands as
// whoCanLoadedMsg and is injected into m.whoCan.subjects by the
// handler.
//
// Note: callers are responsible for setting m.whoCan.loading = true
// before invoking this method. Doing it here would mutate the value
// receiver's copy, and the flag wouldn't persist on the Model returned
// to the Update handler — so the spinner would never show during the
// in-flight fetch.
func (m Model) loadWhoCan() tea.Cmd {
	verb := ui.WhoCanVerbs[m.whoCan.verbCursor]
	// "*" means "any verb"; passed through to verbMatches which treats
	// it as "match any rule with at least one verb". An earlier version
	// rewrote "*" to "get" — that silently dropped subjects whose roles
	// granted list/watch/etc. but not get.
	resource := m.whoCan.resource
	ns := ""
	if len(m.canINamespaces) == 1 && m.canINamespaces[0] != "" {
		ns = m.canINamespaces[0]
	}
	kctx := m.nav.Context
	gen := m.requestGen
	return m.trackBgTask(
		scheduler.KindResourceList,
		"WhoCan: "+verb+" "+resource,
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			subs, err := m.client.WhoCan(context.Background(), kctx, ns, "", resource, verb)
			return whoCanLoadedMsg{
				gen:      gen,
				verb:     verb,
				resource: resource,
				subjects: subs,
				err:      err,
			}
		},
	)
}

// whoCanLoadedMsg carries the WhoCan fetch result back to the runtime.
// gen guards against stale responses (user changed verb/resource
// before the previous fetch returned).
type whoCanLoadedMsg struct {
	gen      uint64
	verb     string
	resource string
	subjects []k8s.WhoCanSubject
	err      error
}

// renderWhoCanOverlay paints the reverse-RBAC view inside the same
// overlay frame that the forward Can-I uses — same dimensions, same
// PlaceOverlay wrap. Splitting the render path off renderCanIOverlay
// keeps each function focused and avoids churning the existing
// Can-I-only code.
func (m Model) renderWhoCanOverlay(background string) string {
	overlayW := min(m.width-4, m.width*90/100)
	overlayH := min(m.height-4, m.height*80/100)
	innerW := overlayW - 4
	innerH := overlayH - 2

	visible := m.whoCanVisibleResources()
	rows := make([]ui.WhoCanRow, len(m.whoCan.subjects))
	for i, s := range m.whoCan.subjects {
		rows[i] = ui.WhoCanRow{Kind: s.Kind, Name: s.Name, Namespace: s.Namespace, Via: s.Via}
	}

	// Match Can-I's scope label format ("ns: all" / "ns: <name>") so
	// Tab between modes shows the same scope chip in the same shape.
	nsLabel := ui.CanIScopeLabel(m.canINamespaces)

	// Filter input renders as the overlay footer — same vertical
	// position Can-I uses for its search bar so the input lands in the
	// same place when the user pivots between modes with Tab.
	var footerBar string
	switch {
	case m.whoCan.resourceFilterActive:
		footerBar = ui.StatusBarBgStyle.Width(innerW).Render(
			ui.HelpKeyStyle.Render("/") +
				ui.BarNormalStyle.Render(m.whoCan.resourceFilter.CursorLeft()) +
				ui.BarDimStyle.Render("█") +
				ui.BarNormalStyle.Render(m.whoCan.resourceFilter.CursorRight()),
		)
	case m.whoCan.resourceFilter.Value != "":
		footerBar = ui.StatusBarBgStyle.Width(innerW).Render(
			ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.whoCan.resourceFilter.Value),
		)
	}

	content := ui.RenderWhoCanView(ui.WhoCanViewParams{
		VerbCursor:     m.whoCan.verbCursor,
		Resources:      visible,
		ResourceCursor: m.whoCan.resourceCursor,
		ResourceScroll: m.whoCan.resourceScroll,
		NamespaceLabel: nsLabel,
		Subjects:       rows,
		SubjectsScroll: m.whoCan.subjectsScroll,
		Loading:        m.whoCan.loading,
		FooterBar:      footerBar,
		Width:          innerW,
		Height:         innerH,
	})
	// Match Can-I: baseBg end-to-end so the overlay frame doesn't show
	// as a different shade than the title bar / column boxes inside.
	content = ui.FillLinesBg(content, overlayW-4, ui.BaseBg)
	overlay := ui.OverlayStyle.
		Background(ui.BaseBg).
		BorderBackground(ui.BaseBg).
		Width(overlayW).Height(overlayH).
		Render(content)
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}

// updateWhoCanLoaded merges the fetch result into Model state so the
// renderer paints the new subject list on the next frame.
func (m Model) updateWhoCanLoaded(msg whoCanLoadedMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale; user moved on
	}
	m.whoCan.loading = false
	if msg.err != nil {
		m.setStatusMessage("WhoCan: "+msg.err.Error(), true)
		return m
	}
	m.whoCan.subjects = msg.subjects
	m.whoCan.subjectsScroll = 0
	return m
}
