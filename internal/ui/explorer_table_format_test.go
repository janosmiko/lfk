package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// tableFormatHostilePayloads mirrors the payload set used across the other
// TASK-880 sanitization tests: a bidi override, a raw CSI sequence, and an
// OSC-52 clipboard write.
var tableFormatHostilePayloads = map[string]string{
	"bidi override": "ab\u202ecd",
	"raw CSI":       "ab\x1b[2Jcd",
	"OSC-52":        "ab\x1b]52;c;aGF4\x07cd",
}

func assertTableFormatClean(t *testing.T, out string) {
	t.Helper()
	assert.NotContains(t, out, "\u202e")
	assert.NotContains(t, out, "\x1b[2J")
	assert.NotContains(t, out, "\x1b]52")
}

// TestRenderContainerDetail_SanitizesFields verifies that every
// cluster-controlled field shown in the container detail pane — name,
// status, image, ready, restarts, and any extra column — is sanitized
// before it reaches the rendered output.
func TestRenderContainerDetail_SanitizesFields(t *testing.T) {
	for name, payload := range tableFormatHostilePayloads {
		t.Run(name, func(t *testing.T) {
			item := &model.Item{
				Name:     "container" + payload,
				Status:   "Running" + payload,
				Extra:    "nginx:latest" + payload,
				Ready:    "1/1" + payload,
				Restarts: "0" + payload,
				Columns: []model.KeyValue{
					{Key: "Node" + payload, Value: "worker-1" + payload},
				},
			}
			out := RenderContainerDetail(item, 120, 30)
			assertTableFormatClean(t, out)
		})
	}
}

// TestRenderContainerDetail_OrdinaryValuesUnchanged is the control case: a
// container with no hostile input renders its fields as plain text.
func TestRenderContainerDetail_OrdinaryValuesUnchanged(t *testing.T) {
	item := &model.Item{
		Name:     "nginx",
		Status:   "Running",
		Extra:    "nginx:1.27",
		Ready:    "1/1",
		Restarts: "0",
	}
	out := RenderContainerDetail(item, 120, 30)
	plain := stripANSI(out)
	assert.Contains(t, plain, "nginx")
	assert.Contains(t, plain, "Running")
	assert.Contains(t, plain, "nginx:1.27")
	assert.Contains(t, plain, "1/1")
}

// Two Update handlers each prepend their own block to item.Columns without
// knowing about the other: the watch-tick relist ends up with Changed ahead of
// CPU and MEM, while an async metrics reply puts CPU and MEM ahead of Changed.
// Whichever ran last decides the physical order, so a renderer that walks
// Columns as-is makes those rows swap places every few seconds. The list table
// is immune because it orders on the KEY via canonicalColumnPriority. This
// panel must do the same.
func TestRenderContainerDetail_OrderIsIndependentOfColumnPosition(t *testing.T) {
	cols := func(kv ...model.KeyValue) []model.KeyValue { return kv }
	changed := model.KeyValue{Key: "Changed", Value: "5m"}
	cpu := model.KeyValue{Key: "CPU", Value: "50m"}
	mem := model.KeyValue{Key: "MEM", Value: "64Mi"}
	req := model.KeyValue{Key: "CPU Request", Value: "100m"}

	afterRelist := model.Item{Name: "app", Columns: cols(changed, cpu, mem, req)}
	afterMetrics := model.Item{Name: "app", Columns: cols(cpu, mem, changed, req)}

	got := RenderContainerDetail(&afterRelist, 60, 20)
	want := RenderContainerDetail(&afterMetrics, 60, 20)

	assert.Equal(t, want, got,
		"the same fields in a different physical order must render identically")
}
