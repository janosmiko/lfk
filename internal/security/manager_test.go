package security

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManagerFetchAllCoalescesConcurrentCalls verifies that overlapping
// FetchAll calls for the same (context, namespace) share a single source
// scan instead of each running the full multi-source fetch. On navigation
// the middle list, the SEC-badge index, and the right-pane preview all call
// FetchAll near-simultaneously; without coalescing each ran the expensive
// scan independently, tripling API load and leaving the right pane spinning
// on its own slow fetch while the list had already resolved.
func TestManagerFetchAllCoalescesConcurrentCalls(t *testing.T) {
	m := NewManager()
	s := &FakeSource{
		NameStr: "s", Available: true,
		FetchDelay: 100 * time.Millisecond,
		Findings:   []Finding{{ID: "x"}},
	}
	m.Register(s)

	const n = 8
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			_, _ = m.FetchAll(t.Context(), "ctx", "")
		})
	}
	wg.Wait()

	assert.Equal(t, int32(1), s.FetchCalls.Load(),
		"concurrent identical FetchAll calls must coalesce into one source scan")
}

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

	res, err := m.FetchAll(t.Context(), "ctx", "")
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
	_, err := m.FetchAll(t.Context(), "ctx", "")
	elapsed := time.Since(start)

	require.NoError(t, err)
	// If serial, elapsed would be >= 160ms. Parallel should be ~80ms + overhead.
	assert.Less(t, elapsed, 150*time.Millisecond, "sources should fetch in parallel")
}

