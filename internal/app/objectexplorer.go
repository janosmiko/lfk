package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// objectExplorerState holds the full-screen Object Explorer: a drill-in
// navigation over the selected resource's live object (Item.Raw). It mirrors
// the API Explorer's interaction but shows actual values, and expands arrays
// into indexed elements so a recursively-nested status field is walkable.
type objectExplorerState struct {
	root   any                 // the resource's unstructured object
	path   []string            // current drill path (map keys and "[i]" indices)
	level  []model.ObjectField // navigable fields at the current path
	cursor int                 // index into the visible (filtered) level
	scroll int
	title  string // "Kind/Name"

	// In-level filter: substring match on keys at the current level.
	filter       string
	filterActive bool // true while the filter input is being typed

	// Scroll offset of the right-hand YAML preview pane.
	previewScroll int

	// Recursive find overlay across the whole object (see objectexplorer_find.go).
	// Presence of the overlay is tracked by m.overlay == overlayObjectExplorerFind.
	findResults      []model.ObjectMatch // current (filtered) results
	findFilter       string              // filter text typed in the overlay
	findFilterActive bool                // true while the filter input is focused
	findCursor       int
	findScroll       int
}

// visible returns the fields shown at the current level after applying the
// in-level filter. The cursor indexes into this slice.
func (rt *objectExplorerState) visible() []model.ObjectField {
	if rt.filter == "" {
		return rt.level
	}
	q := strings.ToLower(rt.filter)
	out := make([]model.ObjectField, 0, len(rt.level))
	for _, f := range rt.level {
		if strings.Contains(strings.ToLower(f.Key), q) {
			out = append(out, f)
		}
	}
	return out
}

// selected returns the field under the cursor, or ok=false when the (filtered)
// level is empty or the cursor is out of range.
func (rt *objectExplorerState) selected() (model.ObjectField, bool) {
	v := rt.visible()
	if rt.cursor < 0 || rt.cursor >= len(v) {
		return model.ObjectField{}, false
	}
	return v[rt.cursor], true
}

// openObjectExplorer enters the Object Explorer for the focused resource.
// It needs the live object (Item.Raw); synthetic items without one get a
// status message.
func (m Model) openObjectExplorer() (tea.Model, tea.Cmd) {
	if m.nav.Level < model.LevelResources {
		m.setStatusMessage("Select a resource first", true)
		return m, scheduleStatusClear()
	}
	sel := m.selectedMiddleItem()
	if sel == nil || sel.Raw == nil {
		m.setStatusMessage("No resource data available", true)
		return m, scheduleStatusClear()
	}
	m.objectExplorerView = objectExplorerState{
		root:  sel.Raw,
		title: sel.Kind + "/" + sel.Name,
	}
	m.objectExplorerView.level = model.ObjectFieldsAt(sel.Raw, nil)
	m.mode = modeObjectExplorer
	return m, nil
}

// exitObjectExplorer resets state and returns to the explorer.
func (m *Model) exitObjectExplorer() {
	m.mode = modeExplorer
	m.objectExplorerView = objectExplorerState{}
}

// handleObjectExplorerKey routes input to the active sub-mode (find / filter
// typing) or handles normal browsing keys.
func (m Model) handleObjectExplorerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.objectExplorerView.filterActive {
		return m.handleObjectExplorerFilterKey(msg)
	}
	return m.handleObjectExplorerNavKey(msg)
}

// handleObjectExplorerNavKey handles normal browsing keys.
func (m Model) handleObjectExplorerNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := ui.ActiveKeybindings
	rt := &m.objectExplorerView
	n := len(rt.visible())
	switch msg.String() {
	case "q":
		m.exitObjectExplorer()
		return m, nil
	case "esc":
		// Clear an active filter first, then back one level, then close.
		switch {
		case rt.filter != "":
			rt.filter = ""
			rt.cursor = 0
			rt.scroll = 0
			rt.previewScroll = 0
		case len(rt.path) > 0:
			m.objectExplorerBack()
		default:
			m.exitObjectExplorer()
		}
		return m, nil
	case "?", "f1":
		m.helpPreviousMode = modeObjectExplorer
		m.helpContextMode = "Object Explorer"
		m.mode = modeHelp
		return m, nil
	case "h", "left", "backspace":
		m.objectExplorerBack()
		return m, nil
	case "l", "right", "enter":
		m.objectExplorerDrill()
		return m, nil
	case "/":
		rt.filterActive = true
		return m, nil
	case "r":
		m.openObjectExplorerFind()
		return m, nil
	case "I":
		return m.openExplainAtObjectPath(m.selectedNodePath(), modeObjectExplorer)
	case "y":
		return m.copySelectedNodePath()
	case "Y":
		return m.copySelectedNodeYAML()
	case "P":
		return m.openSelectedResourceYAML()
	case kb.PreviewDown:
		rt.previewScroll++
		m.clampObjectExplorerPreviewScroll()
		return m, nil
	case kb.PreviewUp:
		rt.previewScroll--
		m.clampObjectExplorerPreviewScroll()
		return m, nil
	case kb.Down, "down", "j":
		if rt.cursor < n-1 {
			rt.cursor++
			rt.previewScroll = 0
		}
	case kb.Up, "up", "k":
		if rt.cursor > 0 {
			rt.cursor--
			rt.previewScroll = 0
		}
	case kb.JumpTop, "g", "home":
		rt.cursor = 0
		rt.previewScroll = 0
	case kb.JumpBottom, "G", "end":
		if n > 0 {
			rt.cursor = n - 1
			rt.previewScroll = 0
		}
	case kb.PageDown, "pgdown", "ctrl+f":
		m.moveObjectExplorerCursor(max(m.objectExplorerBodyHeight()/2, 1))
	case kb.PageUp, "pgup", "ctrl+b":
		m.moveObjectExplorerCursor(-max(m.objectExplorerBodyHeight()/2, 1))
	}
	m.clampObjectExplorerScroll()
	return m, nil
}

