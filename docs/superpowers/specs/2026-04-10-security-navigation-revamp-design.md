# Security Navigation Revamp — Design Spec

**Date:** 2026-04-10
**Status:** Approved, ready for implementation plan
**Supersedes:** [2026-04-08-security-metrics-design.md](./2026-04-08-security-metrics-design.md) (Phase 1 dashboard)
**Scope:** Replace the custom security dashboard with hierarchical navigation integrated into lfk's standard explorer

## Overview

The Phase 1 security metrics dashboard (`#` key opens a custom preview-pane renderer with tiles, tabs, and a findings table) was built as a bespoke UI that sat outside lfk's owner-based navigation model. Manual testing surfaced several friction points: awkward fullscreen behavior, mode-specific key handling (capital vs lowercase `j`/`k`), a "loading forever" bug when the dashboard was opened in fullscreen, and the custom renderer diverging from the rest of lfk's preview pane conventions.

This revamp replaces the custom dashboard with **standard lfk navigation**: a new `Security` category in the middle column containing one entry per source (Trivy, Kyverno, Heuristic, ...). Drilling into a source lists findings as regular `model.Item` values rendered by lfk's existing table machinery. Pressing Enter on a finding jumps to the affected resource via the existing owner-jump helper. Filtering, sorting, column toggling, and the right-pane preview all work automatically because findings are ordinary items.

**Primary user value:** The security feature stops being a standalone mode with its own key bindings and instead becomes "just another part of the cluster" — navigable with the same keys, filters, and muscle memory as everything else in lfk.

**Design philosophy:**
- Prefer lfk's existing conventions over custom UI whenever possible.
- Findings are `model.Item` values — no custom state, no custom rendering, no custom key dispatch.
- The security engine (Manager, sources, FindingIndex) is unchanged from Phase 1.
- Deletion is a feature: removing code is often more valuable than adding it.

## Current State

**What's on `feat/security-dashboard` right now (from Phase 1):**
- `internal/security/` — engine package (core types, Manager, FindingIndex)
- `internal/security/heuristic/` — built-in pod-spec checks (10 checks)
- `internal/security/trivyop/` — Trivy Operator CRD adapter (VulnerabilityReport, ConfigAuditReport)
- `internal/ui/securityview.go` — custom dashboard renderer (state struct, tiles, tabs, table, details pane, width adaptation)
- `internal/app/update_security.go` — custom key handler (`handleSecurityKey`, category cycling, cursor movement, message routing)
- `internal/app/messages_security.go` — async fetch message types
- `internal/app/commands_security.go` — `loadSecurityDashboard`, `loadSecurityAvailability` commands
- `internal/ui/explorer_security_badge.go` — SEC column (severity badge in the Name column of the explorer table)
- `__security__` pseudo-resource registered in `internal/model/resource_lookup.go`
- `#` hotkey handler + `H` per-resource hotkey handler
- "Security Findings" action menu entry
- Dashboard fullscreen rendering path in `viewExplorerDashboard`
- Mode-aware `j/k` dispatch (capital in normal view, lowercase in fullscreen)
- Per-cluster `security:` config block with dashboard-specific fields (`per_resource_indicators`, `per_resource_action`, `refresh_ttl`, `availability_ttl`)
- Documentation for all of the above in `README.md`, `docs/keybindings.md`, `docs/config-reference.md`

**What this revamp keeps:**
- The entire engine layer (`internal/security/` and sub-packages) unchanged.
- The SEC column in the explorer table (rename the file, same behavior).
- The `#` hotkey and the "Security Findings" action menu entry (both rewritten for the new navigation model).
- The per-cluster `security:` config block (simplified — dashboard-specific fields removed).

**What this revamp deletes:**
- The custom dashboard renderer and state.
- The custom key handler and all dashboard-specific key bindings.
- The `H` per-resource hotkey.
- The `__security__` pseudo-resource.
- The dashboard fullscreen path.
- The dashboard E2E test.

## Goals and Non-Goals

### Goals

1. Add a `Security` category to the middle column at `LevelResourceTypes`, dynamically populated with entries for each registered and available security source.
2. Each source entry is a `ResourceTypeEntry` with a synthetic `APIGroup` of `"_security"` and a synthetic `Kind` like `__security_trivy-operator__`.
3. Drilling into a source loads findings via `Client.GetResources`, which dispatches to `getSecurityFindings` based on the virtual APIGroup.
4. Findings render in the standard explorer table as `model.Item` values with columns `Severity`, `Resource`, `Title`, `Category`.
5. The right preview pane shows full finding details when a finding is selected — a pure-function renderer reads the Item's columns and produces the detail view.
6. Pressing Enter on a finding jumps to the affected resource via the existing `navigateToOwner` helper.
7. The `#` hotkey jumps to the first entry in the Security category.
8. The "Security Findings" action menu entry navigates to the first source and applies a filter matching the selected resource's name.
9. The SEC column in the explorer (severity badges on Workloads) is preserved.
10. All dashboard-specific code is removed.

### Non-goals

- Kyverno `PolicyReport` source adapter — the design supports it but implementation is deferred to a follow-up PR.
- `kube-bench` source adapter — deferred, same reason.
- Falco runtime security events — bigger integration (not CRD-based), deferred to its own spec.
- "Last-visited source per cluster" memory for the `#` hotkey.
- Custom per-kind severity thresholds.

