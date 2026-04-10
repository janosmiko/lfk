# Security Metrics Dashboard — Design Spec

> **Status: Superseded by [2026-04-10-security-navigation-revamp-design.md](./2026-04-10-security-navigation-revamp-design.md)**
>
> The Phase 1 dashboard described in this document was implemented on branch `feat/security-dashboard` but never merged. Manual testing surfaced several friction points (see the revamp spec for details). This file is kept for historical context.

**Date:** 2026-04-08
**Status:** Approved, ready for implementation plan
**Scope:** Phase 1 (MVP) focused, with phases 2 and 3 outlined

## Overview

Add a unified security metrics dashboard to lfk that surfaces vulnerabilities, misconfigurations, policy violations, and compliance findings from multiple sources. The dashboard is integrated into the existing middle-column navigation pattern (mirroring the Monitoring dashboard) and supports per-resource drill-down.

**Primary user value:** A single keystroke (`#`) surfaces the security posture of the active Kubernetes context across whatever sources are installed, with graceful degradation when tools are missing. Users can drill into individual findings, filter by resource, and see at-a-glance indicators in the explorer.

**Design philosophy:**
- Dependency-gated — UI elements only appear when their backing source is available.
- Extensible — new sources can be added without touching the UI layer.
- Partial-failure tolerant — one broken source never blocks the rest.
- Consistent with existing lfk patterns — reuses the `*k8s.Client`, dynamic client, theme colors, column visibility, and keybinding systems.

## Current State

lfk already has scattered security-adjacent features:

| Existing | How it works |
|---|---|
| Trivy vulnerability scan (per-container) | `V` in action menu, CLI-based (`exec.LookPath("trivy")`) |
| RBAC `can-i` browser | `U` key, uses Kubernetes API directly |
| Prometheus/Alertmanager alerts | `@` key overlay, uses K8s API proxy |
| API deprecations detector | `internal/k8s/deprecations.go` |
| NetworkPolicy view | `netpol.go` + `overlay_network.go` |
| Secret decode/edit | `Ctrl+S` toggle |

These features are useful but fragmented. There is no aggregate view of cluster security posture, no integration with in-cluster security scanners (Trivy Operator, Kyverno, kube-bench), and no per-resource security indicators in the explorer.

## Goals and Non-Goals

### Goals (Phase 1)

1. Add a `#` hotkey that opens a Security dashboard pseudo-resource in the middle column (mirroring the `@` Monitoring pattern).
2. The dashboard aggregates findings from the **heuristic** source (built-in checks) and the **trivy-operator** source (reads VulnerabilityReport and ConfigAuditReport CRDs).
3. Findings are browsable with severity tiles, category tabs, scrollable table, and an inline details pane.
4. Per-resource integration via a `SEC` column in the explorer table with severity-colored badges, gated on at least one source being available.
5. An action menu entry ("Security Findings") and an optional `H` hotkey open a per-resource filtered view of the same dashboard.
6. All UI elements are dependency-gated: hidden or disabled when no sources are available.
7. Configuration via per-cluster `security:` block in lfk config, with sensible defaults.
8. Errors per source are isolated — a broken source never blocks others.

### Non-goals (Phase 1)

- Kyverno `PolicyReport` integration (phase 2).
- kube-bench / CIS compliance integration (phase 3).
- Runtime security events (Falco, Tetragon) — out of scope for now.
- Supply-chain verification (cosign, SBOM) — out of scope for now.
- Automatic scheduling of kube-bench Jobs — phase 3 decision.
- Watch-mode auto-refresh of the security view — on-demand only in phase 1.
- TUI E2E testing framework — we use one targeted integration test with fake clients.

## Design

### Section 1 — Architecture overview

#### Package layout

```
internal/security/           (new package)
├── source.go                # SecuritySource interface, Finding, Severity, ResourceRef
├── manager.go               # Manager: aggregates sources, caches, dispatches
├── heuristic/               # Built-in checks (pure Go, no deps)
│   ├── heuristic.go
│   └── checks.go            # privileged, runAsRoot, hostPath, ...
├── trivyop/                 # Trivy Operator CRD adapter
│   ├── trivyop.go
│   └── parse.go
├── policyreport/            # Kyverno / open-standard PolicyReport (phase 2)
│   └── policyreport.go
└── kubebench/               # kube-bench (phase 3)
    ├── kubebench.go
    └── parse.go

internal/ui/                 (new files + edits to existing)
├── securityview.go          # NEW: pure rendering for the dashboard
├── securityview_test.go     # NEW
└── config.go                # EDIT: add Keybindings.Security, Keybindings.SecurityResource,
                             #       and per-cluster security config

internal/app/                (new files)
├── update_security.go       # NEW: key handling, state transitions
├── commands_security.go     # NEW: async fetch commands (tea.Cmd wrappers)
└── messages_security.go     # NEW: security-related tea.Msg types

internal/model/              (edits to existing)
└── resource_lookup.go       # EDIT: register __security__ pseudo-resource
```

