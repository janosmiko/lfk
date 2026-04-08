package security

import (
	"context"
	"sync"

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
// concurrently, and exposes an aggregate result. Caching is added in Task A3.
type Manager struct {
	mu      sync.RWMutex
	sources []SecuritySource
}

// NewManager returns an empty Manager. Sources are registered via Register.
func NewManager() *Manager {
	return &Manager{}
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
// IsAvailable(ctx, kubeCtx) == true.
func (m *Manager) AnyAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	for _, s := range m.Sources() {
		ok, err := s.IsAvailable(ctx, kubeCtx)
		if err != nil {
			continue // treat errors as "not available"
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// FetchAll runs Fetch concurrently across all available sources. Per-source
// errors do not cancel other sources; they are collected in result.Errors.
func (m *Manager) FetchAll(ctx context.Context, kubeCtx, namespace string) (FetchResult, error) {
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
	return res, nil
}
