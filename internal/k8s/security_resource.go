// Package k8s — security_resource.go
// Per-resource security findings: the cross-source finding-group list behind
// the "Security Findings" action. Unlike getSecurityFindings (one source per
// virtual resource type), this filters the shared scan to the findings that
// touch a given set of ResourceRefs (the resource itself plus its owners — the
// same set the SEC badge aggregates over) and groups them across ALL sources.
package k8s

import (
	"context"
	"fmt"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// GetSecurityFindingsForResource returns the finding groups affecting any of
// refs, across every source, sorted by severity. Per-source fetch errors are
// not surfaced — partial results are the common case (matches the SEC badge
// index, which also renders whatever sources succeeded).
func (c *Client) GetSecurityFindingsForResource(ctx context.Context, contextName, namespace string, refs []security.ResourceRef) ([]model.Item, error) {
	mgr := c.securityManager.Load()
	if mgr == nil {
		return nil, nil
	}
	res, err := mgr.FetchAll(ctx, contextName, namespace)
	if err != nil {
		return nil, fmt.Errorf("security fetch: %w", err)
	}
	return c.findingsForResourceItems(res, refs), nil
}

// GetSecurityFindingsForResourceCached is the cache-only counterpart to
// GetSecurityFindingsForResource (no scan). ok is false on a cold/expired
// cache, signalling the caller to fall back to a scanning fetch.
func (c *Client) GetSecurityFindingsForResourceCached(contextName, namespace string, refs []security.ResourceRef) ([]model.Item, bool, error) {
	mgr := c.securityManager.Load()
	if mgr == nil {
		return nil, false, nil
	}
	res, ok := mgr.CachedFindings(contextName, namespace)
	if !ok {
		return nil, false, nil
	}
	return c.findingsForResourceItems(res, refs), true, nil
}

// findingsForResourceItems filters a FetchResult to findings whose resource
// matches one of refs, groups them per source (preserving the per-source
// ignore policy), and merges the groups into one severity-sorted item list.
// Each item carries a visible Source column — unlike the single-source list,
// rows here can come from different sources, so the column is not noise.
func (c *Client) findingsForResourceItems(res security.FetchResult, refs []security.ResourceRef) []model.Item {
	keys := make(map[string]bool, len(refs))
	for _, r := range refs {
		keys[r.Key()] = true
	}
	var filtered []security.Finding
	for _, f := range res.Findings {
		if keys[f.Resource.Key()] {
			filtered = append(filtered, f)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	checker, showIgnored := c.securityIgnoreSnapshot()
	// Group once per source in first-occurrence order. The order sources are
	// processed in does not matter: sortFindingGroups below imposes the final
	// cross-source ordering.
	var groups []findingGroup
	seenSource := make(map[string]bool)
	for _, f := range filtered {
		if seenSource[f.Source] {
			continue
		}
		seenSource[f.Source] = true
		groups = append(groups, groupFindings(filtered, f.Source, checker, showIgnored)...)
	}
	sortFindingGroups(groups)
	items := make([]model.Item, 0, len(groups))
	for _, g := range groups {
		item := findingGroupToItem(g)
		item.Columns = append(item.Columns, model.KeyValue{Key: "Source", Value: g.Source})
		items = append(items, item)
	}
	return items
}
