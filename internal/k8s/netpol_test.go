package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNetpolRule_EmptyRule(t *testing.T) {
	rule := parseNetpolRule(map[string]any{}, "from")
	assert.Empty(t, rule.Ports)
	assert.Len(t, rule.Peers, 1, "empty rule should have one 'All' peer")
	assert.Equal(t, "All", rule.Peers[0].Type)
}

func TestParseNetpolRule_InvalidInput(t *testing.T) {
	rule := parseNetpolRule("not a map", "from")
	assert.Empty(t, rule.Ports)
	assert.Empty(t, rule.Peers)
}

func TestParseNetpolRule_WithPorts(t *testing.T) {
	rule := parseNetpolRule(map[string]any{
		"ports": []any{
			map[string]any{
				"protocol": "TCP",
				"port":     8080,
			},
			map[string]any{
				"port": "https",
			},
		},
	}, "from")

	assert.Len(t, rule.Ports, 2)
	assert.Equal(t, "TCP", rule.Ports[0].Protocol)
	assert.Equal(t, "8080", rule.Ports[0].Port)
	// Default protocol is TCP.
	assert.Equal(t, "TCP", rule.Ports[1].Protocol)
	assert.Equal(t, "https", rule.Ports[1].Port)
}

func TestParseNetpolRule_WithPodSelector(t *testing.T) {
	rule := parseNetpolRule(map[string]any{
		"from": []any{
			map[string]any{
				"podSelector": map[string]any{
					"matchLabels": map[string]any{
						"app": "frontend",
					},
				},
			},
		},
	}, "from")

	assert.Len(t, rule.Peers, 1)
	assert.Equal(t, "Pod", rule.Peers[0].Type)
	assert.Equal(t, map[string]string{"app": "frontend"}, rule.Peers[0].Selector)
}

func TestParseNetpolRule_WithNamespaceSelector(t *testing.T) {
	rule := parseNetpolRule(map[string]any{
		"from": []any{
			map[string]any{
				"namespaceSelector": map[string]any{
					"matchLabels": map[string]any{
						"env": "production",
					},
				},
			},
		},
	}, "from")

	assert.Len(t, rule.Peers, 1)
	assert.Equal(t, "Namespace", rule.Peers[0].Type)
	assert.Equal(t, "env=production", rule.Peers[0].Namespace)
}

func TestParseNetpolRule_WithNamespaceAndPodSelector(t *testing.T) {
	rule := parseNetpolRule(map[string]any{
		"from": []any{
			map[string]any{
				"namespaceSelector": map[string]any{
					"matchLabels": map[string]any{
						"env": "staging",
					},
				},
				"podSelector": map[string]any{
					"matchLabels": map[string]any{
						"role": "api",
					},
				},
			},
		},
	}, "from")

	assert.Len(t, rule.Peers, 1)
	assert.Equal(t, "Namespace+Pod", rule.Peers[0].Type)
	assert.Equal(t, "env=staging", rule.Peers[0].Namespace)
	assert.Equal(t, map[string]string{"role": "api"}, rule.Peers[0].Selector)
}

func TestParseNetpolRule_WithCIDR(t *testing.T) {
	rule := parseNetpolRule(map[string]any{
		"to": []any{
			map[string]any{
				"ipBlock": map[string]any{
					"cidr": "10.0.0.0/8",
					"except": []any{
						"10.0.1.0/24",
						"10.0.2.0/24",
					},
				},
			},
		},
	}, "to")

	assert.Len(t, rule.Peers, 1)
	assert.Equal(t, "CIDR", rule.Peers[0].Type)
	assert.Equal(t, "10.0.0.0/8", rule.Peers[0].CIDR)
	assert.Equal(t, []string{"10.0.1.0/24", "10.0.2.0/24"}, rule.Peers[0].Except)
}

func TestParseNetpolRule_MultiplePeers(t *testing.T) {
	rule := parseNetpolRule(map[string]any{
		"from": []any{
			map[string]any{
				"podSelector": map[string]any{
					"matchLabels": map[string]any{"app": "web"},
				},
			},
			map[string]any{
				"ipBlock": map[string]any{
					"cidr": "192.168.0.0/16",
				},
			},
		},
		"ports": []any{
			map[string]any{
				"protocol": "TCP",
				"port":     443,
			},
		},
	}, "from")

	assert.Len(t, rule.Peers, 2)
	assert.Equal(t, "Pod", rule.Peers[0].Type)
	assert.Equal(t, "CIDR", rule.Peers[1].Type)
	assert.Len(t, rule.Ports, 1)
	assert.Equal(t, "443", rule.Ports[0].Port)
}

func TestParsePeer_EmptyNamespaceSelector(t *testing.T) {
	peer := parsePeer(map[string]any{
		"namespaceSelector": map[string]any{},
	})
	assert.Equal(t, "Namespace", peer.Type)
	assert.Equal(t, "(all namespaces)", peer.Namespace)
}

func TestFormatLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{"empty", map[string]string{}, "(all)"},
		{"single", map[string]string{"app": "web"}, "app=web"},
		{"multiple sorted", map[string]string{"env": "prod", "app": "web"}, "app=web, env=prod"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLabels(tt.labels)
			assert.Equal(t, tt.want, got)
		})
	}
}
