package app

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientfake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

func sampleFindings() []security.Finding {
	return []security.Finding{
		{
			ID:       "CVE-2024-1",
			Source:   "trivy-operator",
			Category: "vulnerability",
			Severity: security.SeverityCritical,
			Title:    "critical CVE",
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "web-1"},
			Summary:  "bad",
			Labels:   map[string]string{"cve": "CVE-2024-1"},
		},
		{
			ID:       "policy-2",
			Source:   "kyverno",
			Category: "policy",
			Severity: security.SeverityMedium,
			Title:    "policy violation",
			Resource: security.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "api"},
		},
	}
}

// TestSecurityFindingsCacheRoundTrip verifies findings persist and load back
// intact for a given (host, namespace), preserving all fields used by the
// badge index.
func TestSecurityFindingsCacheRoundTrip(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	host := "https://api.example.test:6443"
	ns := "default"

	require.NoError(t, saveSecurityFindingsCacheForHost(host, ns, sampleFindings(), time.Now().UTC()))

	got := loadSecurityFindingsCacheForHost(host, ns, time.Hour)
	require.Len(t, got, 2)
	assert.Equal(t, "CVE-2024-1", got[0].ID)
	assert.Equal(t, security.SeverityCritical, got[0].Severity)
	assert.Equal(t, "web-1", got[0].Resource.Name)
	assert.Equal(t, security.SeverityMedium, got[1].Severity)
}

// TestSecurityFindingsCacheNamespaceKeyed verifies findings are stored per
// namespace within the host file — loading a different namespace must miss.
func TestSecurityFindingsCacheNamespaceKeyed(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	host := "https://api.example.test:6443"

	require.NoError(t, saveSecurityFindingsCacheForHost(host, "ns-a", sampleFindings(), time.Now().UTC()))

	assert.Len(t, loadSecurityFindingsCacheForHost(host, "ns-a", time.Hour), 2, "stored namespace must load")
	assert.Nil(t, loadSecurityFindingsCacheForHost(host, "ns-b", time.Hour), "other namespace must miss")

	// The all-namespaces key ("") is distinct from a named namespace.
	require.NoError(t, saveSecurityFindingsCacheForHost(host, "", sampleFindings(), time.Now().UTC()))
	assert.Len(t, loadSecurityFindingsCacheForHost(host, "", time.Hour), 2, "all-namespaces key must load independently")
}

// TestSecurityFindingsCacheTTLExpiry verifies an entry older than the TTL is
// treated as a miss (stale-while-revalidate still paints it, but the loader
// here enforces the freshness window the caller passes).
func TestSecurityFindingsCacheTTLExpiry(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	host := "https://api.example.test:6443"
	ns := "default"

	stale := time.Now().UTC().Add(-2 * time.Hour)
	require.NoError(t, saveSecurityFindingsCacheForHost(host, ns, sampleFindings(), stale))

	assert.Nil(t, loadSecurityFindingsCacheForHost(host, ns, time.Hour), "entry older than TTL must be a miss")
	assert.Len(t, loadSecurityFindingsCacheForHost(host, ns, 3*time.Hour), 2, "within a larger TTL it loads")
}

// TestSecurityFindingsCacheMissReturnsNil covers the no-file and empty-host
// paths — both must be nil (fall through to a live scan), never panic.
func TestSecurityFindingsCacheMissReturnsNil(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	assert.Nil(t, loadSecurityFindingsCacheForHost("https://never-written:6443", "default", time.Hour))
	assert.Nil(t, loadSecurityFindingsCacheForHost("", "default", time.Hour), "empty host must be a no-op miss")
	assert.NoError(t, saveSecurityFindingsCacheForHost("", "default", sampleFindings(), time.Now().UTC()), "empty host save is a no-op")
}

// TestSecurityFindingsCacheZeroFindingsRoundTrip verifies a clean scan with
// zero findings is a real cached result (non-nil), not indistinguishable from
// a miss — nil must normalize to an empty slice on save so it survives the
// JSON null round-trip.
func TestSecurityFindingsCacheZeroFindingsRoundTrip(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	host := "https://api.example.test:6443"

	require.NoError(t, saveSecurityFindingsCacheForHost(host, "default", nil, time.Now().UTC()))
	got := loadSecurityFindingsCacheForHost(host, "default", time.Hour)
	require.NotNil(t, got, "a cached zero-findings scan must be a hit (non-nil), not a miss")
	assert.Empty(t, got)

	require.NoError(t, saveSecurityFindingsCacheForHost(host, "other", []security.Finding{}, time.Now().UTC()))
	got = loadSecurityFindingsCacheForHost(host, "other", time.Hour)
	require.NotNil(t, got, "explicit empty slice must also round-trip as a hit")
	assert.Empty(t, got)
}

