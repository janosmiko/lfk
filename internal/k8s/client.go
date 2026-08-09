// Package k8s provides Kubernetes API access for the TUI application.
package k8s

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/singleflight"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
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
	// configMu guards the kubeconfig-derived snapshot fields below
	// (rawConfig, contexts, contextOrder, currentContext). They were
	// originally written once at NewClient and treated as read-only,
	// but ReloadKubeconfig can now update them mid-session whenever a
	// localcluster create/delete (or external kubectl mutation) lands.
	// Reads from concurrent tea.Cmd goroutines (GetContexts,
	// CurrentContext, ContextExists, DefaultNamespace) take RLock;
	// the reload path takes the write lock around the four-field
	// swap.
	configMu sync.RWMutex

	rawConfig    api.Config
	loadingRules *clientcmd.ClientConfigLoadingRules

	// contexts indexes every loaded context by its lfk display name.
	// Disambiguates the duplicate-name case that clientcmd's merge
	// silently collapses.
	contexts map[string]contextInfo

	// contextOrder preserves a deterministic display order for GetContexts.
	contextOrder []string

	// currentContext holds the display name of the global current-context.
	// Sourced from the first kubeconfig file in the precedence list, which
	// matches clientcmd's first-writer-wins merge rule for current-context.
	currentContext string

	// injectedClientset, injectedDynClient, and injectedMetaClient let tests
	// (and NewDemoClient) inject fake clients. When set, the corresponding
	// *ForContext helpers return these instead of building real clients from
	// the kubeconfig.
	injectedClientset  any // kubernetes.Interface (avoid import cycle in non-test code)
	injectedDynClient  any // dynamic.Interface
	injectedMetaClient any // metadata.Interface

	// demo is true when this Client was built by NewDemoClient (--demo flag).
	// It never talks to a real cluster; IsDemo lets the app layer surface a
	// badge instead of inspecting the injected fields directly.
	demo bool

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
	// Accessed concurrently from tea.Cmd goroutines (GetResources et al)
	// while the startup setter (and any future runtime-reload path) writes
	// it; an atomic load/store keeps the access race-free without a lock.
	secretLazyLoading atomic.Bool

	// kubesharkNamespaceOverride is the namespace probed for the kubeshark
	// hub Service in the Traffic Capture overlay. nil or empty pointer
	// value means "use the default" (see capture_backend_kubeshark.go's
	// kubesharkNamespace method). Set once at startup from
	// ui.ConfigKubesharkNamespace via SetKubesharkNamespace. Same
	// concurrency rationale as secretLazyLoading above.
	kubesharkNamespaceOverride atomic.Pointer[string]

	// Guarded by discoveryMu; concurrent tea.Cmd goroutines may discover
	// across different contexts.
	discoveryMu      sync.Mutex
	discoveryClients map[string]*disk.CachedDiscoveryClient

	// clientMu guards clientCache and clientCacheGen. Typed/dynamic/metadata
	// clients are expensive to build (kubeconfig parse + TLS transport +
	// ~1.8k allocs) and were previously rebuilt on every call, discarding
	// connection pooling and resetting the client-side QPS limiter each time.
	// (The exec-credential plugin for EKS/kubelogin runs lazily on the first
	// HTTP request, NOT during build, so it is never part of this critical
	// section.) clientCache memoizes clients per cache key (see clientCacheKey:
	// context, plus a "throttled" variant for the lower-rate security clients).
	// Invalidated on ReloadKubeconfig (the only mid-session config mutation)
	// and per-context via invalidateClientsForContext.
	//
	// The actual construction runs OUTSIDE clientMu via clientGroup (see
	// buildCachedClient): a slow build must never block other callers — in
	// particular the Bubble Tea Update goroutine, which builds the throttled
	// security clients synchronously in refreshSecuritySources. clientMu is
	// only ever held for microsecond map reads/writes around the build.
	clientMu    sync.Mutex
	clientCache map[string]*ctxClients
	// clientGroup coalesces concurrent builds for the same cache key+kind so a
	// burst of cold-cache callers runs ONE construction, not N. clientCacheGen
	// is bumped (under clientMu) on every cache invalidation; a build that
	// races an invalidate compares the generation it captured before building
	// and skips caching its now-stale result, so an invalidated client is
	// never served as a cache hit. (The old lock-across-build made invalidate
	// and build mutually exclusive; building outside the lock reintroduces the
	// race that the generation guard closes.)
	clientGroup    singleflight.Group
	clientCacheGen uint64

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

	// securityManager dispatches finding fetches across registered
	// SecuritySource adapters (Trivy Operator, Heuristic, PolicyReport,
	// Falco). Set at startup by the app layer via SetSecurityManager but
	// read from tea.Cmd goroutines, so it is an atomic pointer for the same
	// concurrency rationale as kubesharkNamespaceOverride above. Nil in
	// tests that don't exercise the security category.
	securityManager atomic.Pointer[security.Manager]

	// securityIgnoreMu guards ignoreChecker + showIgnored. Both are read on
	// every getSecurityFindings call (which runs on a tea.Cmd goroutine) and
	// written by app-layer handlers when the user adds/removes ignore rules
	// or toggles the "show ignored" overlay — so the access is genuinely
	// concurrent and race-prone without synchronization. Read via
	// securityIgnoreSnapshot.
	securityIgnoreMu sync.RWMutex
	// ignoreChecker filters findings/groups marked ignored by the user's
	// security ignore-list. Nil means "show everything." Read via
	// securityIgnoreSnapshot.
	ignoreChecker IgnoreChecker
	// showIgnored, when true, includes ignored findings in the output
	// (rendered with an "ignored" indicator) instead of hiding them. Off
	// by default; toggled via the security ignore-list overlay. Read via
	// securityIgnoreSnapshot.
	showIgnored bool
}

