package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/janosmiko/lfk/internal/model"
)

// SummaryBucket is one value group within a summary dimension (e.g. 12 "Healthy").
type SummaryBucket struct {
	Value string
	Count int
}

// SummaryBar is a single breakdown dimension (e.g. "Health") over a resource
// list, with its value buckets ordered worst-first.
type SummaryBar struct {
	Label   string
	Total   int
	Buckets []SummaryBucket
}

// ListSummary aggregates the status of an in-memory resource list for the
// preview-pane header band. It is purely derived from data already loaded for
// the list, so building it costs no API calls.
type ListSummary struct {
	Total int
	Bars  []SummaryBar
}

// Empty reports whether the summary has nothing worth rendering.
func (s ListSummary) Empty() bool { return len(s.Bars) == 0 }

// summaryDimension extracts a groupable status value from an item.
type summaryDimension struct {
	label   string
	valueOf func(model.Item) string
}

// summaryDimensions returns the breakdown axes for a kind, in render order, or
// nil when the kind has no meaningful status rollup. Only curated kinds are
// summarised: grouping an arbitrary kind by Item.Status surfaces noise — e.g.
// StorageClass/IngressClass/PriorityClass set Status to a "default"-class
// marker, not health — so the summary is opt-in per kind rather than universal.
//
// Signal source by kind:
//   - ArgoCD Application(Set): the Health and Sync columns.
//   - Pod / Job / PV / PVC / Node / Namespace: the resource's own Status
//     (phase / readiness).
//   - Workloads (Deployment, StatefulSet, DaemonSet, ReplicaSet): the
//     ready/desired replica ratio, since these carry no Status.
//   - Flux & cert-manager resources: their "Ready" condition column.
//
// Kinds without any dimension still get a count-only band (just the header);
// see RenderListSummary.
func summaryDimensions(kind string) []summaryDimension {
	switch kind {
	case "Application", "ApplicationSet":
		return []summaryDimension{
			{label: "Health", valueOf: columnValueFn("Health")},
			{label: "Sync", valueOf: columnValueFn("Sync Status")},
		}
	case "Pod", "Job", "PersistentVolume", "PersistentVolumeClaim", "Node", "Namespace":
		return []summaryDimension{{label: "Status", valueOf: statusValue}}
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
		return []summaryDimension{{label: "Status", valueOf: readyRatioValue}}
	case "Kustomization", "GitRepository", "HelmRepository", "HelmChart",
		"OCIRepository", "Bucket", "ImageRepository", "ImagePolicy",
		"ImageUpdateAutomation", "Certificate", "CertificateRequest",
		"Issuer", "ClusterIssuer":
		return []summaryDimension{{label: "Status", valueOf: readyConditionValue}}
	default:
		return nil
	}
}

func statusValue(it model.Item) string { return it.Status }

func columnValueFn(key string) func(model.Item) string {
	return func(it model.Item) string { return it.ColumnValue(key) }
}

// readyRatioValue maps a workload's "ready/desired" replica count to a coarse
// status bucket: "Ready" when all desired replicas are ready (including a
// scaled-to-zero 0/0), "NotReady" otherwise. Returns "" when absent/unparseable
// so the item is skipped.
func readyRatioValue(it model.Item) string {
	r, d, ok := parseReadyRatio(it.Ready)
	if !ok {
		return ""
	}
	if r >= d {
		return "Ready"
	}
	return "NotReady"
}

// readyConditionValue maps a Flux/cert-manager "Ready" condition (True/False/
// Unknown) to a coarse status bucket.
func readyConditionValue(it model.Item) string {
	switch it.ColumnValue("Ready") {
	case "True":
		return "Ready"
	case "False", "Unknown":
		return "NotReady"
	default:
		return ""
	}
}

func parseReadyRatio(ready string) (readyN, desiredN int, ok bool) {
	a, b, found := strings.Cut(ready, "/")
	if !found {
		return 0, 0, false
	}
	r, errR := strconv.Atoi(strings.TrimSpace(a))
	d, errD := strconv.Atoi(strings.TrimSpace(b))
	if errR != nil || errD != nil {
		return 0, 0, false
	}
	return r, d, true
}

// BuildListSummary aggregates items into per-dimension buckets for the kind.
// Dimensions with no populated values are omitted, so the result is Empty when
// there is nothing meaningful to show.
func BuildListSummary(kind string, items []model.Item) ListSummary {
	s := ListSummary{Total: len(items)}
	for _, dim := range summaryDimensions(kind) {
		if bar := buildSummaryBar(dim, items); bar.Total > 0 {
			s.Bars = append(s.Bars, bar)
		}
	}
	return s
}

