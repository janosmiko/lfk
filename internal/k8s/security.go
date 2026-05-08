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
	"time"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// severityToStatus maps Severity onto one of the status strings lfk's table
// renderer already colors (Failed = red, Progressing = yellow/orange,
// Running = green, Pending = dim).
func severityToStatus(s security.Severity) string {
	switch s {
	case security.SeverityCritical, security.SeverityHigh:
		return "Failed"
	case security.SeverityMedium:
		return "Progressing"
	case security.SeverityLow:
		return "Pending"
	}
	return "Unknown"
}

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

// titleCase converts a snake_case label key (e.g., "fixed_version") into a
// space-separated, capitalized form ("Fixed Version") so the rendered table
// header reads "FIXED VERSION" rather than "FIXED_VERSION". Already-uppercase
// inputs (e.g., "ALREADY") and single-segment lowercase inputs ("cve") are
// preserved as a single capitalized word. Each underscore-separated segment
// is independently capitalized via ToUpper of its first byte.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// getSecurityFindings is the dispatch target for virtual _security resource
// types. It fetches findings from the manager for the source encoded in
// the ResourceTypeEntry's Kind and returns them as grouped model.Items
// (one item per unique finding title/check). Drilling into a group shows
// the affected resources via GetSecurityAffectedResources.
//
//nolint:unparam // contextName and namespace are passed straight through from GetResources callers.
func (c *Client) getSecurityFindings(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry) ([]model.Item, error) {
	if c.securityManager == nil {
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
	res, err := c.securityManager.FetchAll(ctx, contextName, namespace)
	if err != nil {
		return nil, fmt.Errorf("security fetch: %w", err)
	}
	// Surface the requested source's per-source error so the explorer
	// renders an error state instead of an empty list — without this the
	// caller sees "0 findings" indistinguishable from a fetch failure.
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

// GetSecurityAffectedResources returns the list of resources affected by
// a specific finding group (identified by groupKey) within a security
// source. Each item has Kind __security_affected_resource__ and carries
// the real resource Kind/Name for jumpToFindingResource navigation.
func (c *Client) GetSecurityAffectedResources(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry, groupKey string) ([]model.Item, error) {
	if c.securityManager == nil {
		return nil, nil
	}
	sourceName := sourceNameFromKind(rt.Kind)
	if sourceName == "" {
		// Same defensive path as getSecurityFindings — see comment there.
		return nil, fmt.Errorf("internal: malformed security resource type")
	}
	res, err := c.securityManager.FetchAll(ctx, contextName, namespace)
	if err != nil {
		return nil, fmt.Errorf("security fetch: %w", err)
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
		return refs[i].Name < refs[j].Name
	})
	items := make([]model.Item, 0, len(refs))
	for _, ref := range refs {
		items = append(items, affectedResourceToItem(ref, groupKey, matched))
	}
	return items, nil
}

// findingToItem maps a security.Finding onto the model.Item shape the
// explorer table already knows how to render. All display data for the
// middle column lives in the first five Columns (Severity, Resource,
// Title, Category, ResourceKind). Details-only fields (Source,
// Description, References, raw labels) live in subsequent columns and
// are read by the finding details preview renderer.
func findingToItem(f security.Finding) model.Item {
	item := model.Item{
		Name:      f.Title,
		Kind:      "__security_finding__",
		Namespace: f.Resource.Namespace,
		Status:    severityToStatus(f.Severity),
		Extra:     f.ID,
		CreatedAt: time.Now(),
		Columns: []model.KeyValue{
			{Key: "Severity", Value: severityLabel(f.Severity)},
			{Key: "Resource", Value: shortResource(f.Resource)},
			{Key: "Title", Value: f.Title},
			{Key: "Category", Value: string(f.Category)},
			{Key: "ResourceKind", Value: f.Resource.Kind},
		},
	}
	if f.Source != "" {
		item.Columns = append(item.Columns, model.KeyValue{
			Key: "Source", Value: f.Source,
		})
	}
	if f.Summary != "" || f.Details != "" {
		desc := f.Summary
		if f.Details != "" {
			if desc != "" {
				desc += "\n\n"
			}
			desc += f.Details
		}
		item.Columns = append(item.Columns, model.KeyValue{
			Key: "Description", Value: desc,
		})
	}
	if len(f.References) > 0 {
		item.Columns = append(item.Columns, model.KeyValue{
			Key: "References", Value: strings.Join(f.References, "\n"),
		})
	}
	// Source-specific labels as additional columns with TitleCase keys.
	// Sort for deterministic test output.
	labelKeys := make([]string, 0, len(f.Labels))
	for k := range f.Labels {
		labelKeys = append(labelKeys, k)
	}
	sort.Strings(labelKeys)
	for _, k := range labelKeys {
		item.Columns = append(item.Columns, model.KeyValue{
			Key: titleCase(k), Value: f.Labels[k],
		})
	}
	return item
}
