package ui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// --- formatTableRowOrdered ---

// buildOrder turns a bool-per-column selection into the ordered slice used
// by the new rendering path, matching the canonical default order. "Name"
// leads the order since the rows under test always render a name cell.
func buildOrder(hasNs, hasReady, hasRestarts, hasStatus, hasAge bool) []string {
	order := []string{"Name"}
	if hasNs {
		order = append(order, "Namespace")
	}
	if hasReady {
		order = append(order, "Ready")
	}
	if hasRestarts {
		order = append(order, "Restarts")
	}
	if hasStatus {
		order = append(order, "Status")
	}
	if hasAge {
		order = append(order, "Age")
	}
	return order
}

func TestFormatTableRowOrdered(t *testing.T) {
	tests := []struct {
		name         string
		itemName     string
		ns           string
		ready        string
		restarts     string
		status       string
		nameW        int
		nsW          int
		readyW       int
		restartsW    int
		statusW      int
		hasNs        bool
		hasReady     bool
		hasRestarts  bool
		hasStatus    bool
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:         "all columns shown",
			itemName:     "nginx",
			ns:           "default",
			ready:        "1/1",
			restarts:     "0",
			status:       "Running",
			nameW:        15,
			nsW:          12,
			readyW:       6,
			restartsW:    5,
			statusW:      10,
			hasNs:        true,
			hasReady:     true,
			hasRestarts:  true,
			hasStatus:    true,
			wantContains: []string{"nginx", "default", "1/1", "0", "Running"},
		},
		{
			name:         "name only no optional columns",
			itemName:     "my-pod",
			ns:           "",
			ready:        "",
			restarts:     "",
			status:       "",
			nameW:        20,
			nsW:          0,
			readyW:       0,
			restartsW:    0,
			statusW:      0,
			hasNs:        false,
			hasReady:     false,
			hasRestarts:  false,
			hasStatus:    false,
			wantContains: []string{"my-pod"},
			wantAbsent:   []string{"Running"},
		},
		{
			name:         "namespace and status only",
			itemName:     "pod-1",
			ns:           "prod",
			ready:        "",
			restarts:     "",
			status:       "Pending",
			nameW:        15,
			nsW:          10,
			readyW:       0,
			restartsW:    0,
			statusW:      10,
			hasNs:        true,
			hasReady:     false,
			hasRestarts:  false,
			hasStatus:    true,
			wantContains: []string{"pod-1", "prod", "Pending"},
		},
		{
			name:         "name truncated to nameW with gap",
			itemName:     "very-long-pod-name-that-exceeds-width",
			ns:           "",
			nameW:        10,
			nsW:          0,
			readyW:       0,
			restartsW:    0,
			statusW:      0,
			hasNs:        false,
			hasReady:     false,
			hasRestarts:  false,
			hasStatus:    false,
			wantContains: []string{"very-lon~"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := buildOrder(tt.hasNs, tt.hasReady, tt.hasRestarts, tt.hasStatus, false)
			result := formatTableRowOrdered(tt.itemName, tt.ns, tt.ready, tt.restarts, tt.status, "",
				tt.nameW, 0, tt.nsW, tt.readyW, tt.restartsW, tt.statusW, 0,
				order, nil, nil)
			for _, sub := range tt.wantContains {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, result, absent, "result should not contain %q", absent)
			}
		})
	}
}

// --- formatTableRowOrdered padding ---

func TestFormatTableRowOrdered_Padding(t *testing.T) {
	t.Run("name is padded to nameW", func(t *testing.T) {
		result := formatTableRowOrdered("hi", "", "", "", "", "",
			10, 0, 0, 0, 0, 0, 0, []string{"Name"}, nil, nil)
		assert.Equal(t, 10, len(result), "result length should match nameW")
	})

	t.Run("namespace is padded when present", func(t *testing.T) {
		result := formatTableRowOrdered("pod", "ns", "", "", "", "",
			10, 0, 8, 0, 0, 0, 0, []string{"Name", "Namespace"}, nil, nil)
		// Total = nameW + nsW = 10 + 8 = 18. Name comes first, then Namespace.
		assert.Equal(t, 18, len(result))
	})
}

// --- resourceColumnStyle ---

