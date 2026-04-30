package k8s

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/janosmiko/lfk/internal/model"
)

// Default resync period for informers: zero means "never resync from a full
// list" — we still see live updates via the watch channel. Setting a non-zero
// resync period would replay every cached object through the event handlers
// on a timer, which costs CPU on a 7k-pod cluster without giving the UI any
// new information beyond what the watch already delivers.
const informerResyncPeriod = time.Duration(0)

// initialSyncTimeout caps how long the first GetResources call after enabling
// the cache will block waiting for the informer's initial LIST to complete.
// On a sluggish API server this prevents the UI from hanging indefinitely on
// the first namespace switch — instead we fall back to a direct list and try
// the cache again on the next request, by which time it has typically synced.
const initialSyncTimeout = 30 * time.Second

// InformerCacheMode controls how GetResources is routed: never use the cache,
// always use it, or auto-promote to it on large lists and auto-demote back
// to direct lists when a resource type shrinks below the demote threshold.
type InformerCacheMode string

// Recognised modes for the informer_cache config option. The string values
// are also what the config file accepts.
const (
	// InformerCacheOff routes every list directly to the apiserver. Matches
	// kubectl semantics; recommended when the apiserver dislikes extra
	// watch traffic.
	InformerCacheOff InformerCacheMode = "off"
	// InformerCacheAuto starts in direct-list mode per (context, GVR) and
	// promotes to a shared informer once a list returns more items than
	// autoPromoteAt. The watch is closed (demoted) again after
	// autoDemoteAfterN consecutive cached lists fall below autoDemoteBelow,
	// so a one-off large list does not strand a permanent watch.
	InformerCacheAuto InformerCacheMode = "auto"
	// InformerCacheAlways routes every list through the informer cache,
	// starting watches eagerly on first use. Matches the always-on behaviour
	// from issue #86's first cut; use when you know the cluster is large.
	InformerCacheAlways InformerCacheMode = "always"
)

// Auto-mode thresholds. Tuned for the issue #86 reporter's 7k-pod cluster:
// promotion fires at 1000 items (well above the noise floor of typical
// namespaced views), demotion happens once cached size has been below 500
// for autoDemoteAfterN consecutive calls. The hysteresis between
// autoPromoteAt and autoDemoteBelow stops the cache flapping when a list
// hovers around the threshold.
const (
	autoPromoteAt    = 1000
	autoDemoteBelow  = 500
	autoDemoteAfterN = 3
)

// gvrAutoState tracks the auto-mode position for one (context, GVR) so a
// resource type can move between direct-list and informer-backed without
// losing its history of recent list sizes.
type gvrAutoState struct {
	mu               sync.Mutex
	promoted         bool // true => current path is the informer cache
	consecutiveSmall int  // count of cached lists below autoDemoteBelow
}

// itemMemoEntry caches one converted model.Item alongside the
// resourceVersion of the unstructured object it was built from. Per-item
// keying (rather than per-list) is what makes the cache useful on a
// busy cluster: pod-A churn doesn't invalidate pod-B's cached row, so
// "the list as a whole" almost always sees a high hit rate even when
// the informer's LastSyncResourceVersion changes between every call.
//
// The cached model.Item is shared by-value with every consumer. Slice
// fields (Columns, Conditions, GroupedRefs) point to the same backing
// arrays across reuses — the read paths in this codebase either
// reassign whole slices (carryOverMetricsColumns) or clone first
// (cloneEventItem), so this aliasing is safe. New mutators must follow
// the same convention or call DeepCopy themselves.
type itemMemoEntry struct {
	rv   string
	item model.Item
}

// informerEntry tracks a single (context, GVR) informer lifecycle.
type informerEntry struct {
	informer cache.SharedIndexInformer
	stopCh   chan struct{}
	synced   chan struct{} // closed when the informer's first LIST has completed

	// memoMu guards memo. Keys are "<namespace>/<name>" (or "/<name>"
	// for cluster-scoped resources). Entries grow over the cache's
	// lifetime; entries for deleted pods linger until Stop. At typical
	// scales (a few thousand items, hours-long sessions) this is a
	// negligible memory footprint relative to the indexer itself.
	memoMu sync.Mutex
	memo   map[string]itemMemoEntry
}

