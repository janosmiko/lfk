package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- parseNetpolRule: missing branches ---

func TestParseNetpolRule_PortEntryNotMap(t *testing.T) {
	// A port entry that is not a map should be skipped (line 127-128).
	rule := parseNetpolRule(map[string]any{
		"ports": []any{
			"not-a-map",
			map[string]any{
				"port": 80,
			},
		},
	}, "from")

	assert.Len(t, rule.Ports, 1, "non-map port entries should be skipped")
	assert.Equal(t, "80", rule.Ports[0].Port)
	assert.Equal(t, "TCP", rule.Ports[0].Protocol, "default protocol should be TCP")
}

func TestParseNetpolRule_PeerEntryNotMap(t *testing.T) {
	// A peer entry that is not a map should be skipped (line 148-149).
	rule := parseNetpolRule(map[string]any{
		"from": []any{
			"not-a-map",
			map[string]any{
				"podSelector": map[string]any{
					"matchLabels": map[string]any{
						"app": "web",
					},
				},
			},
		},
	}, "from")

	assert.Len(t, rule.Peers, 1, "non-map peer entries should be skipped")
	assert.Equal(t, "Pod", rule.Peers[0].Type)
}

func TestParseNetpolRule_EgressDirection(t *testing.T) {
	// Ensure the "to" peerField works correctly for egress rules.
	rule := parseNetpolRule(map[string]any{
		"to": []any{
			map[string]any{
				"podSelector": map[string]any{
					"matchLabels": map[string]any{
						"role": "db",
					},
				},
			},
		},
		"ports": []any{
			map[string]any{
				"protocol": "UDP",
				"port":     5432,
			},
		},
	}, "to")

	assert.Len(t, rule.Peers, 1)
	assert.Equal(t, "Pod", rule.Peers[0].Type)
	assert.Equal(t, map[string]string{"role": "db"}, rule.Peers[0].Selector)
	assert.Len(t, rule.Ports, 1)
	assert.Equal(t, "UDP", rule.Ports[0].Protocol)
	assert.Equal(t, "5432", rule.Ports[0].Port)
}

// --- parsePeer: missing branches ---

func TestParsePeer_EmptyPeerMap(t *testing.T) {
	// A completely empty peer map should resolve to "All" type (line 216-217).
	peer := parsePeer(map[string]any{})
	assert.Equal(t, "All", peer.Type)
}

func TestParsePeer_PodSelectorWithoutMatchLabels(t *testing.T) {
	// A podSelector without matchLabels should still be typed as "Pod"
	// but have no Selector labels (line 198-206).
	peer := parsePeer(map[string]any{
		"podSelector": map[string]any{},
	})
	assert.Equal(t, "Pod", peer.Type)
	assert.Nil(t, peer.Selector, "pod selector without matchLabels should have nil Selector")
}

func TestParsePeer_CIDRWithoutExcept(t *testing.T) {
	// An ipBlock with cidr but no except list.
	peer := parsePeer(map[string]any{
		"ipBlock": map[string]any{
			"cidr": "172.16.0.0/12",
		},
	})
	assert.Equal(t, "CIDR", peer.Type)
	assert.Equal(t, "172.16.0.0/12", peer.CIDR)
	assert.Nil(t, peer.Except)
}

func TestParsePeer_CIDRWithoutCidrField(t *testing.T) {
	// An ipBlock map exists but has no "cidr" key.
	peer := parsePeer(map[string]any{
		"ipBlock": map[string]any{},
	})
	assert.Equal(t, "CIDR", peer.Type)
	assert.Equal(t, "", peer.CIDR)
}

func TestParsePeer_NamespaceAndPodSelectorBothEmpty(t *testing.T) {
	// Both selectors present but with empty matchLabels.
	peer := parsePeer(map[string]any{
		"namespaceSelector": map[string]any{},
		"podSelector":       map[string]any{},
	})
	assert.Equal(t, "Namespace+Pod", peer.Type)
	assert.Equal(t, "(all namespaces)", peer.Namespace)
	assert.Nil(t, peer.Selector)
}
