# Union Context Feature

## Overview

The `--union-context` flag merges resources from multiple Kubernetes clusters into a single view. It is intended for teams that run the same workload across 2–3 clusters (e.g., blue/green/canary) and want to see all resources side-by-side without switching contexts.

```shell
lfk --union-context blue --union-context green --union-context yellow --namespace cloud-cd
```

This shows all Pods (or any resource type) in namespace `cloud-cd` from all three clusters, merged in one table with a **Cluster** column identifying the source.

## CLI Interface

| Flag | Type | Notes |
|------|------|-------|
| `--union-context` | `stringArray` (repeatable) | Clusters to include. At least two is the typical usage; one is allowed but degenerates to single-cluster with a Cluster column. Validation caps this at 8 contexts. |
| `--union-set` | `string` | Name of a configured `union_sets` entry to expand into contexts and an optional default namespace. |
| `--namespace` / `-n` | required in union mode unless supplied by `union_sets` | Fetching across multiple clusters in all-namespaces mode is impractical; validation requires exactly one namespace. |
| `--context` | mutually exclusive with `--union-context` | Returns an error if both are supplied. |

Validation runs in `runTUI()` in `main.go` before the TUI starts. Unknown context names produce a specific error naming the offending context.

## Config

Named union views can be stored under the top-level `union_sets` key. Each entry has a `name`, optional `namespace`, and `contexts`. Contexts can be plain strings or objects with a `context` field and optional `color` used for the one-character cluster tile.

```yaml
union_sets:
  - name: staging-west
    namespace: cloud-cd
    contexts:
      - blue
      - green
      - context: yellow
        color: yellow
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

`item.Columns` still gets a `{Key: "Cluster", Value: contextName}` entry prepended so the table renderer shows a Cluster column via the existing `collectExtraColumns` mechanism — no changes to the table renderer were needed.

### Discovery context

API resource discovery runs against `unionContexts[0]`. All union contexts are assumed to be homogeneous (same resource types). Resources present in context A but not in context B show as empty lists from B rather than errors. The `discoveredResources` map is keyed by `unionContexts[0]`, not by `"__union__"`. Every place that would look up `discoveredResources[nav.Context]` in the hot path has a guard:

```go
discoveryCtx := m.nav.Context
if m.unionMode && m.nav.Context == "__union__" && len(m.unionContexts) > 0 {
    discoveryCtx = m.unionContexts[0]
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

### Session persistence

Union mode is ephemeral (CLI-only). `saveCurrentSession()` returns early when `m.unionMode` is true so the union state is never written to `~/.local/state/lfk/session.yaml`. On restart without the flags the app opens normally.

## Files Modified

| File | Change |
|------|--------|
| `main.go` | `--union-context` flag (`StringArrayVar`); validation in `runTUI()` |
| `internal/app/options.go` | `UnionContexts []string` field; `IsUnionMode()` helper; updated `HasCLIOverrides()` |
| `internal/model/types.go` | `ClusterName string` field on `Item` |
| `internal/k8s/client.go` | `GetResourcesUnion()`: parallel fan-out, `ClusterName` stamping, Cluster column injection, merge + sort |
| `internal/app/app.go` | `unionMode bool`, `unionContexts []string` fields on `Model`; union initialisation in `NewModel()` |
| `internal/app/tabs.go` | `effectiveContext()` helper |
| `internal/app/commands_load.go` | Union branch in `loadResources`; `effectiveContext()` applied to 6 functions |
| `internal/app/commands_load_preview.go` | `effectiveContext()` applied to `loadPreviewYAML`, `loadPreviewSecretData` |
| `internal/app/update_navigation.go` | `navigateParent`: no-op at `LevelResourceTypes`; sentinel restoration at `LevelOwned`/`LevelContainers`; discovery context guard at `LevelResources`; `navigateChildResourceType`: discovery context guard; `navigateChildResource`/`navigateChildOwned`: set `nav.Context = sel.ClusterName` |
| `internal/app/update_actions.go` | `buildActionCtx`: use `sel.ClusterName` when in union mode |
| `internal/app/update_bookmarks.go` | `restoreSession`: reset `nav.Context = "__union__"` after session restore |
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
            → stamp ClusterName, prepend Cluster column
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
- **Bookmark navigation**: Jumping to a bookmark that carries a single context may bypass the sentinel. Bookmark jumps in union mode are currently undefined behaviour.
- **`:ctx` command bar**: Switching context via the command bar in union mode is not guarded and may produce unexpected results.
- **Drill-down is single-cluster**: Once the user selects a specific resource and drills down, subsequent navigation operates against that resource's cluster only. There is no "multi-cluster owned resources" view.
