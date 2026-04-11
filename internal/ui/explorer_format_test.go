package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// --- formatTableRowOrdered ---

// buildOrder turns a bool-per-column selection into the ordered slice used
// by the new rendering path, matching the canonical default order.
func buildOrder(hasNs, hasReady, hasRestarts, hasStatus, hasAge bool) []string {
	var order []string
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
				tt.nameW, tt.nsW, tt.readyW, tt.restartsW, tt.statusW, 0,
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
			10, 0, 0, 0, 0, 0, nil, nil, nil)
		assert.Equal(t, 10, len(result), "result length should match nameW")
	})

	t.Run("namespace is padded when present", func(t *testing.T) {
		result := formatTableRowOrdered("pod", "ns", "", "", "", "",
			10, 8, 0, 0, 0, 0, []string{"Namespace"}, nil, nil)
		// Total = nameW + nsW = 10 + 8 = 18. Note: in the ordered path Name
		// comes first, then Namespace.
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
			15, 0, 0, 0, 10, 0,
			[]string{"Status"}, nil, nil,
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
			15, 12, 0, 0, 0, 0,
			[]string{"Namespace"}, nil, nil,
		)
		assert.Contains(t, result, "~ ", "truncated name should have space before namespace column")
		assert.Contains(t, result, "prod")
	})

	t.Run("non-truncated columns still padded correctly", func(t *testing.T) {
		result := formatTableRowOrdered(
			"short", "ns", "", "", "OK", "",
			15, 10, 0, 0, 10, 0,
			[]string{"Namespace", "Status"}, nil, nil,
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