// handleObjectExplorerFilterKey handles typing in the in-level filter input.
func (m Model) handleObjectExplorerFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rt := &m.objectExplorerView
	switch msg.String() {
	case "esc":
		rt.filter = ""
		rt.filterActive = false
		rt.cursor = 0
		rt.scroll = 0
		rt.previewScroll = 0
	case "enter":
		// Keep the filter, leave typing mode so j/k navigate the results.
		rt.filterActive = false
	case "backspace":
		if rt.filter != "" {
			rt.filter = rt.filter[:len(rt.filter)-1]
		}
		rt.cursor = 0
		rt.previewScroll = 0
	default:
		if msg.Type == tea.KeyRunes {
			rt.filter += string(msg.Runes)
			rt.cursor = 0
			rt.previewScroll = 0
		}
	}
	m.clampObjectExplorerScroll()
	return m, nil
}

// copySelectedNodeYAML copies the YAML of the node under the cursor to the
// system clipboard.
func (m Model) copySelectedNodeYAML() (tea.Model, tea.Cmd) {
	f, ok := m.objectExplorerView.selected()
	if !ok {
		return m, nil
	}
	out := m.selectedNodeYAML()
	if out == "" {
		m.setStatusMessage("Nothing to copy", true)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("Copied "+f.Key+" to clipboard", false)
	return m, tea.Batch(copyToSystemClipboard(out), scheduleStatusClear())
}

// copySelectedNodePath copies the dotted path of the node under the cursor.
func (m Model) copySelectedNodePath() (tea.Model, tea.Cmd) {
	f, ok := m.objectExplorerView.selected()
	if !ok {
		return m, nil
	}
	full := append(append([]string{}, m.objectExplorerView.path...), f.Key)
	p := formatObjectPath(full)
	m.setStatusMessage("Copied path: "+p, false)
	return m, tea.Batch(copyToSystemClipboard(p), scheduleStatusClear())
}

// openSelectedResourceYAML hands off to the full-screen YAML viewer for the
// whole resource (the same object the tree is browsing).
func (m Model) openSelectedResourceYAML() (tea.Model, tea.Cmd) {
	m.mode = modeYAML
	m.yamlReturnMode = modeObjectExplorer
	m.yamlPendingPath = m.selectedNodePath() // sync the YAML cursor to this node on load
	m.yamlView.scroll = 0
	m.yamlView.content = "Loading..."
	m.yamlView.sections = nil
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	return m, m.loadYAML()
}

// selectedNodePath returns the full object path of the node under the tree
// cursor (current path + selected key), or nil when nothing is selected.
func (m Model) selectedNodePath() []string {
	rt := m.objectExplorerView
	f, ok := rt.selected()
	if !ok {
		return nil
	}
	return append(append([]string{}, rt.path...), f.Key)
}

// objectExplorerDrill descends into the object/array field under the cursor.
func (m *Model) objectExplorerDrill() {
	rt := &m.objectExplorerView
	f, ok := rt.selected()
	if !ok || !f.HasChildren {
		return
	}
	rt.path = append(append([]string{}, rt.path...), f.Key)
	rt.level = model.ObjectFieldsAt(rt.root, rt.path)
	rt.resetLevelView()
}

// objectExplorerBack pops one path segment and restores the cursor onto the
// field it was drilled from. At root it is a no-op (callers decide whether to
// close).
func (m *Model) objectExplorerBack() {
	rt := &m.objectExplorerView
	if len(rt.path) == 0 {
		return
	}
	last := rt.path[len(rt.path)-1]
	rt.path = rt.path[:len(rt.path)-1]
	rt.level = model.ObjectFieldsAt(rt.root, rt.path)
	rt.resetLevelView()
	for i, f := range rt.level {
		if f.Key == last {
			rt.cursor = i
			break
		}
	}
	m.clampObjectExplorerScroll()
}

// resetLevelView clears the filter and resets cursor/scroll after the current
// level changes (drill or back).
func (rt *objectExplorerState) resetLevelView() {
	rt.filter = ""
	rt.filterActive = false
	rt.cursor = 0
	rt.scroll = 0
	rt.previewScroll = 0
}

// moveObjectExplorerCursor advances the cursor by delta, clamped to the visible
// level, and resets the preview scroll.
func (m *Model) moveObjectExplorerCursor(delta int) {
	rt := &m.objectExplorerView
	rt.cursor = max(0, min(rt.cursor+delta, len(rt.visible())-1))
	rt.previewScroll = 0
}

// objectExplorerBodyHeight mirrors the contentHeight the renderer uses (title +
// hint bar + borders consume 4 lines).
func (m Model) objectExplorerBodyHeight() int {
	return max(m.height-4, 3)
}

// previewPaneHeight is the number of YAML lines visible in the preview pane.
func (m Model) previewPaneHeight() int {
	return max(m.height-5, 1) // contentHeight (height-4) minus the header line
}

// clampObjectExplorerPreviewScroll keeps the preview scroll within the selected node's YAML.
func (m *Model) clampObjectExplorerPreviewScroll() {
	rt := &m.objectExplorerView
	lines := strings.Count(m.selectedNodeYAML(), "\n")
	maxScroll := max(0, lines-m.previewPaneHeight())
	rt.previewScroll = max(0, min(rt.previewScroll, maxScroll))
}

// clampObjectExplorerScroll keeps the cursor within the visible window.
func (m *Model) clampObjectExplorerScroll() {
	rt := &m.objectExplorerView
	if rt.cursor < 0 {
		rt.cursor = 0
	}
	if n := len(rt.visible()); rt.cursor >= n {
		rt.cursor = max(0, n-1)
	}
	visible := max(m.objectExplorerBodyHeight()-1, 1) // -1 for the column header
	if rt.cursor < rt.scroll {
		rt.scroll = rt.cursor
	}
	if rt.cursor >= rt.scroll+visible {
		rt.scroll = rt.cursor - visible + 1
	}
	if rt.scroll < 0 {
		rt.scroll = 0
	}
}

// viewObjectExplorer renders the browser: PATH breadcrumb | NAME/VALUE list |
// YAML preview of the selected node's subtree.
func (m Model) viewObjectExplorer() string {
	rt := m.objectExplorerView

	hint := ui.RenderHintBar([]ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "l/Enter", Desc: "drill"},
		{Key: "h/Esc", Desc: "back"},
		{Key: "/", Desc: "filter"},
		{Key: "r", Desc: "find"},
		{Key: "J/K", Desc: "scroll preview"},
		{Key: "y/Y", Desc: "yank path/node"},
		{Key: "P", Desc: "full yaml"},
		{Key: "I", Desc: "explain"},
		{Key: "q", Desc: "close"},
	}, m.width)

	parentFields, parentCursor := m.objectExplorerParentLevel()
	return ui.RenderObjectExplorerView(
		rt.visible(),
		rt.cursor,
		rt.scroll,
		parentFields,
		parentCursor,
		m.selectedNodeYAML(),
		rt.previewScroll,
		rt.filterBar(),
		hint,
		m.width,
		m.height,
	)
}

