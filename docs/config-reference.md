# Configuration Reference

The configuration file is located at `~/.config/lfk/config.yaml`. All fields are optional — only the values you specify will override the defaults.

## Top-Level Fields

| Field | Type | Default | Description |
|---|---|---|---|
| `colorscheme` | string | `"tokyonight"` | Built-in color scheme name (460+ available). Press `T` in-app to browse. Custom `theme` overrides are applied on top. |
| `transparent_background` | bool | `false` | Use the terminal's own background for bars. Selection highlights remain opaque. |
| `icons` | string | `"auto"` | Icon display mode. One of: `"auto"` (detects Nerd Font terminals; default), `"unicode"`, `"nerdfont"` (Material Design Icons; requires Nerd Font in terminal), `"simple"` (ASCII labels), `"emoji"`, or `"none"`. Unknown values fall back to `"unicode"`. Can be overridden at runtime by the `LFK_ICONS` environment variable. |
| `log_path` | string | `"~/.local/share/lfk/lfk.log"` | Path to the application log file. |
| `dashboard` | bool | `true` | Show cluster dashboard when entering a context. Set to `false` to go directly to resource types. |
| `monitoring` | map[string]object | `{}` | Per-cluster monitoring endpoint configuration. Keys are context names or `"default"`. See [Monitoring](#monitoring) section. |
| `security` | map[string]object | *(see default)* | Per-cluster security findings browser configuration. Keys are context names or `"default"`. See [Security](#security) section. |
| `resource_columns` | map[string]list | `{}` | Per-resource-type column configuration. Keys are resource Kind names (case-insensitive). When not set for a kind, columns are auto-detected. |
| `clusters` | map[string]object | `{}` | Per-cluster configuration overrides. Keys are context names. See [Clusters](#clusters) section. |
| `theme` | object | *(see Theme section)* | Custom color theme overrides. |
| `keybindings` | object | *(see Keybindings section)* | Custom keybinding overrides for direct actions. |
| `abbreviations` | map[string]string | *(see Abbreviations section)* | Custom search abbreviation overrides/extensions. |
| `custom_actions` | map[string]list | `{}` | User-defined actions per resource type. |
| `filter_presets` | map[string]list | `{}` | User-defined quick filter presets per resource type. |
| `terminal` | string | `"pty"` | How exec/shell commands run: `"pty"` (embedded in TUI) or `"exec"` (takes over terminal). |
| `pinned_groups` | list[string] | `[]` | CRD API groups to pin after built-in categories. Also manageable in-app with `p` key (stored per-context in `~/.local/state/lfk/pinned.yaml`). |
| `tips` | bool | `true` | Show a random tip in the status bar on startup. Set to `false` to disable. |
| `log_tail_lines` | int | `1000` | Number of log lines to load initially via `--tail`. Scrolling to the top loads older history. |
| `confirm_on_exit` | bool | `true` | Show quit confirmation when pressing `ctrl+c` on the last tab. Set to `false` to exit immediately. |
| `scrolloff` | int | `5` | Number of lines to keep visible above/below the cursor when scrolling. Used by all views with cursor-based navigation. |
| `mouse` | bool | `true` | Capture mouse input for click navigation, scroll, and tab switching. Set to `false` to allow native terminal text selection. Also available as `--no-mouse` CLI flag. |

### Icon mode auto-detection

When `icons: auto` (the default), lfk inspects your environment at startup to
choose between `nerdfont` and `unicode`:

1. `LFK_ICONS` environment variable wins if set to any valid mode value.
2. `TERM` is checked for substrings `ghostty`, `kitty`, `wezterm` (works with
   tmux-through-ghostty setups where `TERM=xterm-ghostty` but
   `TERM_PROGRAM=tmux`).
3. `TERM_PROGRAM` is checked for `ghostty`, `WezTerm`, `kitty`.
4. Falls back to `unicode` otherwise.

If detection guesses wrong for your setup, set `icons: nerdfont` (or the mode
you want) in your config, or export `LFK_ICONS=nerdfont` in your shell.

## Monitoring

Configure Prometheus and Alertmanager endpoints for the monitoring dashboard (`@` key) and alert views. By default, lfk auto-discovers monitoring services by trying common service names across common namespaces. Use this section to override the endpoints per cluster or set a default for all clusters.

Keys are kubeconfig context names. The special key `"default"` applies to any cluster without an explicit entry.

```yaml
monitoring:
  # Override for a specific cluster context:
  my-prod-cluster:
    prometheus:
      namespaces: ["monitoring"]
      services: ["thanos-query"]
      port: "9090"
    alertmanager:
      namespaces: ["monitoring"]
      services: ["alertmanager-main"]
      port: "9093"
    node_metrics: prometheus   # "prometheus" or "metrics-api" (default: auto-detect)
  # Default for all other clusters:
  default:
    prometheus:
      namespaces: ["monitoring", "observability"]
      services: ["prometheus-server", "prometheus"]
```

### Monitoring Entry Fields

Each monitoring entry accepts the following top-level fields:

| Field | Type | Default | Description |
|---|---|---|---|
| `prometheus` | object | *(auto-discovery)* | Prometheus endpoint configuration. See endpoint fields below. |
| `alertmanager` | object | *(auto-discovery)* | Alertmanager endpoint configuration. See endpoint fields below. |
| `node_metrics` | string | *(auto-detect)* | Node metrics source: `"prometheus"` (use Prometheus queries), `"metrics-api"` (use metrics.k8s.io API). When empty, uses Prometheus if a prometheus endpoint is configured, otherwise falls back to metrics-api. |

### Monitoring Endpoint Fields

Each endpoint (`prometheus` and `alertmanager`) accepts:

| Field | Type | Default | Description |
|---|---|---|---|
| `namespaces` | list[string] | *(auto-discovery)* | Namespaces to search for the service. Tried in order. |
| `services` | list[string] | *(auto-discovery)* | Service names to try. Tried in order within each namespace. |
| `port` | string | `"9090"` / `"9093"` | Service port (default: `9090` for Prometheus, `9093` for Alertmanager). |

When not configured, the following defaults are used for auto-discovery:

| Component | Default Namespaces | Default Services |
|---|---|---|
| Prometheus | `monitoring`, `prometheus`, `observability`, `kube-prometheus-stack` | `prometheus-kube-prometheus-prometheus`, `prometheus-server`, `prometheus`, `prometheus-operated` |
| Alertmanager | `monitoring`, `prometheus`, `observability`, `kube-prometheus-stack` | `alertmanager-operated`, `alertmanager`, `prometheus-kube-prometheus-alertmanager`, `alertmanager-main` |

## Security

Configure the security findings browser and SEC column. All fields are
optional with sensible defaults. Keys are kubeconfig context names; the
special key `"default"` applies to any cluster without an explicit entry.

```yaml
security:
  default:
    enabled: true
    sec_column: true
    sources:
      heuristic:
        enabled: true
      trivy_operator:
        enabled: true
      policy_report:
        enabled: true
      falco:
        enabled: true
      kube_bench:
        enabled: false
```

Per-cluster overrides work the same way as the monitoring section:

```yaml
security:
  my-prod-cluster:
    sources:
      falco:
        enabled: false
  default:
    enabled: true
    sec_column: true
    sources:
      heuristic:
        enabled: true
      trivy_operator:
        enabled: true
      policy_report:
        enabled: true
      falco:
        enabled: true
      kube_bench:
        enabled: false
```

### Fields

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `true` | Master switch for the entire security feature |
| `sec_column` | bool | `true` | Show the SEC badge in the Workloads explorer table |
| `sources` | map | *(see default)* | Per-source configuration |

### Source fields

Each source entry under `sources:` accepts:

| Field | Type | Description |
|---|---|---|
| `enabled` | bool | Opt-out toggle for the source |
| `checks` | list[string] | (heuristic only) List of checks to run. Omit or set to `null` to run all built-in checks. |

Available heuristic checks: `privileged`, `host_namespaces`, `host_path`,
`readonly_root_fs`, `run_as_root`, `allow_priv_esc`, `dangerous_caps`,
`missing_resource_limits`, `default_sa`, `latest_tag`.

### Source requirements

| Source | Default | Requirements |
|---|---|---|
| `heuristic` | enabled | No external dependencies. Built-in pod security checks run against the Kubernetes API directly. |
| `trivy_operator` | enabled | Requires [Trivy Operator](https://aquasecurity.github.io/trivy-operator/) installed in the cluster. Reads `VulnerabilityReport` and `ConfigAuditReport` CRDs from the `aquasecurity.github.io/v1alpha1` API group. Automatically disabled when those CRDs are not present. |
| `policy_report` | enabled | Requires [Kyverno](https://kyverno.io/) or any implementation of the [Policy Reports API](https://github.com/kubernetes-sigs/wg-policy-prototypes). Reads `PolicyReport` and `ClusterPolicyReport` CRDs from the `wgpolicyk8s.io/v1alpha2` API group. Automatically disabled when those CRDs are not present. |
| `falco` | enabled | Requires [Falco](https://falco.org/) DaemonSet installed. For findings to appear, falcosidekick must be configured with Kubernetes output enabled (`config.kubernetes.enabled: true` in the falcosidekick Helm values). Automatically disabled when Falco is not detected. |
| `kube_bench` | disabled | Not yet implemented (placeholder). |

## Clusters

Per-cluster configuration overrides allow you to customize settings for individual kubeconfig contexts. Keys are context names.

Currently supported per-cluster overrides:

| Field | Type | Description |
|---|---|---|
| `resource_columns` | map[string]list | Per-resource-type column overrides for this cluster. Same format as the global `resource_columns`. |

Per-cluster `resource_columns` take precedence over the global `resource_columns` setting.

```yaml
clusters:
  my-prod-cluster:
    resource_columns:
      Pod: ["IP", "Node", "Image", "CPU", "MEM"]
      Deployment: ["Replicas", "Available"]
  my-staging-cluster:
    resource_columns:
      Pod: ["IP", "Node", "Image"]
```

## Theme

All theme colors accept CSS hex color codes (e.g., `"#7aa2f7"`). Only specify the colors you want to change; unspecified colors keep their defaults from the selected colorscheme.

| Field | Default (Tokyonight) | Description |
|---|---|---|
| `primary` | `#7aa2f7` | Borders, headers, breadcrumbs, active column highlight |
| `secondary` | `#9ece6a` | Help keys, running/success status, selection markers |
| `text` | `#c0caf5` | Normal text color |
| `selected_fg` | `#1a1b26` | Foreground of selected/highlighted items |
| `selected_bg` | `#7aa2f7` | Background of selected/highlighted items |
| `border` | `#3b4261` | Inactive column borders |
| `dimmed` | `#565f89` | Placeholder text, help descriptions, dimmed items |
| `error` | `#f7768e` | Error messages, failed status, delete confirmations |
| `warning` | `#e0af68` | Warning messages, pending status, namespace indicator |
| `purple` | `#bb9af7` | Special values |
| `base` | `#1a1b26` | Dark background base |
| `bar_bg` | `#24283b` | Title and status bar backgrounds |
| `surface` | `#1f2335` | Surface color for overlays |

## Keybindings

All keybindings can be overridden. Only specify the keys you want to change -- defaults apply for everything else. See [`keybindings.md`](keybindings.md) for the full list.

| Field | Default | Action |
|---|---|---|
| `logs` | `L` | View logs for selected resource |
| `refresh` | `R` | Refresh current view |
| `restart` | `r` | Restart resource (action menu only) |
| `exec` | `s` | Exec into container (action menu only) |
| `describe` | `v` | Describe selected resource |
| `delete` | `D` | Delete resource (force delete Pod/Job if already deleting) |
| `force_delete` | `X` | Force delete with --grace-period=0 (Pod/Job only) |
| `scale` | `S` | Scale resource (deployments, statefulsets, daemonsets) |
| `edit` | `E` | Edit selected resource in $EDITOR |
| `label_editor` | `i` | Edit labels/annotations |
| `secret_editor` | `e` | Secret/ConfigMap editor |
| `column_toggle` | `,` | Column visibility toggle |
| `sort_next` | `>` | Sort by next column |
| `sort_prev` | `<` | Sort by previous column |
| `sort_flip` | `=` | Toggle sort direction |
| `sort_reset` | `-` | Reset sort to default |
| `error_log` | `!` | Error log overlay |
| `finalizer_search` | `ctrl+g` | Finalizer search and remove |
| `terminal_toggle` | `ctrl+t` | Toggle terminal mode (pty/exec) |
| `toggle_rare` | `H` | Toggle rarely used resource types in the sidebar |

## Resource Columns

Use `resource_columns` to configure which columns are displayed for specific resource types. When not configured for a kind, columns are auto-detected from the resource data.

Available column keys depend on the resource type. Common examples:

| Column Key | Shown For | Description |
|---|---|---|
| `IP` | Pods, Services | IP address |
| `Node` | Pods | Node the pod is running on |
| `Image` | Pods | Container image |
| `QoS` | Pods | Quality of Service class |
| `Replicas` | Deployments, StatefulSets | Replica count |
| `Available` | Deployments | Available replicas |
| `CPU` | Pods, Deployments | CPU usage |
| `MEM` | Pods, Deployments | Memory usage |
| `CPU/R` | Pods | CPU request percentage |
| `CPU/L` | Pods | CPU limit percentage |
| `MEM/R` | Pods | Memory request percentage |
| `MEM/L` | Pods | Memory limit percentage |

Use `["*"]` to show all available columns for a given resource type.

```yaml
resource_columns:
  Pod: ["IP", "Node", "Image", "CPU", "MEM"]
  Deployment: ["Replicas", "Available"]
  Service: ["*"]                  # Show all for Services
  StatefulSet: ["Replicas"]
```

Keys are resource Kind names and are case-insensitive (e.g., `Pod`, `pod`, `POD` all work).

## Abbreviations

Search abbreviations allow typing short names to jump to resource types. The default abbreviations include all standard kubectl short names.

To override or extend:

```yaml
abbreviations:
  po: pod
  dep: deployment
  deploy: deployment
  svc: service
  ing: ingress
  cm: configmap
  sec: secret
  ns: namespace
  no: node
  pvc: persistentvolumeclaim
  hpa: horizontalpodautoscaler
  # Add your own:
  myapp: myapplication.example.com
```

### Default Abbreviations

| Abbreviation | Resource Type |
|---|---|
| `po` | pod |
| `dp` | deployment |
| `dep` | deployment |
| `deploy` | deployment |
| `rs` | replicaset |
| `sts` | statefulset |
| `ds` | daemonset |
| `svc` | service |
| `ing` | ingress |
| `cm` | configmap |
| `sec` | secret |
| `ns` | namespace |
| `no` | node |
| `pvc` | persistentvolumeclaim |
| `pv` | persistentvolume |
| `sc` | storageclass |
| `sa` | serviceaccount |
| `hpa` | horizontalpodautoscaler |
| `vpa` | verticalpodautoscaler |
| `cj` | cronjob |
| `job` | job |
| `crd` | customresourcedefinition |
| `ev` | event |
| `ep` | endpoint |
| `eps` | endpointslice |
| `rb` | rolebinding |
| `crb` | clusterrolebinding |
| `cr` | clusterrole |
| `role` | role |
| `limit` | limitrange |
| `quota` | resourcequota |
| `pdb` | poddisruptionbudget |
| `netpol` | networkpolicy |
| `rc` | replicationcontroller |

## Custom Actions

Define custom shell commands for specific resource types. Custom actions appear in the action menu (`x` key) after the built-in actions.

```yaml
custom_actions:
  Pod:
    - label: "SSH to Node"
      command: "ssh {Node}"
      key: "S"
      description: "SSH into the pod's node"
    - label: "Copy Logs"
      command: "kubectl logs {name} -n {namespace} --context {context} > /tmp/{name}.log"
      key: "C"
      description: "Copy pod logs to /tmp"
  Deployment:
    - label: "Image History"
      command: "kubectl rollout history deployment/{name} -n {namespace} --context {context}"
      key: "H"
      description: "Show rollout history"
```

### Custom Action Fields

| Field | Required | Description |
|---|---|---|
| `label` | Yes | Display name in the action menu |
| `command` | Yes | Shell command to execute (supports template variables) |
| `key` | Yes | Shortcut key in the action menu |
| `description` | No | Description shown next to the action |

### Template Variables

Template variables are substituted before execution:

| Variable | Description |
|---|---|
| `{name}` | Resource name |
| `{namespace}` | Resource namespace |
| `{context}` | Kubeconfig context name |
| `{kind}` | Resource kind (e.g., "Pod", "Deployment") |
| `{<ColumnKey>}` | Any column value from the resource (e.g., `{Node}`, `{IP}`) — case-insensitive |

Custom action commands are executed via `sh -c` with `KUBECONFIG` set in the environment. Interactive commands (like `ssh`) hand over the terminal to the subprocess.

## Pinned Groups

Pin CRD API groups so they appear right after the built-in categories (Workloads, Networking, etc.) instead of being sorted alphabetically under Custom Resources.

```yaml
pinned_groups:
  - karpenter.sh
  - monitoring.coreos.com
  - argoproj.io
```

Pinned groups can also be managed interactively: press `p` at the resource types level to pin/unpin the selected CRD group. In-app pins are stored per-context in `~/.local/state/lfk/pinned.yaml` and are merged with the config file pins at runtime.

## Filter Presets

Define custom quick filter presets per resource type. These appear alongside the built-in presets when you press `.` at the resource level.

```yaml
filter_presets:
  Pod:
    - name: "On GPU Nodes"
      key: "g"
      match:
        column: "Node"
        column_value: "gpu"
  Deployment:
    - name: "Single Replica"
      key: "1"
      match:
        column: "Replicas"
        column_value: "1"
```

### Filter Preset Fields

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Display name in the filter overlay |
| `key` | Yes | Single-character shortcut key (must not conflict with built-in presets) |
| `match` | Yes | Filter criteria object (all fields are AND-ed) |

### Match Criteria

| Field | Description |
|---|---|
| `status` | Substring match against the resource status (case-insensitive) |
| `ready_not` | `true` to match resources where ready count != desired (e.g., "1/3") |
| `restarts_gt` | Match pods with restart count greater than this number |
| `column` | Column key to check (case-insensitive, e.g., "Node", "IP") |
| `column_value` | Substring match against the column value (case-insensitive) |

## Terminal Mode

Controls how exec and shell commands are executed.

```yaml
terminal: pty
```

| Value | Description |
|---|---|
| `pty` (default) | Embedded in the TUI — command output appears inside the application |
| `exec` | Takes over the terminal — the TUI suspends and the command runs in the foreground |

## Color Schemes

Over 460 built-in color schemes are available, generated from [ghostty terminal themes](https://github.com/ghostty-org/ghostty). Popular schemes include:

`tokyonight` (default), `dracula`, `nord`, `catppuccin-mocha`, `catppuccin-latte`, `rose-pine`, `gruvbox-dark`, `gruvbox-light`, `everforest-dark`, `one-half-dark`, `ayu-dark`, `nightfox`, `monokai-pro`, `github-dark`, `github-light`, `solarized-dark`, `solarized-light`, and many more.

Switch themes at runtime with `T` to browse all available themes interactively with live preview. Runtime changes are not persisted — set `colorscheme` in your config to make it permanent.

To regenerate themes from the latest ghostty source, run `make generate-themes`.

## Session Persistence

The application automatically saves and restores the last visited context, namespace, and resource type across restarts. The session state is stored at `~/.local/state/lfk/session.yaml`.

This is transparent and requires no configuration. To start fresh, delete the session file.

## Files

The application follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/):

| File | Description |
|---|---|
| `$XDG_CONFIG_HOME/lfk/config.yaml` | Main configuration file (default: `~/.config/lfk/config.yaml`) |
| `$XDG_STATE_HOME/lfk/bookmarks.yaml` | Saved bookmarks (default: `~/.local/state/lfk/bookmarks.yaml`) |
| `$XDG_STATE_HOME/lfk/session.yaml` | Last session state, auto-managed (default: `~/.local/state/lfk/session.yaml`) |
| `$XDG_STATE_HOME/lfk/pinned.yaml` | Per-context pinned CRD groups, managed via `p` key (default: `~/.local/state/lfk/pinned.yaml`) |
| `~/.local/share/lfk/lfk.log` (default) | Application log file (configurable via `log_path`) |

State files stored at the legacy `~/.config/lfk/` location are automatically migrated to the new XDG state directory on first access.