The `internal/security/` package sits alongside `internal/k8s/` rather than nested inside it, because security sources extend beyond raw K8s API reads (kube-bench Jobs, potential CLI integrations). The `Manager` receives a `*k8s.Client` via constructor dependency injection.

#### Data flow

```
[# key pressed at ResourceTypes level or deeper]
    -> update_keys_actions.handleExplorerActionKeySecurity()
    -> Finds __security__ item in middle column, sets cursor
    -> loadPreview() detects __security__ extra and triggers fetch
    -> commands_security.fetchAll dispatches security.Manager.FetchAll(ctx, kubeCtx, ns)
    -> Manager iterates registered sources; for each s where s.IsAvailable(ctx) { go s.Fetch(ctx) }
    -> errgroup fans in results; per-source errors collected non-fatally
    -> securityFindingsLoadedMsg returned to model
    -> update_security applies result to SecurityViewState, computes counts, redraws
    -> view_right.go renders the dashboard in the right preview pane
```

#### Key design choices

- The `Manager` lives in `internal/security`, not `internal/k8s`, so non-K8s sources fit naturally.
- The `Manager` takes a `*k8s.Client` via constructor dependency injection. Sources are registered at startup via functional options.
- `IsAvailable(ctx)` is cached with a 60-second TTL to avoid re-probing CRDs on every fetch. Manual refresh invalidates the cache.
- Findings are denormalized to a common `Finding` shape for the UI layer, but sources can keep typed raw data for richer details pane content.

---

### Section 2 — The `SecuritySource` interface

```go
// internal/security/source.go
package security

import "context"

// Category groups a finding by kind so the UI can tab/filter on it.
type Category string

const (
    CategoryVuln       Category = "vuln"       // CVEs from scanners
    CategoryMisconfig  Category = "misconfig"  // workload hardening issues
    CategoryPolicy     Category = "policy"     // policy engine violations
    CategoryCompliance Category = "compliance" // CIS / NSA benchmark fails
)

// Severity is a 4-level scale; sources map their own scales onto these.
type Severity int

const (
    SeverityUnknown Severity = iota
    SeverityLow
    SeverityMedium
    SeverityHigh
    SeverityCritical
)

// ResourceRef identifies the Kubernetes object a finding is attached to.
// Empty ResourceRef means the finding is cluster-scoped (e.g., a CIS control).
type ResourceRef struct {
    Namespace string
    Kind      string
    Name      string
    Container string // optional
}

// Finding is the denormalized row the UI renders. Sources construct these.
type Finding struct {
    ID          string            // stable id for de-dup and inline details
    Source      string            // human name of the source (e.g., "trivy-operator")
    Category    Category
    Severity    Severity
    Title       string            // short one-line title
    Resource    ResourceRef
    Summary     string            // 1-2 line description for the table
    Details     string            // multi-line details for the inline pane
    References  []string          // URLs (CVE links, runbooks)
    Labels      map[string]string // arbitrary extra fields (CVE, package, rule, etc.)
}

// SecuritySource is the contract every adapter implements.
// Implementations are expected to be goroutine-safe and honor ctx cancellation.
type SecuritySource interface {
    // Name returns a stable, lowercase identifier used for config keys,
    // logging, and the source column in the UI. E.g., "trivy-operator".
    Name() string

    // Categories returns the set of categories this source produces.
    // Used by the UI to decide which tabs this source contributes to.
    Categories() []Category

    // IsAvailable returns true if the source's dependencies are reachable
    // from the given cluster context. Results are cached by the Manager
    // (default TTL 60s). Must return quickly or honor ctx timeout.
    // Errors are treated as "not available" but logged for diagnostics.
    IsAvailable(ctx context.Context, kubeCtx string) (bool, error)

    // Fetch collects findings for the given cluster context and optional
    // namespace filter. Empty namespace means all-namespaces.
    // Must honor ctx cancellation.
    Fetch(ctx context.Context, kubeCtx, namespace string) ([]Finding, error)
}
```

