package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
