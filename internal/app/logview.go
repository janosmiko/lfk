package app

import (
	"context"

	"github.com/janosmiko/lfk/internal/model"
)

// logViewState holds all state for the inline log viewer. It groups the
// previously flat log* fields on Model into one cohesive value, mirroring
// yamlViewState / diffViewState / describeViewState.
//
// The log viewer is the most heavily per-tab-persisted view: TabState
// round-trips most of these fields via saveCurrentTab/loadTab (see tabs.go).
// The reference-typed fields (ch, cancel, historyCancel, searchHistory) are
// shared by value across a tab snapshot, matching the pre-extraction
// behaviour; the streaming channel and cancel funcs are live handles, not
// values to deep-copy.
type logViewState struct {
	lines          []string           // buffered log lines (filtered projection of rawLines)
	rawLines       []string           // full unfiltered stream (source of lines)
	rawSev         []int              // per-rawLine severity rank; rawSev[i] is for rawLines[i]; populated when sevThreshold>0
	scroll         int                // scroll offset (top visible source line)
	wrapTopSkip    int                // wrap mode: number of sub-lines to skip from the top of lines[scroll]
	follow         bool               // auto-scroll to bottom
	wrap           bool               // wrap long lines
	lineNumbers    bool               // show line numbers
	timestamps     bool               // show timestamps (--timestamps)
	hidePrefixes   bool               // hide [pod/name/container] prefixes
	previewVisible bool               // show structured preview side panel
	previewScroll  int                // body-row offset within the preview pane (J/K)
	previous       bool               // show previous container logs (--previous)
	isMulti        bool               // multi-log stream (for restart)
	multiItems     []model.Item       // items for multi-log restart
	title          string             // title for the log overlay
	cancel         context.CancelFunc // cancel the kubectl log process
	ch             chan string        // channel for streaming log lines
	tailLines      int                // current --tail value for the active stream
	hasMoreHistory bool               // true if older lines may exist
	loadingHistory bool               // true while fetching older logs
	historyCancel  context.CancelFunc // cancel for the history fetch
	cursor         int                // cursor position (absolute line index), -1 when inactive
	visualMode     bool               // true when in visual line selection mode
	visualStart    int                // anchor line where visual selection started
	visualType     rune               // 'V' = line, 'v' = char, 'B' = block
	visualCol      int                // character column of anchor (for char and block modes)
	visualCurCol   int                // current cursor column (for char and block modes)
	scrollOption   int                // sticky vim 'scroll' option for [count]<C-d>/<C-u>; 0 = default (half viewport)

	// Parent resource context for pod re-selection.
	parentKind   string // original parent resource kind (e.g., "Deployment")
	parentName   string // original parent resource name
	savedPodName string // saved pod name before overlay, for restoring on cancel

	// Auto-reconnect for multi-container Pods.
	autoReconnectAttempt int
	reconnecting         bool

	// Container filter state.
	containers         []string // available container names for current pod
	selectedContainers []string // which containers are currently selected (empty = all)

	// Pod selector filter state.
	podFilterText   string
	podFilterActive bool

	// Container selector filter state.
	containerFilterText        string
	containerFilterActive      bool
	containerSelectionModified bool

	// Jump to line (digits + G).
	lineInput string

	// Filter state (live text filter + min-severity threshold).
	filterActive bool      // true while the live text-filter input is open
	filterInput  TextInput // text input buffer for the live filter
	filterQuery  string    // applied text filter (live)
	sevThreshold int       // 0=off, else minimum ui.Sev* rank

	// Search state.
	searchActive  bool
	searchInput   TextInput
	searchQuery   string // applied search
	searchHistory *commandHistory
}

// copy returns a copy with the value-typed slices (lines, multiItems,
// containers, selectedContainers) cloned so a TabState snapshot never aliases
// the live viewer's backing arrays. The streaming channel, cancel funcs, and
// searchHistory pointer are shared by assignment, matching the pre-extraction
// save/restore semantics (they are live handles, not snapshot values).
func (s logViewState) copy() logViewState {
	cp := s
	if s.lines != nil {
		cp.lines = append([]string(nil), s.lines...)
	}
	if s.rawLines != nil {
		cp.rawLines = append([]string(nil), s.rawLines...)
	}
	if s.rawSev != nil {
		cp.rawSev = append([]int(nil), s.rawSev...)
	}
	if s.multiItems != nil {
		cp.multiItems = append([]model.Item(nil), s.multiItems...)
	}
	if s.containers != nil {
		cp.containers = append([]string(nil), s.containers...)
	}
	if s.selectedContainers != nil {
		cp.selectedContainers = append([]string(nil), s.selectedContainers...)
	}
	return cp
}
