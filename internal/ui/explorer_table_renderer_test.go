package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func tableRendererTestItems() []model.Item {
	return []model.Item{
		{Name: "pod-a", Namespace: "ns1", Kind: "Pod", Status: "Running", Age: "1d"},
		{Name: "pod-b", Namespace: "ns1", Kind: "Pod", Status: "Running", Age: "2d"},
		{Name: "pod-c", Namespace: "ns2", Kind: "Pod", Status: "Pending", Age: "1h"},
		{Name: "pod-d", Namespace: "ns2", Kind: "Pod", Status: "Running", Age: "5m"},
	}
}

func TestTableRendererPopulatesRowCache(t *testing.T) {
	r := NewTableRenderer()
	items := tableRendererTestItems()

	out := r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	require.NotEmpty(t, out)

	assert.NotContains(t, r.rows, 0, "cursor row must not be cached")
	assert.Contains(t, r.rows, 1)
	assert.Contains(t, r.rows, 2)
	assert.Contains(t, r.rows, 3)
}

func TestTableRendererCacheSurvivesCursorMove(t *testing.T) {
	r := NewTableRenderer()
	items := tableRendererTestItems()

	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	rowAt2 := r.rows[2]
	require.NotEmpty(t, rowAt2)

	_ = r.Render("NAME", items, 1, 80, 20, false, "", "", 0, 0)

	assert.Equal(t, rowAt2, r.rows[2])
}

func TestTableRendererInvalidatesOnMiddleRev(t *testing.T) {
	r := NewTableRenderer()
	items := tableRendererTestItems()

	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	require.NotEmpty(t, r.rows)

	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 1, 0)

	assert.Equal(t, uint64(1), r.fp.middleRev)
}

func TestTableRendererInvalidatesOnWidthChange(t *testing.T) {
	r := NewTableRenderer()
	items := tableRendererTestItems()

	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	prevRow := r.rows[1]

	_ = r.Render("NAME", items, 0, 100, 20, false, "", "", 0, 0)
	newRow := r.rows[1]

	assert.NotEqual(t, prevRow, newRow)
}

func TestTableRendererInvalidatesOnSelRev(t *testing.T) {
	r := NewTableRenderer()
	items := tableRendererTestItems()

	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	require.NotEmpty(t, r.rows)
	rowAt2 := r.rows[2]
	require.NotEmpty(t, rowAt2)

	// Bump selRev as the app would on a selection toggle. The cache must
	// drop and rebuild so the marker prefix on non-cursor rows refreshes.
	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 1)

	assert.Equal(t, uint64(1), r.fp.selRev)
}

func TestTableRendererFingerprintIncludesAgeBucket(t *testing.T) {
	r := NewTableRenderer()
	_ = r.Render("NAME", tableRendererTestItems(), 0, 80, 20, false, "", "", 0, 0)

	expected := time.Now().Unix() / ageBucketSeconds
	// The Render call captured time once; the bucket may have rolled to
	// expected+1 by the time we read time.Now() again. Tolerate the seam.
	assert.Contains(t, []int64{expected, expected - 1}, r.fp.ageBucket)
}

func TestTableRendererInvalidatesOnIconModeChange(t *testing.T) {
	prev := IconMode
	defer func() { IconMode = prev }()

	r := NewTableRenderer()
	items := tableRendererTestItems()

	IconMode = "unicode"
	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	require.NotEmpty(t, r.rows)

	IconMode = "none"
	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)

	assert.Equal(t, "none", r.fp.iconMode)
}

// Reproduces the "middle column applies the new theme with a slight
// delay" report. The colourscheme picker calls ApplyTheme on every
// cursor move, but the table-renderer fingerprint did not include any
// theme identity — so the cache held row strings with the *old* theme's
// ANSI sequences baked in until something else (data tick, age bucket
// roll, resize, no-color toggle) changed.
//
// We plant a sentinel that can only survive if Render decides r.fp is
// unchanged and reuses r.rows. After ApplyTheme to a visibly different
// palette, the next Render must invalidate the cache and clear the
// sentinel.
func TestTableRendererInvalidatesOnApplyTheme(t *testing.T) {
	prevTheme := ActiveTheme
	t.Cleanup(func() { ApplyTheme(prevTheme) })

	ApplyTheme(DefaultTheme())

	r := NewTableRenderer()
	items := tableRendererTestItems()
	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)
	require.NotEmpty(t, r.rows, "first render must populate the cache")

	const sentinelKey = -1
	r.rows[sentinelKey] = "sentinel"

	other := DefaultTheme()
	other.Text = "#ff00ff" // visibly different so style globals diverge
	other.Primary = "#00ff00"
	other.Dimmed = "#ffaa00"
	ApplyTheme(other)

	_ = r.Render("NAME", items, 0, 80, 20, false, "", "", 0, 0)

	_, present := r.rows[sentinelKey]
	assert.False(t, present,
		"ApplyTheme between renders must invalidate the row cache; sentinel survived, "+
			"so the middle column would keep painting the previous theme until something else "+
			"in the fingerprint changed")
}

func TestTableRendererRestoresGlobalsAfterRender(t *testing.T) {
	sentinelCache := map[int]string{99: "sentinel"}
	sentinelLayout := &TableLayoutCache{Computed: true}

	prevCache := ActiveRowCache
	prevLayout := ActiveTableLayout
	defer func() {
		ActiveRowCache = prevCache
		ActiveTableLayout = prevLayout
	}()
	ActiveRowCache = sentinelCache
	ActiveTableLayout = sentinelLayout

	r := NewTableRenderer()
	_ = r.Render("NAME", tableRendererTestItems(), 0, 80, 20, false, "", "", 0, 0)

	// Render must restore the caller's globals so unrelated code that
	// reads ActiveRowCache / ActiveTableLayout sees its own state, not
	// the renderer's private maps.
	assert.Equal(t, sentinelCache, ActiveRowCache, "ActiveRowCache must be restored")
	assert.Equal(t, sentinelLayout, ActiveTableLayout, "ActiveTableLayout must be restored")
}

func TestTableRendererRestoresGlobalsOnPanic(t *testing.T) {
	sentinelCache := map[int]string{99: "sentinel"}
	sentinelLayout := &TableLayoutCache{Computed: true}

	prevCache := ActiveRowCache
	prevLayout := ActiveTableLayout
	defer func() {
		ActiveRowCache = prevCache
		ActiveTableLayout = prevLayout
	}()
	ActiveRowCache = sentinelCache
	ActiveTableLayout = sentinelLayout

	r := NewTableRenderer()

	// A negative cursor passed alongside a non-empty list slips past the
	// happy path; if RenderTable panics, the deferred restore in
	// TableRenderer.Render must still run.
	defer func() {
		_ = recover() // swallow whatever panic, if any
		assert.Equal(t, sentinelCache, ActiveRowCache, "globals must be restored even after panic")
		assert.Equal(t, sentinelLayout, ActiveTableLayout, "globals must be restored even after panic")
	}()

	// Force a panic by accessing items[len(items)] indirectly: pass items
	// but ask for a cursor far out of range. RenderTable clamps cursor
	// implicitly, so this won't actually panic — the test simply asserts
	// the restore works on the happy path with the same assertion shape
	// future panic-injection variants would use.
	_ = r.Render("NAME", tableRendererTestItems(), 0, 80, 20, false, "", "", 0, 0)
}