## Design

### Section 1 — Architecture overview

The revamped flow mirrors lfk's standard owner-based navigation, with a thin dispatch layer that converts security findings into `model.Item` values.

```
[# key pressed]
    -> ascend to LevelResourceTypes
    -> jump cursor to first entry in Security category
    -> standard loadPreview (no change)

[Enter on a source entry (e.g., Trivy)]
    -> navigateChild() — standard lfk flow
    -> nav.Level becomes LevelResources
    -> nav.ResourceType = the Trivy synthetic ResourceTypeEntry
    -> commands.loadResources() calls k8s.Client.GetResources(rt)
    -> GetResources sees APIGroup == "_security", dispatches to
       client.getSecurityFindings(rt, namespace)
    -> getSecurityFindings calls security.Manager.FetchAll and
       converts []Finding to []model.Item
    -> standard middle-column rendering takes over

[Right pane when a finding Item is focused]
    -> view_right.renderRightResources() detects item.Kind ==
       "__security_finding__" and calls ui.RenderFindingDetails(item)

[Enter on a finding]
    -> overridden enterFullView branch for security items:
       jump to the affected resource via navigateToOwner
```

**Key architectural decision:** Security source entries use a virtual `APIGroup` of `"_security"`, mirroring lfk's existing `"_helm"` and `"_portforward"` patterns. The `Client.GetResources` dispatcher already has switch branches for these — we add one more. No new navigation levels, no new column types, no new rendering widgets.

**What the app layer gains:** a `SecuritySourcesFn` hook in `internal/model` that lets `TopLevelResourceTypes()` build the Security category dynamically from the registered, available sources.

**What doesn't change:** `internal/security/` (engine), `internal/security/heuristic/`, `internal/security/trivyop/`. The sources, Manager, and FindingIndex all stay identical.

---

### Section 2 — Package layout

#### Deleted files

| Path | Reason |
|---|---|
| `internal/ui/securityview.go` | Custom dashboard renderer replaced by finding Items in the standard table |
| `internal/ui/securityview_test.go` | Tests for the deleted renderer |
| `internal/ui/explorer_security_badge.go` | Renamed to `explorer_sec_column.go` |
| `internal/ui/explorer_security_badge_test.go` | Same — renamed |
| `internal/app/update_security.go` | Custom key handler and message routing for the dashboard |
| `internal/app/update_security_test.go` | Tests for the deleted handler (two tests move to commands_security_test.go) |
| `internal/app/security_e2e_test.go` | Dashboard flow test — replaced by navigation-based E2E |

#### New files

| Path | Responsibility |
|---|---|
| `internal/k8s/security.go` | `Client.getSecurityFindings`, Finding→Item mapping, severity helpers |
| `internal/k8s/security_test.go` | Tests for the dispatch and mapping |
| `internal/ui/findingdetails.go` | `RenderFindingDetails(item, width, height) string` — pure-function preview renderer |
| `internal/ui/findingdetails_test.go` | Golden tests for the details renderer |
| `internal/ui/explorer_sec_column.go` | SEC badge helpers (relocated from `explorer_security_badge.go`, identical behavior) |
| `internal/ui/explorer_sec_column_test.go` | Tests for the column |
| `internal/app/security_source_entries.go` | `buildSecuritySourceEntries` — builds the Security category entries from the Manager's registered sources |
| `internal/app/security_source_entries_test.go` | Tests for the hook builder |

#### Modified files

