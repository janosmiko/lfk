package security

import (
	"context"
	"sync/atomic"
	"time"
)

// FakeSource is a test helper implementing SecuritySource with configurable
// responses. Exported for use by test packages within the module.
type FakeSource struct {
	NameStr       string
	CategoriesVal []Category
	Available     bool
	AvailableErr  error
	Findings      []Finding
	FetchErr      error
	FetchDelay    time.Duration
	FetchCalls    atomic.Int32
	AvailCalls    atomic.Int32
}

func (f *FakeSource) Name() string           { return f.NameStr }
func (f *FakeSource) Categories() []Category { return f.CategoriesVal }

func (f *FakeSource) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	f.AvailCalls.Add(1)
	return f.Available, f.AvailableErr
}

func (f *FakeSource) Fetch(ctx context.Context, kubeCtx, namespace string) ([]Finding, error) {
	f.FetchCalls.Add(1)
	if f.FetchDelay > 0 {
		select {
		case <-time.After(f.FetchDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.FetchErr != nil {
		return nil, f.FetchErr
	}
	out := make([]Finding, len(f.Findings))
	copy(out, f.Findings)
	return out, nil
}
