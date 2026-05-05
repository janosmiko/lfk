package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// TestRescaleRightsizing_HeadroomDoublesRecommendations verifies the
// happy-path: existing recommendations at headroom 1.25 multiplied by
// 2.0/1.25 = 1.6 ratio land on the expected canonical values, and the
// untouched columns (Usage/Current/LowerBound/UpperBound) come through
// unchanged so the renderer doesn't mistake observed/spec values for
// padded ones.
func TestRescaleRightsizing_HeadroomDoublesRecommendations(t *testing.T) {
	src := &model.Rightsizing{
		Headroom: 1.25,
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU: model.ResourceRec{
				Usage:              "80m",
				CurrentRequest:     "100m",
				CurrentLimit:       "500m",
				RecommendedRequest: "100m",
				RecommendedLimit:   "200m",
				LowerBound:         "50m",
				UpperBound:         "150m",
			},
		}},
	}

	out := RescaleRightsizing(src, 2.0)
	require.NotNil(t, out)
	assert.InDelta(t, 2.0, out.Headroom, 1e-9)
	require.Len(t, out.Containers, 1)
	rec := out.Containers[0]

	// Ratio 2.0/1.25 = 1.6 → 100m*1.6 = 160m, 200m*1.6 = 320m
	assert.Equal(t, "160m", rec.CPU.RecommendedRequest, "rec request scales by newHeadroom/oldHeadroom")
	assert.Equal(t, "320m", rec.CPU.RecommendedLimit, "rec limit scales by newHeadroom/oldHeadroom")
	// Untouched columns stay verbatim.
	assert.Equal(t, "80m", rec.CPU.Usage, "Usage is observed, not headroom-derived")
	assert.Equal(t, "100m", rec.CPU.CurrentRequest, "CurrentRequest is spec, not headroom-derived")
	assert.Equal(t, "500m", rec.CPU.CurrentLimit, "CurrentLimit is spec, not headroom-derived")
	assert.Equal(t, "50m", rec.CPU.LowerBound, "LowerBound is VPA confidence, not headroom-derived")
	assert.Equal(t, "150m", rec.CPU.UpperBound, "UpperBound is VPA confidence, not headroom-derived")
}

// TestRescaleRightsizing_NoOpWhenSameHeadroom guards against pointless
// re-snapping. Same-headroom rescales return a fresh struct (so cache
// callers can write under a new key without aliasing) but the values
// are byte-identical to the input.
func TestRescaleRightsizing_NoOpWhenSameHeadroom(t *testing.T) {
	src := &model.Rightsizing{
		Headroom: 1.25,
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU:  model.ResourceRec{RecommendedRequest: "100m", RecommendedLimit: "200m"},
		}},
	}
	out := RescaleRightsizing(src, 1.25)
	require.NotNil(t, out)
	assert.InDelta(t, 1.25, out.Headroom, 1e-9)
	require.Len(t, out.Containers, 1)
	assert.Equal(t, "100m", out.Containers[0].CPU.RecommendedRequest)
	assert.Equal(t, "200m", out.Containers[0].CPU.RecommendedLimit)
}

// TestRescaleRightsizing_NilReturnsNil keeps callers safe — rescaling
// before any data is loaded is a legal no-op, not a panic.
func TestRescaleRightsizing_NilReturnsNil(t *testing.T) {
	out := RescaleRightsizing(nil, 1.5)
	assert.Nil(t, out, "nil input must not panic and must return nil")
}

// TestRescaleRightsizing_ZeroHeadroomReturnsUnchanged covers the
// legacy/back-compat path: when the source data has no headroom
// recorded (e.g. a fixture from before the field existed), rescaling
// would have to invent a ratio. Instead the input is returned
// unchanged so the caller sees the no-op and falls back to a real
// fetch if it needs the new headroom applied.
func TestRescaleRightsizing_ZeroHeadroomReturnsUnchanged(t *testing.T) {
	src := &model.Rightsizing{
		Headroom: 0, // legacy / not set
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU:  model.ResourceRec{RecommendedRequest: "100m"},
		}},
	}
	out := RescaleRightsizing(src, 1.5)
	assert.Same(t, src, out, "data.Headroom == 0 returns input pointer unchanged (no scale)")
}

