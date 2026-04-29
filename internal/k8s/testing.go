package k8s

import (
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// NewTestClient creates a Client with injected fake clients for testing.
// cs should be a kubernetes.Interface (e.g. k8sfake.NewClientset()),
// dyn should be a dynamic.Interface (e.g. dynamicfake.NewSimpleDynamicClient()).
// Both may be nil if the test does not exercise those code paths.
// To inject a fake metadata client, set the testMetaClient field directly on
// the returned *Client (or use NewTestClientWithMeta).
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
		testClientset:     cs,
		testDynClient:     dyn,
		testHostByDisplay: map[string]string{"test-ctx": "https://test-cluster.example.local:6443"},
	}
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
	if c == nil {
		return
	}
	if c.rawConfig.Contexts == nil {
		c.rawConfig.Contexts = make(map[string]*api.Context)
	}
	c.rawConfig.Contexts[displayName] = &api.Context{
		Namespace: "default",
		Cluster:   displayName + "-cluster",
		AuthInfo:  displayName + "-user",
	}
	c.SetTestHostForContext(displayName, host)
}
