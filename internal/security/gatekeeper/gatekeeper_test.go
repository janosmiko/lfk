package gatekeeper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/security"
)

func TestSeverityFromEnforcement(t *testing.T) {
	assert.Equal(t, security.SeverityHigh, severityFromEnforcement("deny"))
	assert.Equal(t, security.SeverityHigh, severityFromEnforcement("DENY"))
	assert.Equal(t, security.SeverityMedium, severityFromEnforcement("warn"))
	assert.Equal(t, security.SeverityLow, severityFromEnforcement("dryrun"))
	assert.Equal(t, security.SeverityMedium, severityFromEnforcement(""))
}

func TestDefaultEnforcementAction(t *testing.T) {
	deny := &unstructured.Unstructured{Object: map[string]any{}}
	assert.Equal(t, "deny", defaultEnforcementAction(deny),
		"empty spec.enforcementAction defaults to deny")

	warn := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{"enforcementAction": "warn"},
	}}
	assert.Equal(t, "warn", defaultEnforcementAction(warn))
}

func TestParseConstraintProducesFindings(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "ns-must-have-gk"},
		"spec":     map[string]any{"enforcementAction": "deny"},
		"status": map[string]any{
			"violations": []any{
				map[string]any{
					"enforcementAction": "deny",
					"kind":              "Namespace",
					"name":              "default",
					"namespace":         "",
					"message":           "you must provide labels",
				},
				map[string]any{
					"kind":      "Namespace",
					"name":      "kube-public",
					"namespace": "",
					"message":   "you must provide labels",
				},
			},
		},
	}}
	u.SetName("ns-must-have-gk")

	findings := parseConstraint(u, "K8sRequiredLabels")
	require.Len(t, findings, 2)

	for _, f := range findings {
		assert.Equal(t, "gatekeeper", f.Source)
		assert.Equal(t, security.CategoryPolicy, f.Category)
		assert.Equal(t, security.SeverityHigh, f.Severity, "deny enforcement → high severity")
		assert.Equal(t, "K8sRequiredLabels", f.Title,
			"Title is the constraint kind so users see groups by policy, not by violating resource")
		assert.Equal(t, "Namespace", f.Resource.Kind)
		assert.Equal(t, "you must provide labels", f.Summary)
		assert.Equal(t, "K8sRequiredLabels", f.Labels["constraint_kind"])
		assert.Equal(t, "ns-must-have-gk", f.Labels["constraint_name"])
		assert.Equal(t, "deny", f.Labels["enforcement_action"])
	}
}

func TestParseConstraintEmptyStatus(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "no-violations"},
	}}
	assert.Empty(t, parseConstraint(u, "K8sRequiredLabels"))
}

// TestSourceWithoutClientsReportsUnavailable — the zero-value Source
// (no clientset wired) must not panic and must report unavailable.
func TestSourceWithoutClientsReportsUnavailable(t *testing.T) {
	s := New()
	ok, err := s.IsAvailable(t.Context(), "kctx")
	assert.False(t, ok)
	assert.NoError(t, err)

	findings, err := s.Fetch(t.Context(), "kctx", "")
	assert.Nil(t, findings)
	assert.NoError(t, err)
}

// TestDiscoverConstraintKindsNotFoundReturnsEmpty — when Gatekeeper's
// constraints.gatekeeper.sh group is not served (webhook installed but
// no ConstraintTemplates registered yet), discovery returns a NotFound
// error. discoverConstraintKinds must treat that as "no kinds to query"
// rather than a hard error so the explorer doesn't spam "could not find
// the requested resource" on every FetchAll cycle.
func TestDiscoverConstraintKindsNotFoundReturnsEmpty(t *testing.T) {
	// FakeDiscovery returns NotFound from ServerResourcesForGroupVersion
	// when the group/version isn't present in Fake.Resources, which is
	// exactly the empty-cluster case we want to exercise.
	clientset := fake.NewSimpleClientset()
	clientset.Resources = []*metav1.APIResourceList{}

	kinds, err := discoverConstraintKinds(context.Background(), clientset)
	require.NoError(t, err, "NotFound must NOT propagate as an error")
	assert.Empty(t, kinds)
}
