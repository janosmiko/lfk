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
	"strconv"

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

// buildCachedClient returns the memoized client of type T for (key, kind),
// constructing it on a miss. The expensive build runs OUTSIDE clientMu: the
// lock is held only for the microsecond map read on entry and the map write on
// store, so a slow construction (kubeconfig parse + TLS transport assembly)
// never blocks another caller — including the Bubble Tea Update goroutine,
// which builds the throttled security clients synchronously. Concurrent builds
// for the same (key, kind) are coalesced by clientGroup so a cold-cache burst
// runs one construction, not N.
//
// get/set read and write the kind-specific ctxClients field. Build performs the
// actual construction. kind disambiguates the singleflight key so a clientset
// and a dynamic build for the same cache key never collapse into each other.
func buildCachedClient[T comparable](
	c *Client,
	key, kind string,
	get func(*ctxClients) T,
	set func(*ctxClients, T),
	build func() (T, error),
) (T, error) {
	var zero T

	// Fast path: a hit returns without entering the singleflight group or the
	// build, holding clientMu only for the map read. The generation is
	// snapshotted under the same lock so it is consistent with the miss.
	c.clientMu.Lock()
	if entry, ok := c.clientCache[key]; ok {
		if v := get(entry); v != zero {
			c.clientMu.Unlock()
			return v, nil
		}
	}
	gen := c.clientCacheGen
	c.clientMu.Unlock()

	// The singleflight key includes BOTH kind and gen. clientCache itself stays
	// keyed by key alone (one ctxClients per context, get/set select the field):
	//   - kind: a clientset and a dynamic build for the same context must be
	//     independent flights, or one caller gets the wrong type.
	//   - gen: a caller that arrives after an invalidate (newer gen) must NOT
	//     join a flight that is still building against the now-stale config —
	//     singleflight would hand it that pre-invalidate client. A newer gen
	//     means a different key, hence a fresh flight built against fresh config.
	sfKey := key + "\x00" + kind + "\x00" + strconv.FormatUint(gen, 10)
	res, err, _ := c.clientGroup.Do(sfKey, func() (any, error) {
		// Re-check under the lock: another caller may have finished the build
		// between our fast-path miss and entering the group.
		c.clientMu.Lock()
		if entry, ok := c.clientCache[key]; ok {
			if v := get(entry); v != zero {
				c.clientMu.Unlock()
				return v, nil
			}
		}
		c.clientMu.Unlock()

		built, berr := build()
		if berr != nil {
			return zero, berr
		}

		c.clientMu.Lock()
		// If an invalidate (ReloadKubeconfig / invalidateClientsForContext)
		// bumped the generation while we built, our client is against stale
		// config: hand it to this flight's callers (all of whom snapshotted the
		// same pre-invalidate gen, so their request predates the change) but do
		// NOT cache it — the next caller snapshots the newer gen, takes a fresh
		// flight, and rebuilds against the fresh config.
		if c.clientCacheGen == gen {
			set(c.cachedClientsLocked(key), built)
		}
		c.clientMu.Unlock()
		return built, nil
	})
	if err != nil {
		return zero, err
	}
	return res.(T), nil
}

// cachedClientset returns the memoized typed clientset for (contextName,
// throttled), building and caching it on first use. restConfig supplies the
// rate-appropriate rest.Config (foreground or throttled) and is only invoked
// on a cache miss.
func (c *Client) cachedClientset(contextName string, throttled bool, restConfig func() (*rest.Config, error)) (kubernetes.Interface, error) {
	return buildCachedClient(c, clientCacheKey(contextName, throttled), "clientset",
		func(e *ctxClients) kubernetes.Interface { return e.clientset },
		func(e *ctxClients, v kubernetes.Interface) { e.clientset = v },
		func() (kubernetes.Interface, error) {
			cfg, err := restConfig()
			if err != nil {
				return nil, err
			}
			return c.buildClientset(contextName, cfg)
		},
	)
}

// cachedDynamic returns the memoized dynamic client for (contextName,
// throttled). See cachedClientset.
func (c *Client) cachedDynamic(contextName string, throttled bool, restConfig func() (*rest.Config, error)) (dynamic.Interface, error) {
	return buildCachedClient(c, clientCacheKey(contextName, throttled), "dynamic",
		func(e *ctxClients) dynamic.Interface { return e.dynamic },
		func(e *ctxClients, v dynamic.Interface) { e.dynamic = v },
		func() (dynamic.Interface, error) {
			cfg, err := restConfig()
			if err != nil {
				return nil, err
			}
			return c.buildDynamic(contextName, cfg)
		},
	)
}

// cachedMetadata returns the memoized metadata-only client for contextName
// (foreground rate only, no throttled metadata variant is used).
func (c *Client) cachedMetadata(contextName string, restConfig func() (*rest.Config, error)) (metadata.Interface, error) {
	return buildCachedClient(c, clientCacheKey(contextName, false), "metadata",
		func(e *ctxClients) metadata.Interface { return e.metadata },
		func(e *ctxClients, v metadata.Interface) { e.metadata = v },
		func() (metadata.Interface, error) {
			cfg, err := restConfig()
			if err != nil {
				return nil, err
			}
			return c.buildMetadata(contextName, cfg)
		},
	)
}

// invalidateClientCache drops every cached client. Called from ReloadKubeconfig
// after the kubeconfig snapshot is swapped, so the next call to any builder
// reconstructs against the fresh config (new endpoint/credentials). Bumping
// clientCacheGen makes any build currently in flight (now against stale config)
// skip caching its result — see buildCachedClient's generation guard.
func (c *Client) invalidateClientCache() {
	c.clientMu.Lock()
	defer c.clientMu.Unlock()
	c.clientCache = nil
	c.clientCacheGen++
}

// invalidateClientsForContext drops the cached clients for a single context
// (both the foreground and throttled variants), leaving other contexts intact.
// The generation bump applies process-wide (not per-context) — a coarse but
// safe choice: at worst an unrelated context's racing build rebuilds once.
//
//nolint:unparam // per-context invalidation API; contextName is constant only because current callers are tests
func (c *Client) invalidateClientsForContext(contextName string) {
	c.clientMu.Lock()
	defer c.clientMu.Unlock()
	delete(c.clientCache, clientCacheKey(contextName, false))
	delete(c.clientCache, clientCacheKey(contextName, true))
	c.clientCacheGen++
}