func buildSummaryBar(dim summaryDimension, items []model.Item) SummaryBar {
	counts := make(map[string]int)
	for _, it := range items {
		v := strings.TrimSpace(dim.valueOf(it))
		if v == "" {
			continue
		}
		counts[v]++
	}
	bar := SummaryBar{Label: dim.label}
	for v, c := range counts {
		bar.Buckets = append(bar.Buckets, SummaryBucket{Value: v, Count: c})
		bar.Total += c
	}
	// Worst-first: failed before progressing before ok, then by count (desc)
	// and value (asc) so ordering is fully deterministic.
	sort.Slice(bar.Buckets, func(i, j int) bool {
		bi, bj := bar.Buckets[i], bar.Buckets[j]
		if ri, rj := StatusSeverityRank(bi.Value), StatusSeverityRank(bj.Value); ri != rj {
			return ri < rj
		}
		if bi.Count != bj.Count {
			return bi.Count > bj.Count
		}
		return bi.Value < bj.Value
	})
	return bar
}

const summaryBarCells = 14

// RenderListSummary renders the summary as a compact band: a header line with
// the total resource count, then one proportional colored bar per dimension.
// Kinds with no status dimension still get the header (a count-only band).
// Returns "" only when there is nothing to count. Every line is truncated to
// width.
func RenderListSummary(s ListSummary, kindLabel string, width int) string {
	if s.Total == 0 || width <= 0 {
		return ""
	}

	labelW := 0
	for _, bar := range s.Bars {
		if len(bar.Label) > labelW {
			labelW = len(bar.Label)
		}
	}

	header := DimStyle.Bold(true).Render("SUMMARY") + DimStyle.Render(fmt.Sprintf("  %d %s", s.Total, kindLabel))
	lines := []string{ansi.Truncate(header, width, "")}
	for _, bar := range s.Bars {
		line := fmt.Sprintf("%-*s ", labelW, bar.Label) + renderSummaryBar(bar) + "  " + renderSummaryLegend(bar)
		lines = append(lines, ansi.Truncate(line, width, ""))
	}
	return strings.Join(lines, "\n")
}

func renderSummaryBar(bar SummaryBar) string {
	counts := make([]int, len(bar.Buckets))
	for i, b := range bar.Buckets {
		counts[i] = b.Count
	}
	cells := allocateCells(counts, bar.Total, summaryBarCells)
	var sb strings.Builder
	for i, b := range bar.Buckets {
		if cells[i] <= 0 {
			continue
		}
		sb.WriteString(StatusStyle(b.Value).Render(strings.Repeat("█", cells[i])))
	}
	return sb.String()
}

func renderSummaryLegend(bar SummaryBar) string {
	parts := make([]string, 0, len(bar.Buckets))
	for _, b := range bar.Buckets {
		parts = append(parts, StatusStyle(b.Value).Render(fmt.Sprintf("%d %s", b.Count, b.Value)))
	}
	return strings.Join(parts, "  ")
}

// allocateCells distributes cells across buckets proportional to their counts,
// always summing to exactly cells. Every nonzero bucket is guaranteed at least
// one cell so rare-but-important statuses (e.g. a handful of Errors among
// hundreds of Running pods) stay visible rather than rounding away. When there
// are more nonzero buckets than cells, the first `cells` buckets in input order
// — which callers pass worst-first — each get one cell and the rest get none.
func allocateCells(counts []int, total, cells int) []int {
	out := make([]int, len(counts))
	if total <= 0 || cells <= 0 {
		return out
	}

	var nz []int
	for i, c := range counts {
		if c > 0 {
			nz = append(nz, i)
		}
	}
	if len(nz) == 0 {
		return out
	}
	// Can't fit one cell each: give the most severe (input order) a cell.
	if len(nz) >= cells {
		for _, i := range nz[:cells] {
			out[i] = 1
		}
		return out
	}

	// Proportional floor, raised to a minimum of one per nonzero bucket.
	rem := make([]float64, len(counts))
	used := 0
	for _, i := range nz {
		exact := float64(counts[i]) * float64(cells) / float64(total)
		out[i] = max(int(exact), 1)
		rem[i] = exact - float64(out[i]) // negative when bumped up to 1
		used += out[i]
	}

	// The min-1 floor can overshoot; reclaim from the largest buckets (never
	// below 1) until we are back within budget.
	for used > cells {
		best := -1
		for _, i := range nz {
			if out[i] <= 1 {
				continue
			}
			if best == -1 || out[i] > out[best] {
				best = i
			}
		}
		if best == -1 {
			break
		}
		out[best]--
		used--
	}

	// Hand out any remaining cells by largest fractional remainder.
	for used < cells {
		best := -1
		for _, i := range nz {
			if rem[i] < 0 {
				continue
			}
			if best == -1 || rem[i] > rem[best] {
				best = i
			}
		}
		if best == -1 {
			break
		}
		out[best]++
		rem[best] = -1
		used++
	}
	return out
}
