package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- RenderNamespaceOverlay ---

func TestRenderNamespaceOverlay(t *testing.T) {
	t.Run("nil items shows loading", func(t *testing.T) {
		result := RenderNamespaceOverlay(nil, "", 0, "default", false, nil, false)
		assert.Contains(t, result, "Loading namespaces")
	})

	t.Run("empty items shows no matching", func(t *testing.T) {
		result := RenderNamespaceOverlay([]model.Item{}, "", 0, "default", false, nil, false)
		assert.Contains(t, result, "No matching namespaces")
	})

	t.Run("renders namespace items", func(t *testing.T) {
		items := []model.Item{
			{Name: "default"},
			{Name: "kube-system"},
			{Name: "production"},
		}
		result := RenderNamespaceOverlay(items, "", 0, "default", false, nil, false)
		assert.Contains(t, result, "default")
		assert.Contains(t, result, "kube-system")
		assert.Contains(t, result, "production")
	})

	t.Run("selected namespaces show checkmark", func(t *testing.T) {
		items := []model.Item{
			{Name: "default"},
			{Name: "staging"},
		}
		selected := map[string]bool{"staging": true}
		result := RenderNamespaceOverlay(items, "", 0, "default", false, selected, false)
		assert.Contains(t, result, "\u2713")
	})

	t.Run("all namespaces checkmark", func(t *testing.T) {
		items := []model.Item{
			{Name: "All Namespaces", Status: "all"},
		}
		result := RenderNamespaceOverlay(items, "", 0, "", true, nil, false)
		assert.Contains(t, result, "\u2713")
	})

	t.Run("filter mode shows cursor", func(t *testing.T) {
		items := []model.Item{{Name: "default"}}
		result := RenderNamespaceOverlay(items, "def", 0, "", false, nil, true)
		assert.Contains(t, result, "def")
		assert.Contains(t, result, "\u2588") // block cursor
	})

	t.Run("shows filter hint when not in filter mode", func(t *testing.T) {
		items := []model.Item{{Name: "default"}}
		result := RenderNamespaceOverlay(items, "", 0, "", false, nil, false)
		assert.Contains(t, result, "/ to filter")
	})

	t.Run("footer hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		items := []model.Item{{Name: "default"}}
		result := RenderNamespaceOverlay(items, "", 0, "", false, nil, false)
		assert.NotContains(t, result, "space: select")
		assert.NotContains(t, result, "enter: apply")
		assert.NotContains(t, result, "esc: close")
	})
}

// --- RenderActionOverlay ---

func TestRenderActionOverlay(t *testing.T) {
	items := []model.Item{
		{Name: "Delete", Extra: "Delete this resource", Status: "d"},
		{Name: "Describe", Extra: "Describe resource", Status: "D"},
	}

	result := RenderActionOverlay(items, 0, 70)
	assert.Contains(t, result, "Actions")
	assert.Contains(t, result, "Delete")
	assert.Contains(t, result, "Describe")
	assert.Contains(t, result, "[d]")
	assert.Contains(t, result, "[D]")
}

// --- RenderConfirmOverlay ---

func TestRenderConfirmOverlay(t *testing.T) {
	result := RenderConfirmOverlay("my-deployment")
	assert.Contains(t, result, "Confirm Delete")
	assert.Contains(t, result, "my-deployment")
}

// --- RenderScaleOverlay ---

func TestRenderScaleOverlay(t *testing.T) {
	t.Run("empty input shows placeholder", func(t *testing.T) {
		result := RenderScaleOverlay("")
		assert.Contains(t, result, "Scale Deployment")
		assert.Contains(t, result, "_")
	})

	t.Run("with input", func(t *testing.T) {
		result := RenderScaleOverlay("5")
		assert.Contains(t, result, "5")
		assert.NotContains(t, result, "_")
	})
}

// --- RenderPortForwardOverlay ---

func TestRenderPortForwardOverlay(t *testing.T) {
	t.Run("empty input shows placeholder", func(t *testing.T) {
		result := RenderPortForwardOverlay("", nil, 0, "my-pod")
		assert.Contains(t, result, "Port Forward")
		assert.Contains(t, result, "local:remote")
	})

	t.Run("with input no ports", func(t *testing.T) {
		result := RenderPortForwardOverlay("9090:80", nil, -1, "my-pod")
		assert.Contains(t, result, "9090:80")
		assert.Contains(t, result, "Port mapping")
	})

	t.Run("with available ports selected shows remote and local", func(t *testing.T) {
		ports := []PortInfo{{Port: "80", Name: "http", Protocol: "TCP"}}
		result := RenderPortForwardOverlay("", ports, 0, "my-svc")
		assert.Contains(t, result, "80")
		assert.Contains(t, result, "Remote port")
		assert.Contains(t, result, "Local port")
		assert.Contains(t, result, "(random)")
	})

	t.Run("with available ports and custom local port", func(t *testing.T) {
		ports := []PortInfo{{Port: "80", Name: "http", Protocol: "TCP"}}
		result := RenderPortForwardOverlay("9090", ports, 0, "my-svc")
		assert.Contains(t, result, "Remote port")
		assert.Contains(t, result, "Local port")
		assert.Contains(t, result, "9090")
		assert.NotContains(t, result, "(same)")
	})

	t.Run("no selection shows port mapping mode", func(t *testing.T) {
		ports := []PortInfo{{Port: "80", Name: "http", Protocol: "TCP"}}
		result := RenderPortForwardOverlay("", ports, -1, "my-svc")
		assert.Contains(t, result, "Port mapping")
		assert.Contains(t, result, "local:remote")
	})

	t.Run("shows resource name", func(t *testing.T) {
		result := RenderPortForwardOverlay("", nil, -1, "my-pod")
		assert.Contains(t, result, "my-pod")
	})

	t.Run("non-TCP protocol shown", func(t *testing.T) {
		ports := []PortInfo{{Port: "53", Name: "dns", Protocol: "UDP"}}
		result := RenderPortForwardOverlay("", ports, 0, "my-svc")
		assert.Contains(t, result, "[UDP]")
	})

	t.Run("hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		ports := []PortInfo{{Port: "80", Name: "http", Protocol: "TCP"}}
		result := RenderPortForwardOverlay("", ports, 0, "my-svc")
		assert.NotContains(t, result, "j/k: select port")
		assert.NotContains(t, result, "enter: forward")
	})
}

