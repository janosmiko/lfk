package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerRegisterAndFetchAll(t *testing.T) {
	m := NewManager()
	s1 := &FakeSource{
		NameStr: "s1", Available: true, CategoriesVal: []Category{CategoryVuln},
		Findings: []Finding{{ID: "1", Source: "s1", Severity: SeverityHigh}},
	}
	s2 := &FakeSource{
		NameStr: "s2", Available: true, CategoriesVal: []Category{CategoryMisconfig},
		Findings: []Finding{{ID: "2", Source: "s2", Severity: SeverityLow}},
	}
	m.Register(s1)
	m.Register(s2)

	res, err := m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err)
	assert.Len(t, res.Findings, 2)
	assert.Empty(t, res.Errors)
	assert.Equal(t, int32(1), s1.FetchCalls.Load())
	assert.Equal(t, int32(1), s2.FetchCalls.Load())
}

func TestManagerFetchAllParallel(t *testing.T) {
	m := NewManager()
	s1 := &FakeSource{NameStr: "s1", Available: true, FetchDelay: 80 * time.Millisecond}
	s2 := &FakeSource{NameStr: "s2", Available: true, FetchDelay: 80 * time.Millisecond}
	m.Register(s1)
	m.Register(s2)

	start := time.Now()
	_, err := m.FetchAll(context.Background(), "ctx", "")
	elapsed := time.Since(start)

	require.NoError(t, err)
	// If serial, elapsed would be >= 160ms. Parallel should be ~80ms + overhead.
	assert.Less(t, elapsed, 150*time.Millisecond, "sources should fetch in parallel")
}

func TestManagerFetchAllPartialFailure(t *testing.T) {
	m := NewManager()
	good := &FakeSource{
		NameStr: "good", Available: true,
		Findings: []Finding{{ID: "ok", Source: "good"}},
	}
	bad := &FakeSource{NameStr: "bad", Available: true, FetchErr: errors.New("boom")}
	m.Register(good)
	m.Register(bad)

	res, err := m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err, "partial failures must not return error")
	assert.Len(t, res.Findings, 1)
	assert.Contains(t, res.Errors, "bad")
	assert.EqualError(t, res.Errors["bad"], "boom")
}

func TestManagerSkipsUnavailableSources(t *testing.T) {
	m := NewManager()
	avail := &FakeSource{
		NameStr: "avail", Available: true,
		Findings: []Finding{{ID: "ok"}},
	}
	gone := &FakeSource{
		NameStr: "gone", Available: false,
		Findings: []Finding{{ID: "should-not-appear"}},
	}
	m.Register(avail)
	m.Register(gone)

	res, err := m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err)
	assert.Len(t, res.Findings, 1)
	assert.Equal(t, "ok", res.Findings[0].ID)
	assert.Equal(t, int32(0), gone.FetchCalls.Load(),
		"unavailable sources must not be fetched")
}

