package ui

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// --- renderKV ---

func TestRenderKV(t *testing.T) {
	t.Run("short value fits", func(t *testing.T) {
		result := renderKV("Node", "worker-1", 60)
		assert.Contains(t, stripANSI(result), "Node:")
		assert.Contains(t, result, "worker-1")
	})

	t.Run("long value truncated with ellipsis", func(t *testing.T) {
		result := renderKV("Labels", "this-is-a-very-long-label-value-that-exceeds-the-width", 30)
		assert.Contains(t, stripANSI(result), "Labels:")
		assert.Contains(t, result, "...")
	})
}

// --- renderDataKV ---

func TestRenderDataKV(t *testing.T) {
	t.Run("single line value", func(t *testing.T) {
		lines := renderDataKV("config.yaml", "value=123", 60)
		assert.Len(t, lines, 1)
		assert.Contains(t, stripANSI(lines[0]), "config.yaml:")
		assert.Contains(t, lines[0], "value=123")
	})

	t.Run("multiline value returns multiple lines", func(t *testing.T) {
		lines := renderDataKV("config", "line1\nline2\nline3", 60)
		assert.Greater(t, len(lines), 1)
		assert.Contains(t, stripANSI(lines[0]), "config:")
	})

	t.Run("escaped newlines are expanded", func(t *testing.T) {
		lines := renderDataKV("key", `first\nsecond`, 60)
		assert.Greater(t, len(lines), 1)
	})
}

// --- renderUsageBar ---

