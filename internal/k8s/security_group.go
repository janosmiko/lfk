// Package k8s -- security_group.go
// Grouping engine for security findings. Groups findings by their
// check/vulnerability title so the explorer can show one row per
// unique check and let users drill into affected resources.
package k8s

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// IgnoreChecker decides whether a finding group or a specific resource
// within a group should be hidden from the UI. Implemented by the app
// layer using the persistent SecurityIgnoreState. A nil checker means
// no filtering (all findings shown).
type IgnoreChecker interface {
	IsGroupIgnored(source, groupKey string) bool
	IsResourceIgnored(source, groupKey, resourceKey string) bool
}

// findingGroup aggregates one or more findings that share the same
// check/vulnerability key into a single navigable row.
type findingGroup struct {
	Key          string
	Title        string
	Severity     security.Severity // highest in group
	Category     security.Category
	Source       string
	Count        int  // number of unique affected resources
	FindingCount int  // total individual findings (may differ from Count due to per-container findings)
	Ignored      bool // true when an IgnoreChecker matched this group
	Summary      string
	Details      string
	References   []string
	Resources    []security.ResourceRef // unique affected resources
}

// findingGroupKey returns the deduplication key for a finding.
// Heuristic findings use the "check" label, trivy vuln findings use
// "cve", trivy misconfig findings use "check_id", policy-report
// findings use "rule". Falls back to Title, then ID.
func findingGroupKey(f security.Finding) string {
	if v := f.Labels["check"]; v != "" {
		return v
	}
	if v := f.Labels["cve"]; v != "" {
		return v
	}
	if v := f.Labels["check_id"]; v != "" {
		return v
	}
	if v := f.Labels["rule"]; v != "" {
		return v
	}
	if f.Title != "" {
		return f.Title
	}
	return f.ID
}

// groupFindings filters findings to those matching sourceName, groups
// them by findingGroupKey, and returns sorted groups (severity desc,
// affected count desc, then title asc). When checker is non-nil,
// globally-ignored groups are omitted unless showIgnored is true.
// Resource-level ignores reduce the group's affected count.
func groupFindings(findings []security.Finding, sourceName string, checker IgnoreChecker, showIgnored bool) []findingGroup {
	type accumulator struct {
		group    findingGroup
		seenRefs map[string]struct{} // ResourceRef.Key() -> seen
	}

	byKey := make(map[string]*accumulator)
	var order []string // preserve insertion order for determinism

	for _, f := range findings {
		if f.Source != sourceName {
			continue
		}

		key := findingGroupKey(f)

		// Apply ignore checks when a checker is provided.
		if checker != nil && !showIgnored {
			if checker.IsGroupIgnored(sourceName, key) {
				continue
			}
			if checker.IsResourceIgnored(sourceName, key, f.Resource.Key()) {
				continue
			}
		}

		acc, exists := byKey[key]
		if !exists {
			acc = &accumulator{
				group: findingGroup{
					Key:      key,
					Title:    f.Title,
					Severity: f.Severity,
					Category: f.Category,
					Source:   f.Source,
				},
				seenRefs: make(map[string]struct{}),
			}
			byKey[key] = acc
			order = append(order, key)
		}

		// Keep highest severity.
		if f.Severity > acc.group.Severity {
			acc.group.Severity = f.Severity
		}

		// First non-empty summary wins.
		if acc.group.Summary == "" && f.Summary != "" {
			acc.group.Summary = f.Summary
		}
		if acc.group.Details == "" && f.Details != "" {
			acc.group.Details = f.Details
		}

		// Merge references, keeping first occurrence order.
		if len(f.References) > 0 && len(acc.group.References) == 0 {
			refs := make([]string, len(f.References))
			copy(refs, f.References)
			acc.group.References = refs
		}

		acc.group.FindingCount++

		// Collect unique ResourceRefs (deduplicated by Key()).
		refKey := f.Resource.Key()
		if _, seen := acc.seenRefs[refKey]; !seen {
			acc.seenRefs[refKey] = struct{}{}
			acc.group.Resources = append(acc.group.Resources, f.Resource)
		}
	}

	groups := make([]findingGroup, 0, len(order))
	for _, key := range order {
		g := byKey[key].group
		g.Count = len(g.Resources)
		// Tag groups as ignored in show-ignored mode.
		if checker != nil && showIgnored && checker.IsGroupIgnored(sourceName, key) {
			g.Ignored = true
		}
		groups = append(groups, g)
	}

	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Severity != groups[j].Severity {
			return groups[i].Severity > groups[j].Severity
		}
		if groups[i].Count != groups[j].Count {
			return groups[i].Count > groups[j].Count
		}
		return groups[i].Title < groups[j].Title
	})

	return groups
}

