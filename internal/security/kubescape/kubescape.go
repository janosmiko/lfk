// Package kubescape reads Kubescape CRDs (WorkloadConfigurationScan)
// produced by the kubescape-operator and exposes failed controls as
// security.Findings.
package kubescape

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/security"
)

// WorkloadConfigurationScanGVR is the primary CRD the kubescape-operator
// emits — one object per scanned workload, listing the result of every
// control it ran. Other Kubescape CRDs (vulnerability manifests, summary
// objects) are out of scope for this MVP. They can be wired in later
// without changing the public Source API.
var WorkloadConfigurationScanGVR = schema.GroupVersionResource{
	Group:    "spdx.softwarecomposition.kubescape.io",
	Version:  "v1beta1",
	Resource: "workloadconfigurationscans",
}

// Source is the kubescape SecuritySource implementation.
type Source struct {
	client dynamic.Interface
}

// NewWithDynamic returns a Source using the given dynamic client.
func NewWithDynamic(client dynamic.Interface) *Source {
	return &Source{client: client}
}

// Name returns the stable identifier.
func (s *Source) Name() string { return "kubescape" }

// Categories returns the categories this source produces.
func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryMisconfig}
}

// IsAvailable checks that the WorkloadConfigurationScan CRD is served.
// NotFound is returned as (false, nil) so the manager's probe treats
// it as a definitive "kubescape-operator not installed" signal.
// Transient errors propagate. A 3s timeout / RBAC blip / API server
// hiccup leaves the previous availability untouched rather than
// briefly hiding Kubescape on shift+r.
func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	if s.client == nil {
		return false, nil
	}
	_, err := s.client.Resource(WorkloadConfigurationScanGVR).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("kubescape availability probe: %w", err)
	}
	return true, nil
}

// Fetch lists WorkloadConfigurationScan CRDs and converts every failing control into a
// security.Finding. Lists are paginated (default 200 items per page) so the per-page response
// stays bounded. Kubescape scans embed the full control list per workload and can produce large
// payloads on busy clusters. Per-object parse errors are swallowed so a malformed report doesn't
// black out the whole feed.
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	if s.client == nil {
		return nil, nil
	}
	list, err := security.ListPaginated(ctx, s.client.Resource(WorkloadConfigurationScanGVR).Namespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("list workloadconfigurationscans: %w", err)
	}
	var findings []security.Finding
	for i := range list.Items {
		findings = append(findings, parseWorkloadConfigurationScan(&list.Items[i])...)
	}
	return findings, nil
}