func TestRenderUsageBar(t *testing.T) {
	// A reservation wide enough for every value string used below; production
	// derives it from the actual CPU/Mem widths (see RenderResourceUsage).
	const reserve = 24

	t.Run("no request and no limit shows value only", func(t *testing.T) {
		result := stripANSI(renderUsageBar(500, 0, 0, 40, FormatCPU, reserve))
		assert.Contains(t, result, "500m")
		assert.Contains(t, result, "no request/limit")
		assert.NotContains(t, result, "[", "no bar without a request or limit")
	})

	t.Run("with limit shows bar and percentage", func(t *testing.T) {
		result := renderUsageBar(500, 0, 1000, 60, FormatCPU, reserve)
		assert.Contains(t, result, "500m")
		assert.Contains(t, result, "1.0")
		assert.Contains(t, result, "50%")
		assert.Contains(t, result, "[")
		assert.Contains(t, result, "]")
	})

	t.Run("with request as fallback reference", func(t *testing.T) {
		result := renderUsageBar(250, 500, 0, 60, FormatCPU, reserve)
		assert.Contains(t, result, "250m")
		assert.Contains(t, result, "50%")
	})

	t.Run("memory usage bar", func(t *testing.T) {
		result := renderUsageBar(512*1024*1024, 0, 1024*1024*1024, 60, FormatMemory, reserve)
		assert.Contains(t, result, "512Mi")
		assert.Contains(t, result, "1.0Gi")
		assert.Contains(t, result, "50%")
	})

	t.Run("over 100 percent capped at 100", func(t *testing.T) {
		result := renderUsageBar(2000, 0, 1000, 60, FormatCPU, reserve)
		assert.Contains(t, result, "100%")
	})

	t.Run("same suffix reservation yields equal bar length", func(t *testing.T) {
		// Two bars with very different value-string widths must produce the
		// same bracketed bar length when given the same reserved suffix width.
		cpu := renderUsageBar(1, 0, 100, 100, FormatCPU, reserve)
		mem := renderUsageBar(69*1024*1024, 0, 160*1024*1024, 100, FormatMemory, reserve)
		assert.Equal(t, barInnerWidth(cpu), barInnerWidth(mem),
			"bars sharing a suffix reservation must be the same length")
	})

	t.Run("line fills full width with right-aligned value", func(t *testing.T) {
		// The value is right-aligned in the reserved column so the line spans
		// the full barWidth and the percentages line up across rows.
		const barWidth = 100
		line := stripANSI(renderUsageBar(1, 0, 100, barWidth, FormatCPU, reserve))
		assert.Equal(t, barWidth, lipgloss.Width(line),
			"line must fill the full width")
		assert.True(t, strings.HasSuffix(line, "1m/100m (1%)"),
			"value text must be flush against the right edge, got %q", line)
	})

	t.Run("bar is segmented at the request boundary", func(t *testing.T) {
		// limit 256, request 128 (half), usage 190: green to half, then hot to
		// ~74%, then dim. No suffix reserve so the bar is the whole 64 cells.
		fill := stripANSI(renderUsageBarFill(64, 190, 128, 256))
		green := strings.Count(fill, zoneBelowRequest.fillGlyph())
		hot := strings.Count(fill, zoneAboveRequest.fillGlyph())
		dim := strings.Count(fill, usageBarEmptyGlyph)
		assert.Equal(t, 32, green, "green band runs to the request (half of 64)")
		assert.Equal(t, 48-32, hot, "hot band runs from request to usage (~74%)")
		assert.Equal(t, 64-48, dim, "dim track is the headroom up to the limit")
	})

	t.Run("no limit scales to usage with request as interior boundary", func(t *testing.T) {
		// request 25, usage 50, no limit: bar full, request at 50% -> half green
		// (within request), half orange (over request), no dim, no red.
		fill := stripANSI(renderUsageBarFill(64, 50, 25, 0))
		assert.Equal(t, 32, strings.Count(fill, zoneBelowRequest.fillGlyph()), "green up to the request (half)")
		assert.Equal(t, 32, strings.Count(fill, zoneAboveRequest.fillGlyph()), "orange over the request (rest)")
		assert.Equal(t, 0, strings.Count(fill, zoneNearLimit.fillGlyph()), "never red without a limit")
		assert.Equal(t, 0, strings.Count(fill, usageBarEmptyGlyph), "no headroom without a limit when over request")
	})

	t.Run("near limit is red even when usage is within the request", func(t *testing.T) {
		// request == limit (4Gi), usage 3.9Gi (97%): usage is within the
		// request, but >=90% of the limit must still show red.
		gi := int64(1024 * 1024 * 1024)
		fill := stripANSI(renderUsageBarFill(80, 39*gi/10, 4*gi, 4*gi))
		assert.Positive(t, strings.Count(fill, zoneNearLimit.fillGlyph()),
			"usage at 97%% of the limit must paint a red band")
		assert.Equal(t, 0, strings.Count(fill, zoneAboveRequest.fillGlyph()),
			"no orange band when the request equals the limit")
	})

	t.Run("request tick shown when usage below request", func(t *testing.T) {
		// usage 32 (< request 128): green to usage, a single green tick at the
		// request position, dim elsewhere.
		fill := stripANSI(renderUsageBarFill(64, 32, 128, 256))
		assert.Equal(t, 0, strings.Count(fill, zoneAboveRequest.fillGlyph()),
			"no hot band when usage is below the request")
		// 8 green cells of usage (32/256*64).
		assert.Equal(t, 8, strings.Count(fill, zoneBelowRequest.fillGlyph()),
			"green usage band")
		assert.Equal(t, 1, strings.Count(fill, usageBarRequestTick),
			"a single request tick marks the request position")
	})

	t.Run("zone reflects usage relative to request and limit", func(t *testing.T) {
		assert.Equal(t, zoneBelowRequest, usageBarZone(50, 100, 200), "below request")
		assert.Equal(t, zoneAboveRequest, usageBarZone(150, 100, 200), "above request, below 90%")
		assert.Equal(t, zoneAboveRequest, usageBarZone(178, 100, 200), "89% of limit is still orange")
		assert.Equal(t, zoneNearLimit, usageBarZone(180, 100, 200), "90% of limit is red")
		assert.Equal(t, zoneNearLimit, usageBarZone(200, 100, 200), "at limit")
		assert.Equal(t, zoneBelowRequest, usageBarZone(50, 0, 200), "no request, low")
		assert.Equal(t, zoneAboveRequest, usageBarZone(150, 100, 0), "no limit, above request")
	})

	t.Run("color and glyph differ per zone for colorless legibility", func(t *testing.T) {
		zones := []usageZone{zoneBelowRequest, zoneAboveRequest, zoneNearLimit}
		colors := map[string]bool{}
		glyphs := map[string]bool{}
		for _, z := range zones {
			colors[z.color()] = true
			glyphs[z.fillGlyph()] = true
			assert.NotEqual(t, usageBarEmptyGlyph, z.fillGlyph(),
				"fill glyph must differ from the empty-track glyph")
		}
		assert.Len(t, colors, 3, "each zone needs a distinct color")
		assert.Len(t, glyphs, 3, "each zone needs a distinct glyph for colorless mode")
	})
}