func TestResourceColumnStyle(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  string
	}{
		{name: "CPU column returns DimStyle", key: "CPU", val: "100m"},
		{name: "MEM column returns DimStyle", key: "MEM", val: "256Mi"},
		{name: "CPU/R percentage", key: "CPU/R", val: "45%"},
		{name: "MEM/L percentage", key: "MEM/L", val: "90%"},
		{name: "CPU% percentage", key: "CPU%", val: "50%"},
		{name: "MEM% percentage", key: "MEM%", val: "80%"},
		{name: "CPU Req returns secondary style", key: "CPU Req", val: "100m"},
		{name: "Last Sync uses StatusStyle", key: "Last Sync", val: "Synced"},
		{name: "Health uses StatusStyle", key: "Health", val: "Healthy"},
		{name: "default key returns DimStyle", key: "Node", val: "node-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := resourceColumnStyle(tt.key, tt.val)
			// Verify the style can render without panicking.
			rendered := style.Render("test")
			assert.NotEmpty(t, rendered)
		})
	}
}

// TestResourceColumnStyle_ValueBased covers issue #488: printer/extra columns
// with low-cardinality values (State: Active/Inactive, Established: True/False)
// are colored semantically instead of always rendering dim.
func TestResourceColumnStyle_ValueBased(t *testing.T) {
	fgKey := func(s lipgloss.Style) string {
		fg := s.GetForeground()
		r, g, b, a := fg.RGBA()
		return fmt.Sprintf("%d:%d:%d:%d", r, g, b, a)
	}
	red := fgKey(StatusFailed)
	green := fgKey(StatusRunning)
	blue := fgKey(StatusProgressing)
	dim := fgKey(DimStyle)

	tests := []struct {
		key  string
		val  string
		want string
		desc string
	}{
		// Boolean-ish values follow the column name's polarity, like conditions.
		{"Established", "True", green, "ready-polarity column, True = healthy"},
		{"Established", "False", red, "ready-polarity column, False = problem"},
		{"established", "true", green, "case-insensitive boolean"},
		{"ESTABLISHED", "FALSE", red, "case-insensitive boolean"},
		{"Failed", "True", red, "error-polarity column, True = problem"},
		{"Failed", "False", green, "error-polarity column, False = healthy"},
		{"Suspend", "True", blue, "info-polarity column, True = active"},
		{"Suspend", "False", dim, "info-polarity column, False = neutral"},
		{"Ready", "Unknown", dim, "unknown is neutral"},

		// Word values reuse the status severity classification.
		{"State", "Active", green, "known-good status word"},
		{"State", "Established", green, "established = healthy"},
		{"State", "Failed", red, "known-bad status word"},
		{"State", "Pending", blue, "progressing status word"},
		{"State", "Inactive", dim, "unclassified word stays dim"},

		// Arbitrary values stay dim.
		{"Node", "node-1", dim, "arbitrary text stays dim"},
		{"Class", "default", dim, "literal default is not a status"},
		{"Replicas", "3", dim, "numbers stay dim"},
		{"Host", "", dim, "empty stays dim"},
	}
	for _, tt := range tests {
		t.Run(tt.key+"/"+tt.val, func(t *testing.T) {
			got := fgKey(resourceColumnStyle(tt.key, tt.val))
			assert.Equal(t, tt.want, got, "%s=%q: %s", tt.key, tt.val, tt.desc)
		})
	}
}

// --- pctStyle ---

func TestPctStyle(t *testing.T) {
	tests := []struct {
		name string
		val  string
		desc string
	}{
		{name: "n/a returns dim", val: "n/a", desc: "dim"},
		{name: "empty returns dim", val: "", desc: "dim"},
		{name: "low percentage", val: "30%", desc: "dim"},
		{name: "mid percentage", val: "50%", desc: "dim"},
		{name: "75 percent threshold", val: "75%", desc: "orange"},
		{name: "high percentage", val: "85%", desc: "orange"},
		{name: "90 percent threshold", val: "90%", desc: "error"},
		{name: "critical percentage", val: "99%", desc: "error"},
		{name: "over 100 percent", val: "150%", desc: "error"},
		{name: "invalid string returns dim", val: "abc%", desc: "dim"},
		{name: "no percent sign", val: "42", desc: "dim"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := pctStyle(tt.val)
			rendered := style.Render("test")
			assert.NotEmpty(t, rendered, "pctStyle(%q) should render", tt.val)
		})
	}

	t.Run("90 percent uses bold error style", func(t *testing.T) {
		s := pctStyle("90%")
		assert.True(t, s.GetBold(), "90%% should be bold")
	})

	t.Run("75 percent uses bold orange style", func(t *testing.T) {
		s := pctStyle("75%")
		assert.True(t, s.GetBold(), "75%% should be bold")
	})

	t.Run("low percent is not bold", func(t *testing.T) {
		s := pctStyle("30%")
		assert.False(t, s.GetBold(), "30%% should not be bold")
	})
}

