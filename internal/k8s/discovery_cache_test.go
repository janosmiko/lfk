package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestCacheHostDir(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{
			name: "strips https scheme",
			host: "https://api.example.test:6443",
			want: "api.example.test_6443",
		},
		{
			name: "strips http scheme",
			host: "http://api.example.test",
			want: "api.example.test",
		},
		{
			name: "preserves dots and slashes",
			host: "https://api.example.test/v1/path",
			want: "api.example.test/v1/path",
		},
		{
			name: "replaces colons with underscore",
			host: "https://10.0.0.1:6443",
			want: "10.0.0.1_6443",
		},
		{
			name: "no scheme passes through",
			host: "api.example.test:6443",
			want: "api.example.test_6443",
		},
		{
			name: "leaves underscores",
			host: "https://my_cluster.example.test",
			want: "my_cluster.example.test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cacheHostDir(tt.host)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCacheHostDir_MatchesKubectlLayout(t *testing.T) {
	// Sanity-check that the sanitization matches kubectl's documented
	// behavior so the on-disk layout under ~/.kube/cache/discovery is
	// shared between kubectl, k9s and lfk. If this test fails, kubectl
	// won't see the cache lfk wrote (and vice versa).
	cases := map[string]string{
		"https://kubernetes.docker.internal:6443": "kubernetes.docker.internal_6443",
		"https://1.2.3.4:8443":                    "1.2.3.4_8443",
	}
	for host, want := range cases {
		assert.Equal(t, want, cacheHostDir(host), "host=%s", host)
	}
}

func TestInvalidateDiscoveryCache_NilSafe(t *testing.T) {
	// Models without a real Client (test fixtures) call this on a nil
	// receiver during refresh — must not panic.
	var c *Client
	assert.NotPanics(t, func() {
		c.InvalidateDiscoveryCache("any")
	})
}

func TestInvalidateDiscoveryCache_MissingKey(t *testing.T) {
	// First-use case: shift+r before the context has ever been discovered.
	// Map is empty, the call should be a no-op rather than panicking.
	c := &Client{}
	assert.NotPanics(t, func() {
		c.InvalidateDiscoveryCache("never-seen")
	})
}

func TestInvalidateDiscoveryCache_PresentKey(t *testing.T) {
	// After a real discoveryForContext() created a cached client for the
	// context, Invalidate must:
	//  - run on the existing memoized client (i.e. not be a no-op),
	//  - not panic,
	//  - leave the entry in the map so subsequent reads still hit the
	//    same client.
	//
	// We prove "Invalidate runs on the right pointer" by reading from a
	// pre-seeded cache before the call (sanity that we own the path the
	// disk client reads from). The post-invalidate behavior of the flag
	// is client-go's responsibility, so we don't re-assert it here —
	// observing it would require a real API server (or a 30s timeout).
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	t.Setenv("KUBECACHEDIR", cacheDir)

	c := newKubeconfigClient(t, tmp)

	first, err := c.discoveryForContext("alpha")
	require.NoError(t, err)
	require.NotNil(t, first)

	seedServerGroupsCache(t, cacheDir, "alpha.example.test_6443", "before-invalidate.example.com")

	// Sanity: client reads from the seeded cache, confirming our path
	// resolution matches what the disk client actually uses.
	groups, err := first.ServerGroups()
	require.NoError(t, err)
	require.Len(t, groups.Groups, 1)
	require.Equal(t, "before-invalidate.example.com", groups.Groups[0].Name)

	assert.NotPanics(t, func() {
		c.InvalidateDiscoveryCache("alpha")
	})

	second, err := c.discoveryForContext("alpha")
	require.NoError(t, err)
	assert.Same(t, first, second, "Invalidate must not evict the memoized client")
}

func TestDiscoveryForContext_TestClientsetEscape(t *testing.T) {
	// When testClientset is set, discoveryForContext must bypass the
	// disk-cache path entirely and return the fake's Discovery() —
	// otherwise unit tests would touch ~/.kube/cache.
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	dc, err := c.discoveryForContext("any")
	require.NoError(t, err)
	assert.Same(t, cs.Discovery(), dc, "test escape must return the fake's Discovery client")

	// And it must not have created any disk cache state.
	assert.Nil(t, c.discoveryClients, "test escape must not initialize the disk-cache map")
}

func TestDiscoveryForContext_HonorsKUBECACHEDIR(t *testing.T) {
	// KUBECACHEDIR mirrors kubectl: when set, the cache lives under that
	// directory rather than ~/.kube/cache. Verified by pre-seeding a
	// servergroups.json at the expected sanitized-host path and confirming
	// the resulting discovery client reads from it. The disk client does
	// not create cache directories at construction time (only on first
	// write), so dir-existence alone wouldn't be a reliable signal.
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "custom-cache")
	t.Setenv("KUBECACHEDIR", cacheDir)

	c := newKubeconfigClient(t, tmp)

	seedServerGroupsCache(t, cacheDir, "alpha.example.test_6443", "kubecachedir-sentinel.example.com")

	dc, err := c.discoveryForContext("alpha")
	require.NoError(t, err)

	groups, err := dc.ServerGroups()
	require.NoError(t, err, "ServerGroups must read from KUBECACHEDIR-overridden disk cache")
	require.Len(t, groups.Groups, 1)
	assert.Equal(t, "kubecachedir-sentinel.example.com", groups.Groups[0].Name,
		"ServerGroups must return our pre-seeded sentinel — proves KUBECACHEDIR was honored")
}

