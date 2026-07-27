package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every preset must pass the same validation a hand-typed taint faces —
// a preset that cannot be staged is worse than no preset.
func TestCommonTaints_AllValid(t *testing.T) {
	require.NotEmpty(t, CommonTaints)
	for _, p := range CommonTaints {
		t.Run(p.Taint.String(), func(t *testing.T) {
			assert.NoError(t, ValidateTaint(p.Taint))
			assert.NotEmpty(t, p.Desc, "each preset explains what it does")
		})
	}
}

// A node cannot carry two taints with the same key+effect, so offering two
// presets with one identity would guarantee a rejected staging.
func TestCommonTaints_NoDuplicateIdentities(t *testing.T) {
	seen := make(map[taintIdentity]bool, len(CommonTaints))
	for _, p := range CommonTaints {
		id := taintIdentity{p.Taint.Key, p.Taint.Effect}
		assert.False(t, seen[id], "duplicate preset identity: %s", p.Taint.String())
		seen[id] = true
	}
}

// The node controller owns the condition taints from live node conditions.
// Offering them for hand-application invites a fight with the controller.
func TestCommonTaints_ExcludeControllerManagedConditions(t *testing.T) {
	managed := []string{
		"node.kubernetes.io/not-ready",
		"node.kubernetes.io/unreachable",
		"node.kubernetes.io/memory-pressure",
		"node.kubernetes.io/disk-pressure",
		"node.kubernetes.io/pid-pressure",
		"node.kubernetes.io/network-unavailable",
	}
	for _, p := range CommonTaints {
		for _, m := range managed {
			assert.NotEqual(t, m, p.Taint.Key,
				"%s is node-controller managed and must not be a preset", m)
		}
	}
}

// dedicated= is the one preset intentionally shipped without a value: it is a
// convention whose value names the workload, so the picker must leave the user
// to fill it in rather than inventing one.
func TestCommonTaints_DedicatedHasNoValue(t *testing.T) {
	for _, p := range CommonTaints {
		if p.Taint.Key == "dedicated" {
			assert.Empty(t, p.Taint.Value)
			assert.True(t, strings.Contains(p.Desc, "value"),
				"description must tell the user to set the value")
			return
		}
	}
	t.Fatal("dedicated preset missing")
}