// barInnerWidth returns the display width of the content between the first '['
// and the matching ']' in a rendered usage bar, ignoring ANSI styling.
func barInnerWidth(line string) int {
	plain := stripANSI(line)
	open := strings.IndexRune(plain, '[')
	closeIdx := strings.IndexRune(plain, ']')
	if open < 0 || closeIdx < 0 || closeIdx <= open {
		return -1
	}
	return lipgloss.Width(plain[open+1 : closeIdx])
}

// --- RenderResourceUsage ---

func TestRenderResourceUsage(t *testing.T) {
	t.Run("basic rendering", func(t *testing.T) {
		result := RenderResourceUsage(500, 1000, 2000, 256*1024*1024, 512*1024*1024, 1024*1024*1024, 80)
		assert.Contains(t, result, "RESOURCE USAGE")
		assert.Contains(t, result, "CPU")
		assert.Contains(t, result, "Mem")
	})

	t.Run("zero values render without panic", func(t *testing.T) {
		result := RenderResourceUsage(0, 0, 0, 0, 0, 0, 60)
		assert.Contains(t, result, "RESOURCE USAGE")
		assert.Contains(t, result, "CPU")
		assert.Contains(t, result, "0m")
	})

	t.Run("CPU and Mem lines fill the full section width", func(t *testing.T) {
		const width = 80
		result := RenderResourceUsage(1, 0, 100, 69*1024*1024, 0, 160*1024*1024, width)
		lines := strings.Split(stripANSI(result), "\n")
		require.GreaterOrEqual(t, len(lines), 3, "expected header + CPU + Mem lines")
		// lines[0] is the "RESOURCE USAGE" header; the two bars follow.
		assert.Equal(t, width, lipgloss.Width(lines[1]), "CPU line must fill the width")
		assert.Equal(t, width, lipgloss.Width(lines[2]), "Mem line must fill the width")
	})
}

// --- RenderPreviewEvents ---

func TestRenderPreviewEvents(t *testing.T) {
	t.Run("empty events returns empty", func(t *testing.T) {
		result := RenderPreviewEvents(nil, 80)
		assert.Equal(t, "", result)
	})

	t.Run("events rendered with headers", func(t *testing.T) {
		events := []EventTimelineEntry{
			{
				Timestamp: time.Now().Add(-5 * time.Minute),
				Type:      "Normal",
				Reason:    "Scheduled",
				Message:   "Successfully assigned pod to node",
				Count:     1,
			},
			{
				Timestamp: time.Now().Add(-3 * time.Minute),
				Type:      "Warning",
				Reason:    "BackOff",
				Message:   "Back-off restarting failed container",
				Count:     5,
			},
		}
		result := RenderPreviewEvents(events, 100)
		assert.Contains(t, result, "EVENTS")
		assert.Contains(t, result, "Scheduled")
		assert.Contains(t, result, "BackOff")
		assert.Contains(t, result, "x5")
	})

	t.Run("normal events use normal dot", func(t *testing.T) {
		events := []EventTimelineEntry{
			{
				Timestamp: time.Now(),
				Type:      "Normal",
				Reason:    "Pulled",
				Message:   "Pulled image",
				Count:     1,
			},
		}
		result := RenderPreviewEvents(events, 80)
		assert.Contains(t, result, "Pulled")
	})

	// Regression: a high count suffix like "(x61)" used to push the rendered
	// line past `width`, so lipgloss would wrap it inside the right-column
	// style. The wrap added a visual row, the right pane overflowed
	// MaxHeight, and the pinned resource-usage footer got clipped off-screen.
	// Assert every visible event line stays within `width` for any count.
	t.Run("event lines with high count fit within width", func(t *testing.T) {
		now := time.Now()
		cases := []struct {
			name  string
			width int
			count int32
			msg   string
		}{
			{"short message x63", 80, 63, "MountVolume.SetUp failed for volume \"argocd-dex-server-tls\""},
			{"long message x61", 80, 61, "MountVolume.MountDevice failed for volume \"pvc-e5942d48-d121-46b0-85a6-3f6ce9d3026f\" : timed out waiting for the condition"},
			{"narrow column x9999", 60, 9999, "MountVolume.SetUp failed"},
			{"single digit count x2", 80, 2, "Back-off restarting failed container"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				events := []EventTimelineEntry{{
					Timestamp: now.Add(-2 * time.Hour),
					Type:      "Warning",
					Reason:    "FailedMount",
					Message:   tc.msg,
					Count:     tc.count,
				}}
				result := RenderPreviewEvents(events, tc.width)
				for line := range strings.SplitSeq(result, "\n") {
					if line == "" {
						continue
					}
					assert.LessOrEqual(t, lipgloss.Width(line), tc.width,
						"event line exceeds width=%d (would force lipgloss wrap and clip the resource-usage footer): %q (visible width %d)",
						tc.width, line, lipgloss.Width(line))
				}
			})
		}
	})
}

