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

	availCache map[string]availEntry // key = kubeCtx
}

type availEntry struct {
	available bool
	at        time.Time
}

// NewManager returns a Manager with sensible cache defaults (30s fetch, 60s availability).
func NewManager() *Manager {
	return &Manager{
		refreshTTL:      30 * time.Second,
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
	m.cacheKey = cacheKey
	m.cachedResult = res
	m.cachedAt = time.Now()
	m.mu.Unlock()
	return res, nil
}
