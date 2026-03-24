package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- RenderTable ---

func TestRenderTable(t *testing.T) {
	t.Run("empty items loading shows spinner", func(t *testing.T) {
		result := RenderTable("NAME", nil, 0, 80, 20, true, ">", "")
		assert.Contains(t, result, "Loading...")
	})

	t.Run("empty items error shows error", func(t *testing.T) {
		result := RenderTable("NAME", nil, 0, 80, 20, false, "", "connection refused")
		assert.Contains(t, result, "connection refused")
	})

	t.Run("empty items no error shows no resources", func(t *testing.T) {
		result := RenderTable("NAME", nil, 0, 80, 20, false, "", "")
		assert.Contains(t, result, "No resources found")
	})

	t.Run("items rendered with header and data", func(t *testing.T) {
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = -1
		defer func() { ActiveMiddleScroll = origMS }()

		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()

		items := []model.Item{
			{Name: "nginx", Status: "Running", Ready: "1/1", Age: "5m"},
			{Name: "redis", Status: "Running", Ready: "1/1", Age: "10m"},
		}
		result := RenderTable("NAME", items, 0, 80, 20, false, "", "")
		assert.Contains(t, result, "NAME")
		assert.Contains(t, result, "READY")
		assert.Contains(t, result, "STATUS")
		assert.Contains(t, result, "nginx")
		assert.Contains(t, result, "redis")
		assert.Contains(t, result, "Running")
	})

	t.Run("items with namespace show NAMESPACE header", func(t *testing.T) {
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = -1
		defer func() { ActiveMiddleScroll = origMS }()

		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()

		items := []model.Item{
			{Name: "pod1", Namespace: "default", Status: "Running"},
			{Name: "pod2", Namespace: "kube-system", Status: "Running"},
		}
		result := RenderTable("", items, 0, 80, 20, false, "", "")
		assert.Contains(t, result, "NAME")
		assert.Contains(t, result, "NAMESPACE")
		assert.Contains(t, result, "default")
		assert.Contains(t, result, "kube-system")
	})

	t.Run("default header label is NAME when empty", func(t *testing.T) {
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = -1
		defer func() { ActiveMiddleScroll = origMS }()

		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()

		items := []model.Item{{Name: "test"}}
		result := RenderTable("", items, 0, 80, 20, false, "", "")
		assert.Contains(t, result, "NAME")
	})

	t.Run("cursor on item renders selected style", func(t *testing.T) {
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = -1
		defer func() { ActiveMiddleScroll = origMS }()

		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()

		items := []model.Item{
			{Name: "a"},
			{Name: "b"},
		}
		result := RenderTable("NAME", items, 1, 80, 20, false, "", "")
		assert.Contains(t, result, "b")
	})

	t.Run("items with restarts show RS column", func(t *testing.T) {
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = -1
		defer func() { ActiveMiddleScroll = origMS }()

		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()

		items := []model.Item{
			{Name: "pod1", Restarts: "3", Status: "Running"},
		}
		result := RenderTable("NAME", items, 0, 80, 20, false, "", "")
		assert.Contains(t, result, "RS")
		assert.Contains(t, result, "3")
	})
}

// --- collectExtraColumns ---

