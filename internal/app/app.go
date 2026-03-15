package app

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hinshun/vt10x"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
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
)

// overlayKind tracks which overlay is currently open.
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayNamespace
	overlayAction
	overlayConfirm
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
	overlayColorscheme
	overlayFilterPreset
	overlayRBAC
	overlayBatchLabel
	overlayPodStartup
	overlayQuotaDashboard
	overlayEventTimeline
	overlayAlerts
	overlayNetworkPolicy
)

// bookmarkOverlayMode tracks the interaction mode for the bookmark overlay.
type bookmarkOverlayMode int

const (
	bookmarkModeNormal bookmarkOverlayMode = iota
	bookmarkModeFilter
)

// sortMode determines how resources are sorted in the middle column.
type sortMode int

const (
	sortByName   sortMode = iota
	sortByAge
	sortByStatus
)

// actionContext stores which resource an action targets.
type actionContext struct {
	kind          string // Kubernetes Kind (e.g., "Pod", "Deployment")
	name          string // resource name
	namespace     string // namespace of the target resource (captured at action time)
	context       string // kubeconfig context name (captured at action time)
	containerName string // container name (for exec/logs at container level)
	resourceType  model.ResourceTypeEntry
	columns       []model.KeyValue // additional item columns (e.g., Node, IP) for custom action templates
}

// TabState holds per-tab navigation state so each tab is fully independent.
type TabState struct {
	nav              model.NavigationState
	leftItems        []model.Item
	middleItems      []model.Item
	rightItems       []model.Item
	leftItemsHistory [][]model.Item
	cursors          [5]int
	cursorMemory     map[string]int
	itemCache        map[string][]model.Item
	yamlContent      string
	yamlScroll       int
	yamlCursor       int // cursor position in visible lines (relative to scroll)
	yamlSearchText   TextInput
	yamlMatchLines   []int
	yamlMatchIdx     int
	yamlCollapsed    map[string]bool // collapsed state for YAML sections
	splitPreview     bool
	fullYAMLPreview  bool
	previewYAML      string
	namespace          string
	allNamespaces      bool
	selectedNamespaces map[string]bool
	sortBy             sortMode
	filterText       string
	watchMode        bool
	requestGen       uint64
	selectedItems    map[string]bool
	fullscreenMiddle    bool
	fullscreenDashboard bool
	dashboardPreview    string
	monitoringPreview   string
	previewScroll       int

	// Toggle to show only Warning events in Event list view.
	warningEventsOnly bool

	// Collapsible tree view state for resource types.
	expandedGroup    string // currently expanded category (accordion behavior)
	allGroupsExpanded bool  // override: show all groups expanded (toggled by hotkey)

	// Per-tab view mode and fullscreen state.
	mode           viewMode
	logLines       []string
	logScroll      int
	logFollow      bool
	logWrap        bool
	logLineNumbers bool
	logTitle       string
	logCancel      context.CancelFunc
	logCh          chan string

	// Log viewer: parent resource context for pod re-selection.
	logParentKind string
	logParentName string

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
	yamlCursor     int    // cursor line in visible-line space
	yamlSearchMode bool      // true when typing in the search bar
	yamlSearchText TextInput // current search query
	yamlMatchLines []int  // line indices matching the search
	yamlMatchIdx   int    // current match index in yamlMatchLines

	// Collapsible YAML sections.
	yamlSections  []yamlSection    // parsed hierarchical sections
	yamlCollapsed map[string]bool  // collapsed state per section key (persists across resources)

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

	// Spinner for loading animation.
	spinner spinner.Model

	// Action context: which resource/kind the action targets.
	actionCtx actionContext

	// Scale input state.
	scaleInput TextInput

	// Port forward input state.
	portForwardInput TextInput

	// Confirm action label (for delete confirmation).
	confirmAction string

	// All-namespaces mode.
	allNamespaces bool

	// Multi-select namespace state.
	selectedNamespaces map[string]bool
	nsFilterMode       bool
	nsSelectionModified bool // tracks if Space was pressed in current ns overlay session

	// Fullscreen middle column: hides left and right columns.
	fullscreenMiddle bool

	// Fullscreen dashboard: renders the cluster overview dashboard full screen.
	fullscreenDashboard bool

	// Sort mode for resources.
	sortBy sortMode

	// Status message (temporary, shown in status bar).
	statusMessage    string
	statusMessageErr bool
	statusMessageExp time.Time // when message expires


	// Pending target: when set, after resources load, find and select this item by name.
	pendingTarget string

	// Vim-style 'gg' command: when true, the next 'g' press jumps to top.
	pendingG bool

	// Watch mode: auto-refresh the current view on a timer.
	watchMode     bool
	watchInterval time.Duration

	// Help screen state.
	helpScroll       int
	helpFilter       TextInput
	helpSearchActive bool
	helpContextMode  string // section to highlight (e.g. "YAML View", "Log Viewer")
	helpSearchInput  textinput.Model

	// Resource filter state (/ key).
	filterText   string // applied filter for middle column
	filterActive bool   // whether the filter input is being typed
	filterInput  TextInput // what user is currently typing

	// Search state (s key).
	searchActive     bool
	searchInput      TextInput
	searchPrevCursor int

	// Log viewer state.
	logLines       []string           // buffered log lines
	logScroll      int                // scroll offset (top visible line)
	logFollow      bool               // auto-scroll to bottom
	logWrap        bool               // wrap long lines
	logLineNumbers bool               // show line numbers
	logTitle       string             // title for the log overlay
	logCancel      context.CancelFunc // cancel the kubectl log process
	logCh          chan string         // channel for streaming log lines

	// Log viewer: parent resource context for pod re-selection.
	logParentKind string // original parent resource kind (e.g., "Deployment")
	logParentName string // original parent resource name

	// Log viewer: jump to line (digits + G).
	logLineInput string

	// Log viewer: search state.
	logSearchActive bool
	logSearchInput  TextInput
	logSearchQuery  string // applied search
	logSearchMatch  int    // index of current match line, -1 if none

	// Describe viewer state.
	describeContent string
	describeScroll  int
	describeTitle   string

	// Diff viewer state.
	diffLeft      string // YAML content of first resource
	diffRight     string // YAML content of second resource
	diffLeftName  string // name of first resource
	diffRightName string // name of second resource
	diffScroll      int    // scroll position in diff view
	diffUnified     bool   // true = unified diff, false = side-by-side
	diffLineNumbers bool   // show line numbers in diff view
	diffLineInput   string // digit accumulator for jump-to-line (digits + G)

	// Embedded terminal state (PTY mode).
	execPTY   *os.File         // PTY master file descriptor
	execTerm  vt10x.Terminal   // Virtual terminal emulator
	execTitle string           // Title for the exec session
	execDone  *atomic.Bool     // Process has exited (shared across copies)
	execMu    *sync.Mutex      // Protects execTerm access

	// Multi-selection state: maps "namespace/name" keys to selected status.
	selectedItems map[string]bool

	// Bulk action mode flag: true when the current action applies to multiple items.
	bulkMode bool

	// Bulk action items: captured list of selected items for bulk operations.
	bulkItems []model.Item

	// Pending action waiting for container selection.
	pendingAction string

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
	bookmarkFilter     TextInput        // filter text (f mode) for bookmark overlay
	bookmarkSearchMode bookmarkOverlayMode // current interaction mode for bookmark overlay

	// Template overlay state.
	templateItems  []model.ResourceTemplate
	templateCursor int

	// Show decoded secret values in preview.
	showSecretValues bool

	// Toggle to show only Warning events in Event list view.
	warningEventsOnly bool

	// Discovered CRDs per context: keyed by context name.
	discoveredCRDs map[string][]model.ResourceTypeEntry

	// Preview scroll offset for the right column.
	previewScroll int

	// Metrics content: rendered bar graph for the preview column.
	metricsContent string

	// Baseline metrics for trend detection (updated every ~60s, not every refresh).
	prevPodMetrics      map[string]model.PodMetrics
	prevPodMetricsTime  time.Time
	prevNodeMetrics     map[string]model.PodMetrics
	prevNodeMetricsTime time.Time

	// Dashboard preview: rendered cluster overview for the right column.
	dashboardPreview string

	// Monitoring preview: rendered monitoring overview for the right column.
	monitoringPreview string

	// Collapsible tree view state for resource types.
	expandedGroup     string // currently expanded category (accordion behavior)
	allGroupsExpanded bool   // override: show all groups expanded (toggled by hotkey)

	// Error log: global buffer of application errors for the error log overlay.
	errorLog        []ui.ErrorLogEntry
	overlayErrorLog bool
	errorLogScroll  int
	showDebugLogs   bool

	// Color scheme selector state.
	schemeEntries      []ui.SchemeEntry
	schemeCursor       int
	schemeFilter       TextInput
	schemeFilterMode   bool   // true when typing into filter
	schemeOriginalName string // scheme name before opening overlay, for cancel restore

	// Secret editor state.
	secretData        *model.SecretData
	secretCursor      int
	secretRevealed    map[string]bool
	secretAllRevealed bool
	secretEditing     bool
	secretEditKey     TextInput
	secretEditValue   TextInput
	secretEditColumn  int // 0=key, 1=value

	// ConfigMap editor state.
	configMapData       *model.ConfigMapData
	configMapCursor     int
	configMapEditing    bool
	configMapEditKey    TextInput
	configMapEditValue  TextInput
	configMapEditColumn int // 0=key, 1=value

	// Rollback overlay state (deployments).
	rollbackRevisions []k8s.DeploymentRevision
	rollbackCursor    int

	// Helm rollback overlay state.
	helmRollbackRevisions []ui.HelmRevision
	helmRollbackCursor    int

	// Label/annotation editor state.
	labelData         *model.LabelAnnotationData
	labelCursor       int
	labelTab          int // 0=labels, 1=annotations
	labelEditing      bool
	labelEditKey      TextInput
	labelEditValue    TextInput
	labelEditColumn   int                    // 0=key, 1=value
	labelResourceType model.ResourceTypeEntry // the resource type being edited

	// Quick filter preset state.
	filterPresets        []FilterPreset
	activeFilterPreset   *FilterPreset  // currently applied filter preset, nil if none
	unfilteredMiddleItems []model.Item  // full list before filter preset was applied

	// RBAC permission check state.
	rbacResults []k8s.RBACCheck
	rbacKind    string

	// Quota dashboard state.
	quotaData []k8s.QuotaInfo

	// Prometheus alerts overlay state.
	alertsData   []k8s.AlertInfo // alerts for current resource
	alertsScroll int             // scroll position in alerts overlay

	// Network policy visualizer state.
	netpolData   *k8s.NetworkPolicyInfo
	netpolScroll int

	// Batch label/annotation editor state.
	batchLabelMode   int    // 0=labels, 1=annotations
	batchLabelInput  TextInput // "key=value" input
	batchLabelRemove bool   // true = remove mode, false = add mode

	// Pod startup analysis state.
	podStartupData *k8s.PodStartupInfo

	// Event timeline overlay state.
	eventTimelineData   []k8s.EventInfo // event timeline data
	eventTimelineScroll int             // scroll position

	// Command bar state.
	commandBarActive             bool
	commandBarInput              TextInput
	commandBarSuggestions        []string
	commandBarSelectedSuggestion int
	commandHistory               *commandHistory

	// Cached namespace names for command bar autocompletion.
	cachedNamespaces []string

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
	pfPortCursor     int // cursor in the available ports list (-1 = manual input)
	pfLastCreatedID  int // ID of the most recently created port forward (for showing resolved port)
}

// ownedParentState captures the navigation state that must be restored
// when backing out of a nested LevelOwned drill-down.
type ownedParentState struct {
	resourceType model.ResourceTypeEntry
	resourceName string
	namespace    string
}