// --- Truncated column spacing ---

func TestTruncatedColumnSpacing(t *testing.T) {
	t.Run("truncated name has space before next column", func(t *testing.T) {
		// Name that exceeds nameW, followed by a status column.
		// After truncation, there must be at least 1 space before the status text.
		result := formatTableRowOrdered(
			"very-long-pod-name-that-definitely-exceeds", "", "", "", "Running", "",
			15, 0, 0, 0, 0, 10, 0,
			[]string{"Name", "Status"}, nil, nil,
		)
		// The name is truncated to 15 chars. The status "Running" should NOT immediately
		// follow the truncated name — there must be at least 1 space gap.
		assert.Contains(t, result, "~ ", "truncated name should have space before next column")
		assert.Contains(t, result, "Running")
	})

	t.Run("truncated name followed by namespace has space", func(t *testing.T) {
		// In the ordered path Name always comes first, so test that a
		// truncated name is followed by a space before the Namespace column.
		result := formatTableRowOrdered(
			"extremely-long-pod-name-here", "prod", "", "", "", "",
			15, 0, 12, 0, 0, 0, 0,
			[]string{"Name", "Namespace"}, nil, nil,
		)
		assert.Contains(t, result, "~ ", "truncated name should have space before namespace column")
		assert.Contains(t, result, "prod")
	})

	t.Run("non-truncated columns still padded correctly", func(t *testing.T) {
		result := formatTableRowOrdered(
			"short", "ns", "", "", "OK", "",
			15, 0, 10, 0, 0, 10, 0,
			[]string{"Name", "Namespace", "Status"}, nil, nil,
		)
		// Short values should be padded as before: nameW + nsW + statusW = 15 + 10 + 10 = 35.
		assert.Equal(t, 35, len(result), "total width should be nameW+nsW+statusW")
	})
}

// --- RenderTabBar ---