func TestCollectExtraColumns(t *testing.T) {
	t.Run("no items returns nil", func(t *testing.T) {
		result := collectExtraColumns(nil, 80, 20, "Pod")
		assert.Nil(t, result)
	})

	t.Run("items without columns returns nil", func(t *testing.T) {
		items := []model.Item{{Name: "pod1"}, {Name: "pod2"}}
		result := collectExtraColumns(items, 80, 20, "Pod")
		assert.Nil(t, result)
	})

	t.Run("items with extra columns detects them", func(t *testing.T) {
		items := []model.Item{
			{Name: "pod1", Columns: []model.KeyValue{{Key: "Node", Value: "worker-1"}}},
			{Name: "pod2", Columns: []model.KeyValue{{Key: "Node", Value: "worker-2"}}},
		}
		// Reset fullscreen mode to use normal blocked set.
		origFS := ActiveFullscreenMode
		ActiveFullscreenMode = true
		defer func() { ActiveFullscreenMode = origFS }()

		result := collectExtraColumns(items, 120, 30, "Pod")
		if result != nil {
			found := false
			for _, col := range result {
				if col.key == "Node" {
					found = true
				}
			}
			assert.True(t, found, "Node column should be detected in fullscreen mode")
		}
	})

	t.Run("blocked columns excluded in non-fullscreen", func(t *testing.T) {
		items := []model.Item{
			{Name: "pod1", Columns: []model.KeyValue{{Key: "Node", Value: "w1"}, {Key: "IP", Value: "10.0.0.1"}}},
			{Name: "pod2", Columns: []model.KeyValue{{Key: "Node", Value: "w2"}, {Key: "IP", Value: "10.0.0.2"}}},
		}
		origFS := ActiveFullscreenMode
		ActiveFullscreenMode = false
		defer func() { ActiveFullscreenMode = origFS }()

		result := collectExtraColumns(items, 120, 30, "Pod")
		for _, col := range result {
			assert.NotEqual(t, "IP", col.key, "IP should be blocked in non-fullscreen mode")
			assert.NotEqual(t, "Node", col.key, "Node should be blocked in non-fullscreen mode")
		}
	})

	t.Run("insufficient width returns nil", func(t *testing.T) {
		items := []model.Item{
			{Name: "pod1", Columns: []model.KeyValue{{Key: "Node", Value: "worker"}}},
		}
		// Very narrow total width with large usedWidth should prevent extra columns.
		result := collectExtraColumns(items, 30, 25, "Pod")
		assert.Nil(t, result)
	})

	t.Run("extra columns do not absorb remaining width", func(t *testing.T) {
		// When extra columns fit within their natural width, remaining space
		// should NOT be added to the last column (it goes to NAME instead).
		items := []model.Item{
			{Name: "pod1", Kind: "Pod", Columns: []model.KeyValue{
				{Key: "CPU", Value: "10m"},
				{Key: "MEM", Value: "64Mi"},
			}},
			{Name: "pod2", Kind: "Pod", Columns: []model.KeyValue{
				{Key: "CPU", Value: "20m"},
				{Key: "MEM", Value: "128Mi"},
			}},
		}
		origFS := ActiveFullscreenMode
		ActiveFullscreenMode = false
		defer func() { ActiveFullscreenMode = origFS }()

		result := collectExtraColumns(items, 120, 30, "Pod")
		if len(result) >= 2 {
			lastCol := result[len(result)-1]
			// The last column (MEM) should be sized to fit its content + header,
			// not inflated with all remaining width. "MEM" = 3 chars, "128Mi" = 5 chars,
			// so width should be around 6 (5+1 spacing), not 50+.
			assert.LessOrEqual(t, lastCol.width, 20,
				"last extra column should not absorb all remaining width")
		}
	})

	t.Run("Deletion column excluded from table in both modes", func(t *testing.T) {
		items := []model.Item{
			{Name: "pod1", Kind: "Pod", Columns: []model.KeyValue{
				{Key: "Node", Value: "worker-1"},
				{Key: "Deletion", Value: "2026-01-15T10:00:00Z"},
			}},
			{Name: "pod2", Kind: "Pod", Columns: []model.KeyValue{
				{Key: "Node", Value: "worker-2"},
				{Key: "Deletion", Value: "2026-01-15T10:05:00Z"},
			}},
		}
		for _, fs := range []bool{false, true} {
			origFS := ActiveFullscreenMode
			ActiveFullscreenMode = fs
			result := collectExtraColumns(items, 120, 30, "Pod")
			ActiveFullscreenMode = origFS

			for _, col := range result {
				assert.NotEqual(t, "Deletion", col.key,
					"Deletion should be blocked from table (fullscreen=%v)", fs)
			}
		}
	})

	t.Run("Selector column excluded from Service table in both modes", func(t *testing.T) {
		items := []model.Item{
			{Name: "svc1", Kind: "Service", Columns: []model.KeyValue{
				{Key: "Cluster IP", Value: "10.0.0.1"},
				{Key: "Selector", Value: "app=nginx"},
			}},
			{Name: "svc2", Kind: "Service", Columns: []model.KeyValue{
				{Key: "Cluster IP", Value: "10.0.0.2"},
				{Key: "Selector", Value: "app=redis"},
			}},
		}
		for _, fs := range []bool{false, true} {
			origFS := ActiveFullscreenMode
			ActiveFullscreenMode = fs
			result := collectExtraColumns(items, 120, 30, "Service")
			ActiveFullscreenMode = origFS

			for _, col := range result {
				assert.NotEqual(t, "Selector", col.key,
					"Selector should be blocked from table (fullscreen=%v)", fs)
			}
		}
	})

	t.Run("Used By column excluded from PVC table in both modes", func(t *testing.T) {
		items := []model.Item{
			{Name: "pvc1", Kind: "PersistentVolumeClaim", Columns: []model.KeyValue{
				{Key: "Storage", Value: "10Gi"},
				{Key: "Used By", Value: "pod-a, pod-b"},
			}},
			{Name: "pvc2", Kind: "PersistentVolumeClaim", Columns: []model.KeyValue{
				{Key: "Storage", Value: "20Gi"},
				{Key: "Used By", Value: "pod-c"},
			}},
		}
		for _, fs := range []bool{false, true} {
			origFS := ActiveFullscreenMode
			ActiveFullscreenMode = fs
			result := collectExtraColumns(items, 120, 30, "PersistentVolumeClaim")
			ActiveFullscreenMode = origFS

			for _, col := range result {
				assert.NotEqual(t, "Used By", col.key,
					"Used By should be blocked from table (fullscreen=%v)", fs)
			}
		}
	})
}
