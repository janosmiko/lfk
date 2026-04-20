// Package policyreport reads Kyverno/Policy Reports API CRDs
// (PolicyReport and ClusterPolicyReport from wgpolicyk8s.io/v1alpha2)
// and exposes them as security.Findings.
package policyreport

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/security"
)

// GVRs for the Policy Reports API CRDs.
var (
	PolicyReportGVR = schema.GroupVersionResource{
		Group: "wgpolicyk8s.io", Version: "v1alpha2", Resource: "policyreports",
	}
	ClusterPolicyReportGVR = schema.GroupVersionResource{
		Group: "wgpolicyk8s.io", Version: "v1alpha2", Resource: "clusterpolicyreports",
	}
)

// Source is the policy-report SecuritySource implementation. It reads
// Kyverno PolicyReport and ClusterPolicyReport CRDs and converts
// failing results into security.Findings.
type Source struct {
	client dynamic.Interface
}

// New returns a Source with no client (Fetch returns nil, IsAvailable false).
func New() *Source { return &Source{} }

// NewWithDynamic returns a Source using the given dynamic client.
func NewWithDynamic(client dynamic.Interface) *Source {
	return &Source{client: client}
}

// Name returns the stable identifier.
func (s *Source) Name() string { return "policy-report" }

// Categories returns the categories this source produces.
func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryPolicy, security.CategoryCompliance}
}

// IsAvailable checks that the PolicyReport CRD is served by the API.
func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	if s.client == nil {
		return false, nil
	}
	_, err := s.client.Resource(PolicyReportGVR).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return false, fmt.Errorf("policy-report availability probe: %w", err)
	}
	return true, nil
}

// Fetch lists PolicyReport and ClusterPolicyReport CRDs and returns
// failing results as findings. Passing results are skipped.
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	if s.client == nil {
		return nil, nil
	}

	var findings []security.Finding

	// Namespaced PolicyReports.
	prList, err := s.client.Resource(PolicyReportGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list policy reports: %w", err)
	}
	for i := range prList.Items {
		findings = append(findings, parsePolicyReport(&prList.Items[i])...)
	}

	// ClusterPolicyReports (cluster-scoped, only when namespace is empty).
	if namespace == "" {
		cprList, err := s.client.Resource(ClusterPolicyReportGVR).List(ctx, metav1.ListOptions{})
		if err != nil {
			// ClusterPolicyReport CRD may not exist — non-fatal.
			return findings, nil //nolint:nilerr // optional CRD, not an error
		}
		for i := range cprList.Items {
			findings = append(findings, parsePolicyReport(&cprList.Items[i])...)
		}
	}

	return findings, nil
}
