package app

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hinshun/vt10x"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
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
	modeObjectExplorer
	modeLogTop
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
	overlayHPAScale // HPA min/max bounds + target replicas (action menu key S)
	overlayPortForward
	overlayContainerSelect
	overlayPodSelect
	overlayBookmarks
	overlaySessions
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
	overlayClusterColor // pick a color tint for the highlighted cluster row
	overlayCrashInvestigator
	overlayOrphans // cluster-wide orphan resource overview (Shift+O)
	overlayRightsizing
	overlaySyncWave           // per-Application sync wave timeline (action menu key W)
	overlayLocalClusters      // local-cluster manager (Ctrl+N at LevelClusters)
	overlayTrafficCapture     // per-pod live packet capture (action menu key c)
	overlayCopyFormat         // Y-key copy-as picker (YAML / JSON / Table)
	overlayCopyField          // ctrl+y single-field copy picker
	overlayShuttingDown       // non-interactive "graceful shutdown in progress" notice
	overlayObjectExplorerFind // recursive key search over the object (r key)
	overlayLogTopGroupBy      // multi-select group-by field picker for Log Top
	overlayLogTopProfile      // single-select log format profile picker for Log Top
	overlayLogTopColumns      // show/hide and reorder column picker for Log Top
	overlayTaintEditor        // node taint editor (action menu key t on a Node)
)

// whoCanState groups the reverse-RBAC ("Who-Can") fields so they live
// together on Model without bloating the main struct over the
// file-length cap. Mutated by handlers in update_whocan.go.
//
// The picker is a 2-column layout: a list of resources on the left,
// subjects on the right. resourceList is the deduped union of all
// resources across canIGroups (built once on enterWhoCanMode);
// resourceCursor indexes into the *visible* (post-filter) slice so the
// cursor stays valid while the user narrows the list.
type whoCanState struct {
	verbCursor           int                 // index into ui.WhoCanVerbs
	resource             string              // last queried resource (drives subjects panel title)
	resourceList         []string            // full deduped sorted resource list (built on entry)
	resourceCursor       int                 // index into the visible (filtered) resource list
	resourceScroll       int                 // first visible row in the resource list — stateful so scrolling up keeps the cursor in place instead of pinning it to the last visible row
	resourceFilterActive bool                // true while typing into the / filter
	resourceFilter       TextInput           // live filter buffer
	subjects             []k8s.WhoCanSubject // last fetch result
	subjectsScroll       int                 // scroll offset into subjects table
	loading              bool                // fetch in flight
}

// localClusterScreen identifies which sub-screen of the local-cluster
// manager overlay is visible: the list, one of the wizard steps, or
// the delete confirmation.
type localClusterScreen int

const (
	localClusterScreenList localClusterScreen = iota
	localClusterScreenWizardProvider
	localClusterScreenWizardName
	localClusterScreenWizardVersion
	localClusterScreenWizardNodes
	localClusterScreenWizardConfirm
	localClusterScreenDeleteConfirm
)

// localClusterRowState distinguishes rows that came back from a
// provider's List() (Real) from synthetic rows for in-flight Create
// dispatches (InFlight) and from rows whose last operation failed
// (Failed). Reconciliation in updateLocalClustersDetected /
// updateLocalClusterCreated keys on Provider+Name and uses this
// state to decide whether to keep, replace, or drop a row.
type localClusterRowState int

const (
	rowStateReal     localClusterRowState = iota // returned by Provider.List
	rowStateInFlight                             // user-initiated Create still running
	rowStateFailed                               // last create/start/stop/delete returned an error
)

// localClusterRow is one row in the manager table — the union of a
// localcluster.Cluster plus per-row UI flags. Lives here (not in the
// localcluster package) because Mutating/ListError/state are
// app-level concerns specific to the manager UI.
type localClusterRow struct {
	Provider    string
	Name        string
	ContextName string
	Status      string
	K8sVersion  string
	Nodes       int
	Age         string
	Mutating    string // "" | "stopping..." | "starting..." | "deleting..."
	ListError   string // populated when this provider's List() failed
	state       localClusterRowState
}

