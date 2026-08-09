package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	name   string // the resource's name, for the breadcrumb

	// Identity of the browsed resource, captured at open so a watch-mode
	// refresh can re-sync root from the matching list item (and skip the sync
	// when the resource was deleted and the cursor lands on a different row).
	namespace string
	kind      string
	extra     string
	cluster   string

	// In-level filter: substring match on keys at the current level (or on
	// keys anywhere in the subtree while tree mode is on).
	filter       string
	filterActive bool // true while the filter input is being typed

	// Tree mode (issue #417): the middle column shows the expanded subtree at
	// path with ASCII-art guides instead of the flat current level. treeRows
	// is the pre-order flattened subtree, rebuilt on every path/root change.
	// treeCollapsed holds the folded rows (space / toggle_fold), keyed by
	// their relative segs; cleared whenever the tree re-roots.
	tree          bool
	treeRows      []model.ObjectTreeRow
	treeCollapsed map[string]struct{}

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
// in-level filter. The cursor indexes into this slice. In tree mode the
// fields mirror the visible tree rows so cursor math stays shared.
func (rt *objectExplorerState) visible() []model.ObjectField {
	if rt.tree {
		rows := rt.visibleTreeRows()
		out := make([]model.ObjectField, len(rows))
		for i, r := range rows {
			out[i] = r.Field
		}
		return out
	}
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
		root:      sel.Raw,
		title:     resourceTitleLabel(sel.Kind, sel.Namespace, sel.Name),
		name:      sel.Name,
		namespace: sel.Namespace,
		kind:      sel.Kind,
		extra:     sel.Extra,
		cluster:   sel.ClusterName,
	}
	m.objectExplorerView.level = model.ObjectFieldsAt(sel.Raw, nil)
	// Session tree-view preference (seeded from ui.ConfigObjectExplorerTree,
	// updated by the T toggle).
	if m.objectExplorerTree {
		m.objectExplorerView.tree = true
		m.objectExplorerView.rebuildTreeRows()
	}
	m.objectExplorerForceSync = false
	// Remember the opener (the explorer, or the YAML viewer when opened via P)
	// so q/esc returns there. m.mode is still the opener at this point.
	m.objectExplorerReturnMode = m.mode
	m.mode = modeObjectExplorer
	return m, nil
}

// syncObjectExplorerLive re-points the Object Explorer at the freshly-loaded
// object after a list refresh (watch tick or manual reload) so .status and
// other live fields update in place instead of showing the snapshot captured at
// open (issue #391). Navigation state (drill path, focused key, scroll) is
// preserved. It is a no-op unless the explorer is open, and it keeps the last
// snapshot when the browsed resource is gone or the cursor now points at a
// different row, rather than silently swapping in unrelated content.
func (m *Model) syncObjectExplorerLive() {
	if m.mode != modeObjectExplorer {
		return
	}
	// Live off: leave the snapshot frozen so the view doesn't shift under the
	// user. A manual refresh (R) sets objectExplorerForceSync to apply one
	// update anyway; consume it here so it stays a single-shot.
	if !m.objectExplorerLive && !m.objectExplorerForceSync {
		return
	}
	m.objectExplorerForceSync = false
	rt := &m.objectExplorerView
	if rt.root == nil {
		return
	}
	sel := m.selectedMiddleItem()
	if sel == nil || sel.Raw == nil ||
		sel.Name != rt.name || sel.Namespace != rt.namespace ||
		sel.Kind != rt.kind || sel.Extra != rt.extra || sel.ClusterName != rt.cluster {
		return
	}

	// Keep the cursor on the same field across the rebuild: values change every
	// tick, map keys rarely do. In tree mode track the full relative path.
	var focusedKey string
	var focusedSegs []string
	if rt.tree {
		if row, ok := rt.selectedTreeRow(); ok {
			focusedSegs = row.Segs
		}
	} else if f, ok := rt.selected(); ok {
		focusedKey = f.Key
	}

	rt.root = sel.Raw
	// Trim the drill path to its deepest still-resolvable prefix in case a
	// field the user had drilled into disappeared (e.g. an array shrank). The
	// trimmed-into segment is gone from the rebuilt level, so the focused-key
	// restore below misses and the cursor clamps to a valid row.
	for len(rt.path) > 0 {
		if _, ok := model.ResolveObjectPath(rt.root, rt.path); ok {
			break
		}
		rt.path = rt.path[:len(rt.path)-1]
	}
	rt.level = model.ObjectFieldsAt(rt.root, rt.path)

	if rt.tree {
		rt.rebuildTreeRows()
		if focusedSegs != nil {
			rt.cursorOnTreeSegs(focusedSegs)
		}
	} else if focusedKey != "" {
		for i, f := range rt.visible() {
			if f.Key == focusedKey {
				rt.cursor = i
				break
			}
		}
	}
	m.clampObjectExplorerScroll()
	m.clampObjectExplorerPreviewScroll()

	// Refresh the recursive-find overlay against the new object, preserving the
	// user's cursor/scroll within it.
	if m.overlay == overlayObjectExplorerFind {
		savedCursor, savedScroll := rt.findCursor, rt.findScroll
		m.recomputeFind()
		rt.findCursor = max(0, min(savedCursor, len(rt.findResults)-1))
		rt.findScroll = savedScroll
		m.clampFindScroll()
	}
}

