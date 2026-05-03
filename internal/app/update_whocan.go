package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// enterWhoCanMode flips the Can-I overlay into reverse-RBAC mode and
// fires the first fetch. Verb defaults to "get" (the most common
// question users ask) and resource pre-fills from the Can-I cursor's
// current row when one is highlighted, so Tab feels like a continuation
// of the previous question rather than a fresh start.
func (m Model) enterWhoCanMode() (tea.Model, tea.Cmd) {
	m.canIMode = canIModeWhoCan
	m.whoCan.verbCursor = 0 // "get"
	m.whoCan.resourceFilter.Clear()
	m.whoCan.resourceFilterActive = false
	m.whoCan.subjectsScroll = 0

	// Pre-fill resource from the highlighted Can-I row, if any.
	if m.whoCan.resource == "" {
		if r := m.canIResourceUnderCursor(); r != "" {
			m.whoCan.resource = r
		}
	}
	if m.whoCan.resource == "" {
		// No prior selection; leave the resource empty so the user is
		// prompted to type a filter rather than seeing an unrelated
		// pre-pick.
		return m, nil
	}
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
		if m.whoCan.verbCursor > 0 {
			m.whoCan.verbCursor--
			if m.whoCan.resource != "" {
				return m, m.loadWhoCan()
			}
		}
		return m, nil
	case "right", "l":
		if m.whoCan.verbCursor < len(ui.WhoCanVerbs)-1 {
			m.whoCan.verbCursor++
			if m.whoCan.resource != "" {
				return m, m.loadWhoCan()
			}
		}
		return m, nil
	case "/":
		m.whoCan.resourceFilterActive = true
		m.whoCan.resourceFilter.Clear()
		return m, nil
	case "j", "down":
		m.whoCan.subjectsScroll++
		return m, nil
	case "k", "up":
		if m.whoCan.subjectsScroll > 0 {
			m.whoCan.subjectsScroll--
		}
		return m, nil
	case "A":
		// Toggle namespace scope: empty string ↔ current namespace.
		if m.canINamespaces == nil || (len(m.canINamespaces) == 1 && m.canINamespaces[0] == "") {
			m.canINamespaces = []string{m.namespace}
		} else {
			m.canINamespaces = []string{""}
		}
		if m.whoCan.resource != "" {
			return m, m.loadWhoCan()
		}
		return m, nil
	}
	return m, nil
}

// handleWhoCanFilterKey processes typing into the resource filter.
// Enter commits the filter as the resource and fires a fetch; Esc
// discards the typing and falls back to the prior resource.
func (m Model) handleWhoCanFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := handleFilterKey(&m.whoCan.resourceFilter, msg.String())
	switch action {
	case filterAccept:
		m.whoCan.resourceFilterActive = false
		if v := m.whoCan.resourceFilter.Value; v != "" {
			m.whoCan.resource = v
			m.whoCan.subjectsScroll = 0
			return m, m.loadWhoCan()
		}
		return m, nil
	case filterEscape:
		m.whoCan.resourceFilterActive = false
		m.whoCan.resourceFilter.Clear()
		return m, nil
	case filterClose:
		m.whoCan.resourceFilterActive = false
		m.whoCan.resourceFilter.Clear()
		m.exitCanIView()
		return m, nil
	}
	return m, nil
}

// loadWhoCan dispatches the WhoCan fetch for the current verb +
// resource + namespace. Fires asynchronously; the result lands as
// whoCanLoadedMsg and is injected into m.whoCan.subjects by the
// handler.
func (m Model) loadWhoCan() tea.Cmd {
	verb := ui.WhoCanVerbs[m.whoCan.verbCursor]
	if verb == "*" {
		// API matches "*" against ANY rule.Verbs, but for the query we
		// need a concrete verb. Send "get" and let the algorithm's
		// wildcard match handle it — any rule that grants "*" matches.
		// Handing "*" through unchanged would also work because the
		// match is symmetric, but "get" makes the fetch result useful
		// to humans reading the underlying rules too.
		verb = "get"
	}
	resource := m.whoCan.resource
	ns := ""
	if len(m.canINamespaces) == 1 && m.canINamespaces[0] != "" {
		ns = m.canINamespaces[0]
	}
	kctx := m.nav.Context
	gen := m.requestGen
	m.whoCan.loading = true
	return m.trackBgTask(
		bgtasks.KindResourceList,
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

	rows := make([]ui.WhoCanRow, len(m.whoCan.subjects))
	for i, s := range m.whoCan.subjects {
		rows[i] = ui.WhoCanRow{Kind: s.Kind, Name: s.Name, Namespace: s.Namespace, Via: s.Via}
	}

	nsLabel := "all-namespaces"
	if len(m.canINamespaces) == 1 && m.canINamespaces[0] != "" {
		nsLabel = "ns: " + m.canINamespaces[0]
	}

	content := ui.RenderWhoCanView(
		m.whoCan.verbCursor,
		m.whoCan.resource,
		m.whoCan.resourceFilter.Value,
		m.whoCan.resourceFilterActive,
		nsLabel,
		rows,
		m.whoCan.subjectsScroll,
		m.whoCan.loading,
		innerW, innerH,
	)
	content = ui.FillLinesBg(content, overlayW-4, ui.SurfaceBg)
	overlay := ui.OverlayStyle.Width(overlayW).Height(overlayH).Render(content)
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
