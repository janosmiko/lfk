// Package heuristic implements a zero-dependency security.SecuritySource that
// walks Pod specs and produces findings for common workload hardening issues.
package heuristic

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/janosmiko/lfk/internal/security"
)

// Source is the heuristic SecuritySource implementation.
type Source struct {
	client            kubernetes.Interface
	ignoredNamespaces map[string]bool
}

// New returns a heuristic source with no client. Fetch returns an empty slice
// and IsAvailable reports false. Callers must prefer NewWithClient when they
// have a kubernetes client.
func New() *Source { return &Source{} }

// NewWithClient returns a heuristic source that lists pods via the given client.
func NewWithClient(client kubernetes.Interface) *Source {
	return &Source{client: client}
}

// SetIgnoredNamespaces configures namespaces to exclude from heuristic checks.
func (s *Source) SetIgnoredNamespaces(namespaces []string) {
	s.ignoredNamespaces = make(map[string]bool, len(namespaces))
	for _, ns := range namespaces {
		s.ignoredNamespaces[ns] = true
	}
}

// Name returns the stable identifier.
func (s *Source) Name() string { return "heuristic" }

// Categories returns the categories this source contributes to.
func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryMisconfig}
}

// IsAvailable returns true only when a kubernetes client has been injected.
func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	return s.client != nil, nil
}

// Fetch lists pods in the given namespace (empty = all namespaces) and runs
// every registered check against every container.
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	if s.client == nil {
		return nil, nil
	}
	list, err := s.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []security.Finding
	for i := range list.Items {
		pod := &list.Items[i]
		if s.ignoredNamespaces[pod.Namespace] {
			continue
		}
		for _, c := range pod.Spec.Containers {
			for _, check := range allChecks {
				findings = append(findings, check(pod, c)...)
			}
		}
	}
	return findings, nil
}

// checkFn is the signature all heuristic checks implement.
type checkFn func(pod *corev1.Pod, c corev1.Container) []security.Finding
