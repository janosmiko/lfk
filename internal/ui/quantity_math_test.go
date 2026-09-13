package ui

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// SnapCPU rounds CPU millicores UP to the nearest 10m and returns the
// canonical k8s string. Inputs >= 1000m are formatted as whole-core
// ("1", "2"); anything below uses the millicore suffix ("80m", "1240m").
func SnapCPU(milli int64) string {
	if milli <= 0 {
		return "0"
	}
	tens := milli / 10
	if milli%10 != 0 {
		tens++
	}
	// tens*10 is formatted, never computed, so it can't overflow near MaxInt64.
	if tens >= 100 && tens%100 == 0 {
		return fmt.Sprintf("%d", tens/100)
	}
	return fmt.Sprintf("%d0m", tens)
}

// SnapMem rounds memory bytes UP to the nearest Mi and returns the
// canonical k8s string ("Mi" suffix). Anything below 1Mi snaps up to "1Mi".
func SnapMem(bytes int64) string {
	if bytes <= 0 {
		return "0"
	}
	const mi = 1024 * 1024
	mibs := (bytes-1)/mi + 1
	return fmt.Sprintf("%dMi", mibs)
}

func TestSnapCPU(t *testing.T) {
	cases := []struct {
		name string
		in   int64 // millicores
		want string
	}{
		{"zero", 0, "0"},
		{"rounds up to 80m", 73, "80m"},
		{"already snapped", 80, "80m"},
		{"rounds up across boundary", 1234, "1240m"},
		{"999m snaps to 1000m → whole core", 999, "1"},
		{"exactly 1000m → whole core", 1000, "1"},
		{"2000m → whole-core 2", 2000, "2"},
		{"2500m stays millicore", 2500, "2500m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SnapCPU(tc.in))
		})
	}
}

func TestSnapCPU_NearMaxInt64DoesNotOverflow(t *testing.T) {
	got := SnapCPU(math.MaxInt64)
	assert.NotContains(t, got, "-", "SnapCPU near MaxInt64 wrapped negative: %s", got)
}

func TestSnapMem(t *testing.T) {
	cases := []struct {
		name string
		in   int64 // bytes
		want string
	}{
		{"zero", 0, "0"},
		{"245.99Mi rounds up to 246Mi", 257949696, "246Mi"},
		{"sub-1Mi snaps to 1Mi", 262144, "1Mi"},
		{"exactly 256Mi", 268435456, "256Mi"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SnapMem(tc.in))
		})
	}
}

func TestSnapMem_NearMaxInt64DoesNotOverflow(t *testing.T) {
	got := SnapMem(math.MaxInt64)
	assert.NotContains(t, got, "-", "SnapMem near MaxInt64 wrapped negative: %s", got)
}

func TestDeltaPercent(t *testing.T) {
	t.Run("over-provisioned (negative)", func(t *testing.T) {
		pct, ok := DeltaPercent("100m", "60m")
		assert.True(t, ok)
		assert.InDelta(t, -40.0, pct, 0.5)
	})
	t.Run("under-provisioned (positive)", func(t *testing.T) {
		pct, ok := DeltaPercent("64Mi", "80Mi")
		assert.True(t, ok)
		assert.InDelta(t, 25.0, pct, 0.5)
	})
	t.Run("equal", func(t *testing.T) {
		pct, ok := DeltaPercent("50m", "50m")
		assert.True(t, ok)
		assert.InDelta(t, 0.0, pct, 0.001)
	})
	t.Run("missing current → not ok", func(t *testing.T) {
		_, ok := DeltaPercent("", "60m")
		assert.False(t, ok)
	})
	t.Run("missing recommended → not ok", func(t *testing.T) {
		_, ok := DeltaPercent("100m", "")
		assert.False(t, ok)
	})
	t.Run("zero current → not ok (avoids div/0)", func(t *testing.T) {
		_, ok := DeltaPercent("0", "60m")
		assert.False(t, ok)
	})
}