// --- RenderResourceSummary ---

func TestRenderResourceSummary(t *testing.T) {
	t.Run("nil item with YAML falls back to YAML", func(t *testing.T) {
		result := RenderResourceSummary(nil, "apiVersion: v1\nkind: Pod", 60, 20)
		assert.Contains(t, result, "apiVersion")
	})

	t.Run("nil item no YAML shows no preview", func(t *testing.T) {
		result := RenderResourceSummary(nil, "", 60, 20)
		assert.Contains(t, result, "No preview")
	})

	t.Run("item with no columns and YAML falls back", func(t *testing.T) {
		item := &model.Item{Name: "pod"}
		result := RenderResourceSummary(item, "kind: Pod", 60, 20)
		assert.Contains(t, result, "kind")
	})

	t.Run("item with columns renders summary table", func(t *testing.T) {
		item := &model.Item{
			Name:      "nginx-pod",
			Namespace: "default",
			Columns: []model.KeyValue{
				{Key: "Node", Value: "worker-1"},
				{Key: "IP", Value: "10.0.0.5"},
				{Key: "QoS", Value: "BestEffort"},
			},
		}
		result := RenderResourceSummary(item, "", 80, 20)
		assert.Contains(t, result, "NAME")
		assert.Contains(t, result, "nginx-pod")
		assert.Contains(t, result, "NAMESPACE")
		assert.Contains(t, result, "default")
		assert.Contains(t, result, "NODE")
		assert.Contains(t, result, "worker-1")
		assert.Contains(t, result, "IP")
	})

	t.Run("metrics columns skipped", func(t *testing.T) {
		item := &model.Item{
			Name: "pod",
			Columns: []model.KeyValue{
				{Key: "Node", Value: "node-1"},
				{Key: "CPU", Value: "500m"},
				{Key: "MEM", Value: "256Mi"},
			},
		}
		result := RenderResourceSummary(item, "", 80, 20)
		assert.Contains(t, result, "NODE")
		assert.NotContains(t, result, "CPU")
	})

	t.Run("labels rendered as multiline", func(t *testing.T) {
		item := &model.Item{
			Name: "pod",
			Columns: []model.KeyValue{
				{Key: "Node", Value: "node-1"},
				{Key: "Labels", Value: "app=nginx, env=prod"},
			},
		}
		result := RenderResourceSummary(item, "", 80, 30)
		assert.Contains(t, result, "LABELS")
		assert.Contains(t, result, "app=nginx")
	})

	t.Run("data fields rendered in data section", func(t *testing.T) {
		item := &model.Item{
			Name: "cm",
			Columns: []model.KeyValue{
				{Key: "data:config.yaml", Value: "key=value"},
			},
		}
		result := RenderResourceSummary(item, "", 80, 30)
		assert.Contains(t, result, "DATA")
		assert.Contains(t, stripANSI(result), "config.yaml")
	})

	t.Run("DATA count counts keys not visual lines (regression: flipped on reveal of multi-line value)", func(t *testing.T) {
		origShow := ActiveShowSecretValues
		defer func() { ActiveShowSecretValues = origShow }()

		item := &model.Item{
			Name: "mysecret",
			Columns: []model.KeyValue{
				{Key: "secret:single", Value: "v1"},
				// Multi-line value (e.g. PEM cert / kubeconfig). Counts
				// as ONE key, but used to inflate DATA(N) when revealed
				// because each visual line was added to dataLines.
				{Key: "secret:multi", Value: "line1\nline2\nline3\nline4\nline5"},
			},
		}

		ActiveShowSecretValues = false
		hidden := RenderResourceSummary(item, "", 80, 30)
		ActiveShowSecretValues = true
		revealed := RenderResourceSummary(item, "", 80, 30)

		assert.Contains(t, hidden, "DATA (2)", "hidden state shows 2 keys")
		assert.Contains(t, revealed, "DATA (2)",
			"revealed state must STILL show 2 keys — counting visual lines made it flip to e.g. DATA (6)")
	})

	t.Run("secret fields masked by default", func(t *testing.T) {
		origShow := ActiveShowSecretValues
		ActiveShowSecretValues = false
		defer func() { ActiveShowSecretValues = origShow }()

		item := &model.Item{
			Name: "mysecret",
			Columns: []model.KeyValue{
				{Key: "secret:password", Value: "super-secret"},
			},
		}
		result := RenderResourceSummary(item, "", 80, 30)
		assert.Contains(t, result, "DATA")
		assert.Contains(t, result, "********")
		assert.NotContains(t, result, "super-secret")
	})

	t.Run("selector rendered as multiline table", func(t *testing.T) {
		item := &model.Item{
			Name: "my-svc",
			Columns: []model.KeyValue{
				{Key: "Cluster IP", Value: "10.0.0.1"},
				{Key: "Selector", Value: "app=nginx, tier=frontend"},
			},
		}
		result := RenderResourceSummary(item, "", 80, 30)
		// Selector should be rendered as a multi-line section with each
		// selector on its own indented line, not as a single key-value row.
		assert.Contains(t, result, "SELECTOR")
		assert.Contains(t, result, "app=nginx")
		assert.Contains(t, result, "tier=frontend")
		// The two selectors must appear on different lines.
		lines := strings.Split(result, "\n")
		var selectorLines []string
		for _, line := range lines {
			if strings.Contains(line, "app=nginx") || strings.Contains(line, "tier=frontend") {
				selectorLines = append(selectorLines, line)
			}
		}
		assert.GreaterOrEqual(t, len(selectorLines), 2,
			"each selector should be on its own line")
	})

	t.Run("secret fields revealed when toggle is on", func(t *testing.T) {
		origShow := ActiveShowSecretValues
		ActiveShowSecretValues = true
		defer func() { ActiveShowSecretValues = origShow }()

		item := &model.Item{
			Name: "mysecret",
			Columns: []model.KeyValue{
				{Key: "secret:token", Value: "abc123"},
			},
		}
		result := RenderResourceSummary(item, "", 80, 30)
		assert.Contains(t, result, "abc123")
	})

	t.Run("conditions render full detail including status, reason, message, and age", func(t *testing.T) {
		item := &model.Item{
			Name: "my-deploy",
			Columns: []model.KeyValue{
				{Key: "Node", Value: "node-1"},
			},
			Conditions: []model.ConditionEntry{
				{
					Type:               "Available",
					Status:             "True",
					Reason:             "MinimumReplicasAvailable",
					Message:            "Deployment has minimum availability.",
					LastTransitionTime: time.Now().Add(-2 * time.Hour),
				},
			},
		}
		result := stripANSI(RenderResourceSummary(item, "", 80, 40))

		assert.Contains(t, result, "CONDITIONS")
		assert.Contains(t, result, "Available")
		// Status text must be visible (not color-only).
		assert.Contains(t, result, "True")
		assert.Contains(t, result, "MinimumReplicasAvailable")
		// Message must show even though the status is "True".
		assert.Contains(t, result, "Deployment has minimum availability.")
		// Age derived from LastTransitionTime (kubectl-style "2h").
		assert.Contains(t, result, "2h")
	})

	t.Run("condition with unknown status renders Unknown text", func(t *testing.T) {
		item := &model.Item{
			Name:    "my-crd",
			Columns: []model.KeyValue{{Key: "Node", Value: "node-1"}},
			Conditions: []model.ConditionEntry{
				{Type: "Ready", Status: ""},
			},
		}
		result := stripANSI(RenderResourceSummary(item, "", 80, 40))
		assert.Contains(t, result, "Ready")
		assert.Contains(t, result, "Unknown")
	})

	t.Run("bespoke condition summary columns are suppressed when structured conditions exist", func(t *testing.T) {
		// ArgoCD emits "condition:<type>" rows and a truncated "Condition"
		// roll-up. With structured conditions present, the CONDITIONS section
		// supersedes them so the preview must not show them again.
		item := &model.Item{
			Name: "my-app",
			Columns: []model.KeyValue{
				{Key: "Health", Value: "Degraded"},
				{Key: "condition:ComparisonError", Value: "rpc error (2h)"},
				{Key: "Condition", Value: "ComparisonE~"},
			},
			Conditions: []model.ConditionEntry{
				{Type: "ComparisonError", Status: "True", Message: "rpc error: failed to load target state"},
			},
		}
		result := stripANSI(RenderResourceSummary(item, "", 72, 40))

		assert.Contains(t, result, "HEALTH")                                 // unrelated status row kept
		assert.NotContains(t, result, "COMPARISONERROR  rpc error (2h)")     // bespoke condition: row dropped
		assert.NotContains(t, result, "ComparisonE~")                        // truncated roll-up dropped
		assert.Contains(t, result, "rpc error: failed to load target state") // shown once, in CONDITIONS
	})

	t.Run("condition summary columns remain when no structured conditions", func(t *testing.T) {
		// Without structured conditions, the bespoke columns are the only
		// representation and must still render.
		item := &model.Item{
			Name: "my-app",
			Columns: []model.KeyValue{
				{Key: "condition:ComparisonError", Value: "rpc error (2h)"},
			},
		}
		result := stripANSI(RenderResourceSummary(item, "", 72, 40))
		assert.Contains(t, result, "COMPARISONERROR")
	})
}

