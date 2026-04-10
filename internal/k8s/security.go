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
// column. Empty refs render as "(cluster-scoped)".
func shortResource(r security.ResourceRef) string {
	if r.Kind == "" || r.Name == "" {
		return "(cluster-scoped)"
	}
	return fmt.Sprintf("%s/%s", shortKind(r.Kind), r.Name)
}

// shortKind abbreviates common workload kinds.
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

// titleCase capitalizes the first byte of s, leaving the rest unchanged.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// getSecurityFindings is the dispatch target for virtual _security resource
// types. It fetches findings from the manager for the source encoded in
// the ResourceTypeEntry's Kind and returns them as model.Items.
//
//nolint:unparam // contextName and namespace are passed straight through from GetResources callers.
func (c *Client) getSecurityFindings(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry) ([]model.Item, error) {
	if c.securityManager == nil {
		return nil, nil
	}
	sourceName := sourceNameFromKind(rt.Kind)
	if sourceName == "" {
		return nil, fmt.Errorf("unrecognized security kind: %s", rt.Kind)
	}
	res, err := c.securityManager.FetchAll(ctx, contextName, namespace)
	if err != nil {
		return nil, fmt.Errorf("security fetch: %w", err)
	}
	items := make([]model.Item, 0)
	for _, f := range res.Findings {
		if f.Source != sourceName {
			continue
		}
		items = append(items, findingToItem(f))
	}
	sort.SliceStable(items, func(i, j int) bool {
		si := severityOrder(items[i])
		sj := severityOrder(items[j])
		if si != sj {
			return si > sj
		}
		if items[i].Namespace != items[j].Namespace {
			return items[i].Namespace < items[j].Namespace
		}
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].ColumnValue("Title") < items[j].ColumnValue("Title")
	})
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
