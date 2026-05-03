package k8s

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// findColumnValue returns the first column matching key, or "" when absent.
// Helper kept private to this test file to avoid noisy public surface.
func findColumnValue(cols []model.KeyValue, key string) string {
	for _, kv := range cols {
		if kv.Key == key {
			return kv.Value
		}
	}
	return ""
}

// --- formatEndpointLine ---

func TestFormatEndpointLine_Ready(t *testing.T) {
	got := formatEndpointLine("192.168.1.5", "Pod", "foo-7d9", "node-a", true)
	assert.Equal(t, "192.168.1.5 → pod/foo-7d9 on node-a", got,
		"ready endpoints render without a state suffix so the user's eye is drawn to broken ones")
}

func TestFormatEndpointLine_NotReady(t *testing.T) {
	got := formatEndpointLine("192.168.1.5", "Pod", "foo-7d9", "node-a", false)
	assert.Equal(t, "192.168.1.5 → pod/foo-7d9 on node-a (NotReady)", got,
		"not-ready endpoints append a (NotReady) marker")
}

func TestFormatEndpointLine_NoTargetRef(t *testing.T) {
	got := formatEndpointLine("10.0.0.1", "", "", "", true)
	assert.Equal(t, "10.0.0.1", got,
		"address with no target ref renders bare — common for headless services with manually registered endpoints")
}

func TestFormatEndpointLine_NoNode(t *testing.T) {
	got := formatEndpointLine("192.168.1.5", "Pod", "foo-7d9", "", true)
	assert.Equal(t, "192.168.1.5 → pod/foo-7d9", got,
		"endpoints without a nodeName drop the 'on <node>' segment cleanly")
}

func TestFormatEndpointLine_LowercasesKindForKubectlStyle(t *testing.T) {
	got := formatEndpointLine("1.2.3.4", "Pod", "x", "", true)
	assert.True(t, strings.Contains(got, "pod/x"),
		"target kind is lowercased so the output matches kubectl's pod/foo style — output: %q", got)
}

// --- populateEndpoints (v1 Endpoints) ---

func TestPopulateEndpoints_EmitsEndpointsMultiLine(t *testing.T) {
	obj := map[string]any{
		"subsets": []any{
			map[string]any{
				"addresses": []any{
					map[string]any{
						"ip":       "192.168.1.5",
						"nodeName": "node-a",
						"targetRef": map[string]any{
							"kind": "Pod",
							"name": "foo-7d9",
						},
					},
					map[string]any{
						"ip":       "192.168.1.6",
						"nodeName": "node-b",
						"targetRef": map[string]any{
							"kind": "Pod",
							"name": "foo-7d9-9fr",
						},
					},
				},
				"notReadyAddresses": []any{
					map[string]any{
						"ip": "192.168.1.7",
						"targetRef": map[string]any{
							"kind": "Pod",
							"name": "foo-7d9-broken",
						},
					},
				},
				"ports": []any{
					map[string]any{"name": "http", "port": float64(80), "protocol": "TCP"},
				},
			},
		},
	}
	ti := &model.Item{}
	populateEndpoints(ti, obj)

	value := findColumnValue(ti.Columns, "Endpoints")
	require := assert.New(t)
	require.NotEmpty(value, "Endpoints multi-line block must be emitted when subsets contain addresses")
	lines := strings.Split(value, "\n")
	assert.Len(t, lines, 3, "one line per address (ready + not-ready), so the preview shows every endpoint")
	assert.Contains(t, lines[0], "192.168.1.5 → pod/foo-7d9 on node-a")
	assert.NotContains(t, lines[0], "NotReady", "ready endpoints have no NotReady marker")
	assert.Contains(t, lines[1], "192.168.1.6 → pod/foo-7d9-9fr on node-b")
	assert.Contains(t, lines[2], "192.168.1.7 → pod/foo-7d9-broken (NotReady)",
		"not-ready endpoints flagged inline so a degraded service is obvious at a glance")
}

func TestPopulateEndpoints_PreservesReadyCounts(t *testing.T) {
	// Existing rollup columns (Ready / Not Ready / Ports) must stay so the
	// preview keeps its summary stats above the new per-endpoint block.
	obj := map[string]any{
		"subsets": []any{
			map[string]any{
				"addresses":         []any{map[string]any{"ip": "1.1.1.1"}},
				"notReadyAddresses": []any{map[string]any{"ip": "2.2.2.2"}},
				"ports":             []any{map[string]any{"port": float64(80), "protocol": "TCP"}},
			},
		},
	}
	ti := &model.Item{}
	populateEndpoints(ti, obj)

	assert.Equal(t, "1", findColumnValue(ti.Columns, "Ready"))
	assert.Equal(t, "1", findColumnValue(ti.Columns, "Not Ready"))
	assert.NotEmpty(t, findColumnValue(ti.Columns, "Ports"))
}

func TestPopulateEndpoints_NoSubsetsFallsBackToNoneNotice(t *testing.T) {
	ti := &model.Item{}
	populateEndpoints(ti, map[string]any{})
	assert.Equal(t, "<none>", findColumnValue(ti.Columns, "Endpoints"),
		"missing subsets surface as <none> so the user sees that the resource is empty rather than just blank")
}