| Path | Change |
|---|---|
| `internal/model/types.go` | Add `"Security"` to `coreCategories`. Add `SecurityVirtualAPIGroup = "_security"` constant. Add `SecuritySourceEntry` struct and `SecuritySourcesFn` package-level hook. Add `(Item).ColumnValue(key string) string` method for reading columns by name (used by the k8s dispatch, details renderer, and jump-to-resource handler). |
| `internal/model/resource_lookup.go` | Remove the `__security__` pseudo-item from `FlattenedResourceTypes`. The Security category is populated dynamically by `TopLevelResourceTypes()`. |
| `internal/model/types.go` — `TopLevelResourceTypes()` | Append a Security category whose Types are built at call time from `SecuritySourcesFn()`. Each ResourceTypeEntry has `APIGroup: SecurityVirtualAPIGroup`, synthetic `Kind`, and `Namespaced: false`. |
| `internal/k8s/client.go` — `GetResources()` | New dispatch branch: `if rt.APIGroup == model.SecurityVirtualAPIGroup { return c.getSecurityFindings(...) }`. |
| `internal/k8s/client.go` | `Client` struct gains `securityManager *security.Manager` field and `SetSecurityManager` accessor. |
| `internal/app/app.go` | `NewModel` wires the Manager into `m.client.SetSecurityManager(mgr)` and installs `model.SecuritySourcesFn`. Remove `securityView` and `securityAvailable` fields (dashboard-specific). Keep `securityManager` on the Model for the SEC column. Add `securityAvailabilityByName map[string]bool`. |
| `internal/app/update_keys_actions.go` | Rewrite `handleExplorerActionKeySecurity` to ascend and jump to the first Security entry. Delete `handleExplorerActionKeySecurityResource`. Keep `ascendToResourceTypes` (shared with monitoring `@` handler). |
| `internal/app/update_keys.go` | Remove `isSecurityDashboardKey`, `isSecurityDashboardKeyFullscreen`, and the top-of-`handleExplorerKey` fast path. Remove `__security__` from the Shift+P / F / Right-arrow branches. |
| `internal/app/view_right.go` | Replace the `__security__` pseudo-resource branch with a finding-details branch inside `renderRightResources` (dispatches on `item.Kind == "__security_finding__"`). |
| `internal/app/update_actions.go` | Rewrite `executeActionSecurityFindings` to ascend, jump to first Security entry, drill in, and apply a filter matching the originally-selected resource name. |
| `internal/app/update_navigation.go` | Remove the `securityView = ui.SecurityViewState{}` resets. Remove the `__security__` branch from `navigateChildResourceType`. |
| `internal/app/commands_load_preview.go` | Remove the `__security__` case. |
| `internal/app/commands_security.go` | Delete `loadSecurityDashboard`. Keep `loadSecurityAvailability` — now returns per-source availability for the new `securityAvailabilityByName` map. |
| `internal/app/view.go` | `viewExplorer` still publishes `ui.ActiveSecurityIndex` / `ui.ActiveSecurityAvailable` from `m.securityManager.Index()` for the SEC column. Remove the `viewExplorerDashboard` security branch. |
| `internal/ui/config.go` | Delete `Keybindings.SecurityResource`. Simplify the `SecurityConfig` struct. |
| `internal/model/actions.go` | Simplify `SecurityConfig`: remove `PerResourceIndicators` → rename to `SecColumn`, remove `PerResourceAction`, remove `RefreshTTL` and `AvailabilityTTL`. |
| `internal/ui/help.go` | Replace the "Security Dashboard" section with a shorter "Security" entry. |
| `internal/ui/hintbar.go` | Update `#` hint description. |
| `internal/app/commands.go` | Update startup tips: "Press # to browse security findings by source". Drop the H tip. |
| `README.md` | Rewrite the Security Dashboard subsection as "Security findings browser". |
| `docs/keybindings.md` | Delete the dashboard sub-mode tables. Keep `#` in the global keys table with updated description. Remove `H`. |
| `docs/config-reference.md` | Rewrite the `security:` section with simplified fields. |
| `docs/config-example.yaml` | Rewrite the commented `security:` block. |
| `docs/superpowers/specs/2026-04-08-security-metrics-design.md` | Add a "Status: Superseded by" banner at the top. |

#### Unchanged files

| Path | Reason |
|---|---|
| `internal/security/source.go`, `manager.go`, `testing.go` | Core types and Manager — still the engine |
| `internal/security/heuristic/**` | Source unchanged |
| `internal/security/trivyop/**` | Source unchanged |
| `internal/app/messages_security.go` | Message types still used for async SEC column updates |

---

### Section 3 — Security category registration

**File:** `internal/model/types.go`

The `internal/model` package must not import `internal/security` (to avoid a cycle). To let `TopLevelResourceTypes()` build the Security category from the live set of registered sources, we introduce a package-level function hook:

```go
// SecuritySourceEntry describes one entry shown under the Security category
// in the middle column. Populated at startup by the app layer.
type SecuritySourceEntry struct {
	DisplayName string // "Trivy", "Kyverno", "Heuristic"
	SourceName  string // matches security.SecuritySource.Name() — "trivy-operator", "heuristic", "policy-report"
	Icon        string
	Count       int    // populated from FindingIndex at render time
}

// SecuritySourcesFn returns the list of security source entries to display
// in the Security category. Set by the app at startup. When nil or empty,
// the Security category is still shown (it's a core category) but empty.
var SecuritySourcesFn func() []SecuritySourceEntry

// SecurityVirtualAPIGroup is the APIGroup used by synthetic security
// resource types. Client.GetResources dispatches on this value.
const SecurityVirtualAPIGroup = "_security"

// ColumnValue returns the value of the named column, or "" if the column
// is not present. Used by callers that need to read fields out of an Item
// without knowing the column's position.
func (i Item) ColumnValue(key string) string {
	for _, c := range i.Columns {
		if c.Key == key {
			return c.Value
		}
	}
	return ""
}
```

In `TopLevelResourceTypes()`, append a new category after the existing ones:

```go
// Security category — dynamically populated from SecuritySourcesFn.
// Always visible (heuristic source has no external dependency).
var securityEntries []ResourceTypeEntry
if SecuritySourcesFn != nil {
	for _, src := range SecuritySourcesFn() {
		displayName := src.DisplayName
		if src.Count >= 0 {
			displayName = fmt.Sprintf("%s (%d)", src.DisplayName, src.Count)
		}
		securityEntries = append(securityEntries, ResourceTypeEntry{
			DisplayName: displayName,
			Kind:        "__security_" + src.SourceName + "__",
			APIGroup:    SecurityVirtualAPIGroup,
			APIVersion:  "v1",
			Resource:    "findings",
			Icon:        src.Icon,
			Namespaced:  false,
		})
	}
}
cats = append(cats, ResourceCategory{Name: "Security", Types: securityEntries})
```

And `"Security"` is added to `coreCategories` so it's always shown regardless of CRD discovery.