// TestManagerFetchAllWaiterRespectsContext guards against a coalesced caller
// blocking uncancellably on an in-flight scan. FetchAll runs inside the app's
// per-context scheduler worker pool, which preempts and times out tasks via
// context cancellation. A waiter that ignored its context would keep a worker
// stuck for the full duration of a slow source scan (Falco/Trivy log reads),
// starving the pod/dashboard loads that share the pool.
func TestManagerFetchAllWaiterRespectsContext(t *testing.T) {
	m := NewManager()
	m.Register(&FakeSource{
		NameStr: "slow", Available: true,
		FetchDelay: 800 * time.Millisecond,
		Findings:   []Finding{{ID: "x"}},
	})

	// Kick off a scan that will be in flight for ~800ms.
	go func() { _, _ = m.FetchAll(t.Context(), "ctx", "") }()
	time.Sleep(50 * time.Millisecond) // let the flight start

	// A second caller for the same key with a short deadline must return when
	// its own context expires, not block until the in-flight scan completes.
	cctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := m.FetchAll(cctx, "ctx", "")
	elapsed := time.Since(start)

	require.Error(t, err, "waiter must return its context error, not the coalesced result")
	assert.Less(t, elapsed, 400*time.Millisecond,
		"waiter must not block for the full in-flight scan")
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

	res, err := m.FetchAll(t.Context(), "ctx", "")
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

	res, err := m.FetchAll(t.Context(), "ctx", "")
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
	ok, err := m.AnyAvailable(t.Context(), "ctx")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestManagerCancellation(t *testing.T) {
	m := NewManager()
	m.Register(&FakeSource{NameStr: "slow", Available: true, FetchDelay: 500 * time.Millisecond})
	ctx, cancel := context.WithCancel(t.Context())
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

	ok, err := m.AnyAvailable(t.Context(), "ctx")
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

	_, err := m.FetchAll(t.Context(), "ctx", "")
	require.NoError(t, err)
	_, err = m.FetchAll(t.Context(), "ctx", "")
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

	_, _ = m.FetchAll(t.Context(), "ctx", "")
	_, _ = m.Refresh(t.Context(), "ctx", "")
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

	_, _ = m.FetchAll(t.Context(), "ctxA", "")
	_, _ = m.FetchAll(t.Context(), "ctxB", "")
	assert.Equal(t, int32(2), s.FetchCalls.Load(),
		"different kubeCtx should bypass cache")
}

func TestManagerAvailabilityCached(t *testing.T) {
	m := NewManager()
	m.SetAvailabilityTTL(200 * time.Millisecond)
	s := &FakeSource{NameStr: "s", Available: true}
	m.Register(s)

	_, _ = m.AnyAvailable(t.Context(), "ctx")
	_, _ = m.AnyAvailable(t.Context(), "ctx")
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

	_, err := m.FetchAll(t.Context(), "ctx", "")
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

// TestFindingIndexCountBySourceCrossSourceOverlap guards the bySource
// counter against undercounting when two sources independently report
// the same Title for the same resource (e.g., Trivy and policy-report
// both flagging the same misconfiguration on a Deployment). Per-resource
// severity counts intentionally dedupe by (resource, Title) so the SEC
// badge does not double-count the same check, but bySource must credit
// every emission so per-source attribution stays accurate.
func TestFindingIndexCountBySourceCrossSourceOverlap(t *testing.T) {
	ref := ResourceRef{Namespace: "p", Kind: "Deployment", Name: "api"}
	idx := BuildFindingIndex([]Finding{
		{Source: "trivy-operator", Severity: SeverityHigh, Title: "privileged", Resource: ref},
		{Source: "policy-report", Severity: SeverityHigh, Title: "privileged", Resource: ref},
	})

	// Per-resource severity bucket dedupes the duplicate report — one
	// "privileged" check on this Deployment.
	assert.Equal(t, 1, idx.For(ref).High)
	// bySource attributes the emission to *both* sources.
	assert.Equal(t, 1, idx.CountBySource("trivy-operator"))
	assert.Equal(t, 1, idx.CountBySource("policy-report"))
}

// TestFindingIndexExcludesReliabilityFromCounts verifies that reliability
// findings (advisor source) never color the per-resource SEC badge: they are
// excluded from the per-resource severity buckets but still credited to
// bySource so the sidebar count stays accurate.
func TestFindingIndexExcludesReliabilityFromCounts(t *testing.T) {
	ref := ResourceRef{Namespace: "p", Kind: "Deployment", Name: "api"}
	idx := BuildFindingIndex([]Finding{
		{Source: "advisor", Category: CategoryReliability, Severity: SeverityMedium, Title: "no PDB", Resource: ref},
		{Source: "heuristic", Category: CategoryMisconfig, Severity: SeverityLow, Title: "default SA", Resource: ref},
	})

	counts := idx.For(ref)
	assert.Equal(t, 0, counts.Medium, "reliability finding must not reach the badge buckets")
	assert.Equal(t, 1, counts.Low, "security finding still counts")
	assert.Equal(t, 1, idx.CountBySource("advisor"), "sidebar per-source count keeps reliability findings")
}

// TestFindingIndexDedupeKeepsHighestSeverity verifies that when two sources
// report the same (resource, Title) at different severities, the per-resource
// bucket records the higher one — independent of iteration order. This guards
// the f.Severity > cur comparison in BuildFindingIndex against silently
// under-coloring the SEC badge if a lower-severity duplicate is seen last.
func TestFindingIndexDedupeKeepsHighestSeverity(t *testing.T) {
	ref := ResourceRef{Namespace: "p", Kind: "Deployment", Name: "api"}
	cases := []struct {
		name     string
		findings []Finding
	}{
		{"low then critical", []Finding{
			{Source: "heuristic", Severity: SeverityLow, Title: "privileged", Resource: ref},
			{Source: "trivy-operator", Severity: SeverityCritical, Title: "privileged", Resource: ref},
		}},
		{"critical then low", []Finding{
			{Source: "trivy-operator", Severity: SeverityCritical, Title: "privileged", Resource: ref},
			{Source: "heuristic", Severity: SeverityLow, Title: "privileged", Resource: ref},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			counts := BuildFindingIndex(tc.findings).For(ref)
			assert.Equal(t, 1, counts.Critical, "highest severity must win")
			assert.Equal(t, 0, counts.Low, "lower-severity duplicate must not be counted")
		})
	}
}

func TestSeverityCountsHighestCount(t *testing.T) {
	cases := []struct {
		name   string
		counts SeverityCounts
		want   int
	}{
		{"empty", SeverityCounts{}, 0},
		{"critical wins over lower buckets", SeverityCounts{Critical: 3, High: 6, Low: 110}, 3},
		{"high when no critical", SeverityCounts{High: 5, Medium: 2}, 5},
		{"medium when only medium and low", SeverityCounts{Medium: 4, Low: 9}, 4},
		{"low only", SeverityCounts{Low: 7}, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.counts.HighestCount())
		})
	}
}

