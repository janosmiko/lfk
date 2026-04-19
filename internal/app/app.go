package app

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hinshun/vt10x"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// viewMode tracks the current view state.
type viewMode int

const (
	modeExplorer viewMode = iota
	modeYAML
	modeHelp
	modeLogs
	modeDescribe
	modeDiff
	modeExec
	modeExplain
	modeEventViewer
	modeKubetris
	modeCredits
)

// overlayKind tracks which overlay is currently open.
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayNamespace
	overlayAction
	overlayConfirm     // y/n confirmation (regular delete, drain)
	overlayConfirmType // requires typing "DELETE" to confirm (force delete, force finalize)
	overlayScaleInput
	overlayPortForward
	overlayContainerSelect
	overlayPodSelect
	overlayBookmarks
	overlayTemplates
	overlaySecretEditor
	overlayConfigMapEditor
	overlayRollback
	overlayLabelEditor
	overlayHelmRollback
	overlayHelmHistory
	overlayColorscheme
	overlayFilterPreset
	overlayRBAC
	overlayBatchLabel
	overlayPodStartup
	overlayQuotaDashboard
	overlayEventTimeline
	overlayAlerts
	overlayNetworkPolicy
	overlayCanISubject
	overlayCanI
	overlayExplainSearch
	overlayLogPodSelect
	overlayLogContainerSelect
	overlayQuitConfirm
	overlayPVCResize
	overlayAutoSync
	overlayFinalizerSearch
	overlayColumnToggle
	overlayPasteConfirm // y/n confirmation for multiline paste into search/filter
	overlayBackgroundTasks
	overlayLogFilter
	overlayLogTimeRange
)

// bookmarkOverlayMode tracks the interaction mode for the bookmark overlay.
type bookmarkOverlayMode int

const (
	bookmarkModeNormal bookmarkOverlayMode = iota
	bookmarkModeFilter
	bookmarkModeConfirmDelete
	bookmarkModeConfirmDeleteAll
)

// sortColDefault is the default sort column name.
const sortColDefault = "Name"

// sortColEventLastSeen is a sentinel used internally by sortMiddleItems
// to override the default "Name" sort for Events. It is NOT a user-visible
// column name and must not appear in the sortable-column cycle.
const sortColEventLastSeen = "__event_last_seen__"

// sortColumnIndex returns the index of sortColumnName in ActiveSortableColumns,
// or 0 if not found.
func sortColumnIndex(name string) int {
	for i, col := range ui.ActiveSortableColumns {
		if col == name {
			return i
		}
	}
	return 0
}

// actionContext stores which resource an action targets.
type actionContext struct {
	kind          string // Kubernetes Kind (e.g., "Pod", "Deployment")
	name          string // resource name
	namespace     string // namespace of the target resource (captured at action time)
	context       string // kubeconfig context name (captured at action time)
	containerName string // container name (for exec/logs at container level)
	image         string // container image (for vuln scan at container level)
	resourceType  model.ResourceTypeEntry
	columns       []model.KeyValue // additional item columns (e.g., Node, IP) for custom action templates
}