// toggleObjectExplorerLive flips live auto-refresh for the Object Explorer.
// Turning it on triggers an immediate catch-up refresh so the view jumps to the
// current state instead of waiting for the next watch tick.
func (m Model) toggleObjectExplorerLive() (tea.Model, tea.Cmd) {
	m.objectExplorerLive = !m.objectExplorerLive
	if m.objectExplorerLive {
		m.setStatusMessage("Object Explorer live refresh: on", false)
		m.objectExplorerForceSync = true
		return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
	}
	m.setStatusMessage("Object Explorer live refresh: off", false)
	return m, scheduleStatusClear()
}

// refreshObjectExplorer fetches the current resource list and applies the result
// to the Object Explorer once, regardless of the live setting.
func (m Model) refreshObjectExplorer() (tea.Model, tea.Cmd) {
	m.objectExplorerForceSync = true
	m.setStatusMessage("Refreshing…", false)
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

// exitObjectExplorer resets state and returns to the mode the Object Explorer
// was opened from (the explorer by default; the YAML viewer when opened via P).
func (m *Model) exitObjectExplorer() {
	m.mode = m.objectExplorerReturnMode
	m.objectExplorerReturnMode = modeExplorer
	m.objectExplorerView = objectExplorerState{}
}

// handleObjectExplorerKey routes input to the active sub-mode (find / filter
// typing) or handles normal browsing keys.
func (m Model) handleObjectExplorerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.objectExplorerView.filterActive {
		return m.handleObjectExplorerFilterKey(msg)
	}
	mdl, cmd := m.handleObjectExplorerNavKey(msg)
	return followFieldDocCursor(mdl, cmd, func(u Model) []string { return u.selectedNodePath() })
}

// handleObjectExplorerNavKey handles normal browsing keys.
func (m Model) handleObjectExplorerNavKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	case kb.WatchMode:
		return m.toggleObjectExplorerLive()
	case kb.TreeView:
		return m.toggleObjectExplorerTree()
	case "space", kb.ToggleFold:
		return m.toggleObjectExplorerTreeFold()
	case kb.Refresh:
		return m.refreshObjectExplorer()
	case "I":
		return m.openExplainAtObjectPath(m.selectedNodePath(), modeObjectExplorer)
	// Before PreviewDown/PreviewUp: a user who rebinds the footnote onto J or K
	// gets the footnote, rather than a key that silently does nothing.
	case kb.FieldDoc:
		return m.toggleFieldDoc(m.selectedNodePath())
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
	case kb.PageDown, "pgdown", "ctrl+f", "shift+down":
		m.moveObjectExplorerCursor(max(m.objectExplorerBodyHeight()/2, 1))
	case kb.PageUp, "pgup", "ctrl+b", "shift+up":
		m.moveObjectExplorerCursor(-max(m.objectExplorerBodyHeight()/2, 1))
	}
	m.clampObjectExplorerScroll()
	return m, nil
}