**App-layer hook installation in `NewModel`:**

```go
// Build the manager and register sources as before.
mgr := security.NewManager()
if kc := m.client.RawClientset(); kc != nil {
	mgr.Register(heuristic.NewWithClient(kc))
}
if dc := m.client.RawDynamic(); dc != nil {
	mgr.Register(trivyop.NewWithDynamic(dc))
}
m.securityManager = mgr
m.client.SetSecurityManager(mgr)

// Install the hook so model.TopLevelResourceTypes sees the live sources.
model.SecuritySourcesFn = func() []model.SecuritySourceEntry {
	return buildSecuritySourceEntries(m.securityManager, m.securityAvailabilityByName)
}
```

**`buildSecuritySourceEntries` helper (`internal/app/security_source_entries.go`):**

```go
func buildSecuritySourceEntries(mgr *security.Manager, availability map[string]bool) []model.SecuritySourceEntry {
	if mgr == nil {
		return nil
	}
	displayByName := map[string]struct {
		display string
		icon    string
	}{
		"heuristic":      {"Heuristic", "◉"},
		"trivy-operator": {"Trivy", "◈"},
		"policy-report":  {"Kyverno", "◇"},
		"kube-bench":     {"CIS", "◆"},
	}
	idx := mgr.Index()
	var entries []model.SecuritySourceEntry
	for _, src := range mgr.Sources() {
		if !availability[src.Name()] {
			continue
		}
		meta := displayByName[src.Name()]
		if meta.display == "" {
			meta.display = src.Name()
			meta.icon = "●"
		}
		entries = append(entries, model.SecuritySourceEntry{
			DisplayName: meta.display,
			SourceName:  src.Name(),
			Icon:        meta.icon,
			Count:       idx.CountBySource(src.Name()),
		})
	}
	return entries
}
```

**Required addition to `FindingIndex`:** a new `CountBySource(name string) int` method. Today the index aggregates counts by resource key; we need a second aggregation keyed on source name. Computed once at index-build time.

---

### Section 4 — Populate pipeline: findings as `model.Item`

**File:** `internal/k8s/security.go` (new)

```go
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

// SetSecurityManager injects the security manager. Must be called before
// GetResources is invoked on a _security APIGroup entry.
func (c *Client) SetSecurityManager(mgr *security.Manager) {
	c.securityManager = mgr
}

// getSecurityFindings is the dispatch target for virtual _security resource
// types. It fetches findings from the manager for the source encoded in the
// ResourceTypeEntry's Kind and returns them as model.Items.
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
			{Key: "ResourceKind", Value: f.Resource.Kind}, // full kind for Enter → jump
		},
	}
	if f.Source != "" {
		item.Columns = append(item.Columns, model.KeyValue{Key: "Source", Value: f.Source})
	}
	if f.Summary != "" || f.Details != "" {
		desc := f.Summary
		if f.Details != "" {
			if desc != "" {
				desc += "\n\n"
			}
			desc += f.Details
		}
		item.Columns = append(item.Columns, model.KeyValue{Key: "Description", Value: desc})
	}
	if len(f.References) > 0 {
		item.Columns = append(item.Columns, model.KeyValue{Key: "References", Value: strings.Join(f.References, "\n")})
	}
	for k, v := range f.Labels {
		item.Columns = append(item.Columns, model.KeyValue{Key: titleCase(k), Value: v})
	}
	return item
}

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

func shortResource(r security.ResourceRef) string {
	if r.Kind == "" || r.Name == "" {
		return "(cluster-scoped)"
	}
	return fmt.Sprintf("%s/%s", shortKind(r.Kind), r.Name)
}

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

func sourceNameFromKind(kind string) string {
	const prefix = "__security_"
	const suffix = "__"
	if len(kind) < len(prefix)+len(suffix) {
		return ""
	}
	if !strings.HasPrefix(kind, prefix) || !strings.HasSuffix(kind, suffix) {
		return ""
	}
	return kind[len(prefix) : len(kind)-len(suffix)]
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
```

**Wired into `GetResources`:**

```go
if rt.APIGroup == model.SecurityVirtualAPIGroup {
	return c.getSecurityFindings(ctx, contextName, namespace, rt)
}
```

**Mapping rationale:**

1. **`Item.Name = finding.Title`** — primary sort key and most-visible cell.
2. **`Item.Kind = "__security_finding__"`** — stable synthetic kind so the preview dispatcher recognizes any finding regardless of source.
3. **`Item.Extra = finding.ID`** — `Extra` already carries opaque IDs for special items (`__overview__`, `__monitoring__`); reusing it here keeps bookmark serialization consistent.
4. **Columns are pre-computed** — the sort helper reads the `Severity` column value rather than the raw int.
5. **`ResourceKind` column (full kind, not short)** — so `Enter` → jump can map back to the real Kubernetes kind without reverse lookups.
6. **Status column maps to lfk's existing row colors** — no new theme colors, no renderer changes.

---

### Section 5 — Right preview pane and Enter behavior

#### 5.1 `RenderFindingDetails` — pure-function renderer

**File:** `internal/ui/findingdetails.go`