// --- RenderContainerSelectOverlay ---

func TestRenderContainerSelectOverlay(t *testing.T) {
	items := []model.Item{
		{Name: "nginx", Status: "Running"},
		{Name: "sidecar", Status: "Waiting"},
	}
	result := RenderContainerSelectOverlay(items, 0)
	assert.Contains(t, result, "Select Container")
	assert.Contains(t, result, "nginx")
	assert.Contains(t, result, "sidecar")
}

// --- RenderPodSelectOverlay ---

func TestRenderPodSelectOverlay(t *testing.T) {
	items := []model.Item{
		{Name: "pod-1", Status: "Running"},
		{Name: "pod-2", Status: "Pending"},
	}
	result := RenderPodSelectOverlay(items, 0, "", false)
	assert.Contains(t, result, "Select Pod")
	assert.Contains(t, result, "pod-1")
	assert.Contains(t, result, "pod-2")
	assert.Contains(t, result, "/ to filter")
}

func TestRenderPodSelectOverlay_FilterActive(t *testing.T) {
	items := []model.Item{
		{Name: "pod-1", Status: "Running"},
		{Name: "pod-2", Status: "Pending"},
	}
	result := RenderPodSelectOverlay(items, 0, "pod", true)
	assert.Contains(t, result, "/ pod")
}

// --- RenderBookmarkOverlay ---

func TestRenderBookmarkOverlay(t *testing.T) {
	t.Run("no bookmarks", func(t *testing.T) {
		result := RenderBookmarkOverlay(nil, "", 0, 0)
		assert.Contains(t, result, "No bookmarks yet")
	})

	t.Run("with bookmarks", func(t *testing.T) {
		bms := []model.Bookmark{
			{Name: "My Pods", Namespace: "default"},
			{Name: "Deployments", Namespace: "staging"},
		}
		result := RenderBookmarkOverlay(bms, "", 0, 0)
		assert.Contains(t, result, "My Pods [default]")
		assert.Contains(t, result, "Deployments [staging]")
	})

	t.Run("filter mode", func(t *testing.T) {
		bms := []model.Bookmark{
			{Name: "My Pods"},
			{Name: "Deployments"},
		}
		result := RenderBookmarkOverlay(bms, "pod", 0, 1)
		assert.Contains(t, result, "filter>")
		assert.Contains(t, result, "pod")
	})

	t.Run("filtered no match", func(t *testing.T) {
		bms := []model.Bookmark{{Name: "Pods"}}
		result := RenderBookmarkOverlay(bms, "zzz", 0, 0)
		assert.Contains(t, result, "No matching bookmarks")
	})

	t.Run("bookmark with namespace shows bracket", func(t *testing.T) {
		bms := []model.Bookmark{{Name: "My Pods", Namespace: "production"}}
		result := RenderBookmarkOverlay(bms, "", 0, 0)
		assert.Contains(t, result, "production")
	})

	t.Run("slot prefix shown for bookmarks with slots", func(t *testing.T) {
		bms := []model.Bookmark{
			{Name: "With Slot A", Slot: "a"},
			{Name: "With Slot B", Slot: "b"},
			{Name: "No Slot"},
		}
		result := RenderBookmarkOverlay(bms, "", 0, 0)
		assert.Contains(t, result, "a")
		assert.Contains(t, result, "With Slot A")
		assert.Contains(t, result, "b")
		assert.Contains(t, result, "With Slot B")
		assert.Contains(t, result, "No Slot")
	})

	t.Run("normal mode shows bookmark content", func(t *testing.T) {
		bms := []model.Bookmark{{Name: "Test"}}
		result := RenderBookmarkOverlay(bms, "", 0, 0)
		assert.Contains(t, result, "Test")
	})

	t.Run("scope indicator shows G for global bookmarks", func(t *testing.T) {
		bms := []model.Bookmark{
			{Name: "Global Mark", Slot: "A", Global: true},
		}
		result := RenderBookmarkOverlay(bms, "", 0, 0)
		assert.Contains(t, result, "[G]", "global bookmark should show [G] scope indicator")
		assert.Contains(t, result, "Global Mark")
	})

	t.Run("scope indicator shows L for local bookmarks", func(t *testing.T) {
		bms := []model.Bookmark{
			{Name: "Local Mark", Slot: "a", Global: false},
		}
		result := RenderBookmarkOverlay(bms, "", 0, 0)
		assert.Contains(t, result, "[L]", "local bookmark should show [L] scope indicator")
		assert.Contains(t, result, "Local Mark")
	})

	t.Run("mixed global and local bookmarks show correct indicators", func(t *testing.T) {
		bms := []model.Bookmark{
			{Name: "Global Pods", Slot: "A", Global: true},
			{Name: "Local Deploys", Slot: "a", Global: false},
			{Name: "Global Svcs", Slot: "B", Global: true},
		}
		result := RenderBookmarkOverlay(bms, "", 0, 0)
		assert.Contains(t, result, "[G]")
		assert.Contains(t, result, "[L]")
		assert.Contains(t, result, "Global Pods")
		assert.Contains(t, result, "Local Deploys")
		assert.Contains(t, result, "Global Svcs")
	})
}

