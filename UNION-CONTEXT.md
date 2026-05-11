# Union Context Feature

## Overview

The `--union-context` flag merges resources from multiple Kubernetes clusters into a single view. It is intended for teams that run the same workload across 2–3 clusters (e.g., blue/green/canary) and want to see all resources side-by-side without switching contexts.

```shell
lfk --union-context blue --union-context green --union-context yellow --namespace cloud-cd
```

This shows all Pods (or any resource type) in namespace `cloud-cd` from all three clusters, merged in one table with a **Context** column identifying the source.

## CLI Interface

| Flag | Type | Notes |
|------|------|-------|
| `--union-context` | `stringArray` (repeatable) | Contexts to include. At least two is the typical usage; one is allowed but degenerates to single-cluster with a Context column. Validation caps this at 8 contexts. |
| `--union-set` | `string` | Name of a configured `union_sets` entry to expand into contexts and an optional default namespace. |
| `--namespace` / `-n` | required in union mode unless supplied by `union_sets` | Fetching across multiple clusters in all-namespaces mode is impractical; validation requires exactly one namespace. |
| `--context` | mutually exclusive with `--union-context` | Returns an error if both are supplied. |

Validation runs in `runTUI()` in `main.go` before the TUI starts. Unknown context names produce a specific error naming the offending context.

## Config

Named union views can be stored under the top-level `union_sets` key. The preferred shape is a map keyed by set name; list form with `name:` is also accepted for compatibility. Each entry has an optional `namespace` and `contexts`. Contexts can be plain strings or objects with a `context` or `name` field, optional `color` for the one-character context tile, and optional `namespace`. Namespace resolution uses the first configured member namespace, then the set-level namespace, then an explicit namespace from one of the member kubeconfig contexts. CLI `--namespace` overrides all configured defaults.

Configured union sets appear at the top of the context picker under a `Union Sets` section, followed by the normal `Contexts` section. Starting lfk with `--union-set` behaves like entering that row from the picker: parent navigation returns to the context picker. Anonymous `--union-context` sessions still have no configured picker row to return to.

```yaml
union_sets:
  staging-west:
    namespace: cloud-cd
    contexts:
      - blue
      - green
      - context: yellow
        color: yellow
        namespace: cloud-cd
```

## Design Decisions

### The `"__union__"` sentinel

`nav.Context` doubles as the routing key for every Kubernetes API call. In union mode, there is no single context to route to at `LevelResources`, so `nav.Context` is set to the string `"__union__"` as a sentinel.

- `"__union__"` is never sent to the Kubernetes API.
- It is used as a cache key (`"__union__/pods"`) in `itemCache` and `cacheFingerprints`.
- On drill-down (`navigateChildResource`, `navigateChildOwned`), `nav.Context` is replaced with the real cluster from `item.ClusterName`, so all post-drill-down API calls use the correct cluster.
- On back-navigation to `LevelResources`, `nav.Context` is restored to `"__union__"`.

### `ClusterName` on `model.Item`

A dedicated `ClusterName string` field was added to `model.Item` (not via `item.Columns`) so that drill-down routing, cursor restoration, and action context construction can all reach the source cluster without scanning display columns.

`item.Columns` still gets a `{Key: "Context", Value: contextName}` entry prepended so the table renderer shows a Context column via the existing `collectExtraColumns` mechanism — no changes to the table renderer were needed.

### Discovery context

API resource discovery runs against `unionContexts[0]`. All union contexts are assumed to be homogeneous (same resource types). Resources present in context A but not in context B show as empty lists from B rather than errors. The `discoveredResources` map is keyed by `unionContexts[0]`, not by `"__union__"`. Hot paths that need discovery metadata use `discoveryContext()`:

```go
func (m Model) discoveryContext() string {
    if m.isUnionSentinel() && len(m.unionContexts) > 0 {
        return m.unionContexts[0]
    }
    return m.nav.Context
}
```

### `effectiveContext()` helper

Many load functions use `kctx := m.nav.Context` and pass it directly to the Kubernetes API. At `LevelResources` in union mode that would send `"__union__"` to the API and fail. A helper method resolves this:

```go
func (m Model) effectiveContext() string {
    if m.unionMode && m.nav.Context == "__union__" {
        if sel := m.selectedMiddleItem(); sel != nil && sel.ClusterName != "" {
            return sel.ClusterName
        }
    }
    return m.nav.Context
}
```

This is safe for all callers:
- **Preview calls at `LevelResources`**: `selectedMiddleItem()` is the hovered item, which has `ClusterName` set → returns the right cluster.
- **Post-drill-down calls**: `nav.Context` is already the real cluster, so the condition is false and `nav.Context` is returned directly.

`effectiveContext()` replaced `m.nav.Context` in: `loadOwned`, `loadResourceTree`, `loadContainers`, `loadYAML`, `loadMetrics`, `loadPreviewEvents`, `loadPreviewYAML`, `loadPreviewSecretData`.

`buildActionCtx` uses the same pattern inline: `if m.unionMode && sel.ClusterName != "" { kctx = sel.ClusterName }`.

### Cluster picker

Configured union sets are appended to the context explorer under a dedicated `Union Sets` group. Selecting a set enters union mode using the set's contexts, colors, and resolved namespace. Back navigation from the resource-type level returns to the context explorer when the union view was entered from there. CLI-started union sessions keep the old no-parent behavior because they did not come from a picker row.