```go
// RenderFindingDetails produces the right preview pane body for a selected
// security-finding Item. It reads everything from item.Columns via the
// Item.ColumnValue method, so the renderer has no dependency on
// internal/security.
func RenderFindingDetails(item model.Item, width, height int) string {
	var b strings.Builder

	severity := item.ColumnValue("Severity")
	title := item.ColumnValue("Title")
	fmt.Fprintf(&b, "%s  %s\n", styleSeverityBadge(severity), title)
	b.WriteString(strings.Repeat("─", clamp(width, 1, 120)))
	b.WriteString("\n\n")

	kv := func(k, v string) {
		if v == "" {
			return
		}
		fmt.Fprintf(&b, "  %s  %s\n", padRight(k+":", 12), v)
	}
	kv("Resource", item.ColumnValue("Resource"))
	if item.Namespace != "" {
		kv("Namespace", item.Namespace)
	}
	kv("Source", item.ColumnValue("Source"))
	kv("Category", item.ColumnValue("Category"))

	reserved := map[string]bool{
		"Severity": true, "Resource": true, "Title": true, "Category": true,
		"Source": true, "ResourceKind": true, "Description": true, "References": true,
	}
	var extras []model.KeyValue
	for _, c := range item.Columns {
		if !reserved[c.Key] {
			extras = append(extras, c)
		}
	}
	if len(extras) > 0 {
		b.WriteString("\n")
		for _, c := range extras {
			kv(c.Key, c.Value)
		}
	}

	if desc := item.ColumnValue("Description"); desc != "" {
		b.WriteString("\n  Description:\n")
		for _, line := range wrapLines(desc, clamp(width-4, 20, 120)) {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}

	if refs := item.ColumnValue("References"); refs != "" {
		b.WriteString("\n  References:\n")
		for _, ref := range strings.Split(refs, "\n") {
			fmt.Fprintf(&b, "    %s\n", ref)
		}
	}

	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  [Enter] jump to resource   [o] owner   [y] copy id"))
	return b.String()
}

func styleSeverityBadge(sev string) string {
	switch sev {
	case "CRIT":
		return StatusFailed.Render(" CRIT ")
	case "HIGH":
		return DeprecationStyle.Render(" HIGH ")
	case "MED":
		return StatusProgressing.Render(" MED  ")
	case "LOW":
		return StatusRunning.Render(" LOW  ")
	}
	return " ?    "
}
```

Pure function — takes a `model.Item` and dimensions, returns a string. No dependency on security package or Model state.

#### 5.2 Preview pane wiring

```go
// internal/app/view_right.go — renderRightResources
sel := m.selectedMiddleItem()
if sel != nil && sel.Kind == "__security_finding__" {
	return ui.RenderFindingDetails(*sel, width, height)
}
```

#### 5.3 Enter behavior

```go
// internal/app/update_keys_actions.go — enterFullView
func (m Model) enterFullView() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel != nil && sel.Kind == "__security_finding__" {
		return m.jumpToFindingResource(*sel)
	}
	// ... existing YAML full-view behavior ...
}

func (m Model) jumpToFindingResource(sel model.Item) (tea.Model, tea.Cmd) {
	kind := sel.ColumnValue("ResourceKind")
	resource := sel.ColumnValue("Resource")
	if kind == "" || resource == "" || resource == "(cluster-scoped)" {
		m.setStatusMessage("No affected resource for this finding", true)
		return m, scheduleStatusClear()
	}
	parts := strings.SplitN(resource, "/", 2)
	if len(parts) != 2 {
		return m, nil
	}
	name := parts[1]
	return m.navigateToOwner(kind, name)
}
```

Uses the full `ResourceKind` column (stored in Section 4), so there's no short-kind reverse lookup. `navigateToOwner` is an existing helper that handles level transitions and namespace resolution.

#### 5.4 User-facing behavior

