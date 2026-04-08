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