// NewModel creates the initial model.
func NewModel(client *k8s.Client) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))

	reqCtx, reqCancel := context.WithCancel(context.Background())
	pinnedSt := loadPinnedState()
	m := Model{
		client:         client,
		nav:            model.NavigationState{Level: model.LevelClusters},
		bookmarks:      loadBookmarks(),
		pendingSession:      loadSession(),
		pendingPortForwards: loadPortForwardState(),
		commandHistory: loadCommandHistory(),
		pinnedState:    pinnedSt,
		namespace:      client.DefaultNamespace(client.CurrentContext()),
		spinner:        s,
		watchInterval:  2 * time.Second,
		splitPreview:   true,
		allNamespaces:  true,
		watchMode:      true,
		cursorMemory:   make(map[string]int),
		itemCache:      make(map[string][]model.Item),
		selectedItems:  make(map[string]bool),
		yamlCollapsed:  make(map[string]bool),
		discoveredCRDs:    make(map[string][]model.ResourceTypeEntry),
		allGroupsExpanded: true,
		warningEventsOnly: true,
		diffLineNumbers:   true,
		reqCtx:            reqCtx,
		reqCancel:         reqCancel,
		tabs: []TabState{{
			nav:                model.NavigationState{Level: model.LevelClusters},
			namespace:          client.DefaultNamespace(client.CurrentContext()),
			splitPreview:       true,
			allNamespaces:      true,
			watchMode:          true,
			warningEventsOnly:  true,
			allGroupsExpanded:  true,
			cursorMemory:       make(map[string]int),
			itemCache:          make(map[string][]model.Item),
			selectedItems:      make(map[string]bool),
			selectedNamespaces: nil,
		}},
		activeTab:      0,
		execMu:         &sync.Mutex{},
		portForwardMgr: k8s.NewPortForwardManager(),
	}
	m.applyPinnedGroups()

	m.helpSearchInput = textinput.New()
	m.helpSearchInput.Prompt = ""
	m.helpSearchInput.CharLimit = 100

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
	cmds := []tea.Cmd{m.loadContexts, m.spinner.Tick}
	if m.stderrChan != nil {
		cmds = append(cmds, m.waitForStderr())
	}
	if m.watchMode {
		cmds = append(cmds, scheduleWatchTick(m.watchInterval))
	}
	return tea.Batch(cmds...)
}

// View renders the UI.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Render fullscreen modes (YAML, Logs, Describe, Diff, Exec) with title bar and tab bar.
	// Each view renders its own hint bar, so the main status bar is not shown.
	if m.mode == modeYAML || m.mode == modeLogs || m.mode == modeDescribe || m.mode == modeDiff || m.mode == modeExec {
		title := m.renderTitleBar()
		m.height -= 1 // title bar

		var tabBar string
		if len(m.tabs) > 1 {
			tabBar = ui.RenderTabBar(m.tabLabels(), m.activeTab, m.width)
			m.height-- // tab bar takes one line
		}

		var content string
		switch m.mode {
		case modeYAML:
			content = m.viewYAML()
		case modeLogs:
			content = m.viewLogs()
		case modeDescribe:
			content = m.viewDescribe()
		case modeDiff:
			content = m.viewDiff()
		case modeExec:
			content = m.viewExecTerminal()
		}

		var parts []string
		parts = append(parts, title)
		if tabBar != "" {
			parts = append(parts, tabBar)
		}
		parts = append(parts, content)
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}

	view := m.viewExplorer()

	// Render overlay on top if active.
	if m.overlay != overlayNone {
		view = m.renderOverlay(view)
	}

	// Render error log overlay on top if active (independent of regular overlays).
	if m.overlayErrorLog {
		view = m.renderErrorLogOverlay(view)
	}

	// Render help screen as an overlay on top of the explorer view.
	if m.mode == modeHelp {
		overlay := ui.RenderHelpScreen(m.width, m.height, m.helpScroll, m.helpFilter.Value, m.helpSearchActive, &m.helpSearchInput)
		view = ui.PlaceOverlay(m.width, m.height, overlay, view)
	}

	return view
}

func (m Model) viewExplorer() string {
	// Set highlight query for search/filter term highlighting.
	ui.ActiveHighlightQuery = m.filterText
	if m.searchActive {
		ui.ActiveHighlightQuery = m.searchInput.Value
	}
	defer func() { ui.ActiveHighlightQuery = "" }()

	// Set secret values visibility for rendering.
	ui.ActiveShowSecretValues = m.showSecretValues

	// Set fullscreen mode for column visibility.
	ui.ActiveFullscreenMode = m.fullscreenMiddle

	// Set selection state for rendering.
	ui.ActiveSelectedItems = m.selectedItems
	defer func() { ui.ActiveSelectedItems = nil }()

	// Calculate column widths: left=12%, middle=51%, right=remainder (~37%).
	usable := m.width - 6 // 3 columns x 2 border chars
	var leftW, middleW, rightW int
	if m.fullscreenDashboard || m.fullscreenMiddle {
		leftW = 0
		rightW = 0
		middleW = m.width - 2 // single column with border
	} else {
		leftW = max(10, usable*12/100)
		middleW = max(10, usable*51/100)
		rightW = max(10, usable-leftW-middleW)
	}

	contentHeight := m.height - 4 // room for title + status bar
	if contentHeight < 3 {
		contentHeight = 3
	}

	// Tab bar (only shown with 2+ tabs).
	var tabBar string
	if len(m.tabs) > 1 {
		tabBar = ui.RenderTabBar(m.tabLabels(), m.activeTab, m.width)
		contentHeight-- // tab bar takes one line
	}

	// Column padding is 1 on each side, so inner content width is 2 less.
	colPad := 2
	leftInner := leftW - colPad
	middleInner := middleW - colPad
	rightInner := rightW - colPad
	if leftInner < 5 {
		leftInner = 5
	}
	if middleInner < 5 {
		middleInner = 5
	}
	if rightInner < 5 {
		rightInner = 5
	}

	// Only show error in the middle column when there are no items (first load failure).
	// Otherwise errors are displayed in the status bar.
	var middleErrMsg string
	if m.err != nil && len(m.middleItems) == 0 {
		middleErrMsg = m.err.Error()
	}

	// Set collapsed state for rendering resource type categories.
	if m.nav.Level == model.LevelResourceTypes && !m.allGroupsExpanded {
		collapsed := make(map[string]bool)
		for _, item := range m.middleItems {
			if item.Category != "" && item.Category != m.expandedGroup {
				collapsed[item.Category] = true
			}
		}
		ui.ActiveCollapsedCategories = collapsed
		ui.ActiveCategoryCounts = m.categoryCounts()
	} else {
		ui.ActiveCollapsedCategories = nil
		ui.ActiveCategoryCounts = nil
	}

	// Build columns.
	middleHeader := m.middleColumnHeader()
	var middleCol string
	switch m.nav.Level {
	case model.LevelResources, model.LevelOwned, model.LevelContainers:
		middleCol = ui.RenderTable(middleHeader, m.visibleMiddleItems(), m.cursor(), middleInner, contentHeight, m.loading, m.spinner.View(), middleErrMsg)
	default:
		middleCol = ui.RenderColumn(middleHeader, m.visibleMiddleItems(), m.cursor(), middleInner, contentHeight, true, m.loading, m.spinner.View(), middleErrMsg)
	}
	middleCol = padToHeight(middleCol, contentHeight)
	middle := ui.ActiveColumnStyle.Width(middleW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(middleCol)

	var columns string
	if m.fullscreenDashboard {
		// Fullscreen dashboard: render cluster/monitoring overview using full width.
		var dashContent string
		sel := m.selectedMiddleItem()
		if sel != nil && sel.Extra == "__monitoring__" {
			dashContent = m.monitoringPreview
			if dashContent == "" {
				dashContent = ui.DimStyle.Render(m.spinner.View() + " Loading monitoring overview...")
			}
		} else {
			dashContent = m.dashboardPreview
			if dashContent == "" {
				dashContent = ui.DimStyle.Render(m.spinner.View() + " Loading cluster overview...")
			}
		}
		// Apply preview scroll.
		if m.previewScroll > 0 {
			lines := strings.Split(dashContent, "\n")
			if m.previewScroll >= len(lines) {
				m.previewScroll = len(lines) - 1
			}
			if m.previewScroll > 0 {
				lines = lines[m.previewScroll:]
			}
			dashContent = strings.Join(lines, "\n")
		}
		dashCol := padToHeight(dashContent, contentHeight)
		columns = ui.ActiveColumnStyle.Width(m.width-2).Height(contentHeight).MaxHeight(contentHeight+2).Render(dashCol)
	} else if m.fullscreenMiddle {
		columns = middle
	} else {
		leftCol := ui.RenderColumn(m.leftColumnHeader(), m.leftItems, m.parentIndex(), leftInner, contentHeight, false, m.loading, m.spinner.View(), "")
		// Clear highlight query for preview/right column — search only applies to the focus pane.
		savedHighlight := ui.ActiveHighlightQuery
		ui.ActiveHighlightQuery = ""
		rightCol := m.renderRightColumn(rightInner, contentHeight)
		ui.ActiveHighlightQuery = savedHighlight
		leftCol = padToHeight(leftCol, contentHeight)
		rightCol = padToHeight(rightCol, contentHeight)
		left := ui.InactiveColumnStyle.Width(leftW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(leftCol)
		right := ui.InactiveColumnStyle.Width(rightW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(rightCol)
		columns = lipgloss.JoinHorizontal(lipgloss.Top, left, middle, right)
	}

	// Title bar with namespace indicator on the right.
	title := m.renderTitleBar()

	// Status bar.
	status := m.statusBar()

	var parts []string
	parts = append(parts, title)
	if tabBar != "" {
		parts = append(parts, tabBar)
	}
	parts = append(parts, columns, status)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) renderTitleBar() string {
	// TitleBarStyle has Padding(0, 1) which adds 2 chars of horizontal padding.
	// The inner content area is m.width - 2.
	innerWidth := m.width - 2
	if innerWidth < 10 {
		innerWidth = 10
	}

	var watchIndicator string
	if m.watchMode {
		watchIndicator = " " + ui.HelpKeyStyle.Render("\u27f3") + " "
	}

	nsText := m.namespace
	if m.allNamespaces {
		nsText = "all"
	} else if len(m.selectedNamespaces) > 1 {
		names := make([]string, 0, len(m.selectedNamespaces))
		for ns := range m.selectedNamespaces {
			names = append(names, ns)
		}
		sort.Strings(names)
		if len(names) > 3 {
			nsText = fmt.Sprintf("%s +%d more", strings.Join(names[:3], ","), len(names)-3)
		} else {
			nsText = strings.Join(names, ",")
		}
	} else if len(m.selectedNamespaces) == 1 {
		for ns := range m.selectedNamespaces {
			nsText = ns
		}
	}
	nsLabel := ui.NamespaceBadgeStyle.Render(" ns: " + nsText + " ")

	var versionLabel string
	if m.version != "" {
		versionLabel = " " + ui.DimStyle.Render(m.version)
	}

	// Calculate available width for breadcrumb.
	fixedWidth := lipgloss.Width(watchIndicator) + lipgloss.Width(nsLabel) + lipgloss.Width(versionLabel)
	maxBcWidth := innerWidth - fixedWidth - 1 // -1 for minimum gap
	if maxBcWidth < 10 {
		maxBcWidth = 10
	}

	bcText := " " + m.breadcrumb() + " "
	if lipgloss.Width(bcText) > maxBcWidth {
		runes := []rune(bcText)
		if len(runes) > maxBcWidth-1 {
			bcText = string(runes[:maxBcWidth-2]) + "~ "
		}
	}
	bc := ui.TitleBreadcrumbStyle.Render(bcText)

	contentWidth := lipgloss.Width(bc) + lipgloss.Width(watchIndicator) + lipgloss.Width(nsLabel) + lipgloss.Width(versionLabel)
	gap := innerWidth - contentWidth
	if gap < 0 {
		gap = 0
	}

	barContent := bc + watchIndicator + strings.Repeat(" ", gap) + nsLabel + versionLabel
	return ui.TitleBarStyle.Width(m.width).Render(barContent)
}

func (m Model) viewYAML() string {
	title := ui.TitleStyle.Render(m.yamlTitle())
	yamlHints := []struct{ key, desc string }{
		{"j/k", "scroll"},
		{"g/G", "top/bottom"},
		{"ctrl+d/u", "half page"},
		{"ctrl+f/b", "page"},
		{"/", "search"},
		{"tab/z", "fold"},
		{"Z", "all folds"},
		{"e", "edit"},
		{"q/esc", "back"},
	}
	var yamlHintParts []string
	for _, h := range yamlHints {
		yamlHintParts = append(yamlHintParts, ui.HelpKeyStyle.Render(h.key)+ui.DimStyle.Render(": "+h.desc))
	}
	hint := ui.StatusBarBgStyle.Width(m.width).Render(strings.Join(yamlHintParts, ui.DimStyle.Render(" \u2502 ")))

	// If search is active, show search bar instead of hints.
	if m.yamlSearchMode {
		searchBar := ui.HelpKeyStyle.Render("/") + ui.NormalStyle.Render(m.yamlSearchText.CursorLeft()) + ui.DimStyle.Render("\u2588") + ui.NormalStyle.Render(m.yamlSearchText.CursorRight())
		hint = ui.StatusBarBgStyle.Width(m.width).Render(searchBar)
	} else if m.yamlSearchText.Value != "" {
		matchInfo := fmt.Sprintf(" [%d/%d]", m.yamlMatchIdx+1, len(m.yamlMatchLines))
		if len(m.yamlMatchLines) == 0 {
			matchInfo = " [no matches]"
		}
		searchBar := ui.HelpKeyStyle.Render("/") + ui.NormalStyle.Render(m.yamlSearchText.Value) + ui.DimStyle.Render(matchInfo)
		hint = ui.StatusBarBgStyle.Width(m.width).Render(searchBar)
	}

	maxLines := m.height - 4
	if maxLines < 3 {
		maxLines = 3
	}

	// Build visible lines with fold indicators, respecting collapsed sections.
	// Mask secret data values when secret display is toggled off.
	yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
	visLines, mapping := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)

	yamlScroll := m.yamlScroll
	if yamlScroll >= len(visLines) {
		yamlScroll = len(visLines) - 1
	}
	if yamlScroll < 0 {
		yamlScroll = 0
	}
	viewport := visLines[yamlScroll:]
	if len(viewport) > maxLines {
		viewport = viewport[:maxLines]
	}

	// Compute line number gutter width.
	totalOrigLines := len(strings.Split(m.yamlContent, "\n"))
	gutterWidth := len(fmt.Sprintf("%d", totalOrigLines))
	if gutterWidth < 2 {
		gutterWidth = 2
	}

	// Build a set of original matching lines for search highlight.
	matchSet := make(map[int]bool)
	for _, ml := range m.yamlMatchLines {
		matchSet[ml] = true
	}
	currentMatchLine := -1
	if len(m.yamlMatchLines) > 0 && m.yamlMatchIdx >= 0 && m.yamlMatchIdx < len(m.yamlMatchLines) {
		currentMatchLine = m.yamlMatchLines[m.yamlMatchIdx]
	}

	// Clamp yamlCursor to valid range.
	if m.yamlCursor < 0 {
		m.yamlCursor = 0
	}
	if m.yamlCursor >= len(visLines) {
		m.yamlCursor = len(visLines) - 1
	}
	if m.yamlCursor < 0 {
		m.yamlCursor = 0
	}

	// Apply YAML highlighting to visible lines, with search highlights and cursor.
	var highlightedLines []string
	for i, line := range viewport {
		visIdx := yamlScroll + i
		origLine := -1
		if visIdx < len(mapping) {
			origLine = mapping[visIdx]
		}
		highlighted := ui.HighlightYAMLLine(line)
		if m.yamlSearchText.Value != "" && origLine >= 0 && matchSet[origLine] {
			if origLine == currentMatchLine {
				highlighted = ui.HighlightSearchInLine(line, m.yamlSearchText.Value, true)
			} else {
				highlighted = ui.HighlightSearchInLine(line, m.yamlSearchText.Value, false)
			}
		}
		// Line number gutter
		lineNumStr := strings.Repeat(" ", gutterWidth+1)
		if origLine >= 0 {
			lineNumStr = fmt.Sprintf("%*d ", gutterWidth, origLine+1)
		}
		// Cursor indicator + line number + content
		if visIdx == m.yamlCursor {
			highlighted = ui.YamlCursorIndicatorStyle.Render("▎") + ui.DimStyle.Render(lineNumStr) + highlighted
		} else {
			highlighted = " " + ui.DimStyle.Render(lineNumStr) + highlighted
		}
		highlightedLines = append(highlightedLines, highlighted)
	}

	// Pad to fill available height so the hint bar stays at the bottom.
	for len(highlightedLines) < maxLines {
		highlightedLines = append(highlightedLines, "")
	}

	bodyContent := strings.Join(highlightedLines, "\n")
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPrimary)).
		Padding(0, 1).
		Width(m.width - 2).
		Height(maxLines)
	body := borderStyle.Render(bodyContent)

	return lipgloss.JoinVertical(lipgloss.Left, title, body, hint)
}