// informerCache holds per-context dynamic informer factories, lazily
// instantiating one informer per resource type on first use. The cache is
// goroutine-safe: tea.Cmd workers may call into it concurrently from different
// contexts and resource types.
//
// Lifecycle: factories and informers persist for the lifetime of the parent
// Client. Stop() closes every watch and blocks until all informer goroutines
// have exited; it is called from Client.Shutdown().
type informerCache struct {
	clientFactory func(contextName string) (dynamic.Interface, error)

	mu      sync.Mutex
	clients map[string]dynamic.Interface
	entries map[string]map[schema.GroupVersionResource]*informerEntry
	auto    map[string]map[schema.GroupVersionResource]*gvrAutoState

	// promoteAt / demoteBelow / demoteAfterN are package-default constants
	// at runtime, but stored on the struct so unit tests can dial them down
	// without needing to fabricate 1000-item fixtures. Test helpers set
	// these directly on a cache instance.
	promoteAt    int
	demoteBelow  int
	demoteAfterN int

	// wg counts the inf.Run + cache-sync goroutines launched in
	// getOrStart. Stop blocks on it so the docstring's "blocks until all
	// informer goroutines have exited" promise is real, not aspirational.
	wg sync.WaitGroup

	stopped bool
}

// newInformerCache builds an informerCache that resolves dynamic clients via
// the supplied factory. Decoupling client construction from the cache lets
// tests inject a fake dynamic client without going through kubeconfig.
func newInformerCache(clientFactory func(string) (dynamic.Interface, error)) *informerCache {
	return &informerCache{
		clientFactory: clientFactory,
		clients:       make(map[string]dynamic.Interface),
		entries:       make(map[string]map[schema.GroupVersionResource]*informerEntry),
		auto:          make(map[string]map[schema.GroupVersionResource]*gvrAutoState),
		promoteAt:     autoPromoteAt,
		demoteBelow:   autoDemoteBelow,
		demoteAfterN:  autoDemoteAfterN,
	}
}

// getAutoState returns the auto-mode bookkeeping for a (context, GVR),
// allocating it on first access. Safe to call from any goroutine.
func (ic *informerCache) getAutoState(contextName string, gvr schema.GroupVersionResource) *gvrAutoState {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	perCtx := ic.auto[contextName]
	if perCtx == nil {
		perCtx = make(map[schema.GroupVersionResource]*gvrAutoState)
		ic.auto[contextName] = perCtx
	}
	state, ok := perCtx[gvr]
	if !ok {
		state = &gvrAutoState{}
		perCtx[gvr] = state
	}
	return state
}

// observeDirectListSize is called from GetResources after a direct (non-cache)
// LIST against the apiserver. When the result crosses promoteAt items, the
// (context, GVR) is flipped to informer-backed for subsequent calls — that's
// what makes namespace switching on a 7k-pod cluster feel instant from the
// second list onward.
func (ic *informerCache) observeDirectListSize(contextName string, gvr schema.GroupVersionResource, n int) {
	if n < ic.promoteAt {
		return
	}
	state := ic.getAutoState(contextName, gvr)
	state.mu.Lock()
	state.promoted = true
	state.consecutiveSmall = 0
	state.mu.Unlock()
}

