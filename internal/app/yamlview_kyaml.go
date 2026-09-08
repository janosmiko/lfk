package app

import (
	"sync/atomic"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/k8s"
)

// kyamlUnavailableMsg is what the features keyed to block-YAML line numbers
// report while the viewer is showing KYAML instead.
const kyamlUnavailableMsg = "Not available in KYAML mode"

// handleYAMLBlameKey gates blame: its per-line owner notes are indexed against
// block-YAML line numbers, which the KYAML rendering does not share.
func (m Model) handleYAMLBlameKey() (tea.Model, tea.Cmd) {
	if m.yamlView.kyaml {
		m.setStatusMessage(kyamlUnavailableMsg, true)
		return m, scheduleStatusClear()
	}
	return m.handleYAMLToggleBlame()
}

// handleYAMLObjectExplorerKey gates the jump: it resolves the object path from
// the cursor line, which only the block-YAML rendering can be parsed for.
func (m Model) handleYAMLObjectExplorerKey() (tea.Model, tea.Cmd) {
	if m.yamlView.kyaml {
		m.setStatusMessage(kyamlUnavailableMsg, true)
		return m, scheduleStatusClear()
	}
	return m.handleYAMLKeyObjectExplorer()
}

// yamlKYAMLRenderedMsg carries the finished re-render of the YAML viewer body.
// req numbers the request so a superseded conversion cannot repaint the view.
type yamlKYAMLRenderedMsg struct {
	content  string
	sections []yamlSection
	kyaml    bool
	announce bool
	req      uint64
	err      error
}

// toggleYAMLKYAML switches the viewer between block YAML and KYAML. Both
// directions re-render from source, so the round trip cannot lose the document.
func (m Model) toggleYAMLKYAML() (tea.Model, tea.Cmd) {
	if m.yamlView.source == "" {
		m.yamlView.source = m.yamlView.content
	}
	want := !m.yamlView.kyaml
	if want {
		m.setStatusMessage("Rendering KYAML...", false)
	}
	return m, m.startYAMLKYAMLRender(want, true)
}

// kyamlReqSeq issues KYAML re-render ids. Process-global like tabUIDSeq, so a
// conversion started on one tab can never be mistaken for another tab's: a
// per-view counter restarts at zero on each tab and would collide.
var kyamlReqSeq atomic.Uint64

// nextKYAMLReq claims an id and makes it the only reply this view will accept.
func (m *Model) nextKYAMLReq() uint64 {
	req := kyamlReqSeq.Add(1)
	m.yamlView.kyamlReq = req
	return req
}

// startYAMLKYAMLRender hands the re-render to a command. Both ToKYAML and
// parseYAMLSections walk the whole document, which stalls the event loop on a
// large CRD, so neither may run on the update path.
func (m *Model) startYAMLKYAMLRender(enable, announce bool) tea.Cmd {
	req := m.nextKYAMLReq()
	source := m.yamlView.source
	return func() tea.Msg {
		if !enable {
			return yamlKYAMLRenderedMsg{
				content:  source,
				sections: parseYAMLSections(source),
				announce: announce,
				req:      req,
			}
		}
		converted, err := k8s.ToKYAML(source)
		if err != nil {
			return yamlKYAMLRenderedMsg{req: req, err: err}
		}
		// sections stay nil: fold ranges address block-YAML line numbers.
		return yamlKYAMLRenderedMsg{content: converted, kyaml: true, announce: announce, req: req}
	}
}

// updateYAMLKYAMLRendered installs a finished re-render.
func (m Model) updateYAMLKYAMLRendered(msg yamlKYAMLRenderedMsg) (tea.Model, tea.Cmd) {
	if msg.req != m.yamlView.kyamlReq {
		// A newer toggle, or a freshly loaded document, already superseded this.
		return m, nil
	}
	if msg.err != nil {
		m.yamlView.kyaml = false
		m.setStatusMessage(msg.err.Error(), true)
		return m, scheduleStatusClear()
	}
	m.yamlView.kyaml = msg.kyaml
	m.yamlView.content = msg.content
	m.yamlView.sections = msg.sections
	m.resetYAMLViewPosition()
	if !msg.announce {
		return m, nil
	}
	if msg.kyaml {
		m.setStatusMessage("KYAML on. Press K for YAML.", false)
	} else {
		m.setStatusMessage("KYAML off", false)
	}
	return m, scheduleStatusClear()
}

// resetYAMLViewPosition drops everything that addresses the old line numbering.
func (m *Model) resetYAMLViewPosition() {
	m.yamlView.cursor = 0
	m.yamlView.scroll = 0
	m.yamlView.lineInput = ""
	m.yamlView.collapsed = map[string]bool{}
	m.yamlView.matchLines = nil
	m.yamlView.matchIdx = 0
	m.yamlView.visualMode = false
	m.yamlView.visualStart = 0
	m.yamlView.visualCol = 0
	// Column 0 is inside the fold gutter, so the first content column is where
	// the viewer parks a fresh cursor.
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.resetBlame()
}

// applyYAMLKYAMLRender adopts a freshly loaded document as the source. The
// bumped counter drops a conversion still in flight for the replaced document.
func (m *Model) applyYAMLKYAMLRender() tea.Cmd {
	m.yamlView.source = m.yamlView.content
	if !m.yamlView.kyaml {
		m.nextKYAMLReq()
		return nil
	}
	return m.startYAMLKYAMLRender(true, false)
}
