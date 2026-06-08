package app

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityFindingsCache_WrittenAsJSON verifies the on-disk format is JSON
// (issue #387 switched off sigs.k8s.io/yaml, whose YAML->JSON->struct decode of
// a tens-of-MB file was the dominant startup allocation).
func TestSecurityFindingsCache_WrittenAsJSON(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	host := "https://api.example.test:6443"
	require.NoError(t, saveSecurityFindingsCacheForHost(host, "default", sampleFindings(), time.Now().UTC()))

	path := securityFindingsCacheFilePathForHost(host)
	assert.Equal(t, ".json", filepath.Ext(path), "cache file uses a .json extension")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(data, &envelope), "file content must be valid JSON")
	assert.Contains(t, envelope, "namespaces")
}

// TestSecurityFindingsCache_LazyDecodeIgnoresOtherNamespaces proves a load
// decodes only the requested namespace: a sibling namespace whose entry is
// undecodable must not affect (or block) loading a healthy namespace.
func TestSecurityFindingsCache_LazyDecodeIgnoresOtherNamespaces(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	host := "https://api.example.test:6443"

	goodEntry, err := json.Marshal(securityFindingsEntry{ScannedAt: time.Now().UTC(), Findings: sampleFindings()})
	require.NoError(t, err)
	// ns-bad has a structurally valid object but findings of the wrong type,
	// so decoding it into securityFindingsEntry fails.
	file := map[string]any{
		"schema_version": securityFindingsCacheSchemaVersion,
		"namespaces": map[string]any{
			"ns-good": json.RawMessage(goodEntry),
			"ns-bad":  json.RawMessage(`{"scanned_at":"2026-01-01T00:00:00Z","findings":"not-an-array"}`),
		},
	}
	data, err := json.Marshal(file)
	require.NoError(t, err)
	path := securityFindingsCacheFilePathForHost(host)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, data, 0o600))

	got := loadSecurityFindingsCacheForHost(host, "ns-good", time.Hour)
	require.Len(t, got, 2, "healthy namespace loads despite a corrupt sibling")
	assert.Equal(t, "CVE-2024-1", got[0].ID)

	assert.Nil(t, loadSecurityFindingsCacheForHost(host, "ns-bad", time.Hour),
		"a corrupt namespace entry is a graceful miss, not a panic")
}

// TestSecurityFindingsCache_LegacyYAMLIsMissAndRemovedOnSave verifies a
// pre-migration YAML cache is ignored (new code reads only the .json path) and
// is cleaned up on the next save so the large legacy file does not linger.
func TestSecurityFindingsCache_LegacyYAMLIsMissAndRemovedOnSave(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	host := "https://api.example.test:6443"

	legacy := securityFindingsCacheLegacyPathForHost(host)
	require.NotEmpty(t, legacy)
	require.NoError(t, os.MkdirAll(filepath.Dir(legacy), 0o700))
	require.NoError(t, os.WriteFile(legacy, []byte("schema_version: 1\nnamespaces: {}\n"), 0o600))

	assert.Nil(t, loadSecurityFindingsCacheForHost(host, "default", time.Hour),
		"legacy YAML cache is not read by the JSON loader")

	require.NoError(t, saveSecurityFindingsCacheForHost(host, "default", sampleFindings(), time.Now().UTC()))
	_, err := os.Stat(legacy)
	assert.True(t, errors.Is(err, fs.ErrNotExist), "legacy YAML file is removed on save")
	_, err = os.Stat(securityFindingsCacheFilePathForHost(host))
	assert.NoError(t, err, "JSON cache file is written")
}

// TestSecurityFindingsCache_SavePreservesOtherNamespaces verifies the
// read-modify-write keeps other namespaces' entries (which are carried as raw
// JSON, never re-decoded) when one namespace is updated.
func TestSecurityFindingsCache_SavePreservesOtherNamespaces(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	host := "https://api.example.test:6443"

	require.NoError(t, saveSecurityFindingsCacheForHost(host, "ns-a", sampleFindings(), time.Now().UTC()))
	require.NoError(t, saveSecurityFindingsCacheForHost(host, "ns-b", sampleFindings()[:1], time.Now().UTC()))

	assert.Len(t, loadSecurityFindingsCacheForHost(host, "ns-a", time.Hour), 2, "first namespace survives the second save")
	assert.Len(t, loadSecurityFindingsCacheForHost(host, "ns-b", time.Hour), 1, "second namespace stored")

	// A third save updates ns-a; ns-b must still be intact.
	require.NoError(t, saveSecurityFindingsCacheForHost(host, "ns-a", sampleFindings()[:1], time.Now().UTC()))
	assert.Len(t, loadSecurityFindingsCacheForHost(host, "ns-a", time.Hour), 1, "ns-a updated")
	assert.Len(t, loadSecurityFindingsCacheForHost(host, "ns-b", time.Hour), 1, "ns-b preserved across ns-a update")
}
