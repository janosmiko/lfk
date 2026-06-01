package app

import (
	"context"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

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

	reqCtx, reqCancel := context.WithCancel(context.Background())
	pinnedSt := loadPinnedState()
	m := Model{
		client: client,
		// Start in the loading state. Init() dispatches loadContexts()
		// asynchronously, so the first frame renders before any
		// contextsLoadedMsg arrives; loading=false there falls through to the
		// "No items" / "No resource types found" empty states until the reply
		// lands. The loaded-message handlers clear the flag once data arrives.
		loading:                    true,
		nav:                        model.NavigationState{Level: model.LevelClusters},
		bookmarks:                  loadBookmarks(),
		pendingSession:             loadSession(),
		pendingPortForwards:        loadPortForwardState(),
		commandHistory:             loadCommandHistory(),
		queryHistory:               loadInputHistory(historyFileQuery),
		logSearchHistory:           loadInputHistory(historyFileLogSearch),
		pinnedState:                pinnedSt,
		namespace:                  defaultNS,
		spinner:                    s,
		watchInterval:              watchInterval,
		splitPreview:               true,
		allNamespaces:              true,
		watchMode:                  true,
		readOnly:                   ui.ResolveReadOnly(contextName, opts.ReadOnly),
		cliReadOnly:                opts.ReadOnly,
		contextROOverrides:         make(map[string]bool),
		clusterColors:              loadClusterColors(),
		localClusterFields:         localClusterFields{localClusterCache: loadLocalClusterState()},
		sortColumnName:             sortColDefault,
		sortAscending:              true,
		sortMemory:                 make(map[string]sortPref),
		cursorMemory:               make(map[string]int),
		filterMemory:               make(map[string]savedFilter),
		itemCache:                  make(map[string][]model.Item),
		cacheFingerprints:          make(map[string]string),
		selectedItems:              make(map[string]bool),
		selectionAnchor:            -1,
		yamlView:                   yamlViewState{collapsed: make(map[string]bool)},
		dashboardAcc:               make(map[string]*dashboardAccumulator),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		discoveringContexts:        make(map[string]bool),
		secretPreviewCache:         make(map[string]*model.SecretData),
		serviceEndpointsCache:      make(map[string]*k8s.ServiceEndpoints),
		orphanCache:                make(map[orphanCacheKey]*k8s.OrphanReport),
		orphanLoadInflight:         make(map[orphanCacheKey]orphanInflight),
		orphans:                    orphanState{strict: true},
		rightsizingCache:           make(map[string]*model.Rightsizing),
		discoveryRefreshedContexts: make(map[string]bool),
		allGroupsExpanded:          true,
		warningEventsOnly:          true,
		eventGrouping:              true,
		logPreviewVisible:          true,
		scheduler:                  scheduler.New(scheduler.DefaultThreshold),
		diffLineNumbers:            true,
		reqCtx:                     reqCtx,
		reqCancel:                  reqCancel,
		middleTableRenderer:        ui.NewTableRenderer(),
		tabs: []TabState{{
			nav:                model.NavigationState{Level: model.LevelClusters},
			namespace:          defaultNS,
			splitPreview:       true,
			allNamespaces:      true,
			watchMode:          true,
			readOnly:           ui.ResolveReadOnly(contextName, opts.ReadOnly),
			sortColumnName:     sortColDefault,
			sortAscending:      true,
			sortMemory:         make(map[string]sortPref),
			warningEventsOnly:  true,
			eventGrouping:      true,
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
	m.refreshSecuritySources()

	// Mirror main.go's startup mouse decision: capture is on unless the
	// --no-mouse flag was set or config disabled it. The toggle keybinding
	// flips mouseCaptured at runtime; when mouse was never available it
	// reports that and does nothing.
	m.mouseAvailable = !opts.NoMouse && ui.ConfigMouse
	m.mouseCaptured = m.mouseAvailable

	return m
}
