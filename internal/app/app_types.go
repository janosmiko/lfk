package app

import (
	"context"
	"os"
	"sync"
	"sync/atomic"

	"github.com/hinshun/vt10x"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
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
	overlayClusterColor // pick a color tint for the highlighted cluster row
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

// sortColDefault is the default sort column name.
const sortColDefault = "Name"

// sortColEventLastSeen is a sentinel used internally by sortMiddleItems
// to override the default "Name" sort for Events. It is NOT a user-visible
// column name and must not appear in the sortable-column cycle.
const sortColEventLastSeen = "__event_last_seen__"

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

	nav                model.NavigationState
	leftItems          []model.Item
	middleItems        []model.Item
	rightItems         []model.Item
	leftItemsHistory   [][]model.Item
	cursors            [5]int
	middleScroll       int // persistent scroll position for middle column (vim-style scrolloff)
	leftScroll         int // persistent scroll position for left column (vim-style scrolloff)
	cursorMemory       map[string]int
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
	previewYAML        string
	namespace          string
	allNamespaces      bool
	selectedNamespaces map[string]bool
	sortColumnName     string // column name to sort by (e.g. "Name", "Age", "CPU")
	sortAscending      bool
	filterText         string
	watchMode          bool
	// readOnly blocks all mutating actions for this tab. Re-evaluated on
	// context switch from CLI flag, per-context config, and global config.
	readOnly               bool
	requestGen             uint64
	selectedItems          map[string]bool
	selectionAnchor        int // anchor index for region selection (-1 = unset)
	fullscreenMiddle       bool
	fullscreenDashboard    bool
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
