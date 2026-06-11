// Secret-level heuristic checks: legacy long-lived ServiceAccount token
// Secrets and expiring TLS certificates. Gated by
// security.heuristic.scan_secrets (default on) because they require listing
// Secret objects; the list is best-effort like the other non-pod scans.
// Summaries never include Secret data.

package heuristic

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// tlsExpiryWindow is how far ahead of NotAfter the tls_secret_expiry check
// starts warning.
const tlsExpiryWindow = 30 * 24 * time.Hour

func makeSecretFinding(ns, name, check string, sev security.Severity, title, summary string) security.Finding {
	return security.Finding{
		ID:       fmt.Sprintf("heuristic/%s/Secret/%s/%s", ns, name, check),
		Source:   "heuristic",
		Category: security.CategoryMisconfig,
		Severity: sev,
		Title:    title,
		Resource: security.ResourceRef{Namespace: ns, Kind: "Secret", Name: name},
		Summary:  summary,
		Labels:   map[string]string{"check": check},
	}
}

// fetchSecretFindings lists Secrets (best-effort) and runs the Secret-level
// checks. Returns immediately when scan_secrets is disabled. Also returns
// the "ns/name" set of existing Secrets for the missing-reference check;
// ok=false (disabled or list failed) means Secret references cannot be
// verified and must be skipped.
func (s *Source) fetchSecretFindings(ctx context.Context, namespace string) (findings []security.Finding, names map[string]bool, ok bool) {
	if !s.scanSecrets {
		return nil, nil, false
	}
	secrets, ok := security.Collect(func(o metav1.ListOptions) ([]corev1.Secret, string, error) {
		l, err := s.client.CoreV1().Secrets(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	if !ok {
		return nil, nil, false
	}
	now := time.Now()
	names = make(map[string]bool, len(secrets))
	var out []security.Finding
	for i := range secrets {
		sec := &secrets[i]
		names[sec.Namespace+"/"+sec.Name] = true
		if s.ignoredNamespaces[sec.Namespace] {
			continue
		}
		switch sec.Type {
		case corev1.SecretTypeServiceAccountToken:
			out = append(out, makeSecretFinding(sec.Namespace, sec.Name, "legacy_sa_token_secret", security.SeverityMedium,
				"legacy ServiceAccount token Secret",
				fmt.Sprintf("Secret %q is a long-lived ServiceAccount token (type %s), deprecated since Kubernetes 1.24; it never expires and is a standing credential-theft target. Use bound token projection instead.", sec.Name, corev1.SecretTypeServiceAccountToken)))
		case corev1.SecretTypeTLS:
			out = append(out, checkTLSExpiry(sec, now)...)
		}
	}
	return out, names, true
}

// checkTLSExpiry parses the certificate of a kubernetes.io/tls Secret and
// flags it when expired (High) or expiring within tlsExpiryWindow (Medium).
// Unparseable data is skipped silently — flagging garbage would duplicate
// cert-manager-style tooling errors without adding signal.
//
// Only the first PEM block is parsed: the Kubernetes TLS convention puts
// the leaf certificate first in tls.crt. A root-first chain (some OpenSSL
// bundles) would check the long-lived CA instead of the leaf — accepted as
// out of convention.
func checkTLSExpiry(sec *corev1.Secret, now time.Time) []security.Finding {
	block, _ := pem.Decode(sec.Data[corev1.TLSCertKey])
	if block == nil {
		return nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil
	}
	switch {
	case now.After(cert.NotAfter):
		return []security.Finding{makeSecretFinding(sec.Namespace, sec.Name, "tls_secret_expiry", security.SeverityHigh,
			"TLS certificate expired",
			fmt.Sprintf("TLS Secret %q holds a certificate that expired on %s; anything serving it is failing TLS handshakes or about to.", sec.Name, cert.NotAfter.Format("2006-01-02")))}
	case now.Add(tlsExpiryWindow).After(cert.NotAfter):
		return []security.Finding{makeSecretFinding(sec.Namespace, sec.Name, "tls_secret_expiry", security.SeverityMedium,
			"TLS certificate expiring soon",
			fmt.Sprintf("TLS Secret %q holds a certificate expiring on %s (within %d days); renew it before clients start failing.", sec.Name, cert.NotAfter.Format("2006-01-02"), int(tlsExpiryWindow.Hours()/24)))}
	}
	return nil
}
