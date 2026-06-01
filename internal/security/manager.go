package security

import (
	"context"
	"maps"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// FetchResult is the aggregated output of Manager.FetchAll.
type FetchResult struct {
	Findings []Finding
	Errors   map[string]error // source name -> error (nil-safe)
	Sources  []SourceStatus
}

// SourceStatus captures the last known state of a registered source.
type SourceStatus struct {
	Name      string
	Available bool
	Count     int
	LastError error
}

// Manager aggregates SecuritySource instances, runs IsAvailable and Fetch
// concurrently, and exposes an aggregate result. It caches FetchAll results by
// (kubeCtx, namespace) and AnyAvailable results by kubeCtx.
type Manager struct {
	mu      sync.RWMutex
	sources []SecuritySource

	refreshTTL          time.Duration
	errorTTL            time.Duration // shorter TTL applied when no source succeeded
	availabilityTTL     time.Duration
	maxFetchConcurrency int           // upper bound on simultaneous source Fetches; 0 = unbounded
	scanTimeout         time.Duration // hard ceiling on a single coalesced FetchAll scan

	cacheKey      string // lastCtx + "|" + lastNamespace
	cachedResult  FetchResult
	cachedAt      time.Time
	cachedSuccess bool // anySourceSucceeded() at the time the result was cached
	cachedIndex   *FindingIndex

	// fetchGroup coalesces concurrent FetchAll calls for the same
	// (kubeCtx, namespace). On navigation the middle list, the SEC-badge
	// index, and the right-pane preview each call FetchAll near-
	// simultaneously; without this they ran the full multi-source scan
	// independently — tripling API load and leaving the right pane
	// spinning on its own slow fetch after the list had resolved.
	fetchGroup singleflight.Group

	availCache map[string]availEntry // key = kubeCtx

	// perSourceAvail is the externally-supplied per-source availability
	// hint, keyed by kubeCtx → sourceName → available. When present,
	// FetchAll skips its own IsAvailable probe for that source and uses
	// the hint directly. The probe inside FetchAll is the dominant
	// non-Fetch API cost on slow / throttled clusters; the app-level
	// loadSecurityAvailability already performed the same probe with a
	// 3s budget per source, so re-doing it inside FetchAll doubles the
	// API load on every navigation. SetAvailability populates this map.
	perSourceAvail map[string]map[string]bool

	ignoredNamespaces map[string]bool // global namespace filter applied to all sources
}

type availEntry struct {
	available bool
	at        time.Time
}

// NewManager returns a Manager with sensible cache defaults (5min fetch, 60s
// availability). The 5-minute fetch TTL keeps findings stable across
// navigation cycles (drill into group → jump to resource → navigate
// back). Users press R for explicit refresh.
//
// errorTTL defaults to 30s — the shorter window applied when every
// source errored. Without it, slow / throttled clusters fall into a
// hammering loop: each navigation tick re-fires the same failing list
// requests, the API server applies more aggressive rate limits, and the
// right pane stays in "Scanning..." indefinitely. 30s is short enough
// that genuine recovery feels responsive but long enough to break the
// throttle spiral.
func NewManager() *Manager {
	return &Manager{
		refreshTTL:          5 * time.Minute,
		errorTTL:            30 * time.Second,
		availabilityTTL:     60 * time.Second,
		maxFetchConcurrency: 2,
		scanTimeout:         60 * time.Second,
		availCache:          make(map[string]availEntry),
	}
}

// SetRefreshTTL overrides the FetchAll cache TTL. Zero disables caching.
func (m *Manager) SetRefreshTTL(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshTTL = d
}

// SetErrorTTL overrides the negative-cache TTL applied when every
// source erred (no findings produced). Zero disables negative caching
// entirely — only do that in tests where you want every call to refire.
func (m *Manager) SetErrorTTL(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorTTL = d
}

// SetMaxFetchConcurrency caps the number of source Fetch calls that
// run simultaneously inside FetchAll. The default (2) keeps peak
// control-plane load roughly an order of magnitude below the original
// fan-out behaviour on busy clusters with many CRD-backed sources
// installed (Trivy + Kyverno + Gatekeeper + Kubescape can each list
// hundreds of objects per call). Pass 0 to disable the cap (run every
// source concurrently); useful in unit tests where the goroutines do
// not actually hit a real API server.
func (m *Manager) SetMaxFetchConcurrency(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n < 0 {
		n = 0
	}
	m.maxFetchConcurrency = n
}

// ScanTimeout returns the hard ceiling on a single coalesced FetchAll scan.
// Callers align their own context deadline to this so a slow-but-succeeding
// scan delivers its result to the triggering load instead of orphaning at a
// shorter caller timeout while the detached scan runs on.
func (m *Manager) ScanTimeout() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scanTimeout
}