func (m Model) yamlTitle() string {
	switch m.nav.Level {
	case model.LevelResources:
		sel := m.selectedMiddleItem()
		if sel != nil {
			return fmt.Sprintf("YAML: %s/%s", m.namespace, sel.Name)
		}
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel != nil {
			return fmt.Sprintf("YAML: %s/%s", m.namespace, sel.Name)
		}
	case model.LevelContainers:
		return fmt.Sprintf("YAML: %s/%s", m.namespace, m.nav.OwnedName)
	}
	return "YAML"
}

func (m Model) viewLogs() string {
	viewH := m.logViewHeight()
	canSwitchPod := m.logParentKind != ""
	return ui.RenderLogViewer(m.logLines, m.logScroll, m.width, viewH, m.logFollow, m.logWrap, m.logLineNumbers, m.logTitle, m.logSearchQuery, m.logSearchInput.Value, m.logSearchActive, canSwitchPod)
}

func (m Model) viewDescribe() string {
	title := ui.TitleStyle.Render(m.describeTitle)
	hints := []struct{ key, desc string }{
		{"j/k", "scroll"},
		{"g/G", "top/bottom"},
		{"ctrl+d/u", "half page"},
		{"ctrl+f/b", "page"},
		{"q/esc", "back"},
	}
	var hintParts []string
	for _, h := range hints {
		hintParts = append(hintParts, ui.HelpKeyStyle.Render(h.key)+ui.DimStyle.Render(": "+h.desc))
	}
	hint := ui.StatusBarBgStyle.Width(m.width).Render(strings.Join(hintParts, ui.DimStyle.Render(" \u2502 ")))

	lines := strings.Split(m.describeContent, "\n")

	maxLines := m.height - 4
	if maxLines < 3 {
		maxLines = 3
	}

	scroll := m.describeScroll
	if scroll > len(lines) {
		scroll = len(lines) - 1
	}
	if scroll < 0 {
		scroll = 0
	}
	visible := lines[scroll:]
	if len(visible) > maxLines {
		visible = visible[:maxLines]
	}

	// Pad to fill available height so the hint bar stays at the bottom.
	for len(visible) < maxLines {
		visible = append(visible, "")
	}

	bodyContent := strings.Join(visible, "\n")
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPrimary)).
		Padding(0, 1).
		Width(m.width - 2).
		Height(maxLines)
	body := borderStyle.Render(bodyContent)

	return lipgloss.JoinVertical(lipgloss.Left, title, body, hint)
}

func (m Model) viewDiff() string {
	if m.diffUnified {
		return ui.RenderUnifiedDiffView(m.diffLeft, m.diffRight, m.diffLeftName, m.diffRightName, m.diffScroll, m.width, m.height, m.diffLineNumbers)
	}
	return ui.RenderDiffView(m.diffLeft, m.diffRight, m.diffLeftName, m.diffRightName, m.diffScroll, m.width, m.height, m.diffLineNumbers)
}

func (m Model) logViewHeight() int {
	h := m.height - 2 // title + footer (border is handled inside RenderLogViewer)
	if h < 3 {
		h = 3
	}
	return h
}

func (m *Model) clampLogScroll() {
	viewH := m.logViewHeight() - 2 // subtract border top + bottom
	if viewH < 1 {
		viewH = 1
	}

	var maxScroll int
	if m.logWrap {
		// When wrapping, each source line may produce multiple visual lines.
		// Walk backward from the end, accumulating visual lines until we
		// exceed viewH. The first source line that pushes us over is the
		// maximum scroll position.
		contentWidth := m.width - 4 // match logviewer.go contentWidth calculation
		if contentWidth < 10 {
			contentWidth = 10
		}
		// Account for line number gutter width.
		lineNumWidth := 0
		if m.logLineNumbers && len(m.logLines) > 0 {
			lineNumWidth = len(fmt.Sprintf("%d", len(m.logLines))) + 1
		}
		availWidth := contentWidth - lineNumWidth
		if availWidth < 10 {
			availWidth = 10
		}

		visualLines := 0
		maxScroll = len(m.logLines) // default: can scroll to end
		for i := len(m.logLines) - 1; i >= 0; i-- {
			visualLines += wrappedLineCount(m.logLines[i], availWidth)
			if visualLines >= viewH {
				maxScroll = i
				break
			}
		}
	} else {
		maxScroll = len(m.logLines) - viewH
	}
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.logScroll > maxScroll {
		m.logScroll = maxScroll
	}
	if m.logScroll < 0 {
		m.logScroll = 0
	}
}

// logMaxScroll returns the maximum valid scroll offset for the log viewer.
// It is wrap-aware when logWrap is enabled.
func (m *Model) logMaxScroll() int {
	viewH := m.logViewHeight() - 2 // subtract border top + bottom
	if viewH < 1 {
		viewH = 1
	}

	if m.logWrap {
		contentWidth := m.width - 4
		if contentWidth < 10 {
			contentWidth = 10
		}
		lineNumWidth := 0
		if m.logLineNumbers && len(m.logLines) > 0 {
			lineNumWidth = len(fmt.Sprintf("%d", len(m.logLines))) + 1
		}
		availWidth := contentWidth - lineNumWidth
		if availWidth < 10 {
			availWidth = 10
		}

		visualLines := 0
		maxScroll := len(m.logLines)
		for i := len(m.logLines) - 1; i >= 0; i-- {
			visualLines += wrappedLineCount(m.logLines[i], availWidth)
			if visualLines >= viewH {
				maxScroll = i
				break
			}
		}
		if maxScroll < 0 {
			return 0
		}
		return maxScroll
	}

	ms := len(m.logLines) - viewH
	if ms < 0 {
		return 0
	}
	return ms
}

// wrappedLineCount returns how many visual lines a source line produces
// when wrapped at the given width.
func wrappedLineCount(line string, width int) int {
	if width <= 0 {
		return 1
	}
	n := len([]rune(line))
	if n == 0 {
		return 1
	}
	return (n + width - 1) / width
}

// clampPreviewScroll prevents scrolling past the preview content.
func (m *Model) clampPreviewScroll() {
	// Render with a very large height to get the full untruncated content.
	usable := m.width - 6
	rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
	fullContent := m.renderRightColumnContent(rightW-2, 10000)
	if m.metricsContent != "" && !m.fullYAMLPreview {
		fullContent += "\n" + m.metricsContent
	}
	totalLines := strings.Count(fullContent, "\n") + 1

	visibleHeight := m.height - 4
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	maxScroll := totalLines - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.previewScroll > maxScroll {
		m.previewScroll = maxScroll
	}
}

