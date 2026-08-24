package app

import (
	"context"
	"strings"
	"sync"
	"testing"

	"charm.land/bubbles/v2/textinput"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	tea "charm.land/bubbletea/v2"
	corev1 "k8s.io/api/core/v1"
	dynfake "k8s.io/client-go/dynamic/fake"
	fake "k8s.io/client-go/kubernetes/fake"
)

// baseFinalModel returns a Model with fake K8s client for final push tests.
func baseFinalModel() Model {
	cs := fake.NewClientset()
	dyn := dynfake.NewSimpleDynamicClient(runtime.NewScheme())

	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		client:              k8s.NewTestClient(cs, dyn),
		namespace:           "default",
		reqCtx:              context.Background(),
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		Resource:   "pods",
		Namespaced: true,
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		name:         "test-resource",
		namespace:    "default",
		kind:         "Pod",
		resourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
	}

	return m
}

// baseFinalModelWithDynamic returns a Model with a properly configured dynamic client.
func baseFinalModelWithDynamic() Model {
	cs := fake.NewClientset()
	dyn := newFinalDynClient()

	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		client:              k8s.NewTestClient(cs, dyn),
		namespace:           "default",
		reqCtx:              context.Background(),
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		Resource:   "pods",
		Namespaced: true,
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		name:         "test-resource",
		namespace:    "default",
		kind:         "Pod",
		resourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
	}

	return m
}

func baseModelActions() Model {
	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
		{Name: "pod-2", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:     "Pod",
		Resource: "pods",
	}
	return m
}

// baseModelBoost2 returns a Model with a fake K8s client for boost tests.
func baseModelBoost2() Model {
	cs := fake.NewClientset()
	dyn := dynfake.NewSimpleDynamicClient(runtime.NewScheme())

	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		client:              k8s.NewTestClient(cs, dyn),
		namespace:           "default",
		reqCtx:              context.Background(),
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		Resource:   "pods",
		Namespaced: true,
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		name:         "test-resource",
		namespace:    "default",
		kind:         "Pod",
		resourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
	}

	return m
}

// colKey returns the context-scoped column-memory key for a kind under the
// empty (test default) context, matching what production code reads/writes
// via Model.columnMemoryKey. Use it whenever a test seeds or asserts on the
// sessionColumns / hiddenBuiltinColumns / columnOrder maps.
func colKey(kind string) string {
	var m Model
	return m.columnMemoryKey(kind)
}

// baseModelCov returns a minimal Model for coverage tests.
func baseModelCov() Model {
	return Model{
		viewerPrefs:                newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                        model.NavigationState{Level: model.LevelResources},
		tabs:                       []TabState{{}},
		selectedItems:              make(map[string]bool),
		cursorMemory:               make(map[string]int),
		itemCache:                  make(map[string][]model.Item),
		cacheFingerprints:          make(map[string]string),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		discoveringContexts:        make(map[string]bool),
		discoveryRefreshedContexts: make(map[string]bool),
		width:                      80,
		height:                     40,
		execMu:                     &sync.Mutex{},
		scheduler:                  scheduler.New(0),
	}
}

// baseModelWithFakeClientAndScheduler returns a Model wired to fake k8s clients
// with a scheduler that has workers started. Workers are stopped at test cleanup.
// Use this instead of baseModelWithFakeClient for tests that execute scheduler-
// routed commands (e.g. loadNamespacesForContext after migration).
func baseModelWithFakeClientAndScheduler(t *testing.T, objs ...runtime.Object) Model {
	t.Helper()
	m := baseModelWithFakeClient(objs...)
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)
	return m
}

func baseModelDescribe() Model {
	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
	m.describeView.content = "line0\nline1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9"
	m.mode = modeDescribe
	return m
}

func baseModelExplain() Model {
	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
	m.mode = modeExplain
	m.explainFields = []model.ExplainField{
		{Name: "apiVersion", Type: "string", Path: "apiVersion"},
		{Name: "kind", Type: "string", Path: "kind"},
		{Name: "metadata", Type: "Object", Path: "metadata"},
		{Name: "spec", Type: "Object", Path: "spec"},
		{Name: "status", Type: "Object", Path: "status"},
	}
	m.explainResource = "deployments"
	m.explainAPIVersion = "apps/v1"
	return m
}

