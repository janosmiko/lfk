package ui

import (
	"fmt"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
)

// renderTableBenchItems mixes categories and ClusterName values so the
// benchmark exercises RenderTable's hasUnion and category/separator scans.
func renderTableBenchItems(n int) []model.Item {
	categories := []string{"Workloads", "Networking", "Storage", "Config"}
	items := make([]model.Item, 0, n)
	for i := range n {
		it := model.Item{
			Name:      fmt.Sprintf("row-%d", i),
			Namespace: "prod",
			Kind:      "Pod",
			Status:    "Running",
			Age:       "5m",
			Category:  categories[i%len(categories)],
		}
		if i%10 == 0 {
			it.ClusterName = "cluster-a"
		}
		items = append(items, it)
	}
	return items
}

// BenchmarkRenderTableUnchanged simulates the spinner-tick render loop:
// repeated Render calls with an unchanged items slice and fingerprint, the
// case that should do near-zero per-item work once the layout is cached.
func BenchmarkRenderTableUnchanged(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		b.Run(fmt.Sprintf("items=%d", n), func(b *testing.B) {
			items := renderTableBenchItems(n)
			r := NewTableRenderer()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				_ = r.Render("NAME", items, 0, 120, 40, false, "", "", 0, 0)
			}
		})
	}
}