// localClusterWizard holds the staged input across wizard sub-screens.
// Cleared every time the overlay closes; populated as the user advances.
type localClusterWizard struct {
	provider     string
	providerCur  int
	installedSet []string
	name         string
	nameErr      string
	version      string
	versionErr   string
	nodes        int
	nodesStr     string
	nodesErr     string
}

// localClusterState bundles every Model field the local-cluster
// manager overlay touches. Grouped to keep app.go below the 800-line
// file cap (mirrors whoCanState pattern).
type localClusterState struct {
	screen    localClusterScreen
	gen       uint64             // race-guard token; bumps on every Detect request
	clusters  []localClusterRow  // unified row list; .state distinguishes Real / InFlight / Failed
	cursor    int                // selected row in the list view
	loading   bool               // Detect in flight
	info      string             // transient info banner ("Creating kind/dev...", "Created kind/dev")
	err       string             // last detect/list error to surface in the header
	wizard    localClusterWizard // wizard input state (cleared on overlay close)
	deleteBuf string             // typed buffer for "type DELETE to confirm"
	deleteRow int                // index of the row being deleted
}

// localClusterFields is embedded into Model to keep the local-cluster
// manager's state accessible as m.localClusterState / m.localClusterCache
// without bloating app.go past its 800-line cap.
//
// We deliberately kept the manager logic on Model (methods on Model
// access these fields) rather than extracting a dedicated
// localClusterController type. The bubbletea Update/View pattern makes
// Model itself the controller, and lifting state out via embedding
// already gives us the practical isolation a controller would —
// without the cost of duplicating Model accessors (m.scheduler,
// m.client, m.middleItems for the cluster-picker integration) inside
// a separate type.
type localClusterFields struct {
	localClusterState localClusterState
	localClusterCache map[string]localClusterCacheEntry
}

// crashInvTab identifies which tab is active in the CrashLoopBackOff
// investigator overlay.
type crashInvTab int

const (
	crashInvTabSummary crashInvTab = iota
	crashInvTabEvents
	crashInvTabLogs
	crashInvTabDescribe
)

// crashInvScrollKey indexes per-tab, per-container scroll offsets so
// switching tabs (or containers within Logs/Describe) preserves the
// reader's position. Summary and Events are not container-scoped, so
// they leave container blank.
type crashInvScrollKey struct {
	tab       crashInvTab
	container string
}

// crashInvState groups the CrashLoopBackOff-investigator fields together
// so they live as a single field on Model. Mirrors whoCanState. The
// scroll map persists per-(tab, container) viewport offsets so switching
// tabs or containers preserves the reader's position; the renderer is
// responsible for clamping the offset when content shrinks.
type crashInvState struct {
	data            *k8s.CrashInvestigation
	activeContainer string
	activeTab       crashInvTab
	showPrevious    bool
	scroll          map[crashInvScrollKey]int
}

// syncWavePane tracks which pane has focus in the Sync Wave Timeline overlay.
//
//nolint:unused // wired in Task 7+ of the two-pane refactor
type syncWavePane int

//nolint:unused // wired in Task 7+ of the two-pane refactor
const (
	paneSidebar syncWavePane = iota
	paneBody
)

// syncWaveBodyCursor identifies a row in the body pane — either a wave
// header (resourceIdx == -1) or a resource row inside a wave (resourceIdx >= 0).
// waveIdx == -1 means the body shows a placeholder (collapsed or empty phase).
//
//nolint:unused // wired in Task 7+ of the two-pane refactor
type syncWaveBodyCursor struct {
	waveIdx     int
	resourceIdx int
}

// syncWaveState groups the Sync Wave Timeline overlay's fields. Mirrors
// crashInvState. The token field rotates on every overlay open so async
// messages and ticks from a previous session can never trigger a load
// into a fresh session.
//
//nolint:unused // bodyCursor and activePane are wired in Task 7+ of the two-pane refactor
type syncWaveState struct {
	data          *k8s.SyncWaveTimeline
	collapsed     map[string]bool // phase keys ("<phase>") and wave keys ("<phase>/<waveLabel>")
	token         uint64
	lastRefreshAt time.Time
	loadingFrame  int

	sidebarCursor int                // index into data.Phases (derives selectedPhase)
	bodyCursor    syncWaveBodyCursor // identifies a row in the focused phase
	bodyScroll    int                // body's first-visible row offset

	activePane syncWavePane
}

