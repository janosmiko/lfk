package security

import (
	"context"

	"github.com/janosmiko/lfk/internal/logger"
)

// maxLabelLookups caps the resolver Gets one scan performs so a huge cluster
// with a label-match pattern can't spend its whole scan budget resolving
// labels. Resolution is deduplicated per resource, so the cap counts distinct
// resources, not findings. Exceeding it is logged, never silent.
const maxLabelLookups = 300

// resolveWorkloadLabels fills in ResourceRef.Labels for findings still missing
// them after propagation, using the injected resolver (typically the live
// object's labels via the throttled security client). No-op when no resolver is
// installed — the app installs one only when label-match ignore patterns exist.
// Lookups are deduplicated by resource key and capped at maxLabelLookups.
func (m *Manager) resolveWorkloadLabels(ctx context.Context, kubeCtx string, findings []Finding) {
	resolver := m.labelResolverFn()
	if resolver == nil {
		return
	}
	memo := make(map[string]map[string]string)
	lookups, skipped := 0, 0
	for i := range findings {
		if len(findings[i].Resource.Labels) > 0 {
			continue
		}
		ref := findings[i].Resource
		// Skip refs the resolver is guaranteed to reject. The mapped kinds are
		// all namespaced, so a namespace-less ref (e.g. a cluster-scoped RBAC
		// finding) resolves to nil. Skipping here keeps such refs from burning
		// the lookup budget and starving resolvable workloads later in the scan.
		if ref.Kind == "" || ref.Name == "" || ref.Namespace == "" {
			continue
		}
		key := ref.Key()
		lbls, seen := memo[key]
		if !seen {
			if lookups >= maxLabelLookups {
				memo[key] = nil // record so the same resource isn't recounted
				skipped++
				continue
			}
			lookups++
			lbls = resolver(ctx, kubeCtx, ref.Namespace, ref.Kind, ref.Name)
			memo[key] = lbls
		}
		if len(lbls) > 0 {
			findings[i].Resource.Labels = lbls
		}
	}
	if skipped > 0 {
		logger.Warn("security label resolution capped",
			"limit", maxLabelLookups, "skipped_resources", skipped)
	}
}

// propagateResourceLabels fills in ResourceRef.Labels for findings whose source
// did not expose them, copying from same-resource findings that did. Matched by
// ResourceRef.Key() (namespace/kind/name), so a label-match ignore pattern
// reaches labelless sources (trivy, kyverno) once another source (heuristic)
// observed the same resource.
//
// Runs in O(n) with one index map. The shared label maps are read-only so
// aliasing them across findings is safe.
func propagateResourceLabels(findings []Finding) {
	// First non-empty label map per resource wins. Later ones are not merged.
	// Today only the heuristic source stamps labels, so a key has at most one
	// authority. A future multi-stamping source would need merge logic here.
	labelsByKey := make(map[string]map[string]string)
	for i := range findings {
		if len(findings[i].Resource.Labels) == 0 {
			continue
		}
		key := findings[i].Resource.Key()
		if _, ok := labelsByKey[key]; !ok {
			labelsByKey[key] = findings[i].Resource.Labels
		}
	}
	if len(labelsByKey) == 0 {
		return
	}
	for i := range findings {
		if len(findings[i].Resource.Labels) > 0 {
			continue
		}
		if lbls, ok := labelsByKey[findings[i].Resource.Key()]; ok {
			findings[i].Resource.Labels = lbls
		}
	}
}
