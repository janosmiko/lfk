package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/k8s"
)

func ownersFrom(t *testing.T, pairs ...string) *k8s.FieldOwners {
	t.Helper()
	require.Zero(t, len(pairs)%2, "pairs must be manager, fieldsJSON")
	now := metav1.NewTime(time.Now())
	entries := make([]metav1.ManagedFieldsEntry, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		fields := &metav1.FieldsV1{}
		fields.SetRawBytes([]byte(pairs[i+1]))
		entries = append(entries, metav1.ManagedFieldsEntry{
			Manager:   pairs[i],
			Operation: metav1.ManagedFieldsOperationUpdate,
			Time:      &now,
			FieldsV1:  fields,
		})
	}
	return k8s.NewFieldOwners(entries)
}

// managersOf reduces the blame result to one manager name per line, so a test
// reads as a picture of the gutter.
func managersOf(blame []blameLine) []string {
	out := make([]string, len(blame))
	for i, b := range blame {
		out[i] = b.manager
	}
	return out
}

func TestComputeYAMLBlame_MapFields(t *testing.T) {
	content := "apiVersion: apps/v1\n" +
		"kind: Deployment\n" +
		"spec:\n" +
		"  replicas: 3\n" +
		"  paused: false\n"
	owners := ownersFrom(t, "kubectl", `{"f:spec":{"f:replicas":{}}}`)

	got := computeYAMLBlame(content, owners)

	// One entry per split line, so the trailing newline contributes a last blank.
	assert.Equal(t, []string{"", "", "", "kubectl", "", ""}, managersOf(got))
}

func TestComputeYAMLBlame_ListItemMatchedByName(t *testing.T) {
	content := "spec:\n" +
		"  containers:\n" +
		"    - name: nginx\n" +
		"      image: nginx:1.27\n" +
		"    - name: sidecar\n" +
		"      image: proxy:2\n"
	owners := ownersFrom(t, "kubectl",
		`{"f:spec":{"f:containers":{"k:{\"name\":\"nginx\"}":{".":{},"f:image":{}}}}}`)

	got := computeYAMLBlame(content, owners)

	assert.Equal(t, "kubectl", got[3].manager, "nginx image is owned")
	assert.Empty(t, got[5].manager, "the sidecar image is not covered by that selector")
}

func TestComputeYAMLBlame_HeaderTakesTheManagerOfAUniformSubtree(t *testing.T) {
	content := "metadata:\n" +
		"  labels:\n" +
		"    app: web\n" +
		"    tier: front\n"
	owners := ownersFrom(t, "argocd",
		`{"f:metadata":{"f:labels":{"f:app":{},"f:tier":{}}}}`)

	got := computeYAMLBlame(content, owners)

	assert.Equal(t, "argocd", got[1].manager, "labels header rolls up from its children")
	assert.True(t, got[1].rolled)
	assert.False(t, got[2].rolled, "an owned line is not a rollup")
}

func TestComputeYAMLBlame_MixedSubtreeLeavesTheHeaderBlank(t *testing.T) {
	content := "metadata:\n" +
		"  labels:\n" +
		"    app: web\n" +
		"    tier: front\n"
	owners := ownersFrom(t,
		"argocd", `{"f:metadata":{"f:labels":{"f:app":{}}}}`,
		"kubectl", `{"f:metadata":{"f:labels":{"f:tier":{}}}}`)

	got := computeYAMLBlame(content, owners)

	assert.Empty(t, got[1].manager, "two managers below, so no single owner")
}

func TestComputeYAMLBlame_BlockScalarBodyFollowsItsKey(t *testing.T) {
	content := "data:\n" +
		"  script: |\n" +
		"    line one: not a key\n" +
		"    line two\n" +
		"  other: x\n"
	owners := ownersFrom(t, "helm", `{"f:data":{"f:script":{}}}`)

	got := computeYAMLBlame(content, owners)

	assert.Equal(t, "helm", got[1].manager)
	assert.Equal(t, "helm", got[2].manager, "block body belongs to the key that opened it")
	assert.Equal(t, "helm", got[3].manager)
	assert.Empty(t, got[4].manager)
}