// observeCachedListSize is called from GetResources after a cache-served list.
// It returns true when the (context, GVR) just transitioned back to the
// direct-list path (auto-demote): the underlying watch is torn down so we
// stop paying for a connection that no longer pays for itself. Hysteresis
// between promoteAt and demoteBelow plus a consecutive-call counter prevents
// flapping when a list size hovers near the threshold.
func (ic *informerCache) observeCachedListSize(contextName string, gvr schema.GroupVersionResource, n int) bool {
	state := ic.getAutoState(contextName, gvr)
	state.mu.Lock()
	if n < ic.demoteBelow {
		state.consecutiveSmall++
	} else {
		state.consecutiveSmall = 0
	}
	shouldDemote := state.promoted && state.consecutiveSmall >= ic.demoteAfterN
	if shouldDemote {
		state.promoted = false
		state.consecutiveSmall = 0
	}
	state.mu.Unlock()

	if shouldDemote {
		ic.stopOne(contextName, gvr)
	}
	return shouldDemote
}

// isPromoted reports whether the auto-mode router should send the next list
// for (contextName, gvr) through the informer cache. False means take the
// direct path; observeDirectListSize will decide whether to flip later.
func (ic *informerCache) isPromoted(contextName string, gvr schema.GroupVersionResource) bool {
	state := ic.getAutoState(contextName, gvr)
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.promoted
}

// stopOne tears down the watch for a single (contextName, gvr). Used by
// auto-demote: a resource type that shrunk no longer justifies a persistent
// connection. Safe to call when no informer was ever started — that's the
// no-op fast path.
//
// The ic.stopped early-out is what prevents the double-close race with
// Stop(): once Stop has flipped the flag, every later stopOne is a no-op
// and Stop owns the channel close. The check sits inside the same lock
// region Stop uses to snapshot entries, so the two cannot interleave.
func (ic *informerCache) stopOne(contextName string, gvr schema.GroupVersionResource) {
	ic.mu.Lock()
	if ic.stopped {
		ic.mu.Unlock()
		return
	}
	perCtx, ok := ic.entries[contextName]
	if !ok {
		ic.mu.Unlock()
		return
	}
	entry, ok := perCtx[gvr]
	if !ok {
		ic.mu.Unlock()
		return
	}
	delete(perCtx, gvr)
	if len(perCtx) == 0 {
		delete(ic.entries, contextName)
	}
	ic.mu.Unlock()

	close(entry.stopCh)
}

// listItems walks the cached objects for (context, GVR) filtered by
// namespace and returns them as []model.Item. Each object is keyed in
// the per-item memo by namespace/name plus its own
// metadata.resourceVersion; items whose RV matches the cached entry
// are reused as-is, only items whose RV differs (or whose key was
// never seen) run through build.
//
// On a busy cluster where most pods are byte-identical between calls
// but a few churn, this delivers a high cache hit rate where per-list
// memoization gets none — the informer's LastSyncResourceVersion
// changes whenever any object changes, but most objects' own RV stays
// stable across calls.
//
// build is the per-item conversion callback (in production:
// c.buildResourceItem). Tests can swap it for a deterministic stub.
// The returned builds count is the number of times build was actually
// invoked, so callers can log hit ratios.
//
// Skipping the DeepCopy is correct here because the indexer replaces
// cache entries by-pointer when watch events arrive: an event landing
// during the loop swaps the indexer's pointer for a fresh
// *unstructured, leaving any captured pointer's Object map untouched.
// That pointer-stability invariant is what makes this faster than the
// previous DeepCopy-every-item path.
func (ic *informerCache) listItems(ctx context.Context, contextName string, gvr schema.GroupVersionResource, namespace string, build func(*unstructured.Unstructured) model.Item) ([]model.Item, int, error) {
	entry, err := ic.getOrStart(contextName, gvr)
	if err != nil {
		return nil, 0, err
	}
	if err := waitForSync(ctx, entry); err != nil {
		return nil, 0, err
	}

	indexer := entry.informer.GetIndexer()
	var raw []*unstructured.Unstructured
	collect := func(obj any) {
		if u, ok := obj.(*unstructured.Unstructured); ok {
			raw = append(raw, u)
		}
	}
	if namespace == "" {
		err = cache.ListAll(indexer, labels.Everything(), collect)
	} else {
		err = cache.ListAllByNamespace(indexer, namespace, labels.Everything(), collect)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("listing cached %s: %w", gvr.Resource, err)
	}

	items, builds := entry.applyMemo(namespace, raw, build)
	return items, builds, nil
}