**Rationale:**
- 3-method interface follows the "1-3 method" Go idiom.
- `IsAvailable` is separate from `Fetch` so the UI can gate visibility without eagerly fetching.
- `Finding` is UI-facing; sources may keep internal typed data for the details pane.
- `ResourceRef` is plain strings so the package does not depend on `client-go` types — heuristic, trivy, kube-bench findings all share the same shape.
- `Severity` is an int enum for sortability.

---

### Section 3 — Source implementations

#### 3.1 `heuristic` source (always available)

Zero dependencies. Reads from the pod cache that lfk already populates, so it adds no additional API calls.

**Availability:** Always `true`.

**Checks (MVP set):**

| Check | Severity | Rationale |
|---|---|---|
| `privileged: true` | Critical | Container escape risk |
| `hostPID`, `hostNetwork`, `hostIPC` | High | Host namespace sharing |
| `hostPath` volume mount | High | Node filesystem access |
| `runAsUser: 0` or unset + no `runAsNonRoot: true` | Medium | Root-in-container |
| `allowPrivilegeEscalation: true` or unset | Medium | Privilege escalation allowed |
| `capabilities.add` contains `SYS_ADMIN`, `NET_ADMIN`, `ALL` | High | Dangerous capabilities |
| Missing `readOnlyRootFilesystem: true` | Low | Writable root FS |
| Missing resource limits (`limits.cpu`/`limits.memory`) | Low | DoS risk |
| Using `default` ServiceAccount for a workload | Low | RBAC hygiene |
| `latest` image tag or no tag | Low | Supply chain hygiene |

Each check is individually configurable via `security.sources.heuristic.checks` so users can disable noisy ones.

**Category:** `CategoryMisconfig` only.

#### 3.2 `trivyop` source — Trivy Operator CRDs

**Why it's in Phase 1:** A single Trivy Operator install publishes multiple CRDs covering vulnerabilities and workload misconfigurations. Reading two of them in Phase 1 delivers the highest ROI of any external integration.

**Availability:** CRD GVR lookup for `vulnerabilityreports.aquasecurity.github.io`. Cached 60s.

**CRDs consumed in Phase 1:**

| CRD | Category | Notes |
|---|---|---|
| `VulnerabilityReport` | `CategoryVuln` | Enumerate `report.vulnerabilities[]` |
| `ConfigAuditReport` | `CategoryMisconfig` | Complements heuristic; de-dup via `ID = source + checkID + ref` |

**CRDs deferred to Phase 2** (listed here for completeness; the implementation plan may promote them to Phase 1 if effort is small):

| CRD | Category | Notes |
|---|---|---|
| `ExposedSecretReport` | `CategoryMisconfig` | Plaintext secrets in images |
| `RbacAssessmentReport` | `CategoryMisconfig` | Over-permissive roles |

**Fetch strategy:** Uses the `dynamic.Interface` already created in `client.go` with GVR lookups. List-only reads; no writes. Namespace filter applied server-side via `List(namespace)`.

**Schema stability:** Pin to the stable v1alpha1 schema; log a warning and skip on parse errors rather than failing the whole fetch.

#### 3.3 `policyreport` — Phase 2

Reads the open-standard `wgpolicyk8s.io/v1alpha2` `PolicyReport` and `ClusterPolicyReport` CRDs. Used by Kyverno and other tools. Filters out `result: pass` entries to keep the view actionable.

#### 3.4 `kubebench` — Phase 3

