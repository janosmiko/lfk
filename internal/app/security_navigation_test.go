package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func TestSecurityResourceTypeForItem_Heuristic(t *testing.T) {
	item := &model.Item{
		Name: "Heuristic",
		Kind: "__security_heuristic__",
	}
	rt, ok := securityResourceTypeForItem(item)
	require.True(t, ok, "well-formed security Kind should resolve")
	assert.Equal(t, model.SecurityVirtualAPIGroup, rt.APIGroup,
		"APIGroup must be the _security sentinel so GetResources dispatches correctly")
	assert.Equal(t, "v1", rt.APIVersion)
	assert.Equal(t, "findings-heuristic", rt.Resource)
	assert.Equal(t, "__security_heuristic__", rt.Kind,
		"Kind preserved so client.getSecurityFindings can extract the source name")
	assert.Equal(t, "Heuristic", rt.DisplayName)
	assert.True(t, rt.Namespaced)
}

func TestSecurityResourceTypeForItem_NonSecurityKind(t *testing.T) {
	item := &model.Item{Name: "nginx", Kind: "Pod"}
	_, ok := securityResourceTypeForItem(item)
	assert.False(t, ok, "regular kinds must not resolve as security entries")
}

func TestSecurityResourceTypeForItem_Nil(t *testing.T) {
	_, ok := securityResourceTypeForItem(nil)
	assert.False(t, ok, "nil input must not panic and must not resolve")
}

func TestSecurityResourceTypeForItem_MalformedKind(t *testing.T) {
	cases := []string{
		"__security___", // empty source name between markers
		"__security_",   // missing trailing markers
		"security_x__",  // missing leading markers
	}
	for _, k := range cases {
		_, ok := securityResourceTypeForItem(&model.Item{Kind: k})
		assert.False(t, ok, "malformed Kind %q must not resolve", k)
	}
}

// TestSecurityResourceTypeForItem_LoaderSentinel guards the navigation
// no-op for the loader entry shown while the availability probe is in
// flight. Without the guard, clicking the loader would dispatch a
// findings fetch for a non-existent "loader" source — unnecessary
// work and a confusing empty list.
func TestSecurityResourceTypeForItem_LoaderSentinel(t *testing.T) {
	item := &model.Item{
		Name: "(probing sources...)",
		Kind: model.SecurityLoaderKind,
	}
	_, ok := securityResourceTypeForItem(item)
	assert.False(t, ok, "loader sentinel must not be navigable")
}