// applyMemo walks objs and returns one model.Item per object, reusing the
// cached row when (key, RV) matches and invoking build only when it
// doesn't. New rows are added to the memo so subsequent calls hit.
//
// listNamespace is the scope of the current list call: "" for all
// namespaces, or a specific namespace name. After the cache lookup we
// prune memo entries that fall within this scope but were not in the
// supplied objs slice — i.e. resources that have been deleted or moved
// out of namespace since the previous call. This keeps memo growth
// bounded by the live cluster size rather than by all-time churn,
// without losing entries from namespaces the current call doesn't
// know about (a per-ns list is not authoritative outside its own ns).
func (e *informerEntry) applyMemo(listNamespace string, objs []*unstructured.Unstructured, build func(*unstructured.Unstructured) model.Item) ([]model.Item, int) {
	e.memoMu.Lock()
	defer e.memoMu.Unlock()
	if e.memo == nil {
		e.memo = make(map[string]itemMemoEntry)
	}

	out := make([]model.Item, 0, len(objs))
	seen := make(map[string]struct{}, len(objs))
	builds := 0
	for _, obj := range objs {
		key := memoKey(obj)
		seen[key] = struct{}{}
		rv := obj.GetResourceVersion()
		if cached, ok := e.memo[key]; ok && cached.rv == rv {
			out = append(out, cached.item)
			continue
		}
		item := build(obj)
		e.memo[key] = itemMemoEntry{rv: rv, item: item}
		out = append(out, item)
		builds++
	}

	// Prune memo entries within the current list's scope that did not
	// appear in objs. An all-namespaces list ("") is authoritative for
	// every key; a namespaced list is only authoritative for keys with
	// that namespace prefix, so we leave other namespaces' entries
	// untouched. Without this, deleted pods accumulate in the memo for
	// the lifetime of the informer.
	scopePrefix := ""
	if listNamespace != "" {
		scopePrefix = listNamespace + "/"
	}
	for key := range e.memo {
		if _, kept := seen[key]; kept {
			continue
		}
		if scopePrefix != "" && !strings.HasPrefix(key, scopePrefix) {
			continue
		}
		delete(e.memo, key)
	}
	return out, builds
}

// memoKey produces a stable identifier for an unstructured resource.
// The leading slash on cluster-scoped objects (empty namespace) keeps
// the format unambiguous: "/foo" cannot collide with "ns/foo" because
// any namespaced resource has a non-empty namespace prefix.
func memoKey(obj *unstructured.Unstructured) string {
	return obj.GetNamespace() + "/" + obj.GetName()
}

