package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

func netpolTestPodObj(name, namespace string, lbls map[string]string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: lbls},
	}
}

// netpolsTestModel returns a Model whose dynamic fake knows the pods,
// services, and networkpolicies GVRs, with scheduler workers started.
func netpolsTestModel(t *testing.T, objs ...runtime.Object) Model {
	t.Helper()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:                                      "PodList",
		{Group: "", Version: "v1", Resource: "services"}:                                  "ServiceList",
		{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}:          "NetworkPolicyList",
		{Group: "cilium.io", Version: "v2", Resource: "ciliumnetworkpolicies"}:            "CiliumNetworkPolicyList",
		{Group: "cilium.io", Version: "v2", Resource: "ciliumclusterwidenetworkpolicies"}: "CiliumClusterwideNetworkPolicyList",
	}
	m := baseModelWithFakeDynamic(gvrToListKind, objs...)
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)
	return m
}

func TestLoadNetworkPoliciesForResource_Pod(t *testing.T) {
	m := netpolsTestModel(t,
		netpolTestPodObj("my-pod", "default", map[string]string{"app": "web"}))
	m = withActionCtx(m, "my-pod", "default", "Pod", model.ResourceTypeEntry{})
	cmd := m.loadNetworkPoliciesForResource()
	msg := execCmd(t, cmd)
	result, ok := msg.(netpolsForResourceLoadedMsg)
	require.True(t, ok)
	require.NoError(t, result.err)
	assert.Equal(t, "Pod", result.info.Kind)
	assert.Equal(t, "my-pod", result.info.Name)
	assert.Empty(t, result.info.Policies)
}

func TestLoadNetworkPoliciesForResource_PodNotFound(t *testing.T) {
	m := netpolsTestModel(t)
	m = withActionCtx(m, "missing-pod", "default", "Pod", model.ResourceTypeEntry{})
	cmd := m.loadNetworkPoliciesForResource()
	msg := execCmd(t, cmd)
	result, ok := msg.(netpolsForResourceLoadedMsg)
	require.True(t, ok)
	assert.Error(t, result.err)
}

func TestLoadNetworkPoliciesForResource_Service(t *testing.T) {
	svc := &corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "web"}},
	}
	m := netpolsTestModel(t, svc,
		netpolTestPodObj("web-1", "default", map[string]string{"app": "web"}))
	m = withActionCtx(m, "my-svc", "default", "Service", model.ResourceTypeEntry{})
	cmd := m.loadNetworkPoliciesForResource()
	msg := execCmd(t, cmd)
	result, ok := msg.(netpolsForResourceLoadedMsg)
	require.True(t, ok)
	require.NoError(t, result.err)
	assert.Equal(t, "Service", result.info.Kind)
	assert.Equal(t, []string{"web-1"}, result.info.BackingPods)
}

func TestExecuteActionNetworkPolicies(t *testing.T) {
	m := netpolsTestModel(t,
		netpolTestPodObj("my-pod", "default", map[string]string{"app": "web"}))
	m = withActionCtx(m, "my-pod", "default", "Pod", model.ResourceTypeEntry{})
	mdl, cmd, ok := m.executeActionCoreK8s("Network Policies")
	require.True(t, ok, "Network Policies must be dispatched")
	require.NotNil(t, cmd)
	result := mdl.(Model)
	assert.True(t, result.loading)
}

func TestUpdateNetpolsForResourceLoaded(t *testing.T) {
	m := baseOverlayModel()
	info := &k8s.NetpolsForResource{Kind: "Pod", Name: "my-pod", Namespace: "default"}
	result, _ := m.Update(netpolsForResourceLoadedMsg{info: info})
	mdl := result.(Model)
	assert.Equal(t, overlayNetworkPolicy, mdl.overlay)
	assert.Equal(t, info, mdl.netpolsData)
	assert.Equal(t, 0, mdl.netpolScroll)
	assert.False(t, mdl.loading)
}

func TestUpdateNetpolsForResourceLoadedError(t *testing.T) {
	m := baseOverlayModel()
	result, cmd := m.Update(netpolsForResourceLoadedMsg{err: assert.AnError})
	mdl := result.(Model)
	assert.NotEqual(t, overlayNetworkPolicy, mdl.overlay)
	assert.Nil(t, mdl.netpolsData)
	assert.NotEmpty(t, mdl.statusMessage)
	assert.NotNil(t, cmd)
}

func TestRenderOverlayNetworkPoliciesMulti(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayNetworkPolicy
	m.netpolsData = &k8s.NetpolsForResource{
		Kind:      "Pod",
		Name:      "my-pod",
		Namespace: "default",
		Policies: []k8s.NetpolForResource{
			{
				NetworkPolicyInfo: k8s.NetworkPolicyInfo{
					Name:        "allow-web",
					Namespace:   "default",
					PodSelector: map[string]string{"app": "web"},
					PolicyTypes: []string{"Ingress"},
					IngressRules: []k8s.NetpolRule{
						{
							Ports: []k8s.NetpolPort{{Protocol: "TCP", Port: "80"}},
							Peers: []k8s.NetpolPeer{{Type: "Pod", Selector: map[string]string{"role": "frontend"}}},
						},
					},
				},
			},
		},
	}
	bg := strings.Repeat("bg\n", 10)
	result := stripANSI(m.renderOverlay(bg))
	assert.Contains(t, result, "Pod: my-pod")
	assert.Contains(t, result, "allow-web")
}