// handleObjectExplorerFilterKey handles typing in the in-level filter input.
func (m Model) handleObjectExplorerFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
		if msg.Text != "" {
			rt.filter += msg.Text
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
	full := m.selectedNodePath()
	if full == nil {
		return m, nil
	}
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
	m.yamlView.resetBlame()
	return m, m.loadYAML()
}

// selectedNodePath returns the full object path of the node under the tree
// cursor (current path + selected key, or + the selected tree row's relative
// path in tree mode), or nil when nothing is selected.
func (m Model) selectedNodePath() []string {
	rt := m.objectExplorerView
	if rt.tree {
		row, ok := rt.selectedTreeRow()
		if !ok {
			return nil
		}
		return append(append([]string{}, rt.path...), row.Segs...)
	}
	f, ok := rt.selected()
	if !ok {
		return nil
	}
	return append(append([]string{}, rt.path...), f.Key)
}

// objectExplorerDrill descends into the object/array field under the cursor.
// In tree mode the tree re-roots at the selected node's path.
func (m *Model) objectExplorerDrill() {
	m.wheel.dead = true // drilling in empties the wheel momentum queue (#524)
	rt := &m.objectExplorerView
	if rt.tree {
		row, ok := rt.selectedTreeRow()
		if !ok || !row.Field.HasChildren {
			return
		}
		rt.path = append(append([]string{}, rt.path...), row.Segs...)
		rt.level = model.ObjectFieldsAt(rt.root, rt.path)
		rt.resetLevelView()
		rt.rebuildTreeRows()
		return
	}
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
	m.wheel.dead = true // backing out empties the wheel momentum queue (#524)
	rt := &m.objectExplorerView
	if len(rt.path) == 0 {
		return
	}
	last := rt.path[len(rt.path)-1]
	rt.path = rt.path[:len(rt.path)-1]
	rt.level = model.ObjectFieldsAt(rt.root, rt.path)
	rt.resetLevelView()
	if rt.tree {
		rt.rebuildTreeRows()
		// A single-element segs match is always right here: back pops exactly
		// one segment, so the drilled-from node sits at depth 0 of the rebuilt
		// tree under its bare key.
		rt.cursorOnTreeSegs([]string{last})
	} else {
		for i, f := range rt.level {
			if f.Key == last {
				rt.cursor = i
				break
			}
		}
	}
	m.clampObjectExplorerScroll()
}

// resetLevelView clears the filter, folds, and cursor/scroll after the
// current level changes (drill or back).
func (rt *objectExplorerState) resetLevelView() {
	rt.filter = ""
	rt.filterActive = false
	rt.cursor = 0
	rt.scroll = 0
	rt.previewScroll = 0
	rt.treeCollapsed = nil
}

// moveObjectExplorerCursor advances the cursor by delta, clamped to the visible
// level, and resets the preview scroll.
func (m *Model) moveObjectExplorerCursor(delta int) {
	rt := &m.objectExplorerView
	rt.cursor = max(0, min(rt.cursor+delta, len(rt.visible())-1))
	rt.previewScroll = 0
}

// objectExplorerBodyHeight mirrors the contentHeight the renderer uses
// (title + hint bar + column borders + outer frame consume 6 lines).
func (m Model) objectExplorerBodyHeight() int {
	return max(m.height-6, 3)
}

// previewPaneHeight is the number of YAML lines visible in the preview pane.
func (m Model) previewPaneHeight() int {
	return max(m.objectExplorerBodyHeight()-1, 1) // minus the header line
}