func (m Model) renderRightColumn(width, height int) string {
	// Request extra lines to account for scroll offset so content isn't
	// truncated before the scroll slice is applied.
	renderHeight := height + m.previewScroll
	result := m.renderRightColumnContent(width, renderHeight)

	// Append resource usage metrics at the very bottom of the right pane (hide in YAML preview mode).
	if m.metricsContent != "" && !m.fullYAMLPreview {
		result += "\n" + ui.DimStyle.Render(strings.Repeat("\u2500", width)) + "\n" + m.metricsContent
	}

	// Apply preview scroll.
	if m.previewScroll > 0 {
		lines := strings.Split(result, "\n")
		if m.previewScroll >= len(lines) {
			m.previewScroll = len(lines) - 1
		}
		if m.previewScroll > 0 {
			lines = lines[m.previewScroll:]
		}
		result = strings.Join(lines, "\n")
	}

	return result
}

func (m Model) renderRightColumnContent(width, height int) string {
	contentHeight := height

	// Resource map view: show relationship tree.
	if m.mapView && m.nav.Level >= model.LevelResources {
		if m.resourceTree == nil {
			return ui.DimStyle.Render("Loading resource tree...")
		}
		return ui.RenderResourceTree(m.resourceTree, width, contentHeight)
	}

	// Full YAML preview mode (Shift+P): show only YAML, no children.
	// Only enabled on actual resources (level 2+), not on cluster/resource-type lists.
	// Exception: container level always shows container details.
	if m.fullYAMLPreview && m.nav.Level >= model.LevelResources && m.nav.Level != model.LevelContainers {
		yaml := m.previewYAML
		if yaml == "" {
			yaml = m.yamlContent
		}
		if yaml == "" {
			return ui.DimStyle.Render("Loading YAML...")
		}
		return ui.RenderYAMLContent(m.maskYAMLIfSecret(yaml), width, contentHeight)
	}

	// Default mode: show details summary (no YAML).

	// Resource types level with Overview or Monitoring selected: show dashboard in preview.
	if m.nav.Level == model.LevelResourceTypes {
		sel := m.selectedMiddleItem()
		if sel != nil && sel.Extra == "__overview__" {
			if m.dashboardPreview == "" {
				return ui.DimStyle.Render(m.spinner.View() + " Loading cluster overview...")
			}
			return m.dashboardPreview
		}
		if sel != nil && sel.Extra == "__monitoring__" {
			if m.monitoringPreview == "" {
				return ui.DimStyle.Render(m.spinner.View() + " Loading monitoring overview...")
			}
			return m.monitoringPreview
		}
	}

	// Clusters level: show resource types with category grouping in right column.
	if m.nav.Level == model.LevelClusters {
		if len(m.rightItems) == 0 {
			if m.loading {
				return ui.DimStyle.Render(m.spinner.View() + " Loading...")
			}
			return ui.DimStyle.Render("No resource types found")
		}
		return ui.RenderColumn("RESOURCE TYPE", m.rightItems, -1, width, contentHeight, false, m.loading, m.spinner.View(), "")
	}

	// Resources with children (Deployment, StatefulSet, etc.) or Pods: split view.
	if m.nav.Level == model.LevelResources && (m.resourceTypeHasChildren() || m.nav.ResourceType.Kind == "Pod") {
		if len(m.rightItems) > 0 {
			return m.renderSplitPreview(width, contentHeight)
		}
		// Fall through to show right items as list.
	}

	// Resources without children (ConfigMap, Secret, etc.): details summary only.
	if m.nav.Level == model.LevelResources && !m.resourceTypeHasChildren() && m.nav.ResourceType.Kind != "Pod" {
		sel := m.selectedMiddleItem()
		if sel != nil && len(sel.Columns) > 0 {
			return ui.RenderResourceSummary(sel, "", width, contentHeight)
		}
		// Fall back to YAML if no detail columns are available.
		return m.renderFallbackYAML(width, contentHeight)
	}

	// Owned level: pods get split view, non-pods get details summary.
	if m.nav.Level == model.LevelOwned {
		sel := m.selectedMiddleItem()
		if sel != nil {
			if sel.Kind == "Pod" {
				if len(m.rightItems) > 0 {
					return m.renderSplitPreview(width, contentHeight)
				}
				// Fall through to show right items (containers) as list.
			} else {
				// Non-pod (e.g., Job): details summary only.
				if len(sel.Columns) > 0 {
					return ui.RenderResourceSummary(sel, "", width, contentHeight)
				}
				// Fall back to YAML if no detail columns are available.
				return m.renderFallbackYAML(width, contentHeight)
			}
		}
	}

	// Container level: show container details.
	if m.nav.Level == model.LevelContainers {
		sel := m.selectedMiddleItem()
		if sel != nil {
			return ui.RenderContainerDetail(sel, width, contentHeight)
		}
	}

	// Otherwise, show right items as a list.
	if len(m.rightItems) == 0 {
		if m.loading {
			return ui.DimStyle.Render(m.spinner.View() + " Loading...")
		}
		return ui.DimStyle.Render("No resources found")
	}
	return ui.RenderTable(strings.ToUpper(m.ownedChildKindLabel()), m.rightItems, -1, width, contentHeight, m.loading, m.spinner.View(), "", false)
}

// renderSplitPreview renders the right column as a split: top children table, bottom details.
func (m Model) renderSplitPreview(width, height int) string {
	childrenHeight := (height - 2) / 3 // -2 for separator + details header
	if childrenHeight < 2 {
		childrenHeight = 2 // at least header + 1 row
	}
	detailsHeight := height - childrenHeight - 2 // separator + details header
	if detailsHeight < 1 {
		detailsHeight = 1
	}

	// Render children as table (same format as middle column).
	childLabel := strings.ToUpper(m.ownedChildKindLabel())
	childrenContent := ui.RenderTable(childLabel, m.rightItems, -1, width, childrenHeight, m.loading, m.spinner.View(), "", false)

	// Separator line.
	separator := ui.DimStyle.Render(strings.Repeat("\u2500", width))

	// Render details summary in bottom portion.
	sel := m.selectedMiddleItem()
	detailsHeader := ui.DimStyle.Bold(true).Render("DETAILS")
	var bottomContent string
	if sel != nil && len(sel.Columns) > 0 {
		bottomContent = ui.RenderResourceSummary(sel, "", width, detailsHeight)
	} else {
		// Fall back to YAML if no detail columns are available.
		yaml := m.previewYAML
		if yaml == "" {
			yaml = m.yamlContent
		}
		if yaml != "" {
			bottomContent = ui.RenderYAMLContent(m.maskYAMLIfSecret(yaml), width, detailsHeight)
		} else {
			bottomContent = ui.DimStyle.Render("No details available")
		}
	}

	return childrenContent + "\n" + separator + "\n" + detailsHeader + "\n" + bottomContent
}

// renderFallbackYAML renders YAML content when no detail columns are available for a resource.
func (m Model) renderFallbackYAML(width, height int) string {
	yaml := m.previewYAML
	if yaml == "" {
		yaml = m.yamlContent
	}
	if yaml != "" {
		return ui.RenderYAMLContent(m.maskYAMLIfSecret(yaml), width, height)
	}
	return ui.DimStyle.Render("No preview")
}

// maskYAMLIfSecret masks secret data values in YAML content when the current
// resource is a Secret and secret display is toggled off.
func (m Model) maskYAMLIfSecret(yaml string) string {
	if !m.showSecretValues && m.nav.ResourceType.Kind == "Secret" {
		return ui.MaskSecretYAML(yaml)
	}
	return yaml
}

func (m Model) resourceTypeHasChildren() bool {
	return kindHasOwnedChildren(m.nav.ResourceType.Kind)
}

// kindHasOwnedChildren reports whether a given Kubernetes resource kind can
// have child/owned resources that GetOwnedResources knows how to fetch.
// This is used both at LevelResources (to decide whether right-arrow navigates
// into owned view) and at LevelOwned (to allow nested drill-down, e.g.,
// ArgoCD Application → Deployment → Pods).
func kindHasOwnedChildren(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob",
		"Service", "Application", "HelmRelease", "Kustomization", "Node":
		return true
	default:
		return false
	}
}

// ownedItemKindLabel returns the label for the items shown in the middle column at LevelOwned.
// This reflects what the owned items *are* (e.g., Pods owned by a Deployment).
func (m Model) ownedItemKindLabel() string {
	switch m.nav.ResourceType.Kind {
	case "CronJob":
		return "Job"
	case "Application", "HelmRelease":
		return "Resource"
	case "Pod":
		return "Container"
	case "Node":
		return "Pod"
	default:
		return "Pod"
	}
}

// ownedChildKindLabel returns the label for the children of the selected owned item,
// shown in the right column (e.g., Containers within a selected Pod).
func (m Model) ownedChildKindLabel() string {
	// At the owned level, if the selected item is a Pod, right column shows containers.
	if m.nav.Level == model.LevelOwned {
		sel := m.selectedMiddleItem()
		if sel != nil && sel.Kind == "Pod" {
			return "Container"
		}
	}
	switch m.nav.ResourceType.Kind {
	case "CronJob":
		return "Job"
	case "Application", "HelmRelease":
		return "Resource"
	case "Pod":
		return "Container"
	case "Node":
		return "Pod"
	default:
		return "NAME"
	}
}

// leftColumnHeader returns the header label for the left (parent) column.
func (m Model) leftColumnHeader() string {
	switch m.nav.Level {
	case model.LevelClusters:
		return "" // no parent at top level
	case model.LevelResourceTypes:
		return "KUBECONFIG"
	case model.LevelResources:
		return "RESOURCE TYPE"
	case model.LevelOwned:
		return strings.ToUpper(m.nav.ResourceType.DisplayName)
	case model.LevelContainers:
		return strings.ToUpper(m.nav.ResourceType.DisplayName)
	default:
		return ""
	}
}

// middleColumnHeader returns the header label for the middle column.
func (m Model) middleColumnHeader() string {
	switch m.nav.Level {
	case model.LevelClusters:
		return "KUBECONFIG"
	case model.LevelResourceTypes:
		return "RESOURCE TYPE"
	case model.LevelResources:
		return strings.ToUpper(m.nav.ResourceType.Kind)
	case model.LevelOwned:
		return strings.ToUpper(m.ownedItemKindLabel())
	case model.LevelContainers:
		return "CONTAINER"
	default:
		return ""
	}
}

func (m Model) breadcrumb() string {
	parts := []string{"lfk"}
	if m.nav.Context != "" {
		parts = append(parts, m.nav.Context)
	}
	if m.nav.ResourceType.DisplayName != "" {
		parts = append(parts, m.nav.ResourceType.DisplayName)
	}
	if m.nav.ResourceName != "" {
		parts = append(parts, m.nav.ResourceName)
	}
	if m.nav.OwnedName != "" {
		parts = append(parts, m.nav.OwnedName)
	}
	return strings.Join(parts, " > ")
}