// --- RenderColorschemeOverlay ---

func TestRenderColorschemeOverlay(t *testing.T) {
	entries := GroupedSchemeEntries()

	t.Run("renders with headers", func(t *testing.T) {
		result := RenderColorschemeOverlay(entries, "", 0, false)
		assert.Contains(t, result, "Select Color Scheme")
		assert.Contains(t, result, "Dark Themes")
		// "Light Themes" header may be scrolled off-screen with many schemes,
		// but it should exist in the entries.
		hasLight := false
		for _, e := range entries {
			if e.IsHeader && e.Name == "Light Themes" {
				hasLight = true
				break
			}
		}
		assert.True(t, hasLight, "entries should include Light Themes header")
	})

	t.Run("filter mode shows cursor", func(t *testing.T) {
		result := RenderColorschemeOverlay(entries, "tokyo", 0, true)
		assert.Contains(t, result, "tokyo")
		assert.Contains(t, result, "█")
	})

	t.Run("filter hides headers", func(t *testing.T) {
		result := RenderColorschemeOverlay(entries, "tokyo", 0, false)
		assert.NotContains(t, result, "Dark Themes")
		assert.NotContains(t, result, "Light Themes")
	})

	t.Run("no matching schemes", func(t *testing.T) {
		result := RenderColorschemeOverlay(entries, "nonexistent", 0, false)
		assert.Contains(t, result, "No matching schemes")
	})

	t.Run("filter hint when empty", func(t *testing.T) {
		result := RenderColorschemeOverlay(entries, "", 0, false)
		assert.Contains(t, result, "/ to filter")
	})
}

// --- RenderErrorLogOverlay ---

func TestRenderErrorLogOverlay(t *testing.T) {
	t.Run("empty entries", func(t *testing.T) {
		result := RenderErrorLogOverlay(nil, 0, 20, false, ErrorLogVisualParams{})
		assert.Contains(t, result, "No log entries")
	})

	t.Run("renders entries", func(t *testing.T) {
		entries := []ErrorLogEntry{
			{Time: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), Message: "first error", Level: "ERR"},
			{Time: time.Date(2024, 1, 1, 10, 1, 0, 0, time.UTC), Message: "second warning", Level: "WRN"},
			{Time: time.Date(2024, 1, 1, 10, 2, 0, 0, time.UTC), Message: "info message", Level: "INF"},
		}
		result := RenderErrorLogOverlay(entries, 0, 20, false, ErrorLogVisualParams{})
		assert.Contains(t, result, "Application Log")
		assert.Contains(t, result, "first error")
		assert.Contains(t, result, "second warning")
		assert.Contains(t, result, "info message")
		assert.Contains(t, result, "3 entries")
	})

	t.Run("shows ERR/WRN/INF levels", func(t *testing.T) {
		entries := []ErrorLogEntry{
			{Time: time.Now(), Message: "err", Level: "ERR"},
			{Time: time.Now(), Message: "wrn", Level: "WRN"},
			{Time: time.Now(), Message: "inf", Level: "INF"},
		}
		result := RenderErrorLogOverlay(entries, 0, 20, false, ErrorLogVisualParams{})
		assert.Contains(t, result, "ERR")
		assert.Contains(t, result, "WRN")
		assert.Contains(t, result, "INF")
	})

	t.Run("debug entries hidden by default", func(t *testing.T) {
		entries := []ErrorLogEntry{
			{Time: time.Now(), Message: "debug msg", Level: "DBG"},
			{Time: time.Now(), Message: "info msg", Level: "INF"},
		}
		result := RenderErrorLogOverlay(entries, 0, 20, false, ErrorLogVisualParams{})
		assert.NotContains(t, result, "debug msg")
		assert.Contains(t, result, "info msg")
		assert.Contains(t, result, "1 hidden")
	})

	t.Run("debug entries shown when enabled", func(t *testing.T) {
		entries := []ErrorLogEntry{
			{Time: time.Now(), Message: "debug msg", Level: "DBG"},
			{Time: time.Now(), Message: "info msg", Level: "INF"},
		}
		result := RenderErrorLogOverlay(entries, 0, 20, true, ErrorLogVisualParams{})
		assert.Contains(t, result, "debug msg")
		assert.Contains(t, result, "DBG")
		assert.Contains(t, result, "info msg")
	})
}

