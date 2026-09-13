package ui

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

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