func TestManagerAnyAvailable(t *testing.T) {
	m := NewManager()
	m.Register(&FakeSource{NameStr: "a", Available: false})
	m.Register(&FakeSource{NameStr: "b", Available: true})
	ok, err := m.AnyAvailable(context.Background(), "ctx")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestManagerCancellation(t *testing.T) {
	m := NewManager()
	m.Register(&FakeSource{NameStr: "slow", Available: true, FetchDelay: 500 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before call

	start := time.Now()
	_, _ = m.FetchAll(ctx, "ctx", "")
	elapsed := time.Since(start)

	// Must return well before the source's 500ms delay — cancellation path.
	assert.Less(t, elapsed, 100*time.Millisecond,
		"cancellation must return quickly, not wait for FetchDelay")
}

func TestManagerAnyAvailableSkipsSourcesWithErrors(t *testing.T) {
	m := NewManager()
	m.Register(&FakeSource{NameStr: "broken", Available: false, AvailableErr: errors.New("probe failed")})
	m.Register(&FakeSource{NameStr: "healthy", Available: true})

	ok, err := m.AnyAvailable(context.Background(), "ctx")
	require.NoError(t, err)
	assert.True(t, ok, "AnyAvailable must skip erroring sources and return true when another is healthy")
}

func TestManagerCachedFetch(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(200 * time.Millisecond)
	s := &FakeSource{
		NameStr: "s", Available: true,
		Findings: []Finding{{ID: "x"}},
	}
	m.Register(s)

	_, err := m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err)
	_, err = m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err)
	assert.Equal(t, int32(1), s.FetchCalls.Load(),
		"second call within TTL should hit cache")
}

func TestManagerForceRefresh(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(1 * time.Hour)
	s := &FakeSource{
		NameStr: "s", Available: true,
		Findings: []Finding{{ID: "x"}},
	}
	m.Register(s)

	_, _ = m.FetchAll(context.Background(), "ctx", "")
	_, _ = m.Refresh(context.Background(), "ctx", "")
	assert.Equal(t, int32(2), s.FetchCalls.Load(),
		"Refresh must bypass the cache")
}

func TestManagerInvalidateOnContextChange(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(1 * time.Hour)
	s := &FakeSource{
		NameStr: "s", Available: true,
		Findings: []Finding{{ID: "x"}},
	}
	m.Register(s)

	_, _ = m.FetchAll(context.Background(), "ctxA", "")
	_, _ = m.FetchAll(context.Background(), "ctxB", "")
	assert.Equal(t, int32(2), s.FetchCalls.Load(),
		"different kubeCtx should bypass cache")
}

func TestManagerAvailabilityCached(t *testing.T) {
	m := NewManager()
	m.SetAvailabilityTTL(200 * time.Millisecond)
	s := &FakeSource{NameStr: "s", Available: true}
	m.Register(s)

	_, _ = m.AnyAvailable(context.Background(), "ctx")
	_, _ = m.AnyAvailable(context.Background(), "ctx")
	assert.Equal(t, int32(1), s.AvailCalls.Load(),
		"availability should be cached within TTL")
}

func TestFindingIndexCountsAndLookup(t *testing.T) {
	m := NewManager()
	s := &FakeSource{NameStr: "s", Available: true, Findings: []Finding{
		{
			ID: "1", Title: "CVE-2024-0001", Severity: SeverityCritical,
			Resource: ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"},
		},
		{
			ID: "2", Title: "CVE-2024-0002", Severity: SeverityHigh,
			Resource: ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"},
		},
		{
			ID: "3", Title: "CVE-2024-0003", Severity: SeverityLow,
			Resource: ResourceRef{Namespace: "prod", Kind: "Pod", Name: "db-0"},
		},
	}}
	m.Register(s)

	_, err := m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err)

	idx := m.Index()
	api := idx.For(ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"})
	assert.Equal(t, 1, api.Critical)
	assert.Equal(t, 1, api.High)
	assert.Equal(t, 0, api.Medium)
	assert.Equal(t, 0, api.Low)
	assert.Equal(t, 2, api.Total())
	assert.Equal(t, SeverityCritical, api.Highest())

	empty := idx.For(ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "nope"})
	assert.Equal(t, 0, empty.Total())
	assert.Equal(t, SeverityUnknown, empty.Highest())
}

func TestFindingIndexCountBySource(t *testing.T) {
	idx := BuildFindingIndex([]Finding{
		{
			Source:   "trivy-operator",
			Severity: SeverityCritical,
			Resource: ResourceRef{Namespace: "p", Kind: "Deployment", Name: "a"},
		},
		{
			Source:   "trivy-operator",
			Severity: SeverityHigh,
			Resource: ResourceRef{Namespace: "p", Kind: "Deployment", Name: "b"},
		},
		{
			Source:   "heuristic",
			Severity: SeverityMedium,
			Resource: ResourceRef{Namespace: "p", Kind: "Pod", Name: "c"},
		},
	})

	assert.Equal(t, 2, idx.CountBySource("trivy-operator"))
	assert.Equal(t, 1, idx.CountBySource("heuristic"))
	assert.Equal(t, 0, idx.CountBySource("missing"))
}

func TestFindingIndexCountBySourceNil(t *testing.T) {
	var idx *FindingIndex
	assert.Equal(t, 0, idx.CountBySource("any"))
}

func TestManagerInvalidateClearsCache(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(1 * time.Hour)
	s := &FakeSource{
		NameStr: "s", Available: true,
		Findings: []Finding{{ID: "x"}},
	}
	m.Register(s)

	_, err := m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err)
	assert.Equal(t, int32(1), s.FetchCalls.Load())

	_, _ = m.FetchAll(context.Background(), "ctx", "")
	assert.Equal(t, int32(1), s.FetchCalls.Load())

	m.Invalidate()
	_, _ = m.FetchAll(context.Background(), "ctx", "")
	assert.Equal(t, int32(2), s.FetchCalls.Load())
}
