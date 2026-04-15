package security

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
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

	refreshTTL      time.Duration
	availabilityTTL time.Duration

	cacheKey     string // lastCtx + "|" + lastNamespace
	cachedResult FetchResult
	cachedAt     time.Time
	cachedIndex  *FindingIndex

	availCache map[string]availEntry // key = kubeCtx
}

type availEntry struct {
	available bool
	at        time.Time
}

// NewManager returns a Manager with sensible cache defaults (5min fetch, 60s availability).
// The 5-minute fetch TTL keeps findings stable across navigation cycles
// (drill into group → jump to resource → navigate back). Users press R
// for explicit refresh.
func NewManager() *Manager {
	return &Manager{
		refreshTTL:      5 * time.Minute,
		availabilityTTL: 60 * time.Second,
		availCache:      make(map[string]availEntry),
	}
}

// SetRefreshTTL overrides the FetchAll cache TTL. Zero disables caching.
func (m *Manager) SetRefreshTTL(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshTTL = d
}

// SetAvailabilityTTL overrides the AnyAvailable cache TTL.
func (m *Manager) SetAvailabilityTTL(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.availabilityTTL = d
}

// Refresh is FetchAll that always bypasses the cache.
func (m *Manager) Refresh(ctx context.Context, kubeCtx, namespace string) (FetchResult, error) {
	m.mu.Lock()
	m.cacheKey = ""
	m.availCache = make(map[string]availEntry)
	m.mu.Unlock()
	return m.FetchAll(ctx, kubeCtx, namespace)
}

// Invalidate clears the fetch cache and the availability cache without
// performing a new fetch. The next call to FetchAll or AnyAvailable will
// go back to the source(s). Used when callers know the underlying cluster
// state has changed (e.g., the user pressed `r` to refresh).
func (m *Manager) Invalidate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheKey = ""
	m.availCache = make(map[string]availEntry)
}

// Register appends a source. Not safe to call concurrently with FetchAll.
func (m *Manager) Register(s SecuritySource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources = append(m.sources, s)
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
// Results are cached by (kubeCtx, namespace) for refreshTTL.
func (m *Manager) FetchAll(ctx context.Context, kubeCtx, namespace string) (FetchResult, error) {
	cacheKey := kubeCtx + "|" + namespace

	m.mu.RLock()
	if cacheKey == m.cacheKey && m.refreshTTL > 0 && time.Since(m.cachedAt) < m.refreshTTL {
		cached := m.cachedResult
		m.mu.RUnlock()
		return cached, nil
	}
	m.mu.RUnlock()

	sources := m.Sources()

	type sourceResult struct {
		name     string
		findings []Finding
		err      error
	}
	results := make(chan sourceResult, len(sources))

	g, gctx := errgroup.WithContext(ctx)
	for _, s := range sources {
		g.Go(func() error {
			// Errors from IsAvailable are intentionally treated as "not
			// available" (see SecuritySource docs) and do not propagate.
			ok, _ := s.IsAvailable(gctx, kubeCtx)
			if !ok {
				results <- sourceResult{name: s.Name()}
				return nil
			}
			findings, ferr := s.Fetch(gctx, kubeCtx, namespace)
			results <- sourceResult{name: s.Name(), findings: findings, err: ferr}
			return nil
		})
	}

	_ = g.Wait() // errgroup is used only for scope; per-source errors are captured in results.
	close(results)

	res := FetchResult{
		Errors: map[string]error{},
	}
	for r := range results {
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

	m.mu.Lock()
	// Only cache non-empty results. Empty results from sources that
	// weren't ready yet (IsAvailable returned false) should not prevent
	// a subsequent fetch from trying again once the sources are up.
	if len(res.Findings) > 0 {
		m.cacheKey = cacheKey
		m.cachedResult = res
		m.cachedAt = time.Now()
		m.cachedIndex = BuildFindingIndex(res.Findings)
	}
	m.mu.Unlock()
	return res, nil
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
// Findings are deduplicated per resource by Title so the SEC badge
// shows unique checks (e.g., "privileged", "run_as_root") rather than
// raw per-container counts. A pod with 3 containers all running
// privileged counts as 1 finding, not 3.
func BuildFindingIndex(findings []Finding) *FindingIndex {
	idx := &FindingIndex{
		counts:   make(map[string]SeverityCounts),
		bySource: make(map[string]int),
	}
	// Track (resource + title) pairs to deduplicate per-container findings.
	seen := make(map[string]bool)
	for _, f := range findings {
		key := f.Resource.Key()
		dedup := key + "|" + f.Title
		if seen[dedup] {
			continue
		}
		seen[dedup] = true
		c := idx.counts[key]
		switch f.Severity {
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
		idx.bySource[f.Source]++
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