// TestSecurityFindingsCacheFilePerms verifies the findings file is written
// 0600 (not world-readable) since it carries CVE/policy/resource detail.
func TestSecurityFindingsCacheFilePerms(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	host := "https://api.example.test:6443"
	require.NoError(t, saveSecurityFindingsCacheForHost(host, "default", sampleFindings(), time.Now().UTC()))

	path := securityFindingsCacheFilePathForHost(host)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "findings file must not be world-readable")
}

// findingsCacheTestModel builds a Model whose client resolves a host for
// "ctx" so the persistence path actually writes a file.
func findingsCacheTestModel(t *testing.T) Model {
	t.Helper()
	c := k8s.NewTestClient(clientfake.NewClientset(), nil)
	c.SetTestHostForContext("ctx", "https://api.persist.test:6443")
	m := Model{}
	m.client = c
	m.nav = model.NavigationState{Context: "ctx"}
	m.namespace = "default"
	return m
}

// TestUpdateSecurityFindingsLoaded_PersistsCleanScan verifies a fully clean
// scan (no per-source errors) is written to the disk cache so the next session
// can paint it.
func TestUpdateSecurityFindingsLoaded_PersistsCleanScan(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	m := findingsCacheTestModel(t)

	_, cmd := m.updateSecurityFindingsLoaded(securityFindingsLoadedMsg{
		context:   "ctx",
		namespace: "default",
		findings:  sampleFindings(),
		errors:    map[string]error{"trivy-operator": nil},
	})
	require.NotNil(t, cmd, "a clean scan must defer persistence to a command")
	cmd() // run the background persistence (off the Update goroutine)

	got := loadSecurityFindingsCacheForHost("https://api.persist.test:6443", "default", time.Hour)
	require.Len(t, got, 2, "a clean scan must be persisted to disk")
}

// TestUpdateSecurityFindingsLoaded_PersistenceIsDeferred verifies the disk
// write does NOT run inline on the Update goroutine — marshaling the full
// findings set to YAML there froze the UI until the write finished. The
// handler must return a command and leave the file unwritten until it runs.
func TestUpdateSecurityFindingsLoaded_PersistenceIsDeferred(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	m := findingsCacheTestModel(t)

	_, cmd := m.updateSecurityFindingsLoaded(securityFindingsLoadedMsg{
		context:   "ctx",
		namespace: "default",
		findings:  sampleFindings(),
		errors:    map[string]error{"trivy-operator": nil},
	})
	require.NotNil(t, cmd, "clean scan must return a persistence command")

	// The write must NOT have happened inline on the Update goroutine.
	assert.Nil(t, loadSecurityFindingsCacheForHost("https://api.persist.test:6443", "default", time.Hour),
		"persistence must be deferred, not run inline on the Update goroutine")

	cmd() // only now, off the Update goroutine, does the write happen
	got := loadSecurityFindingsCacheForHost("https://api.persist.test:6443", "default", time.Hour)
	require.Len(t, got, 2, "running the deferred command writes the cache")
}

// TestUpdateSecurityFindingsLoaded_SkipsPartialScan verifies a scan where any
// source errored is NOT persisted — caching an undercount would show stale-low
// badges until the TTL expires.
func TestUpdateSecurityFindingsLoaded_SkipsPartialScan(t *testing.T) {
	t.Setenv("KUBECACHEDIR", t.TempDir())
	m := findingsCacheTestModel(t)

	_, cmd := m.updateSecurityFindingsLoaded(securityFindingsLoadedMsg{
		context:   "ctx",
		namespace: "default",
		findings:  sampleFindings()[:1], // undercount: one source failed
		errors:    map[string]error{"kyverno": errors.New("timeout")},
	})
	assert.Nil(t, cmd, "a partial (errored) scan must not persist")

	assert.Nil(t, loadSecurityFindingsCacheForHost("https://api.persist.test:6443", "default", time.Hour),
		"a partial (errored) scan must not be persisted")
}
