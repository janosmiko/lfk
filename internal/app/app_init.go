package app

import (
	"context"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
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
		sortColumnName:             sortColDefault,
		sortAscending:              true,
		cursorMemory:               make(map[string]int),
		itemCache:                  make(map[string][]model.Item),
		cacheFingerprints:          make(map[string]string),
		selectedItems:              make(map[string]bool),
		selectionAnchor:            -1,
		yamlCollapsed:              make(map[string]bool),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		discoveringContexts:        make(map[string]bool),
		secretPreviewCache:         make(map[string]*model.SecretData),
		discoveryRefreshedContexts: make(map[string]bool),
		allGroupsExpanded:          true,
		warningEventsOnly:          true,
		eventGrouping:              true,
		logPreviewVisible:          true,
		bgtasks:                    bgtasks.New(bgtasks.DefaultThreshold),
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
	}

	// Stale-while-revalidate: seed discoveredResources from the per-host
	// snapshots under ~/.kube/cache/discovery/<host>/lfk-enriched.yaml so
	// the sidebar paints instantly on first frame instead of waiting for a
	// live discovery roundtrip. The lazy-trigger sites still fire fresh
	// discovery (gated by m.discoveryRefreshedContexts), so the cached
	// values are replaced as soon as the live result lands.
	if cached := loadAllDiscoveryCaches(client); cached != nil {
		pseudo := model.PseudoResources()
		for ctx, entries := range cached {
			merged := make([]model.ResourceTypeEntry, 0, len(pseudo)+len(entries))
			merged = append(merged, pseudo...)
			merged = append(merged, entries...)
			m.discoveredResources[ctx] = merged
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
	}

	m.applyPinnedGroups()

	m.helpSearchInput = textinput.New()
	m.helpSearchInput.Prompt = ""
	m.helpSearchInput.CharLimit = 100

	return m
}