func (m Model) statusBar() string {
	// StatusBarBgStyle has Padding(0, 1) which adds 2 chars of horizontal padding.
	// Use MaxWidth on the content to prevent overflow.
	innerWidth := m.width - 2
	if innerWidth < 10 {
		innerWidth = 10
	}

	// Show command bar when active.
	if m.commandBarActive {
		prompt := ui.HelpKeyStyle.Render(":") + m.commandBarInput.CursorLeft() + ui.DimStyle.Render("█") + m.commandBarInput.CursorRight()
		if len(m.commandBarSuggestions) > 0 {
			prompt += "  "
			for i, s := range m.commandBarSuggestions {
				if i == m.commandBarSelectedSuggestion {
					prompt += ui.OverlaySelectedStyle.Render(" "+s+" ") + " "
				} else {
					prompt += ui.DimStyle.Render(s) + " "
				}
			}
		}
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).Render(prompt)
	}

	// Show filter/search input in status bar when active.
	if m.filterActive {
		prompt := ui.HelpKeyStyle.Render("filter") + ui.DimStyle.Render(": ") + m.filterInput.CursorLeft() + ui.DimStyle.Render("\u2588") + m.filterInput.CursorRight()
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).Render(prompt)
	}
	if m.searchActive {
		prompt := ui.HelpKeyStyle.Render("search") + ui.DimStyle.Render(": ") + m.searchInput.CursorLeft() + ui.DimStyle.Render("\u2588") + m.searchInput.CursorRight()
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).Render(prompt)
	}

	// When a status message is active, show it exclusively (hide key hints).
	if m.hasStatusMessage() {
		msg := m.sanitizeMessage(m.statusMessage)
		var styled string
		if m.statusMessageErr {
			styled = ui.StatusMessageErrStyle.Render(msg)
		} else {
			styled = ui.StatusMessageOkStyle.Render(msg)
		}
		styled = ui.Truncate(styled, innerWidth)
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(styled)
	}

	var parts []string

	// Show selection count when items are selected.
	if m.hasSelection() {
		parts = append(parts, ui.SelectionCountStyle.Render(fmt.Sprintf(" %d selected ", len(m.selectedItems))))
	}

	// Show active filter preset indicator.
	if m.activeFilterPreset != nil {
		parts = append(parts, ui.HelpKeyStyle.Render("[filter: "+m.activeFilterPreset.Name+"]"))
	}

	visible := m.visibleMiddleItems()
	total := len(m.middleItems)
	cur := m.cursor() + 1

	if m.filterText != "" {
		parts = append(parts, ui.DimStyle.Render(fmt.Sprintf("[%d/%d filtered: %d/%d]", cur, len(visible), len(visible), total)))
	} else {
		parts = append(parts, ui.DimStyle.Render(fmt.Sprintf("[%d/%d]", cur, total)))
	}

	// Sort mode indicator.
	parts = append(parts, ui.DimStyle.Render("sort:"+m.sortModeName()))

	// Styled key hints.
	hints := []struct{ key, desc string }{
		{"h/l", "navigate"},
		{"j/k", "move"},
		{"enter", "view"},
		{"\\", "namespace"},
		{"A", "all-ns"},
		{"x", "actions"},
		{"a", "create"},
		{",", "sort"},
		{"f", "filter"},
		{"b/B", "bookmarks"},
		{"?", "help"},
		{"q", "quit"},
	}
	var hintParts []string
	for _, h := range hints {
		hintParts = append(hintParts, ui.HelpKeyStyle.Render(h.key)+ui.DimStyle.Render(": "+h.desc))
	}
	parts = append(parts, strings.Join(hintParts, ui.DimStyle.Render(" \u2502 ")))

	content := strings.Join(parts, "  ")
	return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(content)
}

// --- Overlay rendering ---

func (m Model) renderOverlay(background string) string {
	var content string
	var overlayW, overlayH int

	switch m.overlay {
	case overlayNamespace:
		content = ui.RenderNamespaceOverlay(m.filteredOverlayItems(), m.overlayFilter.Value, m.overlayCursor, m.namespace, m.allNamespaces, m.selectedNamespaces, m.nsFilterMode)
		overlayW = min(60, m.width-10)
		overlayH = min(20, m.height-6)
	case overlayAction:
		overlayW = min(70, m.width-10)
		content = ui.RenderActionOverlay(m.overlayItems, m.overlayCursor, overlayW)
		overlayH = min(15, m.height-6)
	case overlayConfirm:
		content = ui.RenderConfirmOverlay(m.confirmAction)
		overlayW = min(50, m.width-10)
		overlayH = min(8, m.height-6)
	case overlayScaleInput:
		content = ui.RenderScaleOverlay(m.scaleInput.Value)
		overlayW = min(45, m.width-10)
		overlayH = min(8, m.height-6)
	case overlayPortForward:
		content = ui.RenderPortForwardOverlay(m.portForwardInput.Value, m.pfAvailablePorts, m.pfPortCursor, m.actionCtx.name)
		overlayW = min(55, m.width-10)
		overlayH = min(5+len(m.pfAvailablePorts)+4, m.height-6)
	case overlayContainerSelect:
		content = ui.RenderContainerSelectOverlay(m.overlayItems, m.overlayCursor)
		overlayW = min(50, m.width-10)
		overlayH = min(15, m.height-6)
	case overlayPodSelect:
		content = ui.RenderPodSelectOverlay(m.overlayItems, m.overlayCursor)
		overlayW = min(60, m.width-10)
		overlayH = min(20, m.height-6)
	case overlayBookmarks:
		content = ui.RenderBookmarkOverlay(m.bookmarks, m.bookmarkFilter.Value, m.overlayCursor, int(m.bookmarkSearchMode))
		overlayW = min(70, m.width-10)
		overlayH = min(20, m.height-6)
	case overlayTemplates:
		content = ui.RenderTemplateOverlay(m.templateItems, m.templateCursor)
		overlayW = min(60, m.width-10)
		overlayH = min(25, m.height-6)
	case overlayColorscheme:
		content = ui.RenderColorschemeOverlay(m.schemeEntries, m.schemeFilter.Value, m.schemeCursor, m.schemeFilterMode)
		overlayW = min(50, m.width-10)
		overlayH = min(22, m.height-6)
	case overlayFilterPreset:
		var activePresetName string
		if m.activeFilterPreset != nil {
			activePresetName = m.activeFilterPreset.Name
		}
		entries := make([]ui.FilterPresetEntry, len(m.filterPresets))
		for i, p := range m.filterPresets {
			entries[i] = ui.FilterPresetEntry{Name: p.Name, Description: p.Description, Key: p.Key}
		}
		content = ui.RenderFilterPresetOverlay(entries, m.overlayCursor, activePresetName)
		overlayW = min(55, m.width-10)
		overlayH = min(15, m.height-6)
	case overlayRBAC:
		entries := make([]ui.RBACCheckEntry, len(m.rbacResults))
		for i, r := range m.rbacResults {
			entries[i] = ui.RBACCheckEntry{Verb: r.Verb, Allowed: r.Allowed}
		}
		content = ui.RenderRBACOverlay(entries, m.rbacKind)
		overlayW = min(45, m.width-10)
		overlayH = min(15, m.height-6)
	case overlayBatchLabel:
		content = ui.RenderBatchLabelOverlay(m.batchLabelMode, m.batchLabelInput.Value, m.batchLabelRemove)
		overlayW = min(50, m.width-10)
		overlayH = min(12, m.height-6)
	case overlayPodStartup:
		if m.podStartupData != nil {
			entry := ui.PodStartupEntry{
				PodName:   m.podStartupData.PodName,
				Namespace: m.podStartupData.Namespace,
				TotalTime: m.podStartupData.TotalTime,
			}
			for _, p := range m.podStartupData.Phases {
				entry.Phases = append(entry.Phases, ui.StartupPhaseEntry{
					Name:     p.Name,
					Duration: p.Duration,
					Status:   p.Status,
				})
			}
			content = ui.RenderPodStartupOverlay(entry)
		}
		overlayW = min(70, m.width-10)
		overlayH = min(25, m.height-6)
	case overlayQuotaDashboard:
		entries := make([]ui.QuotaEntry, len(m.quotaData))
		for i, q := range m.quotaData {
			resources := make([]ui.QuotaResourceEntry, len(q.Resources))
			for j, r := range q.Resources {
				resources[j] = ui.QuotaResourceEntry{
					Name:    r.Name,
					Hard:    r.Hard,
					Used:    r.Used,
					Percent: r.Percent,
				}
			}
			entries[i] = ui.QuotaEntry{
				Name:      q.Name,
				Namespace: q.Namespace,
				Resources: resources,
			}
		}
		content = ui.RenderQuotaDashboardOverlay(entries, min(80, m.width-10), min(30, m.height-6))
		overlayW = min(80, m.width-10)
		overlayH = min(30, m.height-6)
	case overlayEventTimeline:
		entries := make([]ui.EventTimelineEntry, len(m.eventTimelineData))
		for i, e := range m.eventTimelineData {
			entries[i] = ui.EventTimelineEntry{
				Timestamp:    e.Timestamp,
				Type:         e.Type,
				Reason:       e.Reason,
				Message:      e.Message,
				Source:       e.Source,
				Count:        e.Count,
				InvolvedName: e.InvolvedName,
				InvolvedKind: e.InvolvedKind,
			}
		}
		overlayW = min(100, m.width-6)
		overlayH = min(30, m.height-4)
		content = ui.RenderEventTimelineOverlay(entries, m.actionCtx.name, m.eventTimelineScroll, overlayW, overlayH)
	case overlayAlerts:
		entries := make([]ui.AlertEntry, len(m.alertsData))
		for i, a := range m.alertsData {
			entries[i] = ui.AlertEntry{
				Name:        a.Name,
				State:       a.State,
				Severity:    a.Severity,
				Summary:     a.Summary,
				Description: a.Description,
				Since:       a.Since,
				GrafanaURL:  a.GrafanaURL,
			}
		}
		content = ui.RenderAlertsOverlay(entries, m.alertsScroll, m.width-10, m.height-6)
		overlayW = min(80, m.width-10)
		overlayH = min(25, m.height-6)
	case overlayNetworkPolicy:
		if m.netpolData != nil {
			entry := ui.NetworkPolicyEntry{
				Name:        m.netpolData.Name,
				Namespace:   m.netpolData.Namespace,
				PodSelector: m.netpolData.PodSelector,
				PolicyTypes: m.netpolData.PolicyTypes,
				AffectedPods: m.netpolData.AffectedPods,
			}
			for _, r := range m.netpolData.IngressRules {
				re := ui.NetpolRuleEntry{}
				for _, p := range r.Ports {
					re.Ports = append(re.Ports, ui.NetpolPortEntry{Protocol: p.Protocol, Port: p.Port})
				}
				for _, p := range r.Peers {
					re.Peers = append(re.Peers, ui.NetpolPeerEntry{
						Type: p.Type, Selector: p.Selector,
						CIDR: p.CIDR, Except: p.Except, Namespace: p.Namespace,
					})
				}
				entry.IngressRules = append(entry.IngressRules, re)
			}
			for _, r := range m.netpolData.EgressRules {
				re := ui.NetpolRuleEntry{}
				for _, p := range r.Ports {
					re.Ports = append(re.Ports, ui.NetpolPortEntry{Protocol: p.Protocol, Port: p.Port})
				}
				for _, p := range r.Peers {
					re.Peers = append(re.Peers, ui.NetpolPeerEntry{
						Type: p.Type, Selector: p.Selector,
						CIDR: p.CIDR, Except: p.Except, Namespace: p.Namespace,
					})
				}
				entry.EgressRules = append(entry.EgressRules, re)
			}
			overlayW = min(100, m.width-6)
			overlayH = min(35, m.height-4)
			// Netpol overlay renders its own hint bar, so we build the
			// bordered box here to avoid OverlayStyle.Height() clipping it.
			innerW := overlayW - 4 // account for OverlayStyle Padding(1,2) = 2 chars each side
			innerH := overlayH - 2 // account for OverlayStyle Padding(1,2) = 1 line top + 1 line bottom
			netpolContent := ui.RenderNetworkPolicyOverlay(entry, m.netpolScroll, innerW, innerH)
			overlay := ui.OverlayStyle.Width(overlayW).Render(netpolContent)
			bg := padToHeight(background, m.height)
			return ui.PlaceOverlay(m.width, m.height, overlay, bg)
		}
	case overlaySecretEditor:
		overlay := ui.RenderSecretEditorOverlay(
			m.secretData, m.secretCursor, m.secretRevealed, m.secretAllRevealed,
			m.secretEditing, m.secretEditKey.Value, m.secretEditValue.Value, m.secretEditColumn,
			m.width, m.height,
		)
		bg := padToHeight(background, m.height)
		return ui.PlaceOverlay(m.width, m.height, overlay, bg)
	case overlayConfigMapEditor:
		overlay := ui.RenderConfigMapEditorOverlay(
			m.configMapData, m.configMapCursor,
			m.configMapEditing, m.configMapEditKey.Value, m.configMapEditValue.Value, m.configMapEditColumn,
			m.width, m.height,
		)
		bg := padToHeight(background, m.height)
		return ui.PlaceOverlay(m.width, m.height, overlay, bg)
	case overlayRollback:
		overlay := ui.RenderRollbackOverlay(m.rollbackRevisions, m.rollbackCursor, m.width, m.height)
		bg := padToHeight(background, m.height)
		return ui.PlaceOverlay(m.width, m.height, overlay, bg)
	case overlayHelmRollback:
		overlay := ui.RenderHelmRollbackOverlay(m.helmRollbackRevisions, m.helmRollbackCursor, m.width, m.height)
		bg := padToHeight(background, m.height)
		return ui.PlaceOverlay(m.width, m.height, overlay, bg)
	case overlayLabelEditor:
		overlay := ui.RenderLabelEditorOverlay(
			m.labelData, m.labelCursor, m.labelTab,
			m.labelEditing, m.labelEditKey.Value, m.labelEditValue.Value, m.labelEditColumn,
			m.width, m.height,
		)
		bg := padToHeight(background, m.height)
		return ui.PlaceOverlay(m.width, m.height, overlay, bg)
	default:
		return background
	}

	if overlayW < 10 {
		overlayW = 10
	}
	if overlayH < 3 {
		overlayH = 3
	}

	overlay := ui.OverlayStyle.Width(overlayW).Height(overlayH).Render(content)

	// Ensure background has exactly m.height lines for correct overlay placement.
	bg := padToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}

