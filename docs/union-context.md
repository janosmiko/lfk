# Union View

Union view merges resources from several Kubernetes clusters into a single table. It is built for teams that run the same workload across a handful of clusters — blue/green/canary, or one cluster per region — and want to see everything side by side without switching contexts.

## Starting a union view

Pass `--union-context` once per cluster, along with a namespace:

```shell
lfk --union-context blue --union-context green --union-context yellow --namespace cloud-cd
```

This opens the chosen resource type from namespace `cloud-cd` across all three clusters in one table.

| Flag | Purpose |
|------|---------|
| `--union-context` | A kubeconfig context to include. Repeat it for each cluster, up to 8. |
| `--union-set` | Name of a union set defined in config — expands into its contexts and namespace. |
| `--namespace`, `-n` | Required. Union view fetches a single namespace from every cluster. All-namespaces mode is not available. A `union_sets` entry can supply this instead. |

`--union-context` and `--union-set` cannot be combined with `--context`. Unknown context names are reported at startup.

Configured union sets also appear at the top of the cluster picker under a **Union Sets** heading — pick one to enter union view without passing any flags.

## The context column

Every merged row carries a **Context** column naming its source cluster. It behaves like any other column: sort by it, reorder it, or hide it from the column toggle (`,`). Rows are sorted by name first and context second, so the same resource lines up across clusters.

## Named union sets

Save recurring cluster groups in the config file under `union_sets` so you don't retype long `--union-context` lists. Recall one with `--union-set <name>`.

```yaml
union_sets:
  # Minimal — a namespace and a list of contexts.
  prod-regions:
    namespace: web
    contexts:
      - prod-eu
      - prod-us
      - prod-ap

  # Detailed — per-cluster color tiles and namespace overrides.
  staging-west:
    namespace: cloud-cd
    contexts:
      - blue
      - green
      - context: yellow
        color: yellow
        namespace: cloud-cd
```

- `contexts` — the clusters to merge. Each entry is a context name, or an object with a `context` (or `name`), an optional one-character `color` tile, and an optional `namespace`.
- `namespace` — the namespace to open across the set. The command-line `--namespace` flag overrides it.

For the complete schema, available tile colors, and namespace precedence, see [config-reference.md](config-reference.md#union-sets).

## Working in a union view

- **Drill-down** — selecting a resource follows it into its own cluster, where navigation, dashboards, and actions behave like a normal single-cluster session. Going back returns to the merged table.
- **Merged-level actions** — read-only actions (logs, describe, YAML, events) always work. Mutating actions are limited to deleting a Pod, restarting a workload (Deployment, StatefulSet, DaemonSet), and port-forwarding (Pod, Service, Deployment, StatefulSet, DaemonSet). Drill into a resource to do anything else.
- **Dashboards** — the Cluster and Monitoring dashboards list the member clusters instead of aggregating. Open a member to see its dashboard.
- **Can-I RBAC browser** — `U` checks permissions across every member cluster at once.
- **Bookmarks and pinned groups** — available for named union sets. Ad-hoc `--union-context` sessions have no saved name, so they cannot pin CRD groups or use context-aware bookmarks.
- **Sessions** — union view is always started explicitly. lfk never reopens it automatically on the next launch.

## Limitations

- **Differing CRDs** — if a resource type exists in one cluster but not another, the missing cluster contributes no rows. This is not an error.
- **`:ctx`, shell, and kubectl** — `:ctx` is disabled for the whole union session. Shell and `:kubectl` / `:k` commands are blocked in the merged table and work again once you drill into a single cluster.
- **Reverse Who-Can and orphan scans** — run one cluster at a time. Drill in to use them.
- **Drill-down is single-cluster** — owned-resource and container views below a selected resource show only that resource's cluster. There is no merged owned-resources view.
