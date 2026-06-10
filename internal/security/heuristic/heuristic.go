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
	secretEnvInclude  []string
	secretEnvExclude  []string
}

// New returns a heuristic source with no client. Fetch returns an empty slice
// and IsAvailable reports false. Callers must prefer NewWithClient when they
// have a kubernetes client.
func New() *Source { return &Source{} }

// NewWithClient returns a heuristic source that lists pods via the given client.
func NewWithClient(client kubernetes.Interface) *Source {
	return &Source{client: client}
}

// SetSecretEnvPatterns configures extra include/exclude env-var name globs
// for the secret_env check (from security.secret_env_include / _exclude).
// Must be called before the first Fetch — the fields are not synchronized.
func (s *Source) SetSecretEnvPatterns(include, exclude []string) {
	s.secretEnvInclude = include
	s.secretEnvExclude = exclude
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
	var findings []security.Finding
	// Paginate so an unbounded List can't degrade control-plane
	// responsiveness on large clusters.
	opts := metav1.ListOptions{Limit: 200}
	for {
		list, err := s.client.CoreV1().Pods(namespace).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			pod := &list.Items[i]
			if s.ignoredNamespaces[pod.Namespace] {
				continue
			}
			runChecks := func(c corev1.Container) {
				for _, check := range allChecks {
					findings = append(findings, check(pod, c)...)
				}
				// secret_env takes per-source config, so it is dispatched
				// directly instead of through allChecks.
				findings = append(findings, checkSecretEnvWith(pod, c, s.secretEnvInclude, s.secretEnvExclude)...)
			}
			for _, c := range pod.Spec.InitContainers {
				runChecks(c)
			}
			for _, c := range pod.Spec.Containers {
				runChecks(c)
			}
			// EphemeralContainer embeds EphemeralContainerCommon which mirrors
			// Container's fields. Coerce so the same check signature applies.
			for _, ec := range pod.Spec.EphemeralContainers {
				runChecks(corev1.Container(ec.EphemeralContainerCommon))
			}
		}
		if list.Continue == "" {
			break
		}
		opts.Continue = list.Continue
	}
	return findings, nil
}

// checkFn is the signature all heuristic checks implement.
type checkFn func(pod *corev1.Pod, c corev1.Container) []security.Finding
