// Package k8s provides Kubernetes API access for the TUI application.
package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/janosmiko/lfk/internal/model"
)

// contextInfo decorates a kubeconfig context with its source file plus a
// display name that is unique across all loaded files. When several
// kubeconfigs declare the same context name, lfk disambiguates the display
// name (e.g. "dev (dev-envs)") so the user can still see and select each one
// — issue #23.
type contextInfo struct {
	// display is the unique name shown in the lfk UI. Equal to original
	// when no other file defines a context with the same name; otherwise of
	// the form "original (basename)".
	display string
	// original is the context name as written in the source kubeconfig
	// file. Subprocesses (kubectl --context, helm --kube-context) must be
	// passed this value, since the disambiguated display name only exists
	// inside lfk.
	original string
	// sourcePath is the kubeconfig file that defines the context.
	sourcePath string
	// namespace is the namespace recorded on the source file's context, or
	// empty when the file does not pin one.
	namespace string
}

// Client wraps Kubernetes API access.
type Client struct {
	rawConfig    api.Config
	loadingRules *clientcmd.ClientConfigLoadingRules

	// contexts indexes every loaded context by its lfk display name. Built
	// once at NewClient by collectContexts and treated as read-only after
	// construction so concurrent reads from tea.Cmd goroutines stay
	// race-free. Disambiguates the duplicate-name case that clientcmd's
	// merge silently collapses.
	contexts map[string]contextInfo

	// contextOrder preserves a deterministic display order for GetContexts.
	contextOrder []string

	// currentContext holds the display name of the global current-context.
	// Sourced from the first kubeconfig file in the precedence list, which
	// matches clientcmd's first-writer-wins merge rule for current-context.
	currentContext string

	// testClientset, testDynClient, and testMetaClient allow tests to inject
	// fake clients. When set, the corresponding *ForContext helpers return
	// these instead of building real clients from the kubeconfig.
	testClientset  any // kubernetes.Interface (avoid import cycle in non-test code)
	testDynClient  any // dynamic.Interface
	testMetaClient any // metadata.Interface

	// testPromQuery, when set, replaces the real Service.ProxyGet
	// pipeline used by the right-sizing Prometheus strategies. Tests
	// inject a stub that returns canned PromQL responses without
	// needing a working Service object on the fake clientset.
	// Signature: ctx, contextName, promQL -> JSON body bytes.
	testPromQuery func(ctx context.Context, contextName, query string) ([]byte, error)

	// testHostByDisplay, when set, lets tests bypass kubeconfig host
	// resolution in HostForContext. Most fake test clients are constructed
	// without Cluster definitions (no server URL), so a real
	// restConfigForContext call would fail; this map provides synthetic
	// answers keyed by display name.
	testHostByDisplay map[string]string

	// describeOverride, when set, replaces the kubectl-describe call inside
	// GetCrashInvestigation so tests don't need a real kubectl on PATH.
	// Nil in production — the real path goes through DescribePod.
	describeOverride func(ctx context.Context, contextName, namespace, podName string) (string, error)

	// secretLazyLoading, when true, routes Secret listing through the
	// metadata-only API so decoded values are lazy-fetched on hover instead
	// of being pulled up-front. Configured via the secret_lazy_loading
	// option; off by default so the list behaves like every other resource.
	secretLazyLoading bool

	// Guarded by discoveryMu; concurrent tea.Cmd goroutines may discover
	// across different contexts.
	discoveryMu      sync.Mutex
	discoveryClients map[string]*disk.CachedDiscoveryClient

	// informerMu guards informerMode + informers. Writes happen at most
	// once (SetInformerCacheMode at startup); reads happen on every
	// GetResources. RWMutex is overkill for the call rate but documents
	// the intent — and a future runtime config-reload path can flip the
	// mode safely without retrofitting synchronization across callers.
	informerMu sync.RWMutex
	// informerMode selects the routing strategy for GetResources. See
	// InformerCacheMode for the three values; default (zero value "") is
	// treated as InformerCacheAuto so users get the issue #86 win without
	// any config change. Read via informerSnapshot.
	informerMode InformerCacheMode
	// informers is built lazily the first time the mode resolves to
	// anything other than InformerCacheOff. Stays nil for tests that do not
	// touch the cache, keeping the existing fake-client surface unchanged.
	// Read via informerSnapshot.
	informers *informerCache
}

// informerSnapshot returns the current routing config as a single
// consistent pair so callers don't observe a half-updated state if a
// future SetInformerCacheMode lands between reads.
func (c *Client) informerSnapshot() (InformerCacheMode, *informerCache) {
	c.informerMu.RLock()
	defer c.informerMu.RUnlock()
	return c.informerMode, c.informers
}

// SetSecretLazyLoading toggles the metadata-only list path for Secrets.
// Typically called once at startup after loading the config file.
func (c *Client) SetSecretLazyLoading(enabled bool) {
	c.secretLazyLoading = enabled
}