### Union dashboards

At the union resource-type level, the `Cluster` and `Monitoring` dashboard rows preview the union set's member contexts in the right pane instead of trying to aggregate dashboard data. Right-arrow opens that member list in the middle column. Right-arrow on a member opens that context as a normal single-context resource-type view, where the existing dashboard and context-wide tools keep their original semantics. Back returns first to the union dashboard member list, then to the union resource-type list.

### Session persistence

Union mode is ephemeral. `saveCurrentSession()` returns early when `m.unionMode` is true so the union state is never written to `~/.local/state/lfk/session.yaml`. On restart without the flags the app opens normally.

## Files Modified

| File | Change |
|------|--------|
| `main.go` | `--union-context` flag (`StringArrayVar`); validation in `runTUI()` |
| `internal/app/options.go` | `UnionContexts []string` field; `IsUnionMode()` helper; updated `HasCLIOverrides()` |
| `internal/model/types.go` | `ClusterName string` field on `Item` |
| `internal/k8s/client.go` | `GetResourcesUnion()`: parallel fan-out, `ClusterName` stamping, Context column injection, merge + sort |
| `internal/app/app.go` | `unionMode bool`, `unionContexts []string` fields on `Model`; union initialisation in `NewModel()` |
| `internal/app/app_helpers.go`, `internal/app/tabs.go` | `discoveryContext()` and `effectiveContext()` helpers |
| `internal/app/commands_load.go` | Union branch in `loadResources`; `effectiveContext()` applied to 6 functions |
| `internal/app/commands_load_preview.go` | `effectiveContext()` applied to `loadPreviewYAML`, `loadPreviewSecretData` |
| `internal/app/union_dashboards.go` | Synthetic union dashboard member rows; member drill-in/back helpers |
| `internal/app/update_navigation.go` | `navigateParent`: no-op at `LevelResourceTypes`; sentinel restoration at `LevelOwned`/`LevelContainers`; discovery context guard at `LevelResources`; `navigateChildResourceType`: discovery context guard; `navigateChildResource`/`navigateChildOwned`: set `nav.Context = sel.ClusterName` |
| `internal/app/update_actions.go` | `buildActionCtx`: use `sel.ClusterName` when in union mode |
| `internal/app/update_bookmarks.go`, `internal/app/update_bookmarks_session.go` | session/bookmark restore keeps union navigation keyed by `"__union__"` while using the first member for discovery |
| `internal/app/update_resources_loaded.go` | `updateAPIResourceDiscovery`: `isCurrentContext` guard for sentinel; `updateResourcesLoadedMain`: cluster-aware cursor restoration |
| `internal/app/view_status.go` | `middleColumnHeader`: `[UNION]` suffix; `breadcrumb`: shows `[blue+green+yellow]` |
| `internal/app/session.go` | `saveCurrentSession`: early return in union mode |

## Data Flow at `LevelResources`

```text
watch tick / user navigates to Pods
    → loadResources(false)
        → union branch: nav.Context == "__union__" && unionMode
        → GetResourcesUnion(ctx, unionContexts, namespace, rt)
            → goroutine per context: GetResources(ctx, contextName, ...)
            → stamp ClusterName, prepend Context column
            → merge + sort by (Name, ClusterName)
        → resourcesLoadedMsg → updateResourcesLoadedMain
            → itemCache["__union__/pods"] = items
            → cursor restored by (Name, Namespace, ClusterName) in union mode
            → loadPreview() → effectiveContext() → hovered item's ClusterName
```

## Cursor Stability in Union Mode

Deployments (and other resources with canonical names) exist in every cluster with the same name. The existing `restoreCursorToItem` matches on `Name + Namespace + Extra + Kind`, which always lands on the first alphabetical cluster's entry after a watch-mode refresh.

Fix in `updateResourcesLoadedMain`: capture `prevCluster` before items are replaced; after sorting, prefer a `Name + Namespace + ClusterName` match before falling back to the standard key.

## Known Limitations

- **CRD asymmetry**: If clusters have different CRDs, resources present in cluster A but not cluster B produce empty results from B — not an error. A future improvement would take the union of discovered resources across all contexts.
- **Context-wide tools**: Cluster dashboard and monitoring dashboard expose the union members first, then open a selected member as a normal single-context view. Context-aware bookmarks can target named `union_sets` and switch between union sets and regular contexts; anonymous `--union-context` views still only support context-free bookmarks because they have no durable configured name. Can-I RBAC browser supports union-mode forward checks with allow/deny/mixed verb states. Pinned groups can be managed for named union sets; anonymous union sessions still cannot persist pins. Per-row Port Forward is available for Pods, Services, Deployments, StatefulSets, and DaemonSets because the selected row carries `ClusterName`; the Port Forwards virtual resource remains a normal global lfk view. Reverse Who-Can and orphan scans remain per-context and are blocked at the union sentinel.
- **Command bar context commands**: `:ctx`, shell commands, and `:kubectl`/`:k` commands are blocked at the union sentinel because they require one active context.
- **Drill-down is single-cluster**: Once the user selects a specific resource and drills down, subsequent navigation operates against that resource's cluster only. There is no "multi-cluster owned resources" view.
