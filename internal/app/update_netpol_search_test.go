package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
)

// searchableNetpolModel returns a netpol overlay model whose content has a
// unique EGRESS section far below the first viewport, so search jumps are
// observable as scroll movement.
func searchableNetpolModel() Model {
	rules := make([]k8s.NetpolRule, 20)
	for i := range rules {
		rules[i] = k8s.NetpolRule{Peers: []k8s.NetpolPeer{{Type: "All"}}}
	}
	return Model{
		overlay: overlayNetworkPolicy,
		netpolData: &k8s.NetworkPolicyInfo{
			Name: "searchable-policy", Namespace: "default",
			PolicyTypes:  []string{"Ingress", "Egress"},
			IngressRules: rules,
			EgressRules:  []k8s.NetpolRule{{Peers: []k8s.NetpolPeer{{Type: "CIDR", CIDR: "10.1.2.0/24"}}}},
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
}

func typeNetpolSearch(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		var cmd tea.Cmd
		m, cmd = m.handleNetworkPolicyOverlayKey(runeKey(r))
		assert.Nil(t, cmd)
	}
	return m
}

func TestNetpolSearchActivateTypeCommitJumps(t *testing.T) {
	m := searchableNetpolModel()

	m, _ = m.handleNetworkPolicyOverlayKey(runeKey('/'))
	require.True(t, m.netpolSearchActive)

	m = typeNetpolSearch(t, m, "egress")
	assert.Equal(t, "egress", m.netpolSearchQuery, "query must live-update while typing")

	m, _ = m.handleNetworkPolicyOverlayKey(keyMsg("enter"))
	assert.False(t, m.netpolSearchActive)
	assert.Equal(t, "egress", m.netpolSearchQuery)
	assert.Positive(t, m.netpolScroll, "enter must jump to the EGRESS RULES section below the first viewport")
}

func TestNetpolSearchScrollKeysInert(t *testing.T) {
	m := searchableNetpolModel()
	m, _ = m.handleNetworkPolicyOverlayKey(runeKey('/'))

	m, _ = m.handleNetworkPolicyOverlayKey(runeKey('j'))
	assert.Equal(t, 0, m.netpolScroll, "j must type into the search bar, not scroll")
	assert.Equal(t, "j", m.netpolSearchInput.Value)
}

func TestNetpolSearchNextPrevWraps(t *testing.T) {
	m := searchableNetpolModel()
	m.netpolSearchQuery = "rules" // matches INGRESS RULES + EGRESS RULES headings

	m, _ = m.handleNetworkPolicyOverlayKey(runeKey('n'))
	first := m.netpolScroll
	m, _ = m.handleNetworkPolicyOverlayKey(runeKey('n'))
	second := m.netpolScroll
	assert.Greater(t, second, first, "n must advance to the next match")

	m, _ = m.handleNetworkPolicyOverlayKey(runeKey('N'))
	assert.Equal(t, first, m.netpolScroll, "N must return to the previous match")

	m, _ = m.handleNetworkPolicyOverlayKey(runeKey('n')) // forward to the last match again
	m, _ = m.handleNetworkPolicyOverlayKey(runeKey('n')) // past the last match
	assert.Equal(t, first, m.netpolScroll, "n past the last match must wrap to the first")
}

func TestNetpolSearchNotFoundSetsStatus(t *testing.T) {
	m := searchableNetpolModel()
	m, _ = m.handleNetworkPolicyOverlayKey(runeKey('/'))
	m = typeNetpolSearch(t, m, "zzz-no-such-text")
	m, cmd := m.handleNetworkPolicyOverlayKey(keyMsg("enter"))
	assert.Contains(t, m.statusMessage, "Pattern not found")
	assert.NotNil(t, cmd, "not-found must schedule a status clear")
	assert.Equal(t, 0, m.netpolScroll)
}

func TestNetpolSearchEscWhileTypingCancels(t *testing.T) {
	m := searchableNetpolModel()
	m, _ = m.handleNetworkPolicyOverlayKey(runeKey('/'))
	m = typeNetpolSearch(t, m, "egr")
	m, _ = m.handleNetworkPolicyOverlayKey(keyMsg("esc"))
	assert.False(t, m.netpolSearchActive)
	assert.Empty(t, m.netpolSearchQuery)
	assert.Equal(t, overlayNetworkPolicy, m.overlay, "esc in the search bar must not close the overlay")
}

func TestNetpolSearchEscClearsThenCloses(t *testing.T) {
	m := searchableNetpolModel()
	m.netpolSearchQuery = "egress"

	m, _ = m.handleNetworkPolicyOverlayKey(keyMsg("esc"))
	assert.Empty(t, m.netpolSearchQuery, "first esc clears the committed query")
	assert.Equal(t, overlayNetworkPolicy, m.overlay)

	m, _ = m.handleNetworkPolicyOverlayKey(keyMsg("esc"))
	assert.Equal(t, overlayNone, m.overlay, "second esc closes the overlay")
	assert.Nil(t, m.netpolData)
}

func TestNetpolQClosesAndResetsSearchState(t *testing.T) {
	m := searchableNetpolModel()
	m.netpolSearchQuery = "egress"
	m.netpolSearchInput.Set("egress")

	m, _ = m.handleNetworkPolicyOverlayKey(runeKey('q'))
	assert.Equal(t, overlayNone, m.overlay)
	assert.Empty(t, m.netpolSearchQuery)
	assert.Empty(t, m.netpolSearchInput.Value)
}

func TestNetpolWheelScrollsOverlay(t *testing.T) {
	m := searchableNetpolModel()
	require.Positive(t, m.netpolMaxScroll())

	mdl, _ := m.handleOverlayMouse(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m = mdl.(Model)
	assert.Equal(t, 3, m.netpolScroll, "wheel down must scroll 3 lines")

	mdl, _ = m.handleOverlayMouse(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	m = mdl.(Model)
	assert.Equal(t, 0, m.netpolScroll, "wheel up must scroll back 3 lines")

	mdl, _ = m.handleOverlayMouse(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	m = mdl.(Model)
	assert.Equal(t, 0, m.netpolScroll, "wheel up at the top must clamp at 0")

	m.netpolScroll = m.netpolMaxScroll()
	mdl, _ = m.handleOverlayMouse(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m = mdl.(Model)
	assert.Equal(t, m.netpolMaxScroll(), m.netpolScroll, "wheel down at the bottom must clamp at maxScroll")
}

func TestNetpolWheelMultiPolicyView(t *testing.T) {
	rules := make([]k8s.NetpolRule, 25)
	for i := range rules {
		rules[i] = k8s.NetpolRule{Peers: []k8s.NetpolPeer{{Type: "All"}}}
	}
	m := Model{
		overlay: overlayNetworkPolicy,
		netpolsData: &k8s.NetpolsForResource{
			Kind: "Pod", Name: "web-0", Namespace: "default",
			Policies: []k8s.NetpolForResource{{
				Name: "p1", Namespace: "default",
				PolicyTypes:  []string{"Ingress"},
				IngressRules: rules,
			}},
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	require.Positive(t, m.netpolMaxScroll())

	mdl, _ := m.handleOverlayMouse(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m = mdl.(Model)
	assert.Equal(t, 3, m.netpolScroll)
}

func TestNetpolReopenResetsStaleSearchState(t *testing.T) {
	// Universal ctrl+c closes overlays via closeCurrentOverlay, bypassing
	// closeNetpolOverlay — reopening must not resurrect stale search state.
	m := searchableNetpolModel()
	m.netpolSearchActive = true
	m.netpolSearchQuery = "egress"
	m.netpolSearchInput.Set("egress")
	m.netpolSearchPos = 7
	m.overlay = overlayNone

	mdl, _ := m.updateNetpolLoaded(netpolLoadedMsg{info: &k8s.NetworkPolicyInfo{Name: "p"}})
	got := mdl.(Model)
	assert.Equal(t, overlayNetworkPolicy, got.overlay)
	assert.False(t, got.netpolSearchActive)
	assert.Empty(t, got.netpolSearchQuery)
	assert.Empty(t, got.netpolSearchInput.Value)
	assert.Equal(t, 0, got.netpolSearchPos)

	mdl, _ = m.updateNetpolsForResourceLoaded(netpolsForResourceLoadedMsg{info: &k8s.NetpolsForResource{Kind: "Pod", Name: "w"}})
	got = mdl.(Model)
	assert.False(t, got.netpolSearchActive)
	assert.Empty(t, got.netpolSearchQuery)
}

func TestNetpolSlashClearsPendingG(t *testing.T) {
	m := searchableNetpolModel()
	m.pendingG = true
	m, _ = m.handleNetworkPolicyOverlayKey(runeKey('/'))
	assert.False(t, m.pendingG, "/ must clear a pending g so esc+g later doesn't jump to top")
}
