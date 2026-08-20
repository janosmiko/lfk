// Package trivyop reads Trivy Operator CRDs (VulnerabilityReport and
// ConfigAuditReport) and exposes them as security.Findings.
package trivyop

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/security"
)

// GVRs for the Trivy Operator CRDs Phase 1 consumes.
var (
	VulnerabilityReportGVR = schema.GroupVersionResource{
		Group: "aquasecurity.github.io", Version: "v1alpha1", Resource: "vulnerabilityreports",
	}
	ConfigAuditReportGVR = schema.GroupVersionResource{
		Group: "aquasecurity.github.io", Version: "v1alpha1", Resource: "configauditreports",
	}
)

// Source is the trivy-operator SecuritySource implementation.
type Source struct {
	client dynamic.Interface
}

// NewWithDynamic returns a Source using the given dynamic client.
func NewWithDynamic(client dynamic.Interface) *Source {
	return &Source{client: client}
}

// Name returns the stable identifier.
func (s *Source) Name() string { return "trivy-operator" }

// Categories returns the categories this source produces.
func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryVuln, security.CategoryMisconfig}
}

// IsAvailable checks that the VulnerabilityReport CRD is served by the API. A successful List call
// means the CRD is registered — even if empty. NotFound is returned as (false, nil) so the manager's
// probe treats it as a definitive "not installed" signal (and updates the availability map). Other
// errors (timeouts, RBAC, transient API failures) are returned as (false, err). That lets the probe
// leave the previous-known state untouched rather than briefly hiding the source on a flake.
func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	if s.client == nil {
		return false, nil
	}
	_, err := s.client.Resource(VulnerabilityReportGVR).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("trivy-operator availability probe: %w", err)
	}
	return true, nil
}

// Fetch lists VulnerabilityReport and ConfigAuditReport CRDs and returns
// them as findings. Lists are paginated (default 200 items per page) so
// large clusters don't materialise the whole report set in one response.
// Trivy reports embed full vuln details, and a busy cluster easily
// produces 10s of MB per source. Per-report parse errors are swallowed
// (malformed items skipped).
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	if s.client == nil {
		return nil, nil
	}

	var findings []security.Finding

	vulnList, err := security.ListPaginated(ctx, s.client.Resource(VulnerabilityReportGVR).Namespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("list vulnerability reports: %w", err)
	}
	for i := range vulnList.Items {
		findings = append(findings, parseVulnerabilityReport(&vulnList.Items[i])...)
	}

	auditList, err := security.ListPaginated(ctx, s.client.Resource(ConfigAuditReportGVR).Namespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("list config audit reports: %w", err)
	}
	for i := range auditList.Items {
		findings = append(findings, parseConfigAuditReport(&auditList.Items[i])...)
	}

	return findings, nil
}
