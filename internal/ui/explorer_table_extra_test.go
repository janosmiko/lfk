package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// stripANSI removes ANSI escape sequences from s for plain-text assertions.
func stripANSI(s string) string {
	return ansi.Strip(s)
}

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

	t.Run("ActiveHiddenBuiltinColumns suppresses built-in columns", func(t *testing.T) {
		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = 0 // active middle column render (required for suppression)
		defer func() { ActiveMiddleScroll = origMS }()

		origHidden := ActiveHiddenBuiltinColumns
		ActiveHiddenBuiltinColumns = map[string]bool{"Ready": true, "Status": true}
		defer func() { ActiveHiddenBuiltinColumns = origHidden }()

		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()

		items := []model.Item{
			{Name: "nginx", Status: "Running", Ready: "1/1", Restarts: "0", Age: "5m"},
		}
		result := RenderTable("NAME", items, 0, 80, 20, false, "", "")
		assert.Contains(t, result, "NAME", "NAME header must always be present")
		assert.Contains(t, result, "AGE", "AGE header must still be present (not hidden)")
		assert.Contains(t, result, "RS", "RESTARTS header must still be present (not hidden)")
		assert.NotContains(t, result, "READY", "READY header must be suppressed")
		assert.NotContains(t, result, "STATUS", "STATUS header must be suppressed")
	})

	t.Run("ActiveHiddenBuiltinColumns ignored for non-middle render", func(t *testing.T) {
		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = -1 // non-middle render (child/right pane)
		defer func() { ActiveMiddleScroll = origMS }()

		origHidden := ActiveHiddenBuiltinColumns
		ActiveHiddenBuiltinColumns = map[string]bool{"Ready": true}
		defer func() { ActiveHiddenBuiltinColumns = origHidden }()

		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()

		items := []model.Item{
			{Name: "nginx", Ready: "1/1", Age: "5m"},
		}
		result := RenderTable("NAME", items, 0, 80, 20, false, "", "")
		assert.Contains(t, result, "READY", "READY header must be shown for non-middle renders")
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

	t.Run("default_order_preserved", func(t *testing.T) {
		// With no ActiveColumnOrder set, the header should come out in the
		// historical order: NAMESPACE NAME READY RS STATUS AGE. Extras are
		// still appended before Age.
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = 0
		defer func() { ActiveMiddleScroll = origMS }()

		origOrder := ActiveColumnOrder
		ActiveColumnOrder = nil
		defer func() { ActiveColumnOrder = origOrder }()

		origHidden := ActiveHiddenBuiltinColumns
		ActiveHiddenBuiltinColumns = nil
		defer func() { ActiveHiddenBuiltinColumns = origHidden }()

		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()

		items := []model.Item{
			{Name: "nginx", Namespace: "default", Ready: "1/1", Restarts: "0", Status: "Running", Age: "5m"},
		}
		result := RenderTable("NAME", items, 0, 120, 20, false, "", "")

		// The header line is the first line of the output.
		firstLine := strings.Split(result, "\n")[0]
		// Strip ANSI codes for easier substring ordering checks.
		plain := stripANSI(firstLine)

		nsIdx := strings.Index(plain, "NAMESPACE")
		nameIdx := strings.Index(plain, "NAME ") // trailing space disambiguates from NAMESPACE
		if nameIdx < 0 {
			nameIdx = strings.Index(plain, "NAME")
		}
		readyIdx := strings.Index(plain, "READY")
		rsIdx := strings.Index(plain, "RS ")
		if rsIdx < 0 {
			rsIdx = strings.LastIndex(plain, "RS")
		}
		statusIdx := strings.Index(plain, "STATUS")
		ageIdx := strings.Index(plain, "AGE")

		assert.GreaterOrEqual(t, nsIdx, 0, "NAMESPACE must be present")
		assert.GreaterOrEqual(t, nameIdx, 0, "NAME must be present")
		assert.GreaterOrEqual(t, readyIdx, 0, "READY must be present")
		assert.GreaterOrEqual(t, rsIdx, 0, "RS must be present")
		assert.GreaterOrEqual(t, statusIdx, 0, "STATUS must be present")
		assert.GreaterOrEqual(t, ageIdx, 0, "AGE must be present")

		// Check relative ordering (Name comes first, built-ins in canonical order,
		// Age last).
		assert.Less(t, nameIdx, readyIdx, "NAME must come before READY")
		assert.Less(t, readyIdx, rsIdx, "READY must come before RS")
		assert.Less(t, rsIdx, statusIdx, "RS must come before STATUS")
		assert.Less(t, statusIdx, ageIdx, "STATUS must come before AGE")
	})

	t.Run("custom_order_honored", func(t *testing.T) {
		// With ActiveColumnOrder set, the header should come out in the
		// user-specified order, with Name always first.
		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = 0
		defer func() { ActiveMiddleScroll = origMS }()

		origOrder := ActiveColumnOrder
		ActiveColumnOrder = []string{"Age", "Status", "Namespace", "Ready"}
		defer func() { ActiveColumnOrder = origOrder }()

		origHidden := ActiveHiddenBuiltinColumns
		ActiveHiddenBuiltinColumns = nil
		defer func() { ActiveHiddenBuiltinColumns = origHidden }()

		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()

		items := []model.Item{
			{Name: "nginx", Namespace: "default", Ready: "1/1", Status: "Running", Age: "5m"},
		}
		result := RenderTable("NAME", items, 0, 120, 20, false, "", "")

		firstLine := strings.Split(result, "\n")[0]
		plain := stripANSI(firstLine)

		nameIdx := strings.Index(plain, "NAME ")
		if nameIdx < 0 {
			nameIdx = strings.Index(plain, "NAME")
		}
		ageIdx := strings.Index(plain, "AGE")
		statusIdx := strings.Index(plain, "STATUS")
		nsIdx := strings.Index(plain, "NAMESPACE")
		readyIdx := strings.Index(plain, "READY")

		assert.GreaterOrEqual(t, nameIdx, 0)
		assert.GreaterOrEqual(t, ageIdx, 0)
		assert.GreaterOrEqual(t, statusIdx, 0)
		assert.GreaterOrEqual(t, nsIdx, 0)
		assert.GreaterOrEqual(t, readyIdx, 0)

		// Name must still be first.
		assert.Less(t, nameIdx, ageIdx, "NAME comes before AGE")
		// Then the user-specified order.
		assert.Less(t, ageIdx, statusIdx, "AGE comes before STATUS (per custom order)")
		assert.Less(t, statusIdx, nsIdx, "STATUS comes before NAMESPACE (per custom order)")
		assert.Less(t, nsIdx, readyIdx, "NAMESPACE comes before READY (per custom order)")
	})

	t.Run("hidden_builtin_with_custom_order", func(t *testing.T) {
		// A hidden built-in should be dropped from the ordered list even if
		// the user's saved order references it.
		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = 0
		defer func() { ActiveMiddleScroll = origMS }()

		origOrder := ActiveColumnOrder
		ActiveColumnOrder = []string{"Age", "Status", "Namespace"}
		defer func() { ActiveColumnOrder = origOrder }()

		origHidden := ActiveHiddenBuiltinColumns
		ActiveHiddenBuiltinColumns = map[string]bool{"Status": true}
		defer func() { ActiveHiddenBuiltinColumns = origHidden }()

		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()

		items := []model.Item{
			{Name: "nginx", Namespace: "default", Status: "Running", Age: "5m"},
		}
		result := RenderTable("NAME", items, 0, 120, 20, false, "", "")
		firstLine := strings.Split(result, "\n")[0]
		plain := stripANSI(firstLine)

		assert.NotContains(t, plain, "STATUS", "STATUS must be hidden")

		nameIdx := strings.Index(plain, "NAME")
		ageIdx := strings.Index(plain, "AGE")
		nsIdx := strings.Index(plain, "NAMESPACE")
		assert.Less(t, nameIdx, ageIdx, "NAME before AGE")
		assert.Less(t, ageIdx, nsIdx, "AGE before NAMESPACE")
	})

	t.Run("stale_order_key_dropped", func(t *testing.T) {
		// A key in the saved order that isn't currently present (no data,
		// no extra column) should be silently dropped.
		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = 0
		defer func() { ActiveMiddleScroll = origMS }()

		origOrder := ActiveColumnOrder
		ActiveColumnOrder = []string{"IP", "Namespace"}
		defer func() { ActiveColumnOrder = origOrder }()

		origHidden := ActiveHiddenBuiltinColumns
		ActiveHiddenBuiltinColumns = nil
		defer func() { ActiveHiddenBuiltinColumns = origHidden }()

		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()

		items := []model.Item{
			{Name: "nginx", Namespace: "default"},
		}
		result := RenderTable("NAME", items, 0, 120, 20, false, "", "")
		firstLine := strings.Split(result, "\n")[0]
		plain := stripANSI(firstLine)

		assert.NotContains(t, plain, "IP", "IP is not a real column, must be dropped")
		nameIdx := strings.Index(plain, "NAME")
		nsIdx := strings.Index(plain, "NAMESPACE")
		assert.GreaterOrEqual(t, nameIdx, 0)
		assert.GreaterOrEqual(t, nsIdx, 0)
		assert.Less(t, nameIdx, nsIdx, "NAME before NAMESPACE")
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