// SetScanTimeout overrides the per-scan hard ceiling. Clamped to a positive
// value; non-positive is ignored so a misconfig can't disable the timeout.
func (m *Manager) SetScanTimeout(d time.Duration) {
	if d <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scanTimeout = d
}

// SetAvailabilityTTL overrides the AnyAvailable cache TTL.
func (m *Manager) SetAvailabilityTTL(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.availabilityTTL = d
}

// Refresh is FetchAll that always bypasses the cache. Per-source
// availability hints are kept — the caller (typically the app's
// shift+r refresh flow) re-probes availability separately and will
// re-publish via SetAvailability before/after Refresh.
func (m *Manager) Refresh(ctx context.Context, kubeCtx, namespace string) (FetchResult, error) {
	m.mu.Lock()
	m.cacheKey = ""
	m.availCache = make(map[string]availEntry)
	m.mu.Unlock()
	return m.FetchAll(ctx, kubeCtx, namespace)
}

// Invalidate clears the fetch cache, the availability cache, and the
// per-resource finding index without performing a new fetch. The next
// call to FetchAll or AnyAvailable will go back to the source(s). Used
// when callers know the underlying cluster state has changed (e.g., the
// user pressed `r` to refresh).
//
// Clearing cachedIndex matters because Index() is consulted by the SEC
// badge renderer on every explorer paint — without clearing it, badges
// keep showing pre-invalidate per-resource counts until the next
// FetchAll lands.
func (m *Manager) Invalidate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheKey = ""
	m.availCache = make(map[string]availEntry)
	m.cachedIndex = nil
}

// Register appends a source. Not safe to call concurrently with FetchAll.
func (m *Manager) Register(s SecuritySource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources = append(m.sources, s)
}

// SetAvailability records the per-source availability map for kubeCtx
// so subsequent FetchAll calls can skip their IsAvailable probe and go
// straight to Fetch (or skip the source altogether). Pass the result of
// the app-level availability probe (loadSecurityAvailability) here.
//
// Sources missing from byName are NOT treated as unavailable — they
// fall back to FetchAll's regular IsAvailable probe. This lets callers
// supply a partial map (e.g., only the sources whose probe completed)
// without forcing the rest to be hidden.
func (m *Manager) SetAvailability(kubeCtx string, byName map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.perSourceAvail == nil {
		m.perSourceAvail = make(map[string]map[string]bool)
	}
	// Copy so the caller can mutate their map without racing us.
	hint := make(map[string]bool, len(byName))
	maps.Copy(hint, byName)
	m.perSourceAvail[kubeCtx] = hint
}

// availabilityHint reports whether the caller has previously declared a
// known availability for (kubeCtx, sourceName) via SetAvailability.
// known=false means "no hint, fall back to s.IsAvailable".
func (m *Manager) availabilityHint(kubeCtx, sourceName string) (known, available bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	byName, ok := m.perSourceAvail[kubeCtx]
	if !ok {
		return false, false
	}
	v, ok := byName[sourceName]
	if !ok {
		return false, false
	}
	return true, v
}

// SetIgnoredNamespaces configures namespaces that are excluded from ALL
// sources. Findings with a resource in an ignored namespace are dropped
// after FetchAll collects them. This is in addition to any per-source
// ignored_namespaces config.
func (m *Manager) SetIgnoredNamespaces(namespaces []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ignoredNamespaces = make(map[string]bool, len(namespaces))
	for _, ns := range namespaces {
		m.ignoredNamespaces[ns] = true
	}
}

// Sources returns a snapshot of currently registered sources.
func (m *Manager) Sources() []SecuritySource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]SecuritySource, len(m.sources))
	copy(out, m.sources)
	return out
}

