package app

import (
	"maps"
	"slices"
)

// yamlViewState holds all state for the full-screen YAML viewer (the `y`
// view). It groups the previously flat yaml* fields on Model into one
// cohesive value, mirroring the sub-state pattern already used by
// canIState, whoCanState, syncWaveState, and friends.
//
// Ownership is single-feature: these fields are read and written almost
// entirely within update_yaml.go, update_yaml_visual.go, view_yaml.go, and
// view_right.go. Only the persisted subset is round-tripped per tab via
// TabState (see tabs.go); the remaining fields are transient and reset on
// view entry.
type yamlViewState struct {
	content      string    // rendered body shown in the viewer (block YAML, or KYAML when kyaml is on)
	source       string    // block YAML as loaded; the input every KYAML re-render converts from
	kyaml        bool      // render the document as KYAML instead of block YAML
	kyamlReq     uint64    // numbers each conversion so a superseded reply is dropped
	scroll       int       // top visible line
	cursor       int       // cursor line in visible-line space
	scrollOption int       // sticky vim 'scroll' option for [count]<C-d>/<C-u>; 0 = default (half viewport)
	lineInput    string    // digit buffer for 123G jump-to-line
	searchMode   bool      // true when typing in the search bar
	searchText   TextInput // current search query
	matchLines   []int     // line indices matching the search
	matchIdx     int       // current match index in matchLines

	// Visual selection.
	visualMode   bool // true when in visual line selection mode
	visualStart  int  // anchor line (visible-line index) where visual selection started
	visualType   rune // 'V' = line, 'v' = char, 'B' = block
	visualCol    int  // character column of anchor (for char and block modes)
	visualCurCol int  // current cursor column (for char and block modes)

	wrap bool // word-wrap toggle

	// Field-manager blame, shown as a trailing note on the cursor line.
	// blame has one entry per original content line and is rebuilt whenever
	// the content or the owners change.
	// blameReq numbers each fetch. Two toggles over one document produce two
	// replies the content hash cannot tell apart, so only the newest number
	// is accepted.
	blameOn      bool
	blameLoading bool
	blameReq     uint64
	blame        []blameLine

	sections  []yamlSection   // parsed hierarchical sections
	collapsed map[string]bool // collapsed state per section key (persists across resources)

	// visCache memoizes buildVisibleLines across the render and the
	// per-keystroke handlers that need the same slice.
	visCache *yamlVisibleLinesCache
}

// resetBlame turns the field-manager note off. Blame is per resource and
// costs a fetch, so opening the viewer on something else starts without it.
func (s *yamlViewState) resetBlame() {
	s.blameOn = false
	s.blameLoading = false
	s.blame = nil
}

// yamlVisibleLinesCache memoizes buildVisibleLines, checked against its
// inputs rather than a generation counter: collapsed mutates in place at
// several call sites, and a counter is one missed bump from serving stale.
type yamlVisibleLinesCache struct {
	content   string
	sections  []yamlSection
	collapsed map[string]bool
	visLines  []string
	mapping   []int
	valid     bool
}

// resolve returns buildVisibleLines(content, sections, collapsed), reusing
// the previous result when all three inputs are unchanged. Nil-safe: a
// zero-value holder resolves correctly, just without memoization.
func (c *yamlVisibleLinesCache) resolve(content string, sections []yamlSection, collapsed map[string]bool) ([]string, []int) {
	if c != nil && c.valid && c.content == content &&
		slices.Equal(c.sections, sections) && maps.Equal(c.collapsed, collapsed) {
		return c.visLines, c.mapping
	}
	visLines, mapping := buildVisibleLines(content, sections, collapsed)
	if c == nil {
		return visLines, mapping
	}
	c.content = content
	c.sections = sections
	// Cloned, not aliased: collapsed mutates in place, an aliased map could
	// never report as changed.
	c.collapsed = maps.Clone(collapsed)
	c.visLines = visLines
	c.mapping = mapping
	c.valid = true
	return visLines, mapping
}

func (m Model) yamlVisibleLines() ([]string, []int) {
	return m.yamlView.visCache.resolve(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
}