func TestFilteredErrorLogEntries(t *testing.T) {
	entries := []ErrorLogEntry{
		{Time: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), Message: "first", Level: "ERR"},
		{Time: time.Date(2024, 1, 1, 10, 1, 0, 0, time.UTC), Message: "second", Level: "DBG"},
		{Time: time.Date(2024, 1, 1, 10, 2, 0, 0, time.UTC), Message: "third", Level: "INF"},
	}

	t.Run("returns reversed order", func(t *testing.T) {
		result := FilteredErrorLogEntries(entries, true)
		assert.Len(t, result, 3)
		assert.Equal(t, "third", result[0].Message)
		assert.Equal(t, "first", result[2].Message)
	})

	t.Run("filters debug when disabled", func(t *testing.T) {
		result := FilteredErrorLogEntries(entries, false)
		assert.Len(t, result, 2)
		for _, e := range result {
			assert.NotEqual(t, "DBG", e.Level)
		}
	})
}

func TestErrorLogEntryPlainText(t *testing.T) {
	e := ErrorLogEntry{
		Time:    time.Date(2024, 1, 1, 10, 30, 15, 0, time.UTC),
		Level:   "ERR",
		Message: "something failed",
	}
	result := ErrorLogEntryPlainText(e)
	assert.Equal(t, "10:30:15 [ERR] something failed", result)
}

func TestRenderErrorLogOverlayVisualMode(t *testing.T) {
	entries := []ErrorLogEntry{
		{Time: time.Now(), Message: "line 0", Level: "ERR"},
		{Time: time.Now(), Message: "line 1", Level: "INF"},
		{Time: time.Now(), Message: "line 2", Level: "WRN"},
	}

	t.Run("visual mode shows VISUAL LINE indicator", func(t *testing.T) {
		vp := ErrorLogVisualParams{VisualMode: 'V', VisualStart: 0, CursorLine: 1}
		result := RenderErrorLogOverlay(entries, 0, 20, false, vp)
		assert.Contains(t, result, "VISUAL LINE")
	})

	t.Run("char visual mode shows VISUAL indicator", func(t *testing.T) {
		vp := ErrorLogVisualParams{VisualMode: 'v', VisualStart: 0, CursorLine: 0}
		result := RenderErrorLogOverlay(entries, 0, 20, false, vp)
		assert.Contains(t, result, "VISUAL")
		assert.NotContains(t, result, "VISUAL LINE")
	})

	t.Run("no visual mode has no indicator", func(t *testing.T) {
		result := RenderErrorLogOverlay(entries, 0, 20, false, ErrorLogVisualParams{})
		assert.NotContains(t, result, "VISUAL")
	})
}

// --- RenderTemplateOverlay ---

func TestRenderTemplateOverlay(t *testing.T) {
	t.Run("empty templates", func(t *testing.T) {
		result := RenderTemplateOverlay(nil, "", 0, false, 25)
		assert.Contains(t, result, "No templates available")
	})

	t.Run("with templates", func(t *testing.T) {
		templates := []model.ResourceTemplate{
			{Name: "Deployment", Description: "Basic deployment", Category: "Workloads"},
			{Name: "Service", Description: "ClusterIP service", Category: "Networking"},
		}
		result := RenderTemplateOverlay(templates, "", 0, false, 25)
		assert.Contains(t, result, "Create from Template")
		assert.Contains(t, result, "Deployment")
		assert.Contains(t, result, "Service")
	})
}

// --- RenderPodStartupOverlay ---

