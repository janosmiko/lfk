package heuristic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/security"
)

func configMap(ns, name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		Namespace: ns, Name: name,
		Data: data,
	}
}

func TestConfigMapSecretKeys(t *testing.T) {
	s := NewWithClient(fake.NewSimpleClientset(
		configMap("prod", "leaky", map[string]string{
			"DB_PASSWORD": "hunter2",
			"LOG_LEVEL":   "debug",
		}),
		configMap("ignored-ns", "skipped", map[string]string{"DB_PASSWORD": "x"}),
		configMap("prod", "clean", map[string]string{"LOG_LEVEL": "info"}),
		// Exempt suffixes apply to keys the same way they do to env names.
		configMap("prod", "paths", map[string]string{"TOKEN_PATH": "/var/run/secret"}),
		// Empty values never flag.
		configMap("prod", "empty-val", map[string]string{"API_KEY": ""}),
	))
	s.SetIgnoredNamespaces([]string{"ignored-ns"})
	checks := checksByResource(t, s)

	assert.True(t, checks["prod/ConfigMap/leaky"]["configmap_secret_keys"])
	assert.Empty(t, checks["ignored-ns/ConfigMap/skipped"], "ignored namespaces are skipped")
	assert.Empty(t, checks["prod/ConfigMap/clean"])
	assert.Empty(t, checks["prod/ConfigMap/paths"])
	assert.Empty(t, checks["prod/ConfigMap/empty-val"])
}

// TestConfigMapSecretKeysNeverLeaksValues: the summary lists key names only.
func TestConfigMapSecretKeysNeverLeaksValues(t *testing.T) {
	s := NewWithClient(fake.NewSimpleClientset(
		configMap("prod", "leaky", map[string]string{"DB_PASSWORD": "sup3r-s3cret-value"}),
	))
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Summary, "DB_PASSWORD")
	assert.NotContains(t, findings[0].Summary, "sup3r-s3cret-value")
	assert.Equal(t, security.SeverityMedium, findings[0].Severity)
}

// TestConfigMapSecretKeysHonorsPatterns: the secret_env include/exclude
// globs apply to ConfigMap keys too.
func TestConfigMapSecretKeysHonorsPatterns(t *testing.T) {
	cm := configMap("prod", "cm", map[string]string{
		"MY_CONN_STR":     "Server=db;Pwd=x",
		"LEGACY_PASSWORD": "x",
	})
	s := NewWithClient(fake.NewSimpleClientset(cm))
	s.SetSecretEnvPatterns([]string{"*_CONN_STR"}, []string{"LEGACY_*"})
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Summary, "MY_CONN_STR")
	assert.NotContains(t, findings[0].Summary, "LEGACY_PASSWORD")
}

func TestConfigMapListBestEffort(t *testing.T) {
	client := fake.NewSimpleClientset(
		configMap("prod", "leaky", map[string]string{"DB_PASSWORD": "x"}),
	)
	forbidList(client, "configmaps")
	findings, err := NewWithClient(client).Fetch(t.Context(), "", "")
	require.NoError(t, err)
	assert.Empty(t, findings)
}