// renderErrorLogOverlay renders the error log overlay on top of the given background.
func (m Model) renderErrorLogOverlay(background string) string {
	overlayW := min(140, m.width-4)
	overlayH := min(30, m.height-4)
	if overlayW < 10 {
		overlayW = 10
	}
	if overlayH < 3 {
		overlayH = 3
	}

	content := ui.RenderErrorLogOverlay(m.errorLog, m.errorLogScroll, overlayH, m.showDebugLogs)
	overlay := ui.OverlayStyle.Width(overlayW).Height(overlayH).Render(content)
	bg := padToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}

// --- Model helper methods ---

// parentIndex returns the index of the parent item in leftItems, or -1 if none.
func (m *Model) parentIndex() int {
	var parentName string
	switch m.nav.Level {
	case model.LevelResourceTypes:
		parentName = m.nav.Context
	case model.LevelResources:
		if m.nav.ResourceType.DisplayName != "" {
			parentName = m.nav.ResourceType.DisplayName
		}
	case model.LevelOwned:
		parentName = m.nav.ResourceName
	case model.LevelContainers:
		parentName = m.nav.OwnedName
	default:
		return -1
	}
	for i, item := range m.leftItems {
		if item.Name == parentName {
			return i
		}
	}
	return -1
}

// cursor returns the cursor position for the current level.
func (m *Model) cursor() int {
	return m.cursors[m.nav.Level]
}

// setCursor sets the cursor for the current level.
func (m *Model) setCursor(v int) {
	m.cursors[m.nav.Level] = v
}

// clampCursor ensures the cursor is within bounds for visible (filtered) middleItems.
func (m *Model) clampCursor() {
	c := m.cursor()
	if c < 0 {
		c = 0
	}
	visible := m.visibleMiddleItems()
	if len(visible) > 0 && c >= len(visible) {
		c = len(visible) - 1
	}
	m.setCursor(c)
}

// carryOverMetricsColumns copies metrics columns (CPU, CPU/R, CPU/L, MEM, MEM/R, MEM/L)
// from existing middle items to new items by matching on name+namespace.
// This prevents blinking during watch mode refreshes while metrics load async.
// Only carries over if actual usage data exists (CPU/MEM have real values).
func (m *Model) carryOverMetricsColumns(newItems []model.Item) {
	metricsKeys := map[string]bool{
		"CPU": true, "CPU/R": true, "CPU/L": true,
		"MEM": true, "MEM/R": true, "MEM/L": true,
		"CPU%": true, "MEM%": true,
	}
	// Build lookup from old items — only if they have real usage data.
	type itemKey struct{ ns, name string }
	oldMetrics := make(map[itemKey][]model.KeyValue)
	for _, item := range m.middleItems {
		var cols []model.KeyValue
		hasUsage := false
		for _, kv := range item.Columns {
			if metricsKeys[kv.Key] {
				cols = append(cols, kv)
				if (kv.Key == "CPU" || kv.Key == "MEM") && kv.Value != "" && kv.Value != "0" && kv.Value != "0m" && kv.Value != "0B" {
					hasUsage = true
				}
			}
		}
		if hasUsage && len(cols) > 0 {
			oldMetrics[itemKey{item.Namespace, item.Name}] = cols
		}
	}
	if len(oldMetrics) == 0 {
		return
	}
	// Apply to new items: prepend carried-over metrics columns while keeping
	// the raw request/limit columns (CPU Req, CPU Lim, Mem Req, Mem Lim) so
	// that podMetricsEnrichedMsg can still read them to compute percentages.
	for i := range newItems {
		key := itemKey{newItems[i].Namespace, newItems[i].Name}
		cols, ok := oldMetrics[key]
		if !ok {
			continue
		}
		var kept []model.KeyValue
		for _, kv := range newItems[i].Columns {
			if !metricsKeys[kv.Key] {
				kept = append(kept, kv)
			}
		}
		newItems[i].Columns = append(cols, kept...)
	}
}

// clampAllCursors ensures all cursor positions are within bounds after resize.
func (m *Model) clampAllCursors() {
	m.clampCursor()
}

// navKey builds a unique key from the current navigation state, used for
// cursor memory and item caching.
func (m *Model) navKey() string {
	parts := []string{m.nav.Context}
	if m.nav.ResourceType.Resource != "" {
		parts = append(parts, m.nav.ResourceType.Resource)
	}
	if m.nav.ResourceName != "" {
		parts = append(parts, m.nav.ResourceName)
	}
	if m.nav.OwnedName != "" {
		parts = append(parts, m.nav.OwnedName)
	}
	return strings.Join(parts, "/")
}

// saveCursor stores the current cursor position keyed by navigation path.
func (m *Model) saveCursor() {
	m.cursorMemory[m.navKey()] = m.cursor()
}

// restoreCursor restores the cursor position from memory for the current
// navigation path, or resets to 0 if no saved position exists.
func (m *Model) restoreCursor() {
	if pos, ok := m.cursorMemory[m.navKey()]; ok {
		m.setCursor(pos)
		m.clampCursor()
		return
	}
	m.setCursor(0)
}

// selectedMiddleItem returns the currently selected item in the middle column,
// taking into account any active filter.
func (m *Model) selectedMiddleItem() *model.Item {
	visible := m.visibleMiddleItems()
	c := m.cursor()
	if c >= 0 && c < len(visible) {
		// Return a pointer to the item in middleItems (not the filtered copy).
		target := visible[c]
		for i := range m.middleItems {
			if m.middleItems[i].Name == target.Name &&
				m.middleItems[i].Kind == target.Kind &&
				m.middleItems[i].Extra == target.Extra &&
				m.middleItems[i].Namespace == target.Namespace {
				return &m.middleItems[i]
			}
		}
		// Fallback: return the filtered item directly.
		return &visible[c]
	}
	return nil
}

// selectionKey generates a unique key for an item used in the selectedItems map.
func selectionKey(item model.Item) string {
	if item.Namespace != "" {
		return item.Namespace + "/" + item.Name
	}
	return item.Name
}

// isSelected returns true if the given item is in the multi-selection set.
func (m *Model) isSelected(item model.Item) bool {
	return m.selectedItems[selectionKey(item)]
}

// toggleSelection toggles the selection state of an item.
func (m *Model) toggleSelection(item model.Item) {
	key := selectionKey(item)
	if m.selectedItems[key] {
		delete(m.selectedItems, key)
	} else {
		m.selectedItems[key] = true
	}
}

// clearSelection removes all items from the multi-selection set.
func (m *Model) clearSelection() {
	m.selectedItems = make(map[string]bool)
}

// hasSelection returns true if any items are selected.
func (m *Model) hasSelection() bool {
	return len(m.selectedItems) > 0
}

// selectedItemsList returns the list of currently selected items from visibleMiddleItems.
func (m *Model) selectedItemsList() []model.Item {
	var selected []model.Item
	for _, item := range m.visibleMiddleItems() {
		if m.isSelected(item) {
			selected = append(selected, item)
		}
	}
	return selected
}

