package app

import "github.com/janosmiko/lfk/internal/ui"

// diffViewState holds all state for the full-screen diff viewer (the resource
// compare / revision-diff view). It groups the previously flat diff* fields on
// Model into one cohesive value, mirroring yamlViewState and the other
// sub-state structs (canIState, whoCanState, syncWaveState, ...).
//
// Ownership is single-feature: these fields are read and written almost
// entirely within update_diff.go and view_modes.go. Only the persisted subset
// (left, right, leftName, rightName, scroll, unified) is round-tripped per tab
// via TabState (see tabs.go); the remaining fields are transient and reset on
// view entry.
type diffViewState struct {
	left      string // YAML content of first resource
	right     string // YAML content of second resource
	leftName  string // name of first resource
	rightName string // name of second resource

	scroll      int    // scroll position in diff view
	cursor      int    // cursor line in visible-line space
	cursorSide  int    // 0=left, 1=right (side-by-side only)
	unified     bool   // true = unified diff, false = side-by-side
	wrap        bool   // word wrap toggle for diff view
	lineNumbers bool   // show line numbers in diff view
	lineInput   string // digit accumulator for jump-to-line (digits + G)

	searchMode  bool // true when typing in the search bar
	searchText  TextInput
	searchQuery string // committed search query
	matchLines  []int  // diff line indices with matches
	matchIdx    int    // current match index in matchLines

	foldState []bool // per-unchanged-region collapsed state

	// diffCache memoizes the LCS pass over left/right. A POINTER, and created
	// once in app_init: View() runs on a throwaway Model copy (renderView), so
	// a value cache — or one lazily created there — would be thrown away with
	// it and never hit. Derived, not state: keyed on the content it was built
	// from, so a copy that shares it can only ever get its own diff back.
	diffCache *ui.DiffCache

	visualMode   bool // true when in visual selection mode
	visualType   rune // 'V' = line, 'v' = char, 'B' = block
	visualStart  int  // anchor line (visible-line index)
	visualCol    int  // anchor column
	visualCurCol int  // current cursor column

	scrollOption int // sticky vim 'scroll' option for [count]<C-d>/<C-u>; 0 = default (half viewport)
}
