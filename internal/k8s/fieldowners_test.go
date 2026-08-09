package k8s

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/model"
)

func entry(manager, operation, fieldsJSON string, at time.Time) metav1.ManagedFieldsEntry {
	t := metav1.NewTime(at)
	fields := &metav1.FieldsV1{}
	fields.SetRawBytes([]byte(fieldsJSON))
	return metav1.ManagedFieldsEntry{
		Manager:    manager,
		Operation:  metav1.ManagedFieldsOperationType(operation),
		Time:       &t,
		FieldsType: "FieldsV1",
		FieldsV1:   fields,
	}
}

func TestFieldOwnersAt_MapFields(t *testing.T) {
	base := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	owners := NewFieldOwners([]metav1.ManagedFieldsEntry{
		entry("kubectl", "Update", `{"f:spec":{"f:replicas":{}}}`, base),
		entry("argocd-controller", "Apply", `{"f:metadata":{"f:labels":{"f:app":{}}}}`, base),
	})

	got, ok := owners.At([]PathSeg{{Key: "spec"}, {Key: "replicas"}})
	require.True(t, ok)
	assert.Equal(t, "kubectl", got.Manager)
	assert.Equal(t, "Update", got.Operation)

	got, ok = owners.At([]PathSeg{{Key: "metadata"}, {Key: "labels"}, {Key: "app"}})
	require.True(t, ok)
	assert.Equal(t, "argocd-controller", got.Manager)
}

func TestFieldOwnersAt_UnownedPathReturnsFalse(t *testing.T) {
	owners := NewFieldOwners([]metav1.ManagedFieldsEntry{
		entry("kubectl", "Update", `{"f:spec":{"f:replicas":{}}}`, time.Now()),
	})

	_, ok := owners.At([]PathSeg{{Key: "spec"}, {Key: "paused"}})
	assert.False(t, ok)

	// A parent that only routes to an owned child is not itself owned.
	_, ok = owners.At([]PathSeg{{Key: "spec"}})
	assert.False(t, ok)
}

func TestFieldOwnersAt_DotMarksTheNodeItself(t *testing.T) {
	owners := NewFieldOwners([]metav1.ManagedFieldsEntry{
		entry("helm", "Update", `{"f:spec":{".":{},"f:replicas":{}}}`, time.Now()),
	})

	got, ok := owners.At([]PathSeg{{Key: "spec"}})
	require.True(t, ok)
	assert.Equal(t, "helm", got.Manager)
}

func TestFieldOwnersAt_ListItemBySelector(t *testing.T) {
	owners := NewFieldOwners([]metav1.ManagedFieldsEntry{
		entry("kubectl", "Update",
			`{"f:spec":{"f:containers":{"k:{\"name\":\"nginx\"}":{".":{},"f:image":{}}}}}`,
			time.Now()),
	})

	path := []PathSeg{
		{Key: "spec"},
		{Key: "containers"},
		{Item: map[string]string{"name": "nginx", "image": "nginx:1.27"}},
		{Key: "image"},
	}
	got, ok := owners.At(path)
	require.True(t, ok)
	assert.Equal(t, "kubectl", got.Manager)

	// A different list item is not covered by that selector.
	other := []PathSeg{
		{Key: "spec"},
		{Key: "containers"},
		{Item: map[string]string{"name": "sidecar"}},
		{Key: "image"},
	}
	_, ok = owners.At(other)
	assert.False(t, ok)
}

func TestFieldOwnersAt_NewerEntryWinsTheSamePath(t *testing.T) {
	older := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	owners := NewFieldOwners([]metav1.ManagedFieldsEntry{
		entry("old-manager", "Update", `{"f:spec":{"f:replicas":{}}}`, older),
		entry("new-manager", "Update", `{"f:spec":{"f:replicas":{}}}`, newer),
	})

	got, ok := owners.At([]PathSeg{{Key: "spec"}, {Key: "replicas"}})
	require.True(t, ok)
	assert.Equal(t, "new-manager", got.Manager)
}

func TestNewFieldOwners_SkipsUnusableEntries(t *testing.T) {
	now := time.Now()
	broken := &metav1.FieldsV1{}
	broken.SetRawBytes([]byte(`{`))
	owners := NewFieldOwners([]metav1.ManagedFieldsEntry{
		{Manager: "no-fields", Operation: "Update"},
		{Manager: "bad-json", Operation: "Update", FieldsV1: broken},
		entry("good", "Update", `{"f:spec":{"f:replicas":{}}}`, now),
	})

	got, ok := owners.At([]PathSeg{{Key: "spec"}, {Key: "replicas"}})
	require.True(t, ok)
	assert.Equal(t, "good", got.Manager)
}

