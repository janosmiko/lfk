package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- renderKV ---

func TestRenderKV(t *testing.T) {
	t.Run("short value fits", func(t *testing.T) {
		result := renderKV("Node", "worker-1", 60)
		assert.Contains(t, result, "Node:")
		assert.Contains(t, result, "worker-1")
	})

	t.Run("long value truncated with ellipsis", func(t *testing.T) {
		result := renderKV("Labels", "this-is-a-very-long-label-value-that-exceeds-the-width", 30)
		assert.Contains(t, result, "Labels:")
		assert.Contains(t, result, "...")
	})
}

// --- renderDataKV ---

func TestRenderDataKV(t *testing.T) {
	t.Run("single line value", func(t *testing.T) {
		lines := renderDataKV("config.yaml", "value=123", 60)
		assert.Len(t, lines, 1)
		assert.Contains(t, lines[0], "config.yaml:")
		assert.Contains(t, lines[0], "value=123")
	})

	t.Run("multiline value returns multiple lines", func(t *testing.T) {
		lines := renderDataKV("config", "line1\nline2\nline3", 60)
		assert.Greater(t, len(lines), 1)
		assert.Contains(t, lines[0], "config:")
	})

	t.Run("escaped newlines are expanded", func(t *testing.T) {
		lines := renderDataKV("key", `first\nsecond`, 60)
		assert.Greater(t, len(lines), 1)
	})
}

// --- renderUsageBar ---

func TestRenderUsageBar(t *testing.T) {
	t.Run("zero reference shows just value", func(t *testing.T) {
		result := renderUsageBar(500, 0, 0, 40, FormatCPU)
		assert.Equal(t, "500m", result)
	})

	t.Run("with limit shows bar and percentage", func(t *testing.T) {
		result := renderUsageBar(500, 0, 1000, 60, FormatCPU)
		assert.Contains(t, result, "500m")
		assert.Contains(t, result, "1.0")
		assert.Contains(t, result, "50%")
		assert.Contains(t, result, "[")
		assert.Contains(t, result, "]")
	})

	t.Run("with request as fallback reference", func(t *testing.T) {
		result := renderUsageBar(250, 500, 0, 60, FormatCPU)
		assert.Contains(t, result, "250m")
		assert.Contains(t, result, "50%")
	})

	t.Run("memory usage bar", func(t *testing.T) {
		result := renderUsageBar(512*1024*1024, 0, 1024*1024*1024, 60, FormatMemory)
		assert.Contains(t, result, "512Mi")
		assert.Contains(t, result, "1.0Gi")
		assert.Contains(t, result, "50%")
	})

	t.Run("over 100 percent capped at 100", func(t *testing.T) {
		result := renderUsageBar(2000, 0, 1000, 60, FormatCPU)
		assert.Contains(t, result, "100%")
	})
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
		assert.Contains(t, result, "config.yaml")
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
}