func baseModelFinalizer() Model {
	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
	m.overlay = overlayFinalizerSearch
	m.finalizerSearchResults = []k8s.FinalizerMatch{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Matched: "kubernetes.io/pv-protection"},
		{Name: "pod-2", Namespace: "default", Kind: "Pod", Matched: "kubernetes.io/pv-protection"},
		{Name: "pod-3", Namespace: "kube-system", Kind: "Pod", Matched: "finalizer.example.com"},
	}
	m.finalizerSearchSelected = make(map[string]bool)
	return m
}

func baseModelHandlers2() Model {
	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
	return m
}

func baseModelNav() Model {
	m := Model{
		viewerPrefs: newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-ctx",
			ResourceType: model.ResourceTypeEntry{
				Kind:     "Pod",
				Resource: "pods",
			},
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
		{Name: "pod-2", Namespace: "default", Kind: "Pod", Status: "Running"},
		{Name: "pod-3", Namespace: "default", Kind: "Pod", Status: "Failed"},
		{Name: "pod-4", Namespace: "kube-system", Kind: "Pod", Status: "Running"},
		{Name: "pod-5", Namespace: "kube-system", Kind: "Pod", Status: "Running"},
	}
	return m
}

func baseModelOverlay() Model {
	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
	return m
}

func baseModelSearch() Model {
	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
	m.helpSearchInput = textinput.New()
	return m
}

func baseModelUpdate() Model {
	cs := fake.NewClientset()
	dyn := dynfake.NewSimpleDynamicClient(runtime.NewScheme())

	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		client:              k8s.NewTestClient(cs, dyn),
		namespace:           "default",
		reqCtx:              context.Background(),
		portForwardMgr:      k8s.NewPortForwardManager(),
	}
	m.middleItems = []model.Item{{Name: "pod-1", Namespace: "default", Kind: "Pod"}}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}

	return m
}

// baseModelWithFakeClient returns a Model wired to fake k8s clients.
// The fake clientset is pre-loaded with the given objects.
func baseModelWithFakeClient(objs ...runtime.Object) Model {
	cs := fake.NewClientset(objs...)
	scheme := newFakeScheme()
	dyn := dynfake.NewSimpleDynamicClient(scheme)
	client := k8s.NewTestClient(cs, dyn)

	m := baseModelCov()
	m.client = client
	m.nav.Context = "test-ctx"
	m.namespace = "default"
	m.reqCtx = context.Background()
	return m
}

// baseModelWithFakeDynamic returns a Model with a dynamic client that knows
// about the provided GVR-to-list-kind mappings and unstructured objects.
func baseModelWithFakeDynamic(
	gvrToListKind map[schema.GroupVersionResource]string,
	objs ...runtime.Object,
) Model {
	cs := fake.NewClientset()
	scheme := newFakeScheme()
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objs...)
	client := k8s.NewTestClient(cs, dyn)

	m := baseModelCov()
	m.client = client
	m.nav.Context = "test-ctx"
	m.namespace = "default"
	m.reqCtx = context.Background()
	return m
}

func basePush4Model() Model {
	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		namespace:           "default",
		reqCtx:              context.Background(),
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
		{Name: "pod-2", Namespace: "default", Kind: "Pod", Status: "Running"},
		{Name: "pod-3", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		Resource:   "pods",
		Namespaced: true,
	}

	return m
}

// basePush80Model returns a model with a fake k8s client for coverage tests.
func basePush80Model() Model {
	m := Model{
		viewerPrefs: newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-ctx",
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		namespace:           "default",
		reqCtx:              context.Background(),
		objectExplorerLive:  true, // production default (ui.ConfigObjectExplorerLive)
		watchThrottle:       true, // production default (ui.ConfigWatchThrottle)
		// Mirrors app_init. Without it a diff-viewer test measures the
		// uncached path, which is not the one the app runs.
		diffView: diffViewState{diffCache: &ui.DiffCache{}},
	}
	m.client = k8s.NewTestClient(
		fake.NewClientset(),
		dynfake.NewSimpleDynamicClient(runtime.NewScheme()),
	)
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		Resource:   "pods",
		APIVersion: "v1",
		Namespaced: true,
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
		{Name: "pod-2", Namespace: "ns-2", Kind: "Pod", Status: "Failed"},
		{Name: "pod-3", Namespace: "default", Kind: "Pod", Status: "Pending"},
	}
	return m
}

