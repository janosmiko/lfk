# Feature details

Detail for the README feature list. Features documented elsewhere link straight to their own page.

## List status summary

Hovering a resource type in the resource-type list pins a summary band at the bottom of the children pane, like the resource-usage footer. It always shows the resource count.

Kinds with a health signal add a colored status rollup: ArgoCD Application health and sync, Pod phase, workload ready ratios, Node readiness, Namespace / PV / PVC phase, and Flux and cert-manager Ready. Any other kind that surfaces a `.status.phase` or `.status.conditions` gets a generic rollup, so you can confirm a whole list is healthy without drilling in.

## Schema side pane

`Ctrl+K` opens a bordered pane beside the YAML viewer and the Object Explorer with the cluster's own description of the field under the cursor. It follows the cursor, reads the connected cluster so CRDs resolve like built-in kinds, and caches what it reads.

## Status colors

Running is green, Pending yellow, Failed red. The same rule applies to CRD printer-column values: recognized status words (Active, Failed, Pending, ...) take their severity color, and True/False values follow the column's polarity, so `Established=False` is red and `Failed=False` is green.

## Resource usage metrics

The cluster dashboard shows CPU and memory as color-coded bars.

## Command bar autocomplete

The `:` bar drops a vertical suggestion list under the input. Value positions (namespace, context, resource name, option, column, format) accept fuzzy matches. Command names match on prefix only. kubectl commands complete flags and namespaces.

## Resource templates

`a` opens the create-from-template picker with more than 25 built-in templates plus your own. A Custom Resource template is included as a starting point for CRDs. See [config-reference.md](config-reference.md#resource-templates) for the file rules and [keybindings.md](keybindings.md#action-menu-items) for the picker keys.

## Export as template

`x` -> `T` strips a live object down to a manifest you can apply. On top of the server-set fields listed in [keybindings.md](keybindings.md#action-menu-items), the export also drops `nodeName`, `clusterIP`, and the injected service-account token volume.

## Argo CD

Browse Applications, sync, terminate a running sync, and refresh. With rows multi-selected, `x` also offers bulk sync and bulk refresh. Per-Application keys are in [keybindings.md](keybindings.md#argocd-application-actions).

## Argo Workflows

Suspend and resume Workflows, stop or terminate them, and resubmit them. Submit a new Workflow from a WorkflowTemplate. Suspend and resume CronWorkflows.

## KEDA

Pause and unpause ScaledObjects and ScaledJobs.

## External Secrets

Force a refresh of ExternalSecrets, ClusterExternalSecrets, and PushSecrets.

## Right-sizing advisor

`x` -> `z` suggests CPU and memory requests and limits per container. The suggestion is only as good as the strategy behind it, shown in the `Strategy:` chip.

| Strategy | Source | Use it for |
|---|---|---|
| `snapshot` | One metrics-server reading over the window shown in the header (usually 10-30s) | A quick look. Two runs minutes apart can disagree by 10x on a bursty container. |
| `1d-max`, `1d-avg`, `7d-p95` | Prometheus range queries | Sizing decisions. `7d-p95` is the safest default for requests. |
| `vpa` | VerticalPodAutoscaler recommender | Sizing decisions when a VPA already targets the workload. |

If the Prometheus query fails, the overlay prints the error above the table and the `SUGGESTION` column stays empty. The `USAGE` column still comes from metrics-server.

The chip shows `snapshot` with no `[N/M]` counter when it is the only strategy available. To unlock the others, point lfk at Prometheus or VictoriaMetrics (see [config-reference.md](config-reference.md#monitoring)) or create a VPA in `Off` mode for the workload. Then cycle with `[` and `]`. Keys and headroom are in [keybindings.md](keybindings.md#right-sizing-advisor).

## Embedded terminal

Exec and shell sessions run in an embedded PTY by default. A session keeps running in the background when you switch tabs, so you can leave it and come back. `Ctrl+T` cycles the mode, see [config-reference.md](config-reference.md#terminal-mode).