// --- RenderResourceTree (badge buckets) ---

func TestRenderResourceTree_BadgeBuckets(t *testing.T) {
	t.Run("homogeneous owned children render single-kind badge", func(t *testing.T) {
		root := &model.ResourceNode{
			Name: "my-pod", Kind: "Pod", Namespace: "ns",
			Children: []*model.ResourceNode{
				{Name: "init", Kind: "Container", Namespace: "ns"},
				{Name: "app", Kind: "Container", Namespace: "ns"},
			},
		}

		out := RenderResourceTree(root, 120, 30)

		assert.Contains(t, out, "(2 Container)")
		assert.NotContains(t, out, "refs")
	})

	t.Run("mixed owned and refs children render two-bucket badge", func(t *testing.T) {
		root := &model.ResourceNode{
			Name: "my-pod", Kind: "Pod", Namespace: "ns",
			Children: []*model.ResourceNode{
				{Name: "app", Kind: "Container", Namespace: "ns"},
				{Name: "default", Kind: "ServiceAccount", Namespace: "ns", Group: "refs"},
				{Name: "regcred", Kind: "Secret", Namespace: "ns", Group: "refs"},
			},
		}

		out := RenderResourceTree(root, 120, 30)

		assert.Contains(t, out, "(1 Container, 2 refs)")
	})

	t.Run("only refs children render refs-only badge", func(t *testing.T) {
		root := &model.ResourceNode{
			Name: "my-pod", Kind: "Pod", Namespace: "ns",
			Children: []*model.ResourceNode{
				{Name: "default", Kind: "ServiceAccount", Namespace: "ns", Group: "refs"},
				{Name: "vol-cm", Kind: "ConfigMap", Namespace: "ns", Group: "refs"},
			},
		}

		out := RenderResourceTree(root, 120, 30)

		assert.Contains(t, out, "(2 refs)")
	})
}

