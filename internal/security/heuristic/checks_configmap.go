// ConfigMap-level heuristic check: data keys that look like credentials.
// Reuses the secret_env keyword/glob engine on key NAMES — values are never
// inspected beyond non-emptiness and never appear in summaries.

package heuristic

import (
	"context"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// fetchConfigMapFindings lists ConfigMaps (best-effort) and flags those with
// credential-looking data keys — plaintext secrets outside Secrets are not
// covered by encryption-at-rest or Secret RBAC. Also returns the "ns/name"
// set of existing ConfigMaps for the missing-reference check. ok=false means
// the list failed and reference checks must be skipped.
func (s *Source) fetchConfigMapFindings(ctx context.Context, namespace string) (findings []security.Finding, names map[string]bool, ok bool) {
	cms, ok := security.Collect(func(o metav1.ListOptions) ([]corev1.ConfigMap, string, error) {
		l, err := s.client.CoreV1().ConfigMaps(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	if !ok {
		return nil, nil, false
	}
	names = make(map[string]bool, len(cms))
	var out []security.Finding
	for i := range cms {
		cm := &cms[i]
		names[cm.Namespace+"/"+cm.Name] = true
		if s.ignoredNamespaces[cm.Namespace] {
			continue
		}
		if names := credentialLookingKeys(cm, s.secretEnvInclude, s.secretEnvExclude); len(names) > 0 {
			out = append(out, security.Finding{
				ID:       fmt.Sprintf("heuristic/%s/ConfigMap/%s/configmap_secret_keys", cm.Namespace, cm.Name),
				Source:   "heuristic",
				Category: security.CategoryMisconfig,
				Severity: security.SeverityMedium,
				Title:    "credential-looking keys in ConfigMap",
				Resource: security.ResourceRef{Namespace: cm.Namespace, Kind: "ConfigMap", Name: cm.Name},
				Summary:  fmt.Sprintf("ConfigMap %q holds credential-looking key(s) %s; ConfigMaps are not encrypted at rest or guarded by Secret RBAC. Move them to a Secret.", cm.Name, strings.Join(names, ", ")),
				Labels:   map[string]string{"check": "configmap_secret_keys"},
			})
		}
	}
	return out, names, true
}

// credentialLookingKeys returns ConfigMap data keys whose name matches the
// secret_env keyword/glob engine and whose value is non-empty. Same
// include/exclude semantics as the env check: exclude wins, an explicit
// include overrides a built-in exemption.
func credentialLookingKeys(cm *corev1.ConfigMap, include, exclude []string) []string {
	var names []string
	for key, value := range cm.Data {
		if value == "" {
			continue
		}
		upper := strings.ToUpper(key)
		if matchesAnyGlob(upper, exclude) {
			continue
		}
		builtin := containsAny(upper, secretEnvKeywords) &&
			!containsAny(upper, secretEnvExempt) &&
			!hasAnyPrefix(upper, secretEnvExemptPrefixes)
		if !builtin && !matchesAnyGlob(upper, include) {
			continue
		}
		names = append(names, key)
	}
	// Map iteration order is random. Sort for stable summaries.
	slices.Sort(names)
	return names
}
