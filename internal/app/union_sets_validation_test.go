package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// TestExpandUnionSetConfig_RejectsAmbiguousMemberNamespaces guards the new
// ambiguity check: if multiple member entries declare different
// non-empty namespaces, the function returns an error rather than
// silently picking the first one. Previous behaviour discarded all
// member namespaces after the first non-empty one, producing
// surprising "wrong namespace" results in multi-cluster sets.
func TestExpandUnionSetConfig_RejectsAmbiguousMemberNamespaces(t *testing.T) {
	set := ui.UnionSetConfig{
		Name: "dev-prod",
		Contexts: []ui.UnionSetContextConfig{
			{Context: "dev", Namespace: "dev-ns"},
			{Context: "prod", Namespace: "payments"},
		},
	}

	_, _, _, err := ExpandUnionSetConfig(set, nil)
	require.Error(t, err, "set with divergent member namespaces must be rejected")
	assert.Contains(t, err.Error(), "dev-prod")
	assert.Contains(t, err.Error(), "dev-ns")
	assert.Contains(t, err.Error(), "payments")
}

// TestExpandUnionSetConfig_AllowsRepeatedMemberNamespaces makes sure the
// ambiguity check does not over-fire — multiple members declaring the
// same namespace is fine.
func TestExpandUnionSetConfig_AllowsRepeatedMemberNamespaces(t *testing.T) {
	set := ui.UnionSetConfig{
		Name: "blue-green",
		Contexts: []ui.UnionSetContextConfig{
			{Context: "blue", Namespace: "shared"},
			{Context: "green", Namespace: "shared"},
		},
	}

	contexts, ns, _, err := ExpandUnionSetConfig(set, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"blue", "green"}, contexts)
	assert.Equal(t, "shared", ns)
}

// TestExpandUnionSetConfig_OneMemberOneNamespace covers the common case
// where exactly one member specifies a namespace.
func TestExpandUnionSetConfig_OneMemberOneNamespace(t *testing.T) {
	set := ui.UnionSetConfig{
		Name: "mixed",
		Contexts: []ui.UnionSetContextConfig{
			{Context: "dev"},
			{Context: "prod", Namespace: "payments"},
		},
	}

	_, ns, _, err := ExpandUnionSetConfig(set, nil)
	require.NoError(t, err)
	assert.Equal(t, "payments", ns, "single non-empty member namespace wins")
}
