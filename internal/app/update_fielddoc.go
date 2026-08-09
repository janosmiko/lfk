package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

// fieldDocPaneHeight is the number of body lines the footnote takes away from
// the viewer above it, so the cursor keeps a viewport it fits in.
func (m Model) fieldDocPaneHeight() int {
	if !m.fieldDoc.on {
		return 0
	}
	return ui.FieldDocPaneHeight
}

// renderFieldDocPane draws the footnote, or nothing when it is closed.
func (m Model) renderFieldDocPane() string {
	if !m.fieldDoc.on {
		return ""
	}
	return ui.RenderFieldDocPane(m.width, ui.FieldDocPane{
		Path:      m.fieldDoc.key.path,
		FieldType: m.fieldDoc.entry.fieldType,
		Desc:      m.fieldDoc.entry.desc,
		Err:       m.fieldDoc.err,
		Loading:   m.fieldDoc.loading,
	})
}

// toggleFieldDoc opens or closes the schema footnote pane. Opening it shows the
// field under the cursor; closing it drops what was on screen but keeps the
// cache, so re-opening on a visited field costs nothing.
func (m Model) toggleFieldDoc(objPath []string) (tea.Model, tea.Cmd) {
	if m.fieldDoc.on {
		m.fieldDoc.reset()
		return m, nil
	}
	if _, ok := m.fieldDocKeyForPath(objPath); !ok {
		m.setStatusMessage("Cannot determine resource type", true)
		return m, scheduleStatusClear()
	}
	m.fieldDoc.on = true
	updated, cmd := m.showFieldDoc(objPath)
	return updated, cmd
}

// showFieldDoc points the pane at the field under the cursor. A description
// already read from this cluster renders at once; anything else starts the
// debounce, so walking the document line by line spawns no processes.
func (m Model) showFieldDoc(objPath []string) (Model, tea.Cmd) {
	if !m.fieldDoc.on {
		return m, nil
	}
	key, ok := m.fieldDocKeyForPath(objPath)
	if !ok {
		return m, nil
	}
	if key == m.fieldDoc.key && (m.fieldDoc.loading || m.fieldDoc.err != "" || m.fieldDoc.entry.desc != "") {
		// Already showing this field, or already fetching it.
		return m, nil
	}

	m.fieldDoc.key = key
	m.fieldDoc.err = ""
	// A new target invalidates any fetch still in flight.
	m.fieldDoc.req++

	if entry, hit := m.fieldDoc.cache.get(key); hit {
		m.fieldDoc.entry = entry
		m.fieldDoc.loading = false
		return m, nil
	}

	m.fieldDoc.entry = fieldDocEntry{}
	m.fieldDoc.loading = true
	return m, scheduleFieldDocFetch(m.fieldDoc.req)
}

// updateFieldDocDebounce runs when the cursor has rested long enough. It fetches
// only when the pane is still open and the request it was started for is still
// the current one, so a superseded timer spends nothing.
func (m Model) updateFieldDocDebounce(msg fieldDocDebounceMsg) (tea.Model, tea.Cmd) {
	if !m.fieldDoc.on || msg.req != m.fieldDoc.req || !m.fieldDoc.loading {
		return m, nil
	}
	return m, m.execKubectlExplainField(m.fieldDoc.req, m.fieldDoc.key)
}

// updateFieldDocLoaded stores a description and caches it. An empty description
// is cached too: a field the schema leaves undocumented is a real answer, and
// re-asking for it on every visit would spawn kubectl for nothing.
func (m Model) updateFieldDocLoaded(msg fieldDocLoadedMsg) Model {
	if msg.req != m.fieldDoc.req {
		// An earlier fetch answering late, for a field the cursor has left.
		return m
	}
	m.fieldDoc.loading = false
	if !m.fieldDoc.on {
		// The user closed the pane while the fetch was in flight.
		return m
	}
	if isContextCanceled(msg.err) {
		return m
	}
	if msg.err != nil {
		m.fieldDoc.err = msg.err.Error()
		return m
	}
	m.fieldDoc.cache.put(msg.key, msg.entry)
	m.fieldDoc.entry = msg.entry
	return m
}