// rightsizingState groups the per-session right-sizing overlay fields
// so they live together on Model without bloating the main struct over
// the file-length cap. The cache (Model.rightsizingCache) is kept
// separate because it survives across overlay opens; this struct is
// reset (or its scroll/data swapped) every time the overlay is opened
// for a different workload.
//
//   - data is the currently-displayed payload. Nil means "not loaded
//     yet" and the overlay shows a loading state.
//   - loading is true while a fetch is in flight.
//   - err surfaces a non-recoverable error from the fetch.
//   - gen guards against stale-fetch races (overlay closed + reopened
//     with a different workload before a slow fetch returns).
//   - scroll is the visible-row offset within the overlay's table when
//     it overflows the visible height.
//   - strategy is the currently-selected recommendation algorithm.
//     The [/] picker walks `available` to switch.
//   - available caches the list of usable strategies for the current
//     workload + cluster (computed once on overlay open by
//     k8s.AvailableRightsizingStrategies). Empty means "no probe yet"
//     and the overlay shows snapshot only.
//   - headroom is the safety-margin multiplier applied to the
//     recommendation. The </> picker walks model.RightsizingHeadrooms
//     to cycle through preset values. Seeded to
//     model.DefaultRightsizingHeadroom on overlay open.
type rightsizingState struct {
	data      *model.Rightsizing
	loading   bool
	err       error
	gen       int
	scroll    int
	strategy  model.RightsizingStrategy
	available []model.RightsizingStrategy
	headroom  float64
}

// canIViewMode toggles the Can-I overlay between its forward view
// (subject → permissions, the original Can-I browser) and the
// reverse view (verb + resource → subjects, "Who-Can"). Tab cycles
// between them so users can pivot between "what can I do" and
// "who else can do this" without re-opening the overlay.
type canIViewMode int

const (
	canIModeForward canIViewMode = iota // subject -> permissions (Can-I)
	canIModeWhoCan                      // verb + resource -> subjects (Who-Can)
)

// bookmarkOverlayMode tracks the interaction mode for the bookmark overlay.
type bookmarkOverlayMode int

const (
	bookmarkModeNormal bookmarkOverlayMode = iota
	bookmarkModeFilter
	bookmarkModeConfirmDelete
	bookmarkModeConfirmDeleteAll
)

// kvEditorSearchState backs the / search + multi-row selection +
// Shift+Y format-picker for the K/V editor overlays (secret,
// configmap, label). All state lives here because only one editor
// is open at a time and all three share the same UX.
//
//   - active / query: the / filter — narrows visible keys.
//   - selected: keys marked with `s` for batch copy. Lazy-init in
//     handlers (nil = no selections). Lookup by key string so
//     toggles survive filter changes / row reorders.
//   - formatActive / formatCursor: drive the Shift+Y format-picker
//     chip row (yaml/json/dotenv/...). When active, key input
//     routes to handle*FormatPickerKey instead of normal mode.
//   - editValueScroll: visible-line offset for the value field's
//     edit pane. Sticky scroll — only adjusted when the cursor
//     leaves the visible window — so arrow-up doesn't pin the
//     cursor to the bottom row while content scrolls underneath.
//
// Reset on overlay open + close so stale state can't leak into the
// next editor session.
type kvEditorSearchState struct {
	active          bool
	query           TextInput
	selected        map[string]bool
	formatActive    bool
	formatCursor    int
	editValueScroll int
}

// sortColDefault is the default sort column name.
const sortColDefault = "Name"

// sortColEventLastSeen is a sentinel used internally by sortMiddleItems
// to override the default "Name" sort for Events. It is NOT a user-visible
// column name and must not appear in the sortable-column cycle.
const sortColEventLastSeen = "__event_last_seen__"

// UnionContextSentinel is the value stored in nav.Context when the user is at
// LevelResources in --union-context mode. It is never sent to the Kubernetes
// API; effectiveContext() resolves it to a real cluster for API calls.
const UnionContextSentinel = "__union__"