// visibleMiddleItems returns the filtered subset of middleItems when a filter
// is active, or all middleItems otherwise. At LevelResourceTypes, it also
// applies collapsible group logic (accordion behavior).
func (m *Model) visibleMiddleItems() []model.Item {
	items := m.middleItems

	// Apply text filter first.
	if m.filterText != "" {
		filter := strings.ToLower(m.filterText)

		// First pass: determine which categories match the filter.
		matchedCategories := make(map[string]bool)
		for _, item := range items {
			if item.Category != "" && strings.Contains(strings.ToLower(item.Category), filter) {
				matchedCategories[item.Category] = true
			}
		}

		// Second pass: include items that match by name OR belong to a matched category.
		var filtered []model.Item
		for _, item := range items {
			if item.Category != "" && matchedCategories[item.Category] {
				filtered = append(filtered, item)
				continue
			}
			searchText := item.Name
			if item.Namespace != "" {
				searchText = item.Namespace + "/" + searchText
			}
			if item.Kind != "" {
				searchText += " " + item.Kind
			}
			if item.Extra != "" {
				searchText += " " + item.Extra
			}
			if strings.Contains(strings.ToLower(searchText), filter) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	// Apply collapsible group logic at LevelResourceTypes.
	if m.nav.Level == model.LevelResourceTypes && !m.allGroupsExpanded {
		var collapsed []model.Item
		seenCategories := make(map[string]bool)
		for _, item := range items {
			// Items with no category (e.g., Overview) are always shown.
			if item.Category == "" {
				collapsed = append(collapsed, item)
				continue
			}
			if item.Category == m.expandedGroup {
				// Expanded group: show all items.
				collapsed = append(collapsed, item)
				seenCategories[item.Category] = true
			} else if !seenCategories[item.Category] {
				// Collapsed group: insert a placeholder (header-only, no item line).
				seenCategories[item.Category] = true
				collapsed = append(collapsed, model.Item{
					Name:     item.Category,
					Kind:     "__collapsed_group__",
					Category: item.Category,
				})
			}
		}
		items = collapsed
	}

	return items
}

// categoryCounts returns the number of items in each category from the full
// (unfiltered, uncollapsed) middleItems list. Used for rendering collapsed
// group headers with item counts.
func (m *Model) categoryCounts() map[string]int {
	counts := make(map[string]int)
	for _, item := range m.middleItems {
		if item.Category != "" {
			counts[item.Category]++
		}
	}
	return counts
}

// syncExpandedGroup updates the expanded group to match the category of the
// item currently under the cursor. This is used after cursor jumps (g/G) and
// when navigating back to LevelResourceTypes.
func (m *Model) syncExpandedGroup() {
	if m.nav.Level != model.LevelResourceTypes || m.allGroupsExpanded {
		return
	}
	visible := m.visibleMiddleItems()
	c := m.cursor()
	if c >= len(visible) {
		c = len(visible) - 1
		m.setCursor(c)
	}
	if c >= 0 && c < len(visible) {
		cat := visible[c].Category
		if cat != "" && cat != m.expandedGroup {
			m.expandedGroup = cat
			// Recompute and find the first real item of this category.
			newVisible := m.visibleMiddleItems()
			for i, item := range newVisible {
				if item.Category == cat && item.Kind != "__collapsed_group__" {
					m.setCursor(i)
					return
				}
			}
			m.clampCursor()
		}
	}
}

// filteredOverlayItems returns overlay items matching the current filter.
func (m *Model) filteredOverlayItems() []model.Item {
	if m.overlayFilter.Value == "" {
		return m.overlayItems
	}
	var filtered []model.Item
	filter := strings.ToLower(m.overlayFilter.Value)
	for _, item := range m.overlayItems {
		if strings.Contains(strings.ToLower(item.Name), filter) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// clampOverlayCursor ensures the overlay cursor is within bounds.
func (m *Model) clampOverlayCursor() {
	items := m.filteredOverlayItems()
	if m.overlayCursor < 0 {
		m.overlayCursor = 0
	}
	if len(items) > 0 && m.overlayCursor >= len(items) {
		m.overlayCursor = len(items) - 1
	}
}

// pushLeft saves the current leftItems and promotes middleItems to become the new leftItems.
func (m *Model) pushLeft() {
	m.leftItemsHistory = append(m.leftItemsHistory, m.leftItems)
	m.leftItems = m.middleItems
}

// popLeft restores leftItems from the history stack.
func (m *Model) popLeft() {
	n := len(m.leftItemsHistory)
	if n > 0 {
		m.leftItems = m.leftItemsHistory[n-1]
		m.leftItemsHistory = m.leftItemsHistory[:n-1]
	} else {
		m.leftItems = nil
	}
}

// clearRight resets the right column and YAML preview so stale data doesn't linger.
func (m *Model) clearRight() {
	m.rightItems = nil
	m.yamlContent = ""
	m.yamlSections = nil
	m.previewYAML = ""
	m.metricsContent = ""
	m.resourceTree = nil
	m.mapView = false
}

// selectedResourceKind returns the Kind of the currently selected resource,
// which is context-dependent on the navigation level.
func (m *Model) selectedResourceKind() string {
	switch m.nav.Level {
	case model.LevelResources:
		return m.nav.ResourceType.Kind
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel != nil {
			return sel.Kind
		}
	case model.LevelContainers:
		return "Container"
	}
	return ""
}

// effectiveNamespace returns the namespace to use for API calls.
// Returns empty string when allNamespaces is true or multiple namespaces are
// selected (fetches all, filters client-side).
func (m *Model) effectiveNamespace() string {
	if m.allNamespaces || len(m.selectedNamespaces) > 1 {
		return "" // fetch all, filter client-side
	}
	if len(m.selectedNamespaces) == 1 {
		for ns := range m.selectedNamespaces {
			return ns
		}
	}
	return m.namespace
}

// sortMiddleItems sorts middleItems based on the current sort mode.
// At LevelResourceTypes and LevelClusters, items keep their original ordering.
func (m *Model) sortMiddleItems() {
	if m.nav.Level == model.LevelResourceTypes || m.nav.Level == model.LevelClusters {
		return
	}
	switch m.sortBy {
	case sortByName:
		sort.Slice(m.middleItems, func(i, j int) bool {
			return m.middleItems[i].Name < m.middleItems[j].Name
		})
	case sortByAge:
		sort.Slice(m.middleItems, func(i, j int) bool {
			if m.middleItems[i].CreatedAt.IsZero() && m.middleItems[j].CreatedAt.IsZero() {
				return m.middleItems[i].Name < m.middleItems[j].Name
			}
			if m.middleItems[i].CreatedAt.IsZero() {
				return false
			}
			if m.middleItems[j].CreatedAt.IsZero() {
				return true
			}
			return m.middleItems[i].CreatedAt.After(m.middleItems[j].CreatedAt)
		})
	case sortByStatus:
		sort.Slice(m.middleItems, func(i, j int) bool {
			pi := statusPriority(m.middleItems[i].Status)
			pj := statusPriority(m.middleItems[j].Status)
			if pi != pj {
				return pi < pj
			}
			return m.middleItems[i].Name < m.middleItems[j].Name
		})
	}
}

// statusPriority returns a sort priority for a status string.
func statusPriority(status string) int {
	switch status {
	case "Running", "Active", "Bound", "Available", "Ready", "Healthy", "Healthy/Synced", "Deployed":
		return 0
	case "Pending", "ContainerCreating", "Waiting", "Init", "Progressing", "Progressing/Synced", "Suspended",
		"Pending-install", "Pending-upgrade", "Pending-rollback", "Uninstalling":
		return 1
	case "Failed", "CrashLoopBackOff", "Error", "ImagePullBackOff", "Degraded", "Degraded/OutOfSync":
		return 2
	default:
		return 3
	}
}

// sortModeName returns a display name for the current sort mode.
func (m *Model) sortModeName() string {
	switch m.sortBy {
	case sortByName:
		return "name"
	case sortByAge:
		return "age"
	case sortByStatus:
		return "status"
	}
	return "name"
}

// sanitizeError strips newlines and truncates an error message for status bar display.
func (m *Model) sanitizeError(err error) string {
	s := strings.ReplaceAll(err.Error(), "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	// Collapse multiple spaces.
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	maxLen := m.width - 20
	if maxLen < 40 {
		maxLen = 40
	}
	if len(s) > maxLen {
		s = s[:maxLen-3] + "..."
	}
	return s
}

// fullErrorMessage returns the full error message with newlines collapsed, for logging.
func fullErrorMessage(err error) string {
	s := strings.ReplaceAll(err.Error(), "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

// sanitizeMessage strips newlines and truncates a string for status bar display.
func (m *Model) sanitizeMessage(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	maxLen := m.width - 6 // account for status bar padding
	if maxLen < 40 {
		maxLen = 40
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		s = string(runes[:maxLen-3]) + "..."
	}
	return s
}

// deleteWordBackward removes the last word (or trailing whitespace) from s.
func deleteWordBackward(s string) string {
	if s == "" {
		return s
	}
	// Trim trailing spaces first.
	i := len(s) - 1
	for i >= 0 && s[i] == ' ' {
		i--
	}
	// Then trim the word.
	for i >= 0 && s[i] != ' ' {
		i--
	}
	return s[:i+1]
}

// setStatusMessage sets a temporary status bar message.
// All messages are appended to the application log buffer with appropriate level.
func (m *Model) setStatusMessage(msg string, isErr bool) {
	m.statusMessage = msg
	m.statusMessageErr = isErr
	m.statusMessageExp = time.Now().Add(5 * time.Second)

	level := "INF"
	if isErr {
		level = "ERR"
		logger.Error("Application error", "message", msg)
	} else {
		logger.Info("Status message", "message", msg)
	}
	m.errorLog = append(m.errorLog, ui.ErrorLogEntry{
		Time:    time.Now(),
		Message: msg,
		Level:   level,
	})
	// Keep at most 200 entries (drop oldest).
	if len(m.errorLog) > 200 {
		m.errorLog = m.errorLog[len(m.errorLog)-200:]
	}
}

// setErrorFromErr shows a sanitized error in the status bar and logs the
// full untruncated error to the error log overlay.
func (m *Model) setErrorFromErr(prefix string, err error) {
	// Show truncated version in status bar.
	m.statusMessage = prefix + m.sanitizeError(err)
	m.statusMessageErr = true
	m.statusMessageExp = time.Now().Add(5 * time.Second)

	// Log the full untruncated error to the error log.
	full := fullErrorMessage(err)
	logger.Error("Application error", "message", full)
	m.errorLog = append(m.errorLog, ui.ErrorLogEntry{
		Time:    time.Now(),
		Message: prefix + full,
		Level:   "ERR",
	})
	if len(m.errorLog) > 200 {
		m.errorLog = m.errorLog[len(m.errorLog)-200:]
	}
}

// hasStatusMessage checks whether there's a non-expired status message.
func (m *Model) hasStatusMessage() bool {
	return m.statusMessage != "" && time.Now().Before(m.statusMessageExp)
}

// addLogEntry appends an entry to the in-app error log at the given level.
func (m *Model) addLogEntry(level, msg string) {
	m.errorLog = append(m.errorLog, ui.ErrorLogEntry{
		Time:    time.Now(),
		Message: msg,
		Level:   level,
	})
	if len(m.errorLog) > 500 {
		m.errorLog = m.errorLog[len(m.errorLog)-500:]
	}
}

// logDebug adds a debug entry to the in-app log and file logger.
func (m *Model) logDebug(msg string) {
	logger.Debug(msg)
	m.addLogEntry("DBG", msg)
}

// portForwardItems returns the list of active port forwards as model.Items for display.
func (m *Model) portForwardItems() []model.Item {
	entries := m.portForwardMgr.Entries()
	items := make([]model.Item, 0, len(entries))
	for _, e := range entries {
		displayLocalPort := e.LocalPort
		if displayLocalPort == "0" {
			displayLocalPort = "..."
		}
		name := fmt.Sprintf("%s/%s  %s:%s", e.ResourceKind, e.ResourceName, displayLocalPort, e.RemotePort)
		extra := fmt.Sprintf("%s/%s", e.Namespace, e.Context)
		status := string(e.Status)
		age := time.Since(e.StartedAt).Truncate(time.Second).String()

		items = append(items, model.Item{
			Name:      name,
			Namespace: e.Namespace,
			Status:    status,
			Kind:      "__port_forward_entry__",
			Extra:     extra,
			Age:       age,
			CreatedAt: e.StartedAt,
			Columns: []model.KeyValue{
				{Key: "ID", Value: fmt.Sprintf("%d", e.ID)},
				{Key: "Local", Value: displayLocalPort},
				{Key: "Remote", Value: e.RemotePort},
				{Key: "Resource", Value: e.ResourceKind + "/" + e.ResourceName},
				{Key: "Status", Value: status},
			},
		})
	}
	return items
}

// navigateToPortForwards switches the view to the Port Forwards resource list.
// If pfLastCreatedID is set, the cursor is placed on the matching entry.
func (m *Model) navigateToPortForwards() {
	// Build the correct left column state for LevelResources.
	contexts, _ := m.client.GetContexts()
	var resourceTypes []model.Item
	if crds := m.discoveredCRDs[m.nav.Context]; len(crds) > 0 {
		resourceTypes = model.MergeWithCRDs(crds)
	} else {
		resourceTypes = model.FlattenedResourceTypes()
	}

	m.nav.ResourceType = model.ResourceTypeEntry{
		DisplayName: "Port Forwards",
		Kind:        "__port_forwards__",
		APIGroup:    "_portforward",
		APIVersion:  "v1",
		Resource:    "portforwards",
		Namespaced:  false,
	}
	m.nav.Level = model.LevelResources
	m.leftItemsHistory = [][]model.Item{contexts}
	m.leftItems = resourceTypes
	m.clearRight()
	m.middleItems = m.portForwardItems()
	m.setCursor(0)
	// Try to position cursor on the newly created port forward.
	if m.pfLastCreatedID > 0 {
		for i, item := range m.middleItems {
			if m.getPortForwardID(item.Columns) == m.pfLastCreatedID {
				m.setCursor(i)
				break
			}
		}
	}
	m.clampCursor()
	m.saveCurrentSession()
}

// getPortForwardID extracts the port forward ID from item columns.
func (m *Model) getPortForwardID(columns []model.KeyValue) int {
	for _, kv := range columns {
		if kv.Key == "ID" {
			id, err := strconv.Atoi(kv.Value)
			if err == nil {
				return id
			}
		}
	}
	return 0
}

// tabLabels builds a display label for each tab.
func (m Model) tabLabels() []string {
	labels := make([]string, len(m.tabs))
	for i, t := range m.tabs {
		if t.nav.Context != "" {
			label := t.nav.Context
			if t.nav.ResourceType.DisplayName != "" {
				label += "/" + t.nav.ResourceType.DisplayName
			}
			labels[i] = label
		} else {
			labels[i] = "clusters"
		}
	}
	// Update current tab label from live model state.
	if m.nav.Context != "" {
		label := m.nav.Context
		if m.nav.ResourceType.DisplayName != "" {
			label += "/" + m.nav.ResourceType.DisplayName
		}
		labels[m.activeTab] = label
	} else {
		labels[m.activeTab] = "clusters"
	}
	return labels
}

// saveCurrentTab persists Model fields into the current TabState.
func (m *Model) saveCurrentTab() {
	t := &m.tabs[m.activeTab]
	t.nav = m.nav
	t.leftItems = append([]model.Item(nil), m.leftItems...)
	t.middleItems = append([]model.Item(nil), m.middleItems...)
	t.rightItems = append([]model.Item(nil), m.rightItems...)
	// Deep copy leftItemsHistory.
	t.leftItemsHistory = make([][]model.Item, len(m.leftItemsHistory))
	for i, hist := range m.leftItemsHistory {
		t.leftItemsHistory[i] = append([]model.Item(nil), hist...)
	}
	t.cursors = m.cursors
	t.cursorMemory = copyMapStringInt(m.cursorMemory)
	t.itemCache = copyItemCache(m.itemCache)
	t.yamlContent = m.yamlContent
	t.yamlScroll = m.yamlScroll
	t.yamlCursor = m.yamlCursor
	t.yamlSearchText = m.yamlSearchText
	t.yamlMatchLines = m.yamlMatchLines
	t.yamlMatchIdx = m.yamlMatchIdx
	t.yamlCollapsed = copyMapStringBool(m.yamlCollapsed)
	t.splitPreview = m.splitPreview
	t.fullYAMLPreview = m.fullYAMLPreview
	t.previewYAML = m.previewYAML
	t.namespace = m.namespace
	t.allNamespaces = m.allNamespaces
	t.selectedNamespaces = copyMapStringBool(m.selectedNamespaces)
	t.sortBy = m.sortBy
	t.filterText = m.filterText
	t.watchMode = m.watchMode
	t.requestGen = m.requestGen
	t.selectedItems = copyMapStringBool(m.selectedItems)
	t.fullscreenMiddle = m.fullscreenMiddle
	t.fullscreenDashboard = m.fullscreenDashboard
	t.dashboardPreview = m.dashboardPreview
	t.monitoringPreview = m.monitoringPreview
	t.warningEventsOnly = m.warningEventsOnly
	t.expandedGroup = m.expandedGroup
	t.allGroupsExpanded = m.allGroupsExpanded
	t.mode = m.mode
	t.logLines = append([]string(nil), m.logLines...)
	t.logScroll = m.logScroll
	t.logFollow = m.logFollow
	t.logWrap = m.logWrap
	t.logLineNumbers = m.logLineNumbers
	t.logTitle = m.logTitle
	t.logCancel = m.logCancel
	t.logCh = m.logCh
	t.logParentKind = m.logParentKind
	t.logParentName = m.logParentName
	t.describeContent = m.describeContent
	t.describeScroll = m.describeScroll
	t.describeTitle = m.describeTitle
	t.diffLeft = m.diffLeft
	t.diffRight = m.diffRight
	t.diffLeftName = m.diffLeftName
	t.diffRightName = m.diffRightName
	t.diffScroll = m.diffScroll
	t.diffUnified = m.diffUnified
	t.execPTY = m.execPTY
	t.execTerm = m.execTerm
	t.execTitle = m.execTitle
	t.execDone = m.execDone
	t.execMu = m.execMu
}

// loadTab restores Model fields from the given tab index.
func (m *Model) loadTab(idx int) {
	t := m.tabs[idx]
	m.activeTab = idx
	m.nav = t.nav
	m.leftItems = append([]model.Item(nil), t.leftItems...)
	m.middleItems = append([]model.Item(nil), t.middleItems...)
	m.rightItems = append([]model.Item(nil), t.rightItems...)
	m.leftItemsHistory = make([][]model.Item, len(t.leftItemsHistory))
	for i, hist := range t.leftItemsHistory {
		m.leftItemsHistory[i] = append([]model.Item(nil), hist...)
	}
	m.cursors = t.cursors
	m.cursorMemory = copyMapStringInt(t.cursorMemory)
	m.itemCache = copyItemCache(t.itemCache)
	m.yamlContent = t.yamlContent
	m.yamlScroll = t.yamlScroll
	m.yamlCursor = t.yamlCursor
	m.yamlSearchText = t.yamlSearchText
	m.yamlMatchLines = t.yamlMatchLines
	m.yamlMatchIdx = t.yamlMatchIdx
	m.yamlCollapsed = copyMapStringBool(t.yamlCollapsed)
	m.splitPreview = t.splitPreview
	m.fullYAMLPreview = t.fullYAMLPreview
	m.previewYAML = t.previewYAML
	m.namespace = t.namespace
	m.allNamespaces = t.allNamespaces
	m.selectedNamespaces = copyMapStringBool(t.selectedNamespaces)
	m.sortBy = t.sortBy
	m.filterText = t.filterText
	m.watchMode = t.watchMode
	m.requestGen = t.requestGen
	m.selectedItems = copyMapStringBool(t.selectedItems)
	m.fullscreenMiddle = t.fullscreenMiddle
	m.fullscreenDashboard = t.fullscreenDashboard
	m.dashboardPreview = t.dashboardPreview
	m.monitoringPreview = t.monitoringPreview
	m.warningEventsOnly = t.warningEventsOnly
	m.expandedGroup = t.expandedGroup
	m.allGroupsExpanded = t.allGroupsExpanded

	// Restore per-tab view mode and log state.
	m.mode = t.mode
	m.logLines = append([]string(nil), t.logLines...)
	m.logScroll = t.logScroll
	m.logFollow = t.logFollow
	m.logWrap = t.logWrap
	m.logLineNumbers = t.logLineNumbers
	m.logTitle = t.logTitle
	m.logCancel = t.logCancel
	m.logCh = t.logCh
	m.logParentKind = t.logParentKind
	m.logParentName = t.logParentName
	m.describeContent = t.describeContent
	m.describeScroll = t.describeScroll
	m.describeTitle = t.describeTitle
	m.diffLeft = t.diffLeft
	m.diffRight = t.diffRight
	m.diffLeftName = t.diffLeftName
	m.diffRightName = t.diffRightName
	m.diffScroll = t.diffScroll
	m.diffUnified = t.diffUnified
	m.execPTY = t.execPTY
	m.execTerm = t.execTerm
	m.execTitle = t.execTitle
	m.execDone = t.execDone
	m.execMu = t.execMu

	// Close overlays and reset transient state.
	m.overlay = overlayNone
	m.filterActive = false
	m.searchActive = false
	m.err = nil
}

// cloneCurrentTab creates a deep copy of the current model state as a new TabState.
func (m *Model) cloneCurrentTab() TabState {
	newTab := TabState{
		nav:                 m.nav,
		leftItems:           append([]model.Item(nil), m.leftItems...),
		middleItems:         append([]model.Item(nil), m.middleItems...),
		rightItems:          append([]model.Item(nil), m.rightItems...),
		cursors:             m.cursors,
		cursorMemory:        copyMapStringInt(m.cursorMemory),
		itemCache:           copyItemCache(m.itemCache),
		yamlContent:         m.yamlContent,
		yamlCollapsed:       copyMapStringBool(m.yamlCollapsed),
		splitPreview:        m.splitPreview,
		fullYAMLPreview:     m.fullYAMLPreview,
		previewYAML:         m.previewYAML,
		namespace:           m.namespace,
		allNamespaces:       m.allNamespaces,
		sortBy:              m.sortBy,
		filterText:          m.filterText,
		watchMode:           m.watchMode,
		selectedItems:       copyMapStringBool(m.selectedItems),
		fullscreenMiddle:    m.fullscreenMiddle,
		fullscreenDashboard: m.fullscreenDashboard,
		dashboardPreview:    m.dashboardPreview,
		monitoringPreview:   m.monitoringPreview,
	}
	// Deep copy leftItemsHistory.
	newTab.leftItemsHistory = make([][]model.Item, len(m.leftItemsHistory))
	for i, hist := range m.leftItemsHistory {
		newTab.leftItemsHistory[i] = append([]model.Item(nil), hist...)
	}
	return newTab
}

// copyMapStringInt deep copies a map[string]int.
func copyMapStringInt(m map[string]int) map[string]int {
	if m == nil {
		return make(map[string]int)
	}
	c := make(map[string]int, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// copyMapStringBool deep copies a map[string]bool.
func copyMapStringBool(m map[string]bool) map[string]bool {
	if m == nil {
		return make(map[string]bool)
	}
	c := make(map[string]bool, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// copyItemCache deep copies the item cache.
func copyItemCache(m map[string][]model.Item) map[string][]model.Item {
	if m == nil {
		return make(map[string][]model.Item)
	}
	c := make(map[string][]model.Item, len(m))
	for k, v := range m {
		c[k] = append([]model.Item(nil), v...)
	}
	return c
}

// padToHeight ensures a string has exactly `height` newline-separated lines,
// padding with empty lines or truncating as needed.
// placeTopRight overlays indicator text on the first line's right side.
func placeTopRight(content, indicator string, width int) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}
	first := lines[0]
	firstW := lipgloss.Width(first)
	indW := lipgloss.Width(indicator)
	if firstW+indW+1 <= width {
		// Enough room: pad and append.
		gap := width - firstW - indW
		lines[0] = first + strings.Repeat(" ", gap) + indicator
	} else {
		// Truncate first line to make room.
		lines[0] = ui.Truncate(first, width-indW-1) + " " + indicator
	}
	return strings.Join(lines, "\n")
}

func padToHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// actionNamespace returns the namespace to use for action commands.
// It prefers the namespace captured when the action menu was opened.
func (m Model) actionNamespace() string {
	if m.actionCtx.namespace != "" {
		return m.actionCtx.namespace
	}
	return m.resolveNamespace()
}

// viewExecTerminal renders the embedded PTY terminal view.
func (m Model) viewExecTerminal() string {
	title := ui.TitleStyle.Render(m.execTitle)

	// Render hints.
	hints := []struct{ key, desc string }{
		{"ctrl+]", "exit"},
	}
	var hintParts []string
	for _, h := range hints {
		hintParts = append(hintParts, ui.HelpKeyStyle.Render(h.key)+" "+ui.DimStyle.Render(h.desc))
	}
	hintLine := "  " + strings.Join(hintParts, "  ")

	// Render terminal content (account for border: 2 lines top/bottom, 2 cols left/right).
	viewH := m.height - 6 // title + hint + border top/bottom
	if viewH < 3 {
		viewH = 3
	}
	viewW := m.width - 4 // border left/right + padding
	if viewW < 10 {
		viewW = 10
	}

	var termContent string
	if m.execTerm != nil {
		m.execMu.Lock()
		cols, rows := m.execTerm.Size()
		var lines []string
		for y := 0; y < rows && y < viewH; y++ {
			var line strings.Builder
			for x := 0; x < cols && x < viewW; x++ {
				g := m.execTerm.Cell(x, y)
				ch := g.Char
				if ch == 0 {
					ch = ' '
				}
				// Apply FG/BG colors.
				style := lipgloss.NewStyle()
				if g.FG != vt10x.DefaultFG {
					style = style.Foreground(vt10xColorToLipgloss(g.FG))
				}
				if g.BG != vt10x.DefaultBG {
					style = style.Background(vt10xColorToLipgloss(g.BG))
				}
				// Apply text attributes from glyph mode.
				if g.Mode&(1<<2) != 0 { // bold (attrBold = 4)
					style = style.Bold(true)
				}
				if g.Mode&(1<<1) != 0 { // underline (attrUnderline = 2)
					style = style.Underline(true)
				}
				if g.Mode&1 != 0 { // reverse (attrReverse = 1)
					style = style.Reverse(true)
				}
				line.WriteString(style.Render(string(ch)))
			}
			lines = append(lines, line.String())
		}
		m.execMu.Unlock()
		termContent = strings.Join(lines, "\n")
	} else {
		termContent = ui.DimStyle.Render("Terminal not initialized")
	}

	if m.execDone != nil && m.execDone.Load() {
		termContent += "\n\n" + ui.DimStyle.Render("  Process exited. Press any key to return.")
	}

	// Wrap terminal content in a rounded border.
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPrimary)).
		Padding(0, 0).
		Width(m.width - 2).
		Height(viewH)
	bordered := borderStyle.Render(termContent)

	return lipgloss.JoinVertical(lipgloss.Left, title, bordered, hintLine)
}

// vt10xColorToLipgloss converts a vt10x color to a lipgloss terminal color.
func vt10xColorToLipgloss(c vt10x.Color) lipgloss.TerminalColor {
	return lipgloss.Color(fmt.Sprintf("%d", int(c)))
}
