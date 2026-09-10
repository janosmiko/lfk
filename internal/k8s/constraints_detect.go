package k8s

import (
	"context"
	"errors"

	"golang.org/x/sync/errgroup"
)

// constraintsFanoutConcurrency caps the number of constraint sources
// queried at once, bounding growth as more sources are added.
const constraintsFanoutConcurrency = 4

// constraintSource pairs a lookup with the name its Skipped entry uses —
// the list resource it depends on, matching kubectl's plural names.
type constraintSource struct {
	name string
	fn   func(context.Context) ([]ConstraintRow, error)
}

// DetectConstraints runs every constraint source in parallel and merges
// the rows. A source that errors contributes zero rows and one Skipped
// entry, never a false "nothing found". Mirrors DetectOrphans.
func (c *Client) DetectConstraints(ctx context.Context, kubeCtx string, t ConstraintTarget) (ConstraintReport, error) {
	sources := []constraintSource{
		{"resourcequotas", func(ctx context.Context) ([]ConstraintRow, error) { return c.quotaConstraintRows(ctx, kubeCtx, t) }},
		{"limitranges", func(ctx context.Context) ([]ConstraintRow, error) { return c.limitRangeConstraintRows(ctx, kubeCtx, t) }},
		{"poddisruptionbudgets", func(ctx context.Context) ([]ConstraintRow, error) { return c.pdbConstraintRows(ctx, kubeCtx, t) }},
		{"priorityclasses", func(ctx context.Context) ([]ConstraintRow, error) { return c.priorityConstraintRows(ctx, kubeCtx, t) }},
		{"nodes", func(ctx context.Context) ([]ConstraintRow, error) { return c.nodeConstraintRows(ctx, kubeCtx, t) }},
		{"webhookconfigurations", func(ctx context.Context) ([]ConstraintRow, error) { return c.webhookConstraintRows(ctx, kubeCtx, t) }},
	}

	rowsBySource := make([][]ConstraintRow, len(sources))
	errsBySource := make([]error, len(sources))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(constraintsFanoutConcurrency)
	for i, src := range sources {
		g.Go(func() error {
			rowsBySource[i], errsBySource[i] = src.fn(gctx)
			return nil // per-source errors are swallowed here, never propagated to Wait
		})
	}
	_ = g.Wait()

	var report ConstraintReport
	var errs []error
	for i, src := range sources {
		if err := errsBySource[i]; err != nil {
			report.Skipped = append(report.Skipped, src.name)
			errs = append(errs, err)
			continue
		}
		report.Rows = append(report.Rows, rowsBySource[i]...)
	}
	return report, errors.Join(errs...)
}
