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
	// scanSecrets gates the Secret-listing checks (legacy_sa_token_secret,
	// tls_secret_expiry). From security.heuristic.scan_secrets.
	scanSecrets bool
}

// NewWithClient returns a heuristic source that lists pods via the given client.
func NewWithClient(client kubernetes.Interface) *Source {
	return &Source{client: client}
}

// SetSecretEnvPatterns configures extra include/exclude env-var name globs
// for the secret_env check (from security.heuristic.secret_env_include /
// _exclude).
// Must be called before the first Fetch — the fields are not synchronized.
func (s *Source) SetSecretEnvPatterns(include, exclude []string) {
	s.secretEnvInclude = include
	s.secretEnvExclude = exclude
}

// SetScanSecrets enables the Secret-listing checks (legacy_sa_token_secret,
// tls_secret_expiry). Off unless explicitly enabled by the app from
// security.heuristic.scan_secrets (default true there). Must be called
// before the first Fetch — the field is not synchronized.
func (s *Source) SetScanSecrets(enabled bool) {
	s.scanSecrets = enabled
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

// Categories returns the categories this source contributes to. Reliability
// covers the bare_pod check, which is a recommendation rather than a
// misconfiguration and stays off the SEC badge.
func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryMisconfig, security.CategoryReliability}
}

// IsAvailable returns true only when a kubernetes client has been injected.
func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	return s.client != nil, nil
}

// Fetch lists pods in the given namespace (empty = all namespaces) and runs
// every registered check against every container. Then it runs the
// best-effort non-pod scans (Services, namespaces, ConfigMaps, Ingresses).
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	if s.client == nil {
		return nil, nil
	}
	var findings []security.Finding
	// ConfigMaps and Secrets are listed before the pod scan so each pod's
	// references can be verified against the existing names. Both are
	// best-effort: a failed (or, for Secrets, disabled) list disables only
	// the reference verification, never the pod checks.
	cmFindings, cmNames, cmOK := s.fetchConfigMapFindings(ctx, namespace)
	findings = append(findings, cmFindings...)
	secretFindings, secretNames, secretsOK := s.fetchSecretFindings(ctx, namespace)
	findings = append(findings, secretFindings...)
	// Namespaces that contain at least one scanned pod, for the
	// namespace-level checks (no point flagging a missing NetworkPolicy in
	// an empty namespace).
	nsWithPods := map[string]bool{}
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
			nsWithPods[pod.Namespace] = true
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
			findings = append(findings, checkMissingRefs(pod, cmNames, cmOK, secretNames, secretsOK)...)
		}
		if list.Continue == "" {
			break
		}
		opts.Continue = list.Continue
	}
	findings = append(findings, s.fetchServiceFindings(ctx, namespace)...)
	findings = append(findings, s.fetchNamespaceFindings(ctx, namespace, nsWithPods)...)
	findings = append(findings, s.fetchIngressFindings(ctx, namespace)...)
	return findings, nil
}

// fetchServiceFindings lists Services (paginated) and runs the Service-level
// checks. Best-effort: a list error (typically Forbidden for restricted
// users) stops the Service scan without failing the source — the pod
// findings must survive. Findings from pages that did load are kept. The
// checks are presence-based, so a partial list can under-report but never
// invent findings.
func (s *Source) fetchServiceFindings(ctx context.Context, namespace string) []security.Finding {
	var findings []security.Finding
	opts := metav1.ListOptions{Limit: 200}
	for {
		list, err := s.client.CoreV1().Services(namespace).List(ctx, opts)
		if err != nil {
			return findings
		}
		for i := range list.Items {
			svc := &list.Items[i]
			if s.ignoredNamespaces[svc.Namespace] {
				continue
			}
			for _, check := range serviceChecks {
				findings = append(findings, check(svc)...)
			}
		}
		if list.Continue == "" {
			return findings
		}
		opts.Continue = list.Continue
	}
}

// checkFn is the signature all heuristic checks implement.
type checkFn func(pod *corev1.Pod, c corev1.Container) []security.Finding