// TabState holds per-tab navigation state so each tab is fully independent.
type TabState struct {
	// needsLoad is true for tabs restored from a session file that have not
	// yet had their items loaded.  When loadTab detects this flag it triggers
	// a full refreshCurrentLevel instead of the lighter loadPreview.
	needsLoad bool

	nav                    model.NavigationState
	leftItems              []model.Item
	middleItems            []model.Item
	rightItems             []model.Item
	leftItemsHistory       [][]model.Item
	cursors                [5]int
	middleScroll           int // persistent scroll position for middle column (vim-style scrolloff)
	leftScroll             int // persistent scroll position for left column (vim-style scrolloff)
	cursorMemory           map[string]int
	itemCache              map[string][]model.Item
	yamlContent            string
	yamlScroll             int
	yamlCursor             int // cursor position in visible lines (relative to scroll)
	yamlSearchText         TextInput
	yamlMatchLines         []int
	yamlMatchIdx           int
	yamlCollapsed          map[string]bool // collapsed state for YAML sections
	splitPreview           bool
	fullYAMLPreview        bool
	previewYAML            string
	namespace              string
	allNamespaces          bool
	selectedNamespaces     map[string]bool
	sortColumnName         string // column name to sort by (e.g. "Name", "Age", "CPU")
	sortAscending          bool
	filterText             string
	watchMode              bool
	requestGen             uint64
	selectedItems          map[string]bool
	selectionAnchor        int // anchor index for region selection (-1 = unset)
	fullscreenMiddle       bool
	fullscreenDashboard    bool
	dashboardPreview       string
	dashboardEventsPreview string // warning events for two-column dashboard
	monitoringPreview      string

	// Toggle to show only Warning events in Event list view.
	warningEventsOnly bool

	// Collapse duplicate Events (same Type/Reason/Message/Object) into a
	// single row with a summed Count column. Grouped-by-default reduces
	// noise when many pods hit the same failure mode at once.
	eventGrouping bool

	// Collapsible tree view state for resource types.
	expandedGroup     string // currently expanded category (accordion behavior)
	allGroupsExpanded bool   // override: show all groups expanded (toggled by hotkey)

	// Per-tab view mode and fullscreen state.
	mode                  viewMode
	logLines              []string
	logScroll             int
	logFollow             bool
	logWrap               bool
	logLineNumbers        bool
	logTimestamps         bool
	logRelativeTimestamps bool
	logPrevious           bool
	logIsMulti            bool
	logTitle              string
	logCancel             context.CancelFunc
	logCh                 chan string
	logTailLines          int  // current --tail value for the active stream
	logHasMoreHistory     bool // true if older lines may exist
	logLoadingHistory     bool // true while fetching older logs
	logCursor             int  // cursor position (absolute line index), -1 when inactive
	logVisualMode         bool // true when in visual line selection mode
	logVisualStart        int  // anchor line where visual selection started
	logVisualType         rune // 'V' = line, 'v' = char, 'B' = block
	logVisualCol          int  // character column of anchor (for char and block modes)
	logVisualCurCol       int  // character column where the cursor is now

	// Log viewer: active filter rules and mode. Persisted per tab so
	// switching tabs preserves the rule stack the user built up.
	// logFilterChain and logVisibleIndices are rebuilt from logRules on
	// tab load; no need to snapshot them.
	logRules       []Rule
	logIncludeMode IncludeMode

	// Log viewer: JSON pretty-print mode. When true, JSON log lines
	// render expanded with 2-space indentation; non-JSON lines are
	// unchanged. Persisted per tab so switching tabs preserves the
	// display preference.
	logJSONPretty bool

	// Log viewer: histogram strip toggle. When true, a 1-row sparkline
	// renders between the title bar and the content area showing
	// log-line density over time (bucket counts as box-drawing block
	// characters). Default OFF. Persisted per tab so switching tabs
	// preserves the visibility preference.
	logHistogram bool

	// Log viewer: active --since window (user-typed string, e.g. "5m").
	// Empty means no --since filter.  Persisted per tab so switching
	// tabs restores the setting; the stream is only restarted when the
	// user commits via the overlay, not on tab switch.
	//
	// Retained for one release so pre-migration session files load
	// cleanly; code paths should consult logTimeRange instead.
	logSinceDuration string

	// Log viewer: active time-range window. Replaces logSinceDuration —
	// a LogTimeRange carries both a start and an optional end endpoint,
	// each either relative (offset from now) or absolute (wall-clock).
	// Persisted per tab. When this is set, kubectlSinceArg is derived
	// from r.KubectlSinceArg(now) and the End filter is applied
	// client-side in updateLogLine / updateLogHistory.
	logTimeRange LogTimeRange

	// Log viewer: parent resource context for pod re-selection.
	logParentKind   string
	logParentName   string
	logSavedPodName string // saved pod name before overlay, for restoring on cancel

	// Log viewer: container filter state.
	logContainers         []string // available container names for current pod
	logSelectedContainers []string // which containers are currently selected (empty = all)

	// Describe viewer state (per-tab).
	describeContent string
	describeScroll  int
	describeTitle   string

	// Diff viewer state (per-tab).
	diffLeft      string
	diffRight     string
	diffLeftName  string
	diffRightName string
	diffScroll    int
	diffUnified   bool

	// Exec PTY state (per-tab).
	execPTY   *os.File
	execTerm  vt10x.Terminal
	execTitle string
	execDone  *atomic.Bool
	execMu    *sync.Mutex

	// Explain view state (per-tab).
	explainFields      []model.ExplainField
	explainDesc        string // resource/field-level description
	explainPath        string // current drill-down path (e.g., "spec.template")
	explainResource    string // resource name (e.g., "deployments")
	explainAPIVersion  string // api version for kubectl explain (e.g., "apps/v1")
	explainTitle       string
	explainCursor      int
	explainScroll      int
	explainSearchQuery string // persisted search query for n/N navigation
}

