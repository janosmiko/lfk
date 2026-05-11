package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ctxNames returns just the Context fields from a list of cluster entries —
// shorthand for assertions that compare a set's contexts as []string.
func ctxNames(in []UnionSetContextConfig) []string {
	out := make([]string, 0, len(in))
	for _, c := range in {
		out = append(out, c.Context)
	}
	return out
}

// TestLoadConfig_UnionSets exercises the YAML round-trip and the
// sanitizeUnionSets validation pass: a config with two valid sets — one
// fully colored, one without colors — plus malformed entries that must
// be dropped, and a duplicate-name collision that must collapse to the
// last definition.
func TestLoadConfig_UnionSets(t *testing.T) {
	orig := ConfigUnionSets
	t.Cleanup(func() { ConfigUnionSets = orig })

	yaml := `
union_sets:
  - name: ski-staging-west
    contexts:
      - context: operator/block-sre-operator/square-staging-green-us-west-2
        color: green
      - context: operator/block-sre-operator/square-staging-yellow-us-west-2
        color: yellow
      - context: operator/block-sre-operator/square-staging-blue-us-west-2
        color: blue
    namespace: kube-policies
  - name: ski-prod-east
    contexts:
      - prod-green-east
      - name: prod-blue-east
  - contexts:
      - context: orphan-ctx
  - name: empty-set
  - name: ski-staging-west
    contexts:
      - context: new-green
        color: cyan
      - context: new-blue
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	ConfigUnionSets = nil
	LoadConfig(path)

	require.Len(t, ConfigUnionSets, 2, "nameless and contextless entries must be dropped; duplicate must collapse")

	// Last definition of duplicate name wins (the {new-green, new-blue} version).
	byName := make(map[string]UnionSetConfig)
	for _, s := range ConfigUnionSets {
		byName[s.Name] = s
	}
	dup, ok := byName["ski-staging-west"]
	require.True(t, ok)
	assert.Equal(t, []string{"new-green", "new-blue"}, ctxNames(dup.Contexts),
		"duplicate name must keep the last-defined contexts (last-wins override)")
	require.Len(t, dup.Contexts, 2)
	assert.Equal(t, "cyan", dup.Contexts[0].Color, "color must round-trip through YAML")
	assert.Empty(t, dup.Contexts[1].Color, "missing color must yield empty string, not a default")
	assert.Empty(t, dup.Namespace,
		"duplicate replacement must NOT inherit the dropped entry's namespace")

	prod, ok := byName["ski-prod-east"]
	require.True(t, ok)
	assert.Equal(t, []string{"prod-green-east", "prod-blue-east"}, ctxNames(prod.Contexts))
	for _, c := range prod.Contexts {
		assert.Empty(t, c.Color, "color is optional; missing field must round-trip as empty")
	}
	assert.Empty(t, prod.Namespace, "namespace is optional; absence must not crash sanitization")
}

func TestLoadConfig_UnionSetsMapForm(t *testing.T) {
	orig := ConfigUnionSets
	t.Cleanup(func() { ConfigUnionSets = orig })

	yaml := `
union_sets:
  ski-staging-west:
    namespace: kube-policies
    contexts:
      - staging-green
      - context: staging-blue
        color: blue
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	ConfigUnionSets = nil
	LoadConfig(path)

	require.Len(t, ConfigUnionSets, 1)
	set := ConfigUnionSets[0]
	assert.Equal(t, "ski-staging-west", set.Name)
	assert.Equal(t, "kube-policies", set.Namespace)
	assert.Equal(t, []string{"staging-green", "staging-blue"}, ctxNames(set.Contexts))
	require.Len(t, set.Contexts, 2)
	assert.Empty(t, set.Contexts[0].Color)
	assert.Equal(t, "blue", set.Contexts[1].Color)
}

func TestSanitizeUnionSets_DropsInvalidEntries(t *testing.T) {
	in := []UnionSetConfig{
		{
			Name: "good",
			Contexts: []UnionSetContextConfig{
				{Context: "a", Color: "green"},
			},
		},
		{
			// no name; whole entry dropped
			Contexts: []UnionSetContextConfig{{Context: "b"}},
		},
		{
			Name:     "no-contexts",
			Contexts: nil,
		},
		{
			Name:     "no-contexts-empty-slice",
			Contexts: []UnionSetContextConfig{},
		},
		{
			// every cluster entry is invalid (empty context); whole entry
			// dropped because none survive sanitization.
			Name:     "all-invalid-contexts",
			Contexts: []UnionSetContextConfig{{Context: ""}, {Context: ""}},
		},
	}
	got := sanitizeUnionSets(in)
	require.Len(t, got, 1, "only the well-formed entry survives; got: %+v", got)
	assert.Equal(t, "good", got[0].Name)
}

func TestSanitizeUnionSets_DropsInvalidColorButKeepsEntry(t *testing.T) {
	// An unknown color name must NOT discard the cluster — the user
	// probably wants the cluster in the merged view, just untinted. The
	// renderer reserves a blank cell for empty Color so layout stays
	// aligned with rows that do carry a color.
	in := []UnionSetConfig{{
		Name: "mixed",
		Contexts: []UnionSetContextConfig{
			{Context: "a", Color: "neon-pink"}, // invalid -> dropped to ""
			{Context: "b", Color: "blue"},      // valid -> preserved
		},
	}}
	got := sanitizeUnionSets(in)
	require.Len(t, got, 1)
	require.Len(t, got[0].Contexts, 2)
	assert.Equal(t, "a", got[0].Contexts[0].Context)
	assert.Empty(t, got[0].Contexts[0].Color, "unknown color must be cleared, not preserved")
	assert.Equal(t, "b", got[0].Contexts[1].Context)
	assert.Equal(t, "blue", got[0].Contexts[1].Color)
}

func TestSanitizeUnionSets_DedupesContextsWithinSet(t *testing.T) {
	// A typo or copy-paste mistake repeats a context name. ValidateUnionOptions
	// would later reject the duplicate at runTUI time anyway, but catching it
	// at sanitize time keeps the error surface narrower (only one warning,
	// no later "duplicate context" error from the resolver).
	in := []UnionSetConfig{{
		Name: "dup",
		Contexts: []UnionSetContextConfig{
			{Context: "a", Color: "green"},
			{Context: "a", Color: "yellow"}, // duplicate dropped
			{Context: "b"},
		},
	}}
	got := sanitizeUnionSets(in)
	require.Len(t, got, 1)
	require.Len(t, got[0].Contexts, 2, "duplicate context must be removed within the set")
	assert.Equal(t, "a", got[0].Contexts[0].Context)
	assert.Equal(t, "green", got[0].Contexts[0].Color, "first occurrence's color wins")
	assert.Equal(t, "b", got[0].Contexts[1].Context)
}
