# Configuration reference

The configuration file is located at `~/.config/lfk/config.yaml`. All fields are optional — only the values you specify will override the defaults.

## Editor autocompletion (JSON Schema)

A JSON Schema is published at [`docs/config.schema.json`](config.schema.json). Add this modeline to the top of your `config.yaml` to get autocompletion, inline docs, and typo warnings in any editor running a YAML language server (VS Code's [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml), Neovim with `yamlls`, etc.):

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/janosmiko/lfk/main/docs/config.schema.json
```

Prefer a local copy? Point `$schema` at a relative or absolute path instead of the URL.

## Top-level fields

| Field | Type | Default | Description |
|---|---|---|---|
| `appearance` | object | *(see Appearance section)* | Visual settings group: `colorscheme`, `icons`, `no_color`, `transparent_background`, `min_contrast_ratio`, `dim_overlay`, `row_status_tint`. Canonical home for these knobs. See [Appearance](#appearance). |
| `colorscheme` | string | `"tokyonight-storm"` | **Deprecated** — use `appearance.colorscheme`. Built-in color scheme name (460+ available). Press `T` to browse. Supports dual-mode syntax for auto dark/light switching: `"dark:X,light:Y"`. Custom `theme` overrides are applied on top. |
| `transparent_background` | bool | `false` | **Deprecated** — use `appearance.transparent_background`. Use the terminal's own background for bars. Selection highlights remain opaque. |
| `icons` | string | `"auto"` | **Deprecated** — use `appearance.icons`. Icon display mode. One of: `"auto"` (detects Nerd Font terminals; default), `"unicode"`, `"nerdfont"` (Material Design Icons; requires Nerd Font in terminal), `"simple"` (ASCII labels), `"emoji"`, or `"none"`. Unknown values fall back to `"unicode"`. Can be overridden at runtime by the `LFK_ICONS` environment variable. |
| `log_path` | string | `"~/.local/share/lfk/lfk.log"` | Path to the application log file. |
| `kubeconfig_dir` | string or list[string] | `"~/.kube/config.d"` | Directory (or list of directories) recursively scanned for kubeconfig files, in addition to `~/.kube/config` (unless `KUBECONFIG` is set). See [Kubeconfig Directory](usage.md#kubeconfig-directory). |
| `kubeconfig_exclusive` | bool | `true` | When `KUBECONFIG` is set, load only the files it lists (kubectl semantics), skipping `~/.kube/config` and the default `~/.kube/config.d` scan. Set `false` to merge everything like releases before 0.15. Overridden by `--kubeconfig-exclusive` and `LFK_KUBECONFIG_EXCLUSIVE`. |
| `dashboard` | bool | `true` | Show cluster dashboard when entering a context. Set to `false` to go directly to resource types. |
| `monitoring` | map[string]object | `{}` | Per-cluster monitoring endpoint configuration. Keys are context names or `"_global"`. See [Monitoring](#monitoring) section. |
| `views` | map[string]object | `{}` | Per-resource-type column and sort configuration. Keys are GVR (`apps/v1/deployments`) or Kind (`deployment`, case-insensitive); GVR wins when both match. Supersedes `resource_columns`. See [views](#views). |
| `resource_columns` | map[string]list | `{}` | Per-resource-type column configuration. Keys are resource Kind names (case-insensitive). **Deprecated** — use `views` instead. Bridged automatically; `views` wins when both are set for the same Kind. |
| `clusters` | map[string]object | `{}` | Per-cluster configuration overrides. Keys are context names. See [Clusters](#clusters) section. |
| `theme` | object | *(see Theme section)* | Custom color theme overrides. |
| `keybindings` | object | *(see Keybindings section)* | Custom keybinding overrides for direct actions. |
| `abbreviations` | map[string]string | *(see Abbreviations section)* | Custom search abbreviation overrides/extensions. |
| `custom_actions` | map[string]list | `{}` | User-defined actions per resource type. |
| `filter_presets` | map[string]list | `{}` | User-defined quick filter presets per resource type. |
| `goto_targets` | map | `{}` | Extra g-prefix goto chords. Key = full chord (e.g. `gA`), which must be `jump_top` plus one keypress (`gA`, `gctrl+p`); value = `{kind, group, name}`. Overrides built-ins on collision. |
| `which_key_enabled` | bool | `true` | Show the goto popup while the `g` prefix is pending, and the action panel while the which-key leader (`?`) is armed (explorer and every fullscreen viewer except exec mode). |
| `which_key_delay_ms` | int | `0` | Delay before the goto popup appears (ms, 0-2000). |
| `which_key_leader_delay_ms` | int | `0` | Delay before the which-key action panel appears (ms, 0-2000). |
| `which_key_grouped` | bool | `true` | Startup order of the action panel: `true` groups by category, `false` sorts by key. The leader key toggles it and saves the choice to `~/.local/state/lfk/whichkey_prefs.yaml`, which then outranks this setting; changing this setting afterwards retires the saved choice. |
| `terminal` | string | `"pty"` | How exec/shell commands run: `"pty"` (embedded in TUI), `"exec"` (takes over terminal), or `"mux"` (opens in a new tmux/zellij window/pane; errors out if no multiplexer is detected). |
| `pinned_groups` | list[string] | `[]` | CRD API groups to pin after built-in categories. Also manageable in-app with `p` key (stored per-context and per-union-set in `~/.local/state/lfk/pinned.yaml`). |
| `pinned_summaries` | list[string] | *(built-in defaults)* | Resource types (`group/resource` pin keys) whose status summaries show on the cluster dashboard. Unset shows built-in defaults (Jobs, Deployments, Argo Applications, Flux Kustomizations, cert-manager Certificates); `[]` disables them. Also manageable in-app via the action menu (`x`) at the resource types level (stored per-context and per-union-set in `~/.local/state/lfk/pinned_summaries.yaml`). Max 10 per scope. |
| `union_sets` | map[string]object or list[object] | `[]` | Named multi-cluster groups that the `--union-set <name>` CLI flag expands. See [Union Sets](#union-sets). |
| `tips` | bool | `true` | Show a random tip in the status bar on startup. Set to `false` to disable. |
| `log_viewer` | object | *(see Log Viewer section)* | Log viewer settings: tail sizes, ANSI rendering, and startup-toggle defaults for preview/prefixes/timestamps. See [Log Viewer](#log-viewer). Note: `log_path` (above) is the application's own log file, not a viewer setting. |
| `yaml_viewer` | object | *(see Viewer Defaults)* | YAML viewer startup-toggle defaults. See [Viewer Defaults](#viewer-defaults). |
| `diff_viewer` | object | *(see Viewer Defaults)* | Diff viewer startup-toggle defaults. See [Viewer Defaults](#viewer-defaults). |
| `describe_viewer` | object | *(see Viewer Defaults)* | Describe viewer startup-toggle defaults. See [Viewer Defaults](#viewer-defaults). |
| `object_explorer` | object | *(see Viewer Defaults)* | Object Explorer startup-toggle defaults. See [Viewer Defaults](#viewer-defaults). |
| `api_explorer` | object | *(see Viewer Defaults)* | API Explorer startup-toggle defaults. See [Viewer Defaults](#viewer-defaults). |
| `split_preview` | bool | `true` | Startup default for the split preview pane. Set to `false` to start with it hidden. |
| `watch_mode` | bool | `true` | Startup default for live watch/polling. Set to `false` to start with manual refresh. |
| `watch_interval` | string | `2s` | Polling interval in watch mode. Go duration string, clamped to [500ms, 10m]. Override with `--watch-interval`. |
| `background_watch_interval` | string | `10s` | Watch cadence while the window is unfocused or focused-idle. Same form and [500ms, 10m] clamp as `watch_interval`. Set equal to `watch_interval` to disable background throttling. |
| `foreground_idle_timeout` | string | `120s` | No-input window before a focused window throttles to `background_watch_interval`. Go duration; `0` disables focused-idle throttling (background throttling still applies). Clamped to [0, 10m]. |
| `metrics_interval` | string | `15s` | Minimum gap between two list-wide CPU/MEM fetches on a watch-tick refresh. Go duration, clamped to [2s, 10m]; `0` fetches on every tick. Manual refresh always fetches. |
| `watch_throttle` | bool | `true` | Master switch for focus/idle watch throttling. Set `false` to disable it entirely; the watch tick then always uses `watch_interval`. |
| `all_namespaces` | bool | `true` | Startup namespace scope: `true` shows all namespaces, `false` scopes to the context's default namespace. Unset, a kubeconfig context that pins a namespace scopes to it instead. The `--namespace` CLI flag and per-bookmark/session scope override this. |
| `events` | object | *(see Events section)* | Events view startup-toggle defaults. See [Events](#events). |
| `log_tail_lines` | int | `100` | **Deprecated** — use `log_viewer.tail_lines`. Number of log lines to load initially via `--tail`. |
| `log_tail_lines_short` | int | `10` | **Deprecated** — use `log_viewer.tail_lines_short`. Number of log lines loaded by the action menu "Tail Logs" entry. Non-positive values are ignored. |
| `log_render_ansi` | bool | `true` | **Deprecated** — use `log_viewer.render_ansi`. Render ANSI SGR sequences from log producers. |
| `log_top_default_profile` | string | `auto` | Default Log Top parser: `auto`, `traefik-json`, `ingress-nginx`, `nginx-combined`, `envoy`, `json`, `logfmt`. |
| `confirm_on_exit` | bool | `true` | Show quit confirmation when pressing `ctrl+c` on the last tab. Set to `false` to exit immediately. |
| `delete_propagation_policy` | string | `"background"` | Cascade policy the delete confirm starts on. `background`: delete now, garbage collector removes dependents (kubectl's default). `foreground`: keep the object until dependents are gone. `orphan`: leave dependents running. `none`: send no policy, letting the API server apply its per-resource default. `Tab` cycles it per delete; force delete clamps `none` to `background` because it runs through `kubectl`. |
| `dim_overlay` | bool | `true` | **Deprecated** — use `appearance.dim_overlay`. Fade the rest of the screen while any overlay is up. Set to `false` for terminals where SGR faint looks awkward; no-op when `no_color: true`. |
| `row_status_tint` | string | `"foreground"` | **Deprecated** — use `appearance.row_status_tint`. See [Appearance](#appearance). |
| `scrolloff` | int | `5` | Number of lines to keep visible above/below the cursor when scrolling. Used by all views with cursor-based navigation. |
| `mouse` | bool | `true` | Capture mouse input for click navigation, scroll, and tab switching. Set to `false` to allow native terminal text selection. Also available as `--no-mouse` CLI flag. |
| `read_only` | bool | `false` | Disable all mutating actions (delete, edit, scale, restart, exec, port-forward, drain, cordon, etc.) globally. Per-context overrides under `clusters.<name>.read_only` and the `--read-only` CLI flag take precedence. See [Read-Only Mode](usage.md#read-only-mode). |
| `clusters.<name>.read_only` | bool | _(unset)_ | Per-context read-only override. Wins over the global `read_only` so you can mark specific clusters (e.g. `prod`) read-only while leaving others mutable. |
| `field_manager` | string | `lfk:<os-user>` | Field manager name sent on every write. The API server stores it in `metadata.managedFields`, which the YAML blame view (`m`) reads, so a change shows the person and not only the tool. Set `lfk` to keep your username off the cluster. Cut to 128 characters. |
| `show_rare_types` | bool | `false` | Show the rarely-used resource types (CSI internals, admission webhooks, leases, runtime classes, and the "Advanced" category) in the sidebar from startup, as if `H` were pressed — so the full type list is always visible. The `H` toggle still flips it for the session; it resets to this value on the next launch. |
| `security.enabled` | bool | `true` | Enable the built-in security findings dashboard (Security category + SEC badge). `false` turns the dashboard and all source probing off. Per-context overrides under `clusters.<name>.security` take precedence. See [Security Dashboard](security.md). |
| `security.hide_badges` | bool | `false` | Hide the per-resource SEC severity badge by default. The dashboard still scans; only the inline row badge is suppressed. Toggle at runtime with the security badge key (`B`). Per-context overrides under `clusters.<name>.security.hide_badges` take precedence. |
| `security.sources.<name>` | bool | `true` | Enable/disable individual sources (`heuristic`, `advisor`, `rbac`, `trivy`, `kyverno`, `kubescape`, `falco`, `gatekeeper`). A source omitted from the map stays enabled. |
| `security.ignore_patterns` | list | _(empty)_ | Declarative glob-based ignore rules applied at load. See [Security](#security) below for field details. |
| `security.heuristic.secret_env_include` | list | _(empty)_ | Extra env-var name globs (`*`, `?`; case-insensitive) the heuristic `secret_env` check flags, on top of the built-in credential keywords. An explicit match overrides a built-in exemption. Top-level `security` section only. |
| `security.heuristic.secret_env_exclude` | list | _(empty)_ | Env-var name globs the heuristic `secret_env` check never flags, on top of the built-in exemptions. Wins over `secret_env_include`. Top-level `security` section only. |
| `security.heuristic.scan_secrets` | bool | `true` | Allow the heuristic source to list Secret objects, enabling the `legacy_sa_token_secret` and `tls_secret_expiry` checks and Secret-reference verification. `false` means the source never reads Secrets. Top-level `security` section only. |
| `clusters.<name>.security` | object | _(unset)_ | Per-context security override (`enabled`, `hide_badges`, and/or `sources`). Wins over the global `security` settings for that context. |
| `secret_lazy_loading` | bool | `false` | When `true`, Secret listing fetches metadata only and decoded values load on hover. See [Secret lazy loading](#secret-lazy-loading) for trade-offs. |
| `informer_cache` | bool or string | `auto` | Selects the list strategy. Accepts `off` (round-trip every list — matches kubectl), `auto` (default; promote a resource type to a shared informer once a list crosses 1000 items, demote when sustained below 500), and `always` (open a watch eagerly on first use). Legacy bool form is still accepted: `true` → `always`, `false` → `off`. See [Informer cache](#informer-cache) for trade-offs. |
| `min_contrast_ratio` | float | `0.0` | **Deprecated** — use `appearance.min_contrast_ratio`. Normalized readability knob in `[0.0, 1.0]`. When above zero, foreground colors are nudged in HSL lightness space to meet a minimum WCAG contrast ratio against their paired background. See [Minimum Contrast Ratio](#minimum-contrast-ratio). |

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

### Emoji width

In `icons: emoji`, some emoji are two columns wide only because of a trailing
variation selector (`U+FE0F`). A terminal shows that extra width only when it
supports grapheme clustering (mode 2027) — kitty, WezTerm and Ghostty do,
Konsole does not. lfk asks the terminal at startup and drops the selector when
the answer says no, so those icons render in their narrow monochrome form and
the columns stay aligned. Nothing to configure.

## Appearance

Visual settings, grouped. The flat keys of the same name (`colorscheme`, `icons`, `no_color`, `transparent_background`, `min_contrast_ratio`, `dim_overlay`, `row_status_tint`) are deprecated aliases; when both a flat key and its `appearance` equivalent are set, `appearance` wins. The `theme` object (custom color overrides) and per-cluster overrides remain top-level.

```yaml
appearance:
  colorscheme: tokyonight-storm
  icons: auto
  no_color: false
  transparent_background: false
  min_contrast_ratio: 0.0
  dim_overlay: true
  row_status_tint: foreground
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `colorscheme` | string | `"tokyonight-storm"` | Built-in color scheme name. Dual-mode `"dark:X,light:Y"` supported. |
| `icons` | string | `"auto"` | Icon display mode (`auto`/`unicode`/`nerdfont`/`simple`/`emoji`/`none`). |
| `no_color` | bool | `false` | Strip all colors (monochrome). `NO_COLOR` env var takes precedence. |
| `transparent_background` | bool | `false` | Use the terminal's own background for bars and surfaces. |
| `min_contrast_ratio` | float | `0.0` | Readability knob in `[0.0, 1.0]`. See [Minimum Contrast Ratio](#minimum-contrast-ratio). |
| `dim_overlay` | bool | `true` | Fade the screen behind overlays. No-op when `no_color: true`. |
| `row_status_tint` | string | `"foreground"` | Emphasize failed/progressing rows as a fallback for when the Status cell is not visible. See [Row status tint](#row-status-tint) below. |

`icons` also decides how the which-key panel and the help screen's key column draw keys: Nerd Font keycaps (`󰘴 D`), Unicode symbols (`⌃D`), or names (`ctrl+d`).

### Row status tint

- `foreground`: colors the row's **Name** cell in the status color (same color as the Status cell), only when the Status column is hidden — no change while the Status column is shown. When the Name column is also hidden, the whole row is tinted so the signal is not lost.
- `background`: a muted status-colored background across the whole row, always. The cursor row blends the status color toward the selection color so it stays visible.
- `off`: only the Status cell is colored.

In `no_color` mode the tint degrades to bold (failed) / italic (progressing); a selected failed row uses underline, since the selection is already bold.

## Log Viewer

Settings for the log viewer (`L` key). The startup-toggle defaults let you skip pressing the toggle keys on every open (issue #377). `log_path` is the application's own log file and is intentionally **not** part of this group.

```yaml
log_viewer:
  tail_lines: 100
  tail_lines_short: 10
  render_ansi: true
  show_preview: true
  show_prefixes: true
  show_timestamps: false
  preview_live: false
  wrap: false
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `tail_lines` | int | `100` | Log lines loaded initially via `--tail` (per container). Scrolling to the top loads older history. |
| `tail_lines_short` | int | `10` | Log lines loaded by the action menu "Tail Logs" entry. Non-positive values are ignored. |
| `render_ansi` | bool | `true` | Render ANSI SGR sequences (color, bold, underline) from log producers. `false` strips them (every ESC byte becomes `U+FFFD`); non-SGR CSI sequences are always stripped. Toggle at runtime with `:set ansi` / `:set noansi`. |
| `show_preview` | bool | `true` | Startup default for the structured preview side panel. Runtime toggle: `toggle_preview` (`P`). |
| `show_prefixes` | bool | `true` | Startup default for the `[pod/name/container]` line prefixes. Runtime toggle: `toggle_prefixes` (`p`). |
| `show_timestamps` | bool | `false` | Startup default for log line timestamps. Runtime toggle: `toggle_timestamps` (`s`). |
| `preview_live` | bool | `false` | Startup default for the right-pane live-log preview. When true the explorer opens with live logs streaming in the right pane for the selected pod. Runtime toggle: `toggle_preview_logs` (`L`). |
| `max_lines` | int | `50000` | Max streamed log lines retained per tab; once exceeded, the oldest lines are dropped so a long-running follow stays bounded in memory. Clamped to `[1000, 1000000]`. |
| `wrap` | bool | `false` | Startup default for log line wrapping. Runtime toggle: `toggle_wrap` (`>`). |

The deprecated flat keys `log_tail_lines`, `log_tail_lines_short`, and `log_render_ansi` are still accepted as aliases; when both a flat key and its `log_viewer` equivalent are set, `log_viewer` wins.

## Viewer defaults

Startup defaults for the fullscreen YAML / diff / describe viewers, so display toggles you always want don't need re-pressing on every open. Each viewer mirrors the `log_viewer` group.

```yaml
yaml_viewer:
  wrap: false
diff_viewer:
  wrap: false
  line_numbers: true
  unified: false
describe_viewer:
  wrap: false
object_explorer:
  live: true
  tree: false
api_explorer:
  tree: false
```

| Field | Type | Default | Runtime toggle | Description |
|-------|------|---------|----------------|-------------|
| `yaml_viewer.wrap` | bool | `false` | `toggle_wrap` (`>`) | Line wrapping in the YAML viewer. |
| `diff_viewer.wrap` | bool | `false` | `toggle_wrap` (`>`) | Line wrapping in the diff viewer. |
| `diff_viewer.line_numbers` | bool | `true` | `toggle_line_numbers` (`#`) | Gutter line numbers in the diff viewer. |
| `diff_viewer.unified` | bool | `false` | `toggle_unified` (`u`) | Unified (vs side-by-side) diff layout. |
| `describe_viewer.wrap` | bool | `false` | `toggle_wrap` (`>`) | Line wrapping in the describe viewer. |
| `object_explorer.live` | bool | `true` | `watch_mode` (`w`) | Live-refresh the browsed object as the resource changes under watch mode. Manual refresh with `refresh` (`R`). |
| `object_explorer.tree` | bool | `false` | `tree_view` (`T`) | Open the Object Explorer in the ASCII-art tree view. |
| `api_explorer.tree` | bool | `false` | `tree_view` (`T`) | Open the API Explorer in the ASCII-art tree view (sticky across navigation). |

A toggle changed at runtime sticks for the session and resets to the configured default the next time the viewer opens.

## Events

Startup defaults for the events view.

```yaml
events:
  warnings_only: true
  grouping: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `events.warnings_only` | bool | `true` | Start filtered to Warning events only. |
| `events.grouping` | bool | `true` | Start with related events grouped by reason. |

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

### Monitoring entry fields

Each monitoring entry accepts the following top-level fields:

| Field | Type | Default | Description |
|---|---|---|---|
| `prometheus` | object | *(auto-discovery)* | Prometheus endpoint configuration. See endpoint fields below. |
| `alertmanager` | object | *(auto-discovery)* | Alertmanager endpoint configuration. See endpoint fields below. |
| `node_metrics` | string | *(auto-detect)* | Metrics source for node **and pod** usage: `"prometheus"` or `"metrics-api"`. Empty auto-detects. |

When `node_metrics` is empty, lfk uses Prometheus if a `prometheus` endpoint is configured, otherwise metrics-api. Either way the other source is tried as a fallback, so pod metrics resolve on clusters served only by Prometheus.

### Node uptime column

The nodes list shows an `Uptime` column (how long since each node last booted)
when Prometheus is the configured monitoring source — that is, a `prometheus`
endpoint is set, or `node_metrics: prometheus`. One of the two is required;
the column stays hidden on clusters with no monitoring config. With
`node_metrics: prometheus` and no `prometheus` block, the endpoint itself is
auto-discovered from the well-known namespaces and services listed below.

| Requirement | Detail |
|---|---|
| Source | node_exporter's `node_boot_time_seconds` metric, queried via Prometheus |
| Needs | node_exporter, not just Prometheus (e.g. kube-prometheus-stack ships both) |
| Kubernetes API | Does not expose node boot time, so there is no fallback source |

Clusters without node_exporter never show the column. A node missing from the
query result shows `n/a`, as does every node if Prometheus stops responding.

### Monitoring endpoint fields

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
| `views` | map[string]object | Per-resource-type view overrides for this cluster. Same format as the global `views`. Wins over global `views` for matching keys. |
| `resource_columns` | map[string]list | Per-resource-type column overrides for this cluster. **Deprecated** — use `views` instead. |
| `read_only` | bool | Per-context read-only override. Same semantics as the top-level `read_only`. |
| `security` | object | Per-context security override (`enabled` and/or `sources`). Same semantics as the top-level `security`; wins over it for this context. |
| `k8s_client_qps` | int | Per-context foreground API client QPS override. Wins over the global `scheduler.k8s_client_qps`. See [API client rate limits](#api-client-rate-limits). |
| `k8s_client_burst` | int | Per-context foreground API client burst override. Wins over the global `scheduler.k8s_client_burst`. |

Per-cluster `views` take precedence over the global `views` setting.

```yaml
clusters:
  my-prod-cluster:
    views:
      apps/v1/deployments:
        columns: ["Name", "Replicas", "Available", "REV"]
        sort_column: "REV"         # defaults to desc for REV
    resource_columns:              # deprecated; still works
      Pod: ["IP", "Node", "Image", "CPU", "MEM"]
  my-staging-cluster:
    views:
      pod:
        columns: ["Name", "IP", "Node", "Image"]
```

## Security

Controls the built-in security findings dashboard. Disable it globally or per
cluster, or enable only specific sources. Source keys: `heuristic`, `advisor`,
`rbac`, `trivy`, `kyverno`, `kubescape`, `falco`, `gatekeeper` (the internal ids
`trivy-operator` and `policy-report` are also accepted). Any source omitted
from `sources` stays enabled.

```yaml
security:
  enabled: true            # global default (omit = true)
  sources:
    falco: false           # disable Falco everywhere

  ignore_patterns:        # declarative, glob-based ignore rules (applied at load)
    - source: trivy-operator
      group: "CVE-2024-*"  # CVE id / check label / rule name
      namespace: kube-system  # "" or "*" = any namespace (hides the whole group)
      cluster: "prod-*"    # kube-context glob; "" = any cluster
      comment: accepted in system namespaces
    - labels:              # exclude infra pods by label (key -> glob value; all must match)
        k8s-app: cilium    # silence findings on Cilium pods
      comment: CNI managed separately

clusters:
  prod:
    security:
      enabled: false       # turn the whole dashboard off on prod
  staging:
    security:
      sources:
        trivy: false       # keep the dashboard, drop Trivy on staging only
```

Each `ignore_patterns` entry matches on `cluster`, `source`, `group`, `namespace`
(globs; empty = any), an optional `labels` map (label key -> glob value; all
entries must match), and an optional `comment`. A finding is hidden when every
non-empty field matches. A `labels` entry is resource-scoped — it hides
matching resources, never the whole group — and matches when the finding's
resource has resolvable labels (heuristic-observed pods and same-pod findings,
plus workload labels resolved from the live object for standard kinds when a
label pattern is set). See [Security Dashboard](security.md#label-matching)
for coverage details. An all-empty entry is dropped. Ignore patterns are
read-only from config; the `i` toggle still reveals hidden findings.

Precedence: `clusters.<ctx>.security.*` overrides the global `security.*`. The
interactive per-cluster ignore-list (action menu) and `ignore_patterns` are
applied together; config patterns are read-only. See
[Security Dashboard](security.md) for the source catalog, ignore scopes, and
behavior.

## API client rate limits

lfk caps how fast it calls each cluster's API server (client-side QPS/burst).
The stock client-go default (QPS 5 / burst 10) is low for a multi-resource TUI
and lets a background scan stall foreground lists, so lfk defaults to **QPS 50 /
burst 100**. Tune globally under `scheduler`, or per cluster under
`clusters.<name>`.

| Field | Type | Default | Description |
|---|---|---|---|
| `scheduler.k8s_client_qps` | int | `50` | Sustained foreground requests/sec to the API server. |
| `scheduler.k8s_client_burst` | int | `100` | Burst capacity above QPS for short spikes. |
| `clusters.<name>.k8s_client_qps` | int | _(global)_ | Per-context QPS override. Wins over the global value. |
| `clusters.<name>.k8s_client_burst` | int | _(global)_ | Per-context burst override. |

```yaml
scheduler:
  k8s_client_qps: 50
  k8s_client_burst: 100

clusters:
  big-prod-cluster:
    k8s_client_qps: 200      # raise the ceiling on a large, dedicated cluster
    k8s_client_burst: 400
  shared-cluster:
    k8s_client_qps: 20       # be gentle on a shared/throttled API server
    k8s_client_burst: 40
```

Precedence: `clusters.<ctx>.k8s_client_*` overrides `scheduler.k8s_client_*`,
which overrides the built-in default. Security source scans run on a separate,
deliberately lower budget (QPS 10 / burst 20) so background finding scans never
starve foreground lists — see [Security Dashboard](security.md).

### Background task scheduling

lfk runs API work in a per-context worker pool, classified into priority
lanes: Critical (API discovery, RBAC, namespaces, mutations), High (the view
you are looking at — resource lists, drill-in, preview), and Low (background
work — security scans, metrics, events, dashboards). Three knobs keep
background work flowing without slowing the foreground, which matters most on
slow-responding clusters where calls hold a worker for seconds:

| Field | Type | Default | Description |
|---|---|---|---|
| `scheduler.workers_per_context` | int | `8` | Concurrent workers per cluster context (clamped 1–16). More slots drain the metrics/events backlog faster on slow clusters. |
| `scheduler.critical_reserved_slots` | int | `1` | Workers that prefer Critical work (clamped to ≤ half the pool). |
| `scheduler.low_reserved_slots` | int | `2` | Workers that prefer Low (background) work so metrics/events/dashboards always have a slot even while the foreground floods the pool. Clamped to leave ≥1 general worker. |
| `scheduler.aging_threshold` | int | `8` | Max higher-priority dispatches a background task waits behind before it is promoted for one task. `0` disables aging (strict priority). |

Reserved workers are not idle: a reserved worker prefers its lane but falls
back to any other queued work when its lane is empty, so reservations bias
capacity without wasting it. Critical is never aged. The low-reserved floor
and aging are complementary: the floor guarantees throughput for background
work under a saturated pool, while aging bounds queue-ordering latency when
High and Low are both backed up.

To keep API load bounded on slow clusters, the scheduler avoids redundant work:
an identical fetch that is already in flight (same kind, target, and navigation
generation) absorbs new submissions instead of running twice. This gives
watch-mode its effective cadence under load — a tick that fires while the
previous identical refresh is still running is coalesced away rather than
stacking a duplicate fetch, so metrics/list refreshes never pile up faster than
the cluster can serve them. Automatic and not configurable.

```yaml
scheduler:
  workers_per_context: 8
  critical_reserved_slots: 1
  low_reserved_slots: 2   # always keep slots for metrics/events/dashboards
  aging_threshold: 8      # 0 = strict priority; higher = background waits longer
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

The fullscreen viewers (YAML, diff, describe, log, events) honor the shared `search`, `help`, `next_match`, `prev_match`, and `toggle_preview` bindings, plus the `toggle_*` display toggles below. The log viewer additionally honors `filter` (live text filter: `~`fuzzy, regex auto-detected, `\`literal) and `severity_up`/`severity_down` (step the minimum log severity shown). The `fullscreen` binding (default `F` / Shift+F) maximizes/minimizes everywhere it applies — the explorer's middle column and dashboard, the event timeline, and the error log. Core cursor navigation inside these viewers (`hjkl`, `g`/`G`, page motions, vim word-motions) remains fixed and is not yet rebindable.

| Field | Default | Action |
|---|---|---|
| `logs` | `ctrl+l` | Open the fullscreen log viewer for the selected resource |
| `refresh` | `R` | Refresh current view |
| `describe` | `v` | Describe selected resource |
| `delete` | `D` | Delete resource (force delete Pod/Job if already deleting) |
| `force_delete` | `X` | Force delete (Pod/Job only) |
| `scale` | `S` | Scale resource (deployments, statefulsets, daemonsets) |
| `edit` | `E` | Edit selected resource in $KUBE_EDITOR or $EDITOR |
| `label_editor` | `i` | Edit labels/annotations |
| `secret_editor` | `e` | Secret/ConfigMap editor |
| `column_toggle` | `,` | Column visibility toggle |
| `session_manager` | `C` | Open the session manager (save/switch/delete named workspace sessions). |
| `toggle_wrap` | `>` | Toggle line wrapping in the YAML, diff, describe, log, and event viewers. |
| `toggle_line_numbers` | `#` | Toggle line numbers (diff, log viewers). |
| `toggle_fold` | `z` | Toggle fold on the section/region under the cursor (YAML, diff viewers). |
| `toggle_fold_all` | `Z` | Toggle all folds (YAML, diff viewers). |
| `toggle_follow` | `F` | Toggle follow / auto-scroll (log viewer). Moved from `f`, which now opens the log filter. |
| `severity_up` | `o` | Raise the minimum log severity shown — cycles off → INFO+ → WARN+ → ERROR+ (log viewer). |
| `severity_down` | `i` | Lower the minimum log severity shown (log viewer). |
| `toggle_timestamps` | `s` | Toggle timestamps (log viewer). |
| `toggle_prefixes` | `p` | Toggle `[pod/name/container]` line prefixes (log viewer). |
| `toggle_unified` | `u` | Toggle unified vs side-by-side layout (diff viewer). |
| `tree_view` | `T` | Toggle the ASCII-art tree view (Object Explorer, API Explorer). |
| `field_doc` | `ctrl+k` | Toggle the schema side pane for the field under the cursor (YAML viewer, Object Explorer). |
| `toggle_preview` | `P` | Toggle the structured preview side panel (log viewer) / details↔YAML preview (explorer). |
| `toggle_preview_logs` | `L` | Toggle the right-pane live-log preview for the selected pod or container (explorer; deeper levels only). |
| `fullscreen` | `F` | Explorer: cycle layout (hide sidebar -> fullscreen -> restore). Dashboard / event timeline / error log: fullscreen toggle. |
| `sort_next` | `>` | Sort by next column |
| `sort_prev` | `<` | Sort by previous column |
| `sort_flip` | `=` | Toggle sort direction |
| `sort_reset` | `-` | Reset sort to default |
| `error_log` | `!` | Error log overlay |
| `finalizer_search` | `ctrl+g` | Finalizer search and remove |
| `terminal_toggle` | `ctrl+t` | Cycle terminal mode (pty/exec/mux) |
| `mouse_toggle` | `ctrl+alt+y` | Suspend/resume mouse capture (Ctrl+Option+Y) for native text selection |
| `toggle_rare` | `H` | Toggle rarely used resource types in the sidebar |

### Keybinding notes

- `toggle_fold` also folds tree-view branches in the Object/API Explorers, alongside `Space`.
- `severity_up`/`severity_down` read the parsed level on structured logs; plain-text logs are classified by keyword (error/warn/debug), defaulting to INFO.
- Shared defaults across separate contexts: `>` (`toggle_wrap` in viewers vs. `sort_next` in the resource list), `T` (`tree_view` in explorers vs. `theme_selector` in the resource list), `i` (`severity_down` in the log viewer vs. `label_editor` / `security_ignore_toggle` in the resource list).

## Views

Use `views` to configure columns and default sort for specific resource types. Keys are either a GVR (`apps/v1/deployments`) or a Kind name (`deployment`, case-insensitive). When both match, GVR takes precedence.

### View entry fields

| Field | Type | Default | Description |
|---|---|---|---|
| `columns` | list[string] | *(auto-detected)* | Ordered column specs. See [Column spec syntax](#column-spec-syntax). |
| `sort_column` | string | *(none)* | Default sort column. Format: `"COL"` or `"COL:asc"` / `"COL:desc"`. When direction is omitted, `REV` defaults to `desc`; all other columns default to `asc`. |

### Column spec syntax

```text
NAME[:.jsonpath][|flag]*
```

| Form | Meaning |
|---|---|
| `Name` | Built-in column resolved by the renderer (Name, Age, Ready, REV, etc.) |
| `Name:.jsonpath` | Custom column — JSONPath evaluated against the source object. |
| `Name:.jsonpath\|flag` | Custom column with one or more display flags (stackable with multiple `\|`). |

**Flags** (case-insensitive):

| Flag | Effect |
|---|---|
| `R` | Right-align the column |
| `T` | Humanize value as a timestamp |
| `W` | Wide-only — hidden until the terminal width crosses the wide threshold |

**JSONPath notes:** compile errors at config load emit a `logger.Warn` and skip that view (the rest of config loads normally). A miss at row-render time produces an empty cell with no log spam. The JSONPath dialect is kubectl-flavored (`k8s.io/client-go/util/jsonpath`); advanced filter expressions that differ from k9s's dialect may not be portable.

### Example

```yaml
views:
  # GVR key — wins over a matching Kind key
  apps/v1/deployments:
    columns: ["Name", "Namespace", "Replicas", "Available", "REV", "Age"]
    sort_column: "REV"            # desc by default for REV
  # Kind key (case-insensitive)
  pod:
    columns:
      - "Name"
      - "IP"
      - "Node"
      - "GitSHA:.metadata.labels.git-sha|W"   # custom JSONPath column, wide-only
      - "Age"
    sort_column: "Age:asc"
```

Per-cluster overrides follow the same format under `clusters.<name>.views` and win over global `views` for matching keys.

## Resource columns (deprecated)

> **Deprecated.** Use [`views`](#views) instead. `resource_columns` is bridged automatically — each entry becomes a `views` entry with the same columns and no `sort_column`. A one-time warning is logged when bridging occurs. When `views` is set for the same Kind, `views` wins.

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

## Resource templates

Templates listed in the create-from-template picker (`a`) come from two places: the built-ins, and one YAML file per template in `~/.config/lfk/templates/`.

| Rule | Behaviour |
|---|---|
| File name | The template name (`web.yaml` -> `web`) |
| Content | A plain Kubernetes manifest — no template-specific wrapper (keep the object's own `metadata`) |
| Extensions | `.yaml` and `.yml`; anything else is ignored |
| Category | Always `User`; user templates list before the built-ins |
| Name collision | Both rows stay; the `User` category marks yours |
| Broken file | Skipped and logged; the other templates still load |

`x` -> Export Template writes into this same directory, so exported and hand-written templates live together. Exporting a Secret keeps the keys and the `type` but blanks every value — press `s` on the destination picker to keep the values, or use `y` / the YAML view. See [keybindings.md](keybindings.md#action-menu-items) for the full category list.

```yaml
# ~/.config/lfk/templates/web.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
        - name: app
          image: nginx:latest
```

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

### Default abbreviations

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

## Custom actions

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

### Custom action fields

| Field | Required | Description |
|---|---|---|
| `label` | Yes | Display name in the action menu |
| `command` | Yes | Shell command to execute (supports template variables) |
| `key` | Yes | Shortcut key in the action menu |
| `description` | No | Description shown next to the action |
| `read_only_safe` | No | When `true`, available in read-only mode. Defaults to `false`. |

Set `read_only_safe: true` for view-only commands (e.g. `kubectl describe`, `kubectl logs`).

### Template variables

Template variables are substituted before execution:

| Variable | Description |
|---|---|
| `{name}` | Resource name |
| `{namespace}` | Resource namespace |
| `{context}` | Kubeconfig context name |
| `{kind}` | Resource kind (e.g., "Pod", "Deployment") |
| `{<ColumnKey>}` | Any column value from the resource (e.g., `{Node}`, `{IP}`) — case-insensitive |

Custom action commands are executed via `sh -c` with `KUBECONFIG` set in the environment. Interactive commands (like `ssh`) hand over the terminal to the subprocess.

Substituted values are shell-quoted before insertion, so cluster data (context names, labels, image strings) containing shell metacharacters is passed literally and cannot inject commands. Quoting is transparent for normal argument use — `ssh {Node}` and `/tmp/{name}.log` work as written.

## Pinned groups

Pin CRD API groups so they appear right after the built-in categories (Workloads, Networking, etc.) instead of being sorted alphabetically under Custom Resources.

```yaml
pinned_groups:
  - karpenter.sh
  - monitoring.coreos.com
  - argoproj.io
```

Pinned groups can also be managed interactively: press `p` at the resource types level to pin/unpin the selected CRD group. In-app pins are stored per-context, and for named union sets per union set, in `~/.local/state/lfk/pinned.yaml`; they are merged with the config file pins at runtime. Anonymous `--union-context` sessions have no durable name, so interactive pinning remains disabled there.

Pinning a type's dashboard summary is a separate action (the action menu's "Pin summary" entry, `x` at the resource types level) with its own state file; see the `pinned_summaries` field above. With a named union set active, each member cluster's dashboard preview also shows the union set's pinned summaries, not just its own.

When nothing is pinned anywhere (config or state), the dashboard shows a built-in default set instead: `batch/jobs`, `apps/deployments`, `argoproj.io/applications`, `kustomize.toolkit.fluxcd.io/kustomizations`, `cert-manager.io/certificates`. CRD-backed defaults are silently skipped when the cluster doesn't have the type. Your first interactive pin copies the currently-active defaults into your pinned list, then adds (or removes) the selected type, so the defaults you already see are kept, not replaced. Setting `pinned_summaries` in config is still a full explicit list and replaces the defaults outright. Set `pinned_summaries: []` in config to disable the defaults and show none.

## Union sets

Define named groups of clusters that the `--union-set <name>` CLI flag expands into a merged ("union") view. Avoids retyping long `--union-context` lists for the same recurring groups (blue/green/canary, region triplets, etc.). The flag is mutually exclusive with `--union-context` and `--context`.

```yaml
union_sets:
  ski-staging-west:
    contexts:
      - context: operator/block-sre-operator/square-staging-green-us-west-2
        color: green
      - context: operator/block-sre-operator/square-staging-yellow-us-west-2
        color: yellow
      - context: operator/block-sre-operator/square-staging-blue-us-west-2
        color: blue
    namespace: kube-policies
  ski-prod-east:
    contexts:
      - prod-green-east
      - name: prod-blue-east
    # namespace omitted: --namespace is required on the CLI for this set
    # color omitted on each context: rows render with a blank reserved cell
```

Top-level fields:

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| set name | map key | yes | Identifier referenced by `--union-set <name>` and shown under the cluster picker's `Union Sets` group. List form with `- name: ...` is also accepted; duplicate names log a warning and the last definition wins. |
| `contexts` | list[string or object] | yes | Context entries to merge. Each entry can be a plain string or an object with `context:`/`name:` and optional `color:` (see below). The total count is capped at `MaxUnionContexts` (currently 8) — the same cap that governs repeated `--union-context` flags. |
| `namespace` | string | no | Default namespace for the set. Optional: when omitted, `--namespace` is required on the CLI. The CLI flag always overrides whatever the set declared, so you can keep one canonical set and retarget the namespace per launch. |

Per-cluster entry fields:

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `context` / `name` | string | yes | Kubeconfig context name. Must exist; verified at startup. Plain string entries are shorthand for `context: <name>`. |
| `color` | string | no | One of: `red`, `yellow`, `green`, `blue`, `magenta`, `cyan`, `white`, `gray`. Drives the 1-cell row tile in the merged view (see below). Unknown color names log a warning and the cluster renders untinted rather than failing the config. |

Validation runs at startup: an unknown set name, a missing context, a duplicate context within the set, more than `MaxUnionContexts` clusters, or no namespace from either source produces a clear error before the TUI starts. Malformed list-form entries (no `name`), empty `contexts:`, and nameless context entries are dropped at config load time with a warning rather than failing the whole config.

See [the union view docs in usage.md](usage.md) for the runtime semantics — what's editable at the merged level, how drill-down routes to a single cluster, and how the per-row `Context` column identifies the source.

### Per-row color tiles

When a cluster entry has a `color:` set, every row sourced from that cluster gets a 1-cell colored tile at its leading edge — quick visual identification of the source cluster while scanning a merged list. Rows from clusters without a configured color get a blank reserved cell at the same position so column boundaries stay aligned across the table.

The colors live inside the `union_sets` definition (not the global `cluster_colors` state file used by the cluster picker) so you can pick deliberate "traffic light" semantics per view — for example, the same kubeconfig context can be tinted blue here and untinted in another set, without affecting how it appears in the cluster picker. The textual `Context` column remains the source of truth for unambiguous reads (sortable, copyable, fits screen-readers); the tile is purely visual shorthand.

## Filter presets

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

### Filter preset fields

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Display name in the filter overlay |
| `key` | Yes | Single-character shortcut key (must not conflict with built-in presets) |
| `match` | Yes | Filter criteria object (all fields are AND-ed) |

### Match criteria

| Field | Description |
|---|---|
| `status` | Substring match against the resource status (case-insensitive) |
| `ready_not` | `true` to match resources where ready count != desired (e.g., "1/3") |
| `restarts_gt` | Match pods with restart count greater than this number |
| `column` | Column key to check (case-insensitive, e.g., "Node", "IP") |
| `column_value` | Substring match against the column value (case-insensitive) |
| `invert` | `true` to negate the entire match (fields still AND together first). |

`invert: true` flips the final AND result — useful for "anything except X" filters, e.g. `status: Bound` + `invert: true` matches every PVC that is NOT Bound.

### Built-in presets

Press `.` on any resource list to see the presets available for that kind. The built-ins are:

| Kind | Presets (key) |
|---|---|
| Pod | Failing (`f`), Pending (`p`), Not Ready (`n`), Restarting (`r`), High Restarts (`R`), **Not Running (`x`)** |
| Deployment / StatefulSet / DaemonSet | Not Ready (`n`), Failing (`f`), **Not Running (`x`)** |
| Job | Failed (`f`), **Not Running (`x`)** |
| PersistentVolume | Available (`a`), Released (`r`), Failed (`f`), **Not Bound (`x`)** |
| PersistentVolumeClaim | Pending (`p`), Lost (`l`), **Not Bound (`x`)** |
| Node | Not Ready (`n`), Cordoned (`c`) |
| Certificate / CertificateRequest | Not Ready (`n`), Expiring Soon (`e`) |
| Service | LB No IP (`l`) |
| CronJob | Suspended (`s`) |
| Argo Application | Out of Sync (`s`), Degraded (`d`) |
| Flux HelmRelease / Kustomization | Suspended (`s`), Not Ready (`n`) |
| Event | Warnings (`w`) |

The `x` key (Not Running / Not Bound) is the shared "show me anything not in a healthy state" mnemonic across the kinds where it applies — k9s `Ctrl-Z` equivalent. Universal presets `Old (>30d)` (`o`) and `Recent (<1h)` (`h`) are also available on every list.

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

## Informer cache

Without the cache, every navigation event in the resource list — drill-in,
namespace switch, watch tick, `shift+r` refresh — issues a fresh `LIST`
against the API server. On large clusters this becomes visible: a 7k-pod
list takes 1–2 seconds to round-trip, and switching the namespace filter
re-pays that cost every time even though filtering is conceptually a
client-side operation against an already-known list (issue #86).

The `informer_cache` knob has three modes:

- **`off`** — every list round-trips to the API server, matching kubectl
  semantics. Pick this when the apiserver is sensitive to extra watch
  connections or when you are debugging server-side behavior.
- **`auto`** (default) — start in `off` mode per `(context, resource type)`,
  but the moment a list returns ≥ 1000 items, promote that resource type
  to a [shared informer](https://pkg.go.dev/k8s.io/client-go/dynamic/dynamicinformer).
  A resource type refreshed repeatedly by the visible list or dashboard —
  regardless of size — also promotes, and stays promoted for as long as
  something keeps refreshing it. Only eligible resource types promote:
  Events, Secrets, `metrics.k8s.io`, and types whose declared verbs omit
  `watch` always use direct lists. Subsequent lists for a promoted resource
  type — including namespace switches — are served from the in-memory
  index. Once cached size has stayed below 500 for three consecutive
  cached calls, the watch is closed and the resource type goes back to
  direct lists. Hysteresis between the promote (1000) and demote (500)
  thresholds prevents flapping when a list size hovers near the edge.
- **`always`** — every resource type starts a watch on first use,
  regardless of size. Use when you know the cluster is large and want to
  skip the one-time direct list before promotion.

Promoted resource types serve namespace switches as in-memory walks; the
watch keeps the cache fresh so deleted pods drop out and `Age` advances on
the next render even though we never re-issue a `LIST`.

### Trade-offs

| | `off` | `auto` (default) | `always` |
|---|---|---|---|
| Namespace switch on 7k-pod list | 1–2 s API round trip | In-process walk (after first list) | In-process walk (after first list) |
| API server load (small clusters) | One LIST per nav | One LIST per nav (never promoted) | One LIST + one watch per opened type |
| API server load (large clusters) | One LIST per nav | One initial LIST then one watch per promoted type | Same as `auto` |
| Memory in lfk | View only | Cached objects for promoted types | Cached objects for every opened type |
| One-off large list strands a watch | n/a | No (auto-demote closes it) | Yes (until shutdown) |

The legacy bool form is still accepted for backward compatibility:

- `informer_cache: true` is treated as `always`.
- `informer_cache: false` is treated as `off`.

```yaml
# Default — adapts per resource type without configuration:
informer_cache: auto

# Force everything through informers:
# informer_cache: always

# Disable entirely (kubectl-equivalent semantics):
# informer_cache: off
```

## Minimum contrast ratio

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

Enable when text is hard to read on your terminal. Start at `0.175` (WCAG
AA); raise toward `0.3` if still too dim.

```yaml
min_contrast_ratio: 0.175
```

## Terminal mode

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

## PTY scrollback

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

## kubeshark

| Field | Type | Default | Description |
|---|---|---|---|
| `kubeshark.namespace` | string | `kubeshark` | Namespace probed for `Service kubeshark-hub` by the Traffic Capture overlay's kubeshark backend. |

If the Service isn't found in this namespace, the kubeshark chip is
omitted from the backend picker.

## Color schemes

Over 460 built-in color schemes are available, generated from [ghostty terminal themes](https://github.com/ghostty-org/ghostty). Sample list:

`tokyonight-storm` (default), `dracula`, `nord`, `catppuccin-mocha`, `catppuccin-latte`, `rose-pine`, `gruvbox-dark`, `gruvbox-light`, `everforest-dark`, `one-half-dark`, `ayu-dark`, `nightfox`, `monokai-pro`, `github-dark`, `github-light`, `solarized-dark`, `solarized-light`, and many more.

Switch themes at runtime with `T` to browse all available themes interactively with live preview. Runtime changes are not persisted — set `colorscheme` in your config to make it permanent.

To regenerate themes from the latest ghostty source, run `make generate-themes`.

## Session persistence

The application automatically saves and restores the last visited context, namespace, and resource type across restarts (per tab, including the active tab). It also restores the active list filter (including the `Tab` broad-match mode) and the highlighted row, so you reopen lfk on the same resource you were looking at. The session state is stored at `~/.local/state/lfk/session.yaml`.

This is transparent and requires no configuration. To start fresh, delete the session file.

## Files

The application follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/):

| File | Description |
|---|---|
| `$XDG_CONFIG_HOME/lfk/config.yaml` | Main configuration file (default: `~/.config/lfk/config.yaml`) |
| `$XDG_STATE_HOME/lfk/bookmarks.yaml` | Saved bookmarks (default: `~/.local/state/lfk/bookmarks.yaml`) |
| `$XDG_STATE_HOME/lfk/session.yaml` | Last session state, auto-managed (default: `~/.local/state/lfk/session.yaml`) |
| `$XDG_STATE_HOME/lfk/sort_memory.yaml` | Per-context and per-resource-kind sort column/order, auto-managed (default: `~/.local/state/lfk/sort_memory.yaml`) |
| `$XDG_STATE_HOME/lfk/column_prefs.yaml` | Per-context and per-resource-kind column visibility/order from the `,` toggle overlay, auto-managed (default: `~/.local/state/lfk/column_prefs.yaml`) |
| `$XDG_STATE_HOME/lfk/pinned.yaml` | Per-context and per-union-set pinned CRD groups, managed via `p` key (default: `~/.local/state/lfk/pinned.yaml`) |
| `$XDG_STATE_HOME/lfk/hidden_types.yaml` | Per-context and per-union-set hidden resource types, managed via the action menu (`x`) at the resource types level (default: `~/.local/state/lfk/hidden_types.yaml`) |
| `$XDG_STATE_HOME/lfk/pinned_summaries.yaml` | Per-context and per-union-set resource-type summaries pinned to the cluster dashboard, managed via the action menu (default: `~/.local/state/lfk/pinned_summaries.yaml`) |
| `$XDG_STATE_HOME/lfk/whichkey_prefs.yaml` | Which-key panel entry order from the leader-key toggle, auto-managed; outranks `which_key_grouped` (default: `~/.local/state/lfk/whichkey_prefs.yaml`) |
| `~/.local/share/lfk/lfk.log` (default) | Application log file (configurable via `log_path`) |

State files stored at the legacy `~/.config/lfk/` location are automatically migrated to the new XDG state directory on first access.

### Directory overrides

For portable installs such as Scoop's persist directory, lfk honors three environment variables that override the XDG variables and OS defaults:

| Variable | Overrides | Holds |
|---|---|---|
| `LFK_CONFIG_DIR` | config directory | `config.yaml` |
| `LFK_STATE_DIR` | state directory | bookmarks, session, pinned groups, history, cluster colors, captures |
| `LFK_DATA_DIR` | data directory | `lfk.log` |

Each variable is used verbatim as the lfk directory — unlike `XDG_*_HOME`, no `lfk` component is appended. Resolution precedence per directory:

1. `LFK_<X>_DIR` if set.
2. `$XDG_<X>_HOME/lfk` if set.
3. OS default (`~/.config/lfk`, `~/.local/state/lfk`, `~/.local/share/lfk`).

The `--config` flag and the `log_path` config field are full file-path overrides and still win over `LFK_CONFIG_DIR` and `LFK_DATA_DIR` respectively.
