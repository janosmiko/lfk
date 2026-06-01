// Package k8s — security.go
// Dispatch target for the virtual _security APIGroup. Converts findings
// from the security.Manager into model.Items that the standard explorer
// table renders without modification.
package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// severityLabel returns the abbreviated label used in the Severity column.
func severityLabel(s security.Severity) string {
	switch s {
	case security.SeverityCritical:
		return "CRIT"
	case security.SeverityHigh:
		return "HIGH"
	case security.SeverityMedium:
		return "MED"
	case security.SeverityLow:
		return "LOW"
	}
	return "?"
}

// severityOrder returns a sortable integer for an Item whose severity lives
// in its Columns. Higher = more severe.
func severityOrder(it model.Item) int {
	switch it.ColumnValue("Severity") {
	case "CRIT":
		return 4
	case "HIGH":
		return 3
	case "MED":
		return 2
	case "LOW":
		return 1
	}
	return 0
}

// shortResource returns the "kind/name" display form used in the Resource
// column. When Kind or Name is missing the row renders as
// "(unknown resource)" — the previous "(cluster-scoped)" placeholder was
// misleading because real cluster-scoped objects (e.g., ClusterRole/admin)
// carry a non-empty Kind and Name and render through the normal path; the
// empty-ref case only fires when the source could not extract the resource
// (e.g., Falco events without InvolvedObject), and labelling those as
// cluster-scoped suggested a real K8s cluster-level finding existed.
func shortResource(r security.ResourceRef) string {
	if r.Kind == "" || r.Name == "" {
		return "(unknown resource)"
	}
	return fmt.Sprintf("%s/%s", shortKind(r.Kind), r.Name)
}

// shortKind abbreviates common Kubernetes kinds the security sources
// typically target — workloads (Deployment, StatefulSet, ...), networking
// (Service, Ingress, NetworkPolicy), config (ConfigMap, Secret), and a few
// cluster-scoped kinds (Namespace, PersistentVolume) — using the kubectl
// short names where they exist. Unknown kinds pass through unchanged so
// the output never silently drops information.
func shortKind(k string) string {
	switch k {
	case "Deployment":
		return "deploy"
	case "StatefulSet":
		return "sts"
	case "DaemonSet":
		return "ds"
	case "ReplicaSet":
		return "rs"
	case "CronJob":
		return "cron"
	case "Job":
		return "job"
	case "Pod":
		return "pod"
	case "Service":
		return "svc"
	case "Ingress":
		return "ing"
	case "NetworkPolicy":
		return "netpol"
	case "ConfigMap":
		return "cm"
	case "Secret":
		return "secret"
	case "PersistentVolumeClaim":
		return "pvc"
	case "PersistentVolume":
		return "pv"
	case "ServiceAccount":
		return "sa"
	case "Namespace":
		return "ns"
	}
	return k
}

// sourceNameFromKind extracts the security source name from a synthetic
// Kind string like "__security_trivy-operator__".
func sourceNameFromKind(kind string) string {
	const prefix = "__security_"
	const suffix = "__"
	if len(kind) < len(prefix)+len(suffix) {
		return ""
	}
	if !strings.HasPrefix(kind, prefix) || !strings.HasSuffix(kind, suffix) {
		return ""
	}
	inner := kind[len(prefix) : len(kind)-len(suffix)]
	if inner == "" {
		return ""
	}
	return inner
}

// getSecurityFindings is the dispatch target for virtual _security resource
// types. It fetches findings from the manager for the source encoded in
// the ResourceTypeEntry's Kind and returns them as grouped model.Items
// (one item per unique finding title/check). Drilling into a group shows
// the affected resources via GetSecurityAffectedResources.
//
//nolint:unparam // contextName and namespace are passed straight through from GetResources callers.
func (c *Client) getSecurityFindings(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry) ([]model.Item, error) {
	mgr := c.securityManager.Load()
	if mgr == nil {
		return nil, nil
	}
	sourceName := sourceNameFromKind(rt.Kind)
	if sourceName == "" {
		// Defensive: the synthetic Kind format ("__security_<source>__") is
		// produced inside this package, so a parse failure means an
		// internal mismatch rather than a user input. Don't leak the
		// raw sentinel into the error overlay.
		return nil, fmt.Errorf("internal: malformed security resource type")
	}
	res, err := mgr.FetchAll(ctx, contextName, namespace)
	if err != nil {
		return nil, fmt.Errorf("security fetch: %w", err)
	}
	return c.groupSecurityFindings(res, sourceName)
}

// groupSecurityFindings turns a FetchResult into explorer items for one source:
// it surfaces the source's per-source error (so the explorer shows an error
// state rather than an empty list indistinguishable from a fetch failure),
// then groups and ignore-filters. Shared by the scanning and cache-only paths.
func (c *Client) groupSecurityFindings(res security.FetchResult, sourceName string) ([]model.Item, error) {
	if srcErr, ok := res.Errors[sourceName]; ok && srcErr != nil {
		return nil, fmt.Errorf("source %s: %w", sourceName, srcErr)
	}
	checker, showIgnored := c.securityIgnoreSnapshot()
	groups := groupFindings(res.Findings, sourceName, checker, showIgnored)
	items := make([]model.Item, 0, len(groups))
	for _, g := range groups {
		items = append(items, findingGroupToItem(g))
	}
	return items, nil
}

