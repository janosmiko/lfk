package security

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropagateResourceLabels(t *testing.T) {
	ciliumLabels := map[string]string{"k8s-app": "cilium"}
	findings := []Finding{
		// heuristic finding carries the pod's labels
		{Source: "heuristic", Resource: ResourceRef{Namespace: "kube-system", Kind: "Pod", Name: "cilium-x", Labels: ciliumLabels}},
		// trivy finding on the SAME pod, no labels -> should inherit
		{Source: "trivy-operator", Resource: ResourceRef{Namespace: "kube-system", Kind: "Pod", Name: "cilium-x"}},
		// finding on a different pod, no labels anywhere -> stays empty
		{Source: "trivy-operator", Resource: ResourceRef{Namespace: "default", Kind: "Pod", Name: "web"}},
	}

	propagateResourceLabels(findings)

	assert.Equal(t, ciliumLabels, findings[1].Resource.Labels,
		"trivy finding on the same pod inherits the heuristic-observed labels")
	assert.Nil(t, findings[2].Resource.Labels,
		"a resource no source observed with labels stays empty")
}

// A finding that already carries labels must not be overwritten by another
// finding on the same resource (first non-empty wins, existing labels kept).
func TestPropagateResourceLabels_DoesNotOverwrite(t *testing.T) {
	own := map[string]string{"a": "1"}
	findings := []Finding{
		{Resource: ResourceRef{Kind: "Pod", Name: "p", Labels: own}},
		{Resource: ResourceRef{Kind: "Pod", Name: "p", Labels: map[string]string{"b": "2"}}},
	}
	propagateResourceLabels(findings)
	assert.Equal(t, own, findings[0].Resource.Labels)
	assert.Equal(t, map[string]string{"b": "2"}, findings[1].Resource.Labels)
}

// No labels anywhere is a no-op (and must not panic on nil maps).
func TestPropagateResourceLabels_Empty(t *testing.T) {
	findings := []Finding{{Resource: ResourceRef{Kind: "Pod", Name: "p"}}}
	propagateResourceLabels(findings)
	assert.Nil(t, findings[0].Resource.Labels)
}

func TestResolveWorkloadLabels(t *testing.T) {
	m := NewManager()
	calls := 0
	m.SetLabelResolver(func(_ context.Context, _, namespace, kind, name string) map[string]string {
		calls++
		if kind == "Deployment" && name == "cilium" {
			return map[string]string{"k8s-app": "cilium"}
		}
		return nil
	})

	findings := []Finding{
		// Trivy CVE keyed by a workload, no labels -> resolver fills it.
		{Source: "trivy-operator", Resource: ResourceRef{Namespace: "kube-system", Kind: "Deployment", Name: "cilium"}},
		// Second finding on the SAME workload -> resolver must NOT be called again.
		{Source: "trivy-operator", Resource: ResourceRef{Namespace: "kube-system", Kind: "Deployment", Name: "cilium"}},
		// Already has labels -> skipped entirely.
		{Source: "heuristic", Resource: ResourceRef{Namespace: "default", Kind: "Pod", Name: "web", Labels: map[string]string{"app": "web"}}},
		// Resolver returns nil -> stays empty, no crash.
		{Source: "trivy-operator", Resource: ResourceRef{Namespace: "default", Kind: "Deployment", Name: "other"}},
	}

	m.resolveWorkloadLabels(t.Context(), "ctx", findings)

	assert.Equal(t, map[string]string{"k8s-app": "cilium"}, findings[0].Resource.Labels)
	assert.Equal(t, map[string]string{"k8s-app": "cilium"}, findings[1].Resource.Labels,
		"same-workload finding inherits via memo")
	assert.Equal(t, map[string]string{"app": "web"}, findings[2].Resource.Labels, "pre-labeled finding untouched")
	assert.Nil(t, findings[3].Resource.Labels)
	assert.Equal(t, 2, calls, "resolver deduplicated per resource key (cilium once, other once)")
}

func TestResolveWorkloadLabels_NoResolverIsNoOp(t *testing.T) {
	m := NewManager()
	findings := []Finding{{Resource: ResourceRef{Kind: "Deployment", Name: "x"}}}
	m.resolveWorkloadLabels(t.Context(), "ctx", findings) // resolver nil
	assert.Nil(t, findings[0].Resource.Labels)
}

// Namespace-less refs (e.g. cluster-scoped RBAC findings) must not reach the
// resolver — the mapped kinds are all namespaced, so the lookup is guaranteed
// nil and would only burn the lookup budget.
func TestResolveWorkloadLabels_SkipsNamespacelessRefs(t *testing.T) {
	m := NewManager()
	calls := 0
	m.SetLabelResolver(func(_ context.Context, _, _, _, _ string) map[string]string {
		calls++
		return nil
	})
	findings := []Finding{
		{Source: "rbac", Resource: ResourceRef{Kind: "ClusterRole", Name: "admin"}}, // no namespace
	}
	m.resolveWorkloadLabels(t.Context(), "ctx", findings)
	assert.Equal(t, 0, calls, "namespace-less ref must not consume a lookup")
}

func TestResolveWorkloadLabels_CapsLookups(t *testing.T) {
	m := NewManager()
	calls := 0
	m.SetLabelResolver(func(_ context.Context, _, _, _, _ string) map[string]string {
		calls++
		return nil
	})
	// More distinct resources than the cap; resolver must stop at maxLabelLookups.
	findings := make([]Finding, maxLabelLookups+50)
	for i := range findings {
		findings[i] = Finding{Resource: ResourceRef{Namespace: "default", Kind: "Deployment", Name: fmt.Sprintf("d-%d", i)}}
	}
	m.resolveWorkloadLabels(t.Context(), "ctx", findings)
	assert.Equal(t, maxLabelLookups, calls, "resolver calls capped at maxLabelLookups")
}