// objectExplorerParentLevel returns the fields of the level above the current
// one and the index of the field that was drilled into, for the left (parent)
// pane. Returns nil at the top level.
func (m Model) objectExplorerParentLevel() ([]model.ObjectField, int) {
	rt := m.objectExplorerView
	if len(rt.path) == 0 {
		return nil, 0
	}
	parent := rt.path[:len(rt.path)-1]
	fields := model.ObjectFieldsAt(rt.root, parent)
	key := rt.path[len(rt.path)-1]
	for i, f := range fields {
		if f.Key == key {
			return fields, i
		}
	}
	return fields, 0
}

// filterBar returns the filter input string to render at the bottom (in place
// of the hint bar), or "" when no filter is set or being typed.
func (rt *objectExplorerState) filterBar() string {
	if !rt.filterActive && rt.filter == "" {
		return ""
	}
	caret := ""
	if rt.filterActive {
		caret = "█"
	}
	return rt.filter + caret
}

// selectedNodeYAML marshals the value under the cursor to YAML for the preview
// pane. Returns "" when there is no valid selection.
func (m Model) selectedNodeYAML() string {
	rt := m.objectExplorerView
	f, ok := rt.selected()
	if !ok {
		return ""
	}
	segs := append(append([]string{}, rt.path...), f.Key)
	val, ok := model.ResolveObjectPath(rt.root, segs)
	if !ok {
		return ""
	}
	out, err := yaml.Marshal(val)
	if err != nil {
		return ""
	}
	return string(out)
}

// formatObjectPath renders path segments as a readable dotted path, appending
// array indices like "steps[0]" rather than "steps.[0]".
func formatObjectPath(segs []string) string {
	var b strings.Builder
	for _, s := range segs {
		if strings.HasPrefix(s, "[") {
			b.WriteString(s)
			continue
		}
		if b.Len() > 0 {
			b.WriteString(".")
		}
		b.WriteString(s)
	}
	return b.String()
}