// GetSecurityFindingsCached returns the grouped findings for a security RT from
// the manager cache ONLY (no scan). ok is false when the shared scan is not
// cached / has expired, signalling the caller to fall back to a scanning fetch.
// Lets the explorer render the source's finding list synchronously off the
// scheduler whenever the (coalesced, cached) scan is already warm.
func (c *Client) GetSecurityFindingsCached(contextName, namespace string, rt model.ResourceTypeEntry) ([]model.Item, bool, error) {
	mgr := c.securityManager.Load()
	if mgr == nil {
		return nil, false, nil
	}
	sourceName := sourceNameFromKind(rt.Kind)
	if sourceName == "" {
		return nil, false, nil
	}
	res, ok := mgr.CachedFindings(contextName, namespace)
	if !ok {
		return nil, false, nil
	}
	items, err := c.groupSecurityFindings(res, sourceName)
	return items, true, err
}

// GetSecurityAffectedResources returns the list of resources affected by
// a specific finding group (identified by groupKey) within a security
// source. Each item has Kind __security_affected_resource__ and carries
// the real resource Kind/Name for jumpToFindingResource navigation.
func (c *Client) GetSecurityAffectedResources(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry, groupKey string) ([]model.Item, error) {
	mgr := c.securityManager.Load()
	if mgr == nil {
		return nil, nil
	}
	sourceName := sourceNameFromKind(rt.Kind)
	if sourceName == "" {
		// Same defensive path as getSecurityFindings — see comment there.
		return nil, fmt.Errorf("internal: malformed security resource type")
	}
	res, err := mgr.FetchAll(ctx, contextName, namespace)
	if err != nil {
		return nil, fmt.Errorf("security fetch: %w", err)
	}
	return c.affectedResourcesFromResult(res, sourceName, groupKey)
}

// GetSecurityAffectedResourcesCached is the cache-only counterpart to
// GetSecurityAffectedResources (no scan). ok is false on a cold/expired cache,
// signalling the caller to fall back to a scanning fetch.
func (c *Client) GetSecurityAffectedResourcesCached(contextName, namespace string, rt model.ResourceTypeEntry, groupKey string) ([]model.Item, bool, error) {
	mgr := c.securityManager.Load()
	if mgr == nil {
		return nil, false, nil
	}
	sourceName := sourceNameFromKind(rt.Kind)
	if sourceName == "" {
		return nil, false, nil
	}
	res, ok := mgr.CachedFindings(contextName, namespace)
	if !ok {
		return nil, false, nil
	}
	items, err := c.affectedResourcesFromResult(res, sourceName, groupKey)
	return items, true, err
}

// affectedResourcesFromResult builds the affected-resource items for one
// finding group from a FetchResult: surfaces the source error, matches
// findings by source+group, dedups + sorts resources, and applies the ignore
// policy (hide when show-ignored is off; tag __ignored__ when on). Shared by
// the scanning and cache-only paths.
func (c *Client) affectedResourcesFromResult(res security.FetchResult, sourceName, groupKey string) ([]model.Item, error) {
	// Surface the requested source's per-source error so a source-specific
	// fetch failure isn't mistaken for "no affected resources".
	if srcErr, ok := res.Errors[sourceName]; ok && srcErr != nil {
		return nil, fmt.Errorf("source %s: %w", sourceName, srcErr)
	}
	// Filter to findings matching source and group key.
	var matched []security.Finding
	for _, f := range res.Findings {
		if f.Source == sourceName && findingGroupKey(f) == groupKey {
			matched = append(matched, f)
		}
	}
	// Collect unique resources.
	seen := make(map[string]bool)
	var refs []security.ResourceRef
	for _, f := range matched {
		key := f.Resource.Key()
		if !seen[key] {
			seen[key] = true
			refs = append(refs, f.Resource)
		}
	}
	// Sort refs by namespace then name for stable output.
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Namespace != refs[j].Namespace {
			return refs[i].Namespace < refs[j].Namespace
		}
		if refs[i].Name != refs[j].Name {
			return refs[i].Name < refs[j].Name
		}
		// Tie-break on Kind (then the full key) so same-namespace, same-name
		// resources of different kinds (e.g. Deployment/api vs Service/api)
		// render in a stable order rather than arbitrarily.
		if refs[i].Kind != refs[j].Kind {
			return refs[i].Kind < refs[j].Kind
		}
		return refs[i].Key() < refs[j].Key()
	})
	// Apply the same ignore policy as the group list (getSecurityFindings):
	// hide ignored resources unless show-ignored is on, and when shown, tag
	// them so the renderer can mark them. Without this the drill-in showed
	// ignored resources unconditionally, contradicting the group's filtered
	// affected count.
	checker, showIgnored := c.securityIgnoreSnapshot()
	items := make([]model.Item, 0, len(refs))
	for _, ref := range refs {
		ignored := checker != nil && checker.IsResourceIgnored(sourceName, groupKey, ref.Key())
		if ignored && !showIgnored {
			continue
		}
		item := affectedResourceToItem(ref, groupKey, matched)
		if ignored {
			item.Columns = append(item.Columns, model.KeyValue{Key: "__ignored__", Value: "true"})
		}
		items = append(items, item)
	}
	return items, nil
}