// SetInformerCacheMode selects how GetResources routes its list requests.
// See InformerCacheMode for the three values: off, auto, and always. Unknown
// values fall back to auto — that's the safest default because auto-mode
// stays out of the way until a list is actually large.
//
// The mode argument is normalized (trimmed + lower-cased) before matching,
// so callers passing "Always" or "  off " resolve to the same modes the
// YAML unmarshaler accepts. Without that, programmatic callers would get
// silently dropped to the auto fallback on a casing mismatch.
//
// Issue #86: on a 7k-pod cluster the cached path turns each namespace switch
// from a 1–2s round trip into an in-process slice walk. Auto-mode promotes
// to the cache automatically once a list crosses 1000 items, and demotes
// back to a direct list (closing the watch) after three consecutive cached
// lists fall below 500. The hysteresis prevents flapping when a list size
// hovers near the threshold.
//
// Typically called once at startup after loading the config file. Safe
// to call concurrently with GetResources thanks to informerMu.
func (c *Client) SetInformerCacheMode(mode InformerCacheMode) {
	normalized := InformerCacheMode(strings.ToLower(strings.TrimSpace(string(mode))))
	c.informerMu.Lock()
	defer c.informerMu.Unlock()
	switch normalized {
	case InformerCacheOff, InformerCacheAuto, InformerCacheAlways:
		c.informerMode = normalized
	default:
		c.informerMode = InformerCacheAuto
	}
	if c.informerMode != InformerCacheOff && c.informers == nil {
		c.informers = newInformerCache(c.dynamicForContext)
	}
}

// Shutdown closes any background watches the client started (informer cache,
// future stream subscribers). Idempotent and safe to call from a defer in
// main.go even when no informers were ever started.
func (c *Client) Shutdown() {
	_, infs := c.informerSnapshot()
	if infs != nil {
		infs.Stop()
	}
}

// NewClient creates a new Kubernetes client, loading configs from:
// 1. KUBECONFIG env var
// 2. ~/.kube/config
// 3. All files in ~/.kube/config.d/ (recursively; symlinks to directories are followed)
func NewClient(kubeconfigOverride string) (*Client, error) {
	var kubeconfigPaths []string
	if kubeconfigOverride != "" {
		kubeconfigPaths = []string{kubeconfigOverride}
	} else {
		kubeconfigPaths = buildKubeconfigPaths()
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{
		Precedence: kubeconfigPaths,
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	contexts, order, current := collectContexts(kubeconfigPaths, rawConfig.CurrentContext)

	return &Client{
		rawConfig:      rawConfig,
		loadingRules:   loadingRules,
		contexts:       contexts,
		contextOrder:   order,
		currentContext: current,
	}, nil
}

// GetContexts returns all available kube contexts using their lfk display
// names (which match the original names when there are no collisions and are
// disambiguated as "name (basename)" when several files declare the same
// context name).
func (c *Client) GetContexts() ([]model.Item, error) {
	if len(c.contexts) == 0 {
		// Fallback for tests that construct a Client directly without
		// running NewClient: surface whatever rawConfig holds.
		items := make([]model.Item, 0, len(c.rawConfig.Contexts))
		for name := range c.rawConfig.Contexts {
			status := ""
			if name == c.rawConfig.CurrentContext {
				status = "current"
			}
			items = append(items, model.Item{Name: name, Status: status})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		return items, nil
	}
	items := make([]model.Item, 0, len(c.contextOrder))
	for _, display := range c.contextOrder {
		status := ""
		if display == c.currentContext {
			status = "current"
		}
		items = append(items, model.Item{Name: display, Status: status})
	}
	return items, nil
}

// CurrentContext returns the lfk display name of the current context.
func (c *Client) CurrentContext() string {
	if c.currentContext != "" {
		return c.currentContext
	}
	return c.rawConfig.CurrentContext
}

// ContextExists reports whether the lfk display name is defined.
func (c *Client) ContextExists(displayName string) bool {
	if _, ok := c.contexts[displayName]; ok {
		return true
	}
	// Fallback for clients constructed without collectContexts (tests).
	_, ok := c.rawConfig.Contexts[displayName]
	return ok
}

// DefaultNamespace returns the namespace configured for the given lfk display
// name, falling back to "default" if none is set.
func (c *Client) DefaultNamespace(displayName string) string {
	if info, ok := c.contexts[displayName]; ok && info.namespace != "" {
		return info.namespace
	}
	if ctx, ok := c.rawConfig.Contexts[displayName]; ok && ctx != nil && ctx.Namespace != "" {
		return ctx.Namespace
	}
	return "default"
}

// GetNamespaces returns namespaces for the given context.
func (c *Client) GetNamespaces(ctx context.Context, contextName string) ([]model.Item, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	nsList, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}

	items := make([]model.Item, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		items = append(items, model.Item{Name: ns.Name, Status: string(ns.Status.Phase)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}
