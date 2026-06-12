package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// API Explorer tree mode (issue #417): one `kubectl explain --recursive` call
// renders the whole field subtree of the current schema path with ASCII-art
// guides. Tree mode is a per-level visualization — any level load (drill,
// back, recursive-overlay jump) returns to the flat list.

// explainTreeLoadedMsg carries the recursive field tree for the API
// Explorer's tree mode. fields are pre-order with absolute schema paths; path
// is the tree root's schema path ("" at the resource root).
type explainTreeLoadedMsg struct {
	fields []model.ExplainField
	path   string
	err    error
}

// explainTreeDescMsg carries the per-field descriptions of one schema level
// (a plain kubectl explain of parent), merged into the tree rows under it.
// resource, apiVersion, and kctx identify the request so results arriving
// after the explorer moved to a different resource type, API version, or
// cluster context are dropped instead of merged into the wrong tree.
type explainTreeDescMsg struct {
	resource   string
	apiVersion string
	kctx       string
	parent     string
	fields     []model.ExplainField
	err        error
}

// explainTreeState holds the API Explorer's tree-mode state, embedded in
// Model.
type explainTreeState struct {
	explainTree          bool                 // tree mode: explainFields holds the recursive subtree
	explainTreeDepths    []int                // per-field nesting depth relative to explainPath
	explainTreeAll       []model.ExplainField // full tree before folds are applied
	explainTreeCollapsed map[string]struct{}  // folded subtree roots, keyed by schema path
	explainTreeWanted    bool                 // sticky: re-enter tree mode after every level load
	explainFlatLevel     explainLevel         // flat level stashed while tree mode is on

	// Recursive explain output carries no per-field descriptions, so tree
	// mode lazily describes one schema level at a time (a plain kubectl
	// explain of the cursor row's parent path) and caches which levels are
	// done / in flight for the life of the tree.
	explainTreeDescFetched  map[string]struct{} // levels whose descriptions are merged (or failed)
	explainTreeDescInflight map[string]struct{} // levels with a fetch currently in flight
}

// toggleExplainTree flips the API Explorer between the flat field list and
// the recursive tree. Turning it on fetches the subtree and makes tree mode
// sticky (every subsequent level load re-enters it); turning it off restores
// the flat level captured at fetch time without a re-fetch.
func (m Model) toggleExplainTree() (tea.Model, tea.Cmd) {
	if m.explainTree {
		m.explainTreeWanted = false
		m.restoreExplainFlatLevel()
		return m, nil
	}
	if m.explainTreeWanted {
		// A tree fetch is already in flight (or a sticky re-entry pending):
		// a second press cancels instead of firing a duplicate fetch. The
		// in-flight result is dropped by the wanted guard on arrival.
		m.explainTreeWanted = false
		return m, nil
	}
	m.explainTreeWanted = true
	m.loading = true
	m.setStatusMessage("Loading field tree...", false)
	return m, m.execKubectlExplainTree(m.explainResource, m.explainAPIVersion, m.explainPath)
}

// restoreExplainFlatLevel leaves tree mode and reinstates the flat field list
// (and cursor/scroll) that was active when the tree was loaded.
func (m *Model) restoreExplainFlatLevel() {
	if !m.explainTree {
		return
	}
	fields, cursor, scroll := m.explainFlatLevel.fields, m.explainFlatLevel.cursor, m.explainFlatLevel.scroll
	m.resetExplainTree()
	m.explainFields = fields
	m.explainCursor = cursor
	m.explainScroll = scroll
}

// resetExplainTree drops all tree-mode state without restoring the previous
// flat level. Used when a fresh level has just been loaded. The sticky
// explainTreeWanted preference survives — only the T toggle clears it.
func (m *Model) resetExplainTree() {
	m.explainTree = false
	m.explainTreeDepths = nil
	m.explainTreeAll = nil
	m.explainTreeCollapsed = nil
	m.explainFlatLevel = explainLevel{}
	m.explainTreeDescFetched = nil
	m.explainTreeDescInflight = nil
}

