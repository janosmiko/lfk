package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nodeManifestDoc is a trimmed Node-shaped document used across the
// flattener tests: nested maps, an array of address objects, and a
// managedFields block that must be excluded.
func nodeManifestDoc() map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Node",
		"metadata": map[string]any{
			"name": "worker-1",
			"labels": map[string]any{
				"kubernetes.io/hostname": "worker-1",
			},
			"managedFields": []any{
				map[string]any{"manager": "kubelet"},
			},
		},
		"status": map[string]any{
			"addresses": []any{
				map[string]any{"type": "InternalIP", "address": "10.0.0.5"},
				map[string]any{"type": "ExternalIP", "address": "34.1.2.3"},
			},
			"nodeInfo": map[string]any{
				"kubeletVersion": "v1.31.0",
			},
		},
	}
}

func findCopyFieldEntry(entries []copyFieldEntry, display string) *copyFieldEntry {
	for i := range entries {
		if entries[i].display == display {
			return &entries[i]
		}
	}
	return nil
}

func TestFlattenCopyFields_SemanticArrayLabels(t *testing.T) {
	entries := flattenCopyFields(nodeManifestDoc())

	// Array elements with a unique discriminator are labeled by it, so
	// filtering "ExternalIP" surfaces the address row by path.
	ext := findCopyFieldEntry(entries, "status.addresses[ExternalIP].address")
	require.NotNil(t, ext, "external IP leaf must be listed under a semantic label")
	assert.Equal(t, "34.1.2.3", ext.value)

	name := findCopyFieldEntry(entries, "metadata.name")
	require.NotNil(t, name)
	assert.Equal(t, "worker-1", name.value)

	// Dotted map keys must stay a single segment (k8s label keys contain dots).
	label := findCopyFieldEntry(entries, "metadata.labels.kubernetes.io/hostname")
	require.NotNil(t, label)
}

func TestFlattenCopyFields_DuplicateLabelsFallBackToIndex(t *testing.T) {
	doc := map[string]any{
		"status": map[string]any{
			"addresses": []any{
				map[string]any{"type": "InternalIP", "address": "10.0.0.5"},
				map[string]any{"type": "InternalIP", "address": "10.0.0.6"},
			},
		},
	}
	entries := flattenCopyFields(doc)
	assert.NotNil(t, findCopyFieldEntry(entries, "status.addresses[0].address"),
		"ambiguous labels keep index paths")
	assert.NotNil(t, findCopyFieldEntry(entries, "status.addresses[1].address"))
	assert.Nil(t, findCopyFieldEntry(entries, "status.addresses[InternalIP].address"))
}

func TestFlattenCopyFields_LeavesOnlyAndManagedFieldsSkipped(t *testing.T) {
	entries := flattenCopyFields(nodeManifestDoc())
	for _, e := range entries {
		assert.NotEqual(t, "status", e.display, "non-leaf nodes are not listed")
		assert.NotContains(t, e.display, "managedFields", "metadata.managedFields is noise")
	}
}

func TestFlattenCopyFields_NonMapRootReturnsNil(t *testing.T) {
	assert.Nil(t, flattenCopyFields(nil))
	assert.Nil(t, flattenCopyFields("scalar"))
}

func TestCopyFieldValueString_Scalars(t *testing.T) {
	assert.Equal(t, "34.1.2.3", copyFieldValueString("34.1.2.3"))
	assert.Equal(t, "3", copyFieldValueString(float64(3)))
	assert.Equal(t, "true", copyFieldValueString(true))
	assert.Equal(t, "null", copyFieldValueString(nil))
}

func TestFlattenCopyFields_DisplayValueIsSingleLineAndControlFree(t *testing.T) {
	doc := map[string]any{
		"data": map[string]any{
			"tls.crt": "-----BEGIN CERT-----\nline2\nline3\n-----END CERT-----",
			"evil":    "a\x1b[31mred\x1b[0m\tb",
		},
	}
	entries := flattenCopyFields(doc)

	crt := findCopyFieldEntry(entries, "data.tls.crt")
	require.NotNil(t, crt)
	assert.NotContains(t, crt.value, "\n", "display value must be one line")

	evil := findCopyFieldEntry(entries, "data.evil")
	require.NotNil(t, evil)
	assert.NotContains(t, evil.value, "\x1b", "escape bytes must not reach the renderer")
	assert.NotContains(t, evil.value, "\t")
}

