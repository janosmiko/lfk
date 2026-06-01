// Package k8s — client_cache.go
// Per-context memoization of the typed, dynamic, and metadata clients. Building
// these is expensive (kubeconfig parse + TLS transport + ~1.8k allocs, and an
// exec-credential plugin invocation for EKS/kubelogin contexts) and was
// previously done on every call — which also reset the client-side QPS limiter
// and discarded HTTP connection pooling between calls. Caching restores both
// and makes the QPS/Burst knobs a real cross-call rate cap. Mirrors the
// existing per-context discovery-client cache (discovery_cache.go).
package k8s

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
)

// ctxClients holds the cached clients for one cache key. Fields are built
// lazily — a key may have only the clients that have been requested so far.
type ctxClients struct {
	clientset kubernetes.Interface
	dynamic   dynamic.Interface
	metadata  metadata.Interface
}

// clientCacheKey derives the cache key for a context. The throttled
// (security) clients carry a different rate limit, so they must not share an
// instance with the foreground clients — they get a distinct key.
func clientCacheKey(contextName string, throttled bool) string {
	if throttled {
		return contextName + "\x00throttled"
	}
	return contextName
}

// cachedClientsLocked returns the ctxClients for key, creating an empty entry
// if absent. Caller must hold c.clientMu.
func (c *Client) cachedClientsLocked(key string) *ctxClients {
	if c.clientCache == nil {
		c.clientCache = make(map[string]*ctxClients)
	}
	entry, ok := c.clientCache[key]
	if !ok {
		entry = &ctxClients{}
		c.clientCache[key] = entry
	}
	return entry
}

// buildClientset constructs a typed clientset from cfg with the
// context-tagging HTTP client attached (so exec-credential failures are
// attributed to the cluster in logs). Shared by the foreground and throttled
// paths so neither skips the tagging wrapper.
func (c *Client) buildClientset(contextName string, cfg *rest.Config) (kubernetes.Interface, error) {
	httpClient, err := taggedHTTPClient(cfg, contextName)
	if err != nil {
		return nil, fmt.Errorf("creating http client: %w", err)
	}
	cs, err := kubernetes.NewForConfigAndClient(cfg, httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating clientset: %w", err)
	}
	return cs, nil
}

// buildDynamic constructs a dynamic client from cfg with the context-tagging
// HTTP client attached. See buildClientset.
func (c *Client) buildDynamic(contextName string, cfg *rest.Config) (dynamic.Interface, error) {
	httpClient, err := taggedHTTPClient(cfg, contextName)
	if err != nil {
		return nil, fmt.Errorf("creating http client: %w", err)
	}
	dc, err := dynamic.NewForConfigAndClient(cfg, httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}
	return dc, nil
}

// buildMetadata constructs a metadata-only client from cfg with the
// context-tagging HTTP client attached.
func (c *Client) buildMetadata(contextName string, cfg *rest.Config) (metadata.Interface, error) {
	httpClient, err := taggedHTTPClient(cfg, contextName)
	if err != nil {
		return nil, fmt.Errorf("creating http client: %w", err)
	}
	mc, err := metadata.NewForConfigAndClient(cfg, httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating metadata client: %w", err)
	}
	return mc, nil
}

// cachedClientset returns the memoized typed clientset for (contextName,
// throttled), building and caching it on first use. restConfig supplies the
// rate-appropriate rest.Config (foreground or throttled) and is only invoked
// on a cache miss.
func (c *Client) cachedClientset(contextName string, throttled bool, restConfig func() (*rest.Config, error)) (kubernetes.Interface, error) {
	key := clientCacheKey(contextName, throttled)
	c.clientMu.Lock()
	defer c.clientMu.Unlock()
	entry := c.cachedClientsLocked(key)
	if entry.clientset != nil {
		return entry.clientset, nil
	}
	cfg, err := restConfig()
	if err != nil {
		return nil, err
	}
	cs, err := c.buildClientset(contextName, cfg)
	if err != nil {
		return nil, err
	}
	entry.clientset = cs
	return cs, nil
}

// cachedDynamic returns the memoized dynamic client for (contextName,
// throttled). See cachedClientset.
func (c *Client) cachedDynamic(contextName string, throttled bool, restConfig func() (*rest.Config, error)) (dynamic.Interface, error) {
	key := clientCacheKey(contextName, throttled)
	c.clientMu.Lock()
	defer c.clientMu.Unlock()
	entry := c.cachedClientsLocked(key)
	if entry.dynamic != nil {
		return entry.dynamic, nil
	}
	cfg, err := restConfig()
	if err != nil {
		return nil, err
	}
	dc, err := c.buildDynamic(contextName, cfg)
	if err != nil {
		return nil, err
	}
	entry.dynamic = dc
	return dc, nil
}

// cachedMetadata returns the memoized metadata-only client for contextName
// (foreground rate only; no throttled metadata variant is used).
func (c *Client) cachedMetadata(contextName string, restConfig func() (*rest.Config, error)) (metadata.Interface, error) {
	key := clientCacheKey(contextName, false)
	c.clientMu.Lock()
	defer c.clientMu.Unlock()
	entry := c.cachedClientsLocked(key)
	if entry.metadata != nil {
		return entry.metadata, nil
	}
	cfg, err := restConfig()
	if err != nil {
		return nil, err
	}
	mc, err := c.buildMetadata(contextName, cfg)
	if err != nil {
		return nil, err
	}
	entry.metadata = mc
	return mc, nil
}

// invalidateClientCache drops every cached client. Called from ReloadKubeconfig
// after the kubeconfig snapshot is swapped, so the next call to any builder
// reconstructs against the fresh config (new endpoint/credentials).
func (c *Client) invalidateClientCache() {
	c.clientMu.Lock()
	defer c.clientMu.Unlock()
	c.clientCache = nil
}

// invalidateClientsForContext drops the cached clients for a single context
// (both the foreground and throttled variants), leaving other contexts intact.
func (c *Client) invalidateClientsForContext(contextName string) {
	c.clientMu.Lock()
	defer c.clientMu.Unlock()
	delete(c.clientCache, clientCacheKey(contextName, false))
	delete(c.clientCache, clientCacheKey(contextName, true))
}
