package app

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
	content      string    // rendered YAML body shown in the viewer
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
	blameOn      bool
	blameLoading bool
	blame        []blameLine

	sections  []yamlSection   // parsed hierarchical sections
	collapsed map[string]bool // collapsed state per section key (persists across resources)
}

// resetBlame turns the field-manager note off. Blame is per resource and
// costs a fetch, so opening the viewer on something else starts without it.
func (s *yamlViewState) resetBlame() {
	s.blameOn = false
	s.blameLoading = false
	s.blame = nil
}

// copy returns a deep copy: slices and the map are cloned so a value stored
// in a TabState never aliases the live viewer's backing arrays. Scalars,
// strings, and the TextInput value are copied by assignment.
func (s yamlViewState) copy() yamlViewState {
	cp := s
	if s.matchLines != nil {
		cp.matchLines = append([]int(nil), s.matchLines...)
	}
	if s.sections != nil {
		cp.sections = append([]yamlSection(nil), s.sections...)
	}
	if s.blame != nil {
		cp.blame = append([]blameLine(nil), s.blame...)
	}
	cp.collapsed = copyMapStringBool(s.collapsed)
	return cp
}
