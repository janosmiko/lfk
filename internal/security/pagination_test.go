package security

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// fakeLister produces a deterministic sequence of pages so tests can
// assert that ListPaginated walks the continue chain correctly. It
// lives in this test file rather than in testing.go because the
// production helper deliberately avoids depending on the dynamic
// package, and exposing this lister widely would invite broader use.
type fakeLister struct {
	totalItems int
	failOnPage int    // 1-based; 0 disables. fail surfaces as the listErr.
	listErr    error  // returned when failOnPage matches
	calls      int    // count of List invocations (asserted by tests)
	lastLimit  int64  // last requested Limit
	lastCont   string // last requested Continue token
}

func (f *fakeLister) List(_ context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	f.calls++
	f.lastLimit = opts.Limit
	f.lastCont = opts.Continue
	if f.failOnPage != 0 && f.calls == f.failOnPage {
		return nil, f.listErr
	}
	// Decode the continue token: we encode it as the next start index.
	start := 0
	if opts.Continue != "" {
		_, err := fmt.Sscanf(opts.Continue, "i=%d", &start)
		if err != nil {
			return nil, fmt.Errorf("bad continue token: %w", err)
		}
	}
	end := start + int(opts.Limit)
	if opts.Limit == 0 || end > f.totalItems {
		end = f.totalItems
	}
	items := make([]unstructured.Unstructured, 0, end-start)
	for i := start; i < end; i++ {
		u := unstructured.Unstructured{}
		u.SetName(fmt.Sprintf("item-%d", i))
		items = append(items, u)
	}
	cont := ""
	if end < f.totalItems {
		cont = fmt.Sprintf("i=%d", end)
	}
	out := &unstructured.UnstructuredList{Items: items}
	out.SetContinue(cont)
	return out, nil
}

// TestListPaginatedAccumulatesPages — the dominant happy path.
// pager.New must walk the continue chain until the server returns an
// empty token, and ListPaginated must return all items as a single
// list with the type-meta of the first page preserved.
func TestListPaginatedAccumulatesPages(t *testing.T) {
	const total = 753 // not a multiple of page size on purpose
	f := &fakeLister{totalItems: total}

	out, err := ListPaginated(context.Background(), f)
	require.NoError(t, err)
	assert.Len(t, out.Items, total)
	// 753 / 200 page size = 4 pages (200, 200, 200, 153). pager's first
	// call uses the configured PageSize, so calls should equal 4.
	assert.Equal(t, 4, f.calls, "expected 4 paginated requests for 753 items at page size 200")
	assert.Equal(t, int64(DefaultListPageSize), f.lastLimit,
		"every page must request DefaultListPageSize")
	// Continue token cleared on the final page.
	assert.Empty(t, out.GetContinue())
}

// TestListPaginatedEmpty — zero items must complete in a single call
// and return an empty (but non-nil) list, matching the unpaginated
// List shape callers expect.
func TestListPaginatedEmpty(t *testing.T) {
	f := &fakeLister{totalItems: 0}
	out, err := ListPaginated(context.Background(), f)
	require.NoError(t, err)
	assert.Empty(t, out.Items)
	assert.Equal(t, 1, f.calls)
}

// TestListPaginatedSinglePage — when total < page size, pager must
// still return all items and stop (no follow-up call).
func TestListPaginatedSinglePage(t *testing.T) {
	f := &fakeLister{totalItems: 50}
	out, err := ListPaginated(context.Background(), f)
	require.NoError(t, err)
	assert.Len(t, out.Items, 50)
	assert.Equal(t, 1, f.calls)
}

// TestListPaginatedPropagatesError — a non-recoverable list error from
// the underlying API must surface, not be silently dropped. The pager
// auto-recovers from continue-token-expired (410 Gone), but not from
// arbitrary errors — the lfk side surfaces the error to the user as a
// per-source fetch failure.
func TestListPaginatedPropagatesError(t *testing.T) {
	wantErr := errors.New("connection lost")
	f := &fakeLister{totalItems: 1000, failOnPage: 2, listErr: wantErr}

	_, err := ListPaginated(context.Background(), f)
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

// TestListPaginatedRespectsCancellation — cancellation must short-
// circuit the page walk so a slow server-side list doesn't block the
// FetchAll budget. We cancel after the first page lands and assert
// the walker stops requesting more pages.
func TestListPaginatedRespectsCancellation(t *testing.T) {
	f := &fakeLister{totalItems: 10_000}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	_, err := ListPaginated(ctx, f)
	require.Error(t, err, "cancelled context must propagate")
}
