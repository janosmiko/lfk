package k8s

import (
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/janosmiko/lfk/internal/model"
)

// RescaleRightsizing returns a NEW Rightsizing whose recommendations
// are derived from `data` by re-applying the headroom: every
// RecommendedRequest / RecommendedLimit gets multiplied by
// newHeadroom / data.Headroom and re-snapped to canonical k8s units.
// Usage, CurrentRequest, CurrentLimit, LowerBound, UpperBound are
// untouched — they are observed/spec/VPA values, not headroom-derived.
//
// Pure local arithmetic with no API calls. The headroom picker
// (</>) uses this so flipping headroom doesn't kick a re-fetch and
// the table never flashes through "Computing right-sizing…".
//
// Backward compat: returns nil when data == nil; returns the input
// pointer unchanged when data.Headroom == 0 (legacy / not set) —
// better to do nothing than mis-scale by an unknown ratio. Mutation
// safety: when scaling does happen, a NEW *model.Rightsizing is
// returned (containers slice copied) so the input is never modified —
// callers cache by pointer and would otherwise see stale entries
// silently mutate.
func RescaleRightsizing(data *model.Rightsizing, newHeadroom float64) *model.Rightsizing {
	if data == nil {
		return nil
	}
	if data.Headroom == 0 {
		// Unknown source headroom — refuse to guess a ratio. Returning the
		// input unchanged matches "no-op" semantics callers can detect by
		// pointer identity.
		return data
	}
	if newHeadroom == data.Headroom {
		// No-op rescale: still return a fresh pointer with the same
		// headroom so callers can write it into the cache under the new
		// key without aliasing the source entry. Container slice is
		// copied for safety even though values don't change.
		out := *data
		out.Containers = append([]model.ContainerRec(nil), data.Containers...)
		return &out
	}

	ratio := newHeadroom / data.Headroom
	out := *data
	out.Headroom = newHeadroom
	out.Containers = make([]model.ContainerRec, len(data.Containers))
	for i, c := range data.Containers {
		nc := c
		nc.CPU = rescaleResourceRec(c.CPU, ratio, false)
		nc.Mem = rescaleResourceRec(c.Mem, ratio, true)
		out.Containers[i] = nc
	}
	return &out
}

// rescaleResourceRec returns a NEW ResourceRec with RecommendedRequest
// and RecommendedLimit scaled by `ratio`. All other fields (Usage,
// CurrentRequest, CurrentLimit, LowerBound, UpperBound) are copied
// verbatim — they describe observed / spec / VPA-confidence values
// that are not derived from headroom.
//
// `isMemory` is threaded explicitly (rather than inferred from the
// quantity suffix) because canonical memory values can be plain byte
// counts with no unit suffix — sniffing the string would mis-scale a
// suffix-free memory recommendation as a CPU one and emit "200m"
// where "200Mi" was meant.
func rescaleResourceRec(r model.ResourceRec, ratio float64, isMemory bool) model.ResourceRec {
	out := r
	out.RecommendedRequest = scaleQuantityByRatio(r.RecommendedRequest, ratio, isMemory)
	out.RecommendedLimit = scaleQuantityByRatio(r.RecommendedLimit, ratio, isMemory)
	return out
}

// scaleQuantityByRatio multiplies a k8s canonical quantity string
// (e.g. "100m", "200Mi") by `ratio` and snaps back to canonical form.
// Returns the input unchanged when ratio == 1 (no work needed) or
// when the string is empty / fails to parse — defensive passthrough
// matches scaleQuantityByHeadroom's contract for the same edge cases.
//
// `isMemory` selects the snap target (SnapMemBytesToCanonical vs
// SnapCPUMilliToCanonical). Threaded from the resource-kind known to
// the caller rather than inferred from the suffix because a canonical
// memory quantity can be a plain byte count with no suffix at all
// (resource.Quantity.String() emits "1024" for small byte values),
// in which case suffix sniffing would silently mis-route to the CPU
// snapper and emit "1024m" — visibly wrong.
//
// Lives separate from scaleQuantityByHeadroom because the rescale
// caller already computed a new/old ratio and shouldn't have to fake
// up a "headroom" parameter to reuse the existing helper.
func scaleQuantityByRatio(q string, ratio float64, isMemory bool) string {
	if q == "" || ratio == 1 {
		return q
	}
	parsed, err := resource.ParseQuantity(q)
	if err != nil {
		return q
	}
	if isMemory {
		// MilliValue() for memory returns bytes×1000; convert back to
		// bytes before scaling so SnapMemBytesToCanonical sees the right
		// unit.
		bytes := parsed.MilliValue() / 1000
		return SnapMemBytesToCanonical(int64(float64(bytes) * ratio))
	}
	return SnapCPUMilliToCanonical(int64(float64(parsed.MilliValue()) * ratio))
}
