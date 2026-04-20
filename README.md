# :zap: LFK - Lightning Fast Kubernetes navigator

[![Release](https://img.shields.io/github/v/release/janosmiko/lfk)](https://github.com/janosmiko/lfk/releases) [![CI](https://img.shields.io/github/actions/workflow/status/janosmiko/lfk/ci.yml?branch=main&label=CI)](https://github.com/janosmiko/lfk/actions/workflows/ci.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/janosmiko/lfk)](https://goreportcard.com/report/github.com/janosmiko/lfk) [![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=janosmiko_lfk&metric=security_rating)](https://sonarcloud.io/dashboard?id=janosmiko_lfk) [![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=janosmiko_lfk&metric=vulnerabilities)](https://sonarcloud.io/dashboard?id=janosmiko_lfk) [![codecov](https://codecov.io/gh/janosmiko/lfk/graph/badge.svg)](https://codecov.io/gh/janosmiko/lfk)

**LFK** is a lightning-fast, keyboard-focused, yazi-inspired terminal user interface for navigating and managing Kubernetes clusters. Built for speed and efficiency, it brings a three-column Miller columns layout with an owner-based resource hierarchy to your terminal.

## Screenshots

### Demo

![Demo](./docs/imgs/demo.gif)

### Themes

![Themes](./docs/imgs/themes.gif)

### Pods

![Pods](./docs/imgs/pods.png)

### Pods fullscreen

![Pods fullscreen](./docs/imgs/pods-fullscreen.png)

### Helm integration

![Helm integration](./docs/imgs/helm-integration.png)

### ArgoCD integration

![ArgoCD integration](./docs/imgs/argocd-integration.png)

![ArgoCD auto-sync config](./docs/imgs/argocd-autosync.png)

### ConfigMap and Secret editors

![ConfigMap editor](./docs/imgs/configmap-editor.png)

### Label and annotation editor

![Label editor](./docs/imgs/label-editor.png)

### Can-I RBAC permissions browser

![Can-I viewer](./docs/imgs/can-i.png)

### YAML preview

![Yaml preview](./docs/imgs/yaml-preview.png)

### API Explorer

![API Explorer](./docs/imgs/api-explorer.png)

## Features

### Navigation and Layout

- **Three-column Miller columns** interface (parent / current / preview)
- **Owner-based navigation**: Clusters -> Resource Types -> Resources -> Owned Resources -> Containers
- **Resource groups**: Dashboards, Workloads, Networking, Config, Storage, ArgoCD, Helm, Access Control, Cluster, Custom Resources
- **Pinned CRD groups**: Pin frequently used CRD API groups so they appear after built-in categories. Configurable via `pinned_groups` in config or interactively with `p` key (stored per-context)
- **CRD categories**: Discovered CRDs are grouped by API group name (e.g., `argoproj.io`, `longhorn.io`, `networking.istio.io`)
- **Hide rarely used resources**: CSI internals, admission webhooks, APF, leases, runtime classes, and uncategorized core resources are hidden by default. Press `H` to surface them under their categories and an "Advanced" group (resets each launch)
- **Expandable/collapsible resource groups** with `z`
- **Fullscreen middle column** toggle with `Shift+F`
- **Vim-style keybindings** throughout (fully customizable via config)
- **Mouse support**: Click to navigate, scroll wheel to move, Shift+Drag for native terminal text selection

### Cluster Management

- **Multi-tab support**: Open multiple views side by side
- **Multi-cluster/multi-context support** via merged kubeconfig loading
- **Merged kubeconfig loading**: `~/.kube/config`, `~/.kube/config.d/*` (recursive, symlinks followed), and `KUBECONFIG` env var
- **Cluster dashboard** when entering a context (configurable)
- **Monitoring dashboard** with active Prometheus/Alertmanager alerts (`@` key), configurable endpoints per cluster
- **API Explorer** for interactively browsing resource structure (`I` key) with recursive field browser
- **Namespace selector** overlay with type-to-filter
- **All-namespaces mode** (enabled by default)

### Resource Operations

- **Context-aware action menus**: logs, exec, attach, debug, scale, restart, delete, describe, edit, events, port-forward, vuln scan, PVC resize
- **Custom user-defined actions**: Define custom shell commands per resource type in config
- **Multi-select with bulk actions**: Select multiple resources with Space, range-select with Ctrl+Space, perform bulk delete, scale, restart, and ArgoCD bulk sync/refresh
- **Resource sorting** by name, age, or status
- **Filter and search**: Filter with `f`, search with `/` -- supports substring, regex (auto-detected), and fuzzy (`~` prefix) modes
- **Abbreviated search**: Type `pvc`, `hpa`, `deploy` etc. to jump to resource types
- **Command bar** (`:`) with vertical dropdown autocomplete: resource jumps (`:pod`, `:dep`), built-in commands (`:ns`, `:ctx`, `:set`, `:sort`, `:export`), kubectl with `:k`/`:kubectl` prefix and flag/namespace completion, shell commands (`:!`)
- **Watch mode**: Auto-refresh resources every 2 seconds (enabled by default)
- **Owner/controller navigation**: Jump to the owner of any resource with `o`
- **Events view** with warnings-only filter toggle and duplicate-event grouping (`z`)

### Preview and Editing

- **YAML preview** in the right column with syntax highlighting
- **Full-screen YAML viewer** with scrollable output, search, section folding (`Tab`/`z`), and in-place editing
- **Resource details** summary in split preview (toggle with `Shift+P`)
- **Inline log viewer** with streaming, search, **persistent filter** with severity floor, exclude rules, and **boolean group expressions** (`(foo AND bar)`, `(foo AND (bar OR baz))`), **quick severity cycle** (`>` / `<`), **time-range picker** (Start + optional End; presets such as Last 5m / Last 24h / Today / Yesterday, plus a Relative spinner and Absolute datetime editor for custom windows), line numbers, word wrap, follow mode, timestamps toggle, previous container logs, container filter, tail-first loading, and line jump
- **Inline describe view** with scrollable output
- **Secret viewing/editing** with decode toggle (`Ctrl+S`) and dedicated editor (`e`)
- **Embedded terminal** (PTY mode) for exec and shell with tab switching — PTY keeps running in background when switching tabs

### Resource Management

- **Resource templates**: Create resources from 25+ built-in templates (`a`, `/` to search); includes a Custom Resource template as a starting point
- **Port forwarding** from the action menu (with local port setting and browser open); manage active forwards via the Networking group
- **Clipboard support**: Copy resource name (`y`), YAML (`Y`), paste/apply from clipboard (`Ctrl+P`), paste into search/filter boxes (`Cmd+V` / `Ctrl+Shift+V`)
- **Bookmarks**: Save favorite resource paths for quick navigation
- **Session persistence**: Remembers last context/namespace/resource across restarts
- **Command bar**: Press `:` for shell/kubectl commands with autocompletion

### Security findings browser

- **Grouped findings**: Press `#` to jump to the Security category. Findings are grouped by check, CVE, or rule name. Each group shows severity, affected resource count, and category. Drill into a group to see affected resources.
- **Five security sources**: Heuristic (built-in pod security checks), Trivy Operator (CVE and misconfiguration scanning via CRDs), Kyverno (policy violations via PolicyReports), Falco (runtime security events), CIS (placeholder for kube-bench).
- **Hierarchical navigation**: Security > Source > Finding Group > Affected Resources. From an affected resource, press `Enter` or `l` to jump to the corresponding Kubernetes resource in the explorer.
- **SEC column badge**: Workload rows (Pods, Deployments, etc.) display a severity badge in the Name column. The badge aggregates both direct findings (e.g., heuristic checks on the Pod) and owner findings (e.g., Trivy results targeting the parent Deployment).
- **Per-resource drill-in**: Press `x` on any resource, then `y` to jump to security findings for that resource. When multiple sources have results, a source picker overlay appears.
- **Color-coded severity**: CRIT=red, HIGH=orange, MED=yellow, LOW=green in the Severity column.
- **Refresh**: Press `R` to invalidate the security cache and re-fetch findings from all enabled sources.

### Integrations

- **ArgoCD integration**: Browse Applications, sync, terminate sync, refresh, view managed resources
- **Argo Workflows integration**: Suspend/resume, stop/terminate, resubmit Workflows; submit from WorkflowTemplates; suspend/resume CronWorkflows
- **Helm integration**: Browse releases, view managed resources, uninstall
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

### Homebrew (macOS / Linux)

```bash
brew install janosmiko/tap/lfk
```

### Binary releases

Download pre-built binaries from the [GitHub Releases](https://github.com/janosmiko/lfk/releases) page.

### From source

```bash
go install github.com/janosmiko/lfk@latest
```

### Build from source

```bash
git clone https://github.com/janosmiko/lfk.git
cd lfk
go build -o lfk .
```

### Docker

```bash
docker run -it --rm \
  -v ~/.kube:/home/lfk/.kube:ro \
  janosmiko/lfk
```

To use a specific kubeconfig:

```bash
docker run -it --rm \
  -v /path/to/kubeconfig:/home/lfk/.kube/config:ro \
  janosmiko/lfk
```

For port forwarding, add `--net=host`:

```bash
docker run -it --rm \
  --net=host \
  -v ~/.kube:/home/lfk/.kube:ro \
  janosmiko/lfk
```

### Nightly Builds

Nightly builds track the latest development work and are published as GitHub
pre-releases with every `v*-nightly*` tag.

**Homebrew:**

```bash
brew install janosmiko/tap/lfk-nightly
```

**Docker:**

```bash
# Latest nightly
docker run -it --rm \
  -v ~/.kube:/home/lfk/.kube:ro \
  janosmiko/lfk:nightly

# Specific nightly date
docker run -it --rm \
  -v ~/.kube:/home/lfk/.kube:ro \
  janosmiko/lfk:nightly-20260414
```

**Binary releases:**

Download from [GitHub Releases](https://github.com/janosmiko/lfk/releases) (look for pre-release tags).

### External Dependencies

**Required:**
- `kubectl` - Kubernetes CLI (must be configured and in PATH)

**Optional** (needed only for specific features):
| Command | Feature |
|---------|---------|
| `helm` | Helm release management (values, diff, upgrade, rollback, uninstall) |
| `trivy` | Container image vulnerability scanning ([install](https://aquasecurity.github.io/trivy)) |

All other features (KEDA, External Secrets, Argo Workflows, cert-manager, ArgoCD, FluxCD, PVC resize, etc.) use the Kubernetes API directly and require no additional CLI tools.

## Usage

```bash
# Use default kubeconfig (~/.kube/config + ~/.kube/config.d/*)
lfk

# Start in a specific context
lfk --context my-cluster

# Start in a specific namespace (disables all-namespaces mode)
lfk -n kube-system

# Start with multiple namespaces selected
lfk -n default -n kube-system

# Combine context and namespace
lfk --context production -n monitoring

# Use a specific config file (overrides ~/.config/lfk/config.yaml)
lfk -c /path/to/config.yaml
lfk --config /path/to/config.yaml

# Use a specific kubeconfig file (overrides default discovery)
lfk --kubeconfig /path/to/kubeconfig

# Disable mouse capture (enables native terminal text selection)
lfk --no-mouse

# Disable all colors (selection stays visible via bold/reverse video)
lfk --no-color
# Equivalent environment variable (https://no-color.org):
NO_COLOR=1 lfk

# Override the watch-mode polling interval (default 2s; clamped to [500ms, 10m])
lfk --watch-interval 5s

# Use a specific kubeconfig via environment variable
KUBECONFIG=/path/to/config lfk

# Use multiple kubeconfigs via environment variable
KUBECONFIG=/path/to/config1:/path/to/config2 lfk
```

When `--context` or `--namespace` flags are provided, the saved session state is
ignored and the app opens directly in the specified context/namespace. The user
can still change the namespace during the session.

### Mouse Support

By default, lfk captures mouse input for click navigation, scroll, and tab
switching. If you need native terminal text selection (e.g., shift+click to
select text), you can disable mouse capture:

- **CLI flag:** `lfk --no-mouse`
- **Config file:** Add `mouse: false` to `~/.config/lfk/config.yaml`

> **Note:** macOS Terminal.app does not support shift+click text selection while
> mouse capture is active. Use `--no-mouse` or switch to a terminal that handles
> this correctly (iTerm2, Kitty, Alacritty, WezTerm, Ghostty).

### No-Color Mode

Disable all foreground and background colors while keeping selection and
other highlights visible via bold, underline, and reverse-video SGR codes.
Useful for monochrome terminals, piped output, or lower CPU usage.

- **CLI flag:** `lfk --no-color`
- **Environment variable:** `NO_COLOR=1 lfk` (any non-empty value; see
  [no-color.org](https://no-color.org/))
- **Config file:** Add `no_color: true` to `~/.config/lfk/config.yaml`

Precedence: `--no-color` flag > `NO_COLOR` env var > config file.

### Watch-Mode Interval

Watch mode (toggle with `w`) polls the current resource list on an interval.
The default 2-second interval is a good balance between freshness and API
load. Tune it with:

- **CLI flag:** `lfk --watch-interval 5s` (accepts Go durations: `500ms`,
  `2s`, `1m`, ...)
- **Config file:** Add `watch_interval: 5s` to `~/.config/lfk/config.yaml`

Values outside `[500ms, 10m]` are clamped to the bounds; invalid values fall
back to 2s.

## Navigation Hierarchy

```
Clusters (kubeconfig contexts)
  +-- Resource Types (grouped: Workloads, Networking, Config, Storage, ArgoCD, Helm, ...)
        +-- Resources (e.g., individual Deployments)
              +-- Owned Resources (Pods via ownerReferences, Jobs for CronJobs, etc.)
                    +-- Containers (for Pods)
```

Namespaces are **not** a navigation level. The current namespace is shown in the top-right corner and can be changed by pressing `\`. All-namespaces mode is enabled by default (toggle with `A`).

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
| `gg` | Jump to top of list |
| `G` | Jump to bottom of list |
| `Ctrl+D` / `Ctrl+U` | Half-page scroll down/up |
| `Ctrl+F` / `Ctrl+B` | Full-page scroll down/up |
| `Enter` | Open full-screen YAML view / navigate into |
| `z` | Toggle expand/collapse all resource groups / toggle event grouping (Events view) |
| `p` | Pin/unpin CRD group (at resource types level) |
| `H` | Toggle rarely used resource types (CSI internals, webhooks, leases, advanced core) in the sidebar |
| `0` / `1` / `2` | Jump to clusters / types / resources level |
| `J` / `K` | Scroll preview pane down/up |
| `o` | Jump to owner/controller of selected resource |

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
| `.` | Quick filter presets |
| `!` | Error log (V/v select, y copy, f fullscreen) |
| `Ctrl+S` | Toggle secret value visibility |
| `Ctrl+G` | Finalizer search and remove |
| `I` | API Explorer (browse resource structure interactively) |
| `U` | RBAC permissions browser (can-i) |
| `T` | Open theme selector |
| `:` | Command bar: resource jumps (`:pod`, `:dep`), built-ins (`:ns`, `:ctx`, `:set`, `:sort`, `:export`), kubectl (`:k get pod`), shell (`:! cmd`) |
| `w` | Toggle watch mode (auto-refresh) |
| `,` | Column visibility toggle (show/hide and reorder columns) |
| `>` / `<` | Sort by next / previous column |
| `=` | Toggle sort direction (ascending/descending) |
| `-` | Reset sort to default (Name ascending) |
| `W` | Save resource to file / toggle warnings-only (Events) |
| `Ctrl+T` | Toggle terminal mode (pty embedded / exec takeover) |
| `@` | Monitoring overview (active Prometheus alerts) |
| `Q` | Namespace resource quota dashboard |

### Actions

| Key | Action |
|---|---|
| `x` | Action menu (logs, exec, describe, edit, delete, scale, port-forward, etc.) |
| `\` / `A` | Namespace selector / toggle all-namespaces |
| `L` | View logs |
| `v` | Describe resource |
| `D` / `X` | Delete / force delete |
| `y` / `Y` | Copy name / YAML to clipboard |
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
  logs: "L"
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

## Contributing

Contributions are welcome! Here is how to get started.

### Prerequisites

- Go 1.26.2 or later
- Access to a Kubernetes cluster (for testing)
- `kubectl` configured and working
- `golangci-lint` ([install](https://golangci-lint.run/welcome/install/))

### Development Setup

```bash
# Clone the repository
git clone https://github.com/janosmiko/lfk.git
cd lfk

# Set up git hooks and install dependencies
make setup
go mod download

# Build the binary
make build

# Run it
./lfk
```

### Building and Testing

```bash
# Build
go build -o lfk .

# Run tests (if available)
go test ./...

# Run with race detector
go build -race -o lfk . && ./lfk

# Lint (if you have golangci-lint installed)
golangci-lint run
```

### Project Structure

The application follows a standard Go project layout:

- `main.go` - Entry point, initializes the Kubernetes client, loads config, and starts the Bubbletea program
- `internal/app/` - Core application logic: the Bubbletea model, update loop, async commands, and bookmarks
- `internal/k8s/` - Kubernetes client wrapper handling API calls, owner resolution, and CRD discovery
- `internal/model/` - Shared types, actions, navigation state, and resource templates
- `internal/ui/` - All rendering: columns, overlays, styles, themes, help screen, and log viewer
- `internal/logger/` - Application logging

### Submitting Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Ensure the project builds cleanly (`go build ./...`)
5. Commit your changes with a descriptive message
6. Push to your fork and open a Pull Request

## Support

If you find lfk useful and want to support its development:

- [GitHub Sponsors](https://github.com/sponsors/janosmiko)
- [Buy Me a Coffee](https://buymeacoffee.com/janosmiko)

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