// IsDemo reports whether this Client was built by NewDemoClient (--demo
// flag) rather than against a real kubeconfig. Nil-safe so callers don't
// need a guard before checking a possibly-unset client.
func (c *Client) IsDemo() bool {
	return c != nil && c.demo
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
// Typically called once at startup after loading the config file, but safe
// to call from any goroutine — the underlying field is atomic.
func (c *Client) SetSecretLazyLoading(enabled bool) {
	c.secretLazyLoading.Store(enabled)
}

// SetKubesharkNamespace overrides the namespace probed for Service
// kubeshark-hub in the Traffic Capture overlay. Empty string keeps the
// default ("kubeshark"). Typically called once at startup after the
// config file has been parsed, but safe to call from any goroutine —
// the underlying field is an atomic pointer.
func (c *Client) SetKubesharkNamespace(ns string) {
	c.kubesharkNamespaceOverride.Store(&ns)
}

// SetSecurityManager wires the security source manager into the client so
// GetResources can dispatch _security virtual API group calls. Pass nil to
// disable the security category. Called once at startup by the app layer.
func (c *Client) SetSecurityManager(m *security.Manager) {
	c.securityManager.Store(m)
}

// SecurityManager returns the wired security manager, or nil if SetSecurityManager
// was never called. Callers that need to fetch findings for the dashboard should
// go through this accessor rather than the unexported field.
func (c *Client) SecurityManager() *security.Manager {
	return c.securityManager.Load()
}

// SetIgnoreChecker installs the ignore-list filter consulted when converting
// findings into Items. Pass nil to disable filtering.
func (c *Client) SetIgnoreChecker(checker IgnoreChecker) {
	c.securityIgnoreMu.Lock()
	defer c.securityIgnoreMu.Unlock()
	c.ignoreChecker = checker
}

// SetShowIgnored toggles whether ignored findings are surfaced in the
// security view (true: include with marker, false: hide).
func (c *Client) SetShowIgnored(show bool) {
	c.securityIgnoreMu.Lock()
	defer c.securityIgnoreMu.Unlock()
	c.showIgnored = show
}

// securityIgnoreSnapshot returns the current ignore filter and showIgnored
// flag in a single locked read, so getSecurityFindings always sees a
// consistent pair even if SetIgnoreChecker / SetShowIgnored land between
// the two field reads.
func (c *Client) securityIgnoreSnapshot() (IgnoreChecker, bool) {
	c.securityIgnoreMu.RLock()
	defer c.securityIgnoreMu.RUnlock()
	return c.ignoreChecker, c.showIgnored
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
//  1. KUBECONFIG env var. When set and kubeconfigExclusive is true (the
//     default), it is exclusive (kubectl/k9s semantics): step 2 and the
//     default of step 3 are skipped.
//  2. ~/.kube/config (skipped under an exclusive KUBECONFIG).
//  3. All files in each kubeconfigDirs entry (recursively; symlinks to directories are followed).
//     Defaults to [~/.kube/config.d/] when the slice is empty and no exclusive KUBECONFIG applies.
//
// kubeconfigOverride (--kubeconfig) beats everything: when non-empty it is
// the only file loaded.
func NewClient(kubeconfigOverride string, kubeconfigDirs []string, kubeconfigExclusive bool) (*Client, error) {
	kubeconfigPaths := resolveKubeconfigPaths(kubeconfigOverride, kubeconfigDirs, kubeconfigExclusive)

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

// ReloadKubeconfig re-reads the kubeconfig files from disk and refreshes
// the cached context list. NewClient loads kubeconfig once at startup;
// callers that mutate kubeconfig externally (e.g. minikube/kind/k3d
// creating a new cluster) must call this so subsequent GetContexts /
// ContextExists / etc. observe the new state. Safe to call repeatedly;
// idempotent when nothing on disk changed.
func (c *Client) ReloadKubeconfig() error {
	if c.loadingRules == nil || len(c.loadingRules.Precedence) == 0 {
		return nil
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(c.loadingRules, &clientcmd.ConfigOverrides{})
	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return fmt.Errorf("reloading kubeconfig: %w", err)
	}
	if len(rawConfig.Contexts) == 0 {
		// Reloaded config has no contexts. Either the kubeconfig was
		// emptied externally (rare) or the loading rules point at a
		// stub path (test fixtures using /dev/null). In both cases,
		// preserving existing in-memory state is safer than wiping it
		// — GetContexts has a rawConfig-fallback path that the tests
		// rely on, and a user who genuinely deleted all contexts can
		// just restart lfk.
		return nil
	}
	contexts, order, current := collectContexts(c.loadingRules.Precedence, rawConfig.CurrentContext)
	c.configMu.Lock()
	c.rawConfig = rawConfig
	c.contexts = contexts
	c.contextOrder = order
	c.currentContext = current
	c.configMu.Unlock()
	// Drop cached clients: they were built from the previous kubeconfig
	// snapshot (endpoint, credentials, exec plugin). A reload can change any
	// of those, so the next builder call must reconstruct. Done outside
	// configMu — invalidateClientCache takes its own clientMu — to keep the
	// two locks independent and avoid a lock-ordering constraint.
	c.invalidateClientCache()
	return nil
}

// GetContexts returns all available kube contexts using their lfk display
// names (which match the original names when there are no collisions and are
// disambiguated as "name (basename)" when several files declare the same
// context name).
func (c *Client) GetContexts() ([]model.Item, error) {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
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
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	if c.currentContext != "" {
		return c.currentContext
	}
	return c.rawConfig.CurrentContext
}

// ContextExists reports whether the lfk display name is defined.
func (c *Client) ContextExists(displayName string) bool {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	if _, ok := c.contexts[displayName]; ok {
		return true
	}
	// Fallback for clients constructed without collectContexts (tests).
	_, ok := c.rawConfig.Contexts[displayName]
	return ok
}

func (c *Client) contextNamespaceLocked(displayName string) (string, bool) {
	if info, ok := c.contexts[displayName]; ok {
		ns := strings.TrimSpace(info.namespace)
		return ns, ns != ""
	}
	if ctx, ok := c.rawConfig.Contexts[displayName]; ok && ctx != nil {
		ns := strings.TrimSpace(ctx.Namespace)
		return ns, ns != ""
	}
	return "", false
}

// ContextNamespace returns the namespace explicitly configured on the given
// lfk display context. The boolean is false when the context exists but does
// not pin a namespace, letting callers distinguish "unset" from Kubernetes'
// conventional "default" fallback.
func (c *Client) ContextNamespace(displayName string) (string, bool) {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	return c.contextNamespaceLocked(displayName)
}

// DefaultNamespace returns the namespace configured for the given lfk display
// name, falling back to "default" if none is set.
func (c *Client) DefaultNamespace(displayName string) string {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	if ns, ok := c.contextNamespaceLocked(displayName); ok {
		return ns
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

// GetResourcesUnion fetches resources from multiple contexts in parallel and
// merges the results. Each item is stamped with ClusterName for drill-down
// routing; the UI renders that field as the first-class CONTEXT column.
// Partial results are returned alongside an errors.Join of every per-context
// failure, so the status bar can surface "2 of 8 contexts failed: …" instead
// of silently truncating to the first error.
func (c *Client) GetResourcesUnion(ctx context.Context, contexts []string, namespace string, rt model.ResourceTypeEntry) ([]model.Item, error) {
	type result struct {
		items []model.Item
		err   error
		ctx   string
	}
	results := make([]result, len(contexts))
	var wg sync.WaitGroup
	wg.Add(len(contexts))
	for i, kctx := range contexts {
		go func(idx int, contextName string) {
			defer wg.Done()
			items, err := c.GetResources(ctx, contextName, namespace, rt)
			if err != nil {
				results[idx] = result{ctx: contextName, err: err}
				return
			}
			for j := range items {
				items[j].ClusterName = contextName
			}
			results[idx] = result{items: items, ctx: contextName}
		}(i, kctx)
	}
	wg.Wait()

	var merged []model.Item
	var errs []error
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Errorf("context %q: %w", r.ctx, r.err))
		}
		merged = append(merged, r.items...)
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Name != merged[j].Name {
			return merged[i].Name < merged[j].Name
		}
		if merged[i].ClusterName != merged[j].ClusterName {
			return merged[i].ClusterName < merged[j].ClusterName
		}
		return merged[i].Namespace < merged[j].Namespace
	})
	return merged, errors.Join(errs...)
}
