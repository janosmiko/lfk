package ui

import (
	"strings"
	"testing"
)

// FuzzParseResourceValue drives the Kubernetes resource-string parser
// (e.g. "100m", "1.5Gi") with arbitrary input. ParseResourceValue swallows
// strconv errors and returns int64 — the fuzz target is panic discovery and
// the empty-input contract (empty → 0 for both isCPU true and false).
func FuzzParseResourceValue(f *testing.F) {
	for _, s := range []string{
		"", "100m", "0", "1", "1.5", "2000",
		"128Mi", "1.5Gi", "1024Ki", "1024B", "512M",
		" 100m ", "abc", "-1", "1e10", ".",
	} {
		f.Add(s, true)
		f.Add(s, false)
	}

	f.Fuzz(func(t *testing.T, val string, isCPU bool) {
		got := ParseResourceValue(val, isCPU)

		if strings.TrimSpace(val) == "" && got != 0 {
			t.Fatalf("ParseResourceValue(%q, %v) = %d; empty input must return 0", val, isCPU, got)
		}
	})
}
