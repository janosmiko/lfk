# Configuration Reference

The configuration file is located at `~/.config/lfk/config.yaml`. All fields are optional — only the values you specify will override the defaults.

## Top-Level Fields

| Field | Type | Default | Description |
|---|---|---|---|
| `colorscheme` | string | `"tokyonight"` | Built-in color scheme name. Custom `theme` overrides are applied on top. |
| `icons` | string | `"unicode"` | Icon display mode: `"unicode"`, `"simple"` (ASCII), `"emoji"`, or `"none"`. |
| `log_path` | string | `"~/.local/share/lfk/lfk.log"` | Path to the application log file. |
| `dashboard` | bool | `true` | Show cluster overview dashboard when entering a context. Set to `false` to go directly to resource types. |
| `resource_columns` | map[string]list | `{}` | Per-resource-type column configuration. Keys are resource Kind names (case-insensitive). When not set for a kind, columns are auto-detected. |
| `theme` | object | *(see Theme section)* | Custom color theme overrides. |
| `keybindings` | object | *(see Keybindings section)* | Custom keybinding overrides for direct actions. |
| `abbreviations` | map[string]string | *(see Abbreviations section)* | Custom search abbreviation overrides/extensions. |
| `custom_actions` | map[string]list | `{}` | User-defined actions per resource type. |
| `filter_presets` | map[string]list | `{}` | User-defined quick filter presets per resource type. |
| `terminal` | string | `"pty"` | How exec/shell commands run: `"pty"` (embedded in TUI) or `"exec"` (takes over terminal). |
| `pinned_groups` | list[string] | `[]` | CRD API groups to pin after built-in categories. Also manageable in-app with `p` key (stored per-context in `~/.local/state/lfk/pinned.yaml`). |

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

Only direct-action keybindings can be overridden. Navigation, views, tabs, and other keybindings are fixed.

| Field | Default | Action |
|---|---|---|
| `logs` | `L` | View logs for selected resource |
| `refresh` | `R` | Refresh current view |
| `restart` | `r` | Restart resource (deployments, statefulsets, daemonsets) |
| `exec` | `s` | Exec into container |
| `describe` | `v` | Describe selected resource |
| `delete` | `D` | Delete selected resource (with confirmation) |
| `force_delete` | `X` | Force destroy (remove finalizers + force delete) |
| `scale` | `S` | Scale resource (deployments, statefulsets, daemonsets) |

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

### Available Built-in Schemes

| Dark | Light |
|---|---|
| `tokyonight` (default) | `tokyonight-day` |
| `tokyonight-storm` | `bluloco-light` |
| `kanagawa-wave` | `gruvbox-light` |
| `kanagawa-dragon` | `catppuccin-latte` |
| `bluloco-dark` | |
| `nord` | |
| `gruvbox-dark` | |
| `dracula` | |
| `catppuccin-mocha` | |
| `catppuccin-macchiato` | |
| `catppuccin-frappe` | |

Switch themes at runtime with `T`. Runtime changes are not persisted — set `colorscheme` in your config to make it permanent.

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