func TestRenderOverlayNetworkPoliciesMultiEmpty(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayNetworkPolicy
	m.netpolsData = &k8s.NetpolsForResource{Kind: "Pod", Name: "my-pod", Namespace: "default"}
	bg := strings.Repeat("bg\n", 10)
	result := stripANSI(m.renderOverlay(bg))
	assert.Contains(t, result, "No network policies")
}

func TestLoadNetworkPolicy_CiliumSingleSpec(t *testing.T) {
	cnp := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumNetworkPolicy",
		"metadata":   map[string]any{"name": "cnp-web", "namespace": "default"},
		"spec": map[string]any{
			"endpointSelector": map[string]any{"matchLabels": map[string]any{"app": "web"}},
		},
	}}
	m := netpolsTestModel(t, cnp)
	m = withActionCtx(m, "cnp-web", "default", "CiliumNetworkPolicy", model.ResourceTypeEntry{})
	cmd := m.loadNetworkPolicy()
	msg := execCmd(t, cmd)
	result, ok := msg.(netpolLoadedMsg)
	require.True(t, ok, "single-spec cilium policy must open the single view")
	require.NoError(t, result.err)
	assert.Equal(t, "CiliumNetworkPolicy", result.info.Kind)
}

func TestLoadNetworkPolicy_CiliumMultiSpec(t *testing.T) {
	cnp := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumNetworkPolicy",
		"metadata":   map[string]any{"name": "cnp-multi", "namespace": "default"},
		"specs": []any{
			map[string]any{"endpointSelector": map[string]any{"matchLabels": map[string]any{"app": "a"}}},
			map[string]any{"endpointSelector": map[string]any{"matchLabels": map[string]any{"app": "b"}}},
		},
	}}
	m := netpolsTestModel(t, cnp)
	m = withActionCtx(m, "cnp-multi", "default", "CiliumNetworkPolicy", model.ResourceTypeEntry{})
	cmd := m.loadNetworkPolicy()
	msg := execCmd(t, cmd)
	result, ok := msg.(netpolsForResourceLoadedMsg)
	require.True(t, ok, "multi-spec cilium policy must open the stacked view")
	require.NoError(t, result.err)
	assert.Equal(t, "CiliumNetworkPolicy", result.info.Kind)
	assert.Len(t, result.info.Policies, 2)
}

// tallNetpolModel returns a model showing a multi-policy overlay whose
// content is much taller than the viewport, so scrolling is exercised.
func tallNetpolModel() Model {
	m := baseOverlayModel()
	m.overlay = overlayNetworkPolicy
	policies := make([]k8s.NetpolForResource, 10)
	for i := range policies {
		policies[i] = k8s.NetpolForResource{NetworkPolicyInfo: k8s.NetworkPolicyInfo{
			Name:        "policy-" + string(rune('a'+i)),
			Namespace:   "default",
			PolicyTypes: []string{"Ingress", "Egress"},
		}}
	}
	m.netpolsData = &k8s.NetpolsForResource{
		Kind: "Pod", Name: "my-pod", Namespace: "default", Policies: policies,
	}
	return m
}

// Pressing down past the bottom must not run the scroll counter ahead: a
// single up keypress afterwards must immediately move the view.
func TestNetpolOverlayScrollClampedAtBottom(t *testing.T) {
	m := tallNetpolModel()
	maxScroll := m.netpolMaxScroll()
	require.Positive(t, maxScroll)

	for range maxScroll + 50 {
		m, _ = m.handleNetworkPolicyOverlayKey(tea.KeyPressMsg{Code: 'j', Text: "j"})
	}
	assert.Equal(t, maxScroll, m.netpolScroll, "j must clamp at the bottom")

	m, _ = m.handleNetworkPolicyOverlayKey(tea.KeyPressMsg{Code: 'k', Text: "k"})
	assert.Equal(t, maxScroll-1, m.netpolScroll, "first k after the bottom must scroll up")
}

func TestNetpolOverlayPagingClampedAtBottom(t *testing.T) {
	m := tallNetpolModel()
	maxScroll := m.netpolMaxScroll()
	require.Positive(t, maxScroll)

	for range 20 {
		m, _ = m.handleNetworkPolicyOverlayKey(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	}
	assert.Equal(t, maxScroll, m.netpolScroll, "ctrl+f must clamp at the bottom")

	for range 20 {
		m, _ = m.handleNetworkPolicyOverlayKey(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	}
	assert.Equal(t, maxScroll, m.netpolScroll, "ctrl+d must clamp at the bottom")
}

func TestNetpolOverlayJumpToBottomClamped(t *testing.T) {
	m := tallNetpolModel()
	maxScroll := m.netpolMaxScroll()

	m, _ = m.handleNetworkPolicyOverlayKey(tea.KeyPressMsg{Code: 'G', Text: "G"})
	assert.Equal(t, maxScroll, m.netpolScroll, "G must land exactly on the bottom")

	m.netpolScroll = 0
	m, _ = m.handleNetworkPolicyOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnd})
	assert.Equal(t, maxScroll, m.netpolScroll, "end must land exactly on the bottom")
}

func TestNetpolOverlayEscClearsMultiData(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayNetworkPolicy
	m.netpolsData = &k8s.NetpolsForResource{Kind: "Pod", Name: "my-pod", Namespace: "default"}
	result, _ := m.handleNetworkPolicyOverlayKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	assert.Equal(t, overlayNone, result.overlay)
	assert.Nil(t, result.netpolsData)
}