func TestRenderPodStartupOverlay(t *testing.T) {
	t.Run("renders with phases", func(t *testing.T) {
		entry := PodStartupEntry{
			PodName:   "nginx-abc123",
			Namespace: "default",
			TotalTime: 5 * time.Second,
			Phases: []StartupPhaseEntry{
				{Name: "Scheduling", Duration: 100 * time.Millisecond, Status: "completed"},
				{Name: "Image Pull", Duration: 2 * time.Second, Status: "completed"},
				{Name: "Container Startup", Duration: 1 * time.Second, Status: "completed"},
				{Name: "Readiness Probes", Duration: 500 * time.Millisecond, Status: "completed"},
			},
		}
		result := RenderPodStartupOverlay(entry)
		assert.Contains(t, result, "Pod Startup Analysis")
		assert.Contains(t, result, "nginx-abc123")
		assert.Contains(t, result, "default")
		assert.Contains(t, result, "Scheduling")
		assert.Contains(t, result, "Image Pull")
		assert.Contains(t, result, "Container Startup")
		assert.Contains(t, result, "Readiness Probes")
		// Hint moved to status bar.
		assert.NotContains(t, result, "Press any key to close")
	})

	t.Run("no phases shows message", func(t *testing.T) {
		entry := PodStartupEntry{
			PodName:   "empty-pod",
			Namespace: "test",
			TotalTime: 0,
		}
		result := RenderPodStartupOverlay(entry)
		assert.Contains(t, result, "No startup phases available")
	})

	t.Run("in-progress phase shows indicator", func(t *testing.T) {
		entry := PodStartupEntry{
			PodName:   "starting-pod",
			Namespace: "default",
			TotalTime: 10 * time.Second,
			Phases: []StartupPhaseEntry{
				{Name: "Scheduling", Duration: 10 * time.Second, Status: "in-progress"},
			},
		}
		result := RenderPodStartupOverlay(entry)
		assert.Contains(t, result, "\u25cb") // circle indicator for in-progress
	})

	t.Run("shows legend", func(t *testing.T) {
		entry := PodStartupEntry{
			PodName:   "pod",
			Namespace: "ns",
			TotalTime: 1 * time.Second,
			Phases: []StartupPhaseEntry{
				{Name: "Scheduling", Duration: 1 * time.Second, Status: "completed"},
			},
		}
		result := RenderPodStartupOverlay(entry)
		assert.Contains(t, result, "schedule")
		assert.Contains(t, result, "pull")
		assert.Contains(t, result, "init")
		assert.Contains(t, result, "start")
		assert.Contains(t, result, "ready")
		assert.Contains(t, result, "in-progress")
	})
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{500 * time.Microsecond, "500\u00b5s"},
		{100 * time.Millisecond, "100ms"},
		{1500 * time.Millisecond, "1.5s"},
		{90 * time.Second, "1m30s"},
		{3661 * time.Second, "1h1m"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.duration), func(t *testing.T) {
			result := formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- RenderQuotaDashboardOverlay ---

func TestRenderQuotaDashboardOverlay(t *testing.T) {
	t.Run("renders quota with namespace", func(t *testing.T) {
		quotas := []QuotaEntry{
			{
				Name:      "default-quota",
				Namespace: "production",
				Resources: []QuotaResourceEntry{
					{Name: "cpu", Hard: "4", Used: "2", Percent: 50},
					{Name: "memory", Hard: "8Gi", Used: "6Gi", Percent: 75},
					{Name: "pods", Hard: "20", Used: "19", Percent: 95},
				},
			},
		}
		result := RenderQuotaDashboardOverlay(quotas, 80, 30)
		assert.Contains(t, result, "Resource Quotas - production")
		assert.Contains(t, result, "default-quota")
		assert.Contains(t, result, "cpu")
		assert.Contains(t, result, "memory")
		assert.Contains(t, result, "pods")
		assert.Contains(t, result, "2 / 4")
		assert.Contains(t, result, "6Gi / 8Gi")
		assert.Contains(t, result, "19 / 20")
		assert.Contains(t, result, "50%")
		assert.Contains(t, result, "75%")
		assert.Contains(t, result, "95%")
	})

	t.Run("shows all namespaces when namespace is empty", func(t *testing.T) {
		quotas := []QuotaEntry{
			{Name: "q1", Namespace: ""},
		}
		result := RenderQuotaDashboardOverlay(quotas, 80, 30)
		assert.Contains(t, result, "all namespaces")
	})

	t.Run("renders close hint", func(t *testing.T) {
		quotas := []QuotaEntry{
			{
				Name:      "test",
				Namespace: "default",
				Resources: []QuotaResourceEntry{
					{Name: "pods", Hard: "10", Used: "5", Percent: 50},
				},
			},
		}
		result := RenderQuotaDashboardOverlay(quotas, 80, 30)
		// Hint moved to status bar.
		assert.NotContains(t, result, "esc/q: close")
	})

	t.Run("multiple quotas", func(t *testing.T) {
		quotas := []QuotaEntry{
			{
				Name:      "quota-a",
				Namespace: "staging",
				Resources: []QuotaResourceEntry{
					{Name: "cpu", Hard: "2", Used: "1", Percent: 50},
				},
			},
			{
				Name:      "quota-b",
				Namespace: "staging",
				Resources: []QuotaResourceEntry{
					{Name: "memory", Hard: "4Gi", Used: "3Gi", Percent: 75},
				},
			},
		}
		result := RenderQuotaDashboardOverlay(quotas, 80, 30)
		assert.Contains(t, result, "quota-a")
		assert.Contains(t, result, "quota-b")
	})

	t.Run("zero percent shows zero filled bar", func(t *testing.T) {
		quotas := []QuotaEntry{
			{
				Name:      "empty",
				Namespace: "test",
				Resources: []QuotaResourceEntry{
					{Name: "pods", Hard: "10", Used: "0", Percent: 0},
				},
			},
		}
		result := RenderQuotaDashboardOverlay(quotas, 80, 30)
		assert.Contains(t, result, "0%")
		assert.Contains(t, result, "0 / 10")
	})
}

// --- PlaceOverlay ---

func TestPlaceOverlay(t *testing.T) {
	t.Run("centers overlay", func(t *testing.T) {
		bg := strings.Repeat(".", 20) + "\n" +
			strings.Repeat(".", 20) + "\n" +
			strings.Repeat(".", 20) + "\n" +
			strings.Repeat(".", 20) + "\n" +
			strings.Repeat(".", 20)
		overlay := "XX"
		result := PlaceOverlay(20, 5, overlay, bg)
		lines := strings.Split(result, "\n")
		assert.Equal(t, 5, len(lines))
		// Overlay should appear in the middle row (row 2).
		assert.Contains(t, lines[2], "XX")
	})

	t.Run("handles short background", func(t *testing.T) {
		bg := "short"
		overlay := "OV"
		result := PlaceOverlay(10, 3, overlay, bg)
		lines := strings.Split(result, "\n")
		assert.Equal(t, 3, len(lines))
	})
}

// --- RenderAlertsOverlay ---

func TestRenderAlertsOverlay(t *testing.T) {
	t.Run("empty alerts shows no-alerts message", func(t *testing.T) {
		result := RenderAlertsOverlay(nil, 0, 80, 25)
		assert.Contains(t, result, "No active alerts")
		// Hint moved to status bar.
	})

	t.Run("renders single alert", func(t *testing.T) {
		alerts := []AlertEntry{
			{
				Name:        "HighCPU",
				State:       "firing",
				Severity:    "critical",
				Summary:     "CPU usage above 90%",
				Description: "Pod nginx is using too much CPU",
				Since:       time.Now().Add(-2 * time.Hour),
			},
		}
		result := RenderAlertsOverlay(alerts, 0, 80, 25)
		assert.Contains(t, result, "Active Alerts")
		assert.Contains(t, result, "HighCPU")
		assert.Contains(t, result, "critical")
		assert.Contains(t, result, "firing")
		assert.Contains(t, result, "CPU usage above 90%")
		assert.Contains(t, result, "1 alert(s)")
	})

	t.Run("renders multiple alerts", func(t *testing.T) {
		alerts := []AlertEntry{
			{
				Name:     "HighCPU",
				State:    "firing",
				Severity: "critical",
				Summary:  "CPU high",
			},
			{
				Name:     "LowDisk",
				State:    "pending",
				Severity: "warning",
				Summary:  "Disk running low",
			},
		}
		result := RenderAlertsOverlay(alerts, 0, 80, 25)
		assert.Contains(t, result, "HighCPU")
		assert.Contains(t, result, "LowDisk")
		assert.Contains(t, result, "2 alert(s)")
	})

	t.Run("renders grafana URL", func(t *testing.T) {
		alerts := []AlertEntry{
			{
				Name:       "Alert1",
				State:      "firing",
				Severity:   "info",
				GrafanaURL: "https://grafana.example.com/d/abc",
			},
		}
		result := RenderAlertsOverlay(alerts, 0, 80, 25)
		assert.Contains(t, result, "grafana.example.com")
	})

	t.Run("scroll clamps to bounds", func(t *testing.T) {
		alerts := []AlertEntry{
			{Name: "A1", State: "firing", Severity: "warning"},
		}
		// Scroll beyond content should not panic.
		result := RenderAlertsOverlay(alerts, 100, 80, 25)
		assert.Contains(t, result, "A1")
	})

	t.Run("negative scroll clamps to zero", func(t *testing.T) {
		alerts := []AlertEntry{
			{Name: "A1", State: "firing", Severity: "info"},
		}
		result := RenderAlertsOverlay(alerts, -5, 80, 25)
		assert.Contains(t, result, "A1")
	})
}

// --- formatRelativeTime ---

func TestFormatRelativeTime(t *testing.T) {
	t.Run("seconds ago", func(t *testing.T) {
		result := formatRelativeTime(time.Now().Add(-30 * time.Second))
		assert.Contains(t, result, "s ago")
	})

	t.Run("minutes ago", func(t *testing.T) {
		result := formatRelativeTime(time.Now().Add(-5 * time.Minute))
		assert.Contains(t, result, "m ago")
	})

	t.Run("hours ago", func(t *testing.T) {
		result := formatRelativeTime(time.Now().Add(-3 * time.Hour))
		assert.Contains(t, result, "h")
		assert.Contains(t, result, "ago")
	})

	t.Run("days ago", func(t *testing.T) {
		result := formatRelativeTime(time.Now().Add(-48 * time.Hour))
		assert.Contains(t, result, "d ago")
	})
}

// --- RenderNetworkPolicyOverlay ---

func TestRenderNetworkPolicyOverlay(t *testing.T) {
	t.Run("renders basic policy with title and namespace", func(t *testing.T) {
		info := NetworkPolicyEntry{
			Name:        "deny-all",
			Namespace:   "production",
			PodSelector: map[string]string{"app": "backend"},
			PolicyTypes: []string{"Ingress", "Egress"},
		}
		result := RenderNetworkPolicyOverlay(info, 0, 70, 40)
		assert.Contains(t, result, "deny-all")
		assert.Contains(t, result, "production")
	})

	t.Run("shows pod selector labels", func(t *testing.T) {
		info := NetworkPolicyEntry{
			Name:        "test-policy",
			Namespace:   "default",
			PodSelector: map[string]string{"app": "frontend", "tier": "web"},
			PolicyTypes: []string{"Ingress"},
		}
		result := RenderNetworkPolicyOverlay(info, 0, 70, 40)
		assert.Contains(t, result, "app=frontend")
		assert.Contains(t, result, "tier=web")
	})

	t.Run("shows all pods when no selector", func(t *testing.T) {
		info := NetworkPolicyEntry{
			Name:        "default-deny",
			Namespace:   "default",
			PolicyTypes: []string{"Ingress"},
		}
		result := RenderNetworkPolicyOverlay(info, 0, 70, 40)
		assert.Contains(t, result, "all pods in namespace")
	})

	t.Run("shows affected pod count", func(t *testing.T) {
		info := NetworkPolicyEntry{
			Name:         "test",
			Namespace:    "default",
			PolicyTypes:  []string{"Ingress"},
			AffectedPods: []string{"pod-1", "pod-2", "pod-3"},
		}
		result := RenderNetworkPolicyOverlay(info, 0, 70, 40)
		assert.Contains(t, result, "3 pod(s)")
		assert.Contains(t, result, "pod-1")
		assert.Contains(t, result, "pod-3")
	})

	t.Run("shows denied message when no rules", func(t *testing.T) {
		info := NetworkPolicyEntry{
			Name:        "deny-ingress",
			Namespace:   "default",
			PolicyTypes: []string{"Ingress"},
		}
		result := RenderNetworkPolicyOverlay(info, 0, 70, 40)
		assert.Contains(t, result, "all ingress denied")
	})

	t.Run("renders ingress rule with pod peer", func(t *testing.T) {
		info := NetworkPolicyEntry{
			Name:        "allow-frontend",
			Namespace:   "default",
			PodSelector: map[string]string{"app": "backend"},
			PolicyTypes: []string{"Ingress"},
			IngressRules: []NetpolRuleEntry{
				{
					Ports: []NetpolPortEntry{{Protocol: "TCP", Port: "8080"}},
					Peers: []NetpolPeerEntry{{
						Type:     "Pod",
						Selector: map[string]string{"app": "frontend"},
					}},
				},
			},
		}
		result := RenderNetworkPolicyOverlay(info, 0, 70, 50)
		assert.Contains(t, result, "INGRESS RULES")
		assert.Contains(t, result, "Rule 1")
		assert.Contains(t, result, "app=frontend")
		assert.Contains(t, result, "TCP/8080")
	})

	t.Run("renders egress rule with CIDR peer", func(t *testing.T) {
		info := NetworkPolicyEntry{
			Name:        "allow-egress",
			Namespace:   "default",
			PodSelector: map[string]string{"app": "backend"},
			PolicyTypes: []string{"Egress"},
			EgressRules: []NetpolRuleEntry{
				{
					Ports: []NetpolPortEntry{{Protocol: "TCP", Port: "5432"}},
					Peers: []NetpolPeerEntry{{
						Type:   "CIDR",
						CIDR:   "10.0.0.0/8",
						Except: []string{"10.0.1.0/24"},
					}},
				},
			},
		}
		result := RenderNetworkPolicyOverlay(info, 0, 70, 50)
		assert.Contains(t, result, "EGRESS RULES")
		assert.Contains(t, result, "10.0.0.0/8")
		assert.Contains(t, result, "10.0.1.0/24")
		assert.Contains(t, result, "TCP/5432")
	})

	t.Run("renders namespace peer", func(t *testing.T) {
		info := NetworkPolicyEntry{
			Name:        "cross-ns",
			Namespace:   "default",
			PolicyTypes: []string{"Ingress"},
			IngressRules: []NetpolRuleEntry{
				{
					Peers: []NetpolPeerEntry{{
						Type:      "Namespace",
						Namespace: "env=production",
					}},
				},
			},
		}
		result := RenderNetworkPolicyOverlay(info, 0, 70, 50)
		assert.Contains(t, result, "env=production")
		assert.Contains(t, result, "Namespace")
	})

	t.Run("footer hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		info := NetworkPolicyEntry{
			Name:      "test",
			Namespace: "default",
		}
		result := RenderNetworkPolicyOverlay(info, 0, 70, 40)
		assert.NotContains(t, result, "j/k")
		assert.NotContains(t, result, "close")
	})

	t.Run("scrolling works", func(t *testing.T) {
		info := NetworkPolicyEntry{
			Name:        "test",
			Namespace:   "default",
			PolicyTypes: []string{"Ingress"},
			IngressRules: []NetpolRuleEntry{
				{Peers: []NetpolPeerEntry{{Type: "All"}}},
				{Peers: []NetpolPeerEntry{{Type: "All"}}},
				{Peers: []NetpolPeerEntry{{Type: "All"}}},
			},
		}
		// Use a very small height to force scrolling.
		result0 := RenderNetworkPolicyOverlay(info, 0, 70, 10)
		result1 := RenderNetworkPolicyOverlay(info, 1, 70, 10)
		// Scrolled results should differ.
		assert.NotEqual(t, result0, result1)
	})
}

