package app

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/hinshun/vt10x"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// Model is the top-level bubbletea model.
type Model struct {
	client  *k8s.Client
	version string // application version string shown in the title bar

	// demoMode mirrors k8s.Client.IsDemo(); drives the title-bar DEMO badge.
	demoMode bool

	nav model.NavigationState // navigation state

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
	filterMemory map[string]savedFilter // per-level committed filter, recalled on back-nav; see saveLevelFilter (#303)

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

	// Full-screen YAML viewer state (the `y` view): content, scroll/cursor,
	// search, visual selection, wrap, and collapsible sections. Extracted
	// from the formerly flat yaml* fields into one cohesive value; see
	// yamlview.go. Note: previewYAML/splitPreview/fullYAMLPreview below are
	// distinct right-pane preview state and are intentionally not part of it.
	yamlView yamlViewState

	// Schema side pane (ctrl+k) and its description cache; see fielddoc.go.
	fieldDoc fieldDocState

	// yamlReturnMode is the mode the full-screen YAML viewer returns to on
	// q/esc. Defaults to modeExplorer (the zero value); set to
	// modeObjectExplorer when the YAML viewer is opened from the Object Explorer
	// browser so closing it returns there instead of the explorer.
	yamlReturnMode viewMode

	// yamlPendingPath, when set, is the object path the YAML viewer should
	// position its cursor on once the document finishes loading. Used to sync
	// the cursor when switching from the Object Explorer to the YAML viewer.
	yamlPendingPath []string

	// explainReturnMode is the mode the API Explorer returns to on q/esc.
	// Defaults to modeExplorer; set to modeYAML or modeObjectExplorer when the
	// Explorer is opened from those views (I key) so closing returns there.
	explainReturnMode viewMode

	// explainPendingField, when set, is the field name the API Explorer should
	// place its cursor on once the level finishes loading. Used to land on a
	// specific field (in its siblings' context) when opened from the YAML
	// viewer / Object Explorer.
	explainPendingField string

	// explainAncestors is the stack of API Explorer levels above the current
	// one. It feeds the parent pane (left column) and lets back-navigation
	// restore a level without re-fetching. Pushed on drill, popped on back;
	// reset on a fresh open or a recursive jump.
	explainAncestors []explainLevel

	// Split preview: show children in top 1/3 + YAML in bottom 2/3 of right column.
	splitPreview bool
	// Full YAML preview: show only YAML in the right column (no children list).
	fullYAMLPreview bool
	// Full log preview: show the selected pod's live logs in the right column.
	fullLogPreview bool
	// previewLog holds the bounded buffer and stream state for the live-log
	// right-pane preview (see previewlog.go).
	previewLog previewLogState
	// previewLogCache is a per-pod LRU buffer cache so re-visiting a pod
	// restores lines instantly instead of re-fetching from scratch.
	// Keyed by podRef.key() ("ctx/ns/name"). Bounded to previewLogCacheMax entries.
	// Cleared on context switch to prevent cross-cluster restore.
	previewLogCache      map[string]previewLogCacheEntry
	previewLogCacheOrder []string // insertion/access order for LRU eviction (oldest first)
	// Separate YAML content for the split/full preview in the right column,
	// so it doesn't conflict with the full-screen yamlView.content.
	previewYAML string

	// Current view mode.
	mode viewMode

	overlay             overlayKind  // active overlay kind (see overlayKind enum)
	overlayItems        []model.Item // full list (e.g., all namespaces)
	overlayFilter       TextInput    // typed filter text
	overlayCursor       int
	copyFormatPicker    copyFormatPickerState      // Y-key copy-as picker — see openCopyFormatPicker
	copyFieldPicker     copyFieldPickerState       // ctrl+y field picker — see updateCopyFieldManifests
	lastCopyFieldByKind map[string]copyFieldMemory // last entry copied per kind (ctrl+y preselect; session-only, all tabs)
	taintEditor         taintEditorState           // node taint editor — see openTaintEditor

	namespace string // current namespace (not a navigation level; displayed in top-right)

	// Terminal dimensions.
	width, height int

	err error // error to display

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
	// spinnerTicking guards the tick loop; armSpinner never stacks a second loop.
	spinnerTicking bool

	// initialSecuritySeedCmd holds the SEC-badge findings-cache seed command
	// produced by refreshSecuritySources during NewModel. NewModel cannot
	// return a command, so Init dispatches it (the read runs off the Update
	// goroutine). Nil when security is disabled or no findings cache applies.
	initialSecuritySeedCmd tea.Cmd

	// Action context: which resource/kind the action targets.
	actionCtx actionContext

	// Scale input state.
	scaleInput TextInput
	hpaScale   hpaScaleState // HPA scale overlay: min/max bounds + target replicas
	// PVC resize: current size displayed in the overlay.
	pvcCurrentSize string

	// Port forward input state.
	portForwardInput TextInput

	// Confirm action label + the pending delete's cascade policy (see delete_propagation.go).
	confirmAction      string
	confirmPropagation model.DeletePropagation
	blast              blastRadiusState // what the action costs (see blastradius.go)

	// Title and question for the type-to-confirm overlay.
	confirmTitle, confirmQuestion string

	// Text input for type-to-confirm overlay (e.g., Force Finalize).
	confirmTypeInput TextInput

	// Namespace selection state. saved* stash the selection during all-namespaces mode so toggling off restores it.
	allNamespaces                               bool
	selectedNamespaces, savedSelectedNamespaces map[string]bool
	nsSelectionNegated, savedNsSelectionNegated bool     // nsSelectionNegated: EXCLUDE set
	nsFilterMode, nsSelectionModified           bool     // nsSelectionModified: Space pressed in current ns overlay session
	nsFilterEntryItem, nsOverlayContext         string   // filter-entry ns (restored on Esc); context the open overlay lists (in-overlay R refresh)
	previousNsScope                             *nsScope // scope before the last change; g\ swaps back to it (per-tab)
	nsOverlayEntryScope                         *nsScope // scope when the ns overlay opened; the pre-edit "previous" recorded on commit

	// Fullscreen toggles: middle = hides left and right columns; dashboard
	// = renders the cluster dashboard full screen.
	fullscreenMiddle, fullscreenDashboard bool
	// hideLeftPane hides only the left resource-type sidebar; middle and
	// right preview stay visible. One phase of the kb.Fullscreen cycle.
	hideLeftPane bool

	sortColumnName string              // which column to sort by (e.g. "Name", "Age")
	sortAscending  bool                // true = ascending, false = descending
	sortMemory     map[string]sortPref // remembered sort per resource kind (context+GVR), persisted to sort_memory.yaml
	// Status message (temporary, shown in status bar).
	statusMessage    string
	statusMessageErr bool
	statusMessageExp time.Time // when message expires
	statusMessageTip bool      // true when the message is a startup tip (dismiss on keypress)

	pendingTarget string // when set, resources load selects this item by name
	// pendingTargetNamespace narrows pendingTarget to one ns (empty = name-only).
	pendingTargetNamespace string

	// pendingG: vim 'gg' -> next 'g' jumps to top; whichKey: popup + leader-panel state.
	pendingG bool
	whichKey whichKeyState

	// Vim text-object operator pending in visual mode ('i'/'a'); 0 = none.
	pendingTextObject byte

	// Vim-style named marks: m<key> sets a mark, '<key> jumps to it.
	pendingMark     bool            // waiting for the slot key after 'm'
	pendingBookmark *model.Bookmark // bookmark awaiting overwrite confirmation

	// Watch mode: auto-refresh the current view on a timer.
	watchMode     bool
	watchInterval time.Duration
	// focused tracks terminal focus (DECSET-1004); defaults true.
	focused bool
	// lastInputAt is the time of the most recent key press, for idle detection.
	lastInputAt time.Time
	// backgroundWatchInterval is the watch cadence while background or focused-idle.
	backgroundWatchInterval time.Duration
	// watchThrottle enables focus/idle throttling; false uses watchInterval always.
	watchThrottle bool
	// foregroundIdleTimeout is the no-input window before a focused window throttles; 0 disables.
	foregroundIdleTimeout time.Duration
	// watchTickGen guards the watch-tick chain (see watch_interval.go). A tick
	// whose gen does not match is a retired chain and is ignored.
	watchTickGen uint64

	// objectExplorerLive controls whether the Object Explorer re-syncs its
	// browsed object on list refreshes (issue #391). Defaults from
	// ui.ConfigObjectExplorerLive; runtime toggle is w inside the explorer.
	// objectExplorerForceSync forces a single re-sync on the next list refresh
	// even when live is off, so manual refresh (R) updates the view once.
	objectExplorerLive, objectExplorerForceSync bool
	objectExplorerTree                          bool // session tree-view pref (T); seeded from ui.ConfigObjectExplorerTree

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
	// contextBadgeOverrides holds session-scoped per-context hide-badges state
	// set by the user via kb.SecurityBadgeToggle. A present entry wins over
	// per-context and global config when (re-)entering that context, so a toggle
	// sticks within a context but never leaks across contexts.
	contextBadgeOverrides map[string]bool

	// clusterColors: per-context tint assignments set via Ctrl+L; persisted
	// to $XDG_STATE_HOME/lfk/cluster-colors.yaml. Values validated against
	// ui.ClusterColorNames; absent key = no tint.
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

	// Inline log viewer state: buffered lines, scroll/cursor, follow/wrap/
	// timestamps toggles, the streaming channel and cancel handles, container
	// filter and selection, pod/container selector filters, and search.
	// Extracted from the formerly flat log* fields into one cohesive value;
	// see logview.go.
	logView logViewState
	logTop  logTopState //nolint:unused // wired in later Log Top tasks
	// logReaderInFlight records, per log channel, whether a one-shot reader
	// goroutine is currently blocked on it. Switching into a logs tab used to
	// arm a fresh reader unconditionally, accumulating duplicate readers (and
	// out-of-order lines) on every switch. The guard lets tab switches skip
	// arming when a reader is already outstanding; updateLogLine keeps the
	// single reader alive. Shared map (reference type) so all by-value Model
	// copies observe the same state; only touched on the update goroutine.
	logReaderInFlight map[chan string]bool

	// Full-screen describe viewer state (kubectl-describe output): content,
	// scroll/cursor, auto-refresh, search, and visual selection. Extracted
	// from the formerly flat describe* fields into one cohesive value; see
	// describeview.go.
	describeView describeViewState

	// Full-screen Object Explorer state: drill-in navigation over the
	// selected resource's live object (Item.Raw), mirroring the API Explorer
	// but showing actual values. Driven entirely from objectexplorer.go.
	objectExplorerView objectExplorerState

	// objectExplorerReturnMode is the mode the Object Explorer returns to on
	// q/esc. It is the explorer by default but the YAML viewer when the Object
	// Explorer was opened from there (P), so closing returns to the opener.
	objectExplorerReturnMode viewMode

	// Full-screen diff viewer state (resource compare / revision diff):
	// left/right content, scroll/cursor, unified vs side-by-side, search,
	// fold regions, and visual selection. Extracted from the formerly flat
	// diff* fields into one cohesive value; see diffview.go.
	diffView diffViewState

	// Embedded terminal state (PTY mode).
	execPTY          *os.File       // PTY master file descriptor
	execTerm         vt10x.Terminal // Virtual terminal emulator
	execTitle        string         // Title for the exec session
	execDone         *atomic.Bool   // Process has exited (shared across copies)
	execMu           *sync.Mutex    // Protects execTerm access
	execEscPressed   bool           // Ctrl+] prefix pressed, waiting for follow-up key
	execScrollback   *scrollback    // Line ring captured from the PTY byte stream for scrollback
	execScrollOffset int            // 0 = live; >0 = N rows scrolled back into history
	// execTickGen is the generation token for the 50ms terminal-refresh tick.
	// Every arm (tab switch, focus, PTY start) takes a fresh generation; the
	// tick handler re-arms only the current generation, so older chains die
	// instead of accumulating one render loop per tab switch. Shared pointer so
	// all by-value Model copies see the same counter.
	execTickGen *atomic.Uint64

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
	// bookmarkLoadNamespace controls whether the next jump from the bookmark
	// overlay replays the bookmark's saved namespace scope. Loading is the
	// default (seeded on open); Tab opts out to keep the tab's current scope,
	// surfaced by a `[KEEP CURRENT NS]` title chip. Reset to the default on
	// overlay close and consumed after each jump so it never leaks between opens.
	bookmarkLoadNamespace bool
	sessionsOverlayState
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

	// scheduler tracks in-flight async loads AND owns priority-based
	// dispatch (resource lists, YAML fetches, metrics, dashboards).
	// Process-global instance shared across tabs so the title bar reflects
	// all activity, not just the active tab's.
	scheduler *scheduler.Registry

	// suppressBgtasks routes loaders through Registry.StartUntracked so
	// watch-mode auto-refreshes don't flash the title-bar indicator.
	suppressBgtasks bool

	// :scheduler overlay state: tasksOverlayShowCompleted (Tab) flips
	// running ↔ history; tasksOverlayShowAll (`a`, history only) lifts
	// the sub-second filter; tasksOverlayFrozenHistory pauses the live
	// history while scrolled (cleared on scroll-to-top, Tab, `a`, esc).
	tasksOverlayShowCompleted, tasksOverlayShowAll bool
	tasksOverlayFrozenHistory                      []ui.BackgroundTaskRow

	// tasksOverlayScroll is the first-visible-row index for the :tasks
	// overlay. Bumped by j/k (and friends) inside the overlay; reset on
	// open and on Tab mode switch. The renderer clamps this into a
	// valid range so the handler can bump it blindly.
	tasksOverlayScroll int

	// dashboardAcc holds the per-(kctx,gen) fan-out accumulator; keyed by dashboardAccKey.
	dashboardAcc map[string]*dashboardAccumulator
	// Discovered CRDs per context (unsynchronized: only the bubbletea update goroutine writes).
	discoveredResources map[string][]model.ResourceTypeEntry

	// Contexts with an in-flight API discovery call. Used to avoid
	// spamming the cluster API (and its OIDC auth flow) when the user
	// rapidly cursors through many contexts at the cluster list. Entries
	// are added when discoverAPIResources is kicked off and removed in
	// updateAPIResourceDiscovery when the result arrives.
	discoveringContexts map[string]bool

	// Contexts whose discoveredResources entries have been refreshed
	// (i.e. live-fetched) during this session. NewModel prefills
	// discoveredResources from the on-disk discovery cache for instant first
	// paint, so an entry's mere presence no longer implies "fresh" — this flag
	// is the source of truth for stale-while-revalidate gating in lazy discovery.
	discoveryRefreshedContexts map[string]bool

	// bookmarkAwaitingDiscovery holds a bookmark whose target resource type
	// can't be resolved yet because API discovery for the effective context
	// hasn't completed (typical at session restore — the seed list resolves
	// Pods/Deployments synchronously but CRDs like ArgoCD Applications are only
	// known after the discovery round-trip lands). Set by navigateToBookmark,
	// consumed by updateAPIResourceDiscovery, which replays the navigation once
	// the matching context's entries arrive. Distinct from pendingBookmark
	// (which gates save-overwrite confirmation).
	bookmarkAwaitingDiscovery *model.Bookmark
	// sessionResourceTypeAwaitingDiscovery captures the resource type ref a
	// just-restored session wants to land on when the type wasn't yet known to
	// the seed list (CRD-backed views like ArgoCD Application). The matching
	// apiResourceDiscoveryMsg consumes it and navigates to the resource type so
	// the user lands back on the view they quit from rather than the type level.
	sessionResourceTypeAwaitingDiscovery string
	// sessionResourceNameAwaitingDiscovery is the resource name to land on once
	// the type-await above resolves; mirrors pendingTarget but only when deferred.
	sessionResourceNameAwaitingDiscovery string
	// pendingSessionList carries the saved filter + cursor through a deferred restore.
	pendingSessionList pendingSessionListState

	// Preview scroll offset for the right column.
	previewScroll int

	// previewMeasure memoizes the scrollable right-pane content line count so
	// scrolling a large list doesn't re-render the whole list on every keystroke
	// in clampPreviewScroll. Recomputed only when the content/layout key changes.
	previewMeasureKey   previewMeasureKey
	previewMeasureLines int

	// Mouse capture runtime state. mouseAvailable is true when mouse
	// capture was enabled at startup (no --no-mouse flag and config allows
	// it); mouseCaptured tracks the current runtime state so the toggle
	// keybinding can suspend capture for native terminal text selection and
	// later re-enable it. When mouse was never available the toggle is a
	// no-op.
	mouseAvailable, mouseCaptured bool
	// wheel tracks the active wheel-scroll burst for momentum-tail dropping (#524).
	wheel wheelBurst

	// Metrics content: rendered bar graph for the preview column.
	metricsContent string
	metricsData    *metricsInputs // raw numbers behind metricsContent; recomposed on theme/resize, nil when none
	metricsLoading bool           // true while a metrics fetch for the focused resource is in flight; renders a placeholder bar instead of the previous resource's numbers

	// Preview events content: rendered event timeline for the preview column.
	previewEventsContent string
	previewEventsData    []ui.EventTimelineEntry // raw entries behind previewEventsContent; recomposed on theme/resize

	// Baseline metrics for trend detection (updated every ~60s, not every refresh).
	prevPodMetrics      map[string]model.PodMetrics
	prevPodMetricsTime  time.Time
	prevNodeMetrics     map[string]model.PodMetrics
	prevNodeMetricsTime time.Time

	// Dashboard state for the cluster overview / monitoring previews.
	dashboardPreview       string                    // rendered cluster dashboard (right column / fullscreen)
	dashboardEventsPreview string                    // warning events for the two-column layout
	dashboardData          map[string]dashboardData  // retained per context; recomposed at current width on resize / fullscreen toggle
	monitoringPreview      string                    // rendered monitoring dashboard
	monitoringData         map[string]monitoringData // raw alerts retained per context; recomposed on theme change / resize

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
	schemeEntries         []ui.SchemeEntry
	schemeCursor          int
	schemeFilter          TextInput
	schemeFilterMode      bool   // true when typing into filter
	schemeOriginalName    string // scheme name before opening overlay, for cancel restore
	schemeFilterEntryName string // scheme name selected when filter mode was entered; restored on Esc

	serviceEndpointsCache map[string]*k8s.ServiceEndpoints // stale-while-revalidate cache for the Service endpoint rollup; see commands_load_preview.go
	// orphanCache holds the most recent OrphanReport per (kubeContext, namespace); see commands_orphans.go
	orphanCache        map[orphanCacheKey]*k8s.OrphanReport
	orphanLoadInflight map[orphanCacheKey]orphanInflight
	orphanGen          uint64 // monotonic counter; bumped per scan so a superseded result is dropped on arrival
	orphans            orphanState
	// rightsizingCache stores GetRightsizing results keyed by ctx/ns/kind/name/strategy/headroom; see commands_load_overlays.go
	rightsizingCache map[string]*model.Rightsizing
	rightsizing      rightsizingState // overlay session state; see rightsizingState in app_types.go
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

	// Shared loading flag for the helm rollback + history overlays (set on dispatch, cleared on result).
	helmRevisionsLoading bool
	// editorSearch backs the / search across the K/V editor overlays
	// (secret, configmap, label). Shared because only one editor is
	// open at a time; reset on overlay open.
	editorSearch kvEditorSearchState

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
	autoSyncEnabled, autoSyncSelfHeal, autoSyncPrune bool
	autoSyncCursor                                   int // 0=autosync, 1=selfheal, 2=prune

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

	// Network policy visualizer state. netpolData holds the single-policy
	// view (Visualize on a NetworkPolicy); netpolsData the multi-policy view
	// (Network Policies on a Pod/Service). Exactly one is set at a time.
	netpolData         *k8s.NetworkPolicyInfo
	netpolsData        *k8s.NetpolsForResource
	netpolScroll       int
	netpolLineInput    string    // digit buffer for 123G jump-to-line
	netpolSearchActive bool      // true while typing in the / search bar
	netpolSearchInput  TextInput // current search input
	netpolSearchQuery  string    // committed search query (highlight + n/N)
	netpolSearchPos    int       // line index of the last n/N match (search anchor)

	// Batch label/annotation editor state.
	batchLabelMode   int       // 0=labels, 1=annotations
	batchLabelInput  TextInput // "key=value" input
	batchLabelRemove bool      // true = remove mode, false = add mode

	// Pod startup analysis state.
	podStartupData *k8s.PodStartupInfo

	crashInv           crashInvState // Crash Investigator overlay (per-pod multi-tab diagnostic view).
	syncWave           syncWaveState // Sync Wave Timeline overlay state (per-ArgoCD-Application).
	localClusterFields               // Local-cluster manager overlay (Ctrl+N at LevelClusters); see app_types.go.

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
	stderrChan    <-chan string
	shutdownState // graceful-shutdown flags (m.shuttingDown, m.shutdownNotifier)
	// Resource map view: shows relationship tree in the right column.
	mapView      bool
	resourceTree *model.ResourceNode

	// Union view mode: when true, resources are fetched from multiple clusters and merged.
	unionMode     bool
	unionContexts []string // contexts to query in union mode
	unionSetName  string   // configured union_sets entry currently active, when any
	// unionStartedFromPicker is true when the user entered union mode from the
	// cluster picker, so back-navigation can return to the picker and clear the
	// union state. CLI-started union sessions keep the old no-parent behavior.
	unionStartedFromPicker bool
	// unionContextColors maps each unionContexts entry to its configured
	// color (from the union_sets per-cluster `color:` field). Drives the
	// 1-cell row tile in the merged view. Distinct from the global
	// clusterColors map (which feeds the cluster picker) so users can
	// pick deliberate per-set palette without mutating the global state.
	unionContextColors  map[string]string
	pendingUnionSetName string // namespace picker is choosing for this union set

	// Session persistence: restores navigation state across restarts.
	pendingSession                    *SessionState      // loaded session waiting to be applied after contexts load
	pendingPortForwards               *PortForwardStates // loaded port forwards waiting to be re-established
	sessionRestored, restoringSession bool               // apply-once guard; restore in flight (session_restore_guard.go)

	// Jump history: back stack for "teleport" jumps; jump_back pops it. Capped at jumpHistoryCap.
	jumpBackStack []navSnapshot

	// Stack of LevelOwned parents for nested drill-downs (popped by navigateParent).
	ownedParentStack []ownedParentState
	// Overlay to restore when the current closes — set when a nested overlay flow opens (e.g., RBAC → namespace selector → back to RBAC).
	previousOverlay overlayKind

	pinnedState, pinnedSummariesState *PinnedState      // sidebar pins / dashboard summary pins, both per-context or per-union-set
	hiddenState                       *HiddenTypesState // per-context hidden resource types state
	portForwardMgr                    *k8s.PortForwardManager
	captureMgr                        *k8s.CaptureManager // tracks active packet capture processes
	captureOverlay                    captureOverlayState

	// Port forward overlay state: discovered ports for the selected resource.
	pfAvailablePorts          []ui.PortInfo
	pfPortCursor              int              // cursor in the available ports list (-1 = manual input)
	pfLastCreatedID           int              // ID of the most recently created port forward (for showing resolved port)
	pfLoggedErrors            map[int]struct{} // port forward IDs whose failures have been logged to errorLog
	pfOpenInBrowserAfterStart bool             // open localhost:<port> once the next forward resolves ("Port Forward & Open")
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
	explainTreeState                                  // embedded — tree-mode state; see explain_tree.go
	explainRecursiveResults      []model.ExplainField // results from recursive search
	explainRecursiveCursor       int
	explainRecursiveScroll       int
	explainRecursiveFilter       TextInput // filter input for recursive search overlay
	explainRecursiveFilterActive bool      // true when typing in filter

	// canIState (embedded, not named) — Can-I/Who-Can RBAC explorer state; see cani_state.go.
	canIState

	// Finalizer search overlay state.
	finalizerSearchPattern      string
	finalizerSearchResults      []k8s.FinalizerMatch
	finalizerSearchCursor       int
	finalizerSearchSelected     map[string]bool // "ns/kind/name" keys
	finalizerSearchLoading      bool
	finalizerSearchFilter       string
	finalizerSearchFilterActive bool
	// Column toggle overlay state; see columnToggleState in update_column_toggle.go.
	columnToggleState
	// Easter egg state (Konami, nyan, credits, kubetris).
	easterEggState
	securityModelState
}