func basePush80v2Model() Model {
	m := Model{
		viewerPrefs: newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-ctx",
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		namespace:           "default",
		reqCtx:              context.Background(),
		objectExplorerLive:  true, // production default (ui.ConfigObjectExplorerLive)
		watchThrottle:       true, // production default (ui.ConfigWatchThrottle)
	}
	m.client = k8s.NewTestClient(
		fake.NewClientset(),
		dynfake.NewSimpleDynamicClient(runtime.NewScheme()),
	)
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		Resource:   "pods",
		APIVersion: "v1",
		Namespaced: true,
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
		{Name: "pod-2", Namespace: "ns-2", Kind: "Pod", Status: "Failed"},
		{Name: "pod-3", Namespace: "default", Kind: "Pod", Status: "Pending"},
	}
	return m
}

func basePush80v3Model() Model {
	m := Model{
		viewerPrefs: newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-ctx",
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		namespace:           "default",
		reqCtx:              context.Background(),
		objectExplorerLive:  true, // production default (ui.ConfigObjectExplorerLive)
		watchThrottle:       true, // production default (ui.ConfigWatchThrottle)
	}
	m.client = k8s.NewTestClient(
		fake.NewClientset(),
		dynfake.NewSimpleDynamicClient(runtime.NewScheme()),
	)
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		Resource:   "pods",
		APIVersion: "v1",
		Namespaced: true,
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
		{Name: "pod-2", Namespace: "ns-2", Kind: "Pod", Status: "Failed"},
	}
	return m
}

func baseRichModel() Model {
	cs := fake.NewClientset()
	dyn := newRichDynClient()

	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		client:              k8s.NewTestClient(cs, dyn),
		namespace:           "default",
		reqCtx:              context.Background(),
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		Resource:   "pods",
		Namespaced: true,
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		name:         "test-resource",
		namespace:    "default",
		kind:         "Pod",
		resourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
	}

	return m
}

func bp4() Model {
	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120, height: 40, execMu: &sync.Mutex{}, namespace: "default",
		reqCtx: context.Background(),
	}
	m.client = k8s.NewTestClient(fake.NewClientset(), dynfake.NewSimpleDynamicClient(runtime.NewScheme()))
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
		{Name: "pod-2", Namespace: "ns-2", Kind: "Pod", Status: "Failed"},
	}
	return m
}

// execCmd runs a tea.Cmd and returns the resulting tea.Msg.
func execCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	require.NotNil(t, cmd)
	return cmd()
}

// execScheduled runs a cmd that was Submitted to m.scheduler via scheduleK8sCall.
// Such a cmd blocks until a worker delivers its Future, so the model's scheduler
// must have workers running. StartWorkers retroactively spawns pools for queues
// created by an earlier synchronous Submit, so starting here (after the cmd is
// built) is sufficient. Workers are stopped before returning to avoid leaks.
func execScheduled(t *testing.T, m Model, cmd tea.Cmd) tea.Msg {
	t.Helper()
	require.NotNil(t, cmd)
	m.scheduler.StartWorkers()
	defer m.scheduler.StopWorkers()
	return cmd()
}

// keyNameCodes maps the key names Bubble Tea prints back to their key codes,
// so keyMsg can round-trip any keystroke string a binding might contain.
var keyNameCodes = map[string]rune{
	"esc": tea.KeyEsc, "escape": tea.KeyEscape, "enter": tea.KeyEnter,
	"tab": tea.KeyTab, "backspace": tea.KeyBackspace, "space": tea.KeySpace,
	"delete": tea.KeyDelete, "insert": tea.KeyInsert,
	"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
	"pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown, "home": tea.KeyHome, "end": tea.KeyEnd,
	"f1": tea.KeyF1, "f2": tea.KeyF2, "f3": tea.KeyF3, "f4": tea.KeyF4,
	"f5": tea.KeyF5, "f6": tea.KeyF6, "f7": tea.KeyF7, "f8": tea.KeyF8,
	"f9": tea.KeyF9, "f10": tea.KeyF10, "f11": tea.KeyF11, "f12": tea.KeyF12,
}

