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
	"github.com/hinshun/vt10x"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

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

	// cacheFingerprints maps the same keys as itemCache to a fingerprint
	// of the fetch-affecting state (namespace, allNamespaces,
	// selectedNamespaces) that was in effect when the entry was written.
	// loadResources uses it to decide whether a primed cache entry is
	// still applicable: if the current fingerprint matches, the fetch can
	// be served from cache instead of hitting the API. This is populated
	// only by updateResourcesLoadedPreview and updateResourcesLoadedMain
	// — the paths that fetch data under the current state. Other writers
	// (session restore, bookmarks, toggleRare rebuild) leave the entry
	// without a fingerprint, which safely defaults to a real fetch.
	cacheFingerprints map[string]string

	// Preview / YAML content for the right column or full screen view.
	yamlContent      string
	yamlScroll       int
	yamlCursor       int       // cursor line in visible-line space
	yamlScrollOption int       // sticky vim 'scroll' option for [count]<C-d>/<C-u>; 0 = default (half viewport)
	yamlLineInput    string    // digit buffer for 123G jump-to-line
	yamlSearchMode   bool      // true when typing in the search bar
	yamlSearchText   TextInput // current search query
	yamlMatchLines   []int     // line indices matching the search
	yamlMatchIdx     int       // current match index in yamlMatchLines

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

	// Read-only mode: blocks all mutating actions for the active tab. Mirrors
	// the active TabState.readOnly; re-evaluated on context switch and tab
	// switch.
	readOnly bool
	// cliReadOnly is the value of --read-only at startup. Sticky for the life
	// of the process so context switches can't drop it.
	cliReadOnly bool
	// contextROOverrides holds session-scoped per-context read-only state set
	// by the user via Ctrl+R on a row in the cluster picker. A present entry
	// wins over per-context and global config when entering that context;
	// CLI --read-only still wins over both.
	contextROOverrides map[string]bool

	// clusterColors holds per-context background-tint assignments set by the
	// user via Ctrl+L on a row in the cluster picker. Persisted to
	// $XDG_STATE_HOME/lfk/cluster-colors.yaml so the tint survives restarts.
	// Absent key means "no tint"; values are validated against
	// ui.ClusterColorNames at load and save time.
	clusterColors map[string]string

	// clusterColorOverlay state: cursor position within the picker's color
	// rows. Captured as Model state (not closures) so the same overlay code
	// can be re-rendered on every Update tick.
	clusterColorOverlayCursor int
	// clusterColorOverlayContext is the context name the overlay was opened
	// against — captured at open so a later refresh of m.middleItems can't
	// retarget the save to the wrong row.
	clusterColorOverlayContext string
	// clusterColorFilter holds the in-overlay / filter input so the picker
	// can narrow the visible colour list. Mirrors the schemeFilter /
	// templateFilter pattern so the standard FilterInput / handleFilterKey
	// helpers handle paste, ctrl+w, etc. uniformly.
	clusterColorFilter TextInput
	// clusterColorFilterMode is true while the user is typing into the
	// filter input; in this mode every keystroke goes to the input and
	// navigation keys (j/k/enter) are deferred until Enter or Esc exits
	// filter mode.
	clusterColorFilterMode bool

	// Help screen state.
	helpScroll       int
	helpFilter       TextInput // applied filter (f key) — narrows visible lines
	helpFilterActive bool      // whether the f filter input is being typed
	helpSearchActive bool      // whether the / search input is being typed
	helpSearchQuery  string    // applied search query (/ key) — highlights matches without filtering
	helpMatchLines   []int     // line indices in the filtered list that contain helpSearchQuery
	helpMatchIdx     int       // current position within helpMatchLines for n/N navigation
	helpContextMode  string    // section to highlight (e.g. "YAML View", "Log Viewer")
	helpPreviousMode viewMode  // mode to return to when help is closed
	helpSearchInput  textinput.Model

	// Resource filter state (/ key).
	filterText      string    // applied filter for middle column
	filterActive    bool      // whether the filter input is being typed
	filterInput     TextInput // what user is currently typing
	filterBroadMode bool      // Tab toggle: also match column values (annotations, labels, ...)

	// Search state (s key).
	searchActive     bool
	searchInput      TextInput
	searchPrevCursor int
	searchBroadMode  bool // Tab toggle inside search input: also match column values

	// Log viewer state.
	logLines          []string           // buffered log lines
	logScroll         int                // scroll offset (top visible source line)
	logWrapTopSkip    int                // wrap mode: number of sub-lines to skip from the top of logLines[logScroll]
	logFollow         bool               // auto-scroll to bottom
	logWrap           bool               // wrap long lines
	logLineNumbers    bool               // show line numbers
	logTimestamps     bool               // show timestamps (--timestamps)
	logHidePrefixes   bool               // hide [pod/name/container] prefixes
	logPreviewVisible bool               // show structured preview side panel
	logPreviewScroll  int                // body-row offset within the preview pane (J/K)
	logPrevious       bool               // show previous container logs (--previous)
	logIsMulti        bool               // multi-log stream (for restart)
	logMultiItems     []model.Item       // items for multi-log restart
	logTitle          string             // title for the log overlay
	logCancel         context.CancelFunc // cancel the kubectl log process
	logCh             chan string        // channel for streaming log lines
	logTailLines      int                // current --tail value for the active stream
	logHasMoreHistory bool               // true if older lines may exist
	logLoadingHistory bool               // true while fetching older logs
	logHistoryCancel  context.CancelFunc // cancel for the history fetch
	logCursor         int                // cursor position (absolute line index), -1 when inactive
	logVisualMode     bool               // true when in visual line selection mode
	logVisualStart    int                // anchor line where visual selection started
	logVisualType     rune               // 'V' = line, 'v' = char, 'B' = block
	logVisualCol      int                // character column of anchor (for char and block modes)
	logVisualCurCol   int                // current cursor column (for char and block modes)
	logScrollOption   int                // sticky vim 'scroll' option for [count]<C-d>/<C-u>; 0 = default (half viewport)

	// Log viewer: parent resource context for pod re-selection.
	logParentKind   string // original parent resource kind (e.g., "Deployment")
	logParentName   string // original parent resource name
	logSavedPodName string // saved pod name before overlay, for restoring on cancel

	// Log viewer: auto-reconnect for multi-container Pods. When following all
	// containers of a Pod, the kubectl stream ends as soon as the current set
	// of containers all exit (e.g. an init container finishes before the next
	// one has started). logAutoReconnectAttempt counts consecutive empty
	// reconnects so we can give up when the pod is really terminated. It is
	// reset to 0 every time a line arrives. logReconnecting tells
	// startLogStream to suppress --tail so we don't re-fetch history we
	// already have.
	logAutoReconnectAttempt int
	logReconnecting         bool

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
	logSearchActive  bool
	logSearchInput   TextInput
	logSearchQuery   string // applied search
	logSearchHistory *commandHistory

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
	describeScrollOption int            // sticky vim 'scroll' option for [count]<C-d>/<C-u>; 0 = default (half viewport)
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
	diffScrollOption int    // sticky vim 'scroll' option for [count]<C-d>/<C-u>; 0 = default (half viewport)

	// Embedded terminal state (PTY mode).
	execPTY          *os.File       // PTY master file descriptor
	execTerm         vt10x.Terminal // Virtual terminal emulator
	execTitle        string         // Title for the exec session
	execDone         *atomic.Bool   // Process has exited (shared across copies)
	execMu           *sync.Mutex    // Protects execTerm access
	execEscPressed   bool           // Ctrl+] prefix pressed, waiting for follow-up key
	execScrollback   *scrollback    // Line ring captured from the PTY byte stream for scrollback
	execScrollOffset int            // 0 = live; >0 = N rows scrolled back into history

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

	// middleItemsRev is the authoritative cache-invalidation signal for the
	// middle-column TableRenderer. It MUST be bumped whenever a render of
	// the same indices would produce different output: in-place element
	// mutation AND every slice reassignment (use setMiddleItems for the
	// latter). itemsPtr in the fingerprint is only a fast-path safety net.
	middleItemsRev uint64
	// selectionRev is bumped on every change to selectedItems so the row
	// cache invalidates and the selection marker on non-cursor rows updates.
	selectionRev uint64

	middleTableRenderer *ui.TableRenderer

	previewDebounceGen uint64

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
	// bookmarkLoadNamespace, when true, instructs the next jump issued
	// from the bookmark overlay to apply the bookmark's saved namespace
	// scope. By default bookmark jumps ignore the saved namespace and
	// keep the tab's current scope (slot case only controls context
	// switching, not namespace handling). Toggled by Tab inside the
	// overlay and shown as a `[LOAD NAMESPACE]` chip in the title.
	// Reset on overlay close and consumed after each jump so it never
	// leaks between opens.
	bookmarkLoadNamespace bool

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

	// Contexts with an in-flight API discovery call. Used to avoid
	// spamming the cluster API (and its OIDC auth flow) when the user
	// rapidly cursors through many contexts at the cluster list. Entries
	// are added when discoverAPIResources is kicked off and removed in
	// updateAPIResourceDiscovery when the result arrives.
	discoveringContexts map[string]bool

	// Contexts whose discoveredResources entries have been refreshed
	// (i.e. live-fetched) during this session. NewModel prefills
	// discoveredResources from the on-disk discovery cache for instant
	// first paint, so the bare presence of an entry no longer implies
	// "fresh" — this flag is the source of truth for stale-while-revalidate
	// gating in the lazy discovery triggers.
	discoveryRefreshedContexts map[string]bool

	// bookmarkAwaitingDiscovery holds a bookmark whose target resource type
	// can't be resolved yet because API discovery for the effective context
	// hasn't completed (typical at session restore — the seed list resolves
	// Pods/Deployments synchronously but CRDs like ArgoCD Applications are
	// only known after the discovery round-trip lands). Set by
	// navigateToBookmark, consumed by updateAPIResourceDiscovery, which
	// replays the navigation once the matching context's entries arrive.
	// Distinct from pendingBookmark (which gates save-overwrite confirmation).
	bookmarkAwaitingDiscovery *model.Bookmark

	// sessionResourceTypeAwaitingDiscovery captures the resource type ref a
	// just-restored session wants to land on when the type wasn't yet known
	// to the seed list (CRD-backed views like ArgoCD Application). The
	// matching apiResourceDiscoveryMsg consumes it and navigates to the
	// resource type so the user lands back on the view they quit from
	// instead of being dumped at the resource types level.
	sessionResourceTypeAwaitingDiscovery string
	// sessionResourceNameAwaitingDiscovery is the resource name to land on
	// once the type-await above resolves. Mirrors pendingTarget but is only
	// armed when the type itself was deferred.
	sessionResourceNameAwaitingDiscovery string

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

	serviceEndpointsCache map[string]*k8s.ServiceEndpoints // lazy Service endpoint rollup; see commands_load_preview.go and update_metrics_msgs.go
	// secretPreviewCache caches decoded secret data keyed "ctx/ns/name" to skip
	// redundant API calls on hover-after-refresh; invalidated on successful save.
	secretPreviewCache map[string]*model.SecretData

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
	eventTimelineScrollOption int             // sticky vim 'scroll' option for [count]<C-d>/<C-u>; 0 = default (half viewport)
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
	queryHistory                 *commandHistory // shared by explorer / search and f filter

	// Cached namespace names for command bar autocompletion, keyed by
	// context name. Each tab may have its own nav.Context, so keying by
	// context keeps completions correct when switching tabs or running
	// `:ctx` within a tab. Entries carry a fetchedAt timestamp so the
	// command bar can refresh them after namespaceCacheTTL without
	// refetching on every open (stale-while-revalidate: the old entry
	// stays visible while the refresh runs).
	cachedNamespaces map[string]namespaceCacheEntry

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
	// columnToggleSnapshot captures the pre-overlay values of session/
	// hidden/order maps for the current kind so Esc can revert when the
	// user explored toggles live and changed their mind. Captured at
	// openColumnToggle, consumed at handleColumnToggleKeyEsc.
	columnToggleSnapshot columnToggleSnapshot
	sessionColumns       map[string][]string // kind -> ordered visible extra column keys (session-only)
	hiddenBuiltinColumns map[string][]string // kind -> hidden built-in column keys (session-only)
	columnOrder          map[string][]string // kind -> ordered column keys (built-ins + extras interleaved; Name is implicit)

	// Easter egg state.
	konamiProgress int  // current position in the Konami Code sequence
	konamiActive   bool // true when cheat code was just activated (clears after 5s)
	nyanMode       bool // toggleable nyan mode indicator
	nyanTick       int  // animation tick for nyan mode
	creditsScroll  int  // scroll position for credits screen
	creditsStopped bool // true when credits reached center and waiting to close
	kubetrisGame   *kubetrisGame
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
	if ui.ColorModeEnabled() {
		cmds = append(cmds, ui.EnableColorModeCmd())
	}
	return tea.Batch(cmds...)
}