// --- minimal summary (no columns, no YAML) ---

func TestRenderResourceSummaryMinimal(t *testing.T) {
	t.Run("identity rows rendered without columns", func(t *testing.T) {
		item := &model.Item{Name: "sa-1", Namespace: "kube-system"}
		result := stripANSI(RenderResourceSummary(item, "", 60, 20))
		assert.Contains(t, result, "NAME")
		assert.Contains(t, result, "sa-1")
		assert.Contains(t, result, "NAMESPACE")
		assert.Contains(t, result, "kube-system")
	})

	t.Run("terminating item shows deletion row", func(t *testing.T) {
		item := &model.Item{Name: "sa-1", Deleting: true}
		result := stripANSI(RenderResourceSummary(item, "", 60, 20))
		assert.Contains(t, result, "DELETION")
	})

	t.Run("multibyte value truncates on display width without breaking UTF-8", func(t *testing.T) {
		item := &model.Item{Name: strings.Repeat("日", 40), Namespace: "default"}
		result := stripANSI(RenderResourceSummary(item, "", 30, 20))
		for line := range strings.SplitSeq(result, "\n") {
			assert.True(t, utf8.ValidString(line), "line must stay valid UTF-8: %q", line)
			assert.LessOrEqual(t, lipgloss.Width(line), 30, "line must fit width: %q", line)
		}
	})
}