func TestRenderTabBar(t *testing.T) {
	tests := []struct {
		name       string
		labels     []string
		activeTab  int
		width      int
		wantSubstr []string
	}{
		{
			name:       "single tab",
			labels:     []string{"Pods"},
			activeTab:  0,
			width:      80,
			wantSubstr: []string{"1 Pods"},
		},
		{
			name:       "multiple tabs with active highlighted",
			labels:     []string{"Pods", "Deployments", "Services"},
			activeTab:  1,
			width:      120,
			wantSubstr: []string{"1 Pods", "2 Deployments", "3 Services"},
		},
		{
			name:       "active tab index 0",
			labels:     []string{"Tab1", "Tab2"},
			activeTab:  0,
			width:      80,
			wantSubstr: []string{"1 Tab1", "2 Tab2"},
		},
		{
			name:       "narrow width shows arrows for overflow",
			labels:     []string{"AAAA", "BBBB", "CCCC", "DDDD", "EEEE", "FFFF", "GGGG", "HHHH"},
			activeTab:  4,
			width:      50,
			wantSubstr: []string{"5 EEEE"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderTabBar(tt.labels, tt.activeTab, tt.width)
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
		})
	}

	t.Run("tab bar contains separator", func(t *testing.T) {
		result := RenderTabBar([]string{"A", "B"}, 0, 80)
		// The separator is a styled pipe character.
		assert.True(t, strings.Contains(result, "│") || strings.Contains(result, "|"),
			"tab bar should contain a separator")
	})

	t.Run("overflow shows left arrow indicator", func(t *testing.T) {
		labels := make([]string, 20)
		for i := range labels {
			labels[i] = "Tab"
		}
		result := RenderTabBar(labels, 10, 60)
		// When tabs overflow to the left, a left-arrow indicator should appear.
		assert.Contains(t, result, "◂")
	})

	t.Run("overflow shows right arrow indicator", func(t *testing.T) {
		labels := make([]string, 20)
		for i := range labels {
			labels[i] = "Tab"
		}
		result := RenderTabBar(labels, 0, 60)
		assert.Contains(t, result, "▸")
	})

	t.Run("9 long-labeled tabs with middle active stays on one line", func(t *testing.T) {
		// Regression: with 9 tabs and the active tab in the middle, the
		// windowing budget did not reserve space for the leading and
		// trailing arrow indicators (◂ ... ▸). The tab bar then overflowed
		// `width` and lipgloss wrapped it to a second line, which pushed
		// the title bar off-screen.
		labels := []string{
			"prod/Deployments/web-server",
			"prod/Pods/api-7d8c-abc",
			"prod/StatefulSets/db",
			"prod/Services/frontend",
			"prod/ConfigMaps/app-config",
			"prod/Secrets/credentials",
			"prod/Ingresses/api",
			"prod/Jobs/migrate-db",
			"prod/CronJobs/cleanup",
		}
		// Sweep across realistic terminal widths and every active-tab
		// position so we catch any combination that breaks the windowing
		// budget (off-by-one for indicators, padding mismatch, etc.).
		for _, width := range []int{60, 70, 80, 90, 100, 120, 150, 200} {
			for activeTab := range labels {
				result := RenderTabBar(labels, activeTab, width)
				height := lipgloss.Height(result)
				assert.Equal(t, 1, height,
					"tab bar must render as a single line (width=%d, active=%d, got %d lines)\n%s",
					width, activeTab, height, result)
				rendered := lipgloss.Width(result)
				assert.LessOrEqual(t, rendered, width,
					"tab bar must not exceed configured width (width=%d, active=%d, got %d)",
					width, activeTab, rendered)
			}
		}
	})
}

// TestColumnHeaderLabelSecurityAliases verifies the security-finding column
// keys render as space-separated, human-readable headers instead of
// "RESOURCEKIND" / "FINDINGCOUNT".
func TestColumnHeaderLabelSecurityAliases(t *testing.T) {
	assert.Equal(t, "RESOURCE KIND", ColumnHeaderLabel("ResourceKind"))
	assert.Equal(t, "FINDINGS", ColumnHeaderLabel("FindingCount"))
}

// --- Row builder byte-parity regression ---
//
// These tests pin exact output for the row builders so the wantContains-style
// tests above can't silently let column-cell padding/truncation/styling drift
// past us. They're the safety net for the builtin-column-registry refactor:
// if any registry entry diverges from its old switch-arm semantics, the
// expected byte string here fails immediately.

func TestFormatTableRowOrdered_AllColumnsExactBytes(t *testing.T) {
	prev := ActiveHighlightQuery
	ActiveHighlightQuery = ""
	t.Cleanup(func() { ActiveHighlightQuery = prev })

	item := model.Item{
		Name:        "pod-1",
		Namespace:   "default",
		Status:      "Running",
		Ready:       "1/1",
		Restarts:    "0",
		Age:         "5d",
		ClusterName: "prod",
	}
	order := []string{"Name", "Context", "Namespace", "Ready", "Restarts", "Status", "Age"}

	got := formatTableRowOrdered(
		item.Name, item.Namespace, item.Ready, item.Restarts, item.Status, item.Age,
		10, 8, 10, 6, 4, 10, 4,
		order, nil, &item,
	)

	// name(10) + context(8) + ns(10) + ready(6) + restarts(4) + status(10) + age(4) = 52
	want := "pod-1     prod    default   1/1   0   Running   5d  "
	assert.Equal(t, want, got)
	assert.Equal(t, 52, len(got), "total width must match sum of column widths")
}

func TestFormatTableRowOrdered_ContextZeroWidthFallsThrough(t *testing.T) {
	prev := ActiveHighlightQuery
	ActiveHighlightQuery = ""
	t.Cleanup(func() { ActiveHighlightQuery = prev })

	item := model.Item{Name: "pod-1", Namespace: "default", ClusterName: "should-not-render"}
	order := []string{"Name", "Context", "Namespace"}

	got := formatTableRowOrdered(
		"pod-1", "default", "", "", "", "",
		10, 0, 10, 0, 0, 0, 0,
		order, nil, &item,
	)

	want := "pod-1     default   "
	assert.Equal(t, want, got)
	assert.NotContains(t, got, "should-not-render", "zero-width Context must not render ClusterName")
}