var keyNameMods = map[string]tea.KeyMod{
	"ctrl": tea.ModCtrl, "alt": tea.ModAlt, "shift": tea.ModShift,
	"meta": tea.ModMeta, "hyper": tea.ModHyper, "super": tea.ModSuper,
}

// keyMsg builds the key-press message a terminal would deliver for the
// keystroke string s ("j", "ctrl+u", "shift+tab", "space").
//
// Text is set only for a bare printable character, matching what a real
// terminal does: modified and named keys carry a code but produce no text, so
// text inputs correctly ignore them.
func keyMsg(s string) tea.KeyPressMsg {
	parts := strings.Split(s, "+")
	key, mods := parts[len(parts)-1], parts[:len(parts)-1]
	if key == "" && len(mods) > 0 { // the key itself is "+", e.g. "ctrl++"
		key, mods = "+", mods[:len(mods)-1]
	}

	var mod tea.KeyMod
	for _, m := range mods {
		mod |= keyNameMods[m]
	}

	if code, ok := keyNameCodes[key]; ok {
		k := tea.KeyPressMsg{Code: code, Mod: mod}
		if code == tea.KeySpace && mod == 0 {
			k.Text = " " // a real terminal reports the character it produced
		}
		return k
	}
	if r := []rune(key); len(r) == 1 {
		k := tea.KeyPressMsg{Code: r[0], Mod: mod}
		if mod == 0 {
			k.Text = key
		}
		return k
	}
	// Unknown multi-rune name: a key with no text, so inputs ignore it.
	return tea.KeyPressMsg{Mod: mod}
}

func logContainerOverlayItems(containers []string) []model.Item {
	items := make([]model.Item, 0, 1+len(containers))
	items = append(items, model.Item{Name: "All Containers", Status: "all"})
	for _, c := range containers {
		items = append(items, model.Item{Name: c})
	}
	return items
}

// newFakeScheme creates a runtime.Scheme with core resources registered.
func newFakeScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	return s
}

// newFinalDynClient creates a fake dynamic client with common GVRs registered.
func newFinalDynClient() *dynfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "nodes"}:                                     "NodeList",
		{Group: "", Version: "v1", Resource: "pods"}:                                      "PodList",
		{Group: "", Version: "v1", Resource: "namespaces"}:                                "NamespaceList",
		{Group: "", Version: "v1", Resource: "events"}:                                    "EventList",
		{Group: "", Version: "v1", Resource: "secrets"}:                                   "SecretList",
		{Group: "", Version: "v1", Resource: "configmaps"}:                                "ConfigMapList",
		{Group: "", Version: "v1", Resource: "services"}:                                  "ServiceList",
		{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}:                    "PersistentVolumeClaimList",
		{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}:                "PodDisruptionBudgetList",
		{Group: "apps", Version: "v1", Resource: "deployments"}:                           "DeploymentList",
		{Group: "apps", Version: "v1", Resource: "replicasets"}:                           "ReplicaSetList",
		{Group: "apps", Version: "v1", Resource: "statefulsets"}:                          "StatefulSetList",
		{Group: "apps", Version: "v1", Resource: "daemonsets"}:                            "DaemonSetList",
		{Group: "batch", Version: "v1", Resource: "jobs"}:                                 "JobList",
		{Group: "batch", Version: "v1", Resource: "cronjobs"}:                             "CronJobList",
		{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}:          "NetworkPolicyList",
		{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}:             "ApplicationList",
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"}: "KustomizationList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}:                  "NodeMetricsList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:                   "PodMetricsList",
		{Group: "", Version: "v1", Resource: "resourcequotas"}:                            "ResourceQuotaList",
	}
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs)
}