func TestComputeYAMLBlame_KeysContainingDots(t *testing.T) {
	content := "metadata:\n" +
		"  annotations:\n" +
		"    kubectl.kubernetes.io/last-applied-configuration: '{}'\n"
	owners := ownersFrom(t, "kubectl",
		`{"f:metadata":{"f:annotations":{"f:kubectl.kubernetes.io/last-applied-configuration":{}}}}`)

	got := computeYAMLBlame(content, owners)

	assert.Equal(t, "kubectl", got[2].manager)
}

func TestComputeYAMLBlame_NoOwnersReturnsNothing(t *testing.T) {
	assert.Nil(t, computeYAMLBlame("spec:\n  replicas: 1\n", nil))
	assert.Nil(t, computeYAMLBlame("spec:\n  replicas: 1\n", k8s.NewFieldOwners(nil)))
}

func TestComputeYAMLBlame_LineCountMatchesTheContent(t *testing.T) {
	content := "spec:\n\n  # a comment\n  replicas: 3\n"
	owners := ownersFrom(t, "kubectl", `{"f:spec":{"f:replicas":{}}}`)

	got := computeYAMLBlame(content, owners)

	require.Len(t, got, 5, "one entry per line, so the gutter stays aligned")
	assert.Equal(t, "kubectl", got[3].manager)
}

func TestComputeYAMLBlame_StripsControlBytesFromTheManagerName(t *testing.T) {
	// A manager name is cluster-controlled. A non-core apiserver can set it
	// to anything, including an escape sequence aimed at the terminal.
	// \u009b is the C1 CSI, encoded as two UTF-8 bytes but one rune above
	// 0x7f, which an ASCII-only filter would let through.
	hostile := "kubectl\x1b[2J\u009bAevil\x7f"
	owners := ownersFrom(t, hostile, `{"f:spec":{"f:replicas":{}}}`)

	got := computeYAMLBlame("spec:\n  replicas: 3\n", owners)

	assert.Equal(t, "kubectl[2JAevil", got[1].manager)
	assert.NotContains(t, got[1].manager, "\x1b")
	assert.NotContains(t, got[1].manager, "\u009b")
}

func TestComputeYAMLBlame_QuotedKeyContainingAColon(t *testing.T) {
	content := "metadata:\n" +
		"  annotations:\n" +
		"    \"weird:key\": value\n"
	owners := ownersFrom(t, "kubectl",
		`{"f:metadata":{"f:annotations":{"f:weird:key":{}}}}`)

	got := computeYAMLBlame(content, owners)

	assert.Equal(t, "kubectl", got[2].manager, "the key ends at the colon outside the quotes")
}

func TestKeyColonIndex(t *testing.T) {
	assert.Equal(t, 3, keyColonIndex("abc: v"))
	assert.Equal(t, 11, keyColonIndex(`"weird:key": v`))
	assert.Equal(t, 11, keyColonIndex("'weird:key': v"))
	assert.Equal(t, -1, keyColonIndex("no colon here"))
}

func TestComputeYAMLBlame_HeaderStaysBlankWhenChildrenDifferInTime(t *testing.T) {
	// One manager wrote both labels, but in two separate applies. The header
	// has no single write time, so it must not borrow one from a child.
	content := "metadata:\n" +
		"  labels:\n" +
		"    app: web\n" +
		"    tier: front\n"
	older := metav1.NewTime(time.Now().Add(-72 * time.Hour))
	newer := metav1.NewTime(time.Now())
	entry := func(at metav1.Time, raw string) metav1.ManagedFieldsEntry {
		fields := &metav1.FieldsV1{}
		fields.SetRawBytes([]byte(raw))
		return metav1.ManagedFieldsEntry{
			Manager:   "argocd",
			Operation: metav1.ManagedFieldsOperationApply,
			Time:      &at,
			FieldsV1:  fields,
		}
	}
	owners := k8s.NewFieldOwners([]metav1.ManagedFieldsEntry{
		entry(older, `{"f:metadata":{"f:labels":{"f:app":{}}}}`),
		entry(newer, `{"f:metadata":{"f:labels":{"f:tier":{}}}}`),
	})

	got := computeYAMLBlame(content, owners)

	assert.Equal(t, "argocd", got[2].manager, "the child keeps its own owner")
	assert.Empty(t, got[1].manager,
		"the header covers two different writes, so it names neither")
}
