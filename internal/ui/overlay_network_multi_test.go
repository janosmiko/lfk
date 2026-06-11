package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// --- RenderNetworkPoliciesOverlay ---

func TestRenderNetworkPoliciesOverlay_PodWithPolicies(t *testing.T) {
	info := ResourceNetpolsEntry{
		Kind:      "Pod",
		Name:      "web-1",
		Namespace: "default",
		Policies: []NetworkPolicyEntry{
			{
				Name:        "allow-web",
				Namespace:   "default",
				PodSelector: map[string]string{"app": "web"},
				PolicyTypes: []string{"Ingress"},
				IngressRules: []NetpolRuleEntry{
					{
						Ports: []NetpolPortEntry{{Protocol: "TCP", Port: "80"}},
						Peers: []NetpolPeerEntry{{Type: "Pod", Selector: map[string]string{"app": "db"}}},
					},
				},
			},
			{
				Name:        "default-deny",
				Namespace:   "default",
				PolicyTypes: []string{"Ingress", "Egress"},
			},
		},
	}
	result := RenderNetworkPoliciesOverlay(info, 0, 80, 60)
	assert.Contains(t, result, "Pod: web-1")
	assert.Contains(t, result, "allow-web")
	assert.Contains(t, result, "default-deny")
	assert.Contains(t, result, "2 network policies")
}

func TestRenderNetworkPoliciesOverlay_NoPolicies(t *testing.T) {
	info := ResourceNetpolsEntry{
		Kind:      "Pod",
		Name:      "web-1",
		Namespace: "default",
	}
	result := RenderNetworkPoliciesOverlay(info, 0, 80, 40)
	assert.Contains(t, result, "No network policies")
	assert.Contains(t, result, "all traffic allowed")
}

func TestRenderNetworkPoliciesOverlay_ServiceCoveredPods(t *testing.T) {
	info := ResourceNetpolsEntry{
		Kind:        "Service",
		Name:        "web-svc",
		Namespace:   "default",
		BackingPods: []string{"web-1", "web-2"},
		Policies: []NetworkPolicyEntry{
			{
				Name:        "canary-only",
				Namespace:   "default",
				PodSelector: map[string]string{"track": "canary"},
				MatchedPods: []string{"web-2"},
			},
		},
	}
	result := RenderNetworkPoliciesOverlay(info, 0, 80, 60)
	assert.Contains(t, result, "Service: web-svc")
	assert.Contains(t, result, "canary-only")
	// Partial coverage must be visible: the policy covers 1 of 2 backing pods.
	assert.Contains(t, result, "1 of 2 backing pod(s)")
	assert.Contains(t, result, "web-2")
}

// wideNetpolEntry returns a policy whose diagram boxes hit the width cap, to
// exercise truncation paths.
func wideNetpolEntry() NetworkPolicyEntry {
	return NetworkPolicyEntry{
		Name:        "very-long-network-policy-name-that-keeps-going",
		Namespace:   "default",
		PodSelector: map[string]string{"app.kubernetes.io/name": "a-very-long-application-name"},
		PolicyTypes: []string{"Ingress", "Egress"},
		IngressRules: []NetpolRuleEntry{
			{
				Ports: []NetpolPortEntry{{Protocol: "TCP", Port: "8080"}},
				Peers: []NetpolPeerEntry{{
					Type:      "Namespace+Pod",
					Selector:  map[string]string{"app.kubernetes.io/component": "an-extremely-long-component-value"},
					Namespace: "team-with-a-very-long-namespace-label-value",
				}},
			},
		},
		EgressRules: []NetpolRuleEntry{
			{
				Peers: []NetpolPeerEntry{{Type: "CIDR", CIDR: "10.0.0.0/8", Except: []string{"10.0.1.0/24"}}},
			},
		},
	}
}

// Every rendered line must fit within the requested width at every scroll
// position; over-wide lines wrap inside the overlay frame and change its
// height while scrolling.
func TestRenderNetworkPolicyOverlay_LinesNeverExceedWidth(t *testing.T) {
	const width, height = 60, 12
	info := wideNetpolEntry()
	total := NetworkPolicyOverlayLineCount(info, width)
	for scroll := 0; scroll <= total; scroll++ {
		out := RenderNetworkPolicyOverlay(info, scroll, width, height)
		for line := range strings.SplitSeq(out, "\n") {
			assert.LessOrEqual(t, lipgloss.Width(line), width,
				"scroll=%d line %q exceeds width", scroll, line)
		}
	}
}

// The rendered height must not depend on the scroll position.
func TestRenderNetworkPolicyOverlay_HeightStableAcrossScroll(t *testing.T) {
	const width, height = 60, 12
	info := wideNetpolEntry()
	base := strings.Count(RenderNetworkPolicyOverlay(info, 0, width, height), "\n")
	total := NetworkPolicyOverlayLineCount(info, width)
	for scroll := 1; scroll <= total; scroll++ {
		got := strings.Count(RenderNetworkPolicyOverlay(info, scroll, width, height), "\n")
		assert.Equal(t, base, got, "height changed at scroll=%d", scroll)
	}
}