func TestManagerInvalidateClearsCache(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(1 * time.Hour)
	s := &FakeSource{
		NameStr: "s", Available: true,
		Findings: []Finding{{ID: "x"}},
	}
	m.Register(s)

	_, err := m.FetchAll(t.Context(), "ctx", "")
	require.NoError(t, err)
	assert.Equal(t, int32(1), s.FetchCalls.Load())

	_, _ = m.FetchAll(t.Context(), "ctx", "")
	assert.Equal(t, int32(1), s.FetchCalls.Load())

	m.Invalidate()
	_, _ = m.FetchAll(t.Context(), "ctx", "")
	assert.Equal(t, int32(2), s.FetchCalls.Load())
}

// TestManagerInvalidateClearsIndex guards against a stale FindingIndex
// surviving Invalidate(). Without the clear, Index() keeps serving the
// pre-invalidate per-resource counts (driving the SEC badge in the
// explorer) until the next FetchAll lands.
func TestManagerInvalidateClearsIndex(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(1 * time.Hour)
	ref := ResourceRef{Namespace: "p", Kind: "Pod", Name: "x"}
	m.Register(&FakeSource{
		NameStr: "s", Available: true,
		Findings: []Finding{{ID: "1", Source: "s", Severity: SeverityCritical, Resource: ref}},
	})

	_, _ = m.FetchAll(t.Context(), "ctx", "")
	require.Equal(t, 1, m.Index().For(ref).Total(), "index populated after FetchAll")

	m.Invalidate()
	assert.Equal(t, 0, m.Index().For(ref).Total(), "Invalidate must clear cachedIndex too")
}

// TestManagerCachesSuccessfulEmptyResults locks in the fix for the
// "clean cluster refetches every nav tick" bug. A source that returns
// zero findings successfully (Available=true, no error, no findings) is
// a valid result and must be cached so subsequent FetchAll calls within
// refreshTTL skip the source. Previously the code only cached when
// len(res.Findings) > 0, so a zero-finding cluster hammered the source
// on every navigation cycle.
func TestManagerCachesSuccessfulEmptyResults(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(1 * time.Hour)
	s := &FakeSource{NameStr: "s", Available: true} // zero findings on purpose
	m.Register(s)

	_, err := m.FetchAll(t.Context(), "ctx", "")
	require.NoError(t, err)
	require.Equal(t, int32(1), s.FetchCalls.Load(), "first call hits the source")

	_, _ = m.FetchAll(t.Context(), "ctx", "")
	assert.Equal(t, int32(1), s.FetchCalls.Load(), "second call must hit the cache, not the source")
}

// TestManagerNegativeCacheOnAllErroredFetch locks in the throttling-spiral
// fix: when every source errors out (the typical pattern on a slow /
// throttled cluster where every list request times out or fails on
// HTTP/2 stream errors), the result is cached for a short window so the
// next navigation tick doesn't re-fire the same failing requests and
// dig the rate-limit hole even deeper. The window is intentionally
// short (errorTTL ≪ refreshTTL) so genuine recovery is fast.
func TestManagerNegativeCacheOnAllErroredFetch(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(1 * time.Hour)
	m.SetErrorTTL(50 * time.Millisecond)
	bad := &FakeSource{NameStr: "bad", Available: true, FetchErr: errors.New("boom")}
	m.Register(bad)

	_, _ = m.FetchAll(t.Context(), "ctx", "")
	require.Equal(t, int32(1), bad.FetchCalls.Load())

	// Within the error TTL, the result is served from cache.
	_, _ = m.FetchAll(t.Context(), "ctx", "")
	assert.Equal(t, int32(1), bad.FetchCalls.Load(),
		"all-errored fetch within errorTTL must hit the negative cache")

	// After the error TTL, the next call re-fires.
	time.Sleep(60 * time.Millisecond)
	_, _ = m.FetchAll(t.Context(), "ctx", "")
	assert.Equal(t, int32(2), bad.FetchCalls.Load(),
		"after errorTTL elapses the source must be re-probed")
}