// getOrStart returns the informer entry for (contextName, gvr), creating and
// starting it if necessary. It does not block on cache sync — that is the
// caller's responsibility via waitForSync, so the lock is released before any
// network IO begins.
//
// We bypass dynamicinformer.DynamicSharedInformerFactory because that factory
// caches informers per-GVR forever and refuses to restart one whose stop
// channel has been closed — fatal for auto-mode, where a demote followed by
// a re-promote needs a fresh informer. NewFilteredDynamicInformer hands us a
// standalone informer per (context, GVR) lifecycle that we own end-to-end.
func (ic *informerCache) getOrStart(contextName string, gvr schema.GroupVersionResource) (*informerEntry, error) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	if ic.stopped {
		return nil, fmt.Errorf("informer cache is shutting down")
	}

	if perCtx, ok := ic.entries[contextName]; ok {
		if entry, ok := perCtx[gvr]; ok {
			return entry, nil
		}
	}

	dc, ok := ic.clients[contextName]
	if !ok {
		built, err := ic.clientFactory(contextName)
		if err != nil {
			return nil, fmt.Errorf("dynamic client for context %q: %w", contextName, err)
		}
		dc = built
		ic.clients[contextName] = dc
	}

	// All namespaces — namespace filtering happens client-side from the
	// cache. That is the entire point of issue #86: one watch streams every
	// pod, and namespace selector switching becomes an in-memory walk.
	generic := dynamicinformer.NewFilteredDynamicInformer(
		dc, gvr, metav1.NamespaceAll, informerResyncPeriod,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		nil,
	)
	informer := generic.Informer()
	entry := &informerEntry{
		informer: informer,
		stopCh:   make(chan struct{}),
		synced:   make(chan struct{}),
	}

	// Two goroutines per (context, GVR): one runs the informer, one watches
	// for the initial sync to complete. Both are tracked on ic.wg so Stop
	// can join them — that is what makes the "blocks until all informer
	// goroutines have exited" docstring true. Add() before the goroutines
	// start so a concurrent Stop+Wait can never see a zero counter that
	// races the goroutine launch.
	ic.wg.Add(2)
	go func(inf cache.SharedIndexInformer, stopCh chan struct{}) {
		defer ic.wg.Done()
		inf.Run(stopCh)
	}(informer, entry.stopCh)
	go func(inf cache.SharedIndexInformer, stopCh, synced chan struct{}) {
		defer ic.wg.Done()
		// HasSynced flips true after the initial LIST completes; we close
		// `synced` so the first list() call can return promptly without
		// burning CPU in cache.WaitForCacheSync's polling loop. When
		// stopCh closes before sync (Stop or auto-demote during warmup),
		// WaitForCacheSync returns false and we leave `synced` open — any
		// list() caller still inside waitForSync will pick up the stopCh
		// signal instead.
		if cache.WaitForCacheSync(stopCh, inf.HasSynced) {
			close(synced)
		}
	}(informer, entry.stopCh, entry.synced)

	if ic.entries[contextName] == nil {
		ic.entries[contextName] = make(map[schema.GroupVersionResource]*informerEntry)
	}
	ic.entries[contextName][gvr] = entry
	return entry, nil
}

// waitForSync blocks until the informer's initial LIST has completed, the
// informer is stopped (Stop or auto-demote), the caller's context is
// cancelled, or initialSyncTimeout elapses. The timeout is a safety valve
// for slow API servers: rather than freezing the UI on the first namespace
// switch, we surface an error and the caller falls back to a direct list,
// by which time the watch has usually caught up in the background.
//
// The stopCh case keeps Shutdown responsive — without it a list() blocked
// here would have to ride out the full timeout before returning, which
// would also stall ic.wg.Wait inside Stop.
func waitForSync(ctx context.Context, entry *informerEntry) error {
	select {
	case <-entry.synced:
		return nil
	default:
	}

	timer := time.NewTimer(initialSyncTimeout)
	defer timer.Stop()

	select {
	case <-entry.synced:
		return nil
	case <-entry.stopCh:
		return fmt.Errorf("informer cache: stopped before initial sync")
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("informer cache: initial sync timed out after %s", initialSyncTimeout)
	}
}

// Stop closes every watch and blocks until all informer goroutines have
// exited. Idempotent — safe to call from a defer in main.go even if the
// cache was never used; concurrent Stop calls all wait on the same
// WaitGroup. After Stop, getOrStart returns an error rather than silently
// spinning up a new informer that would leak.
func (ic *informerCache) Stop() {
	ic.mu.Lock()
	if ic.stopped {
		ic.mu.Unlock()
		// A second caller still wants the same guarantee the first one
		// got: by the time Stop returns, no informer goroutine is
		// running. Wait on the same counter the first caller is draining.
		ic.wg.Wait()
		return
	}
	ic.stopped = true
	allEntries := make([]*informerEntry, 0)
	for _, perCtx := range ic.entries {
		for _, entry := range perCtx {
			allEntries = append(allEntries, entry)
		}
	}
	ic.mu.Unlock()

	for _, entry := range allEntries {
		close(entry.stopCh)
	}
	ic.wg.Wait()
}