// Model is the top-level bubbletea model.
type Model struct {
	client  *k8s.Client
	version string // application version string shown in the title bar

	// Navigation state.
	nav model.NavigationState

	// Column data.
	leftItems   []model.Item
	middleItems []model.Item
	rightItems  []model.Item

	// History stack for the left column: pushed on navigateChild, popped on navigateParent.
	leftItemsHistory [][]model.Item

	// Cursor positions per level so we restore them when going back.
	cursors [5]int // indexed by model.Level (0..4)

	// Cursor memory: maps navigation path to cursor position for back-and-forth navigation.
	cursorMemory map[string]int

	// Item cache: maps navigation path to loaded items for faster back navigation.
	itemCache map[string][]model.Item

	// Preview / YAML content for the right column or full screen view.
	yamlContent    string
	yamlScroll     int
	yamlCursor     int       // cursor line in visible-line space
	yamlLineInput  string    // digit buffer for 123G jump-to-line
	yamlSearchMode bool      // true when typing in the search bar
	yamlSearchText TextInput // current search query
	yamlMatchLines []int     // line indices matching the search
	yamlMatchIdx   int       // current match index in yamlMatchLines

	// Visual selection in YAML view.
	yamlVisualMode   bool // true when in visual line selection mode
	yamlVisualStart  int  // anchor line (visible-line index) where visual selection started
	yamlVisualType   rune // 'V' = line, 'v' = char, 'B' = block
	yamlVisualCol    int  // character column of anchor (for char and block modes)
	yamlVisualCurCol int  // current cursor column (for char and block modes)

	// Word wrap toggle for YAML view.
	yamlWrap bool

	// Collapsible YAML sections.
	yamlSections  []yamlSection   // parsed hierarchical sections
	yamlCollapsed map[string]bool // collapsed state per section key (persists across resources)

	// Split preview: show children in top 1/3 + YAML in bottom 2/3 of right column.
	splitPreview bool
	// Full YAML preview: show only YAML in the right column (no children list).
	fullYAMLPreview bool
	// Separate YAML content for the split/full preview in the right column,
	// so it doesn't conflict with the full-screen yamlContent.
	previewYAML string

	// Current view mode.
	mode viewMode

	// Overlay state.
	overlay       overlayKind
	overlayItems  []model.Item // full list (e.g., all namespaces)
	overlayFilter TextInput    // typed filter text
	overlayCursor int

	// Namespace (not a navigation level; displayed in top-right).
	namespace string

	// Terminal dimensions.
	width  int
	height int

	// Error to display.
	err error

	// Loading indicator.
	loading bool

	// previewLoading is set true when a preview load is in flight for the
	// right pane. It is independent from `loading` so that the right pane
	// can keep showing its spinner during the gap between the main list
	// load completing (which clears `loading`) and the preview load
	// completing. Without this the right pane briefly renders
	// "No resources found" between the two transitions.
	previewLoading bool

	// Spinner for loading animation.
	spinner spinner.Model

	// Action context: which resource/kind the action targets.
	actionCtx actionContext

	// Scale input state.
	scaleInput TextInput

	// PVC resize: current size displayed in the overlay.
	pvcCurrentSize string

	// Port forward input state.
	portForwardInput TextInput

	// Confirm action label (for delete confirmation).
	confirmAction string

	// Title and question for the type-to-confirm overlay.
	confirmTitle    string
	confirmQuestion string

	// Text input for type-to-confirm overlay (e.g., Force Finalize).
	confirmTypeInput TextInput

	// All-namespaces mode.
	allNamespaces bool

	// Multi-select namespace state.
	selectedNamespaces  map[string]bool
	nsFilterMode        bool
	nsSelectionModified bool // tracks if Space was pressed in current ns overlay session

	// Fullscreen middle column: hides left and right columns.
	fullscreenMiddle bool

	// Fullscreen dashboard: renders the cluster dashboard full screen.
	fullscreenDashboard bool

	// Sort state for resources: column name and direction.
	sortColumnName string // which column to sort by (e.g. "Name", "Age")
	sortAscending  bool   // true = ascending, false = descending

	// Status message (temporary, shown in status bar).
	statusMessage    string
	statusMessageErr bool
	statusMessageExp time.Time // when message expires
	statusMessageTip bool      // true when the message is a startup tip (dismiss on keypress)

	// Pending target: when set, after resources load, find and select this item by name.
	pendingTarget string

	// Vim-style 'gg' command: when true, the next 'g' press jumps to top.
	pendingG bool

	// Vim-style named marks: m<key> sets a mark, '<key> jumps to it.
	pendingMark     bool            // waiting for the slot key after 'm'
	pendingBookmark *model.Bookmark // bookmark awaiting overwrite confirmation

	// Watch mode: auto-refresh the current view on a timer.
	watchMode     bool
	watchInterval time.Duration

	// Help screen state.
	helpScroll       int
	helpFilter       TextInput
	helpSearchActive bool
	helpContextMode  string   // section to highlight (e.g. "YAML View", "Log Viewer")
	helpPreviousMode viewMode // mode to return to when help is closed
	helpSearchInput  textinput.Model

	// Resource filter state (/ key).
	filterText   string    // applied filter for middle column
	filterActive bool      // whether the filter input is being typed
	filterInput  TextInput // what user is currently typing

	// Search state (s key).
	searchActive     bool
	searchInput      TextInput
	searchPrevCursor int

	// Log viewer state.
	logLines              []string           // buffered log lines
	logScroll             int                // scroll offset (top visible line)
	logFollow             bool               // auto-scroll to bottom
	logWrap               bool               // wrap long lines
	logLineNumbers        bool               // show line numbers
	logTimestamps         bool               // show timestamps (--timestamps)
	logRelativeTimestamps bool               // render timestamps as "5m ago" when logTimestamps is true
	logJSONPretty         bool               // expand JSON log lines to pretty-printed multi-line form
	logHistogram          bool               // show 1-row time-density sparkline above the content area
	logHidePrefixes       bool               // hide [pod/name/container] prefixes
	logPrevious           bool               // show previous container logs (--previous)
	logIsMulti            bool               // multi-log stream (for restart)
	logMultiItems         []model.Item       // items for multi-log restart
	logTitle              string             // title for the log overlay
	logCancel             context.CancelFunc // cancel the kubectl log process
	logCh                 chan string        // channel for streaming log lines
	logTailLines          int                // current --tail value for the active stream
	logHasMoreHistory     bool               // true if older lines may exist
	logLoadingHistory     bool               // true while fetching older logs
	logHistoryCancel      context.CancelFunc // cancel for the history fetch
	logCursor             int                // cursor position (absolute line index), -1 when inactive
	logVisualMode         bool               // true when in visual line selection mode
	logVisualStart        int                // anchor line where visual selection started
	logVisualType         rune               // 'V' = line, 'v' = char, 'B' = block
	logVisualCol          int                // character column of anchor (for char and block modes)
	logVisualCurCol       int                // current cursor column (for char and block modes)

	// Log viewer: parent resource context for pod re-selection.
	logParentKind   string // original parent resource kind (e.g., "Deployment")
	logParentName   string // original parent resource name
	logSavedPodName string // saved pod name before overlay, for restoring on cancel

	// Log viewer: container filter state.
	logContainers         []string // available container names for current pod
	logSelectedContainers []string // which containers are currently selected (empty = all)

	// Log pod selector filter state.
	logPodFilterText   string
	logPodFilterActive bool

	// Log container selector filter state.
	logContainerFilterText        string
	logContainerFilterActive      bool
	logContainerSelectionModified bool

	// Log viewer: jump to line (digits + G).
	logLineInput string

	// Log viewer: search state.
	logSearchActive bool
	logSearchInput  TextInput
	logSearchQuery  string // applied search

	// Filter pipeline (Phase 1 of log preview overhaul).
	logRules          []Rule      //nolint:unused // wired in subsequent Phase C tasks
	logIncludeMode    IncludeMode //nolint:unused // wired in subsequent Phase C tasks
	logVisibleIndices []int       //nolint:unused // wired in subsequent Phase C tasks
	// Filter modal UI fields — populated in Phase D/E.
	logFilterModalOpen  bool      //nolint:unused // populated in Phase D/E
	logFilterInput      TextInput //nolint:unused // populated in Phase D/E
	logFilterListCursor int       //nolint:unused // populated in Phase D/E
	logFilterFocusInput bool      //nolint:unused // populated in Phase D/E
	logFilterEditingIdx int       //nolint:unused // -1 = adding new; >=0 = editing existing rule index
	logSavePresetPrompt bool      //nolint:unused // populated in Phase D/E
	logLoadPresetOpen   bool      //nolint:unused // populated in Phase D/E
	logLoadPresetCursor int       //nolint:unused // populated in Phase D/E

	// Log viewer: active --since window, displayed as a title-bar chip
	// and appended to the kubectl logs args when non-empty.  Empty
	// disables the filter.  Session-only — not persisted to disk.
	//
	// Retained for one release so pre-migration tab snapshots still
	// load — populated into logTimeRange on loadTab and never set
	// again by production code.
	logSinceDuration string

	// Log viewer: active time-range window (Phase 1+ replacement for
	// logSinceDuration). A zero value disables the filter. Session-only
	// — not persisted to disk beyond the tab-state snapshot.
	logTimeRange LogTimeRange

	// Log viewer: time-range picker state (overlayLogTimeRange).
	//
	// logTimeRangePresets is the list of presets shown in the left
	// column of the picker; populated when the overlay opens so
	// Today/Yesterday anchors use the correct "now". logTimeRangeCursor
	// indexes into it. logTimeRangePendingRange holds the range the
	// user has built up inside the picker but hasn't committed yet —
	// Enter from the preset column writes it to logTimeRange; Esc
	// discards it.
	logTimeRangePresets      []LogTimeRangePreset
	logTimeRangeCursor       int
	logTimeRangePendingRange LogTimeRange
	// logTimeRangeFocus selects which panel of the picker currently
	// accepts keyboard input: Presets (default), Start editor, End
	// editor. Phase 2/3 populate the editor focus values; Phase 1
	// keeps it pinned on Presets.
	logTimeRangeFocus logTimeRangeFocus
	// Start / End editor panels used in Phase 2+ for the Custom…
	// workflow. Wired into commitLogTimeRange so Enter from a Start
	// panel applies the current editor values and keeps the overlay
	// open on the End panel; Enter from the End panel commits the
	// full range.
	logTimeRangeStart logTimeRangeEditor
	logTimeRangeEnd   logTimeRangeEditor

	// Severity detector — initialized once at startup; reused for all log views.
	logSeverityDetector *severityDetector //nolint:unused // wired in subsequent Phase C tasks

	// Filter chain — rebuilt whenever logRules or logIncludeMode changes.
	logFilterChain *FilterChain //nolint:unused // wired in subsequent Phase C tasks

	// logJSONCache caches JSON-detection results for log lines. Keyed by
	// the raw line's content hash (fnv64a of the line string) so entries
	// survive slice reslicing of logLines but are automatically garbage-
	// collected when lines roll out of the buffer via the eviction policy.
	// Populated on the stream-append and history-prepend paths; read by
	// jsonLineAt for callers that need the parsed JSON value.
	logJSONCache *lruJSONCache

	// Pending bracket for two-keystroke jump-to-severity sequences (]e/[e/]w/[w).
	// Set to ']' or '[' on the first keystroke; cleared on the next key.
	logPendingBracket rune

	// Describe viewer state.
	describeContent      string
	describeScroll       int
	describeTitle        string
	describeWrap         bool           // word wrap toggle for describe view
	describeAutoRefresh  bool           // when true, describe viewer auto-refreshes every 2s
	describeRefreshFunc  func() tea.Cmd // returns the load command for auto-refresh
	describeLineInput    string         // digit buffer for 123G jump-to-line
	describeCursor       int            // cursor line position
	describeCursorCol    int            // cursor column position
	describeVisualMode   byte           // 0=off, 'v'=char, 'V'=line, 'B'=block
	describeVisualStart  int            // anchor line for visual selection
	describeVisualCol    int            // anchor column for visual mode
	describeSearchActive bool
	describeSearchInput  TextInput
	describeSearchQuery  string

	// Diff viewer state.
	diffLeft         string // YAML content of first resource
	diffRight        string // YAML content of second resource
	diffLeftName     string // name of first resource
	diffRightName    string // name of second resource
	diffScroll       int    // scroll position in diff view
	diffCursor       int    // cursor line in visible-line space
	diffCursorSide   int    // 0=left, 1=right (side-by-side only)
	diffUnified      bool   // true = unified diff, false = side-by-side
	diffWrap         bool   // word wrap toggle for diff view
	diffLineNumbers  bool   // show line numbers in diff view
	diffLineInput    string // digit accumulator for jump-to-line (digits + G)
	diffSearchMode   bool   // true when typing in the search bar
	diffSearchText   TextInput
	diffSearchQuery  string // committed search query
	diffMatchLines   []int  // diff line indices with matches
	diffMatchIdx     int    // current match index in diffMatchLines
	diffFoldState    []bool // per-unchanged-region collapsed state
	diffVisualMode   bool   // true when in visual selection mode
	diffVisualType   rune   // 'V' = line, 'v' = char, 'B' = block
	diffVisualStart  int    // anchor line (visible-line index)
	diffVisualCol    int    // anchor column
	diffVisualCurCol int    // current cursor column

	// Embedded terminal state (PTY mode).
	execPTY        *os.File       // PTY master file descriptor
	execTerm       vt10x.Terminal // Virtual terminal emulator
	execTitle      string         // Title for the exec session
	execDone       *atomic.Bool   // Process has exited (shared across copies)
	execMu         *sync.Mutex    // Protects execTerm access
	execEscPressed bool           // Ctrl+] prefix pressed, waiting for follow-up key

	// Multi-selection state: maps "namespace/name" keys to selected status.
	selectedItems   map[string]bool
	selectionAnchor int // anchor index for region selection (-1 = unset)

	// Bulk action mode flag: true when the current action applies to multiple items.
	bulkMode bool

	// Bulk action items: captured list of selected items for bulk operations.
	bulkItems []model.Item

	// Pending action waiting for container selection.
	pendingAction string
	pendingPaste  string      // multiline paste awaiting confirmation
	pasteTargetID pasteTarget // identifies which input to insert into after confirm

	// Request generation counter for stale response detection.
	// Incremented on every navigation change; async messages carry the gen
	// they were created with and are discarded if it no longer matches.
	requestGen uint64

	// Context cancellation for in-flight API requests. Cancelled on every
	// navigation change so stale requests are aborted early instead of
	// running to completion.
	reqCtx    context.Context
	reqCancel context.CancelFunc

	// Tab support.
	tabs      []TabState
	activeTab int

	// Bookmarks: saved navigation paths for quick access.
	bookmarks          []model.Bookmark
	bookmarkFilter     TextInput           // filter text (f mode) for bookmark overlay
	bookmarkSearchMode bookmarkOverlayMode // current interaction mode for bookmark overlay

	// Template overlay state.
	templateItems      []model.ResourceTemplate
	templateCursor     int
	templateFilter     TextInput // filter text for template overlay
	templateSearchMode bool      // true when typing in filter mode

	// Show decoded secret values in preview.
	showSecretValues bool

	// Toggle to show only Warning events in Event list view.
	warningEventsOnly bool

	// Collapse duplicate Events (per-tab mirror of Model.eventGrouping).
	eventGrouping bool

	// bgtasks tracks in-flight async loads (resource lists, YAML fetches,
	// metrics, dashboards). Process-global instance shared across tabs so
	// the title bar reflects all activity, not just the active tab's.
	bgtasks *bgtasks.Registry

	// suppressBgtasks, when true, makes loaders call Registry.StartUntracked
	// instead of Registry.Start so their tasks don't appear in the title-bar
	// indicator. Set by updateWatchTick before dispatching watch-mode
	// auto-refreshes — periodic refreshes shouldn't flash through the
	// indicator every 2 seconds.
	suppressBgtasks bool

	// tasksOverlayShowCompleted selects which view the :tasks overlay
	// renders when it's open. false (default) shows currently running
	// tasks with a live ELAPSED column; true shows the recent
	// completed-task history with a fixed DURATION column. Toggled with
	// Tab inside the overlay; reset to false every time the overlay is
	// opened fresh.
	tasksOverlayShowCompleted bool

	// tasksOverlayScroll is the first-visible-row index for the :tasks
	// overlay. Bumped by j/k (and friends) inside the overlay; reset on
	// open and on Tab mode switch. The renderer clamps this into a
	// valid range so the handler can bump it blindly.
	tasksOverlayScroll int

	// Discovered CRDs per context: keyed by context name.
	discoveredResources map[string][]model.ResourceTypeEntry

	// Preview scroll offset for the right column.
	previewScroll int

	// Metrics content: rendered bar graph for the preview column.
	metricsContent string

	// Preview events content: rendered event timeline for the preview column.
	previewEventsContent string

	// Baseline metrics for trend detection (updated every ~60s, not every refresh).
	prevPodMetrics      map[string]model.PodMetrics
	prevPodMetricsTime  time.Time
	prevNodeMetrics     map[string]model.PodMetrics
	prevNodeMetricsTime time.Time

	// Dashboard preview: rendered cluster dashboard for the right column.
	dashboardPreview string

	// Dashboard events preview: warning events for the two-column dashboard layout.
	dashboardEventsPreview string

	// Monitoring preview: rendered monitoring dashboard for the right column.
	monitoringPreview string

	// Collapsible tree view state for resource types.
	expandedGroup     string // currently expanded category (accordion behavior)
	allGroupsExpanded bool   // override: show all groups expanded (toggled by hotkey)
	showRareResources bool   // override: show rarely used resources and uncategorized core built-ins (H hotkey)

	// Error log: global buffer of application errors for the error log overlay.
	errorLog               []ui.ErrorLogEntry
	overlayErrorLog        bool
	errorLogScroll         int
	showDebugLogs          bool
	errorLogFullscreen     bool   // true = fullscreen, false = overlay
	errorLogVisualMode     byte   // 0 = off, 'v' = char, 'V' = line
	errorLogVisualStart    int    // anchor line index in visual mode
	errorLogVisualStartCol int    // anchor column when entering char visual mode
	errorLogCursorLine     int    // cursor position (line index into visible entries)
	errorLogCursorCol      int    // cursor column for character visual mode
	errorLogLineInput      string // digit buffer for 123G jump-to-line

	// Color scheme selector state.
	schemeEntries      []ui.SchemeEntry
	schemeCursor       int
	schemeFilter       TextInput
	schemeFilterMode   bool   // true when typing into filter
	schemeOriginalName string // scheme name before opening overlay, for cancel restore

	// Secret editor state.
	secretData         *model.SecretData
	secretDataOriginal map[string]string // snapshot taken at load time for dirty detection
	secretCursor       int
	secretRevealed     map[string]bool
	secretAllRevealed  bool
	secretEditing      bool
	secretEditKey      TextInput
	secretEditValue    TextInput
	secretEditColumn   int // 0=key, 1=value

	// ConfigMap editor state.
	configMapData         *model.ConfigMapData
	configMapDataOriginal map[string]string // snapshot taken at load time for dirty detection
	configMapCursor       int
	configMapEditing      bool
	configMapEditKey      TextInput
	configMapEditValue    TextInput
	configMapEditColumn   int // 0=key, 1=value

	// Rollback overlay state (deployments).
	rollbackRevisions []k8s.DeploymentRevision
	rollbackCursor    int

	// Helm rollback overlay state.
	helmRollbackRevisions []ui.HelmRevision
	helmRollbackCursor    int

	// Helm history (read-only) overlay state.
	helmHistoryRevisions []ui.HelmRevision
	helmHistoryCursor    int

	// helmRevisionsLoading is shared between the helm rollback and history
	// overlays. It is set to true when the helm history subprocess is
	// dispatched and cleared when the result (success or error) arrives so
	// the overlay can show a loading placeholder instead of flashing the
	// empty-state message.
	helmRevisionsLoading bool

	// Label/annotation editor state.
	labelData                *model.LabelAnnotationData
	labelLabelsOriginal      map[string]string // snapshot of labels at load time
	labelAnnotationsOriginal map[string]string // snapshot of annotations at load time
	labelCursor              int
	labelTab                 int // 0=labels, 1=annotations
	labelEditing             bool
	labelEditKey             TextInput
	labelEditValue           TextInput
	labelEditColumn          int                     // 0=key, 1=value
	labelResourceType        model.ResourceTypeEntry // the resource type being edited

	// ArgoCD autosync overlay state.
	autoSyncEnabled  bool
	autoSyncSelfHeal bool
	autoSyncPrune    bool
	autoSyncCursor   int // 0=autosync, 1=selfheal, 2=prune

	// Quick filter preset state.
	filterPresets         []FilterPreset
	activeFilterPreset    *FilterPreset // currently applied filter preset, nil if none
	unfilteredMiddleItems []model.Item  // full list before filter preset was applied

	// RBAC permission check state.
	rbacResults []k8s.RBACCheck
	rbacKind    string

	// Quota dashboard state.
	quotaData []k8s.QuotaInfo

	// Prometheus alerts overlay state.
	alertsData      []k8s.AlertInfo // alerts for current resource
	alertsScroll    int             // scroll position in alerts overlay
	alertsLineInput string          // digit buffer for 123G jump-to-line

	// Network policy visualizer state.
	netpolData      *k8s.NetworkPolicyInfo
	netpolScroll    int
	netpolLineInput string // digit buffer for 123G jump-to-line

	// Batch label/annotation editor state.
	batchLabelMode   int       // 0=labels, 1=annotations
	batchLabelInput  TextInput // "key=value" input
	batchLabelRemove bool      // true = remove mode, false = add mode

	// Pod startup analysis state.
	podStartupData *k8s.PodStartupInfo

	// Event timeline overlay state.
	eventTimelineData         []k8s.EventInfo // event timeline data
	eventTimelineLines        []string        // flat text lines for cursor navigation
	eventTimelineScroll       int             // scroll position
	eventTimelineLineInput    string          // digit buffer for 123G jump-to-line
	eventTimelineCursor       int             // cursor position (line index in rendered lines)
	eventTimelineWrap         bool            // word wrap toggle
	eventTimelineFullscreen   bool            // fullscreen mode
	eventTimelineVisualMode   byte            // 0=off, 'v'=char, 'V'=line, 'B'=block
	eventTimelineVisualStart  int             // anchor line for visual selection
	eventTimelineVisualCol    int             // anchor column for char visual mode
	eventTimelineCursorCol    int             // cursor column for char visual mode
	eventTimelineSearchActive bool
	eventTimelineSearchInput  TextInput
	eventTimelineSearchQuery  string

	// Command bar state.
	commandBarActive             bool
	commandBarInput              TextInput
	commandBarSuggestions        []ui.Suggestion
	commandBarSelectedSuggestion int
	commandBarPreview            string // ghost text shown dimmed after cursor (tab preview)
	commandHistory               *commandHistory

	// Cached namespace names for command bar autocompletion.
	cachedNamespaces []string

	// Async resource name cache for cross-namespace kubectl completion.
	// Key: "context/namespace/resource" -> list of resource names.
	commandBarNameCache   map[string][]string
	commandBarNameLoading string // cache key currently being fetched ("" if idle)

	// Stderr capture channel for exec credential plugin errors.
	stderrChan <-chan string

	// Resource map view: shows relationship tree in the right column.
	mapView      bool
	resourceTree *model.ResourceNode

	// Session persistence: restores navigation state across restarts.
	pendingSession      *SessionState      // loaded session waiting to be applied after contexts load
	sessionRestored     bool               // true once the pending session has been applied
	pendingPortForwards *PortForwardStates // loaded port forwards waiting to be re-established

	// Nested owned navigation: stack of parent states pushed when drilling
	// from LevelOwned into a child that itself has children (e.g., ArgoCD
	// Application → Deployment → Pods). Popped by navigateParent.
	ownedParentStack []ownedParentState

	// Per-context pinned CRD groups state.
	pinnedState *PinnedState

	// Port forward manager: tracks active kubectl port-forward processes.
	portForwardMgr *k8s.PortForwardManager

	// Port forward overlay state: discovered ports for the selected resource.
	pfAvailablePorts []ui.PortInfo
	pfPortCursor     int              // cursor in the available ports list (-1 = manual input)
	pfLastCreatedID  int              // ID of the most recently created port forward (for showing resolved port)
	pfLoggedErrors   map[int]struct{} // port forward IDs whose failures have been logged to errorLog

	// Explain view state (API browser).
	explainFields                []model.ExplainField
	explainDesc                  string // resource/field-level description
	explainPath                  string // current drill-down path (e.g., "spec.template")
	explainResource              string // resource name (e.g., "deployments")
	explainAPIVersion            string // api version for kubectl explain (e.g., "apps/v1")
	explainTitle                 string
	explainCursor                int
	explainScroll                int
	explainLineInput             string               // digit buffer for 123G jump-to-line
	explainSearchActive          bool                 // true when typing in search bar
	explainSearchInput           TextInput            // current search input
	explainSearchQuery           string               // persisted search query for n/N navigation
	explainSearchPrevCursor      int                  // cursor position before search started
	explainRecursiveResults      []model.ExplainField // results from recursive search
	explainRecursiveCursor       int
	explainRecursiveScroll       int
	explainRecursiveFilter       TextInput // filter input for recursive search overlay
	explainRecursiveFilterActive bool      // true when typing in filter

	// Can-I browser state.
	canIGroups            []model.CanIGroup
	canIGroupCursor       int // selected group in left column
	canIGroupScroll       int
	canIResourceScroll    int       // scroll offset for the resource column
	canISubject           string    // "" = current user, or "system:serviceaccount:ns:name"
	canISubjectName       string    // display name for the subject ("Current User" or "sa/name")
	canIServiceAccounts   []string  // cached SA list for the selector
	canISearchActive      bool      // true when typing in search bar
	canISearchInput       TextInput // current search input
	canISearchQuery       string    // confirmed search query for filtering
	canISubjectFilterMode bool      // true when typing in subject filter bar
	canIAllowedOnly       bool      // true = show only allowed permissions
	canINamespaces        []string  // namespaces used for SelfSubjectRulesReview

	// Finalizer search overlay state.
	finalizerSearchPattern      string
	finalizerSearchResults      []k8s.FinalizerMatch
	finalizerSearchCursor       int
	finalizerSearchSelected     map[string]bool // "ns/kind/name" keys
	finalizerSearchLoading      bool
	finalizerSearchFilter       string
	finalizerSearchFilterActive bool

	// Column toggle overlay state.
	columnToggleItems        []columnToggleEntry
	columnToggleCursor       int
	columnToggleFilter       string
	columnToggleFilterActive bool
	sessionColumns           map[string][]string // kind -> ordered visible extra column keys (session-only)
	hiddenBuiltinColumns     map[string][]string // kind -> hidden built-in column keys (session-only)
	columnOrder              map[string][]string // kind -> ordered column keys (built-ins + extras interleaved; Name is implicit)

	// Easter egg state.
	konamiProgress int  // current position in the Konami Code sequence
	konamiActive   bool // true when cheat code was just activated (clears after 5s)
	nyanMode       bool // toggleable nyan mode indicator
	nyanTick       int  // animation tick for nyan mode
	creditsScroll  int  // scroll position for credits screen
	creditsStopped bool // true when credits reached center and waiting to close
	kubetrisGame   *kubetrisGame
}