// TestManagerSetAvailabilityHintSkipsIsAvailableProbe exercises the
// hint mechanism that the app's loadSecurityAvailability uses to tell
// the manager what each source's availability already is. With the hint
// in place, FetchAll must skip the source's IsAvailable probe — the
// app-level probe already paid that cost (with a 3s budget) and
// re-doing it on every FetchAll doubles the API request volume on slow
// clusters, which is precisely what triggers the client-side throttle
// we're trying to avoid.
func TestManagerSetAvailabilityHintSkipsIsAvailableProbe(t *testing.T) {
	m := NewManager()
	available := &FakeSource{
		NameStr: "trivy", Available: true,
		Findings: []Finding{{ID: "1", Source: "trivy"}},
	}
	hidden := &FakeSource{NameStr: "kubescape", Available: true} // would normally fetch
	m.Register(available)
	m.Register(hidden)

	m.SetAvailability("ctx", map[string]bool{
		"trivy":     true,
		"kubescape": false, // hint says NOT available — skip Fetch entirely
	})

	res, err := m.FetchAll(t.Context(), "ctx", "")
	require.NoError(t, err)
	assert.Len(t, res.Findings, 1, "only available source's findings show")
	assert.Equal(t, int32(0), available.AvailCalls.Load(),
		"hinted-available source must skip the IsAvailable probe")
	assert.Equal(t, int32(1), available.FetchCalls.Load())
	assert.Equal(t, int32(0), hidden.AvailCalls.Load(),
		"hinted-unavailable source must skip both IsAvailable AND Fetch")
	assert.Equal(t, int32(0), hidden.FetchCalls.Load())
}

// TestManagerFetchAllRespectsConcurrencyCap exercises the
// SetMaxFetchConcurrency knob that bounds how many source Fetches can
// run simultaneously. With cap=2 and 6 sources at 50ms each, the total
// elapsed time must be ≥ ceil(6/2) * 50ms = 150ms — the cap forces a
// 3-wave fan-out on a 6-source cluster, which is exactly the
// control-plane-friendliness property we want on busy clusters with
// every CRD-backed source installed.
func TestManagerFetchAllRespectsConcurrencyCap(t *testing.T) {
	m := NewManager()
	m.SetMaxFetchConcurrency(2)
	const sourceCount = 6
	const delay = 50 * time.Millisecond
	for i := range sourceCount {
		m.Register(&FakeSource{
			NameStr:    fmt.Sprintf("s%d", i),
			Available:  true,
			FetchDelay: delay,
		})
	}

	start := time.Now()
	res, err := m.FetchAll(t.Context(), "ctx", "")
	elapsed := time.Since(start)
	require.NoError(t, err)
	assert.Len(t, res.Sources, sourceCount, "every source must produce a status")

	// 6 sources, cap=2 → 3 waves. Lower bound: 3 * 50ms = 150ms.
	// Upper bound is loose to absorb scheduler jitter on CI.
	assert.GreaterOrEqual(t, elapsed, 3*delay,
		"cap=2 with 6 sources must serialize into at least 3 waves")
	assert.Less(t, elapsed, 6*delay+50*time.Millisecond,
		"the cap must not serialize completely — parallel waves must still happen")
}

// TestManagerFetchAllUnboundedConcurrency confirms the
// SetMaxFetchConcurrency(0) escape hatch still runs every source in
// parallel (matches the pre-cap behaviour for tests that need it).
func TestManagerFetchAllUnboundedConcurrency(t *testing.T) {
	m := NewManager()
	m.SetMaxFetchConcurrency(0) // disable cap
	const sourceCount = 6
	const delay = 50 * time.Millisecond
	for i := range sourceCount {
		m.Register(&FakeSource{
			NameStr:    fmt.Sprintf("s%d", i),
			Available:  true,
			FetchDelay: delay,
		})
	}

	start := time.Now()
	_, err := m.FetchAll(t.Context(), "ctx", "")
	elapsed := time.Since(start)
	require.NoError(t, err)
	// Fully parallel: elapsed ≈ delay + scheduler overhead.
	assert.Less(t, elapsed, 2*delay,
		"SetMaxFetchConcurrency(0) must run every source in parallel")
}

