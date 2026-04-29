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

	t.Run("namespace shrinks so long pod names render without truncation", func(t *testing.T) {
		// Regression: a 28-char namespace burned ~29 columns on the
		// NAMESPACE column at the default cap, leaving the NAME column
		// too narrow for a 35-char pod name even though the row had
		// plenty of total width. Now nsW shrinks toward its header
		// width so name fits when the budget allows it.
		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = -1
		defer func() { ActiveMiddleScroll = origMS }()
		origLayout := ActiveTableLayout
		ActiveTableLayout = nil
		defer func() { ActiveTableLayout = origLayout }()
		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()

		longName := "nginx-deployment-7c9f8d8f6c-x9k2m" // 33 chars
		longNs := "kube-system-very-long-name"          // 26 chars
		items := []model.Item{
			{Name: longName, Namespace: longNs, Status: "Running", Ready: "1/1", Age: "5m"},
		}
		result := RenderTable("NAME", items, 0, 100, 20, false, "", "")
		plain := stripANSI(result)
		assert.Contains(t, plain, longName,
			"long pod name must render in full when total width allows it")
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

	t.Run("capped column grows into surplus budget instead of padding NAME", func(t *testing.T) {
		// Reproduces the bug where Service PORTS get truncated (e.g. "443:3~")
		// while NAME ends up with a huge empty pad. The 20-char cap prevented
		// Ports from using available leftover budget that flowed to NAME.
		items := []model.Item{
			{Name: "svc1", Kind: "Service", Columns: []model.KeyValue{
				{Key: "Type", Value: "LoadBalancer"},
				{Key: "Ports", Value: "80:31539/TCP, 443:31443/TCP"}, // 27 chars
			}},
			{Name: "svc2", Kind: "Service", Columns: []model.KeyValue{
				{Key: "Type", Value: "ClusterIP"},
				{Key: "Ports", Value: "80/TCP"},
			}},
		}
		origFS := ActiveFullscreenMode
		ActiveFullscreenMode = false
		defer func() { ActiveFullscreenMode = origFS }()

		// 200-wide table, usedWidth=30. Plenty of surplus.
		result := collectExtraColumns(items, 200, 30, "Service")

		var portsW int
		for _, col := range result {
			if col.key == "Ports" {
				portsW = col.width
			}
		}
		// Natural width = 27 (value) + 1 (spacing) = 28. Should grow up to that
		// instead of being capped at 20+1=21 while NAME absorbs the slack.
		assert.GreaterOrEqual(t, portsW, 28,
			"Ports column should grow to fit content when surplus is available")
	})

	t.Run("surplus growth respects natural width (no over-padding)", func(t *testing.T) {
		// Even with abundant width, columns that already fit should not grow
		// past their natural size — slack still flows to NAME for short content.
		items := []model.Item{
			{Name: "pod1", Kind: "Pod", Columns: []model.KeyValue{
				{Key: "CPU", Value: "10m"},
				{Key: "MEM", Value: "64Mi"},
			}},
		}
		origFS := ActiveFullscreenMode
		ActiveFullscreenMode = false
		defer func() { ActiveFullscreenMode = origFS }()

		result := collectExtraColumns(items, 200, 30, "Pod")
		for _, col := range result {
			// Header is the longest of {key, value+1}; +1 spacing column.
			natural := len(col.key)
			if col.key == "MEM" && natural < 5 {
				natural = 5 // value "64Mi" is 4, +1 arrow buffer not in play here
			}
			assert.LessOrEqual(t, col.width, natural+1+1,
				"column %q grew beyond natural width", col.key)
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

// TestCollectExtraColumns_NameReservationGrowsWithLongestName is the
// regression test for issue #53: when item names are long, the Name
// column gets squeezed to the 20-char floor while extras (HOSTS,
// TLS HOSTS, ADDRESS, …) fill the rest. Reserve more for Name when
// items have long names so the Name column gets the space it needs and
// extras fall back to their cap (or get dropped) instead of always
// winning the budget.
func TestCollectExtraColumns_NameReservationGrowsWithLongestName(t *testing.T) {
	// Identical extras, identical width budget — only Name length differs.
	// Extras' natural total exceeds the post-Name-reserve budget so the
	// difference between long and short Name reservation actually shows
	// up at the wire.
	extras := []model.KeyValue{
		{Key: "HOSTS", Value: "very-long-ingress-host.example.com"},
		{Key: "TLS HOSTS", Value: "another-long-tls-host.example.com"},
		{Key: "ADDRESS", Value: "ingress.aws.elb.example.com"},
		{Key: "INGRESS CLASS", Value: "nginx-public-ingress-controller"},
	}
	longName := "very-long-ingress-name-that-needs-room" // 39 chars
	shortName := "ing-1"                                 // 5 chars

	longItems := []model.Item{{Name: longName, Kind: "Ingress", Columns: extras}}
	shortItems := []model.Item{{Name: shortName, Kind: "Ingress", Columns: extras}}

	const totalWidth, usedWidth = 180, 25

	origFS := ActiveFullscreenMode
	ActiveFullscreenMode = true
	t.Cleanup(func() { ActiveFullscreenMode = origFS })

	sumWidths := func(cols []extraColumn) int {
		total := 0
		for _, c := range cols {
			total += c.width
		}
		return total
	}

	longResult := collectExtraColumns(longItems, totalWidth, usedWidth, "Ingress")
	shortResult := collectExtraColumns(shortItems, totalWidth, usedWidth, "Ingress")

	longTotal := sumWidths(longResult)
	shortTotal := sumWidths(shortResult)

	assert.Less(t, longTotal, shortTotal,
		"long item names must shrink the budget available to extra columns "+
			"(longTotal=%d, shortTotal=%d) so the Name column gets the "+
			"space it needs — issue #53", longTotal, shortTotal)

	// And the Name column (computed by the caller as totalWidth - usedWidth -
	// extraTotal) must end up wider than the legacy 20-char floor in the
	// long-name case.
	longNameW := totalWidth - usedWidth - longTotal
	assert.Greater(t, longNameW, 20,
		"with long item names, the residual Name column must exceed the "+
			"old 20-char hard floor (got %d)", longNameW)
}

// TestCollectExtraColumns_NameReservationCappedAtHalfWidth guards against
// over-correction: a single pathologically long name (200 chars) must
// not eat the entire budget. Cap the Name reservation at half the total
// width so extras still get a fair share.
func TestCollectExtraColumns_NameReservationCappedAtHalfWidth(t *testing.T) {
	extras := []model.KeyValue{
		{Key: "HOSTS", Value: "small.example.com"},
	}
	hugeName := strings.Repeat("a", 200)
	items := []model.Item{{Name: hugeName, Kind: "Ingress", Columns: extras}}

	origFS := ActiveFullscreenMode
	ActiveFullscreenMode = true
	t.Cleanup(func() { ActiveFullscreenMode = origFS })

	const totalWidth, usedWidth = 120, 25
	result := collectExtraColumns(items, totalWidth, usedWidth, "Ingress")

	assert.NotEmpty(t, result,
		"a single very long name must not starve extras to nothing — "+
			"Name reservation has to be capped so HOSTS still appears")
}

// TestRenderTable_NodeNamesNotTruncatedEarly is a regression test for the
// reported "node names still get truncated" bug: realistic Node items with
// FQDN-style names plus the columns that populateNodeDetails attaches (Role,
// Version, Taints, address types in fullscreen) must not be cut off when the
// terminal is wide enough to show them in full.
//
// The previous fix (commit 1331bd2) reserved longestName+1 for the Name
// column inside collectExtraColumns. Long node names re-surface the bug when
// an aggressive extras column (Role with several comma-separated values,
// Taints with NoSchedule/NoExecute markers) inflates beyond the cap, eating
// the Name budget.
func TestRenderTable_NodeNamesNotTruncatedEarly(t *testing.T) {
	origMS := ActiveMiddleScroll
	ActiveMiddleScroll = -1
	t.Cleanup(func() { ActiveMiddleScroll = origMS })

	origQuery := ActiveHighlightQuery
	ActiveHighlightQuery = ""
	t.Cleanup(func() { ActiveHighlightQuery = origQuery })

	origSel := ActiveSelectedItems
	ActiveSelectedItems = nil
	t.Cleanup(func() { ActiveSelectedItems = origSel })

	// Realistic node names: long FQDN format common on managed K8s. These
	// are 50+ chars; the regression hides exactly when names are long.
	names := []string{
		"ip-10-0-1-100.eu-west-1.compute.internal",
		"prod-eu-west-1-worker-pool-spot-instance-12345-67890",
		"gke-cluster-default-pool-1a2b3c4d-abcd",
	}
	items := make([]model.Item, 0, len(names))
	for _, n := range names {
		items = append(items, model.Item{
			Name:   n,
			Kind:   "Node",
			Status: "Ready",
			Ready:  "True",
			Age:    "10d",
			Columns: []model.KeyValue{
				{Key: "Role", Value: "control-plane,master,etcd"},
				{Key: "Version", Value: "v1.28.5+rke2r1"},
				{Key: "Taints", Value: "node-role.kubernetes.io/control-plane:=NoSchedule, node.kubernetes.io/disk-pressure:=NoSchedule"},
				{Key: "InternalIP", Value: "10.0.1.100"},
				{Key: "ExternalIP", Value: "54.123.45.67"},
				{Key: "Hostname", Value: "ip-10-0-1-100"},
			},
		})
	}

	t.Run("normal mode 200 columns: full names visible", func(t *testing.T) {
		origFS := ActiveFullscreenMode
		ActiveFullscreenMode = false
		t.Cleanup(func() { ActiveFullscreenMode = origFS })

		out := stripANSI(RenderTable("NAME", items, 0, 200, 20, false, "", ""))
		for _, n := range names {
			assert.Contains(t, out, n,
				"node name %q must appear in full at 200 cols (truncated form would have a trailing ~)", n)
		}
	})

	t.Run("fullscreen 200 columns: full names visible", func(t *testing.T) {
		origFS := ActiveFullscreenMode
		ActiveFullscreenMode = true
		t.Cleanup(func() { ActiveFullscreenMode = origFS })

		out := stripANSI(RenderTable("NAME", items, 0, 200, 20, false, "", ""))
		for _, n := range names {
			assert.Contains(t, out, n,
				"node name %q must appear in full at 200 cols in fullscreen mode", n)
		}
	})

	t.Run("normal mode 140 columns: longest name still visible", func(t *testing.T) {
		origFS := ActiveFullscreenMode
		ActiveFullscreenMode = false
		t.Cleanup(func() { ActiveFullscreenMode = origFS })

		out := stripANSI(RenderTable("NAME", items, 0, 140, 20, false, "", ""))
		for _, n := range names {
			assert.Contains(t, out, n,
				"node name %q must appear in full at 140 cols", n)
		}
	})

	t.Run("middle column 100 columns: longest name still visible", func(t *testing.T) {
		origFS := ActiveFullscreenMode
		ActiveFullscreenMode = false
		t.Cleanup(func() { ActiveFullscreenMode = origFS })

		out := stripANSI(RenderTable("NAME", items, 0, 100, 20, false, "", ""))
		for _, n := range names {
			assert.Contains(t, out, n,
				"node name %q must appear in full at 100 cols", n)
		}
	})

	// Realistic middle-column widths: at 51% of total, a 200-char terminal
	// gives the middle column ~97 chars; a 160-char terminal gives ~80;
	// a 120-char terminal gives ~58. The middle column is where Nodes
	// actually render in the user's three-column layout.
	t.Run("middle column at 200-col terminal (~97 wide)", func(t *testing.T) {
		origFS := ActiveFullscreenMode
		ActiveFullscreenMode = false
		t.Cleanup(func() { ActiveFullscreenMode = origFS })

		out := stripANSI(RenderTable("NAME", items, 0, 97, 20, false, "", ""))
		for _, n := range names {
			assert.Contains(t, out, n,
				"node name %q must appear in full at 97 cols (middle col of 200-wide terminal)", n)
		}
	})

	t.Run("middle column at 160-col terminal (~80 wide): names up to ~50 chars fit", func(t *testing.T) {
		origFS := ActiveFullscreenMode
		ActiveFullscreenMode = false
		t.Cleanup(func() { ActiveFullscreenMode = origFS })

		// Drop the 52-char outlier — at 80 cols the budget genuinely
		// can't accommodate it once builtins (~19) and one extras
		// column (~22) are reserved. Asserting the 38- and 40-char
		// names still fit captures the real complaint: moderate
		// names should not be truncated.
		shorter := []model.Item{items[0], items[2]} // 40-char + 38-char
		out := stripANSI(RenderTable("NAME", shorter, 0, 80, 20, false, "", ""))
		assert.Contains(t, out, "ip-10-0-1-100.eu-west-1.compute.internal",
			"40-char node name must appear in full at 80 cols")
		assert.Contains(t, out, "gke-cluster-default-pool-1a2b3c4d-abcd",
			"38-char node name must appear in full at 80 cols")
	})
}

// Regression: long Pod statuses (PodInitializing, ContainerCreating,
// Succeeded, ...) used to burn enough STATUS-column width on narrow
// layouts to force NAME / NAMESPACE truncation. Now the column shrinks
// to its abbreviated cap when the budget would otherwise truncate names,
// and the renderer swaps to the abbreviation for affected rows.
func TestRenderTable_PodStatusAbbreviatesUnderWidthPressure(t *testing.T) {
	origMS := ActiveMiddleScroll
	ActiveMiddleScroll = -1
	t.Cleanup(func() { ActiveMiddleScroll = origMS })

	origQuery := ActiveHighlightQuery
	ActiveHighlightQuery = ""
	t.Cleanup(func() { ActiveHighlightQuery = origQuery })

	origSel := ActiveSelectedItems
	ActiveSelectedItems = nil
	t.Cleanup(func() { ActiveSelectedItems = origSel })

	origLayout := ActiveTableLayout
	ActiveTableLayout = nil
	t.Cleanup(func() { ActiveTableLayout = origLayout })

	// Realistic pod-list snapshot mirroring the user's report.
	items := []model.Item{
		{Name: "magento-cron-29622472-abcde", Namespace: "m2communityteam-lkajzltdo", Kind: "Pod", Status: "PodInitializing", Ready: "0/1", Restarts: "0", Age: "45s"},
		{Name: "magento-cron-29622472-fghij", Namespace: "m2communityteam-lkajzltdo", Kind: "Pod", Status: "Succeeded", Ready: "0/1", Restarts: "0", Age: "1m"},
		{Name: "magento-cron-29622473-klmno", Namespace: "m2commerceteam-zxywvutsr", Kind: "Pod", Status: "PodInitializing", Ready: "0/1", Restarts: "0", Age: "7s"},
		{Name: "magento-pwa-69dc46b76-pqrst", Namespace: "good360uat-12345", Kind: "Pod", Status: "Running", Ready: "1/1", Restarts: "0", Age: "12d"},
	}

	t.Run("narrow layout abbreviates long statuses", func(t *testing.T) {
		// 65-col middle column reproduces the user's report: the longest
		// pod name (~30 chars) won't fit alongside a 20-char namespace
		// header, READY, RS, STATUS=PodInitializing, AGE, and gutter.
		ActiveTableLayout = nil
		out := stripANSI(RenderTable("NAME", items, 0, 65, 20, false, "", ""))
		assert.Contains(t, out, "Init",
			"PodInitializing must abbreviate to Init when layout is too narrow")
		assert.Contains(t, out, "Done",
			"Succeeded must abbreviate to Done when layout is too narrow")
		assert.NotContains(t, out, "PodInitializing",
			"full PodInitializing must NOT appear once the layout has shrunk STATUS")
	})

	t.Run("wide layout keeps full status labels", func(t *testing.T) {
		ActiveTableLayout = nil
		out := stripANSI(RenderTable("NAME", items, 0, 200, 20, false, "", ""))
		assert.Contains(t, out, "PodInitializing",
			"full PodInitializing must remain at 200 cols where there's no width pressure")
		assert.Contains(t, out, "Succeeded",
			"full Succeeded must remain at 200 cols")
	})
}