// AnyAvailable returns true if at least one registered source reports
// IsAvailable(ctx, kubeCtx) == true. Results are cached per kubeCtx.
func (m *Manager) AnyAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	m.mu.RLock()
	if entry, ok := m.availCache[kubeCtx]; ok && time.Since(entry.at) < m.availabilityTTL {
		m.mu.RUnlock()
		return entry.available, nil
	}
	m.mu.RUnlock()

	for _, s := range m.Sources() {
		ok, _ := s.IsAvailable(ctx, kubeCtx)
		if ok {
			m.mu.Lock()
			m.availCache[kubeCtx] = availEntry{available: true, at: time.Now()}
			m.mu.Unlock()
			return true, nil
		}
	}
	m.mu.Lock()
	m.availCache[kubeCtx] = availEntry{available: false, at: time.Now()}
	m.mu.Unlock()
	return false, nil
}

// FetchAll runs Fetch concurrently across all available sources. Per-source
// errors do not cancel other sources; they are collected in result.Errors.
// Results are cached by (kubeCtx, namespace) for refreshTTL on success
// and for errorTTL when every source erred (negative cache, breaks the
// throttling spiral on slow clusters).
func (m *Manager) FetchAll(ctx context.Context, kubeCtx, namespace string) (FetchResult, error) {
	cacheKey := kubeCtx + "|" + namespace

	if cached, ok := m.cachedFetch(cacheKey); ok {
		return cached, nil
	}

	// Coalesce overlapping identical scans (see fetchGroup). DoChan (not Do)
	// is deliberate: callers run inside the app's per-context scheduler
	// worker pool, which preempts and times out work via context
	// cancellation. A worker blocked on the uncancellable Do would ignore
	// its ctx and stay stuck for the whole scan, starving the pod/dashboard
	// loads that share the pool. With DoChan each caller selects on its own
	// ctx and returns promptly when cancelled.
	//
	// The scan itself runs on a detached, bounded context — not the caller's
	// — so it completes and populates the cache even if the caller that
	// started it navigates away. Tying the scan to one caller's ctx would
	// let a preempted caller abort the shared scan and hand every waiter a
	// premature empty result.
	// Capture scanTimeout under the lock before the closure: the DoChan
	// goroutine reads it on a separate goroutine, and SetScanTimeout writes it
	// under m.mu — reading m.scanTimeout directly inside the closure would be
	// an unsynchronized read racing that write.
	scanTimeout := m.ScanTimeout()
	ch := m.fetchGroup.DoChan(cacheKey, func() (any, error) {
		if cached, ok := m.cachedFetch(cacheKey); ok {
			return cached, nil
		}
		scanCtx, cancel := context.WithTimeout(context.Background(), scanTimeout)
		defer cancel()
		return m.fetchAllScan(scanCtx, kubeCtx, namespace, cacheKey), nil
	})
	select {
	case res := <-ch:
		if res.Err != nil {
			return FetchResult{}, res.Err
		}
		return res.Val.(FetchResult), nil
	case <-ctx.Done():
		// Caller cancelled / preempted / timed out. The detached scan keeps
		// running and caches its result for the next call; this caller
		// returns immediately so a scheduler worker is never stuck waiting.
		return FetchResult{}, ctx.Err()
	}
}

// cachedFetch returns the cached FetchResult for cacheKey when it is still
// within the applicable TTL (refreshTTL on success, errorTTL on a fully-
// errored result). The bool is false on a miss or an expired entry.
func (m *Manager) cachedFetch(cacheKey string) (FetchResult, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if cacheKey != m.cacheKey {
		return FetchResult{}, false
	}
	ttl := m.refreshTTL
	if !m.cachedSuccess {
		ttl = m.errorTTL
	}
	if ttl > 0 && time.Since(m.cachedAt) < ttl {
		return m.cachedResult, true
	}
	return FetchResult{}, false
}

