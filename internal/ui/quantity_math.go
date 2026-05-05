package ui

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

// SnapCPU rounds CPU millicores UP to the nearest 10m and returns
// the canonical k8s string. Used by the metrics-server fallback path
// in right-sizing recommendations so `usage × 1.2` doesn't produce
// ugly values like "73.2m".
//
// Inputs ≥ 1000m are formatted as whole-core ("1", "2"); anything
// below uses the millicore suffix ("80m", "1240m").
func SnapCPU(milli int64) string {
	if milli <= 0 {
		return "0"
	}
	snapped := ((milli + 9) / 10) * 10
	if snapped >= 1000 && snapped%1000 == 0 {
		return fmt.Sprintf("%d", snapped/1000)
	}
	return fmt.Sprintf("%dm", snapped)
}

// SnapMem rounds memory bytes UP to the nearest Mi and returns the
// canonical k8s string ("Mi" suffix). Anything below 1Mi snaps up
// to "1Mi" so we never recommend zero memory.
func SnapMem(bytes int64) string {
	if bytes <= 0 {
		return "0"
	}
	const mi = 1024 * 1024
	mibs := (bytes + mi - 1) / mi
	return fmt.Sprintf("%dMi", mibs)
}

// DeltaPercent computes the percentage change from `current` to
// `recommended` for two k8s quantity strings. Returns (pct, true)
// on success; (0, false) when either string is empty or when current
// parses to zero (avoids divide-by-zero).
//
// Sign convention: negative = recommended is smaller (over-provisioned),
// positive = recommended is larger (under-provisioned).
func DeltaPercent(current, recommended string) (float64, bool) {
	if current == "" || recommended == "" {
		return 0, false
	}
	cur, err := resource.ParseQuantity(current)
	if err != nil {
		return 0, false
	}
	rec, err := resource.ParseQuantity(recommended)
	if err != nil {
		return 0, false
	}
	curF := float64(cur.MilliValue())
	if curF == 0 {
		return 0, false
	}
	recF := float64(rec.MilliValue())
	return (recF - curF) / curF * 100, true
}