func TestFormatTableRowStyledOrdered_VisibleWidthAndContent(t *testing.T) {
	prev := ActiveHighlightQuery
	ActiveHighlightQuery = ""
	t.Cleanup(func() { ActiveHighlightQuery = prev })

	item := model.Item{
		Name:        "pod-1",
		Namespace:   "default",
		Status:      "Running",
		Ready:       "1/1",
		Restarts:    "0",
		Age:         "5d",
		ClusterName: "prod",
	}
	order := []string{"Name", "Context", "Namespace", "Ready", "Restarts", "Status", "Age"}

	got := formatTableRowStyledOrdered(item,
		10, 8, 10, 6, 4, 10, 4,
		order, nil, false, nil)

	// Visible width pins padding; ANSI-stripped content pins per-cell text.
	// Together these catch any styling/padding drift in the registry's
	// styled render functions without overcommitting to lipgloss escape
	// byte sequences.
	assert.Equal(t, 52, lipgloss.Width(got),
		"visible width must equal sum of column widths")
	assert.Equal(t,
		"pod-1     prod    default   1/1   0   Running   5d  ",
		ansi.Strip(got),
		"ANSI-stripped styled row must match plain row layout")
}

// --- Sort-column header highlight (issue: highlight the active sort arrow) ---

// TestRenderStyledHeader_ActiveColumnAccented verifies the header cell for the
// active sort column (label + sort arrow) is rendered with SortActiveHeaderStyle
// while every other header cell stays dim, so the sorted column stands out.
func TestRenderStyledHeader_ActiveColumnAccented(t *testing.T) {
	// Force color mode + a color profile + populated theme so the dim and
	// accent styles emit distinguishable ANSI; restore the mutable globals
	// afterwards. ConfigNoColor is pinned because a prior test may have left
	// it set, which would route ApplyTheme through the no-color path and drop
	// the profile back to Ascii.
	origNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigNoColor = origNoColor
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	ApplyTheme(DefaultTheme())
	// ApplyTheme restores the saved (Ascii, under test) profile, so force the
	// profile last; styles carry color specs that resolve at render time.

	prev := ActiveSortColumnName
	t.Cleanup(func() { ActiveSortColumnName = prev })
	ActiveSortColumnName = "QoS"

	segs := []headerSegment{
		{text: "NAME      ", colName: "Name"},
		{text: "QOS↑    ", colName: "QoS"},
		{text: "AGE  ", colName: "Age"},
	}
	out := renderStyledHeader(segs, 200)

	assert.Contains(t, out, SortActiveHeaderStyle.Render("QOS↑    "),
		"active sort column header must use the accent style")
	assert.Contains(t, out, DimStyle.Bold(true).Render("NAME      "),
		"inactive header cells must stay dim+bold")
	assert.NotContains(t, out, DimStyle.Bold(true).Render("QOS↑    "),
		"active sort column must not be rendered dim")
	// Visible content is preserved verbatim (styling adds no visible width).
	assert.Equal(t, "NAME      QOS↑    AGE  ", ansi.Strip(out))
}

// TestHeaderWithIndicator_NoSpaceNoTruncate verifies the sort arrow sits in the
// column's trailing spacing with no space before it, and does not eat the last
// character of a tight column label (regression: "RS" sorted rendered "R ↑").
func TestHeaderWithIndicator_NoSpaceNoTruncate(t *testing.T) {
	prevCol, prevAsc := ActiveSortColumnName, ActiveSortAscending
	t.Cleanup(func() { ActiveSortColumnName, ActiveSortAscending = prevCol, prevAsc })
	ActiveSortAscending = true

	// RS column: width 3 = "RS" (2) + 1 trailing spacing. The arrow uses the
	// spacing slot: both letters kept, no leading space.
	ActiveSortColumnName = "Restarts"
	assert.Equal(t, "RS↑", headerWithIndicator("RS", "Restarts", 3))

	// Wide column: arrow directly follows the label (no space), padded to width.
	ActiveSortColumnName = "Status"
	assert.Equal(t, "STATUS↑   ", headerWithIndicator("STATUS", "Status", 10))

	// Descending shows the down arrow.
	ActiveSortAscending = false
	assert.Equal(t, "STATUS↓   ", headerWithIndicator("STATUS", "Status", 10))

	// Not the sort column: plain padded label, no arrow.
	ActiveSortColumnName = "Status"
	assert.Equal(t, "READY ", headerWithIndicator("READY", "Ready", 6))
}

