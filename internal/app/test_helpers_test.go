package app

import (
	"context"
	"sync"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	tea "github.com/charmbracelet/bubbletea"
	corev1 "k8s.io/api/core/v1"
	dynfake "k8s.io/client-go/dynamic/fake"
	fake "k8s.io/client-go/kubernetes/fake"
)

// baseFinalModel returns a Model with fake K8s client for final push tests.
func baseFinalModel() Model {
	cs := fake.NewClientset()
	dyn := dynfake.NewSimpleDynamicClient(runtime.NewScheme())

	m := Model{
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
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
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
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
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
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
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
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

// baseModelCov returns a minimal Model for coverage tests.
func baseModelCov() Model {
	return Model{
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
}

func baseModelDescribe() Model {
	m := Model{
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
	m.describeContent = "line0\nline1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9"
	m.mode = modeDescribe
	return m
}

func baseModelExplain() Model {
	m := Model{
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
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
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
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
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
	return m
}

func baseModelNav() Model {
	m := Model{
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
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
	return m
}

func baseModelSearch() Model {
	m := Model{
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
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
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
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
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
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
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-ctx",
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		namespace:           "default",
		reqCtx:              context.Background(),
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
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-ctx",
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		namespace:           "default",
		reqCtx:              context.Background(),
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
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-ctx",
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		namespace:           "default",
		reqCtx:              context.Background(),
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
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
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
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
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

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case "ctrl+f":
		return tea.KeyMsg{Type: tea.KeyCtrlF}
	case "ctrl+b":
		return tea.KeyMsg{Type: tea.KeyCtrlB}
	case "ctrl+w":
		return tea.KeyMsg{Type: tea.KeyCtrlW}
	case "ctrl+a":
		return tea.KeyMsg{Type: tea.KeyCtrlA}
	case "ctrl+e":
		return tea.KeyMsg{Type: tea.KeyCtrlE}
	case "ctrl+n":
		return tea.KeyMsg{Type: tea.KeyCtrlN}
	case "ctrl+p":
		return tea.KeyMsg{Type: tea.KeyCtrlP}
	case "ctrl+v":
		return tea.KeyMsg{Type: tea.KeyCtrlV}
	default:
		if len(s) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
		}
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
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
	node1 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Node",
		"metadata": map[string]interface{}{"name": "node-1"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
			"allocatable": map[string]interface{}{
				"cpu":    "4",
				"memory": "8Gi",
			},
		},
	}}
	node2 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Node",
		"metadata": map[string]interface{}{"name": "node-2"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "False"},
			},
			"allocatable": map[string]interface{}{
				"cpu":    "2",
				"memory": "4Gi",
			},
		},
	}}

	// Create pods with different statuses.
	pod1 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]interface{}{"name": "pod-running", "namespace": "default"},
		"status":   map[string]interface{}{"phase": "Running"},
	}}
	pod2 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]interface{}{"name": "pod-pending", "namespace": "default"},
		"status":   map[string]interface{}{"phase": "Pending"},
	}}
	pod3 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]interface{}{"name": "pod-failed", "namespace": "default"},
		"status":   map[string]interface{}{"phase": "Failed"},
	}}

	// Namespaces.
	ns1 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Namespace",
		"metadata": map[string]interface{}{"name": "default"},
	}}
	ns2 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Namespace",
		"metadata": map[string]interface{}{"name": "kube-system"},
	}}

	// Events.
	evt1 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Event",
		"metadata": map[string]interface{}{"name": "evt-warning", "namespace": "default"},
		"type":     "Warning",
		"reason":   "FailedScheduling",
		"message":  "0/2 nodes are available",
		"count":    int64(3),
		"involvedObject": map[string]interface{}{
			"name": "pod-pending",
		},
	}}
	evt2 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Event",
		"metadata": map[string]interface{}{"name": "evt-normal", "namespace": "default"},
		"type":     "Normal",
		"reason":   "Pulled",
		"message":  "Successfully pulled image",
		"involvedObject": map[string]interface{}{
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
		nav:                 model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
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