// findingGroupToItem maps a findingGroup onto a model.Item for the
// explorer table. The Kind sentinel lets navigation code distinguish
// group rows from flat finding rows.
func findingGroupToItem(g findingGroup) model.Item {
	// Use the group key as the Name (CVE ID for trivy, check label for
	// heuristic). Show the full title in the Title column when it differs
	// from the key (e.g., "CVE-2024-1234 in openssl").
	name := g.Key
	item := model.Item{
		Kind:  "__security_finding_group__",
		Name:  name,
		Extra: g.Key,
		Columns: []model.KeyValue{
			{Key: "Severity", Value: severityLabel(g.Severity)},
			{Key: "Affected", Value: fmt.Sprintf("%d", g.FindingCount)},
			{Key: "Category", Value: string(g.Category)},
			{Key: "Source", Value: g.Source},
		},
	}
	// Show the full title when it provides more info than the key alone.
	if g.Title != "" && g.Title != g.Key {
		item.Columns = append(item.Columns, model.KeyValue{
			Key: "Title", Value: g.Title,
		})
	}

	if g.Ignored {
		item.Columns = append(item.Columns, model.KeyValue{
			Key: "__ignored__", Value: "true",
		})
	}

	// Hidden column with affected resource names so the text filter
	// matches when the user navigates from a specific resource via the
	// "Security Findings" action (e.g., filtering by "web" matches
	// groups that affect pod/web). The __ prefix keeps it hidden.
	if len(g.Resources) > 0 {
		var names []string
		for _, r := range g.Resources {
			names = append(names, shortResource(r))
			// Also add the bare resource name for simpler matching.
			if r.Name != "" {
				names = append(names, r.Name)
			}
		}
		item.Columns = append(item.Columns, model.KeyValue{
			Key: "__resources__", Value: strings.Join(names, " "),
		})
	}

	if g.Summary != "" || g.Details != "" {
		desc := g.Summary
		if g.Details != "" {
			if desc != "" {
				desc += "\n\n"
			}
			desc += g.Details
		}
		item.Columns = append(item.Columns, model.KeyValue{
			Key: "Description", Value: desc,
		})
	}

	if len(g.References) > 0 {
		item.Columns = append(item.Columns, model.KeyValue{
			Key: "References", Value: strings.Join(g.References, "\n"),
		})
	}

	return item
}

// affectedResourceToItem creates a model.Item for a single affected
// resource within a finding group drill-down. It filters the full
// findings slice to those matching both the groupKey and the ref,
// then summarizes them into a single row.
func affectedResourceToItem(ref security.ResourceRef, groupKey string, findings []security.Finding) model.Item {
	var highestSev security.Severity
	var count int
	var descriptions []string

	for _, f := range findings {
		if findingGroupKey(f) != groupKey {
			continue
		}
		if f.Resource.Key() != ref.Key() {
			continue
		}
		count++
		if f.Severity > highestSev {
			highestSev = f.Severity
		}
		// Build a per-finding description block with summary + details.
		desc := f.Summary
		if f.Details != "" {
			if desc != "" {
				desc += "\n"
			}
			desc += f.Details
		}
		if desc != "" {
			descriptions = append(descriptions, desc)
		}
	}

	item := model.Item{
		Kind:      "__security_affected_resource__",
		Name:      shortResource(ref),
		Namespace: ref.Namespace,
		Extra:     groupKey,
		CreatedAt: time.Now(),
		Columns: []model.KeyValue{
			{Key: "Severity", Value: severityLabel(highestSev)},
			{Key: "Resource", Value: shortResource(ref)},
			{Key: "ResourceKind", Value: ref.Kind},
			{Key: "Namespace", Value: ref.Namespace},
			{Key: "FindingCount", Value: fmt.Sprintf("%d", count)},
			{Key: "__resource_key__", Value: ref.Key()},
		},
	}
	// Add finding descriptions for the details preview. Separate
	// multiple findings with a divider line.
	if len(descriptions) > 0 {
		item.Columns = append(item.Columns, model.KeyValue{
			Key: "Description", Value: strings.Join(descriptions, "\n---\n"),
		})
	}
	return item
}