// updateExplainTreeLoaded swaps the recursive field tree into the active
// field list so navigation and search work unchanged, keeping the flat level
// aside for the toggle-off restore. The cursor follows the flat selection
// onto the matching tree row.
func (m Model) updateExplainTreeLoaded(msg explainTreeLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	// Dropped silently when the user canceled tree mode while the fetch was
	// in flight (every live tree load has explainTreeWanted set).
	if !m.explainTreeWanted {
		return m, nil
	}
	if msg.err != nil {
		m.setErrorFromErr("Field tree failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	if len(msg.fields) == 0 {
		m.setStatusMessage("No fields found", true)
		return m, scheduleStatusClear()
	}
	// Drop stale results: the user may have drilled to a different level
	// while the recursive fetch was in flight.
	if msg.path != m.explainPath {
		return m, nil
	}
	var selPath string
	if !m.explainTree && m.explainCursor >= 0 && m.explainCursor < len(m.explainFields) {
		selPath = m.explainFields[m.explainCursor].Path
	}
	// The tree's depth-0 rows are the same fields the flat level showed —
	// carry their descriptions over and mark the level as described.
	seedExplainDescriptions(msg.fields, m.explainFields)
	m.explainTreeDescFetched = map[string]struct{}{m.explainPath: {}}
	m.explainTreeDescInflight = nil
	m.explainFlatLevel = explainLevel{fields: m.explainFields, cursor: m.explainCursor, scroll: m.explainScroll}
	m.explainTree = true
	m.explainTreeAll = msg.fields
	m.explainTreeCollapsed = nil
	m.explainCursor = 0
	m.explainScroll = 0
	m.applyExplainTreeVisible(selPath)
	return m, nil
}

// seedExplainDescriptions copies non-empty descriptions from src onto the
// dst fields with the same schema path.
func seedExplainDescriptions(dst, src []model.ExplainField) {
	descByPath := make(map[string]string, len(src))
	for _, f := range src {
		if f.Description != "" {
			descByPath[f.Path] = f.Description
		}
	}
	if len(descByPath) == 0 {
		return
	}
	for i := range dst {
		if d, ok := descByPath[dst[i].Path]; ok {
			dst[i].Description = d
		}
	}
}

// updateExplainTreeDescLoaded merges an arrived description batch into the
// tree rows under msg.parent (both the full tree and the visible list) and
// marks the level described. Failed fetches mark the level described too —
// retrying on every cursor move would spawn a kubectl process per keypress;
// the cache resets with the next tree load.
func (m Model) updateExplainTreeDescLoaded(msg explainTreeDescMsg) tea.Model {
	delete(m.explainTreeDescInflight, msg.parent)
	if !m.explainTree || msg.resource != m.explainResource ||
		msg.apiVersion != m.explainAPIVersion || msg.kctx != m.effectiveContext() {
		return m
	}
	if m.explainTreeDescFetched == nil {
		m.explainTreeDescFetched = make(map[string]struct{})
	}
	m.explainTreeDescFetched[msg.parent] = struct{}{}
	if msg.err != nil || len(msg.fields) == 0 {
		return m
	}
	seedExplainDescriptions(m.explainTreeAll, msg.fields)
	seedExplainDescriptions(m.explainFields, msg.fields)
	return m
}

// maybeFetchExplainTreeDesc returns a command fetching the descriptions for
// the cursor row's schema level when tree mode hasn't loaded them yet, and
// remembers the fetch as in flight. Nil outside tree mode or when the level
// is already described / loading.
func (m *Model) maybeFetchExplainTreeDesc() tea.Cmd {
	if !m.explainTree || m.explainCursor < 0 || m.explainCursor >= len(m.explainFields) {
		return nil
	}
	parent := explainParentPath(m.explainFields[m.explainCursor].Path)
	if _, ok := m.explainTreeDescFetched[parent]; ok {
		return nil
	}
	if _, ok := m.explainTreeDescInflight[parent]; ok {
		return nil
	}
	if m.explainTreeDescInflight == nil {
		m.explainTreeDescInflight = make(map[string]struct{})
	}
	m.explainTreeDescInflight[parent] = struct{}{}
	return m.execKubectlExplainTreeDesc(m.explainResource, m.explainAPIVersion, parent)
}

// withExplainTreeDescFetch chains a tree-description fetch for the cursor
// row's level onto a key handler's result when one is needed.
func withExplainTreeDescFetch(mdl tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m, ok := mdl.(Model)
	if !ok {
		return mdl, cmd
	}
	fetch := m.maybeFetchExplainTreeDesc()
	switch {
	case fetch == nil:
		return m, cmd
	case cmd == nil:
		return m, fetch
	default:
		return m, tea.Batch(cmd, fetch)
	}
}

// explainParentPath returns the schema path one level above path ("" at the
// resource root).
func explainParentPath(path string) string {
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		return path[:idx]
	}
	return ""
}

