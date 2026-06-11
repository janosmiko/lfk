package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetworkPoliciesActionPresent(t *testing.T) {
	for _, kind := range []string{"Pod", "Service"} {
		t.Run(kind, func(t *testing.T) {
			items := ActionsForKind(kind)
			found := false
			for _, it := range items {
				if it.Label == "Network Policies" {
					found = true
					assert.Equal(t, "N", it.Key)
					assert.NotEmpty(t, it.Description)
					break
				}
			}
			assert.True(t, found, "%q must offer the Network Policies action item", kind)
		})
	}
}

func TestNetworkPoliciesActionAbsentForUnsupported(t *testing.T) {
	for _, kind := range []string{"ConfigMap", "Secret", "Node", "NetworkPolicy"} {
		t.Run(kind, func(t *testing.T) {
			items := ActionsForKind(kind)
			for _, it := range items {
				assert.NotEqual(t, "Network Policies", it.Label,
					"%q must not offer the Network Policies action item", kind)
			}
		})
	}
}

func TestNetworkPoliciesActionKeysUnique(t *testing.T) {
	for _, kind := range []string{"Pod", "Service"} {
		t.Run(kind, func(t *testing.T) {
			keys := make(map[string]string)
			for _, a := range ActionsForKind(kind) {
				prev, dup := keys[a.Key]
				assert.False(t, dup, "duplicate key %q used by %q and %q", a.Key, prev, a.Label)
				keys[a.Key] = a.Label
			}
		})
	}
}
