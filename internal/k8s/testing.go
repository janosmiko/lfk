package k8s

import (
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/janosmiko/lfk/internal/k8s/demo"
)

// NewTestClient creates a Client with injected fake clients for testing.
// cs should be a kubernetes.Interface (e.g. k8sfake.NewClientset()),
// dyn should be a dynamic.Interface (e.g. dynamicfake.NewSimpleDynamicClient()).
// Both may be nil if the test does not exercise those code paths.
// To inject a fake metadata client, set the injectedMetaClient field directly
// on the returned *Client (or use NewTestClientWithMeta).
func NewTestClient(cs, dyn any) *Client {
	return &Client{
		rawConfig: api.Config{
			Contexts: map[string]*api.Context{
				"test-ctx": {Namespace: "default", Cluster: "test-cluster", AuthInfo: "test-user"},
			},
			CurrentContext: "test-ctx",
		},
		loadingRules: &clientcmd.ClientConfigLoadingRules{
			Precedence: []string{"/dev/null"},
		},
		injectedClientset: cs,
		injectedDynClient: dyn,
		testHostByDisplay: map[string]string{"test-ctx": "https://test-cluster.example.local:6443"},
	}
}

// NewDemoClient builds a Client backed by the internal/k8s/demo fake
// clientset and dynamic client, for the --demo flag. It reuses
// NewTestClient's kubeconfig isolation (loadingRules pointed at /dev/null,
// a synthesized context) so a demo Client can never read the user's real
// kubeconfig, then marks itself IsDemo so the app layer can show a badge.
func NewDemoClient() (*Client, error) {
	c := NewTestClient(demo.NewClientset(), demo.NewDynamicClient())
	c.demo = true
	return c, nil
}

// SetTestHostForContext registers a synthetic host URL for a context so
// HostForContext returns a deterministic value without going through
// kubeconfig resolution. Intended for tests; production code uses real
// kubeconfig data.
func (c *Client) SetTestHostForContext(displayName, host string) {
	if c == nil {
		return
	}
	if c.testHostByDisplay == nil {
		c.testHostByDisplay = make(map[string]string)
	}
	c.testHostByDisplay[displayName] = host
}

// AddTestContext registers a synthetic context with its host URL so tests
// can simulate multi-cluster setups. The context becomes visible to
// GetContexts and HostForContext returns the supplied host.
func (c *Client) AddTestContext(displayName, host string) {
	c.AddTestContextWithNamespace(displayName, host, "default")
}

// AddTestContextWithNamespace is AddTestContext with an explicit kubeconfig
// namespace override. Pass an empty namespace to model a context that leaves
// namespace unset.
func (c *Client) AddTestContextWithNamespace(displayName, host, namespace string) {
	if c == nil {
		return
	}
	if c.rawConfig.Contexts == nil {
		c.rawConfig.Contexts = make(map[string]*api.Context)
	}
	c.rawConfig.Contexts[displayName] = &api.Context{
		Namespace: namespace,
		Cluster:   displayName + "-cluster",
		AuthInfo:  displayName + "-user",
	}
	c.SetTestHostForContext(displayName, host)
}