// applyExplainTreeVisible recomputes the visible tree (explainFields +
// explainTreeDepths) from the full tree minus folded subtrees, optionally
// placing the cursor on keepPath.
func (m *Model) applyExplainTreeVisible(keepPath string) {
	all := m.explainTreeAll
	depthsAll := explainTreeDepths(all, m.explainPath)
	fields := make([]model.ExplainField, 0, len(all))
	depths := make([]int, 0, len(all))
	skipDepth := -1
	for i, f := range all {
		d := depthsAll[i]
		if skipDepth >= 0 {
			if d > skipDepth {
				continue
			}
			skipDepth = -1
		}
		fields = append(fields, f)
		depths = append(depths, d)
		if _, collapsed := m.explainTreeCollapsed[f.Path]; collapsed {
			skipDepth = d
		}
	}
	m.explainFields = fields
	m.explainTreeDepths = depths
	if keepPath != "" {
		for i, f := range fields {
			if f.Path == keepPath {
				m.explainCursor = i
				break
			}
		}
	}
	m.explainCursor = max(min(m.explainCursor, len(fields)-1), 0)
	m.clampExplainScroll()
}

// toggleExplainTreeFold folds/unfolds the field subtree under the cursor.
// A no-op outside tree mode and on fields without child rows.
func (m Model) toggleExplainTreeFold() (tea.Model, tea.Cmd) {
	if !m.explainTree || m.explainCursor < 0 || m.explainCursor >= len(m.explainFields) {
		return m, nil
	}
	f := m.explainFields[m.explainCursor]
	if !explainTreeHasChildren(m.explainTreeAll, f.Path) {
		return m, nil
	}
	if m.explainTreeCollapsed == nil {
		m.explainTreeCollapsed = make(map[string]struct{})
	}
	if _, collapsed := m.explainTreeCollapsed[f.Path]; collapsed {
		delete(m.explainTreeCollapsed, f.Path)
	} else {
		m.explainTreeCollapsed[f.Path] = struct{}{}
	}
	m.applyExplainTreeVisible(f.Path)
	return m, nil
}

// explainTreeHasChildren reports whether any field in the full tree nests
// under path.
func explainTreeHasChildren(all []model.ExplainField, path string) bool {
	prefix := path + "."
	for _, f := range all {
		if strings.HasPrefix(f.Path, prefix) {
			return true
		}
	}
	return false
}

// explainTreeDepths computes each field's nesting depth relative to the tree
// root at basePath, from the dot count of its absolute schema path. Kubernetes
// schema field names never contain dots, so dot count equals nesting depth.
func explainTreeDepths(fields []model.ExplainField, basePath string) []int {
	baseDots := 0
	if basePath != "" {
		baseDots = strings.Count(basePath, ".") + 1
	}
	depths := make([]int, len(fields))
	for i, f := range fields {
		depths[i] = max(strings.Count(f.Path, ".")-baseDots, 0)
	}
	return depths
}

// viewExplainTree renders the tree-mode variant of the API Explorer.
func (m Model) viewExplainTree(title, hint, searchQuery string) string {
	parentFields, parentCursor := m.explainParentLevel()
	folded := make([]bool, len(m.explainFields))
	for i, f := range m.explainFields {
		_, folded[i] = m.explainTreeCollapsed[f.Path]
	}
	return ui.RenderExplainTreeView(
		m.explainFields,
		m.explainTreeDepths,
		folded,
		m.explainCursor,
		m.explainScroll,
		m.explainDesc,
		title,
		parentFields,
		parentCursor,
		searchQuery,
		hint,
		m.width,
		m.height,
	)
}