// TestRenderStyledHeader_NoActiveSortAllDim verifies that with no active sort
// column every header cell is dim — the pre-highlight behaviour is unchanged.
func TestRenderStyledHeader_NoActiveSortAllDim(t *testing.T) {
	prev := ActiveSortColumnName
	t.Cleanup(func() { ActiveSortColumnName = prev })
	ActiveSortColumnName = ""

	segs := []headerSegment{
		{text: "NAME ", colName: "Name"},
		{text: "AGE ", colName: "Age"},
	}
	out := renderStyledHeader(segs, 200)

	want := DimStyle.Bold(true).Render("NAME ") + DimStyle.Bold(true).Render("AGE ")
	assert.Equal(t, want, out)
}

// TestRenderStyledHeader_TruncatesToWidth verifies the header is capped at the
// given width (matching the prior Truncate(hdr, width) behaviour).
func TestRenderStyledHeader_TruncatesToWidth(t *testing.T) {
	prev := ActiveSortColumnName
	t.Cleanup(func() { ActiveSortColumnName = prev })
	ActiveSortColumnName = ""

	segs := []headerSegment{
		{text: "NAMECOLUMN", colName: "Name"},
		{text: "AGECOLUMN", colName: "Age"},
	}
	out := renderStyledHeader(segs, 6)
	assert.Equal(t, 6, lipgloss.Width(ansi.Strip(out)+""))
	assert.LessOrEqual(t, lipgloss.Width(out), 6)
}

// --- sanitization: TASK-880 ---

// formatHostilePayloads are cluster-controlled strings carrying a terminal attack:
// a bidi override reorders rendered text, a raw CSI sequence can rewrite the
// screen, and OSC-52 writes the operator's clipboard.
var formatHostilePayloads = map[string]string{
	"bidi override": "ab\u202ecd",
	"raw CSI":       "ab\x1b[2Jcd",
	"OSC-52":        "ab\x1b]52;c;aGF4\x07cd",
}

func TestPlainNameCellWithBadge_SanitizesName(t *testing.T) {
	for name, payload := range formatHostilePayloads {
		t.Run(name, func(t *testing.T) {
			out := plainNameCellWithBadge(payload, nil, 40)
			assert.NotContains(t, out, "\u202e")
			assert.NotContains(t, out, "\x1b[2J")
			assert.NotContains(t, out, "\x1b]52")
			assert.Contains(t, out, "ab")
			assert.Contains(t, out, "cd")
		})
	}
	assert.Contains(t, plainNameCellWithBadge("nginx", nil, 40), "nginx")
}

func TestStyledNameCell_SanitizesName(t *testing.T) {
	for name, payload := range formatHostilePayloads {
		t.Run(name, func(t *testing.T) {
			item := model.Item{Name: payload}
			out := styledNameCell(item, 40, nil)
			assert.NotContains(t, out, "\u202e")
			assert.NotContains(t, out, "\x1b[2J")
			assert.NotContains(t, out, "\x1b]52")
		})
	}
	assert.Contains(t, stripANSI(styledNameCell(model.Item{Name: "nginx"}, 40, nil)), "nginx")
}

func TestPlainExtraCell_SanitizesValue(t *testing.T) {
	ec := extraColumn{key: "IP", width: 20}
	for name, payload := range formatHostilePayloads {
		t.Run(name, func(t *testing.T) {
			item := &model.Item{Columns: []model.KeyValue{{Key: "IP", Value: payload}}}
			out := plainExtraCell(ec, item)
			assert.NotContains(t, out, "\u202e")
			assert.NotContains(t, out, "\x1b[2J")
			assert.NotContains(t, out, "\x1b]52")
		})
	}
	item := &model.Item{Columns: []model.KeyValue{{Key: "IP", Value: "10.0.0.1"}}}
	assert.Contains(t, plainExtraCell(ec, item), "10.0.0.1")
}