// TestRescaleRightsizing_SnapsToCanonical guards against floating-
// point noise leaking into the table. "100m" * 2 = 200m (not "200.0m"
// or similar) — the snap helpers floor to canonical k8s units the
// renderer recognises.
func TestRescaleRightsizing_SnapsToCanonical(t *testing.T) {
	src := &model.Rightsizing{
		Headroom: 1.0,
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU:  model.ResourceRec{RecommendedRequest: "100m"},
			Mem:  model.ResourceRec{RecommendedRequest: "100Mi"},
		}},
	}
	out := RescaleRightsizing(src, 2.0)
	require.NotNil(t, out)
	require.Len(t, out.Containers, 1)
	assert.Equal(t, "200m", out.Containers[0].CPU.RecommendedRequest, "CPU snaps to canonical 'm' suffix")
	assert.Equal(t, "200Mi", out.Containers[0].Mem.RecommendedRequest, "memory snaps to canonical 'Mi' suffix")
}

// TestRescaleRightsizing_DoesNotMutateInput ensures cache safety —
// callers cache by pointer and a mutating rescale would silently
// poison every reader of the cached entry.
func TestRescaleRightsizing_DoesNotMutateInput(t *testing.T) {
	src := &model.Rightsizing{
		Headroom: 1.25,
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU:  model.ResourceRec{RecommendedRequest: "100m", RecommendedLimit: "500m"},
		}},
	}
	_ = RescaleRightsizing(src, 2.0)
	assert.InDelta(t, 1.25, src.Headroom, 1e-9, "source headroom unchanged after rescale")
	assert.Equal(t, "100m", src.Containers[0].CPU.RecommendedRequest, "source RecommendedRequest unchanged")
	assert.Equal(t, "500m", src.Containers[0].CPU.RecommendedLimit, "source RecommendedLimit unchanged")
}

// TestRescaleRightsizing_SuffixLessMemoryStaysMemory guards against
// a regression where scaleQuantityByRatio inferred CPU vs memory from
// the quantity suffix. Canonical memory quantities can be plain byte
// counts ("1024") with no suffix at all — sniffing the string
// silently mis-routed those to SnapCPUMilliToCanonical and emitted
// "1024m" (CPU shape) for a memory recommendation.
//
// The fix threads the resource kind through rescaleResourceRec so
// memory always snaps to canonical Mi/Gi units regardless of the
// input string's shape.
func TestRescaleRightsizing_SuffixLessMemoryStaysMemory(t *testing.T) {
	src := &model.Rightsizing{
		Headroom: 1.0,
		Containers: []model.ContainerRec{{
			Name: "app",
			Mem:  model.ResourceRec{RecommendedRequest: "1024", RecommendedLimit: "2048"},
		}},
	}
	out := RescaleRightsizing(src, 2.0)
	require.NotNil(t, out)
	require.Len(t, out.Containers, 1)
	rec := out.Containers[0].Mem.RecommendedRequest
	// Whatever the snap target rounds to, it must use a memory unit
	// suffix (Mi/Gi/...) — never the CPU "m" suffix.
	assert.NotContains(t, rec, "m", "memory recommendation must never get a CPU 'm' suffix from rescaling")
	assert.Contains(t, rec, "Mi", "memory rescale must snap to canonical Mi units")
}

// TestRescaleRightsizing_EmptyRecommendationsLeftEmpty covers the no-
// metrics case: containers without recommendations stay without
// recommendations after a rescale (no surprise "0m" or similar
// generated from empty input).
func TestRescaleRightsizing_EmptyRecommendationsLeftEmpty(t *testing.T) {
	src := &model.Rightsizing{
		Headroom: 1.25,
		Containers: []model.ContainerRec{{
			Name: "no-metrics",
			CPU:  model.ResourceRec{CurrentRequest: "100m"},
			Mem:  model.ResourceRec{CurrentRequest: "256Mi"},
		}},
	}
	out := RescaleRightsizing(src, 2.0)
	require.NotNil(t, out)
	require.Len(t, out.Containers, 1)
	assert.Empty(t, out.Containers[0].CPU.RecommendedRequest, "empty recommendation stays empty")
	assert.Empty(t, out.Containers[0].CPU.RecommendedLimit, "empty recommendation stays empty")
	assert.Empty(t, out.Containers[0].Mem.RecommendedRequest, "empty recommendation stays empty")
	// Current values come through untouched.
	assert.Equal(t, "100m", out.Containers[0].CPU.CurrentRequest)
	assert.Equal(t, "256Mi", out.Containers[0].Mem.CurrentRequest)
}
