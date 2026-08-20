package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// layoutCacheTestHeapSink pins items to the heap so itemsPtr is stable
// across calls, same requirement as TestTableRendererInvalidatesOnSecurityBadgeToggle.
var layoutCacheTestHeapSink []model.Item

func TestRenderTable_HasUnionCachedAcrossRenders(t *testing.T) {
	r := NewTableRenderer()
	items := []model.Item{{Name: "a", Kind: "Pod"}, {Name: "b", Kind: "Pod"}}
	layoutCacheTestHeapSink = items

	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	require.True(t, r.layout.Computed)
	assert.False(t, r.layout.HasUnion)

	items[0].ClusterName = "cluster-a"
	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	assert.False(t, r.layout.HasUnion,
		"cached layout must not rescan for hasUnion when the fingerprint is unchanged")
}

func TestRenderTable_HasUnionRecomputesOnFingerprintChange(t *testing.T) {
	r := NewTableRenderer()
	items := []model.Item{{Name: "a", Kind: "Pod"}, {Name: "b", Kind: "Pod"}}
	layoutCacheTestHeapSink = items

	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	items[0].ClusterName = "cluster-a"

	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 1, 0) // bump middleRev
	assert.True(t, r.layout.HasUnion)
}

func TestRenderTable_CategoryForItemCachedAcrossRenders(t *testing.T) {
	r := NewTableRenderer()
	items := []model.Item{
		{Name: "a", Category: "Workloads"},
		{Name: "b", Category: "Workloads"},
		{Name: "c", Category: "Networking"},
	}
	layoutCacheTestHeapSink = items

	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	require.Len(t, r.layout.CategoryForItem, 3)
	assert.Equal(t, "Workloads", r.layout.CategoryForItem[0])
	assert.Equal(t, "Networking", r.layout.CategoryForItem[2])
	assert.True(t, r.layout.HasSepForItem[2])

	items[2].Category = "Storage"
	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	assert.Equal(t, "Networking", r.layout.CategoryForItem[2],
		"cached categories must not rescan when the fingerprint is unchanged")
}

func TestRenderTable_CategoryForItemRecomputesOnFingerprintChange(t *testing.T) {
	r := NewTableRenderer()
	items := []model.Item{
		{Name: "a", Category: "Workloads"},
		{Name: "b", Category: "Workloads"},
		{Name: "c", Category: "Networking"},
	}
	layoutCacheTestHeapSink = items

	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	items[2].Category = "Storage"

	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 1, 0) // bump middleRev
	assert.Equal(t, "Storage", r.layout.CategoryForItem[2])
}

// TestRenderTable_CategoryHeadersRenderInOutput checks the cached data still
// produces the category header line in the rendered output.
func TestRenderTable_CategoryHeadersRenderInOutput(t *testing.T) {
	items := []model.Item{
		{Name: "init-a", Category: "Init Containers"},
		{Name: "sidecar-a", Category: "Sidecar Containers"},
	}
	out := RenderTable("NAME", items, 0, 80, 20, false, "", "")
	assert.Contains(t, out, "Init Containers")
	assert.Contains(t, out, "Sidecar Containers")
}