const (
	contextCategory                  = "Contexts"
	unionSetItemKind                 = "__union_set__"
	unionSetCategory                 = "Union Sets"
	unionClusterDashboardKind        = "__union_cluster_dashboard__"
	unionMonitoringDashboardKind     = "__union_monitoring_dashboard__"
	unionDashboardMemberItemKind     = "__union_dashboard_member__"
	unionDashboardMemberAPIGroup     = "_union_dashboard"
	unionClusterDashboardResource    = "cluster-dashboards"
	unionMonitoringDashboardResource = "monitoring-dashboards"
)

// actionContext stores which resource an action targets.
type actionContext struct {
	kind          string // Kubernetes Kind (e.g., "Pod", "Deployment")
	name          string // resource name
	namespace     string // namespace of the target resource (captured at action time)
	context       string // kubeconfig context name (captured at action time)
	containerName string // container name (for exec/logs at container level)
	os            string // pod OS ("windows"/"linux"/""); resolved before exec to pick the shell
	image         string // container image (for vuln scan at container level)
	resourceType  model.ResourceTypeEntry
	columns       []model.KeyValue // additional item columns (e.g., Node, IP) for custom action templates
	raw           map[string]any   // full source object; used to prefill scale overlays from spec/status
}

// hpaScaleState backs the HPA scale overlay. The overlay edits the HPA's
// own min/max bounds (patched on the HPA) and, separately, the replica count
// of its scale target (a Deployment/StatefulSet/ReplicaSet — scaled directly,
// which the HPA may revert on its next reconcile). orig* snapshots the
// prefilled values so apply only issues a call for fields the user changed.
type hpaScaleState struct {
	min        TextInput
	max        TextInput
	target     TextInput
	field      int // active row: 0=min, 1=max, 2=target
	targetKind string
	targetName string
	origMin    string
	origMax    string
	origTarget string
}