func TestRenderMinimalSummaryNoIdentity(t *testing.T) {
	// An item with nothing to show must not render a blank pane.
	result := stripANSI(RenderResourceSummary(&model.Item{}, "", 60, 20))
	assert.Contains(t, result, "No preview")
}

// --- sanitization: TASK-880 ---

// resourceHostilePayloads mirrors the payload set used across the other
// TASK-880 sanitization tests.
var resourceHostilePayloads = map[string]string{
	"bidi override": "ab\u202ecd",
	"raw CSI":       "ab\x1b[2Jcd",
	"OSC-52":        "ab\x1b]52;c;aGF4\x07cd",
}

func assertResourceClean(t *testing.T, out string) {
	t.Helper()
	assert.NotContains(t, out, "\u202e", "bidi override must not survive")
	assert.NotContains(t, out, "\x1b[2J", "CSI erase display must not survive")
	assert.NotContains(t, out, "\x1b]52", "OSC-52 clipboard write must not survive")
}

func TestRenderResourceSummary_IdentityRowsSanitized(t *testing.T) {
	for name, payload := range resourceHostilePayloads {
		t.Run(name, func(t *testing.T) {
			item := &model.Item{Name: "pod" + payload, Namespace: "ns" + payload, Deleting: true}
			out := RenderResourceSummary(item, "", 80, 30)
			assertResourceClean(t, out)
		})
	}
}

func TestRenderResourceSummary_DetailRowsSanitized(t *testing.T) {
	for name, payload := range resourceHostilePayloads {
		t.Run(name, func(t *testing.T) {
			item := &model.Item{
				Name: "svc-1",
				Columns: []model.KeyValue{
					{Key: "Health" + payload, Value: "Healthy" + payload},
					{Key: "Reason", Value: "Started" + payload},
					{Key: "Message", Value: "back-off restarting" + payload},
					{Key: "condition:Ready" + payload, Value: "True" + payload},
				},
			}
			out := RenderResourceSummary(item, "", 100, 40)
			assertResourceClean(t, out)
		})
	}
}

func TestRenderResourceSummary_MultiLineFieldsSanitized(t *testing.T) {
	for name, payload := range resourceHostilePayloads {
		t.Run(name, func(t *testing.T) {
			item := &model.Item{
				Name: "svc-1",
				Columns: []model.KeyValue{
					{Key: "Labels", Value: "app=web" + payload + ", tier=frontend"},
					{Key: "Endpoints", Value: "10.0.0.1:80" + payload + "\n10.0.0.2:80"},
					{Key: "Taints", Value: "node-role.kubernetes.io/master" + payload},
				},
			}
			out := RenderResourceSummary(item, "", 100, 40)
			assertResourceClean(t, out)
		})
	}
}

