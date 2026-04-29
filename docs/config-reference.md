# Configuration Reference

The configuration file is located at `~/.config/lfk/config.yaml`. All fields are optional — only the values you specify will override the defaults.

## Top-Level Fields

| Field | Type | Default | Description |
|---|---|---|---|
| `colorscheme` | string | `"tokyonight"` | Built-in color scheme name (460+ available). Press `T` to browse. Supports dual-mode syntax for auto dark/light switching: `"dark:X,light:Y"`. Custom `theme` overrides are applied on top. |
| `transparent_background` | bool | `false` | Use the terminal's own background for bars. Selection highlights remain opaque. |
| `icons` | string | `"auto"` | Icon display mode. One of: `"auto"` (detects Nerd Font terminals; default), `"unicode"`, `"nerdfont"` (Material Design Icons; requires Nerd Font in terminal), `"simple"` (ASCII labels), `"emoji"`, or `"none"`. Unknown values fall back to `"unicode"`. Can be overridden at runtime by the `LFK_ICONS` environment variable. |
| `log_path` | string | `"~/.local/share/lfk/lfk.log"` | Path to the application log file. |
| `dashboard` | bool | `true` | Show cluster dashboard when entering a context. Set to `false` to go directly to resource types. |
| `monitoring` | map[string]object | `{}` | Per-cluster monitoring endpoint configuration. Keys are context names or `"_global"`. See [Monitoring](#monitoring) section. |
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
| `log_tail_lines_short` | int | `10` | Number of log lines loaded by the action menu "Tail Logs" entry (`l` key). Intended for lightweight peeks without the full history hit. Non-positive values are ignored. |
| `log_render_ansi` | bool | `true` | Render ANSI SGR sequences (colour, bold, underline) emitted by log producers. Set to `false` to strip them — the viewer replaces every ESC byte with `U+FFFD`, matching the original safe-but-noisy behaviour. Non-SGR CSI sequences (cursor movement, screen erase) are always stripped regardless. Toggle at runtime with `:set ansi` / `:set noansi`. |
| `confirm_on_exit` | bool | `true` | Show quit confirmation when pressing `ctrl+c` on the last tab. Set to `false` to exit immediately. |
| `scrolloff` | int | `5` | Number of lines to keep visible above/below the cursor when scrolling. Used by all views with cursor-based navigation. |
| `mouse` | bool | `true` | Capture mouse input for click navigation, scroll, and tab switching. Set to `false` to allow native terminal text selection. Also available as `--no-mouse` CLI flag. |
| `read_only` | bool | `false` | Disable all mutating actions (delete, edit, scale, restart, exec, port-forward, drain, cordon, etc.) globally. Per-context overrides under `clusters.<name>.read_only` and the `--read-only` CLI flag take precedence. See [Read-Only Mode](usage.md#read-only-mode). |
| `clusters.<name>.read_only` | bool | _(unset)_ | Per-context read-only override. Wins over the global `read_only` so you can mark specific clusters (e.g. `prod`) read-only while leaving others mutable. |
| `secret_lazy_loading` | bool | `false` | When `true`, Secret listing fetches metadata only and decoded values load on hover. Much faster in clusters with many Helm release secrets (each release is a multi-hundred-KB Secret) or large TLS payloads, at the cost of an extra GET per hovered Secret. When `false` (default), Secrets list like every other resource type — full objects are pulled and `data` is eagerly decoded, so the Type column and decoded values are visible immediately. See [Secret lazy loading](#secret-lazy-loading) for trade-offs. |
| `min_contrast_ratio` | float | `0.0` | Normalized readability knob in `[0.0, 1.0]`. When above zero, foreground colors are nudged in HSL lightness space to meet a minimum WCAG contrast ratio against their paired background. See [Minimum Contrast Ratio](#minimum-contrast-ratio). |

### Auto dark/light mode

When `colorscheme` uses the `dark:X,light:Y` dual-mode syntax (either segment
may be omitted), lfk subscribes to your terminal's operating system
color-scheme preference via the standard CSI 2031/996 protocol:

- On startup lfk sends `CSI ?2031h` (subscribe to notifications) and
  `CSI ?996n` (request current preference).
- The terminal responds with `CSI ?997;1n` (dark) or `CSI ?997;2n` (light).
- Whenever the OS appearance changes, the terminal sends another notification
  and lfk switches to the configured scheme in real time.

**Supported terminals**: Ghostty, kitty ≥ 0.27, Contour, WezTerm (recent
nightly builds). Other terminals silently ignore the sequences.

```yaml
# Example: dark → Catppuccin Mocha, light → Catppuccin Latte
colorscheme: "dark:catppuccin-mocha,light:catppuccin-latte"

# Spaces in scheme names are fine — they are normalised to hyphens internally:
colorscheme: "dark:Rose Pine,light:Rose Pine Dawn"
```

When only one side is configured the other side performs no automatic switch.
Plain (non-dual) `colorscheme` values continue to work as before and disable
automatic dark/light switching entirely.

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

Keys are kubeconfig context names. The special key `"_global"` applies to any cluster without an explicit entry.

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
  # Global fallback for all other clusters:
  _global:
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
| `terminal_toggle` | `ctrl+t` | Cycle terminal mode (pty/exec/mux) |
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
      read_only_safe: true   # safe to run when read-only mode is on
  Deployment:
    - label: "Image History"
      command: "kubectl rollout history deployment/{name} -n {namespace} --context {context}"
      key: "H"
      description: "Show rollout history"
      read_only_safe: true
```

### Custom Action Fields

| Field | Required | Description |
|---|---|---|
| `label` | Yes | Display name in the action menu |
| `command` | Yes | Shell command to execute (supports template variables) |
| `key` | Yes | Shortcut key in the action menu |
| `description` | No | Description shown next to the action |
| `read_only_safe` | No | When `true`, the action remains available in read-only mode. Defaults to `false` (treated as mutating and blocked) so destructive shell commands don't silently bypass the read-only gate. Set to `true` for view-only commands like `kubectl describe`, `kubectl logs`, `kubectl rollout history`. |

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

## Secret lazy loading

By default, `Secret` resources list like every other resource type: the full
objects are pulled from the API server and their `data` map is eagerly
base64-decoded into each row. This keeps the Type column and decoded values
visible immediately, at the cost of potentially heavy list responses —
Helm v3 stores every release (and every revision) as a Secret of
`type=helm.sh/release.v1`, typically 100 KB to 1 MB per release, so clusters
with many Helm apps can push tens of megabytes on a single Secrets list.

Set `secret_lazy_loading: true` to opt into a different strategy:

- The list fetches **metadata only** via the Kubernetes
  `PartialObjectMetadataList` API (name, namespace, age, owner references,
  deletion timestamp). No `data` payload crosses the wire for the list.
- The right-hand detail pane lazily fetches each Secret's decoded values on
  **hover** (one per-item GET, cached until the secret is edited or the
  list refreshes). Masked-bullet rendering is unchanged — toggle visibility
  with the secret-toggle key as usual.

### Trade-offs

| | `false` (default) | `true` |
|---|---|---|
| List payload for 100 Helm releases | ~50–100 MB (full objects) | Kilobytes (metadata only) |
| Type column visible in list | Yes | **No** — dropped; the metadata API doesn't return it |
| Decoded values on first hover | Instant (already on the item) | Brief delay until per-item GET returns, then cached |
| API calls per hover at `LevelResources` | 0 extra | 1 per distinct Secret (cached thereafter) |

Turn this on if you notice hovering **Secrets** in the left pane is
noticeably slower than hovering other resource types — that's the symptom
this option is designed to fix.

```yaml
secret_lazy_loading: true
```

## Minimum Contrast Ratio

`min_contrast_ratio` is a normalized knob in `[0.0, 1.0]` that makes lfk
automatically adjust foreground colors to be more readable against their
backgrounds. It is useful when a built-in theme happens to produce low-contrast
text on your particular terminal or display.

### WCAG mapping

The value is mapped linearly to a [WCAG 2.1](https://www.w3.org/TR/WCAG21/#contrast-minimum)
contrast ratio target:

```
wcagTarget = 1.0 + value * 20.0
```

| Config value | WCAG ratio | Standard |
|---|---|---|
| `0.0` (default) | — | Off; colors unchanged |
| `0.175` | 4.5:1 | WCAG AA (normal text) |
| `0.3` | 7.0:1 | WCAG AAA |
| `1.0` | 21.0:1 | Maximum (forces pure black or white fg) |

### How it works

- Only HSL **lightness** is adjusted. Hue and saturation are preserved, so
  chromatic colors keep their identity at moderate values.
- Nudge direction preserves the designer's existing fg/bg relationship: if fg
  was darker than bg the mutator nudges it further darker; if lighter, further
  lighter. This avoids flipping a dark-on-light pair past the bg toward the
  opposite extreme, which would silently collapse the contrast for
  mid-luminance backgrounds.
- The adjustment runs per color pair, so some colors may shift more than others
  depending on how far they are from the target. At `value=1.0` expect some
  colors to collapse toward achromatic white or black.
- Named colors (e.g. `red`) and malformed hex values are left unchanged rather
  than causing an error.

### Pairs adjusted

| Foreground | Background |
|---|---|
| `text` | `base` |
| `dimmed` | `base` |
| `error` | `base` |
| `warning` | `base` |
| `secondary` | `base` |
| `primary` | `base` |
| `purple` | `base` |
| `selected_fg` | `selected_bg` |

`border` is intentionally **not** in this list. It doubles as the background
for the left-pane selected-row highlight (`ParentHighlightStyle` renders
`text` on `border`), so enforcing its contrast as a foreground color would
drive it toward `text`'s luminance and collapse that highlight.

### When to use this

Turn it on when you notice text is hard to read with a particular colorscheme
on your terminal. Start with `0.175` (WCAG AA) — it is a good balance between
readability and color fidelity. Increase toward `0.3` if you still find the
text too dim.

```yaml
min_contrast_ratio: 0.175
```

## Terminal Mode

Controls how interactive shells (`exec`, `attach`, `debug`, debug pods,
node shell) run.

```yaml
terminal: pty
```

| Value | Description |
|---|---|
| `pty` (default) | Embedded in the TUI via an internal vt10x terminal — output appears inside lfk |
| `exec` | Hands the host terminal to the shell via `tea.ExecProcess`; lfk suspends until the shell exits |
| `mux` | Opens the shell in a new window (tmux) or floating pane (zellij) of the surrounding multiplexer; lfk stays foregrounded alongside it |

`Ctrl+T` cycles modes at runtime: `pty -> exec -> mux -> pty`. The `mux`
step is skipped automatically when no tmux/zellij is detected, so the
cycle becomes `pty -> exec -> pty` in that case. Setting `terminal:` in
the config picks the default at startup.

### Selecting and copying text in `pty` mode

Inside the embedded PTY view, the host terminal handles text selection.
`shift+drag` copies a region; on macOS, `shift+option+drag` (or `alt+drag`
on Linux/Windows) selects a rectangular block, which is useful when
several panes share the screen.

For lines that have already scrolled off the visible viewport, lfk keeps
an in-app scrollback ring (~5000 lines per PTY tab, ANSI-stripped). Use
`Ctrl+]` `Ctrl+U` / `Ctrl+D` to scroll by half a viewport, `Ctrl+]`
`Ctrl+B` / `Ctrl+F` for full-viewport pages, `Ctrl+]` `g` to jump to the
oldest captured line, `Ctrl+]` `G` to snap back to live. Typing a real
character also snaps to live so subsequent input goes to the visible prompt.

The scrollback ring is populated from the byte stream, not the rendered
screen, so full-screen curses programs (vim, less, htop) that paint
absolute screen positions will produce noisy history while running. If
you need precise scrollback, cycle to `exec` or `mux` mode for the next
shell — the host terminal's own scrollback then takes over.

### `mux` mode requirements

`mux` mode requires lfk to be running inside `tmux` or `zellij` and the
corresponding binary to be on `PATH`. lfk detects this via the `$TMUX` /
`$ZELLIJ` env vars. When the requirement isn't met, an interactive shell
attempt under `terminal: mux` fails fast with an error in the status bar
— either set the env, install the binary, or cycle to `exec`/`pty`.

KUBECONFIG and any other per-context env that lfk passes to subprocesses
are forwarded to the new pane via inline shell variable assignments,
since neither tmux nor zellij propagate parent env reliably across
their spawn APIs.

## PTY Scrollback

`scrollback_lines` sets the per-tab capacity of the embedded PTY
scrollback ring buffer (in lines). Only applies to `pty` mode — `exec`
and `mux` delegate scrollback to the host terminal.

```yaml
scrollback_lines: 5000
```

| Field | Default | Range | Description |
|---|---|---|---|
| `scrollback_lines` | `5000` | `[100, 100000]` | Per-tab line cap for the captured PTY scrollback. Out-of-range values are clamped and a warning is logged. |

Memory budget is roughly `scrollback_lines × average_line_length` per
PTY tab. With the default and ~120-character lines, that's around
600 KiB per tab — safe to leave at default for most users. Bump it if
you frequently `cat` large logs from inside the embedded shell and
want to scroll back through them; lower it on very memory-constrained
systems.

Navigate the captured scrollback with `Ctrl+]` `Ctrl+U`/`Ctrl+D` (half
page), `Ctrl+]` `Ctrl+B`/`Ctrl+F` (full page), `Ctrl+]` `g`/`G`
(oldest/live), or the mouse wheel (1 line per tick).

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
