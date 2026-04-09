// Package heuristic implements a zero-dependency security.SecuritySource that
// walks Pod specs and produces findings for common workload hardening issues.
package heuristic

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// Source is the heuristic SecuritySource implementation.
type Source struct{}

// New returns a new heuristic source.
func New() *Source { return &Source{} }

// Name returns the stable identifier.
func (s *Source) Name() string { return "heuristic" }

// Categories returns the categories this source contributes to.
func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryMisconfig}
}

// IsAvailable is always true for heuristic — it has no external dependencies.
func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	return true, nil
}

// Fetch is implemented in a later task. Stub for now.
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	return nil, nil
}

// checkFn is the signature all heuristic checks implement.
type checkFn func(pod *corev1.Pod, c corev1.Container) []security.Finding

// allChecks is the ordered list of checks the Source runs against each container.
var allChecks = []checkFn{
	checkPrivileged,
	checkHostNamespaces,
	checkHostPath,
	checkReadOnlyRootFilesystem,
	checkRunAsRoot,
	checkAllowPrivilegeEscalation,
}