// --- populateEndpointSlice (discovery.k8s.io/v1) ---

func TestPopulateEndpointSlice_EmitsEndpointsMultiLine(t *testing.T) {
	obj := map[string]any{
		"addressType": "IPv4",
		"endpoints": []any{
			map[string]any{
				"addresses":  []any{"192.168.1.5"},
				"conditions": map[string]any{"ready": true},
				"nodeName":   "node-a",
				"targetRef":  map[string]any{"kind": "Pod", "name": "foo-7d9"},
			},
			map[string]any{
				"addresses":  []any{"192.168.1.6"},
				"conditions": map[string]any{"ready": false},
				"targetRef":  map[string]any{"kind": "Pod", "name": "foo-7d9-broken"},
			},
		},
		"ports": []any{
			map[string]any{"name": "http", "port": float64(80), "protocol": "TCP"},
		},
	}
	ti := &model.Item{}
	populateEndpointSlice(ti, obj)

	value := findColumnValue(ti.Columns, "Endpoints")
	assert.NotEmpty(t, value)
	lines := strings.Split(value, "\n")
	assert.Len(t, lines, 2)
	assert.Contains(t, lines[0], "192.168.1.5 → pod/foo-7d9 on node-a")
	assert.NotContains(t, lines[0], "NotReady")
	assert.Contains(t, lines[1], "192.168.1.6 → pod/foo-7d9-broken (NotReady)")
}

// Regression for the previous inverted default: per
// discovery.k8s.io/v1, an absent / null conditions.ready means
// "unknown" and must be interpreted as *ready*. The previous default
// of false was flagging every endpoint on a slice without an explicit
// ready field as (NotReady) — including the slices written by older
// API versions that don't populate the field.
func TestPopulateEndpointSlice_MissingConditionsTreatedAsReady(t *testing.T) {
	obj := map[string]any{
		"addressType": "IPv4",
		"endpoints": []any{
			map[string]any{
				"addresses": []any{"192.168.1.5"},
				// conditions absent: API spec says treat as ready.
				"targetRef": map[string]any{"kind": "Pod", "name": "foo"},
			},
			map[string]any{
				"addresses":  []any{"192.168.1.6"},
				"conditions": map[string]any{},
				// conditions.ready absent: API spec says treat as ready.
				"targetRef": map[string]any{"kind": "Pod", "name": "bar"},
			},
			map[string]any{
				"addresses":  []any{"192.168.1.7"},
				"conditions": map[string]any{"ready": nil},
				// conditions.ready explicitly null: API spec says treat as ready.
				"targetRef": map[string]any{"kind": "Pod", "name": "baz"},
			},
		},
	}
	ti := &model.Item{}
	populateEndpointSlice(ti, obj)

	value := findColumnValue(ti.Columns, "Endpoints")
	for i, line := range strings.Split(value, "\n") {
		assert.NotContains(t, line, "NotReady",
			"row %d (%q): missing/null conditions.ready must be treated as Ready, not (NotReady)",
			i, line)
	}
	assert.Equal(t, "3", findColumnValue(ti.Columns, "Ready"),
		"all three endpoints with absent/empty/null conditions count as Ready")
	assert.Empty(t, findColumnValue(ti.Columns, "Not Ready"),
		"no Not Ready column when every endpoint is treated as ready")
}

func TestPopulateEndpointSlice_ExplicitlyFalseConditionsAreNotReady(t *testing.T) {
	// Symmetric check: when conditions.ready *is* set and is false, the
	// endpoint must still flag as (NotReady) — the missing-treated-as-
	// ready fix above must not also swallow explicit false.
	obj := map[string]any{
		"addressType": "IPv4",
		"endpoints": []any{
			map[string]any{
				"addresses":  []any{"192.168.1.5"},
				"conditions": map[string]any{"ready": false},
				"targetRef":  map[string]any{"kind": "Pod", "name": "broken"},
			},
		},
	}
	ti := &model.Item{}
	populateEndpointSlice(ti, obj)
	assert.Contains(t, findColumnValue(ti.Columns, "Endpoints"), "(NotReady)")
	assert.Equal(t, "1", findColumnValue(ti.Columns, "Not Ready"))
}

func TestPopulateEndpointSlice_MultipleAddressesPerEndpointEmitOneLineEach(t *testing.T) {
	// EndpointSlice entries can carry multiple addresses (rare for IPv4, normal
	// for IPv4/IPv6 dual-stack). Each address must get its own preview line so
	// users can tell which address is up.
	obj := map[string]any{
		"addressType": "IPv4",
		"endpoints": []any{
			map[string]any{
				"addresses":  []any{"192.168.1.5", "192.168.1.6"},
				"conditions": map[string]any{"ready": true},
				"targetRef":  map[string]any{"kind": "Pod", "name": "foo"},
			},
		},
	}
	ti := &model.Item{}
	populateEndpointSlice(ti, obj)
	value := findColumnValue(ti.Columns, "Endpoints")
	lines := strings.Split(value, "\n")
	assert.Len(t, lines, 2, "each address in a multi-address endpoint gets its own preview line")
}