// --- RenderDiffView ---

func TestRenderDiffView(t *testing.T) {
	t.Run("identical content shows no colored lines", func(t *testing.T) {
		yaml := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test"
		result := RenderDiffView(yaml, yaml, "pod-a", "pod-b", 0, 80, 30, false)
		assert.Contains(t, result, "Resource Diff")
		assert.Contains(t, result, "pod-a")
		assert.Contains(t, result, "pod-b")
		assert.Contains(t, result, "apiVersion: v1")
	})

	t.Run("different content shows both sides", func(t *testing.T) {
		left := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: left-pod"
		right := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: right-pod"
		result := RenderDiffView(left, right, "left", "right", 0, 80, 30, false)
		assert.Contains(t, result, "Resource Diff")
		assert.Contains(t, result, "left")
		assert.Contains(t, result, "right")
		assert.Contains(t, result, "left-pod")
		assert.Contains(t, result, "right-pod")
	})

	t.Run("empty content renders without panic", func(t *testing.T) {
		result := RenderDiffView("", "", "a", "b", 0, 80, 20, false)
		assert.Contains(t, result, "Resource Diff")
	})

	t.Run("scroll position is respected", func(t *testing.T) {
		lines := make([]string, 0, 50)
		for i := range 50 {
			lines = append(lines, fmt.Sprintf("line-%d: value", i))
		}
		yaml := strings.Join(lines, "\n")
		result0 := RenderDiffView(yaml, yaml, "a", "b", 0, 80, 20, false)
		result10 := RenderDiffView(yaml, yaml, "a", "b", 10, 80, 20, false)
		// First line visible at scroll 0 should differ from scroll 10.
		assert.Contains(t, result0, "line-0")
		assert.NotContains(t, result10, "line-0")
		assert.Contains(t, result10, "line-10")
	})

	t.Run("shows hint bar", func(t *testing.T) {
		result := RenderDiffView("a: 1", "a: 2", "x", "y", 0, 140, 20, false)
		assert.Contains(t, result, "j/k")
		assert.Contains(t, result, "q/esc")
	})
}