func TestFlattenCopyFields_HostileLabelSanitizedInDisplay(t *testing.T) {
	doc := map[string]any{
		"spec": map[string]any{
			"items": []any{
				map[string]any{"name": "ok\x1b[31mred", "v": "1"},
			},
		},
	}
	entries := flattenCopyFields(doc)
	for _, e := range entries {
		assert.NotContains(t, e.display, "\x1b", "labels are sanitized before display")
	}
}

func TestFlattenCopyFields_HostileMapKeySanitizedInDisplay(t *testing.T) {
	doc := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				"evil\x1b[31mkey\ttab": "v",
			},
		},
	}
	entries := flattenCopyFields(doc)
	require.NotEmpty(t, entries)
	for _, e := range entries {
		assert.NotContains(t, e.display, "\x1b", "map keys are sanitized before display")
		assert.NotContains(t, e.display, "\t")
	}
}

func TestBuildCopyFieldPayload_PreservesRawValue(t *testing.T) {
	doc := map[string]any{"data": map[string]any{"crt": "line1\nline2"}}
	entries := flattenCopyFields(doc)
	e := findCopyFieldEntry(entries, "data.crt")
	require.NotNil(t, e)
	payload, found, missing := buildCopyFieldPayload([]any{doc}, e.path)
	assert.Equal(t, "line1\nline2", payload, "clipboard keeps the raw value")
	assert.Equal(t, 1, found)
	assert.Equal(t, 0, missing)
}

func TestBuildCopyFieldPayload_LabelResolvesAcrossReorderedDocs(t *testing.T) {
	docA := nodeManifestDoc() // ExternalIP at index 1
	docB := map[string]any{   // ExternalIP at index 0 — order flipped
		"status": map[string]any{
			"addresses": []any{
				map[string]any{"type": "ExternalIP", "address": "49.12.73.237"},
				map[string]any{"type": "InternalIP", "address": "10.0.0.6"},
			},
		},
	}
	entries := flattenCopyFields(docA)
	e := findCopyFieldEntry(entries, "status.addresses[ExternalIP].address")
	require.NotNil(t, e)

	payload, found, missing := buildCopyFieldPayload([]any{docA, docB}, e.path)
	assert.Equal(t, "34.1.2.3\n49.12.73.237", payload,
		"the labeled element is resolved per doc, not by index")
	assert.Equal(t, 2, found)
	assert.Equal(t, 0, missing)
}

func TestBuildCopyFieldPayload_MultiDocSkipsMissing(t *testing.T) {
	docA := nodeManifestDoc()
	docC := map[string]any{"kind": "Node"} // path missing entirely

	entries := flattenCopyFields(docA)
	e := findCopyFieldEntry(entries, "status.addresses[ExternalIP].address")
	require.NotNil(t, e)

	payload, found, missing := buildCopyFieldPayload([]any{docA, docC}, e.path)
	assert.Equal(t, "34.1.2.3", payload)
	assert.Equal(t, 1, found)
	assert.Equal(t, 1, missing)
}

func TestParseManifestDocs_MultiDoc(t *testing.T) {
	content := "kind: Node\nmetadata:\n  name: a\n---\nkind: Node\nmetadata:\n  name: b\n"
	docs := parseManifestDocs(content, 0)
	require.Len(t, docs, 2)
	first, ok := docs[0].(map[string]any)
	require.True(t, ok)
	md, ok := first["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "a", md["name"])
}

func TestParseManifestDocs_SkipsUnparsableDoc(t *testing.T) {
	content := "kind: Node\n---\n\t: not yaml {{{\n---\nkind: Pod\n"
	docs := parseManifestDocs(content, 0)
	require.Len(t, docs, 2, "bad doc dropped, good docs kept")
}

func TestParseManifestDocs_CapsAtMaxDocs(t *testing.T) {
	content := "kind: A\n---\nkind: B\n---\nkind: C\n"
	docs := parseManifestDocs(content, 2)
	require.Len(t, docs, 2, "parsing stops at the requested doc count")
}
