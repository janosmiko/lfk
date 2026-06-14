package gatekeeper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/security"
)

// newConstraintTemplateDynClient builds a fake dynamic client that knows
// the list kind for both ConstraintTemplate API versions.
func newConstraintTemplateDynClient() *dynfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	listKinds := make(map[schema.GroupVersionResource]string, len(constraintTemplateGVRs))
	for _, gvr := range constraintTemplateGVRs {
		listKinds[gvr] = "ConstraintTemplateList"
	}
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
}

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

// TestIsAvailableFallsBackToV1beta1 — on Gatekeeper releases older than
// v3.10 the cluster serves ConstraintTemplate as v1beta1 only. The probe
// must fall back to v1beta1 after v1 returns NotFound rather than
// reporting Gatekeeper unavailable.
func TestIsAvailableFallsBackToV1beta1(t *testing.T) {
	dc := newConstraintTemplateDynClient()
	dc.PrependReactor("list", "constrainttemplates",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			if action.GetResource().Version == "v1" {
				return true, nil, apierrors.NewNotFound(
					action.GetResource().GroupResource(), "")
			}
			// v1beta1: defer to the tracker, which returns an empty list.
			return false, nil, nil
		})

	s := NewWithClients(nil, dc)
	ok, err := s.IsAvailable(t.Context(), "ctx")
	require.NoError(t, err)
	assert.True(t, ok, "v1 NotFound must fall back to v1beta1")
}

// TestIsAvailableBothVersionsNotFound — when neither ConstraintTemplate
// version is served the probe reports a definitive "not installed".
func TestIsAvailableBothVersionsNotFound(t *testing.T) {
	dc := newConstraintTemplateDynClient()
	dc.PrependReactor("list", "constrainttemplates",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(
				action.GetResource().GroupResource(), "")
		})

	s := NewWithClients(nil, dc)
	ok, err := s.IsAvailable(t.Context(), "ctx")
	require.NoError(t, err)
	assert.False(t, ok)
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

	kinds, err := discoverConstraintKinds(t.Context(), clientset)
	require.NoError(t, err, "NotFound must NOT propagate as an error")
	assert.Empty(t, kinds)
}