func TestDiffViewTotalLines(t *testing.T) {
	t.Run("identical content", func(t *testing.T) {
		yaml := "a: 1\nb: 2\nc: 3"
		total := DiffViewTotalLines(yaml, yaml)
		// 3 content lines + 1 header line = 4.
		assert.Equal(t, 4, total)
	})

	t.Run("completely different content", func(t *testing.T) {
		left := "a: 1\nb: 2"
		right := "c: 3\nd: 4"
		total := DiffViewTotalLines(left, right)
		// No common lines: 2 left-only + 2 right-only = 4 diff lines + 1 header = 5.
		assert.Equal(t, 5, total)
	})

	t.Run("empty content", func(t *testing.T) {
		total := DiffViewTotalLines("", "")
		// 0 diff lines + 1 header = 1.
		assert.Equal(t, 1, total)
	})
}

func TestComputeDiff(t *testing.T) {
	t.Run("identical lines", func(t *testing.T) {
		lines := computeDiff("a\nb\nc", "a\nb\nc")
		assert.Len(t, lines, 3)
		for _, l := range lines {
			assert.Equal(t, byte('='), l.status)
		}
	})

	t.Run("left only lines", func(t *testing.T) {
		lines := computeDiff("a\nb\nc", "a\nc")
		// a is common, b is left-only, c is common.
		assert.Len(t, lines, 3)
		assert.Equal(t, byte('='), lines[0].status)
		assert.Equal(t, byte('<'), lines[1].status)
		assert.Equal(t, "b", lines[1].left)
		assert.Equal(t, byte('='), lines[2].status)
	})

	t.Run("right only lines", func(t *testing.T) {
		lines := computeDiff("a\nc", "a\nb\nc")
		assert.Len(t, lines, 3)
		assert.Equal(t, byte('='), lines[0].status)
		assert.Equal(t, byte('>'), lines[1].status)
		assert.Equal(t, "b", lines[1].right)
		assert.Equal(t, byte('='), lines[2].status)
	})

	t.Run("completely different", func(t *testing.T) {
		lines := computeDiff("a\nb", "c\nd")
		// No common lines, so all are additions/removals.
		assert.Len(t, lines, 4)
		leftCount := 0
		rightCount := 0
		for _, l := range lines {
			if l.status == '<' {
				leftCount++
			}
			if l.status == '>' {
				rightCount++
			}
		}
		assert.Equal(t, 2, leftCount)
		assert.Equal(t, 2, rightCount)
	})

	t.Run("empty inputs", func(t *testing.T) {
		lines := computeDiff("", "")
		assert.Len(t, lines, 0)
	})
}

func TestTruncateToWidth(t *testing.T) {
	t.Run("short string unchanged", func(t *testing.T) {
		assert.Equal(t, "hello", truncateToWidth("hello", 10))
	})

	t.Run("long string truncated", func(t *testing.T) {
		result := truncateToWidth("hello world this is long", 10)
		assert.LessOrEqual(t, len(result), 10)
		assert.True(t, strings.HasSuffix(result, "~"))
	})

	t.Run("empty string", func(t *testing.T) {
		assert.Equal(t, "", truncateToWidth("", 10))
	})
}

func TestPadToWidth(t *testing.T) {
	t.Run("short string padded", func(t *testing.T) {
		result := padToWidth("hi", 10)
		assert.Equal(t, 10, len(result))
		assert.True(t, strings.HasPrefix(result, "hi"))
	})

	t.Run("exact width unchanged", func(t *testing.T) {
		result := padToWidth("1234567890", 10)
		assert.Equal(t, "1234567890", result)
	})
}