// columnToggleEntry represents a single column in the column toggle overlay.
// The builtin flag distinguishes built-in columns (Namespace/Ready/Restarts/
// Status/Age, sourced from Item fields) from extra columns (from Item.Columns,
// sourced from additionalPrinterColumns). The distinction matters because the
// two kinds are persisted in different maps on Model and have different
// name-collision handling when a CRD reuses a built-in column name.
type columnToggleEntry struct {
	key     string
	visible bool
	builtin bool
}

// ownedParentState captures the navigation state that must be restored
// when backing out of a nested LevelOwned drill-down.
type ownedParentState struct {
	resourceType model.ResourceTypeEntry
	resourceName string
	namespace    string
}

// NewModel creates the initial model.
func NewModel(client *k8s.Client, opts StartupOptions) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))

	contextName := client.CurrentContext()
	if opts.Context != "" {
		contextName = opts.Context
	}
	defaultNS := client.DefaultNamespace(contextName)

	// Watch interval precedence: CLI flag > config > default.
	watchInterval := ui.ConfigWatchInterval
	if opts.WatchInterval > 0 {
		watchInterval = ui.ClampWatchInterval(opts.WatchInterval)
	}
	if watchInterval <= 0 {
		watchInterval = ui.DefaultWatchInterval
	}

	reqCtx, reqCancel := context.WithCancel(context.Background())
	pinnedSt := loadPinnedState()
	m := Model{
		client:              client,
		nav:                 model.NavigationState{Level: model.LevelClusters},
		bookmarks:           loadBookmarks(),
		pendingSession:      loadSession(),
		pendingPortForwards: loadPortForwardState(),
		commandHistory:      loadCommandHistory(),
		pinnedState:         pinnedSt,
		namespace:           defaultNS,
		spinner:             s,
		watchInterval:       watchInterval,
		splitPreview:        true,
		allNamespaces:       true,
		watchMode:           true,
		sortColumnName:      sortColDefault,
		sortAscending:       true,
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		selectedItems:       make(map[string]bool),
		selectionAnchor:     -1,
		yamlCollapsed:       make(map[string]bool),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		allGroupsExpanded:   true,
		warningEventsOnly:   true,
		eventGrouping:       true,
		bgtasks:             bgtasks.New(bgtasks.DefaultThreshold),
		diffLineNumbers:     true,
		reqCtx:              reqCtx,
		reqCancel:           reqCancel,
		tabs: []TabState{{
			nav:                model.NavigationState{Level: model.LevelClusters},
			namespace:          defaultNS,
			splitPreview:       true,
			allNamespaces:      true,
			watchMode:          true,
			sortColumnName:     sortColDefault,
			sortAscending:      true,
			warningEventsOnly:  true,
			eventGrouping:      true,
			allGroupsExpanded:  true,
			cursorMemory:       make(map[string]int),
			itemCache:          make(map[string][]model.Item),
			selectedItems:      make(map[string]bool),
			selectionAnchor:    -1,
			selectedNamespaces: nil,
		}},
		activeTab:      0,
		execMu:         &sync.Mutex{},
		portForwardMgr: k8s.NewPortForwardManager(),
	}

	// When CLI flags are provided, replace the file-loaded session with a
	// synthetic one so the app opens in the requested context/namespace.
	if opts.HasCLIOverrides() {
		tab := SessionTab{
			Context: contextName,
		}
		if len(opts.Namespaces) > 0 {
			tab.AllNamespaces = false
			tab.Namespace = opts.Namespaces[0]
			tab.SelectedNamespaces = opts.Namespaces
		} else {
			tab.AllNamespaces = true
		}
		m.pendingSession = &SessionState{
			Context: contextName,
			Tabs:    []SessionTab{tab},
		}
	}

	m.applyPinnedGroups()

	m.helpSearchInput = textinput.New()
	m.helpSearchInput.Prompt = ""
	m.helpSearchInput.CharLimit = 100

	m.logSeverityDetector, _ = newSeverityDetector(nil) // TODO Phase I: pass extras from config
	m.logFilterChain = NewFilterChain(nil, IncludeAny, m.logSeverityDetector)
	m.logFilterEditingIdx = -1
	m.logJSONCache = newLRUJSONCache(defaultJSONCacheCap)

	return m
}