// newRichDynClient creates a fake dynamic client pre-populated with resources
// for comprehensive dashboard testing.
func newRichDynClient() *dynfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "nodes"}:                      "NodeList",
		{Group: "", Version: "v1", Resource: "pods"}:                       "PodList",
		{Group: "", Version: "v1", Resource: "namespaces"}:                 "NamespaceList",
		{Group: "", Version: "v1", Resource: "events"}:                     "EventList",
		{Group: "", Version: "v1", Resource: "secrets"}:                    "SecretList",
		{Group: "", Version: "v1", Resource: "configmaps"}:                 "ConfigMapList",
		{Group: "", Version: "v1", Resource: "services"}:                   "ServiceList",
		{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}:     "PersistentVolumeClaimList",
		{Group: "", Version: "v1", Resource: "resourcequotas"}:             "ResourceQuotaList",
		{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}: "PodDisruptionBudgetList",
		{Group: "apps", Version: "v1", Resource: "deployments"}:            "DeploymentList",
		{Group: "apps", Version: "v1", Resource: "replicasets"}:            "ReplicaSetList",
		{Group: "apps", Version: "v1", Resource: "statefulsets"}:           "StatefulSetList",
		{Group: "apps", Version: "v1", Resource: "daemonsets"}:             "DaemonSetList",
		{Group: "batch", Version: "v1", Resource: "jobs"}:                  "JobList",
		{Group: "batch", Version: "v1", Resource: "cronjobs"}:              "CronJobList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}:   "NodeMetricsList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:    "PodMetricsList",
	}

	// Create nodes.
	node1 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Node",
		"metadata": map[string]any{"name": "node-1"},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "True"},
			},
			"allocatable": map[string]any{
				"cpu":    "4",
				"memory": "8Gi",
			},
		},
	}}
	node2 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Node",
		"metadata": map[string]any{"name": "node-2"},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False"},
			},
			"allocatable": map[string]any{
				"cpu":    "2",
				"memory": "4Gi",
			},
		},
	}}

	// Create pods with different statuses.
	pod1 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{"name": "pod-running", "namespace": "default"},
		"status":   map[string]any{"phase": "Running"},
	}}
	pod2 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{"name": "pod-pending", "namespace": "default"},
		"status":   map[string]any{"phase": "Pending"},
	}}
	pod3 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]any{"name": "pod-failed", "namespace": "default"},
		"status":   map[string]any{"phase": "Failed"},
	}}

	// Namespaces.
	ns1 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Namespace",
		"metadata": map[string]any{"name": "default"},
	}}
	ns2 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Namespace",
		"metadata": map[string]any{"name": "kube-system"},
	}}

	// Events.
	evt1 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Event",
		"metadata": map[string]any{"name": "evt-warning", "namespace": "default"},
		"type":     "Warning",
		"reason":   "FailedScheduling",
		"message":  "0/2 nodes are available",
		"count":    int64(3),
		"involvedObject": map[string]any{
			"name": "pod-pending",
		},
	}}
	evt2 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Event",
		"metadata": map[string]any{"name": "evt-normal", "namespace": "default"},
		"type":     "Normal",
		"reason":   "Pulled",
		"message":  "Successfully pulled image",
		"involvedObject": map[string]any{
			"name": "pod-running",
		},
	}}

	return dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs,
		node1, node2, pod1, pod2, pod3, ns1, ns2, evt1, evt2)
}

// testModelExec creates a Model with a fake client for exec command tests.
func testModelExec() Model {
	cs := fake.NewClientset()
	dyn := dynfake.NewSimpleDynamicClient(runtime.NewScheme())

	m := Model{
		viewerPrefs:         newViewerPrefValues(), // production toggle defaults, as NewModel seeds them
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		client:              k8s.NewTestClient(cs, dyn),
		namespace:           "default",
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		name:         "test-resource",
		namespace:    "default",
		kind:         "Pod",
		resourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
	}

	return m
}

// withActionCtx sets common action context fields on a model.
// Uses "test-ctx" as the default kube context for tests.
func withActionCtx(m Model, name, ns, kind string, rt model.ResourceTypeEntry) Model {
	m.actionCtx = actionContext{
		name:         name,
		namespace:    ns,
		context:      "test-ctx",
		kind:         kind,
		resourceType: rt,
	}
	return m
}

// withMiddleItem sets a single item in the middle pane so selectedMiddleItem() works.
func withMiddleItem(m Model, item model.Item) Model {
	m.middleItems = []model.Item{item}
	m.setCursor(0)
	return m
}
