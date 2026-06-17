package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsLonghornNode disambiguates the longhorn.io Node CRD from the core
// Kubernetes Node, which share the Kind "Node". Force delete and replica
// eviction must only apply to the longhorn.io variant.
func TestIsLonghornNode(t *testing.T) {
	tests := []struct {
		name string
		rt   ResourceTypeEntry
		want bool
	}{
		{
			name: "longhorn node",
			rt:   ResourceTypeEntry{APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "nodes", Kind: "Node"},
			want: true,
		},
		{
			name: "core node",
			rt:   ResourceTypeEntry{APIGroup: "", APIVersion: "v1", Resource: "nodes", Kind: "Node"},
			want: false,
		},
		{
			name: "longhorn volume",
			rt:   ResourceTypeEntry{APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "volumes", Kind: "Volume"},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsLonghornNode(tc.rt))
		})
	}
}

// TestActionsForLonghornNode locks in the dedicated action menu for
// longhorn.io nodes. It must offer the Longhorn-native verbs (Evict
// Replicas / Cancel Eviction) and Force Delete (disable scheduling then
// delete past the validating webhook), but NOT the core-node kubectl verbs
// (Cordon / Drain / Taint / Shell) that do not apply to a longhorn.io node.
func TestActionsForLonghornNode(t *testing.T) {
	items := ActionsForLonghornNode()
	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}

	assert.Contains(t, labels, "Evict Replicas",
		"Longhorn node must offer Evict Replicas (spec.evictionRequested=true)")
	assert.Contains(t, labels, "Cancel Eviction",
		"Longhorn node must offer Cancel Eviction (spec.evictionRequested=false)")
	assert.Contains(t, labels, "Force Delete",
		"Longhorn node must offer Force Delete (disable scheduling then delete)")
	assert.Contains(t, labels, "Delete")
	assert.Contains(t, labels, "Describe")
	assert.Contains(t, labels, "Edit")
	assert.Contains(t, labels, "Events")

	// Core-node-only kubectl verbs do not apply to a longhorn.io node.
	assert.NotContains(t, labels, "Cordon")
	assert.NotContains(t, labels, "Drain")
	assert.NotContains(t, labels, "Taint")
	assert.NotContains(t, labels, "Shell")

	// Force Delete stays on the canonical X hotkey (see TestForceDeleteHotkeyConsistency).
	fd, ok := findAction(items, "Force Delete")
	require.True(t, ok)
	assert.Equal(t, "X", fd.Key)

	// Hotkeys within the menu must be unique.
	seen := map[string]string{}
	for _, it := range items {
		if it.Key == "" {
			continue
		}
		if prev, dup := seen[it.Key]; dup {
			t.Fatalf("hotkey %q reused by %q and %q", it.Key, prev, it.Label)
		}
		seen[it.Key] = it.Label
	}
}
