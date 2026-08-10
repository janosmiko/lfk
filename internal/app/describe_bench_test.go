package app

import (
	"fmt"
	"strings"
	"testing"
)

// makeDescribeBenchContent synthesizes describe-style content (tab-aligned
// key/value pairs, matching real kubectl describe output) at the given
// line count, for BenchmarkViewDescribe.
func makeDescribeBenchContent(lines int) string {
	var b strings.Builder
	for i := range lines {
		fmt.Fprintf(&b, "Field%d:\tvalue-%d\n", i, i)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// BenchmarkViewDescribe measures the per-frame cost of viewDescribe after
// moving sanitizing to the five content producers (see
// describe_sanitize.go). Before the fix, viewDescribe re-split and
// re-sanitized the whole buffer on every bubbletea render; content here is
// pre-sanitized once via sanitizeDescribeContent, matching what a real
// producer stores, so this measures only the render path's own cost.
func BenchmarkViewDescribe(b *testing.B) {
	for _, lines := range []int{200, 5000, 20000} {
		b.Run(fmt.Sprintf("%d_lines", lines), func(b *testing.B) {
			m := baseModelDescribe()
			m.describeView.content = sanitizeDescribeContent(makeDescribeBenchContent(lines))
			b.ReportAllocs()
			for b.Loop() {
				_ = m.viewDescribe()
			}
		})
	}
}
