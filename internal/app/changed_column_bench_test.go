package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

// benchItems builds a list the size of a busy cluster. Half the rows resolve
// through conditions, half fall through to the managedFields parse, which is
// the expensive path.
func benchItems(n int) []model.Item {
	now := time.Now().UTC()
	items := make([]model.Item, 0, n)
	for i := range n {
		it := model.Item{Name: fmt.Sprintf("row-%d", i), Namespace: "prod", Kind: "Pod"}
		if i%2 == 0 {
			it.Conditions = []model.ConditionEntry{
				{Type: "Ready", LastTransitionTime: now.Add(-time.Hour)},
				{Type: "Initialized", LastTransitionTime: now.Add(-2 * time.Hour)},
				{Type: "PodScheduled", LastTransitionTime: now.Add(-3 * time.Hour)},
			}
		} else {
			entries := make([]any, 0, 4)
			for j := range 4 {
				entries = append(entries, map[string]any{
					"manager": "kubelet",
					"time":    now.Add(-time.Duration(j) * time.Minute).Format(time.RFC3339),
				})
			}
			it.Raw = map[string]any{"metadata": map[string]any{"managedFields": entries}}
		}
		items = append(items, it)
	}
	return items
}

func BenchmarkApplyChangedColumn(b *testing.B) {
	for _, n := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			items := benchItems(n)
			now := time.Now()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				applyChangedColumn(items, now)
			}
		})
	}
}