// clampObjectExplorerPreviewScroll keeps the preview scroll within the selected node's YAML.
func (m *Model) clampObjectExplorerPreviewScroll() {
	rt := &m.objectExplorerView
	lines := strings.Count(m.selectedNodeYAML(), "\n")
	maxScroll := max(0, lines-m.previewPaneHeight())
	rt.previewScroll = max(0, min(rt.previewScroll, maxScroll))
}

// clampObjectExplorerScroll keeps the cursor within the visible window with
// the vim-style scrolloff margin.
func (m *Model) clampObjectExplorerScroll() {
	rt := &m.objectExplorerView
	n := len(rt.visible())
	if rt.cursor < 0 {
		rt.cursor = 0
	}
	if rt.cursor >= n {
		rt.cursor = max(0, n-1)
	}
	visible := max(m.objectExplorerBodyHeight()-1, 1) // -1 for the column header
	identity := func(from, to int) int { return to - from }
	rt.scroll = ui.VimScrollOff(rt.scroll, rt.cursor, n, visible, ui.ConfigScrollOff, identity)
}

// viewObjectExplorer renders the browser: PATH breadcrumb | NAME/VALUE list |
// YAML preview of the selected node's subtree.
func (m Model) viewObjectExplorer() string {
	rt := m.objectExplorerView

	kb := ui.ActiveKeybindings
	hints := []ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "l/Enter", Desc: "drill"},
		{Key: "h/Esc", Desc: "back"},
		{Key: "/", Desc: "filter"},
		{Key: "r", Desc: "find"},
		{Key: kb.TreeView, Desc: "tree"},
	}
	if rt.tree {
		hints = append(hints, ui.HintEntry{Key: "space", Desc: "fold"})
	}
	hints = append(hints,
		ui.HintEntry{Key: "J/K", Desc: "scroll preview"},
		ui.HintEntry{Key: "y/Y", Desc: "yank path/node"},
		ui.HintEntry{Key: "P", Desc: "full yaml"},
		ui.HintEntry{Key: kb.Refresh, Desc: "refresh"},
		ui.HintEntry{Key: kb.WatchMode, Desc: "live on/off"},
		ui.HintEntry{Key: "I", Desc: "explain"},
		ui.HintEntry{Key: kb.FieldDoc, Desc: "field doc"},
		ui.HintEntry{Key: "q", Desc: "close"},
	)
	// The schema pane takes columns off the right, so narrow m.width before
	// anything below measures itself against it. m is a value copy, so the
	// narrowing stays inside this render.
	schemaPane := m.renderFieldDocPane(m.height, false)
	if schemaPane != "" {
		m.width -= lipgloss.Width(schemaPane)
	}

	hint := ui.RenderHintBar(hints, m.width)

	title := "Object Explorer: " + rt.title
	if !m.objectExplorerLive {
		title += " [PAUSED]"
	}
	if rt.tree {
		title += " [TREE]"
	}

	if rt.tree {
		return joinFieldDocPane(m.viewObjectExplorerTree(title, hint), schemaPane)
	}

	parentFields, parentCursor := m.objectExplorerParentLevel()
	return joinFieldDocPane(ui.RenderObjectExplorerView(
		rt.visible(),
		rt.cursor,
		rt.scroll,
		title,
		parentFields,
		parentCursor,
		m.selectedNodeYAML(),
		rt.previewScroll,
		rt.filterBar(),
		hint,
		m.width,
		m.height,
	), schemaPane)
}

// joinFieldDocPane puts the schema pane beside a rendered view, or returns the
// view untouched when the pane is closed or does not fit.
func joinFieldDocPane(view, pane string) string {
	if pane == "" {
		return view
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, view, pane)
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
	segs := m.selectedNodePath()
	if segs == nil {
		return ""
	}
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

// formatObjectPath renders path segments as a readable dotted path; see
// model.FormatObjectPath.
func formatObjectPath(segs []string) string {
	return model.FormatObjectPath(segs)
}