// fetchAllScan runs Fetch across all available sources, aggregates the
// results, and writes the cache. It is invoked through fetchGroup so only
// one scan per cacheKey runs at a time.
func (m *Manager) fetchAllScan(ctx context.Context, kubeCtx, namespace, cacheKey string) FetchResult {
	sources := m.Sources()

	type sourceResult struct {
		name        string
		findings    []Finding
		err         error
		unavailable bool // source's IsAvailable returned false; skip status emission
	}
	results := make(chan sourceResult, len(sources))

	// Per-source errors flow into res.Errors via the results channel and
	// goroutines never return an error worth cancelling siblings on, so
	// errgroup's WithContext semantics added dead weight without any
	// benefit. A buffered semaphore caps simultaneous Fetch calls — the
	// default cap of 2 keeps a 6-source cluster from issuing six large
	// list responses in parallel and stressing etcd. The semaphore is
	// taken AFTER the IsAvailable hint check so unavailable sources cost
	// nothing concurrency-wise.
	maxConc := m.maxFetchConcurrency
	if maxConc <= 0 || maxConc > len(sources) {
		maxConc = len(sources)
	}
	sem := make(chan struct{}, maxConc)
	var wg sync.WaitGroup
	for _, s := range sources {
		wg.Go(func() {
			// Skip the IsAvailable probe when the caller supplied a
			// hint via SetAvailability (the app-level
			// loadSecurityAvailability already paid this cost with a
			// 3s-per-source budget). Without the hint, fall back to
			// the source's own probe — errors are intentionally
			// treated as "not available" (see SecuritySource docs).
			known, available := m.availabilityHint(kubeCtx, s.Name())
			if !known {
				ok, _ := s.IsAvailable(ctx, kubeCtx)
				available = ok
			}
			if !available {
				results <- sourceResult{name: s.Name(), unavailable: true}
				return
			}
			// Acquire a slot before performing the (potentially expensive)
			// list. If ctx is cancelled while waiting, exit promptly so
			// the FetchAll budget isn't burned on already-cancelled work.
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results <- sourceResult{name: s.Name(), err: ctx.Err()}
				return
			}
			defer func() { <-sem }()

			findings, ferr := s.Fetch(ctx, kubeCtx, namespace)
			results <- sourceResult{name: s.Name(), findings: findings, err: ferr}
		})
	}

	wg.Wait()
	close(results)

	res := FetchResult{
		Errors: map[string]error{},
	}
	for r := range results {
		// Unavailable sources (IsAvailable said no) emit a SourceStatus
		// with Available=false so the cache decision sees them as
		// "no successful fetch happened". Without this they'd fall into
		// the success branch below and be reported as Available:true
		// with 0 findings — making anySourceSucceeded return true on a
		// cluster with zero installed security tools.
		if r.unavailable {
			res.Sources = append(res.Sources, SourceStatus{Name: r.name})
			continue
		}
		if r.err != nil {
			res.Errors[r.name] = r.err
			res.Sources = append(res.Sources, SourceStatus{Name: r.name, LastError: r.err})
			continue
		}
		res.Findings = append(res.Findings, r.findings...)
		res.Sources = append(res.Sources, SourceStatus{
			Name: r.name, Available: true, Count: len(r.findings),
		})
	}

	// Apply global namespace filter. Snapshot the map under RLock so a
	// concurrent SetIgnoredNamespaces can't race with the read. Build the
	// filtered slice into a fresh backing array; an in-place `Findings[:0]`
	// trick aliases the backing memory that future cache hits return to
	// callers, which we'd then overwrite from the next FetchAll's append.
	m.mu.RLock()
	ignored := m.ignoredNamespaces
	m.mu.RUnlock()
	if len(ignored) > 0 {
		filtered := make([]Finding, 0, len(res.Findings))
		for _, f := range res.Findings {
			if !ignored[f.Resource.Namespace] {
				filtered = append(filtered, f)
			}
		}
		res.Findings = filtered
	}

	m.mu.Lock()
	// Cache the result regardless of success — a clean cluster with zero
	// findings is a valid full-TTL outcome (anySourceSucceeded=true,
	// just empty), and an all-errored cluster gets the shorter errorTTL
	// negative cache so the next navigation tick doesn't re-fire the
	// same failing requests and dig the throttle hole even deeper.
	// SetErrorTTL(0) disables negative caching for callers (tests) that
	// need the old "always retry on error" behavior.
	success := anySourceSucceeded(res.Sources)
	if success || m.errorTTL > 0 {
		m.cacheKey = cacheKey
		m.cachedResult = res
		m.cachedAt = time.Now()
		m.cachedSuccess = success
		m.cachedIndex = BuildFindingIndex(res.Findings)
	}
	m.mu.Unlock()
	return res
}

// anySourceSucceeded reports whether at least one entry in s carries
// Available=true (set by FetchAll on a non-erroring source). Used to
// decide whether the FetchAll result is cacheable.
func anySourceSucceeded(s []SourceStatus) bool {
	for _, st := range s {
		if st.Available {
			return true
		}
	}
	return false
}

