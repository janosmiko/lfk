package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityAvailabilityCacheRoundTrip writes a snapshot to disk and
// reads it back, asserting the availability map survives serialization
// unchanged. Locks in the on-disk shape so a forward-incompat refactor
// is caught early.
func TestSecurityAvailabilityCacheRoundTrip(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	host := "https-cluster.example.com"

	avail := map[string]bool{
		"heuristic":      true,
		"trivy-operator": false,
		"policy-report":  true,
	}
	require.NoError(t, saveSecurityAvailabilityCacheForHost(host, avail))

	loaded := loadSecurityAvailabilityCacheForHost(host)
	require.NotNil(t, loaded, "round trip must round-trip")
	assert.Equal(t, securityAvailabilityCacheSchemaVersion, loaded.SchemaVersion)
	assert.Equal(t, avail, loaded.Availability)
}

// TestSecurityAvailabilityCacheMissReturnsNil — fresh-cluster path: no
// cache file yet, loader must return nil so the live probe is the only
// signal the sidebar has.
func TestSecurityAvailabilityCacheMissReturnsNil(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	loaded := loadSecurityAvailabilityCacheForHost("never-written.example.com")
	assert.Nil(t, loaded)
}

// TestSecurityAvailabilityCacheCorruptIgnored — a hand-corrupted file
// must not crash the loader; the worst case is an extra probe roundtrip.
func TestSecurityAvailabilityCacheCorruptIgnored(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KUBECACHEDIR", tmp)
	host := "corrupt.example.com"
	path := securityAvailabilityCacheFilePathForHost(host)
	require.NotEmpty(t, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("not yaml :(\n"), 0o644))

	loaded := loadSecurityAvailabilityCacheForHost(host)
	assert.Nil(t, loaded)
}

// TestSecurityAvailabilityCacheSchemaMismatchIgnored — a forward-incompat
// schema bump from a newer binary must be ignored by older binaries
// rather than misinterpreted.
func TestSecurityAvailabilityCacheSchemaMismatchIgnored(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("KUBECACHEDIR", tmp)
	host := "future.example.com"
	path := securityAvailabilityCacheFilePathForHost(host)
	require.NotEmpty(t, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("schema_version: 99\nhost: future.example.com\nupdated_at: 2025-01-01T00:00:00Z\navailability: {}\n"), 0o644))

	loaded := loadSecurityAvailabilityCacheForHost(host)
	assert.Nil(t, loaded)
}
