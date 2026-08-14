package app

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// NewModel creates the initial model.
func NewModel(client *k8s.Client, opts StartupOptions) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.ThemeColor("62"))

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

	// Seed the session "show rarely-used types" state from config so the full
	// resource-type list can be visible from launch. model.ShowRareResources is
	// the global the sidebar builder reads; keep it in sync with the field.
	model.ShowRareResources = ui.ConfigShowRareTypes

	reqCtx, reqCancel := context.WithCancel(context.Background())
	pinnedSt := loadPinnedState()
	pinnedSummariesSt := loadPinnedSummariesState()
	hiddenSt := loadHiddenTypesState()
	sortMem := loadSortMemory()
	colPrefs := loadColumnPrefs()
	// Resolve which session to open: --session / LFK_SESSION, else the
	// persisted active session, else the default workspace (session.yaml).
	activeSession, pendingSession := loadStartupSession(opts.Session)
	m := Model{
		client:   client,
		demoMode: client.IsDemo(),
		// Start in the loading state. Init() dispatches loadContexts()
		// asynchronously, so the first frame renders before any
		// contextsLoadedMsg arrives; loading=false there falls through to the
		// "No items" / "No resource types found" empty states until the reply
		// lands. The loaded-message handlers clear the flag once data arrives.
		loading:        true,
		nav:            model.NavigationState{Level: model.LevelClusters},
		bookmarks:      loadBookmarks(),
		pendingSession: pendingSession,
		// Arm the restore guard from the very first frame so a keystroke that
		// beats the resource list is not undone when the list arrives.
		restoringSession:    pendingSession != nil,
		pendingPortForwards: loadPortForwardState(),
		commandHistory:      loadCommandHistory(),
		queryHistory:        loadInputHistory(historyFileQuery),
		logView: logViewState{
			searchHistory:  loadInputHistory(historyFileLogSearch),
			previewVisible: ui.ConfigLogShowPreview,
			hidePrefixes:   !ui.ConfigLogShowPrefixes,
			timestamps:     ui.ConfigLogShowTimestamps,
			wrap:           ui.ConfigLogWrap,
		},
		pinnedState:             pinnedSt,
		pinnedSummariesState:    pinnedSummariesSt,
		hiddenState:             hiddenSt,
		namespace:               defaultNS,
		spinner:                 s,
		spinnerTicking:          true,
		watchInterval:           watchInterval,
		focused:                 true,
		lastInputAt:             time.Now(),
		backgroundWatchInterval: resolveBackgroundInterval(opts),
		watchThrottle:           ui.ConfigWatchThrottle,
		foregroundIdleTimeout:   resolveForegroundIdle(opts),
		fullLogPreview:          ui.ConfigLogPreviewLive,
		splitPreview:            ui.ConfigSplitPreview,
		allNamespaces:           ui.ConfigAllNamespaces,
		watchMode:               ui.ConfigWatchMode,
		objectExplorerLive:      ui.ConfigObjectExplorerLive,
		objectExplorerTree:      ui.ConfigObjectExplorerTree,
		explainTreeState:        explainTreeState{explainTreeWanted: ui.ConfigAPIExplorerTree},
		readOnly:                ui.ResolveReadOnly(contextName, opts.ReadOnly),
		cliReadOnly:             opts.ReadOnly,
		showRareResources:       ui.ConfigShowRareTypes,
		contextROOverrides:      make(map[string]bool),
		contextBadgeOverrides:   make(map[string]bool),
		clusterColors:           loadClusterColors(),
		localClusterFields:      localClusterFields{localClusterCache: loadLocalClusterState()},
		sortColumnName:          sortColDefault,
		sortAscending:           true,
		sortMemory:              sortMem,
		columnToggleState: columnToggleState{
			sessionColumns:       colPrefs.sessionColumns,
			hiddenBuiltinColumns: colPrefs.hiddenBuiltinColumns,
			columnOrder:          colPrefs.columnOrder,
		},
		whichKey:                   whichKeyState{grouping: loadWhichKeyGrouping()},
		sessionsOverlayState:       sessionsOverlayState{activeSession: activeSession},
		cursorMemory:               make(map[string]int),
		filterMemory:               make(map[string]savedFilter),
		itemCache:                  make(map[string][]model.Item),
		cacheFingerprints:          make(map[string]string),
		perms:                      newPermissionState(),
		selectedItems:              make(map[string]bool),
		selectionAnchor:            -1,
		yamlView:                   yamlViewState{collapsed: make(map[string]bool), wrap: ui.ConfigYAMLViewerWrap},
		describeView:               describeViewState{wrap: ui.ConfigDescribeViewerWrap},
		dashboardAcc:               make(map[string]*dashboardAccumulator),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		discoveringContexts:        make(map[string]bool),
		previewLogCache:            make(map[string]previewLogCacheEntry),
		secretPreviewCache:         make(map[string]*model.SecretData),
		serviceEndpointsCache:      make(map[string]*k8s.ServiceEndpoints),
		orphanCache:                make(map[orphanCacheKey]*k8s.OrphanReport),
		orphanLoadInflight:         make(map[orphanCacheKey]orphanInflight),
		orphans:                    orphanState{strict: true},
		rightsizingCache:           make(map[string]*model.Rightsizing),
		discoveryRefreshedContexts: make(map[string]bool),
		allGroupsExpanded:          true,
		warningEventsOnly:          ui.ConfigEventsWarningsOnly,
		eventGrouping:              ui.ConfigEventsGrouping,
		scheduler:                  scheduler.New(scheduler.DefaultThreshold),
		diffView:                   diffViewState{wrap: ui.ConfigDiffViewerWrap, lineNumbers: ui.ConfigDiffViewerLineNumbers, unified: ui.ConfigDiffViewerUnified, diffCache: &ui.DiffCache{}},
		fieldDoc:                   fieldDocState{cache: newFieldDocCache()},
		execTickGen:                &atomic.Uint64{},
		logReaderInFlight:          make(map[chan string]bool),
		reqCtx:                     reqCtx,
		reqCancel:                  reqCancel,
		middleTableRenderer:        ui.NewTableRenderer(),
		tabs: []TabState{{
			uid:                nextTabUID(),
			nav:                model.NavigationState{Level: model.LevelClusters},
			namespace:          defaultNS,
			splitPreview:       ui.ConfigSplitPreview,
			allNamespaces:      ui.ConfigAllNamespaces,
			watchMode:          ui.ConfigWatchMode,
			objectExplorerLive: ui.ConfigObjectExplorerLive,
			objectExplorerTree: ui.ConfigObjectExplorerTree,
			readOnly:           ui.ResolveReadOnly(contextName, opts.ReadOnly),
			sortColumnName:     sortColDefault,
			sortAscending:      true,
			sortMemory:         copyMapStringSortPref(sortMem),
			warningEventsOnly:  ui.ConfigEventsWarningsOnly,
			eventGrouping:      ui.ConfigEventsGrouping,
			allGroupsExpanded:  true,
			cursorMemory:       make(map[string]int),
			filterMemory:       make(map[string]savedFilter),
			itemCache:          make(map[string][]model.Item),
			cacheFingerprints:  make(map[string]string),
			selectedItems:      make(map[string]bool),
			selectionAnchor:    -1,
			selectedNamespaces: nil,
		}},
		activeTab:      0,
		execMu:         &sync.Mutex{},
		portForwardMgr: k8s.NewPortForwardManager(),
		captureMgr:     k8s.NewCaptureManager(),
	}

	// Stale-while-revalidate: the per-host snapshots under
	// ~/.kube/cache/discovery/<host>/lfk-enriched.yaml are loaded
	// asynchronously by discoveryCachePreloadCmd dispatched from Init().
	// Loading them here would block startup behind a clientcmd.ClientConfig()
	// call per kubeconfig context — at multi-thousand-context scale that
	// hangs the model construction (and therefore the first render). The
	// async path overlays cached entries onto m.discoveredResources when the
	// preload message arrives; until then the sidebar shows the seed list.
	//
	// The one context a restored session opens is the exception, and it is a
	// single clientcmd call rather than one per context. A saved CRD view has
	// no match in the built-in seeds, so without the snapshot the restore has
	// to park on the resource-type browser and jump to the list a second later
	// when live discovery answers. Reading the snapshot here lets the restore
	// navigate straight to the saved view on the first frame.
	if ctxName := sessionRestoreContext(pendingSession); ctxName != "" {
		if entries := loadDiscoveryCacheForContext(client, ctxName); len(entries) > 0 {
			m.discoveredResources[ctxName] = entries
		}
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
		// CLI overrides (--context/--namespace/--union) replace the workspace
		// with a synthetic one, so auto-save must target the default
		// (session.yaml), not clobber a named session (e.g. --session foo
		// --context bar) with mismatched state.
		m.activeSession = ""
	}

	if opts.IsUnionMode() {
		m.unionMode = true
		m.unionContexts = opts.UnionContexts
		m.unionSetName = opts.UnionSet
		m.unionStartedFromPicker = opts.UnionSet != ""
		m.unionContextColors = opts.UnionContextColors
		// In union mode there is no single active context whose config can
		// populate Model.readOnly. Per-row action dispatch resolves each
		// target cluster independently; keep only the process-wide CLI flag
		// here so the kubeconfig's current-context does not accidentally
		// make the entire union read-only.
		m.readOnly = opts.ReadOnly
		if len(m.tabs) > 0 {
			m.tabs[0].readOnly = opts.ReadOnly
		}
		m.nav.Context = UnionContextSentinel
		m.allNamespaces = false
		m.namespace = opts.Namespaces[0]
		if len(opts.Namespaces) > 1 {
			m.selectedNamespaces = make(map[string]bool, len(opts.Namespaces))
			for _, ns := range opts.Namespaces {
				m.selectedNamespaces[ns] = true
			}
		}
		// Use the first union context for API resource discovery and session restore.
		m.pendingSession = &SessionState{
			Context: m.unionContexts[0],
			Tabs: []SessionTab{{
				Context:            m.unionContexts[0],
				AllNamespaces:      false,
				Namespace:          opts.Namespaces[0],
				SelectedNamespaces: opts.Namespaces,
			}},
		}
	}

	m.applyPinnedTypes()

	m.helpSearchInput = textinput.New()
	m.helpSearchInput.Prompt = ""
	m.helpSearchInput.CharLimit = 100

	m.scheduler.StartWorkers()

	// Security feature wiring. Install the SecuritySourcesFn hook before
	// refreshSecuritySources so the very first sidebar build sees an
	// empty Security category (rather than nil, which would suppress the
	// pseudo-header). loadSecurityIgnores reads the user's ignore-list
	// YAML; refreshSecuritySources builds the per-cluster manager and
	// publishes it to the hook state.
	installSecuritySourcesHook()
	m.securityIgnores = loadSecurityIgnores()
	m.hideSecurityBadges = ui.ResolveSecurityHideBadges(contextName)
	m.initialSecuritySeedCmd = m.refreshSecuritySources()

	// Mirror main.go's startup mouse decision: capture is on unless the
	// --no-mouse flag was set or config disabled it. The toggle keybinding
	// flips mouseCaptured at runtime; when mouse was never available it
	// reports that and does nothing.
	m.mouseAvailable = !opts.NoMouse && ui.ConfigMouse
	m.mouseCaptured = m.mouseAvailable

	return m
}