// TestManagerFetchAllFallsBackToProbeWhenNoHint guards the fallback path:
// without SetAvailability, FetchAll must still call IsAvailable so
// callers that don't pre-probe (tests, embedded uses) keep working.
func TestManagerFetchAllFallsBackToProbeWhenNoHint(t *testing.T) {
	m := NewManager()
	s := &FakeSource{
		NameStr: "s", Available: true,
		Findings: []Finding{{ID: "1"}},
	}
	m.Register(s)

	_, err := m.FetchAll(t.Context(), "ctx", "")
	require.NoError(t, err)
	assert.Equal(t, int32(1), s.AvailCalls.Load(),
		"no hint → IsAvailable must run as before")
}

// TestManagerUnavailableSourceReportsAvailableFalse guards the fix for
// the cache-decision bug: sources whose IsAvailable returns false were
// previously reported with Available:true in res.Sources, which made
// anySourceSucceeded return true on a cluster with zero installed
// security tools and skipped the negative cache.
func TestManagerUnavailableSourceReportsAvailableFalse(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(1 * time.Hour)
	m.SetErrorTTL(50 * time.Millisecond)
	notInstalled := &FakeSource{NameStr: "trivy", Available: false}
	m.Register(notInstalled)

	res, err := m.FetchAll(t.Context(), "ctx", "")
	require.NoError(t, err)
	require.Len(t, res.Sources, 1)
	assert.False(t, res.Sources[0].Available,
		"unavailable source must report Available=false")
}

// TestManagerErroringSourceUsesNegativeCache verifies that a cluster
// with installed-but-erroring sources falls into the errorTTL window
// rather than the refreshTTL window — without this the throttling
// spiral fix is undone for sources that are Available but throwing.
func TestManagerErroringSourceUsesNegativeCache(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(1 * time.Hour)
	m.SetErrorTTL(40 * time.Millisecond)
	bad := &FakeSource{NameStr: "s", Available: true, FetchErr: errors.New("boom")}
	m.Register(bad)

	_, _ = m.FetchAll(t.Context(), "ctx", "")
	require.Equal(t, int32(1), bad.FetchCalls.Load())

	time.Sleep(60 * time.Millisecond)
	_, _ = m.FetchAll(t.Context(), "ctx", "")
	assert.Equal(t, int32(2), bad.FetchCalls.Load(),
		"errored source must re-fire after errorTTL")
}

// TestManagerIgnoredNamespacesFilterDoesNotAliasCachedSlice guards the
// fix for the in-place namespace-filter aliasing bug. The old code did
// `filtered := res.Findings[:0]` which reused the backing array; the
// next FetchAll then appended into the same array and corrupted any
// caller still iterating the prior cache hit.
func TestManagerIgnoredNamespacesFilterDoesNotAliasCachedSlice(t *testing.T) {
	m := NewManager()
	m.SetIgnoredNamespaces([]string{"kube-system"})
	s := &FakeSource{
		NameStr: "s", Available: true,
		Findings: []Finding{
			{ID: "1", Source: "s", Resource: ResourceRef{Namespace: "kube-system"}},
			{ID: "2", Source: "s", Resource: ResourceRef{Namespace: "default"}},
		},
	}
	m.Register(s)

	res, err := m.FetchAll(t.Context(), "ctx", "")
	require.NoError(t, err)
	require.Len(t, res.Findings, 1)
	// Mutating the returned slice must not corrupt the cached one.
	res.Findings = append(res.Findings, Finding{ID: "tamper"})
	cached, _ := m.FetchAll(t.Context(), "ctx", "")
	require.Len(t, cached.Findings, 1, "cache must be insulated from caller mutation")
	assert.NotEqual(t, "tamper", cached.Findings[0].ID)
}

// TestScanTimeoutGetterSetter covers the default, override, and no-op clamp
// for the per-scan timeout knob used by callers to align their own deadline.
func TestScanTimeoutGetterSetter(t *testing.T) {
	m := NewManager()
	assert.Equal(t, 60*time.Second, m.ScanTimeout(), "default scan timeout")

	m.SetScanTimeout(15 * time.Second)
	assert.Equal(t, 15*time.Second, m.ScanTimeout(), "override takes effect")

	m.SetScanTimeout(0)
	assert.Equal(t, 15*time.Second, m.ScanTimeout(), "zero is a no-op")
	m.SetScanTimeout(-1 * time.Second)
	assert.Equal(t, 15*time.Second, m.ScanTimeout(), "negative is a no-op")
}
