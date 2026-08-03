package app

import tea "charm.land/bubbletea/v2"

// describeViewState holds all state for the full-screen describe viewer (the
// `kubectl describe`-style output view). It groups the previously flat
// describe* fields on Model into one cohesive value, mirroring yamlViewState /
// diffViewState and the other sub-state structs.
//
// Ownership is single-feature: these fields are read and written almost
// entirely within update_describe.go and view_modes.go. Only the persisted
// subset (content, scroll, title) is round-tripped per tab via TabState (see
// tabs.go); the remaining fields are transient and reset on view entry.
//
// All fields are value-typed (no slices or maps), so a plain assignment is a
// complete copy — no copy() helper is needed, unlike yamlViewState /
// diffViewState. refreshFunc is a closure shared by value, which is the
// intended behaviour (it is not persisted across tabs).
type describeViewState struct {
	content     string
	scroll      int
	title       string
	wrap        bool           // word wrap toggle for describe view
	autoRefresh bool           // when true, describe viewer auto-refreshes every 2s
	refreshFunc func() tea.Cmd // returns the load command for auto-refresh
	lineInput   string         // digit buffer for 123G jump-to-line
	cursor      int            // cursor line position
	cursorCol   int            // cursor column position

	visualMode   byte // 0=off, 'v'=char, 'V'=line, 'B'=block
	visualStart  int  // anchor line for visual selection
	visualCol    int  // anchor column for visual mode
	scrollOption int  // sticky vim 'scroll' option for [count]<C-d>/<C-u>; 0 = default (half viewport)

	searchActive bool
	searchInput  TextInput
	searchQuery  string
}