Reads existing kube-bench results from a known ConfigMap or CRD (likely the Aqua `kube-bench` operator's `CISKubeBenchReport`). Does not schedule Jobs automatically in this phase.

---

### Section 4 — UI: Security as a pseudo-resource in the middle column

**Pattern:** Mirrors the existing Monitoring dashboard exactly. A `__security__` pseudo-item appears in the Resource Types column. Pressing `#` jumps the cursor to it. The dashboard renders in the right preview pane.

**Pseudo-item placement:** In the Dashboards group, immediately after the Monitoring entry.

#### State

```go
// Lives on the Model, scoped per-tab like other view state.
type SecurityViewState struct {
    AvailableCategories []security.Category   // controls visible tabs
    ActiveCategory      security.Category
    CountsBySeverity    map[security.Severity]int
    CountsByCategory    map[security.Category]int
    Findings            []security.Finding    // for active category, after filter
    Cursor              int
    Scroll              int
    ShowDetail          bool
    Filter              string
    FilterFocus         bool
    ResourceFilter      *security.ResourceRef // non-nil = per-resource view
    Loading             bool
    LastError           error
    LastFetch           time.Time
}
```

#### Layout in the right preview pane

```
Security Dashboard                        updated 12s
─────────────────────────────────────────────────────
 CRIT 12   HIGH 34   MED 89   LOW 120

[Vulns 46]  Misconf 8   Policies 3
─────────────────────────────────────────────────────
 SEV   RESOURCE              SOURCE      TITLE
> CRIT deploy/api  (prod)    trivy-op    CVE-2024-1234
  CRIT deploy/web  (prod)    trivy-op    CVE-2024-5678
  HIGH sts/db      (prod)    trivy-op    CVE-2024-9012
  HIGH ds/agent    (kube-sys) heuristic  privileged
  MED  deploy/web  (prod)    heuristic   runAsRoot
  ...

─ Details ───────────────────────────────────────────
 CVE-2024-1234              CRIT
 Package:  openssl 3.0.7
 Fixed:    3.0.13
 Resource: prod/deploy/api
 https://nvd.nist.gov/vuln/detail/CVE-2024-1234
─────────────────────────────────────────────────────
[/]filter [Tab]tab [Enter]det [r]refresh [J/K]scroll
```

#### Width adaptation

- **Width < 80:** collapse the SOURCE column, show severity as a single colored letter (`C`/`H`/`M`/`L`).
- **Width < 60:** stack tiles vertically (one per line).
- **Width < 40:** only show counts and the active list, no tile grid.

#### Key bindings (scoped to the security preview)

| Key | Action |
|---|---|
| `J`/`K` | Scroll preview down/up (existing convention) |
| `Tab`/`Shift+Tab` | Cycle category tab |
| `1`-`4` | Jump to category by index |
| `Enter` | Toggle inline details pane |
| `o` | Jump to the resource referenced by selected finding |
| `y` | Copy finding ID + URL to clipboard |
| `/` | Open text filter input (substring search over findings) |
| `r` | Force refresh |
| `C` | Clear resource filter (only meaningful when per-resource view is active; no-op otherwise) |

Standard explorer keys (`h`/`l` to leave the level, `j`/`k` to move cursor) continue working as they do for other previews.

#### `#` key handler

```go
// internal/app/update_keys_actions.go
func (m Model) handleExplorerActionKeySecurity() (tea.Model, tea.Cmd, bool) {
    if m.nav.Level < model.LevelResourceTypes {
        m.setStatusMessage("Select a cluster first", true)
        return m, scheduleStatusClear(), true
    }
    for i, item := range m.middleItems {
        if item.Extra == "__security__" {
            m.setCursor(i)
            m.clampCursor()
            return m, m.loadPreview(), true
        }
    }
    return m, nil, true
}
```

Wired in `handleExplorerActionKey` next to the existing `kb.Monitoring` case.

#### UI empty/error states

| Condition | Rendering |
|---|---|
| `Loading: true`, no cached data | Centered "Loading security findings..." with spinner |
| `LastError != nil`, no findings | Error message + "Press r to retry" hint |
| No available sources | "No security sources available. Install Trivy Operator to enable vulnerability scanning." |
| Empty findings, source(s) available | "No security findings" |
| Some sources failed, some succeeded | Affected tab shows a status line: "vulns tab: trivy-operator unreachable" |

---

### Section 5 — Per-resource integration

#### 5.1 `SEC` column in the explorer table

A new optional column with severity-colored indicators.

**Symbols + colors:**

| Symbol | Meaning |
|---|---|
| `●` red | One or more Critical findings |
| `◐` orange | One or more High (no Crit) |
| `○` yellow | Medium or Low only |
| (none) | No findings |

The number after the symbol is the **total** count across all severities for that resource. Highest severity wins when multiple are present.

**Visibility rules:**
1. The column is only added to the table if `Manager.AnyAvailable() == true` for the active context.
2. Controllable from the standard column visibility overlay (`,`).
3. Default: shown when at least one source is available; users can hide it.
4. Explicitly disablable via `security.per_resource_indicators: false`.

**Data source:** A `FindingIndex` map maintained by the Manager, keyed on `ResourceRef` (without container), aggregating `SeverityCounts`. Built once per `FetchAll`, looked up in O(1) per row during render.

#### 5.2 Per-resource detail view

Two entry points:

1. **Action menu entry** — "Security Findings" appears in the `x` action menu when the selected resource type can produce findings AND at least one source is available. Selecting it opens the dashboard pre-filtered to the selected resource.

2. **Optional hotkey** — `Keybindings.SecurityResource` (default: `H`) opens the per-resource view directly. Set to empty string to disable. Auto-disabled (with a status message hint) when no sources are available.

The per-resource view uses the same `SecurityViewState` and rendering as the global view, with `ResourceFilter` set. The header shows the filter:

```
Security: prod/deploy/api    [C] clear filter   updated 12s
```

Pressing `C` (Shift+c, unbound at top level) inside the filtered view clears the filter and returns to the global view. `Esc` leaves the dashboard pane entirely (standard behavior). `x` retains its meaning as the explorer action menu and does **not** clear the filter.

#### 5.3 Configuration

```yaml
security:
  enabled: true                          # master switch (default: true)
  per_resource_indicators: true          # SEC column auto-shown when sources available
  per_resource_action: true              # "Security Findings" entry in action menu
  refresh_ttl: 30s                       # cache TTL for fetched findings
  availability_ttl: 60s                  # cache TTL for IsAvailable checks
  sources:
    heuristic:
      enabled: true
      checks:
        - privileged
        - host_namespaces
        - host_path
        - run_as_root
        - dangerous_capabilities
        - missing_resource_limits
        - default_service_account
        - latest_image_tag
    trivy_operator:
      enabled: true                      # auto-disabled if CRDs not present
    policy_report:
      enabled: true                      # phase 2
    kube_bench:
      enabled: false                     # phase 3, opt-in

keybindings:
  security: "#"                          # default
  security_resource: "H"                 # default; "" to disable
```

All flags default to true. Dependency gating uses `IsAvailable()` at runtime — the config flag is for users who want to explicitly disable a source even when it is available.

---

### Section 6 — Error handling, observability, integration

#### Error handling philosophy

Per-source errors are **isolated** and **never fatal**. The dashboard shows partial results.

```go
// internal/security/manager.go
type FetchResult struct {
    Findings []Finding
    Errors   map[string]error // source name -> error
    Sources  []SourceStatus   // per-source availability + last error
}

type SourceStatus struct {
    Name      string
    Available bool
    LastError error
    LastFetch time.Time
    Count     int
}
```

`Manager.FetchAll` runs sources concurrently via `errgroup`. Each source's error is captured in `Errors[sourceName]` but does not cancel the others. The aggregated result is returned even when some sources failed.

#### Error surfacing

| Severity | Where shown | Example |
|---|---|---|
| All sources failed / none available | Centered preview pane message | "No security sources available." |
| Some sources failed | Per-tab status line | "vulns: trivy-operator unreachable (timeout). 0 findings." |
| Source unavailable (expected) | Tab hidden entirely | Trivy Operator not installed |
| Transient fetch error | Status bar + auto-retry next refresh | API server hiccup |
| Detailed per-source health | `?` inside the dashboard shows Sources pane | Debug empty tab |

#### Logging

All source operations log via `internal/logger`:

```go
logger.Info("security source fetch", "source", s.Name(), "ctx", kubeCtx,
    "namespace", ns, "count", len(findings), "duration", elapsed)
logger.Error("security source fetch failed", "source", s.Name(), "ctx", kubeCtx, "error", err)
```

Errors also flow into the existing error log overlay (`!` key, `ErrorLogEntry` system) so users can see them in the standard place.

#### Integration with existing patterns

| Existing pattern | How security uses it |
|---|---|
| `*k8s.Client` | Sources receive via constructor, no new clientset created |
| `client.dynamic` | CRD reads reuse the existing dynamic interface |
| Pod cache from `client_populate.go` | Heuristic source reads directly, zero extra API calls |
| `Theme.Critical/High/Medium/Low` colors | Severity tiles and badges use these |
| `loadPreview()` dispatch | `__security__` case added, mirroring `__monitoring__` |
| `setStatusMessage()` | Transient feedback ("Refreshing security findings...") |
| `ErrorLogEntry` | Source errors appended via `addLogEntry("ERR", ...)` |
| Column visibility (`,`) | `SEC` column uses standard column-toggle machinery |
| Keybinding config | `Security` and `SecurityResource` added to `Keybindings` |
| Per-cluster config | `security` block follows the `monitoring` pattern |

#### Refresh & lifecycle

| Event | Effect |
|---|---|
| Open security pseudo-item first time | Trigger `FetchAll`; show loading; cache for `refresh_ttl` (default 30s) |
| Re-select within TTL | Show cached immediately, no fetch |
| Re-select after TTL | Show cached + background re-fetch |
| `r` key in dashboard | Force refresh, invalidate availability cache |
| Context switch | Drop cache for previous context |
| Namespace change (`\`) | Drop cache; lazy re-fetch |
| Watch mode (`w`) toggled on | No-op in Phase 1; on-demand only |

#### Cancellation

`Manager.FetchAll` takes a `context.Context` and propagates it via `errgroup.WithContext`. Closing the security view or navigating away cancels in-flight fetches. Each source honors `ctx.Done()` between API calls.

---

### Section 7 — Testing strategy

Following the project's TDD convention (red/green), 80% coverage minimum, table-driven Go tests, `-race`.

#### Per-source unit tests

**`internal/security/heuristic/heuristic_test.go`** — pure functions over `*v1.Pod` specs. Table-driven. **Coverage target: 95%.**

**`internal/security/trivyop/trivyop_test.go`** — uses `dynamic/fake.NewSimpleDynamicClient` to seed fake CRD resources. Tests `IsAvailable` (CRD present/absent/timeout), `Fetch` (parse, namespace filter, empty, malformed, cancellation). **Coverage target: 85%.**

#### Manager-level tests

**`internal/security/manager_test.go`** — uses fake sources implementing the interface. Tests:

| Test | Covers |
|---|---|
| `TestManagerFetchAllParallel` | Sources run concurrently |
| `TestManagerFetchAllPartialFailure` | Some error, others succeed |
| `TestManagerFetchAllAllFail` | Empty Findings, full Errors map |
| `TestManagerCachedAvailability` | IsAvailable cached for `availability_ttl` |
| `TestManagerCachedFetch` | FetchAll cached for `refresh_ttl` |
| `TestManagerInvalidateOnContextSwitch` | Cache dropped on context change |
| `TestManagerForceRefresh` | Bypass cache |
| `TestManagerCancellation` | Cancelling parent ctx propagates |
| `TestManagerFindingsByResource` | Resource filter works |
| `TestManagerSeverityCounts` | Aggregated counts correct |

**Coverage target: 90%.**

#### UI rendering tests

**`internal/ui/securityview_test.go`** — pure rendering, golden-style tests. Covers empty, loading, error, tab visibility, width adaptation (40/60/80/120), cursor highlight, details pane, filter, severity colors. **Coverage target: 85%.**

#### App integration tests

**`internal/app/update_security_test.go`** + **`commands_security_test.go`** — model state transitions using existing test helpers. Covers `#` key behavior, fetch dispatch, message handling, filter-by-resource, per-resource hotkey gating, column visibility gating. **Coverage target: 80%.**

#### End-to-end smoke

**`internal/app/security_e2e_test.go`** — one flow test that wires real Manager + real heuristic + real trivyop against a fake dynamic client seeded with sample Pods and CRDs, drives the model through user keystrokes, asserts on rendered output.

#### CI

Existing `go test -race -cover ./...` in CI catches regressions. The pre-commit hook runs the same. No new CI infrastructure required.

#### Out of scope for Phase 1 testing

- Real Trivy Operator integration — manual smoke test on a real cluster.
- kube-bench — Phase 3.
- Visual regression for terminal rendering — rely on string-equality golden tests.
- Performance benchmarks — defer until a real issue appears.

---

### Section 8 — Phasing, docs, rollout

#### Phase 1 — Foundation + heuristic + trivy-operator (MVP)

**Scope:**

| Item | Files |
|---|---|
| `SecuritySource` interface, `Finding`, `Severity`, `ResourceRef` | `internal/security/source.go` |
| `Manager` (parallel fetch, caching, registration) | `internal/security/manager.go` |
| `heuristic` source | `internal/security/heuristic/...` |
| `trivyop` source | `internal/security/trivyop/...` |
| `__security__` pseudo-resource registration | `internal/model/resource_lookup.go` |
| Dashboard rendering | `internal/ui/securityview.go` |
| App wiring (key, messages, fetch dispatch) | `internal/app/update_security.go`, `commands_security.go`, `messages_security.go`, `update_keys_actions.go` |
| `Keybindings.Security`, `Keybindings.SecurityResource` | `internal/ui/config.go` |
| `security:` config block | `internal/ui/config.go` |
| Action menu "Security Findings" entry | `internal/model/actions.go`, `internal/app/update_actions.go` |
| `SEC` column with badges | `internal/ui/explorer_table.go`, `internal/ui/explorer_format.go` |
| `H` per-resource hotkey | `internal/app/update_keys_actions.go` |
| Tests | `*_test.go` next to production code |
| Help screen entries | `internal/ui/help.go` |
| Hint bar updates | `internal/ui/hintbar.go` |
| README + `docs/keybindings.md` updates | docs |

**Definition of done:**
- All tests pass with `-race`
- 80%+ coverage on new code
- `golangci-lint` clean
- Manual smoke test on a cluster with and without Trivy Operator
- README screenshot updated; new demo gif optional

#### Phase 2 — PolicyReport + per-resource polish

| Item | Notes |
|---|---|
| `policyreport` source | Kyverno + open standard |
| Add `CategoryPolicy` tab | Auto-enabled when source available |
| Per-resource badge UX polish | Based on Phase 1 feedback |

#### Phase 3 — kube-bench / CIS compliance

| Item | Notes |
|---|---|
| `kubebench` source — read existing reports | From operator CRD or ConfigMap |
| Pick supported kube-bench operator | Likely Aqua's `CISKubeBenchReport` |
| `CategoryCompliance` tab | Auto-shown when available |
| Optional: on-demand run action | Schedules Job, watches completion |

#### Documentation updates (Phase 1)

| Doc | What changes |
|---|---|
| `README.md` | Security added to Features; new Integrations entry; new screenshot |
| `docs/keybindings.md` | `#` and `H` in global keys; new dashboard sub-mode section |
| `docs/config-reference.md` | New `security:` section parallel to `monitoring:` |
| `docs/config-example.yaml` | Commented example `security:` block |
| `internal/ui/help.go` | New Security section in built-in help |
| `internal/ui/hintbar.go` | `#` added to explorer hint bar |
| `internal/app/commands.go` | Startup tips: "Press # to open the security dashboard", "Press H on a resource to see its security findings" |

#### Backwards compatibility

- No breaking changes.
- All new fields have defaults; existing configs keep working.
- `__security__` pseudo-resource is additive.
- `SEC` column opt-in via auto-detection.
- Users with custom keybindings already using `#` or `H` can override via config.

#### Rollout

- Ships behind no feature flag. `enabled: true` default.
- Users who don't want it: `security.enabled: false`.
- Phase 2 and 3 decided by feedback, not calendar.

#### Risks and mitigations

| Risk | Mitigation |
|---|---|
| CRD GVR discovery slow → IsAvailable delays | 60s cache, off hot path, disablable |
| 10k+ findings → laggy preview | Cursor + viewport pagination, render only visible rows |
| Heuristic false positives | Per-check config to disable |
| Trivy Operator schema changes | Pin v1alpha1, warn + skip on parse errors |
| Per-resource lookup during render | Pre-built `FindingIndex`, O(1) per row |
| Users already using `#` / `H` | Configurable override, document in changelog |
| Cluttered action menu | Entry only appears when at least one source available |

## Open Questions

- **Optional Trivy Operator CRDs** — `ExposedSecretReport` and `RbacAssessmentReport` are mentioned in Section 3.2 as optional. Implementation plan should decide whether they land in Phase 1 or Phase 2.
- **kube-bench operator choice** — Phase 3 will need to pick between Aqua's `kube-bench` operator, running Jobs directly, or a ConfigMap convention.
- **Watch mode auto-refresh** — Phase 1 is on-demand only. Revisit if users ask for it.
- **Benchmarks for heuristic source** — Not planned for Phase 1. Add if a performance issue appears.

## Summary

Phase 1 delivers a working `#` security dashboard with two sources (heuristic + trivy-operator), per-resource indicators, action menu integration, and an optional per-resource hotkey. All UI elements are dependency-gated. The architecture is extensible — adding Phase 2 (Kyverno PolicyReports) and Phase 3 (kube-bench) is strictly additive. Testing targets 80%+ coverage with a combination of unit, manager-level, UI rendering, app integration, and one E2E smoke test.