func TestRenderResourceSummary_ConditionsSanitized(t *testing.T) {
	for name, payload := range resourceHostilePayloads {
		t.Run(name, func(t *testing.T) {
			item := &model.Item{
				Name:    "deploy-1",
				Columns: []model.KeyValue{{Key: "Node", Value: "node-1"}},
				Conditions: []model.ConditionEntry{
					{Type: "Available" + payload, Status: "True" + payload, Reason: "MinimumReplicas" + payload, Message: "Deployment has minimum availability" + payload},
				},
			}
			out := RenderResourceSummary(item, "", 100, 40)
			assertResourceClean(t, out)
		})
	}
}

func TestRenderResourceSummary_StepsSanitized(t *testing.T) {
	for name, payload := range resourceHostilePayloads {
		t.Run(name, func(t *testing.T) {
			item := &model.Item{
				Name: "workflow-1",
				Columns: []model.KeyValue{
					{Key: "step:build" + payload, Value: "Succeeded" + payload},
				},
			}
			out := RenderResourceSummary(item, "", 100, 40)
			assertResourceClean(t, out)
		})
	}
}

func TestRenderResourceSummary_SecretDataSanitized(t *testing.T) {
	prev := ActiveShowSecretValues
	t.Cleanup(func() { ActiveShowSecretValues = prev })
	ActiveShowSecretValues = true
	for name, payload := range resourceHostilePayloads {
		t.Run(name, func(t *testing.T) {
			item := &model.Item{
				Name: "secret-1",
				Columns: []model.KeyValue{
					{Key: "secret:token" + payload, Value: "s3cr3t" + payload},
					{Key: "data:config.yaml" + payload, Value: "line1\\nline2" + payload},
				},
			}
			out := RenderResourceSummary(item, "", 100, 40)
			assertResourceClean(t, out)
		})
	}
}

func TestRenderPreviewEvents_Sanitized(t *testing.T) {
	for name, payload := range resourceHostilePayloads {
		t.Run(name, func(t *testing.T) {
			events := []EventTimelineEntry{{
				Timestamp: time.Now(),
				Type:      "Warning",
				Reason:    "BackOff" + payload,
				Message:   "Back-off restarting failed container" + payload,
				Count:     1,
			}}
			out := RenderPreviewEvents(events, 100)
			assertResourceClean(t, out)
		})
	}
}

func TestRenderResourceTree_LabelsSanitized(t *testing.T) {
	for name, payload := range resourceHostilePayloads {
		t.Run(name, func(t *testing.T) {
			root := &model.ResourceNode{
				Name: "pod" + payload, Kind: "Pod", Namespace: "ns" + payload,
				Status: "Running" + payload,
				Children: []*model.ResourceNode{
					{Name: "child" + payload, Kind: "Container", Namespace: "other-ns" + payload, Status: "Ready" + payload},
				},
			}
			out := RenderResourceTree(root, 120, 30)
			assertResourceClean(t, out)
		})
	}
}

// The preview pane deliberately does NOT keep SGR. Cluster metadata has no
// business colouring the pane, and keeping the escape is what let the rune and
// byte slicing further down count escape bytes as visible width, which broke
// alignment in a narrow pane. The escape is replaced with U+FFFD, the same
// marker the log viewer shows for any other non-printable byte, so the value
// reads as suspect instead of being obeyed.
//
// Asserting on "[31m" alone would prove nothing: that printable tail survives
// either way. The ESC is what matters.
func TestPreviewBodyPathsDropTheEscape(t *testing.T) {
	const red = "\x1b[31mFAILED\x1b[0m"
	const escSGR = "\x1b[31m"

	cases := map[string]*model.Item{
		"condition message": {
			Name:       "deploy-1",
			Columns:    []model.KeyValue{{Key: "Node", Value: "node-1"}},
			Conditions: []model.ConditionEntry{{Type: "Available", Status: "False", Message: red}},
		},
		"Labels entry": {
			Name:    "svc-1",
			Columns: []model.KeyValue{{Key: "Labels", Value: red}},
		},
	}

	for name, item := range cases {
		t.Run(name, func(t *testing.T) {
			out := RenderResourceSummary(item, "", 100, 40)
			assert.NotContains(t, out, escSGR, "the escape must not reach the pane")
			assert.Contains(t, out, "\ufffd", "and the byte it stood for is marked, not dropped silently")
		})
	}
}
