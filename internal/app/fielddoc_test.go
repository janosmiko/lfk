package app

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldDocPath(t *testing.T) {
	tests := []struct {
		name    string
		objPath []string
		want    string
	}{
		{name: "empty", objPath: nil, want: ""},
		{name: "root field", objPath: []string{"spec"}, want: "spec"},
		{name: "nested leaf", objPath: []string{"spec", "dnsPolicy"}, want: "spec.dnsPolicy"},
		{
			name:    "array index dropped",
			objPath: []string{"spec", "containers", "[0]", "image"},
			want:    "spec.containers.image",
		},
		{
			name:    "trailing array index dropped",
			objPath: []string{"spec", "containers", "[2]"},
			want:    "spec.containers",
		},
		{
			name:    "crd nested path",
			objPath: []string{"spec", "instances", "[1]", "storage", "size"},
			want:    "spec.instances.storage.size",
		},
		{name: "only an index", objPath: []string{"[0]"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, fieldDocPath(tt.objPath))
		})
	}
}

func TestParseExplainFieldHeader(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantName string
		wantType string
	}{
		{
			name: "leaf field with type",
			output: "KIND:       Pod\nVERSION:    v1\n\n" +
				"FIELD: dnsPolicy <string>\n\nDESCRIPTION:\n    Set DNS policy for the pod.\n",
			wantName: "dnsPolicy",
			wantType: "<string>",
		},
		{
			name:     "required marker kept out of the type",
			output:   "FIELD: name <string> -required-\n\nDESCRIPTION:\n    The container name.\n",
			wantName: "name",
			wantType: "<string>",
		},
		{
			name:     "object type",
			output:   "FIELD: resources <ResourceRequirements>\n\nDESCRIPTION:\n    Compute resources.\n",
			wantName: "resources",
			wantType: "<ResourceRequirements>",
		},
		{
			name:     "no FIELD header on a root explain",
			output:   "KIND:       Pod\nVERSION:    v1\n\nDESCRIPTION:\n    Pod is a collection of containers.\n\nFIELDS:\n  spec\t<PodSpec>\n",
			wantName: "",
			wantType: "",
		},
		{
			name:     "field header without a type",
			output:   "FIELD: metadata\n\nDESCRIPTION:\n    Standard object metadata.\n",
			wantName: "metadata",
			wantType: "",
		},
		{name: "empty output", output: "", wantName: "", wantType: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotType := parseExplainFieldHeader(tt.output)
			assert.Equal(t, tt.wantName, gotName, "name")
			assert.Equal(t, tt.wantType, gotType, "type")
		})
	}
}

// A field the schema documents, a CRD field, and a field with no description
// all go through the same parse path, so they are covered together here.
func TestParseExplainOutputLeafDescription(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "built-in field",
			output: "KIND:       Pod\nVERSION:    v1\n\nFIELD: dnsPolicy <string>\n\n" +
				"DESCRIPTION:\n    Set DNS policy for the pod. Defaults to \"ClusterFirst\".\n",
			want: `Set DNS policy for the pod. Defaults to "ClusterFirst".`,
		},
		{
			name: "crd field",
			output: "GROUP:      postgresql.cnpg.io\nKIND:       Cluster\nVERSION:    v1\n\n" +
				"FIELD: instances <integer>\n\nDESCRIPTION:\n    Number of instances required in the cluster\n",
			want: "Number of instances required in the cluster",
		},
		{
			name:   "field the schema leaves undocumented",
			output: "KIND:       Widget\nVERSION:    v1\n\nFIELD: opaque <string>\n\nDESCRIPTION:\n\n",
			want:   "",
		},
		{
			name:   "no description section at all",
			output: "KIND:       Widget\nVERSION:    v1\n\nFIELD: opaque <string>\n",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, _ := parseExplainOutput(tt.output, "")
			assert.Equal(t, tt.want, desc)
		})
	}
}

func TestFieldDocCacheGetPut(t *testing.T) {
	c := newFieldDocCache()

	k := fieldDocKey{context: "kind-a", apiVersion: "v1", resource: "pods", path: "spec.dnsPolicy"}
	_, ok := c.get(k)
	assert.False(t, ok, "empty cache must miss")

	c.put(k, fieldDocEntry{fieldType: "<string>", desc: "Set DNS policy."})
	got, ok := c.get(k)
	require.True(t, ok, "put then get must hit")
	assert.Equal(t, "Set DNS policy.", got.desc)
	assert.Equal(t, "<string>", got.fieldType)
}

// An empty description is a real answer from the schema, not a miss. Caching it
// stops a documented-as-blank field from re-spawning kubectl on every visit.
func TestFieldDocCacheStoresEmptyDescription(t *testing.T) {
	c := newFieldDocCache()
	k := fieldDocKey{context: "kind-a", apiVersion: "v1", resource: "widgets", path: "spec.opaque"}

	c.put(k, fieldDocEntry{fieldType: "<string>", desc: ""})

	got, ok := c.get(k)
	require.True(t, ok, "an empty description must still count as cached")
	assert.Empty(t, got.desc)
}

func TestFieldDocCacheKeySeparatesClustersAndVersions(t *testing.T) {
	c := newFieldDocCache()
	base := fieldDocKey{context: "kind-a", apiVersion: "v1", resource: "pods", path: "spec.dnsPolicy"}

	otherCtx := base
	otherCtx.context = "kind-b"
	otherVer := base
	otherVer.apiVersion = "v2"
	otherPath := base
	otherPath.path = "spec.restartPolicy"

	c.put(base, fieldDocEntry{desc: "base"})
	c.put(otherCtx, fieldDocEntry{desc: "other context"})
	c.put(otherVer, fieldDocEntry{desc: "other version"})
	c.put(otherPath, fieldDocEntry{desc: "other path"})

	for _, tt := range []struct {
		key  fieldDocKey
		want string
	}{
		{base, "base"},
		{otherCtx, "other context"},
		{otherVer, "other version"},
		{otherPath, "other path"},
	} {
		got, ok := c.get(tt.key)
		require.True(t, ok)
		assert.Equal(t, tt.want, got.desc)
	}
}

func TestFieldDocCacheEvictsAtTheCap(t *testing.T) {
	c := newFieldDocCache()

	for i := range fieldDocMaxEntries + 10 {
		c.put(fieldDocKey{context: "kind-a", resource: "pods", path: pathForIndex(i)},
			fieldDocEntry{desc: "d"})
	}

	assert.LessOrEqual(t, c.len(), fieldDocMaxEntries, "cache must stay bounded")

	// The oldest keys go first, the newest survive.
	_, ok := c.get(fieldDocKey{context: "kind-a", resource: "pods", path: pathForIndex(0)})
	assert.False(t, ok, "oldest entry must be evicted")

	newest := fieldDocMaxEntries + 9
	_, ok = c.get(fieldDocKey{context: "kind-a", resource: "pods", path: pathForIndex(newest)})
	assert.True(t, ok, "newest entry must survive")
}

// Re-putting a key must not add a second eviction-order record, or the cache
// evicts live entries early and holds fewer than the cap.
func TestFieldDocCacheRepeatedPutKeepsOneOrderRecord(t *testing.T) {
	c := newFieldDocCache()
	k := fieldDocKey{context: "kind-a", resource: "pods", path: "spec"}

	for range 5 {
		c.put(k, fieldDocEntry{desc: "d"})
	}

	assert.Equal(t, 1, c.len())
}

func pathForIndex(i int) string {
	return "spec.field" + strconv.Itoa(i)
}