func TestStyledExtraCell_SanitizesValue(t *testing.T) {
	ec := extraColumn{key: "IP", width: 20}
	for name, payload := range formatHostilePayloads {
		t.Run(name, func(t *testing.T) {
			item := &model.Item{Columns: []model.KeyValue{{Key: "IP", Value: payload}}}
			out := styledExtraCell(ec, item)
			assert.NotContains(t, out, "\u202e")
			assert.NotContains(t, out, "\x1b[2J")
			assert.NotContains(t, out, "\x1b]52")
		})
	}
	item := &model.Item{Columns: []model.KeyValue{{Key: "IP", Value: "10.0.0.1"}}}
	assert.Contains(t, stripANSI(styledExtraCell(ec, item)), "10.0.0.1")
}

func TestRestartsCells_SanitizeValue(t *testing.T) {
	for name, payload := range formatHostilePayloads {
		t.Run("styled/"+name, func(t *testing.T) {
			item := model.Item{Restarts: "0" + payload}
			out := styledRestartsCell(item, 20, false)
			assert.NotContains(t, out, "\u202e")
			assert.NotContains(t, out, "\x1b[2J")
			assert.NotContains(t, out, "\x1b]52")
		})
		t.Run("plain/"+name, func(t *testing.T) {
			item := model.Item{Restarts: "0" + payload}
			out := plainRestartsCell(item, false)
			assert.NotContains(t, out, "\u202e")
			assert.NotContains(t, out, "\x1b[2J")
			assert.NotContains(t, out, "\x1b]52")
		})
	}
	assert.Contains(t, stripANSI(styledRestartsCell(model.Item{Restarts: "3"}, 20, false)), "3")
}

// A sparkline sits in the same prefix position as the trend arrow. Without
// stripping it, every CPU and MEM sort in sparkline mode compares glyphs
// instead of numbers.
func TestParseResourceValueOK_StripsSparklinePrefix(t *testing.T) {
	tests := []struct {
		name  string
		val   string
		isCPU bool
		want  int64
	}{
		{"cpu millicores", "▁▂▃▅▇ 240m", true, 240},
		{"cpu cores", "▁▂▃▅▇ 1.5", true, 1500},
		{"memory", "▃▃▄▄▄ 512Mi", false, 512 * 1024 * 1024},
		{"sparkline with gap", "▁▂ ▅▇ 240m", true, 240},
		{"leading gap", " ▁▂▅▇ 240m", true, 240},
		{"single glyph", "█ 80m", true, 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseResourceValueOK(tt.val, tt.isCPU)
			require.True(t, ok, "value %q must parse", tt.val)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseResourceValueOK_SparklineOnlyIsNotAValue(t *testing.T) {
	_, ok := ParseResourceValueOK("▁▂▃▅▇", true)
	assert.False(t, ok, "glyphs with no number must sort as missing, not as 0")
}

func TestParseResourceValueOK_PlainValuesUnaffected(t *testing.T) {
	v, ok := ParseResourceValueOK("240m", true)
	require.True(t, ok)
	assert.Equal(t, int64(240), v)

	v, ok = ParseResourceValueOK("↑ 240m", true)
	require.True(t, ok)
	assert.Equal(t, int64(240), v)

	_, ok = ParseResourceValueOK("n/a", true)
	assert.False(t, ok)
}

func TestAbbreviateStatusForWidthCordoned(t *testing.T) {
	// The status column caps at 20 columns, so the kubectl-length string only
	// fits once a wide layout gives it room.
	assert.Equal(t, "Ready,Cordoned", AbbreviateStatusForWidth("Ready,SchedulingDisabled", 19))
	assert.Equal(t, "NotReady,Cordoned", AbbreviateStatusForWidth("NotReady,SchedulingDisabled", 19))
	assert.Equal(t, "Cordoned", AbbreviateStatusForWidth("SchedulingDisabled", 10))
	assert.Equal(t, "Ready,SchedulingDisabled", AbbreviateStatusForWidth("Ready,SchedulingDisabled", 30))
}