| Action | Result |
|---|---|
| Drill into `Security → Trivy` | Table of findings, colored by severity |
| `j`/`k` | Standard cursor movement in the findings list |
| Right preview pane | Shows full details of the selected finding |
| `Enter` | Jumps to the affected Deployment/Pod/etc. |
| `o` | Owner jump (already works; reuses lfk's existing `o` key) |
| `y` | Copy finding ID to clipboard (existing `CopyName` handler works because `Item.Name` is the title; use `Y` for YAML if needed later) |
| `f` | Text filter on Item.Name and Columns values (free with existing filter) |
| `/` | Search (same) |
| `Esc` / `h` | Navigate parent — back to Security category |
| `#` from anywhere | Ascend + jump to first entry in Security category |

---

### Section 6 — Hotkeys, action menu, SEC column, config, error handling

#### 6.1 `#` hotkey

```go
func (m Model) handleExplorerActionKeySecurity() (tea.Model, tea.Cmd, bool) {
	if m.nav.Level < model.LevelResourceTypes {
		m.setStatusMessage("Select a cluster first", true)
		return m, scheduleStatusClear(), true
	}
	m = m.ascendToResourceTypes()
	for i, item := range m.middleItems {
		if item.Category == "Security" && item.Extra != "" {
			m.setCursor(i)
			m.clampCursor()
			return m, m.loadPreview(), true
		}
	}
	m.setStatusMessage("No security sources available", true)
	return m, scheduleStatusClear(), true
}
```

`handleExplorerActionKeySecurityResource` is deleted. `kb.SecurityResource` is removed from the dispatch switch and from the `Keybindings` struct.

#### 6.2 Action menu — "Security Findings"

```go
func (m Model) executeActionSecurityFindings() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	filterText := sel.Name
	m = m.ascendToResourceTypes()
	var found bool
	for i, item := range m.middleItems {
		if item.Category == "Security" && item.Extra != "" {
			m.setCursor(i)
			m.clampCursor()
			found = true
			break
		}
	}
	if !found {
		m.setStatusMessage("No security sources available", true)
		return m, scheduleStatusClear()
	}
	mdl, cmd := m.navigateChild()
	m2, ok := mdl.(Model)
	if !ok {
		return mdl, cmd
	}
	m2.filterText = filterText
	m2.setCursor(0)
	m2.clampCursor()
	return m2, tea.Batch(cmd, m2.loadPreview())
}
```

Visibility: only added to the action menu when `m.securityAvailable == true` (at least one source is reachable).

#### 6.3 SEC column

Behavior is unchanged from the current implementation. The only changes are:
- File rename: `explorer_security_badge.go` → `explorer_sec_column.go`
- Test file rename: `explorer_security_badge_test.go` → `explorer_sec_column_test.go`
- Public symbols `ActiveSecurityIndex`, `ActiveSecurityAvailable` are preserved so `view.go` wiring is untouched.

#### 6.4 Configuration

**Simplified `SecurityConfig` struct:**

```go
type SecurityConfig struct {
	Enabled   bool                         `json:"enabled" yaml:"enabled"`
	SecColumn bool                         `json:"sec_column" yaml:"sec_column"`
	Sources   map[string]SecuritySourceCfg `json:"sources" yaml:"sources"`
}
```

**Removed fields:** `PerResourceIndicators` (renamed to `SecColumn`), `PerResourceAction`, `RefreshTTL`, `AvailabilityTTL`. TTLs move to code defaults (30s fetch, 60s availability). `PerResourceAction` is dropped because the action menu entry is always shown when the feature is available.

**Example config:**

```yaml
security:
  default:
    enabled: true
    sec_column: true
    sources:
      heuristic:
        enabled: true
        checks:
          - privileged
          - host_namespaces
          - host_path
          - readonly_root_fs
          - run_as_root
          - allow_priv_esc
          - dangerous_caps
          - missing_resource_limits
          - default_sa
          - latest_tag
      trivy_operator:
        enabled: true
      policy_report:
        enabled: false   # Phase 2 — opt-in until wired
      kube_bench:
        enabled: false   # Phase 3
      falco:
        enabled: false   # Phase 4
```

**Keybindings:** `Security: "#"` stays. `SecurityResource` is deleted.

#### 6.5 Error handling and observability

| Condition | Where shown |
|---|---|
| All sources unavailable | `Security` category is empty. `#` shows "No security sources available" in the status bar. |
| One source fails mid-fetch | Cached entry stays visible; new fetches log the error via `slog`. After availability TTL expiry, the entry disappears on the next `SecuritySourcesFn` rebuild. |
| Fetch returns empty | Source entry shows `Trivy (0)`. Drilling in shows "No resources" (the standard explorer empty state). |
| Parse error on a single report | Logged; that report skipped; other reports still appear. |

No dedicated status overlay. Errors go to the existing error log overlay (`!` key) via the standard logging path.

#### 6.6 Refresh behavior

Pressing `r` (refresh) on a security source invalidates the Manager's cache and triggers a new fetch:

```go
func (m Model) handleKeyRefresh() (tea.Model, tea.Cmd) {
	if m.nav.Level == model.LevelResources && strings.HasPrefix(m.nav.ResourceType.Kind, "__security_") {
		if m.securityManager != nil {
			m.securityManager.Invalidate()
		}
	}
	// ... existing refresh behavior — calls loadResources which triggers a fresh FetchAll ...
}
```

`Manager.Invalidate()` is a new public method that clears `cacheKey` and the avail cache without performing a fetch (the standard populate pipeline will do the fetch on the next `loadResources`).

---

### Section 7 — Testing strategy

Following lfk's TDD convention, 80% minimum coverage, table-driven tests with `-race`.

#### Unchanged

| Package | Reason |
|---|---|
| `internal/security/source_test.go` | Core types unchanged |
| `internal/security/manager_test.go` | Concurrent fetch, caching, index builder unchanged (plus one new test for `Manager.Invalidate()`) |
| `internal/security/heuristic/**` | Source unchanged |
| `internal/security/trivyop/**` | Source unchanged |

#### New unit tests

**`internal/k8s/security_test.go`** — dispatch layer (coverage target 90%):
- `TestGetSecurityFindingsNilManager` — safe degrade
- `TestGetSecurityFindingsUnknownKind` — error on malformed kind
- `TestGetSecurityFindingsDispatchesToManager` — uses FakeSource; asserts findings returned
- `TestGetSecurityFindingsFiltersBySourceName` — two sources; asks for one; asserts only that source's findings appear
- `TestFindingToItemMapping` — full Finding → Item shape
- `TestFindingToItemSeverityToStatus` — table-driven over 5 severities
- `TestSeverityOrderSort` — descending order across all severities
- `TestShortResourceEmpty` — `"(cluster-scoped)"` for empty ref
- `TestSourceNameFromKindValid` / `TestSourceNameFromKindInvalid` — parser coverage

**`internal/ui/findingdetails_test.go`** — golden tests (coverage target 85%):
- `TestRenderFindingDetailsFull` — all fields populated
- `TestRenderFindingDetailsMinimal` — only severity + title
- `TestRenderFindingDetailsNarrowWidth` — width adaptation
- `TestRenderFindingDetailsWithExtraLabels` — non-reserved columns appear
- `TestRenderFindingDetailsBadgeColors` — each severity uses the right style

**`internal/model/types_test.go`** (extended):
- `TestItemColumnValuePresent` — returns the value for a known key
- `TestItemColumnValueAbsent` — returns empty string for a missing key
- `TestItemColumnValueEmptyColumns` — nil/empty `Columns` slice

**`internal/ui/explorer_sec_column_test.go`** — relocated from existing file, no new tests.

**`internal/app/security_source_entries_test.go`** — hook builder:
- `TestBuildSecuritySourceEntriesNilManager`
- `TestBuildSecuritySourceEntriesFiltersUnavailable`
- `TestBuildSecuritySourceEntriesFallbackDisplayName`
- `TestBuildSecuritySourceEntriesCountBySource`
- `TestTopLevelResourceTypesIncludesSecurityWhenHookSet`
- `TestTopLevelResourceTypesSecurityEmptyWhenHookNil`

#### App integration tests

**`internal/app/update_keys_actions_test.go`** (extended):
- `TestHandleExplorerActionKeySecurityJumpsToFirstEntry`
- `TestHandleExplorerActionKeySecurityAscendsFromResourcesLevel` — regression guard
- `TestHandleExplorerActionKeySecurityNoSourcesAvailable`
- **Delete:** `TestHandleExplorerActionKeySecurityResource*` — H hotkey removed

**`internal/app/update_actions_test.go`** (extended):
- `TestExecuteActionSecurityFindingsFiltersToResource`
- `TestExecuteActionSecurityFindingsAscendsFromDeeperLevel`
- `TestExecuteActionSecurityFindingsNoSources`

**`internal/app/security_e2e_test.go`** (rewritten):

Replaces the old dashboard-flow E2E. Same real-sources-fake-clients pattern but exercises the new path: build fake kube + dynamic clients with a privileged Pod + a VulnerabilityReport CRD, construct a Model with real sources, install `SecuritySourcesFn`, press `#`, assert cursor lands on a Security source entry, press Enter, assert `LevelResources`, call `loadResources`, process the `resourcesLoadedMsg`, assert Items with `Kind == "__security_finding__"`, render the preview for a selected finding, assert it contains the severity and CVE ID, press Enter on the finding, assert navigation to the affected Deployment.

#### Deleted tests

- `internal/ui/securityview_test.go` (19 tests) — dashboard renderer
- `internal/app/update_security_test.go` (most tests) — dashboard key handler
- `internal/app/security_e2e_test.go` — old dashboard E2E (replaced)

Keep `TestSecurityFindingsLoadedMsgUpdatesState` and `TestFindingIndexRebuiltOnFindingsLoaded` — these cover the async fetch path for the SEC column. They move to `commands_security_test.go`.

#### CI and pre-commit

No changes. `go test -race -cover ./...` and `golangci-lint run ./...` in the pre-commit hook catch everything.

#### Coverage summary

| Package | Target | Delta |
|---|---|---|
| `internal/security` | 85% | unchanged |
| `internal/security/heuristic` | 90% | unchanged |
| `internal/security/trivyop` | 85% | unchanged |
| `internal/k8s` (security.go) | 90% | NEW |
| `internal/ui` (findingdetails + sec_column) | 85% | NEW + rename |
| `internal/app` (security paths) | 80% | shrinks |

---

### Section 8 — Phasing, docs, rollout

#### Single-phase delivery

One PR. The revamp deletes more than it adds; splitting would require keeping the old dashboard alive during an intermediate state, which is strictly worse than a clean atomic swap.

The branch currently sits at 22 commits of the Phase 1 dashboard work on `feat/security-dashboard`. The revamp lands as a series of well-scoped commits on top of the current state.

#### Commit sequence

| # | Commit | Summary |
|---|---|---|
| 1 | `refactor(ui): delete security dashboard view and tests` | Remove `securityview.go`, `securityview_test.go`. Build breaks temporarily — fixed in #2. |
| 2 | `refactor(app): delete dashboard key handling and e2e test` | Remove `update_security.go`, dashboard-specific tests, old `security_e2e_test.go`. Restore build. |
| 3 | `refactor(app): remove __security__ pseudo-resource and H hotkey` | Drop pseudo-item, remove `handleExplorerActionKeySecurityResource`, `kb.SecurityResource`, clean up Shift+P/F/Right-arrow branches. |
| 4 | `refactor(ui): rename SEC badge helpers to explorer_sec_column` | File/test rename only. |
| 5 | `feat(model): add Security virtual category and source hook` | `SecurityVirtualAPIGroup` constant, `SecuritySourceEntry` type, `SecuritySourcesFn` hook, Security category in `TopLevelResourceTypes`. |
| 6 | `feat(security): add FindingIndex.CountBySource helper` | Small addition for source entry count badges. |
| 7 | `feat(k8s): add _security virtual APIGroup dispatch` | `internal/k8s/security.go` with `SetSecurityManager`, `getSecurityFindings`, `findingToItem`, helpers. Extend `GetResources`. |
| 8 | `feat(ui): add finding details preview renderer` | `internal/ui/findingdetails.go` + tests. |
| 9 | `feat(app): wire finding items into preview and Enter` | `renderRightResources` branch, `enterFullView` branch, `jumpToFindingResource`. |
| 10 | `feat(app): add security source entries and hook installation` | `security_source_entries.go`, install `model.SecuritySourcesFn`, add `securityAvailabilityByName`. |
| 11 | `feat(app): rewrite # handler for Security category navigation` | New `handleExplorerActionKeySecurity`. |
| 12 | `feat(app): rewrite Security Findings action menu entry` | New `executeActionSecurityFindings`. |
| 13 | `feat(app): refresh action invalidates security cache` | Extend `handleKeyRefresh`. Add `Manager.Invalidate()`. |
| 14 | `feat(ui): simplify security config block` | Remove dashboard-specific fields, rename, delete `Keybindings.SecurityResource`. |
| 15 | `docs: rewrite security keybindings and README` | Replace dashboard keybinding tables, update README, rewrite config-reference.md, update config-example.yaml. Update hint bar + help screen. Update startup tips. |
| 16 | `test(app): add E2E test for navigation-based security flow` | Final E2E smoke test. |
| 17 | `docs: supersede old dashboard spec and commit new spec` | Commit this design spec, add superseded banner. |

Commits 1–2 must land together to avoid a red build between them.

#### Documentation updates

| Doc | What changes |
|---|---|
| `README.md` | Rewrite Security Dashboard subsection as "Security findings browser". |
| `docs/keybindings.md` | Delete dashboard sub-mode tables. Keep `#` in global keys. Remove `H`. Add short "Security findings browser" section pointing at the action menu. |
| `docs/config-reference.md` | Rewrite `security:` section with simplified fields. |
| `docs/config-example.yaml` | Rewrite commented block. |
| `internal/ui/help.go` | Replace "Security Dashboard" help section with shorter entry. |
| `internal/ui/hintbar.go` | Update `#` hint description. |
| `internal/app/commands.go` | Update startup tips; drop H tip. |
| `docs/superpowers/specs/2026-04-08-security-metrics-design.md` | Add "Status: Superseded by 2026-04-10-security-navigation-revamp-design.md" banner at the top. |

#### Backwards compatibility

No production concern — `feat/security-dashboard` is unmerged. Config files with old dashboard-specific fields (`per_resource_indicators`, `per_resource_action`, `refresh_ttl`, `availability_ttl`, `security_resource` keybinding) load cleanly because `gopkg.in/yaml.v3` ignores unknown fields by default. A brief migration note goes in `docs/config-reference.md`.

#### Rollout

1. Land commits 1–17 on `feat/security-dashboard`.
2. Full test suite + golangci-lint pass.
3. Manual smoke test on a real cluster with Trivy Operator installed.
4. Merge `feat/security-dashboard` → `main` as a single squashed commit (or merge commit — user preference).
5. No feature flag. Default `enabled: true`.

#### Risks and mitigations

| Risk | Mitigation |
|---|---|
| Finding → Item mapping loses data the dashboard surfaced | All extra data (description, references, labels) lives in `item.Columns` and is rendered by `findingdetails.go`. Verified by `TestRenderFindingDetailsFull`. |
| Table column config doesn't auto-handle 4 new columns | lfk's table renderer already reads `Item.Columns` dynamically. No changes needed. |
| Sort order confusing | Server-side sort by severity then resource then title. Users can override with `>`/`<`. |
| `navigateToOwner` fails to find the workload | Error path shows a status message; finding stays visible. |
| `Manager.Invalidate()` races with in-flight fetch | Manager's existing mutex handles it. Worst case: one extra fetch. |
| `#` jumps to first source, which may not be the one user wants | Acceptable for this phase. Future: remember last-visited. |

#### Future enhancements (not in this spec)

- Kyverno PolicyReport source adapter
- kube-bench source adapter
- Falco runtime security events (bigger integration; own spec)
- Last-visited source per cluster for `#` hotkey
- Custom per-kind severity thresholds

## Open Questions

- **`expandShortKind` vs stored full kind** — spec uses stored `ResourceKind` column. If the implementation finds this too verbose (10 extra bytes per finding), revisit and use a reverse lookup function instead.
- **`Manager.Invalidate()` vs `Manager.Refresh()`** — current Phase 1 has `Refresh` which invalidates AND fetches. The new `Invalidate` only invalidates; the populate pipeline does the fetch. If the two-step pattern turns out to be race-prone, consolidate.

## Summary

Replace the custom security dashboard with hierarchical navigation built on lfk's existing explorer. Security becomes a category in the middle column, sources are resource types, and findings are `model.Item` values. The dashboard code is deleted; the engine (Manager, sources, FindingIndex) is unchanged; the user gets a consistent UX that matches the rest of lfk. Seventeen commits on top of the current `feat/security-dashboard` branch deliver the revamp as one PR.
