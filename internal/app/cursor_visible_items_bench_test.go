package app

import (
	"fmt"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
)

// BenchmarkVisibleMiddleItemsUnchanged simulates repeated render-loop calls
// with the same filter/nav state each time, the case a memo should serve
// from cache instead of re-filtering, re-scoring, and re-collapsing.
func BenchmarkVisibleMiddleItemsUnchanged(b *testing.B) {
	b.Run("filtered", func(b *testing.B) {
		for _, n := range []int{1000, 5000} {
			b.Run(fmt.Sprintf("items=%d", n), func(b *testing.B) {
				m := newTestModel()
				m.setMiddleItems(benchItems(n))
				m.filterText = "row-1"
				m.filterBroadMode = true
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					_ = m.visibleMiddleItems()
				}
			})
		}
	})

	b.Run("resourceTypesCollapse", func(b *testing.B) {
		for _, n := range []int{1000, 5000} {
			b.Run(fmt.Sprintf("items=%d", n), func(b *testing.B) {
				m := newTestModel()
				m.nav.Level = model.LevelResourceTypes
				items := benchItems(n)
				categories := []string{"Workloads", "Networking", "Storage"}
				for i := range items {
					items[i].Category = categories[i%len(categories)]
				}
				m.setMiddleItems(items)
				m.expandedGroup = "Workloads"
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					_ = m.visibleMiddleItems()
				}
			})
		}
	})
}