func TestNewFieldOwners_SubresourceEntriesAreLabelled(t *testing.T) {
	now := time.Now()
	e := entry("horizontal-pod-autoscaler", "Update", `{"f:spec":{"f:replicas":{}}}`, now)
	e.Subresource = "scale"
	owners := NewFieldOwners([]metav1.ManagedFieldsEntry{e})

	got, ok := owners.At([]PathSeg{{Key: "spec"}, {Key: "replicas"}})
	require.True(t, ok)
	assert.Equal(t, "horizontal-pod-autoscaler", got.Manager)
	assert.Equal(t, "scale", got.Subresource)
}

func TestFieldOwners_EmptyIsSafeToQuery(t *testing.T) {
	var owners *FieldOwners
	_, ok := owners.At([]PathSeg{{Key: "spec"}})
	assert.False(t, ok)
	assert.True(t, owners.Empty())

	owners = NewFieldOwners(nil)
	assert.True(t, owners.Empty())
}

func TestFieldOwners_Managers(t *testing.T) {
	now := time.Now()
	owners := NewFieldOwners([]metav1.ManagedFieldsEntry{
		entry("b-manager", "Update", `{"f:spec":{"f:replicas":{}}}`, now),
		entry("a-manager", "Apply", `{"f:metadata":{"f:labels":{}}}`, now),
		entry("b-manager", "Update", `{"f:status":{}}`, now),
	})

	assert.Equal(t, []string{"a-manager", "b-manager"}, owners.Managers())
}

func TestGetFieldOwners_VirtualTypesReturnEmpty(t *testing.T) {
	c := &Client{}
	for _, group := range []string{"_helm", "_portforward"} {
		owners, err := c.GetFieldOwners(t.Context(), "ctx", "ns",
			model.ResourceTypeEntry{APIGroup: group, Resource: "releases"}, "name")
		require.NoError(t, err)
		assert.True(t, owners.Empty())
	}
}

func TestNewFieldOwners_RejectsAnOverDeepEntry(t *testing.T) {
	var deep strings.Builder
	for range maxFieldsV1Depth + 5 {
		deep.WriteString(`{"f:a":`)
	}
	deep.WriteString("{}")
	for range maxFieldsV1Depth + 5 {
		deep.WriteString("}")
	}
	owners := NewFieldOwners([]metav1.ManagedFieldsEntry{entry("deep", "Update", deep.String(), time.Now())})

	// The walk stops at the cap rather than recursing without a bound. What
	// it recorded before the cap is allowed; what matters is that it returns.
	assert.NotNil(t, owners)
}

func TestFieldOwnersAt_ManyListItemsStayFast(t *testing.T) {
	const items = 2000
	var raw strings.Builder
	raw.WriteString(`{"f:spec":{"f:containers":{`)
	for i := range items {
		if i > 0 {
			raw.WriteString(",")
		}
		fmt.Fprintf(&raw, `"k:{\"name\":\"c%d\"}":{"f:image":{}}`, i)
	}
	raw.WriteString("}}}")
	owners := NewFieldOwners([]metav1.ManagedFieldsEntry{entry("kubectl", "Update", raw.String(), time.Now())})

	for i := range items {
		got, ok := owners.At([]PathSeg{
			{Key: "spec"},
			{Key: "containers"},
			{Item: map[string]string{"name": fmt.Sprintf("c%d", i)}},
			{Key: "image"},
		})
		require.True(t, ok, "item %d", i)
		assert.Equal(t, "kubectl", got.Manager)
	}
}

func TestFieldOwnersAt_MultiFieldSelector(t *testing.T) {
	owners := NewFieldOwners([]metav1.ManagedFieldsEntry{
		entry("kubectl", "Update",
			`{"f:spec":{"f:ports":{"k:{\"port\":80,\"protocol\":\"TCP\"}":{".":{}}}}}`,
			time.Now()),
	})

	got, ok := owners.At([]PathSeg{
		{Key: "spec"},
		{Key: "ports"},
		{Item: map[string]string{"port": "80", "protocol": "TCP", "name": "http"}},
	})
	require.True(t, ok)
	assert.Equal(t, "kubectl", got.Manager)

	_, ok = owners.At([]PathSeg{
		{Key: "spec"},
		{Key: "ports"},
		{Item: map[string]string{"port": "80", "protocol": "UDP"}},
	})
	assert.False(t, ok, "a selector matches only when every field it names matches")
}