func TestDiscoveryForContext_Memoizes(t *testing.T) {
	// Two calls for the same context must return the identical pointer —
	// otherwise every refresh would reopen the disk cache and rebuild the
	// HTTP transport, defeating the whole point of caching.
	tmp := t.TempDir()
	t.Setenv("KUBECACHEDIR", filepath.Join(tmp, "cache"))

	c := newKubeconfigClient(t, tmp)

	first, err := c.discoveryForContext("alpha")
	require.NoError(t, err)
	second, err := c.discoveryForContext("alpha")
	require.NoError(t, err)

	assert.Same(t, first, second, "discoveryForContext must memoize per-context")
}

func TestDiscoveryForContext_PerContextMemoization(t *testing.T) {
	// Different contexts must get distinct cached clients — they point at
	// different API servers and have separate on-disk cache directories.
	tmp := t.TempDir()
	t.Setenv("KUBECACHEDIR", filepath.Join(tmp, "cache"))

	c := newKubeconfigClient(t, tmp)

	alpha, err := c.discoveryForContext("alpha")
	require.NoError(t, err)
	beta, err := c.discoveryForContext("beta")
	require.NoError(t, err)

	assert.NotSame(t, alpha, beta, "different contexts must get distinct discovery clients")
}

// seedServerGroupsCache writes a fake servergroups.json under
// <cacheDir>/discovery/<host>/ that the disk client will treat as a
// fresh cache hit. Used to assert observable behavior (which cache
// directory the client reads from) without standing up an HTTP server.
func seedServerGroupsCache(t *testing.T, cacheDir, hostDir, sentinelGroupName string) {
	t.Helper()
	dir := filepath.Join(cacheDir, "discovery", hostDir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	body := `{"kind":"APIGroupList","apiVersion":"v1","groups":[{"name":"` +
		sentinelGroupName +
		`","versions":[{"groupVersion":"` + sentinelGroupName +
		`/v1","version":"v1"}],"preferredVersion":{"groupVersion":"` +
		sentinelGroupName + `/v1","version":"v1"}}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "servergroups.json"), []byte(body), 0o644))
}

// newKubeconfigClient builds a real Client backed by a two-context
// kubeconfig in tmp. Used by tests that exercise the disk-cache path
// (discoveryForContext goes through restConfigForContext, which needs a
// resolvable kubeconfig). The token-only auth means construction never
// touches the network.
func newKubeconfigClient(t *testing.T, tmp string) *Client {
	t.Helper()

	kubeconfig := filepath.Join(tmp, "kubeconfig")
	require.NoError(t, os.WriteFile(kubeconfig, []byte(`apiVersion: v1
kind: Config
current-context: alpha
clusters:
- name: cluster-alpha
  cluster:
    server: https://alpha.example.test:6443
    insecure-skip-tls-verify: true
- name: cluster-beta
  cluster:
    server: https://beta.example.test:6443
    insecure-skip-tls-verify: true
contexts:
- name: alpha
  context:
    cluster: cluster-alpha
    user: user-alpha
- name: beta
  context:
    cluster: cluster-beta
    user: user-beta
users:
- name: user-alpha
  user:
    token: alpha-token
- name: user-beta
  user:
    token: beta-token
`), 0o600))

	t.Setenv("KUBECONFIG", kubeconfig)

	c, err := NewClient("")
	require.NoError(t, err)
	return c
}
