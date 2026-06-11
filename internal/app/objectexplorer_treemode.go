package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// toggleObjectExplorerTree flips the Object Explorer between the flat
// current-level list and the expanded tree view. The cursor follows the
// selection across the switch: flat key -> its tree row, tree row -> its
// top-level ancestor.
func (m Model) toggleObjectExplorerTree() (tea.Model, tea.Cmd) {
	rt := &m.objectExplorerView
	if rt.tree {
		var topKey string
		if rows := rt.visibleTreeRows(); rt.cursor >= 0 && rt.cursor < len(rows) {
			topKey = rows[rt.cursor].Segs[0]
		}
		m.objectExplorerTree = false // session preference for the next open
		rt.tree = false
		rt.treeRows = nil
		rt.treeCollapsed = nil
		rt.cursor = 0
		rt.scroll = 0
		rt.previewScroll = 0
		if topKey != "" {
			for i, f := range rt.visible() {
				if f.Key == topKey {
					rt.cursor = i
					break
				}
			}
		}
		m.clampObjectExplorerScroll()
		return m, nil
	}

	var flatKey string
	if f, ok := rt.selected(); ok {
		flatKey = f.Key
	}
	m.objectExplorerTree = true // session preference for the next open
	rt.tree = true
	rt.rebuildTreeRows()
	rt.cursor = 0
	rt.scroll = 0
	rt.previewScroll = 0
	if flatKey != "" {
		rt.cursorOnTreeSegs([]string{flatKey})
	}
	m.clampObjectExplorerScroll()
	if len(rt.treeRows) >= model.DefaultObjectTreeRowLimit {
		m.setStatusMessage("Tree truncated: drill in to see deeper levels", true)
		return m, scheduleStatusClear()
	}
	return m, nil
}

// rebuildTreeRows re-flattens the subtree at the current path. Call after any
// change to root or path while tree mode is on.
func (rt *objectExplorerState) rebuildTreeRows() {
	rt.treeRows = model.ObjectTreeRowsAt(rt.root, rt.path, 0)
}

// visibleTreeRows returns the tree rows shown after applying folds and the
// in-level filter. The filter searches the whole subtree, so it ignores
// folds (a match inside a collapsed branch still surfaces).
func (rt *objectExplorerState) visibleTreeRows() []model.ObjectTreeRow {
	if rt.filter == "" {
		if len(rt.treeCollapsed) == 0 {
			return rt.treeRows
		}
		out := make([]model.ObjectTreeRow, 0, len(rt.treeRows))
		skipDepth := -1
		for _, r := range rt.treeRows {
			if skipDepth >= 0 {
				if r.Depth > skipDepth {
					continue
				}
				skipDepth = -1
			}
			out = append(out, r)
			if _, collapsed := rt.treeCollapsed[segsKey(r.Segs)]; collapsed {
				skipDepth = r.Depth
			}
		}
		return out
	}
	q := strings.ToLower(rt.filter)
	out := make([]model.ObjectTreeRow, 0, len(rt.treeRows))
	for _, r := range rt.treeRows {
		if strings.Contains(strings.ToLower(r.Field.Key), q) {
			out = append(out, r)
		}
	}
	return out
}

// toggleObjectExplorerTreeFold folds/unfolds the subtree under the cursor.
// A no-op outside tree mode, while filtering, and on leaf rows.
func (m Model) toggleObjectExplorerTreeFold() (tea.Model, tea.Cmd) {
	rt := &m.objectExplorerView
	if !rt.tree || rt.filter != "" {
		return m, nil
	}
	row, ok := rt.selectedTreeRow()
	if !ok || !row.Field.HasChildren {
		return m, nil
	}
	k := segsKey(row.Segs)
	if rt.treeCollapsed == nil {
		rt.treeCollapsed = make(map[string]struct{})
	}
	if _, collapsed := rt.treeCollapsed[k]; collapsed {
		delete(rt.treeCollapsed, k)
	} else {
		rt.treeCollapsed[k] = struct{}{}
	}
	// Folding only removes rows after the cursor, so the cursor index is
	// unchanged; just keep the scroll window valid.
	m.clampObjectExplorerScroll()
	return m, nil
}

// segsKey builds a map key from path segments. Segments are joined with NUL,
// which cannot appear in YAML map keys, so distinct paths never collide.
func segsKey(segs []string) string {
	return strings.Join(segs, "\x00")
}

// cursorOnTreeSegs places the cursor on the visible row whose relative path
// equals segs; the cursor is left unchanged when no row matches.
func (rt *objectExplorerState) cursorOnTreeSegs(segs []string) {
	for i, r := range rt.visibleTreeRows() {
		if slicesEqual(r.Segs, segs) {
			rt.cursor = i
			return
		}
	}
}

// selectedTreeRow returns the tree row under the cursor in tree mode.
func (rt *objectExplorerState) selectedTreeRow() (model.ObjectTreeRow, bool) {
	rows := rt.visibleTreeRows()
	if rt.cursor < 0 || rt.cursor >= len(rows) {
		return model.ObjectTreeRow{}, false
	}
	return rows[rt.cursor], true
}

// slicesEqual reports whether two string slices are element-wise equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// viewObjectExplorerTree renders the tree-mode variant of the Object Explorer.
func (m Model) viewObjectExplorerTree(title, hint string) string {
	rt := m.objectExplorerView
	parentFields, parentCursor := m.objectExplorerParentLevel()
	return ui.RenderObjectExplorerTreeView(
		rt.visibleTreeRows(),
		rt.filter != "",
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
	)
}