// cancelAndReset cancels any in-flight API requests and creates a fresh
// context for subsequent requests. Safe to call multiple times.
func (m *Model) cancelAndReset() {
	if m.reqCancel != nil {
		m.reqCancel()
	}
	m.reqCtx, m.reqCancel = context.WithCancel(context.Background())
}

// applyPinnedGroups merges config-level pinned groups with per-context pinned groups
// and sets model.PinnedGroups.
func (m *Model) applyPinnedGroups() {
	// Start with config-level pins.
	seen := make(map[string]bool)
	var merged []string
	for _, g := range ui.ConfigPinnedGroups {
		if !seen[g] {
			merged = append(merged, g)
			seen[g] = true
		}
	}
	// Add per-context pins.
	if m.pinnedState != nil && m.nav.Context != "" {
		for _, g := range m.pinnedState.Contexts[m.nav.Context] {
			if !seen[g] {
				merged = append(merged, g)
				seen[g] = true
			}
		}
	}
	model.PinnedGroups = merged
}

// SetVersion sets the application version string displayed in the title bar.
func (m *Model) SetVersion(v string) {
	m.version = v
}

// SetStderrChan sets the channel for receiving captured stderr messages.
func (m *Model) SetStderrChan(ch <-chan string) {
	m.stderrChan = ch
}

// Init loads the initial context list.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.loadContexts(), m.spinner.Tick}
	if m.stderrChan != nil {
		cmds = append(cmds, m.waitForStderr())
	}
	if m.watchMode {
		cmds = append(cmds, scheduleWatchTick(m.watchInterval))
	}
	if ui.ConfigTipsEnabled {
		cmds = append(cmds, scheduleStartupTip())
	}
	return tea.Batch(cmds...)
}
