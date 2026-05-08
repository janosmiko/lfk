// Package falco reads Falco runtime security events from the Kubernetes
// Events API (created by falcosidekick) and exposes them as security.Findings.
package falco

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/janosmiko/lfk/internal/security"
)

// tailLines is the number of log lines to read from each Falco pod.
// Falco outputs one JSON line per alert; 500 covers the recent history
// without pulling excessive data.
const tailLines int64 = 500

// Source is the falco SecuritySource implementation. It reads Kubernetes
// Events created by falcosidekick (reporting component = "falco" or
// "falcosidekick") and converts them to findings.
type Source struct {
	client kubernetes.Interface
}

// New returns a Source with no client (Fetch returns nil, IsAvailable false).
func New() *Source { return &Source{} }

// NewWithClient returns a Source using the given Kubernetes clientset.
func NewWithClient(client kubernetes.Interface) *Source {
	return &Source{client: client}
}

// Name returns the stable identifier.
func (s *Source) Name() string { return "falco" }

// Categories returns the categories this source produces.
func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryPolicy, security.CategoryMisconfig}
}

// falcoLabelSelectors are tried in order to find Falco DaemonSet pods.
// Different Helm chart versions and installation methods use different labels.
var falcoLabelSelectors = []string{
	"app.kubernetes.io/name=falco",
	"app=falco",
	"app.kubernetes.io/instance=falco",
}

// IsAvailable checks if Falco is installed by looking for pods matching
// common Falco label selectors across all namespaces.
func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	if s.client == nil {
		return false, nil
	}
	for _, sel := range falcoLabelSelectors {
		pods, err := s.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			LabelSelector: sel,
			Limit:         1,
		})
		if err != nil {
			continue
		}
		if len(pods.Items) > 0 {
			return true, nil
		}
	}
	// Last resort: check the "falco" namespace for any DaemonSet pods.
	pods, err := s.client.CoreV1().Pods("falco").List(ctx, metav1.ListOptions{Limit: 5})
	if err != nil {
		return false, err //nolint:wrapcheck // propagate API error directly
	}
	for _, p := range pods.Items {
		if strings.Contains(p.Name, "falco") && !strings.Contains(p.Name, "sidekick") {
			return true, nil
		}
	}
	return false, nil
}

// Fetch reads alerts from Falco pod logs (JSON output). This works with
// any falcosidekick configuration since Falco always writes JSON to stdout
// when json_output is enabled. Falls back to k8s Events if no log-based
// findings are found.
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	if s.client == nil {
		return nil, nil
	}

	findings := s.fetchFromLogs(ctx, namespace)
	if len(findings) > 0 {
		return findings, nil
	}

	// Fallback: try k8s Events (if falcosidekick has kubernetes output).
	return s.fetchFromEvents(ctx, namespace)
}

// fetchFromLogs reads recent JSON alerts from Falco DaemonSet pod logs.
func (s *Source) fetchFromLogs(ctx context.Context, namespace string) []security.Finding {
	// Find Falco pods using the same label selectors as IsAvailable.
	var falcoPods []corev1.Pod
	for _, sel := range falcoLabelSelectors {
		pods, err := s.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			LabelSelector: sel,
		})
		if err == nil && len(pods.Items) > 0 {
			falcoPods = pods.Items
			break
		}
	}
	// Fallback: check the "falco" namespace by name.
	if len(falcoPods) == 0 {
		pods, err := s.client.CoreV1().Pods("falco").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil
		}
		for _, p := range pods.Items {
			if strings.Contains(p.Name, "falco") && !strings.Contains(p.Name, "sidekick") {
				falcoPods = append(falcoPods, p)
			}
		}
	}
	if len(falcoPods) == 0 {
		return nil
	}

	var findings []security.Finding
	seen := make(map[string]bool)
	tail := tailLines

	for i := range falcoPods {
		pod := &falcoPods[i]
		// Read logs from the falco container.
		logOpts := &corev1.PodLogOptions{
			TailLines: &tail,
			Container: "falco",
		}
		stream, err := s.client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOpts).Stream(ctx)
		if err != nil {
			// Try without container name (single-container pod).
			logOpts.Container = ""
			stream, err = s.client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOpts).Stream(ctx)
			if err != nil {
				continue
			}
		}
		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			line := scanner.Bytes()
			for _, f := range parseLogLine(line, namespace) {
				if !seen[f.ID] {
					findings = append(findings, f)
					seen[f.ID] = true
				}
			}
		}
		_ = stream.(io.Closer).Close()
	}
	return findings
}

// fetchFromEvents reads Falco alerts from Kubernetes Events.
func (s *Source) fetchFromEvents(ctx context.Context, namespace string) ([]security.Finding, error) {
	var findings []security.Finding

	events, err := s.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "reportingComponent=falcosidekick",
	})
	if err != nil {
		return nil, fmt.Errorf("list falco events: %w", err)
	}
	for i := range events.Items {
		findings = append(findings, parseEvent(&events.Items[i])...)
	}

	events2, err := s.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "source=falco",
	})
	if err != nil {
		return findings, nil //nolint:nilerr // secondary source, non-fatal
	}
	seen := make(map[string]bool)
	for _, f := range findings {
		seen[f.ID] = true
	}
	for i := range events2.Items {
		for _, f := range parseEvent(&events2.Items[i]) {
			if !seen[f.ID] {
				findings = append(findings, f)
				seen[f.ID] = true
			}
		}
	}
	return findings, nil
}

// parseSeverity maps Falco priority strings to our severity scale.
func parseSeverity(priority string) security.Severity {
	switch strings.ToUpper(priority) {
	case "EMERGENCY", "ALERT", "CRITICAL":
		return security.SeverityCritical
	case "ERROR":
		return security.SeverityHigh
	case "WARNING":
		return security.SeverityMedium
	case "NOTICE", "INFORMATIONAL", "DEBUG":
		return security.SeverityLow
	}
	return security.SeverityUnknown
}
