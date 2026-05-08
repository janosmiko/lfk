package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestUpdateContextsLoaded_StampsLocalClusterStatusFromCache verifies
// that the contexts-loaded handler walks the freshly loaded list and
// stamps each row's LocalClusterStatus from m.localClusterCache, keyed
// by context name. Rows without an entry in the cache must be left
// with an empty status so the renderer treats them as "not local".
func TestUpdateContextsLoaded_StampsLocalClusterStatusFromCache(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	m.localClusterCache = map[string]localClusterCacheEntry{
		"kind-dev":  {Provider: "kind", Name: "dev", ContextName: "kind-dev", Status: "running"},
		"k3d-stage": {Provider: "k3d", Name: "stage", ContextName: "k3d-stage", Status: "stopped"},
	}
	msg := contextsLoadedMsg{items: []model.Item{
		{Name: "kind-dev"},
		{Name: "k3d-stage"},
		{Name: "gke-prod"},
	}}
	updated, _ := m.Update(msg)
	rm := updated.(Model)
	assert.Equal(t, "running", rm.middleItems[0].LocalClusterStatus,
		"kind-dev must inherit running from the cache")
	assert.Equal(t, "stopped", rm.middleItems[1].LocalClusterStatus,
		"k3d-stage must inherit stopped from the cache")
	assert.Equal(t, "", rm.middleItems[2].LocalClusterStatus,
		"gke-prod has no cache entry — status must be empty so the renderer skips the icon")
}

// TestFormatItem_PrependsRunningIconForLocalCluster asserts that a
// row whose Item carries LocalClusterStatus="running" renders the
// filled-circle glyph in front of the name.
func TestFormatItem_PrependsRunningIconForLocalCluster(t *testing.T) {
	item := model.Item{Name: "kind-dev", IsContext: true, LocalClusterStatus: "running"}
	out := ui.FormatItem(item, 40)
	assert.True(t, strings.Contains(out, "●"),
		"running local-cluster row must include the filled-circle status glyph; got %q", out)
}

// TestFormatItem_PrependsStoppedIconForLocalCluster asserts that a
// row whose Item carries LocalClusterStatus="stopped" renders the
// hollow-circle glyph in front of the name.
func TestFormatItem_PrependsStoppedIconForLocalCluster(t *testing.T) {
	item := model.Item{Name: "kind-dev", IsContext: true, LocalClusterStatus: "stopped"}
	out := ui.FormatItem(item, 40)
	assert.True(t, strings.Contains(out, "○"),
		"stopped local-cluster row must include the hollow-circle status glyph; got %q", out)
}

// TestFormatItem_NoIconForNonLocalCluster ensures the icon is suppressed
// for plain-context rows so we don't paint a glyph on managed clusters.
func TestFormatItem_NoIconForNonLocalCluster(t *testing.T) {
	item := model.Item{Name: "gke-prod"}
	out := ui.FormatItem(item, 40)
	assert.False(t, strings.Contains(out, "●"),
		"non-local row must not gain the running glyph; got %q", out)
	assert.False(t, strings.Contains(out, "○"),
		"non-local row must not gain the stopped glyph; got %q", out)
}