// SeverityCounts holds severity breakdown for a single resource.
type SeverityCounts struct {
	Critical, High, Medium, Low int
}

// Total returns the sum of all severity buckets.
func (c SeverityCounts) Total() int {
	return c.Critical + c.High + c.Medium + c.Low
}

// Highest returns the highest severity present, or SeverityUnknown if none.
func (c SeverityCounts) Highest() Severity {
	switch {
	case c.Critical > 0:
		return SeverityCritical
	case c.High > 0:
		return SeverityHigh
	case c.Medium > 0:
		return SeverityMedium
	case c.Low > 0:
		return SeverityLow
	default:
		return SeverityUnknown
	}
}

// HighestCount returns the number of findings in the highest-severity bucket
// that has any, or 0 when empty. Pairs with Highest: the badge colors by
// Highest and labels with HighestCount so a red badge means "this many
// criticals", never the all-severity total.
func (c SeverityCounts) HighestCount() int {
	switch {
	case c.Critical > 0:
		return c.Critical
	case c.High > 0:
		return c.High
	case c.Medium > 0:
		return c.Medium
	case c.Low > 0:
		return c.Low
	default:
		return 0
	}
}

// FindingIndex aggregates findings by resource for O(1) per-row lookup.
type FindingIndex struct {
	counts   map[string]SeverityCounts
	bySource map[string]int
}

// For returns the aggregated counts for the given resource. Zero value when absent.
func (i *FindingIndex) For(ref ResourceRef) SeverityCounts {
	if i == nil {
		return SeverityCounts{}
	}
	return i.counts[ref.Key()]
}

// CountBySource returns the total finding count for the given source
// name. Returns 0 if the index is nil or the source isn't present.
func (i *FindingIndex) CountBySource(name string) int {
	if i == nil {
		return 0
	}
	return i.bySource[name]
}

// BuildFindingIndex constructs an index from a slice of findings.
// Per-resource severity counts deduplicate by (resource, Title) so the
// SEC badge shows unique checks (e.g., "privileged", "run_as_root")
// rather than raw per-container counts: a pod with 3 containers all
// running privileged contributes 1 to its severity bucket, not 3.
//
// bySource counts every emission without deduplication so callers like
// CountBySource see the true per-source contribution; deduplicating
// bySource by (resource, Title) silently undercounts when two sources
// (e.g., Trivy + policy-report) overlap on the same misconfiguration —
// the second source got skipped before its bySource increment.
func BuildFindingIndex(findings []Finding) *FindingIndex {
	idx := &FindingIndex{
		counts:   make(map[string]SeverityCounts),
		bySource: make(map[string]int),
	}
	// Pick the highest severity per (resource, Title) pair before counting.
	// Two-pass keeps the per-source totals accurate (every finding increments
	// bySource in the first pass) while ensuring a later duplicate that
	// reports a higher severity isn't lost to first-write-wins iteration
	// order, which would silently under-color the SEC badge.
	maxSev := make(map[string]Severity)
	maxKey := make(map[string]string) // dedup -> resource Key for the second pass
	for _, f := range findings {
		idx.bySource[f.Source]++
		dedup := f.Resource.Key() + "|" + f.Title
		if cur, ok := maxSev[dedup]; !ok || f.Severity > cur {
			maxSev[dedup] = f.Severity
			maxKey[dedup] = f.Resource.Key()
		}
	}
	for dedup, sev := range maxSev {
		key := maxKey[dedup]
		c := idx.counts[key]
		switch sev {
		case SeverityCritical:
			c.Critical++
		case SeverityHigh:
			c.High++
		case SeverityMedium:
			c.Medium++
		case SeverityLow:
			c.Low++
		}
		idx.counts[key] = c
	}
	return idx
}

// Index returns the FindingIndex for the most recent FetchAll result.
// Returns an empty index if FetchAll has not been called yet.
func (m *Manager) Index() *FindingIndex {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cachedIndex == nil {
		return &FindingIndex{
			counts:   map[string]SeverityCounts{},
			bySource: map[string]int{},
		}
	}
	return m.cachedIndex
}

// SetIndex overrides the cached FindingIndex. Used by callers that produce
// findings outside of FetchAll (e.g., async message paths in internal/app
// that receive a FetchResult via a tea.Msg and bypass the cache).
func (m *Manager) SetIndex(idx *FindingIndex) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cachedIndex = idx
}
