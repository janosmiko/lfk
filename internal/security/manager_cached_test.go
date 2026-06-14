package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CachedFindings must peek the FetchAll cache without scanning: cold for an
// unseen key, warm after a FetchAll, and keyed by (context, namespace).
func TestCachedFindings(t *testing.T) {
	mgr := NewManager()
	mgr.SetRefreshTTL(time.Hour)
	src := &FakeSource{NameStr: "h", Available: true, Findings: []Finding{{ID: "1", Source: "h"}}}
	mgr.Register(src)

	_, ok := mgr.CachedFindings("kctx", "")
	assert.False(t, ok, "cold cache before any fetch")
	assert.Equal(t, int32(0), src.FetchCalls.Load(), "peek must not scan")

	_, err := mgr.FetchAll(t.Context(), "kctx", "")
	require.NoError(t, err)
	require.Equal(t, int32(1), src.FetchCalls.Load())

	res, ok := mgr.CachedFindings("kctx", "")
	require.True(t, ok, "warm after fetch")
	require.Len(t, res.Findings, 1)
	assert.Equal(t, int32(1), src.FetchCalls.Load(), "peek must not re-scan")

	_, ok = mgr.CachedFindings("other-ctx", "")
	assert.False(t, ok, "different context key is cold")
	_, ok = mgr.CachedFindings("kctx", "ns1")
	assert.False(t, ok, "different namespace key is cold")
}
