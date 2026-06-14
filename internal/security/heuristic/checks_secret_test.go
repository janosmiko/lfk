package heuristic

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/security"
)

// certPEM builds a self-signed certificate with the given NotAfter.
func certPEM(t *testing.T, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    notAfter.Add(-365 * 24 * time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func tlsSecret(ns, name string, cert []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Type:       corev1.SecretTypeTLS,
		Data:       map[string][]byte{corev1.TLSCertKey: cert},
	}
}

func scanningSource(objs ...*corev1.Secret) *Source {
	converted := make([]runtime.Object, 0, len(objs))
	for _, o := range objs {
		converted = append(converted, o)
	}
	s := NewWithClient(fake.NewSimpleClientset(converted...))
	s.SetScanSecrets(true)
	return s
}

func TestSecretChecks(t *testing.T) {
	expired := tlsSecret("prod", "expired-cert", certPEM(t, time.Now().Add(-24*time.Hour)))
	expiring := tlsSecret("prod", "expiring-cert", certPEM(t, time.Now().Add(7*24*time.Hour)))
	healthy := tlsSecret("prod", "healthy-cert", certPEM(t, time.Now().Add(90*24*time.Hour)))
	garbage := tlsSecret("prod", "garbage-cert", []byte("not a pem"))
	ignored := tlsSecret("ignored-ns", "ignored-cert", certPEM(t, time.Now().Add(-24*time.Hour)))
	legacy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "old-token"},
		Type:       corev1.SecretTypeServiceAccountToken,
	}
	opaque := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "plain"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"PASSWORD": []byte("x")},
	}

	s := scanningSource(expired, expiring, healthy, garbage, legacy, opaque, ignored)
	s.SetIgnoredNamespaces([]string{"ignored-ns"})
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)

	bySecret := map[string]security.Finding{}
	for _, f := range findings {
		if f.Resource.Kind == "Secret" {
			bySecret[f.Resource.Name] = f
		}
	}
	require.Contains(t, bySecret, "expired-cert")
	assert.Equal(t, security.SeverityHigh, bySecret["expired-cert"].Severity)
	require.Contains(t, bySecret, "expiring-cert")
	assert.Equal(t, security.SeverityMedium, bySecret["expiring-cert"].Severity)
	assert.NotContains(t, bySecret, "healthy-cert")
	assert.NotContains(t, bySecret, "garbage-cert", "unparseable cert data is skipped")
	require.Contains(t, bySecret, "old-token")
	assert.Equal(t, "legacy_sa_token_secret", bySecret["old-token"].Labels["check"])
	assert.NotContains(t, bySecret, "plain", "opaque secret content is never inspected")
	assert.NotContains(t, bySecret, "ignored-cert", "ignored namespaces are skipped")
}

// TestSecretChecksDisabledByDefault: a Source without SetScanSecrets(true)
// must never list Secrets.
func TestSecretChecksDisabledByDefault(t *testing.T) {
	legacy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "old-token"},
		Type:       corev1.SecretTypeServiceAccountToken,
	}
	client := fake.NewSimpleClientset(legacy)
	listed := false
	client.PrependReactor("list", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		listed = true
		return false, nil, nil
	})
	findings, err := NewWithClient(client).Fetch(t.Context(), "", "")
	require.NoError(t, err)
	for _, f := range findings {
		assert.NotEqual(t, "Secret", f.Resource.Kind)
	}
	assert.False(t, listed, "scan_secrets off must mean zero Secret list calls")
}

func TestSecretListBestEffort(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "old-token"},
			Type:       corev1.SecretTypeServiceAccountToken,
		},
		// A pod with a finding proves the rest of the scan survives the
		// forbidden secret list.
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "prod", Name: "p",
				OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "rs"}},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "c", SecurityContext: &corev1.SecurityContext{Privileged: new(true)},
			}}},
		},
	)
	forbidList(client, "secrets")
	s := NewWithClient(client)
	s.SetScanSecrets(true)
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)
	var privileged bool
	for _, f := range findings {
		assert.NotEqual(t, "Secret", f.Resource.Kind, "no secret findings without the list")
		privileged = privileged || f.Labels["check"] == "privileged"
	}
	assert.True(t, privileged, "pod findings must survive a forbidden secret list")
}

// TestSecretSummariesNeverLeakData: no finding summary may contain secret
// bytes.
func TestSecretSummariesNeverLeakData(t *testing.T) {
	legacy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "old-token"},
		Type:       corev1.SecretTypeServiceAccountToken,
		Data:       map[string][]byte{"token": []byte("super-secret-token-bytes")},
	}
	s := scanningSource(legacy)
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)
	require.NotEmpty(t, findings)
	for _, f := range findings {
		assert.NotContains(t, f.Summary, "super-secret-token-bytes")
		assert.NotContains(t, f.Title, "super-secret-token-bytes")
	}
}
