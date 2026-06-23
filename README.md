<img src="docs/imgs/github-header.png" alt="lfk">

# :zap: LFK - Lightning Fast Kubernetes navigator

[![Release](https://img.shields.io/github/v/release/janosmiko/lfk)](https://github.com/janosmiko/lfk/releases) [![CI](https://img.shields.io/github/actions/workflow/status/janosmiko/lfk/ci.yml?branch=main&label=CI)](https://github.com/janosmiko/lfk/actions/workflows/ci.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/janosmiko/lfk)](https://goreportcard.com/report/github.com/janosmiko/lfk) [![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=janosmiko_lfk&metric=security_rating)](https://sonarcloud.io/dashboard?id=janosmiko_lfk) [![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=janosmiko_lfk&metric=vulnerabilities)](https://sonarcloud.io/dashboard?id=janosmiko_lfk) [![codecov](https://codecov.io/gh/janosmiko/lfk/graph/badge.svg)](https://codecov.io/gh/janosmiko/lfk) [![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/janosmiko/lfk/badge)](https://scorecard.dev/viewer/?uri=github.com/janosmiko/lfk) [![OpenSSF Best Practices](https://www.bestpractices.dev/projects/12677/badge)](https://www.bestpractices.dev/projects/12677)[![Cloudsmith](https://img.shields.io/badge/OSS%20hosting%20by-cloudsmith-blue?logo=cloudsmith&style=flat-square&link=https%3A%2F%2Fcloudsmith.com)](https://cloudsmith.com)

**LFK** is a keyboard-focused, yazi-inspired terminal user interface for navigating and managing Kubernetes clusters. It brings a three-column Miller columns layout with an owner-based resource hierarchy to your terminal.

## Support

LFK is a side project I build in my free time, and the tools that go into it
(IDE licenses, AI assistants) are not free. If LFK saves you time and you'd like
to help cover those costs and fund continued development, consider sponsoring:

- [GitHub Sponsors](https://github.com/sponsors/janosmiko)
- [Buy Me a Coffee](https://buymeacoffee.com/janosmiko)

Every contribution is appreciated and helps make LFK sustainable.

**Thank you for your support!**

Package repository hosting is graciously provided by [Cloudsmith](https://cloudsmith.com).
Cloudsmith is the only fully hosted, cloud-native, universal package management solution, that enables your organization to create, store and share packages in any format, to any place, with total confidence.

## Screenshots

### Demo

![Demo](./docs/imgs/demo.gif)

### Themes

![Themes](./docs/imgs/themes.gif)

### Features

| Feature | |
| --- | --- |
| Pods | <img src="./docs/imgs/pods.png" alt="Pods" width="600"> |
| Pods fullscreen | <img src="./docs/imgs/pods-fullscreen.png" alt="Pods fullscreen" width="600"> |
| Pod action menu | <img src="./docs/imgs/pods-actions.png" alt="Pod actions" width="600"> |
| Cluster Dashboard | <img src="./docs/imgs/cluster-dashboard.png" alt="Cluster Dashboard" width="600"> |
| Object Explorer | <img src="./docs/imgs/object-explorer.png" alt="Object Explorer" width="600"> |
| API Explorer | <img src="./docs/imgs/api-explorer.png" alt="API Explorer" width="600"> |
| YAML Viewer | <img src="./docs/imgs/yaml-preview.png" alt="YAML Viewer" width="600"> |
| Log Viewer | <img src="./docs/imgs/log-viewer.png" alt="Log Viewer" width="600"> |
| Log Top | <img src="./docs/imgs/log-top.png" alt="Log Top" width="600"> |
| Helm integration | <img src="./docs/imgs/helm-integration.png" alt="Helm integration" width="600"> |
| ArgoCD integration | <img src="./docs/imgs/argocd-integration.png" alt="ArgoCD integration" width="600"> |
| ArgoCD AutoSync config | <img src="./docs/imgs/argocd-autosync.png" alt="ArgoCD auto-sync config" width="300"> |
| Security dashboard | <img src="./docs/imgs/security-heuristics.png" alt="Built-in security heuristics" width="600"> |
| Trivy integration | <img src="./docs/imgs/trivy-integration.png" alt="Trivy integration" width="600"> |
| Crash Investigator | <img src="./docs/imgs/crash-investigator.png" alt="Crash Investigator" width="600"> |
| Pod Startup Analysis | <img src="./docs/imgs/startup-analysis.png" alt="Pod Startup Analysis" width="600"> |
| Can-I RBAC permissions browser | <img src="./docs/imgs/can-i.png" alt="Can-I viewer" width="600"> |
| Label and annotation editor | <img src="./docs/imgs/label-annotation-editor.png" alt="Label and annotation editor" width="600"> |
| Quick filters | <img src="./docs/imgs/quick-filters.png" alt="Quick filters" width="600"> |
| Column visibility and reordering | <img src="./docs/imgs/column-visibility-reordering.png" alt="Column visibility and reordering" width="600"> |
| Bookmarks | <img src="./docs/imgs/bookmarks.png" alt="Bookmarks" width="600"> |
| Multi-tab support | <img src="./docs/imgs/tab-support.png" alt="Multi-tab support" width="600"> |
| Union view | <img src="./docs/imgs/union-sets.png" alt="Union sets" width="600"> |
| Local cluster management | <img src="./docs/imgs/local-clusters.png" alt="Local clusters" width="600"> |

## Features

### Navigation and Layout

- **Three-column Miller columns** interface (parent / current / preview)
- **Owner-based navigation**: Clusters -> Resource Types -> Resources -> Owned Resources -> Containers
- **Resource groups**: Dashboards, Workloads, Networking, Config, Storage, ArgoCD, Helm, Access Control, Cluster, Custom Resources
- **Pinned resource types**: Pin individual resource types (built-in or CRD) into a "Pinned" section at the top of the list, below the dashboards. Configurable via `pinned_types` in config (legacy `pinned_groups` also supported) or interactively with `p` key (stored per-context or per named union set)
- **CRD categories**: Discovered CRDs are grouped by API group name (e.g., `argoproj.io`, `longhorn.io`, `networking.istio.io`)
- **Hide rarely used resources**: CSI internals, admission webhooks, APF, leases, runtime classes, and uncategorized core resources are hidden by default. Press `H` to surface them (dimmed) under their categories and an "Advanced" group (resets each launch; set `show_rare_types: true` in config to show them from startup)
- **Hide individual resource types**: At the resource types level, open the action menu (`x`) on a type to hide or show it. Hidden types are saved per cluster context (or named union set) and stay hidden across restarts. Press `H` to reveal hidden types dimmed so you can un-hide them
- **Expandable/collapsible resource groups** with `z`
- **Fullscreen middle column** toggle with `Shift+F`
- **Vim-style keybindings** throughout (fully customizable via config)
- **Mouse support**: Click to navigate, scroll wheel scrolls the pane under the pointer, `Ctrl+Option+Y` toggles mouse capture, Shift+Drag for native terminal text selection

### Cluster Management

- **Multi-tab support**: Open multiple views side by side
- **Multi-cluster/multi-context support** via merged kubeconfig loading
- **Merged kubeconfig loading**: `~/.kube/config`, `~/.kube/config.d/*` (recursive, symlinks followed), `KUBECONFIG` env var, and `KUBECONFIG_DIR` env var.
- **Union view**: Merge resources from multiple clusters into a single table with a `Context` column identifying the source, via `--union-context` (repeatable) or a named `--union-set` from config. See [Union View](docs/union-context.md).
- **Cluster dashboard** when entering a context (configurable)
- **Monitoring dashboard** with active Prometheus/Alertmanager alerts (`@` key), configurable endpoints per cluster
- **API Explorer** for interactively browsing resource structure (`I` key) with recursive field browser and an ASCII-art tree view (`T`, sticky across navigation; `Space` folds branches)
- **Object Explorer** for drilling into the selected resource's live object (`O` key); arrays expand into indexed elements, so recursive status trees (e.g. `.status.steps[].steps[]`) are walkable; live-refreshes under watch mode (`w` to pause, `R` to refresh manually); `T` toggles a tree view that expands the whole subtree with ASCII-art guides, `Space` folds branches
- **Namespace selector** overlay with type-to-filter
- **All-namespaces mode** (enabled by default)
- **Local cluster management** — create, list, and delete `kind` clusters; create, list, start, stop, and delete `k3d` and `minikube` clusters (kind has no native start/stop) from inside lfk via the `Ctrl+N` manager overlay.

### Resource Operations

- **Read-only mode**: Lock a session against destructive actions (delete, edit, scale, restart, exec, port-forward, drain, etc.). Enable with `--read-only`, the `read_only: true` config field, per-context `clusters.<name>.read_only`, or the in-app `Ctrl+R` toggle (toggles the highlighted row's `[RO]` marker at the cluster picker; toggles the current tab inside a context). A `[RO]` badge in the title bar marks active sessions. See [Read-Only Mode](docs/usage.md#read-only-mode).
- **Security dashboard**: Aggregated findings from Trivy, Kyverno, Kubescape, Falco, Gatekeeper, plus two built-in zero-dependency scanners: a heuristic Pod-spec scanner and an Advisor source with reliability recommendations (missing PDBs, quotas, probes, requests — dashboard-only, never on the SEC badge). Auto-detects installed sources, shows a per-resource SEC badge, and probes lazily on first use. Enable/disable globally or per cluster via `security.enabled` / `clusters.<name>.security`. See [Security Dashboard](docs/security.md).
- **Context-aware action menus**: logs, exec, attach, debug, scale, restart, delete, describe, edit, events, port-forward, vuln scan, PVC resize
- **Custom user-defined actions**: Define custom shell commands per resource type in config
- **Multi-select with bulk actions**: Select multiple resources with Space, range-select with Ctrl+Space, perform bulk delete, scale, restart, and ArgoCD bulk sync/refresh
- **Resource sorting** by name, age, or status, remembered per resource kind for the session
- **Filter and search**: Filter with `f`, search with `/` -- supports substring, regex (auto-detected), and fuzzy (`~` prefix) modes
- **Abbreviated search**: Type `pvc`, `hpa`, `deploy` etc. to jump to resource types
- **Command bar** (`:`) with vertical dropdown autocomplete: resource jumps (`:pod`, `:dep`), built-in commands (`:ns`, `:ctx`, `:set`, `:sort`, `:export`), kubectl with `:k`/`:kubectl` prefix and flag/namespace completion, shell commands (`:!`). Value positions (namespace, context, resource name, option, column, format) accept fuzzy matches; command names stay on prefix.
- **Watch mode**: Auto-refresh resources every 2 seconds (enabled by default)
- **Owner/controller navigation**: Jump to the owner of any resource with `o`
- **Events view** with warnings-only filter toggle and duplicate-event grouping (`z`)
- **Crash Investigator** — per-Pod tabbed view combining restart history, pod-scoped events, previous/current container logs, and container-scoped `describe` — accessible from the Pod action menu (`x` → `I`). Refresh with `Shift+R`.
- **Traffic capture** — per-pod packet capture (kubectl-debug or kubeshark, auto-detected) with live decode and pcap export. Press `c` on a Pod or Service.

### Preview and Editing

- **YAML preview** in the right column with syntax highlighting
- **Full-screen YAML viewer** with scrollable output, search, section folding (`Tab`/`z`), and in-place editing
- **Resource details** summary in split preview (toggle with `Shift+P`)
- **List status summary** band pinned at the bottom of the children pane (like the resource-usage footer) when hovering a resource type in the resource-type list — always shows the resource count, plus a colored status rollup for kinds with a health signal (ArgoCD Application health/sync, Pod phase, workload ready ratios, Node readiness, Namespace/PV/PVC phase, Flux & cert-manager Ready) — plus a generic `.status.phase` or `.status.conditions` rollup for any other kind that surfaces one — so you can confirm a whole list is healthy without drilling in
- **Inline log viewer** with streaming, search, live text filter (`f`: plain/`~`fuzzy/regex/`\`literal), severity filter (`i`/`o`: step the minimum level shown — INFO+/WARN+/ERROR+), line numbers, word wrap, follow mode (`F`), timestamps toggle, previous container logs, container filter, tail-first loading, line jump, structured preview pane (`P`: parses the selected line as JSON or logfmt, falls back to plain text), and automatic reconnect across init-container transitions (stays attached as each init container finishes and the next one starts)
- **Log Top** — aggregate a resource's logs by parsed attributes (method, host, path, status, service, or JSON/logfmt keys) showing request counts, REQ/s, ERR%, 4xx/5xx counts, avg/max latency, and p50/p95/p99 percentiles (when duration data is present). Columns auto-fit the terminal width (wider = more columns, up to all of REQ, REQ/s, ERR%, P50/P95/P99, ERR, 4XX, 5XX, AVG, MAX, %); `,` toggles/reorders columns. Auto-detects Traefik JSON, ingress-nginx, Envoy, NCSA common/combined access logs (nginx, Apache, Traefik default), JSON, and logfmt. Launch from the resource action menu ("Log Top") or press `T` in the open log viewer. In view: `g` group by, `p` profile, `,` columns (show/hide and reorder), `enter` drill down, sort key to re-sort, `esc` back.
- **Inline describe view** with scrollable output
- **Secret viewing/editing** with decode toggle (`Ctrl+S`) and dedicated editor (`e`)
- **Embedded terminal** (PTY mode) for exec and shell with tab switching — PTY keeps running in background when switching tabs

### Resource Management

- **Resource templates**: Create resources from 25+ built-in templates (`a`, `/` to search); includes a Custom Resource template as a starting point
- **Port forwarding** from the action menu (with local port setting and browser open); manage active forwards via the Networking group
- **Network policy visualizer**: Visualize a NetworkPolicy's ingress/egress rules (`x` → `N` on a NetworkPolicy), or list every policy affecting a Pod or Service (`x` → "Network Policies" on the resource) — including which backing pods a policy covers. Supports CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy (entities, FQDNs, deny rules, L7 indicators) when the Cilium CRDs are installed. The dialog scrolls with the mouse wheel and is searchable (`/`, `n`/`N`)
- **Clipboard support**: Copy resource name (`y`), open copy-as picker (`Y`: YAML / JSON / Table), paste/apply from clipboard (`Ctrl+P`), paste into search/filter boxes (`Cmd+V` / `Ctrl+Shift+V`)
- **Bookmarks**: Save favorite resource paths (including the active list filter) for quick navigation
- **Orphan detection**: Press `Shift+Z` (or run bare `:orphans`) to open the cluster-wide orphan overview across 11 kinds — Pods, Secrets, ConfigMaps, Services, PVCs, HPAs, PDBs, NetworkPolicies, Roles, ClusterRoles, RoleBindings, ClusterRoleBindings. Per-list filters are still available via the filter-preset overlay (`.`) on each kind, or jump straight to a filtered view with `:orphans <kind>` (e.g., `:orphans secrets`). A strict / lenient toggle (`s`) flips between "truly unused" and "currently idle but referenced by workload templates" (e.g. CronJob between firings). Auto-excludes Helm release Secrets, ServiceAccount tokens, owner-managed resources, and `kube-root-ca.crt`.
- **Session persistence**: Remembers last context/namespace/resource, the active list filter, and the highlighted row across restarts

### Integrations

- **ArgoCD integration**: Browse Applications, sync, terminate sync, refresh, view managed resources
- **Argo Workflows integration**: Suspend/resume, stop/terminate, resubmit Workflows; submit from WorkflowTemplates; suspend/resume CronWorkflows
- **Helm integration**: Browse releases, view managed resources, uninstall
- **HPA scaling**: Press `S` on a HorizontalPodAutoscaler to edit its min/max replica bounds and scale its target workload (Deployment / StatefulSet / ReplicaSet) in one overlay
- **KEDA integration**: Pause/unpause ScaledObjects and ScaledJobs
- **External Secrets integration**: Force refresh ExternalSecrets, ClusterExternalSecrets, and PushSecrets
- **CRD discovery**: Automatically discovers installed CRDs and groups them by API group

### Customization

- **460+ built-in color schemes** from [ghostty themes](https://github.com/ghostty-org/ghostty): Tokyonight, Catppuccin, Dracula, Nord, Rose Pine, Gruvbox, and many more. Transparent background support.
- **Runtime theme switching**: Press `T` to preview and switch themes without restarting
- **Auto dark/light mode**: configure a dark and a light scheme; lfk switches automatically when the OS appearance changes (requires CSI 996/2031 terminal support: Ghostty, kitty, Contour, …)
- **Custom color themes** via config file (Tokyonight theme by default)
- **Configurable keybindings** for direct actions
- **Configurable search abbreviations**
- **Configurable filter presets** per resource type (extend built-in quick filters with `.`)
- **Configurable icon modes**: `auto` (default, detects Nerd Font-capable terminals like Ghostty/Kitty/WezTerm), `unicode`, `nerdfont` (Material Design Icons), `simple` (ASCII labels), `emoji`, or `none`. Override at runtime with the `LFK_ICONS` environment variable.
- **Configurable table columns** (global, per-resource-type, and per-cluster)
- **Column visibility toggle** overlay to show/hide and reorder columns at runtime (`,` key)
- **Startup tips**: Random tips on startup to help discover features (configurable via `tips: false`)
- **Status-aware coloring**: Running=green, Pending=yellow, Failed=red
- **Resource usage metrics**: CPU/MEM with color-coded bars in dashboard

## Installation

```bash
# Homebrew (macOS / Linux)
brew install janosmiko/tap/lfk

# Windows: Scoop
scoop bucket add janosmiko https://github.com/janosmiko/scoop-bucket && scoop install lfk
# or: winget install janosmiko.lfk
# or: choco install lfk

# Linux: Arch / AUR
yay -S lfk-bin

# Linux: Debian / Ubuntu (Cloudsmith APT)
curl -1sLf 'https://dl.cloudsmith.io/public/janosmiko/lfk/setup.deb.sh' | sudo -E bash && sudo apt update && sudo apt install lfk

# Linux: Fedora / RHEL (Cloudsmith DNF)
curl -1sLf 'https://dl.cloudsmith.io/public/janosmiko/lfk/setup.rpm.sh' | sudo -E bash && sudo dnf install lfk

# Go
go install github.com/janosmiko/lfk@latest

# Nix
nix run github:janosmiko/lfk
```

`kubectl` is required and must be configured. `helm` and `trivy` are optional (Helm management, image vulnerability scanning).

> See [docs/installation.md](docs/installation.md) for Windows, Docker, NixOS/home-manager flake input, building from source, and the full list of optional CLI dependencies.

## Usage

```bash
# Use default kubeconfig (~/.kube/config + ~/.kube/config.d/*)
lfk

# Start in a specific context / namespace
lfk --context my-cluster -n kube-system

# Use a specific kubeconfig
lfk --kubeconfig /path/to/kubeconfig
KUBECONFIG=/path/to/config1:/path/to/config2 lfk

# Use a custom directory for kubeconfigs (repeat the flag for multiple)
lfk --kubeconfig-dir /path/to/configs/
lfk --kubeconfig-dir /team-a/configs --kubeconfig-dir /team-b/configs
KUBECONFIG_DIR=/path/to/configs/ lfk
KUBECONFIG_DIR=/team-a/configs:/team-b/configs lfk
```

> See [docs/usage.md](docs/usage.md) for the full CLI reference and runtime tuning options: mouse capture, no-color mode, read-only mode, watch-mode interval, discovery cache (`KUBECACHEDIR`), and Secret lazy loading.

## Navigation Hierarchy

```
Clusters (kubeconfig contexts)
  +-- Resource Types (grouped: Workloads, Networking, Config, Storage, ArgoCD, Helm, ...)
        +-- Resources (e.g., individual Deployments)
              +-- Owned Resources (Pods via ownerReferences, Jobs for CronJobs, etc.)
                    +-- Containers (for Pods)
```

Namespaces are **not** a navigation level. The current namespace is shown in the top-right corner and can be changed by pressing `\`. All-namespaces mode is enabled by default (toggle with `A`). Inside the namespace selector, press `Space` to include namespaces, `Tab` to exclude them (negative selection — shows all except the marked namespaces, each prefixed with `!`), `A` to reset to all-namespaces mode, or `R` to refresh the list from the cluster. Press `.` to quick-filter straight to the namespace of the resource highlighted behind the overlay.

### Owner Resolution

- **Deployments** show their Pods (resolved through ReplicaSets, flattened)
- **StatefulSets / DaemonSets / Jobs** show their Pods directly
- **CronJobs** show their Jobs
- **Services** show Pods matching the service selector
- **ArgoCD Applications** show managed resources (from status or label discovery)
- **Helm Releases** show managed resources (via `app.kubernetes.io/instance` label)
- **Pods** show their Containers
- **ConfigMaps / Secrets / Ingresses / PVCs** show details preview (no children)

## Keybindings

> For the complete keybinding reference (YAML view, log viewer, describe, diff, exec mode, and all sub-modes), see [docs/keybindings.md](docs/keybindings.md). Press `?` or `F1` in-app for the built-in help screen.

### Navigation

| Key | Action |
|---|---|
| `h` / `Left` | Navigate to parent level |
| `l` / `Right` | Navigate into selected item |
| `j` / `Down` | Move cursor down |
| `k` / `Up` | Move cursor up |
| `gg` / `Home` | Jump to top of list |
| `G` / `End` | Jump to bottom of list |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Half-page scroll down/up |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Full-page scroll down/up |
| `Enter` | Open full-screen YAML view / navigate into |
| `z` | Toggle expand/collapse all resource groups / toggle event grouping (Events view) |
| `p` | Pin/unpin resource type (at resource types level) |
| `x` | At resource types level: pin/unpin or hide/show the selected resource type (saved per cluster context / union set) |
| `H` | Toggle rarely used + hidden resource types (CSI internals, webhooks, APF, leases, advanced core); revealed types shown dimmed |
| `0` / `1` / `2` | Jump to clusters / types / resources level |
| `J` / `K` | Scroll preview pane down/up |
| `o` | Jump to owner/controller of selected resource |
| `Backspace` | Jump back through teleport history (owner / port-forward / orphan / finding / mark jumps) |
| `g`+`p/d/s/n/N/i/j/c/r/D/t/C/S/h/v/V/b` | Goto resource type: Pods / Deployments / Services / Nodes / Namespaces / Ingresses / Jobs / CronJobs / ReplicaSets / DaemonSets / StatefulSets / ConfigMaps / Secrets / HPAs / PVCs / PVs / PDBs (press `g` for which-key popup; add CRDs like ArgoCD via `goto_targets`) |

### Views and Modes

| Key | Action |
|---|---|
| `?` | Toggle help screen |
| `f` | Filter items in current view |
| `/` | Search and jump to match |
| `n` / `N` | Next / previous search match |
| `P` | Toggle between details and YAML preview |
| `M` | Toggle resource relationship map |
| `F` | Toggle fullscreen (middle column or dashboard) |
| `\|` | Hide / show the left resource-type sidebar |
| `.` | Quick filter presets |
| `!` | Error log (V/v select, y copy, f fullscreen) |
| `Ctrl+S` | Toggle secret value visibility |
| `Ctrl+G` | Finalizer search and remove |
| `I` | API Explorer (browse resource structure interactively) |
| `O` | Object Explorer (browse the selected resource's live object as a drill-in tree) |
| `U` | RBAC permissions browser (can-i) |
| `T` | Open theme selector |
| `:` | Command bar: resource jumps (`:pod`, `:dep`), built-ins (`:ns`, `:ctx`, `:set`, `:sort`, `:export`), kubectl (`:k get pod`), shell (`:! cmd`) |
| `w` | Toggle watch mode (auto-refresh) |
| `,` | Column visibility toggle (show/hide and reorder columns) |
| `>` / `<` | Sort by next / previous column |
| `=` | Toggle sort direction (ascending/descending) |
| `-` | Reset sort to default (Name ascending) |
| `W` | Save resource to file / toggle warnings-only (Events) |
| `Ctrl+T` | Cycle terminal mode (pty / exec / mux — mux skipped without tmux/zellij) |
| `@` | Monitoring overview (active Prometheus alerts) |
| `Q` | Namespace resource quota dashboard |

### Actions

| Key | Action |
|---|---|
| `x` | Action menu (logs, exec, describe, edit, delete, scale, port-forward, etc.) |
| `\` / `A` | Namespace selector / toggle all-namespaces |
| `L` | Toggle live-log preview pane (right pane, streaming tail; pod and container rows) |
| `Ctrl+L` | Open fullscreen log viewer |
| `v` | Describe resource |
| `D` / `X` | Delete / force delete |
| `y` / `Y` | Copy name / open copy-as picker (YAML / JSON / Table) |
| `Space` | Toggle multi-selection (bulk actions via `x`) |
| `m<slot>` / `'<slot>` | Set / jump to bookmark (lowercase = context-aware, uppercase = context-free) |
| `t` / `]` / `[` | New tab / next / previous |

All views (YAML, logs, describe, diff, exec) use vim-style navigation (`j`/`k`, `gg`/`G`, `Ctrl+D`/`Ctrl+U`, `/` search, `v`/`V` visual selection). See [docs/keybindings.md](docs/keybindings.md) for the full reference.

> For the complete command bar reference (built-in commands, shell/kubectl execution, resource jumps), see [docs/commands.md](docs/commands.md).

## Configuration

Create `~/.config/lfk/config.yaml` to customize the application. All fields are optional; only the values you specify will override the defaults.

> For the complete configuration reference, see [docs/config-reference.md](docs/config-reference.md) and [docs/config-example.yaml](docs/config-example.yaml).

### Quick Start

```yaml
# Color scheme (press T in-app to browse 460+ themes with live preview)
# Auto dark/light mode — Ghostty-style "dark:X,light:Y" syntax switches the
# scheme when the OS appearance changes (CSI 996/2031; Ghostty, kitty >= 0.27, …)
colorscheme: "dark:catppuccin-mocha,light:catppuccin-latte"

# Use terminal's own background
transparent_background: true

# Icon mode: "auto" (default, detects Nerd Font terminals like Ghostty/Kitty/WezTerm),
# "unicode", "nerdfont" (requires Nerd Font in terminal), "simple" (ASCII labels),
# "emoji", or "none". The LFK_ICONS env var overrides this setting.
icons: auto

# Disable mouse capture (allows native terminal text selection)
mouse: false

# Custom keybinding overrides (only specify what you want to change)
keybindings:
  logs: "ctrl+l"
  toggle_preview_logs: "L"
  describe: "v"
  delete: "D"

# Search abbreviations (extend built-in abbreviations for :pod, :dep, etc.)
abbreviations:
  myapp: myapplications
```

### Search Modes

All search and filter inputs support three modes, auto-detected from the query string:

| Mode | Syntax | Example |
|---|---|---|
| Substring | plain text | `nginx` |
| Regex | auto-detected | `err[0-9]+` |
| Fuzzy | `~` prefix | `~deplymnt` |
| Literal | `\` prefix | `\err.*` |

**Clipboard paste**: All search, filter, and command bar inputs accept pasted text (`Cmd+V` on macOS, `Ctrl+Shift+V` on Linux). Multiline paste shows a confirmation dialog.

**Recall previous queries**: While the `f` filter or `/` search input is open, press `Up` / `Down` to cycle through previous queries. `/` and `f` share one history (the matcher and matched fields are identical between them), kept separate from the `:` command bar. The log viewer's `/` search has its own history because it matches raw log lines (substring/regex over arbitrary text) rather than resource names — pooling it would surface irrelevant entries. All three persist across sessions under `$XDG_STATE_HOME/lfk/` (default `~/.local/state/lfk/`) — `query-history` for explorer `/` and `f`, `log-search-history` for the log viewer's `/`, and `history` for the command bar.

## Tips and Tricks

- Peek at Pod/Deployment logs with `L` (live-log preview pane), or open the fullscreen log viewer with `Ctrl+L`
- Jump straight to a resource type from anywhere: type `:pod`, `:dep`, `:pvc` in the command bar
- Press `o` on a resource to jump to its owner (e.g. Pod -> Deployment), then `Backspace` to jump back
- Typos are fine in search: `/~deplymnt` fuzzy-matches `deployments`
- Multi-select with `Space` (range-select with `Ctrl+Space`), then bulk delete/scale/restart via `x`
- Set a bookmark with `m<letter>`, jump back with `'<letter>` - lowercase slots are context-aware. Press tab to jump to the namespace. An active list filter is saved with the bookmark and reapplied on jump (shown as `> /<filter>` in the slot name)
- Press `.` for quick filter presets (e.g. only failing Pods); extend them per resource type in config
- Decode Secret values in the preview with `Ctrl+S`, or edit them decoded with `e`
- Copy the resource name with `y`; press `Y` to copy as YAML, JSON, or Table
- Apply a manifest straight from your clipboard with `Ctrl+P`
- Hunt down unused ConfigMaps, Secrets, PVCs and more with `Shift+Z` (orphan detection)
- Run kubectl without leaving lfk (`:k get pods -o wide`) or any shell command (`:! curl ...`)
- Investigate a crash-looping Pod with `x` -> `I`: restart history, events, previous logs, and describe in one tabbed view
- Lock a session against destructive actions with `Ctrl+R` (read-only mode)
- Try a new look without restarting: `T` live-previews 460+ themes
- Resource stuck in Terminating? `Ctrl+G` searches its finalizers and removes them
- Press `Tab` inside `/` or `f` to broaden matching to labels, annotations, finalizers, and other column values
- Recall earlier queries with `Up`/`Down` inside `/`, `f`, and the `:` command bar - history persists across restarts
- See how a resource connects to everything else with `M` (relationship map)
- Need everything except a few namespaces? In the `\` selector, `Tab` excludes namespaces instead of selecting them
- Pin your daily-driver resource types with `p` and hide noisy ones via `x` - both remembered per cluster
- Teleport between levels with `0` / `1` / `2` (clusters / resource types / resources)
- Sort by any column with `>` / `<`, flip direction with `=`, reset with `-`
- Check firing Prometheus/Alertmanager alerts with `@` and namespace quotas with `Q`
- Open an Ingress host - or an active port-forward's localhost URL - in your browser with `Ctrl+O`; on a Service, `Ctrl+O` starts a port forward and opens it
- Save the selected resource manifest to a file with `W`
- Spin up a throwaway kind/k3d/minikube cluster without leaving lfk: `Ctrl+N` at the cluster list
- Capture a Pod's network traffic with `c` - live decode plus pcap export (kubectl-debug or kubeshark)
- Walk any resource's live object with `O` (Object Explorer): `r` finds keys recursively, `T` expands an ASCII tree, `y` copies the field path
- Forget `kubectl explain` - `I` opens the API Explorer, and `n`/`N` searches auto-drill into nested fields
- In the YAML viewer, press `O` on a line to jump into the Object Explorer at that attribute, or `I` to see its schema
- Fold YAML sections with `z` (`Z` folds all); edit the resource in your `$EDITOR` with `Ctrl+E`
- Every viewer speaks vim: counts (`100j`, `42G`, `5n`), visual selections (`v` / `V` / `Ctrl+V`), and text objects (`viw`) work everywhere
- Make noisy logs readable with `P` - the structured preview parses JSON, logfmt, klog, zap, nginx, envoy, Java, and postgres lines
- Save logs to a file with `S` (loaded lines) or `Ctrl+S` (full history) - the path lands on your clipboard
- Switch pods or filter containers without leaving the log viewer: press `\`
- Replay a resource's event history as a timeline with `V`
- Flip the RBAC question: inside the Can-I browser (`U`), `Tab` opens Who-Can - every subject allowed to run a verb on a resource
- Get per-container CPU/memory recommendations with `x` -> `z` (Right-sizing Advisor, VPA-backed when available)
- Watch an ArgoCD Application roll out wave by wave: `x` -> `W` opens the Sync Wave Timeline
- Waiting for a rollout? `:nyan` and `:kubetris` are real commands

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) for prerequisites, development setup, build/test commands, project layout, and the PR submission flow.

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.

## Star History

<a href="https://www.star-history.com/?repos=janosmiko%2Flfk&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=janosmiko/lfk&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=janosmiko/lfk&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=janosmiko/lfk&type=date&legend=top-left" />
 </picture>
</a>