// TabState holds per-tab navigation state so each tab is fully independent.
type TabState struct {
	// needsLoad is true for tabs restored from a session file that have not
	// yet had their items loaded.  When loadTab detects this flag it triggers
	// a full refreshCurrentLevel instead of the lighter loadPreview.
	needsLoad bool

	nav              model.NavigationState
	leftItems        []model.Item
	middleItems      []model.Item
	rightItems       []model.Item
	leftItemsHistory [][]model.Item
	// jumpBackStack holds this tab's teleport history so each tab navigates
	// its own jumps independently. navSnapshots are never mutated after
	// captureNavSnapshot deep-copies their slices, so copying the outer
	// slice on save/restore is enough.
	jumpBackStack      []navSnapshot
	cursors            [5]int
	middleScroll       int // persistent scroll position for middle column (vim-style scrolloff)
	leftScroll         int // persistent scroll position for left column (vim-style scrolloff)
	cursorMemory       map[string]int
	filterMemory       map[string]savedFilter
	itemCache          map[string][]model.Item
	cacheFingerprints  map[string]string
	yamlContent        string
	yamlScroll         int
	yamlCursor         int // cursor position in visible lines (relative to scroll)
	yamlScrollOption   int // sticky vim 'scroll' option for [count]<C-d>/<C-u>; 0 = default (half viewport)
	yamlSearchText     TextInput
	yamlMatchLines     []int
	yamlMatchIdx       int
	yamlCollapsed      map[string]bool // collapsed state for YAML sections
	splitPreview       bool
	fullYAMLPreview    bool
	fullLogPreview     bool
	previewYAML        string
	namespace          string
	allNamespaces      bool
	selectedNamespaces map[string]bool
	nsSelectionNegated bool
	// Stashed namespace selection while all-namespaces mode is active, so
	// toggling the mode off restores the prior multi-select instead of
	// resetting to the default namespace.
	savedSelectedNamespaces map[string]bool
	savedNsSelectionNegated bool
	// previousNsScope is this tab's jump-to-previous-namespace target.
	previousNsScope *nsScope
	sortColumnName  string // column name to sort by (e.g. "Name", "Age", "CPU")
	sortAscending   bool
	sortMemory      map[string]sortPref
	filterText      string
	filterBroadMode bool
	// Quick filter preset applied to this tab, and the pre-filter middle list
	// captured when it was applied. Per-tab so a preset set on one tab does not
	// leak onto siblings (the next data load would otherwise re-apply it and
	// hide non-matching rows).
	activeFilterPreset    *FilterPreset
	unfilteredMiddleItems []model.Item
	// Identity of the highlighted resource row, captured on save so session
	// restore can reopen every tab (not just the active one) on the same item.
	cursorName         string
	cursorNamespace    string
	watchMode          bool
	objectExplorerLive bool
	objectExplorerTree bool
	// readOnly blocks all mutating actions for this tab. Re-evaluated on
	// context switch from CLI flag, per-context config, and global config.
	readOnly               bool
	requestGen             uint64
	selectedItems          map[string]bool
	selectionAnchor        int // anchor index for region selection (-1 = unset)
	fullscreenMiddle       bool
	fullscreenDashboard    bool
	hideLeftPane           bool
	dashboardPreview       string
	dashboardEventsPreview string // warning events for two-column dashboard
	monitoringPreview      string
	// metricsContent and previewEventsContent are right-pane footers
	// rendered below the children list. They have to live per-tab —
	// otherwise switching from a Pods tab (which has metrics) to a
	// Services tab (which doesn't) leaves the Pods metrics rendered
	// at the bottom of the Services preview because no loader fires
	// to clear them.
	metricsContent       string
	previewEventsContent string
	// Raw inputs behind the two footers above, retained per-tab so a theme
	// change / resize can re-render them in place (see recomposeThemedContent).
	metricsData       *metricsInputs
	previewEventsData []ui.EventTimelineEntry
	metricsLoading    bool

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
	mode              viewMode
	logLines          []string
	logScroll         int
	logWrapTopSkip    int
	logFollow         bool
	logWrap           bool
	logLineNumbers    bool
	logTimestamps     bool
	logPrevious       bool
	logIsMulti        bool
	logTitle          string
	logCancel         context.CancelFunc
	logCh             chan string
	logTailLines      int  // current --tail value for the active stream
	logHasMoreHistory bool // true if older lines may exist
	logLoadingHistory bool // true while fetching older logs
	logFilterQuery    string
	logSevThreshold   int
	logCursor         int  // cursor position (absolute line index), -1 when inactive
	logVisualMode     bool // true when in visual line selection mode
	logVisualStart    int  // anchor line where visual selection started
	logVisualType     rune // 'V' = line, 'v' = char, 'B' = block
	logVisualCol      int  // character column of anchor (for char and block modes)
	logVisualCurCol   int  // current cursor column (for char and block modes)
	logScrollOption   int  // sticky vim 'scroll' option for [count]<C-d>/<C-u>; 0 = default (half viewport)

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
	execPTY          *os.File
	execTerm         vt10x.Terminal
	execTitle        string
	execDone         *atomic.Bool
	execMu           *sync.Mutex
	execScrollback   *scrollback // line ring captured from the PTY byte stream
	execScrollOffset int         // 0 = live; >0 = N rows scrolled back into history

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

	// Log Top aggregation state (per-tab).
	logTopProfile     string
	logTopGroupBy     []string
	logTopSortCol     string
	logTopSortAsc     bool
	logTopAutoProf    bool
	logTopFilterQuery string
	logTopColOrder    []string
	logTopColHidden   []string // serialized (sorted key) form of the runtime colHidden map[string]bool

	// Security feature state — per-tab so two tabs pointing at different
	// clusters keep their own source manager and availability map.
	// Without these, the active tab's state leaked into other tabs via
	// the global SecuritySourcesFn hook, so the sidebar's Security
	// category showed the wrong sources after tab switches.
	// securityIgnores stays at Model level (the rules database is keyed
	// by context internally and shared across tabs).
	securityManager            *security.Manager
	securityAvailabilityByName map[string]bool
	securityIndex              *security.FindingIndex
	securityActiveGroup        string
	securityActiveSource       string
	securityResourceFilter     []security.ResourceRef
	showSecurityIgnored        bool
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