// Short titles are the regression case: OverlayTitleStyle carries bottom
// padding, so its Render output spans two terminal rows. If that lands in
// the line list as one entry, the overlay height shifts by a row whenever
// it scrolls in or out of view. (Long titles masked this: width truncation
// cut the line before the embedded newline.)
func TestRenderNetworkPolicyOverlay_HeightStableWithShortTitle(t *testing.T) {
	const width, height = 80, 10
	info := NetworkPolicyEntry{
		Name:        "vault",
		Namespace:   "vault",
		PolicyTypes: []string{"Ingress"},
		IngressRules: []NetpolRuleEntry{
			{Peers: []NetpolPeerEntry{{Type: "All"}}},
			{Peers: []NetpolPeerEntry{{Type: "All"}}},
			{Peers: []NetpolPeerEntry{{Type: "All"}}},
		},
	}
	base := strings.Count(RenderNetworkPolicyOverlay(info, 0, width, height), "\n")
	total := NetworkPolicyOverlayLineCount(info, width)
	for scroll := 1; scroll <= total; scroll++ {
		got := strings.Count(RenderNetworkPolicyOverlay(info, scroll, width, height), "\n")
		assert.Equal(t, base, got, "height changed at scroll=%d", scroll)
	}
}

func TestRenderNetworkPoliciesOverlay_HeightStableWithShortTitle(t *testing.T) {
	const width, height = 80, 10
	info := ResourceNetpolsEntry{
		Kind:      "Pod",
		Name:      "vault-3",
		Namespace: "vault",
		Policies: []NetworkPolicyEntry{
			{
				Name: "vault", Namespace: "vault", PolicyTypes: []string{"Ingress"},
				IngressRules: []NetpolRuleEntry{
					{Peers: []NetpolPeerEntry{{Type: "All"}}},
					{Peers: []NetpolPeerEntry{{Type: "All"}}},
				},
			},
		},
	}
	base := strings.Count(RenderNetworkPoliciesOverlay(info, 0, width, height), "\n")
	total := NetworkPoliciesOverlayLineCount(info, width)
	for scroll := 1; scroll <= total; scroll++ {
		got := strings.Count(RenderNetworkPoliciesOverlay(info, scroll, width, height), "\n")
		assert.Equal(t, base, got, "height changed at scroll=%d", scroll)
	}
}

func TestRenderNetworkPoliciesOverlay_LinesNeverExceedWidth(t *testing.T) {
	const width, height = 60, 12
	info := ResourceNetpolsEntry{
		Kind:        "Service",
		Name:        "a-service-with-quite-a-long-name",
		Namespace:   "default",
		BackingPods: []string{"web-1", "web-2"},
		Policies:    []NetworkPolicyEntry{wideNetpolEntry(), wideNetpolEntry()},
	}
	total := NetworkPoliciesOverlayLineCount(info, width)
	for scroll := 0; scroll <= total; scroll++ {
		out := RenderNetworkPoliciesOverlay(info, scroll, width, height)
		for line := range strings.SplitSeq(out, "\n") {
			assert.LessOrEqual(t, lipgloss.Width(line), width,
				"scroll=%d line %q exceeds width", scroll, line)
		}
	}
}

func TestOverlayMaxScroll(t *testing.T) {
	// 50 lines in a height-12 viewport leaves 38 hidden.
	assert.Equal(t, 38, OverlayMaxScroll(50, 12))
	// Content shorter than the viewport never scrolls.
	assert.Equal(t, 0, OverlayMaxScroll(5, 12))
}

// Overflowing content shows the shared right-edge scrollbar instead of a
// [n/m] line indicator.
func TestRenderNetworkPolicyOverlay_ScrollbarReplacesIndicator(t *testing.T) {
	const width, height = 80, 10
	info := wideNetpolEntry()
	out := RenderNetworkPolicyOverlay(info, 0, width, height)
	assert.Contains(t, out, "█", "overflowing content must show a scrollbar thumb")
	assert.Contains(t, out, "│", "overflowing content must show the scrollbar track")
	assert.NotContains(t, out, "[1/", "the [n/m] indicator must be gone")

	// The thumb must move as the view scrolls: at the bottom the first
	// visible row is track, not thumb.
	bottom := RenderNetworkPolicyOverlay(info, OverlayMaxScroll(NetworkPolicyOverlayLineCount(info, width), height), width, height)
	topRows := strings.SplitN(out, "\n", 2)
	bottomRows := strings.SplitN(bottom, "\n", 2)
	assert.True(t, strings.HasSuffix(topRows[0], "█"), "thumb starts at the top row")
	assert.False(t, strings.HasSuffix(bottomRows[0], "█"), "thumb must leave the top row when scrolled to the bottom")
}

func TestRenderNetworkPolicyOverlay_NoScrollbarWhenContentFits(t *testing.T) {
	info := NetworkPolicyEntry{Name: "tiny", Namespace: "default"}
	out := RenderNetworkPolicyOverlay(info, 0, 80, 40)
	assert.NotContains(t, out, "█")
}

func TestRenderNetworkPoliciesOverlay_ServiceNoSelector(t *testing.T) {
	info := ResourceNetpolsEntry{
		Kind:       "Service",
		Name:       "external-svc",
		Namespace:  "default",
		NoSelector: true,
	}
	result := RenderNetworkPoliciesOverlay(info, 0, 80, 40)
	assert.Contains(t, result, "no pod selector")
}
