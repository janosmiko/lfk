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
		client:                     client,
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
		cursorMemory:               make(map[string]int),
		itemCache:                  make(map[string][]model.Item),
		cacheFingerprints:          make(map[string]string),
		selectedItems:              make(map[string]bool),
		selectionAnchor:            -1,
		yamlCollapsed:              make(map[string]bool),
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
			warningEventsOnly:  true,
			eventGrouping:      true,
			allGroupsExpanded:  true,
			cursorMemory:       make(map[string]int),
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

	m.applyPinnedGroups()

	m.helpSearchInput = textinput.New()
	m.helpSearchInput.Prompt = ""
	m.helpSearchInput.CharLimit = 100

	m.scheduler.StartWorkers()

	return m
}
