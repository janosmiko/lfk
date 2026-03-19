# :zap: LFK - Lightning Fast Kubernetes navigator

**LFK** is a lightning-fast, keyboard-focused, yazi-inspired terminal user interface for navigating and managing Kubernetes clusters. Built for speed and efficiency, it brings a three-column Miller columns layout with an owner-based resource hierarchy to your terminal.

![Demo](./docs/imgs/demo.gif)

> **Disclaimer**: This project is largely vibe coded with AI assistance. While it works well for daily Kubernetes operations, please review the code and use it at your own risk. Contributions and bug reports are welcome!

[![GitHub Sponsors](https://img.shields.io/github/sponsors/janosmiko?style=flat&logo=githubsponsors&label=Sponsors)](https://github.com/sponsors/janosmiko)
[![Buy Me a Coffee](https://img.shields.io/badge/Buy%20Me%20a%20Coffee-support-yellow?style=flat&logo=buymeacoffee)](https://buymeacoffee.com/janosmiko)

## Features

### Navigation and Layout

- **Three-column Miller columns** interface (parent / current / preview)
- **Owner-based navigation**: Clusters -> Resource Types -> Resources -> Owned Resources -> Containers
- **Resource groups**: Dashboards, Workloads, Networking, Config, Storage, ArgoCD, Helm, Access Control, Cluster, Custom Resources
- **Pinned CRD groups**: Pin frequently used CRD API groups so they appear after built-in categories. Configurable via `pinned_groups` in config or interactively with `p` key (stored per-context)
- **CRD categories**: Discovered CRDs are grouped by API group name (e.g., `argoproj.io`, `longhorn.io`, `networking.istio.io`)
- **Expandable/collapsible resource groups** with `z`
- **Fullscreen middle column** toggle with `Shift+F`
- **Vim-style keybindings** throughout (fully customizable via config)
- **Mouse support**: Click to navigate, scroll wheel to move, Shift+Drag for native terminal text selection

### Cluster Management

- **Multi-tab support**: Open multiple views side by side
- **Multi-cluster/multi-context support** via merged kubeconfig loading
- **Merged kubeconfig loading**: `~/.kube/config`, `~/.kube/config.d/*` (recursive), and `KUBECONFIG` env var
- **Cluster dashboard** when entering a context (configurable)
- **Monitoring dashboard** with active Prometheus/Alertmanager alerts (`@` key), configurable endpoints per cluster
- **API Explorer** for interactively browsing resource structure (`I` key) with recursive field browser
- **Namespace selector** overlay with type-to-filter
- **All-namespaces mode** (enabled by default)

### Resource Operations

- **Context-aware action menus**: logs, exec, attach, debug, scale, restart, delete, describe, edit, events, port-forward
- **Custom user-defined actions**: Define custom shell commands per resource type in config
- **Multi-select with bulk actions**: Select multiple resources with Space, range-select with Ctrl+Space, perform bulk delete, scale, restart
- **Resource sorting** by name, age, or status
- **Filter and search**: Filter with `f`, search with `/`
- **Abbreviated search**: Type `pvc`, `hpa`, `deploy` etc. to jump to resource types
- **Watch mode**: Auto-refresh resources every 2 seconds (enabled by default)
- **Owner/controller navigation**: Jump to the owner of any resource with `o`
- **Events view** with warnings-only filter toggle

### Preview and Editing

- **YAML preview** in the right column with syntax highlighting
- **Full-screen YAML viewer** with scrollable output, search, section folding (`Tab`/`z`), and in-place editing
- **Resource details** summary in split preview (toggle with `Shift+P`)
- **Inline log viewer** with streaming, search, line numbers, word wrap, follow mode, timestamps toggle, previous container logs, container filter, tail-first loading, and line jump
- **Inline describe view** with scrollable output
- **Secret viewing/editing** with decode toggle (`Ctrl+S`) and dedicated editor (`e`)
- **Embedded terminal** (PTY mode) for exec and shell with tab switching — PTY keeps running in background when switching tabs

### Resource Management

- **Resource templates**: Create resources from built-in templates (`a`)
- **Port forwarding** from the action menu (with local port setting and browser open); manage active forwards via the Networking group
- **Clipboard support**: Copy resource name (`y`), YAML (`Ctrl+Y`), paste/apply from clipboard (`Ctrl+P`)
- **Bookmarks**: Save favorite resource paths for quick navigation
- **Session persistence**: Remembers last context/namespace/resource across restarts
- **Command bar**: Press `:` for shell/kubectl commands with autocompletion

### Integrations

- **ArgoCD integration**: Browse Applications, sync, terminate sync, refresh, view managed resources
- **Helm integration**: Browse releases, view managed resources, uninstall
- **CRD discovery**: Automatically discovers installed CRDs and groups them by API group

### Customization

- **Built-in color schemes**: Tokyonight, Kanagawa, Bluloco, Nord, Gruvbox, Dracula, Catppuccin (with dark/light variants)
- **Runtime theme switching**: Press `T` to preview and switch themes without restarting
- **Custom color themes** via config file (Tokyonight theme by default)
- **Configurable keybindings** for direct actions
- **Configurable search abbreviations**
- **Configurable filter presets** per resource type (extend built-in quick filters with `.`)
- **Configurable icon modes**: Unicode (default), simple ASCII, or no icons
- **Configurable table columns** (global and per-resource-type)
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

## Usage

```bash
# Use default kubeconfig (~/.kube/config + ~/.kube/config.d/*)
lfk

# Use a specific kubeconfig
KUBECONFIG=/path/to/config lfk

# Use multiple kubeconfigs
KUBECONFIG=/path/to/config1:/path/to/config2 lfk
```

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

### Navigation

| Key | Action |
|---|---|
| `h` / `Left` | Navigate to parent level |
| `l` / `Right` | Navigate into selected item |
| `j` / `Down` | Move cursor down |
| `k` / `Up` | Move cursor up |
| `gg` | Jump to top of list |
| `G` | Jump to bottom of list |
| `Enter` | Open full-screen YAML view / navigate into |
| `z` | Toggle expand/collapse all resource groups |
| `p` | Pin/unpin CRD group (at resource types level) |
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
| `!` | Error log |
| `Ctrl+S` | Toggle secret value visibility |
| `I` | API Explorer (browse resource structure interactively) |
| `U` | RBAC permissions browser (can-i) |
| `w` | Toggle watch mode (auto-refresh) |
| `,` | Cycle sort mode (name / age / status) |
| `W` | Save resource to file / toggle warnings-only (Events) |
| `@` | Monitoring overview (active Prometheus alerts) |
| `Q` | Namespace resource quota dashboard |

### Actions

| Key | Action |
|---|---|
| `\` | Open namespace selector |
| `A` | Toggle all-namespaces mode |
| `x` | Action menu (logs, exec, debug, describe, edit, delete, scale, port-forward, events, etc.) |
| `L` | View logs for selected resource |
| `i` | Edit labels/annotations for selected resource |
| `R` | Refresh current view |
| `v` | Describe selected resource |
| `D` | Delete selected resource (with confirmation) |
| `X` | Force destroy (remove finalizers + force delete) |
| `o` | Jump to owner/controller of selected resource |
| `y` | Copy resource name to clipboard |
| `Ctrl+Y` | Copy resource YAML to clipboard |
| `Ctrl+P` | Apply resource from clipboard (`kubectl apply`) |

### Multi-Selection

| Key | Action |
|---|---|
| `Space` | Toggle selection on current item |
| `Ctrl+Space` | Select range from anchor to cursor |
| `Ctrl+A` | Select / deselect all visible items |
| `Esc` | Clear selection |
| `x` | Bulk action menu (delete, force delete, scale, restart, diff) |
| `d` | Diff: compare YAML of two selected resources |

### Tabs

| Key | Action |
|---|---|
| `t` | New tab (clone current) |
| `]` | Next tab |
| `[` | Previous tab |

### Bookmarks

| Key | Action |
|---|---|
| `m<key>` | Set mark at current location (`a-z`, `0-9`) |
| `'` | Open bookmarks list (`a-z`/`0-9` jumps to named mark in overlay) |
| `j` / `k` | Navigate bookmarks (in overlay) |
| `/` | Filter bookmarks by name (in overlay) |
| `Enter` | Jump to selected bookmark (in overlay) |
| `D` | Delete selected bookmark (in overlay) |
| `Ctrl+X` | Delete all bookmarks (in overlay) |

### In YAML View

| Key | Action |
|---|---|
| `j` / `k` | Scroll up/down |
| `h` / `l` | Move cursor column left/right |
| `0` / `$` | Move cursor to line start/end |
| `w` / `b` | Move cursor to next/previous word |
| `g` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `/` | Search in YAML |
| `n` / `N` | Next / previous search match |
| `v` | Character visual selection (from cursor column) |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection |
| `y` | Copy selected text (in visual mode) |
| `Tab` / `z` | Toggle fold on section under cursor |
| `Z` | Toggle all folds (collapse/expand all) |
| `e` | Edit resource in `$EDITOR` |
| `q` / `Esc` | Back to explorer |

### In Log Viewer

| Key | Action |
|---|---|
| `j` / `k` | Scroll up/down |
| `h` / `l` | Move cursor column left/right |
| `$` | Move cursor to line end |
| `e` / `b` | Move cursor to word end/previous word |
| `g` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `f` | Toggle follow mode (auto-scroll) |
| `w` | Toggle line wrapping |
| `#` | Toggle line numbers |
| `/` | Search in logs |
| `n` / `N` / `p` | Next / previous search match |
| `123G` | Jump to line number |
| `s` | Toggle timestamps |
| `c` | Toggle previous container logs |
| `W` | Save loaded logs to file |
| `Ctrl+S` | Save all logs to file |
| `v` | Character visual selection (from cursor column) |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection |
| `y` | Copy selected text (in visual mode) |
| `\` | Switch pod / filter containers |
| `q` / `Esc` | Close log viewer |

### Exec Mode (embedded terminal)

`Ctrl+]` is a prefix key (like tmux's `Ctrl+b`):

| Key | Action |
|---|---|
| `Ctrl+]` `Ctrl+]` | Exit terminal and return to explorer |
| `Ctrl+]` `]` | Next tab (PTY keeps running in background) |
| `Ctrl+]` `[` | Previous tab (PTY keeps running in background) |
| `Ctrl+]` `t` | New tab (clone current context) |

All other keys are forwarded to the PTY process.

### General

| Key | Action |
|---|---|
| `T` | Switch color scheme |
| `q` | Quit |
| `Esc` | Go back / quit |
| `Ctrl+C` | Close tab (quit if last) |

## Configuration

Create `~/.config/lfk/config.yaml` to customize the application. All fields are optional; only the values you specify will override the defaults.

### Full Configuration Example

```yaml
# Built-in color scheme (overrides default theme; custom theme overrides are applied on top)
# Options: tokyonight, tokyonight-storm, tokyonight-day, kanagawa-wave, kanagawa-dragon,
#          bluloco-dark, bluloco-light, nord, gruvbox-dark, gruvbox-light, dracula,
#          catppuccin-mocha, catppuccin-macchiato, catppuccin-frappe, catppuccin-latte
colorscheme: tokyonight

# Icon mode: "unicode" (default), "simple" (ASCII), "emoji", "none"
icons: unicode

# Log file path (default: ~/.local/share/lfk/lfk.log)
log_path: /tmp/lfk.log

# Show cluster dashboard when entering a context (default: true)
dashboard: true

# Monitoring endpoints per cluster context (auto-discovered by default)
# Keys are context names; "default" applies to clusters without explicit config
monitoring:
  # my-prod-cluster:
  #   prometheus:
  #     namespaces: ["monitoring"]
  #     services: ["thanos-query"]
  #     port: "9090"
  #   alertmanager:
  #     namespaces: ["monitoring"]
  #     services: ["alertmanager-main"]
  #     port: "9093"
  # default:
  #   prometheus:
  #     namespaces: ["monitoring", "observability"]
  #     services: ["prometheus-server"]

# Per-resource-type column configuration
# Keys are resource Kind names (case-insensitive)
# When not set for a kind, columns are auto-detected from item data
resource_columns:
  # Pod: ["IP", "Node", "Image", "CPU", "MEM"]
  # Deployment: ["Replicas", "Available"]

# Custom color theme (Tokyonight-inspired defaults shown)
theme:
  primary: "#7aa2f7"       # Blue - borders, headers, breadcrumbs, active columns
  secondary: "#9ece6a"     # Green - help keys, running status, success markers
  text: "#c0caf5"          # Light purple - normal text
  selected_fg: "#1a1b26"   # Dark - selected item foreground
  selected_bg: "#7aa2f7"   # Blue - selected item background
  border: "#3b4261"        # Dark blue - inactive borders
  dimmed: "#565f89"        # Muted purple - help text, placeholders, dimmed items
  error: "#f7768e"         # Red/Pink - errors, failures, delete confirmations
  warning: "#e0af68"       # Orange/Yellow - warnings, pending status, namespace indicator
  purple: "#bb9af7"        # Purple - special values
  base: "#1a1b26"          # Dark background base
  bar_bg: "#24283b"        # Slightly lighter bar background (title/status bars)
  surface: "#1f2335"       # Surface background for overlays

# Custom keybindings for direct actions
keybindings:
  logs: "L"                # View logs (default: L)
  refresh: "R"             # Refresh view (default: R)
  restart: "r"             # Restart resource (default: r)
  exec: "s"                # Exec into pod (default: s)
  describe: "v"            # Describe resource (default: v)
  delete: "D"              # Delete resource (default: D)
  force_delete: "X"        # Force delete resource (default: X)
  scale: "S"               # Scale resource (default: S)

# Search abbreviations (extend or override built-in abbreviations)
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
  pv: persistentvolume
  sc: storageclass
  sa: serviceaccount
  hpa: horizontalpodautoscaler
  vpa: verticalpodautoscaler
  ds: daemonset
  sts: statefulset
  rs: replicaset
  cj: cronjob
  job: job
  crd: customresourcedefinition
  ev: event
  rb: rolebinding
  crb: clusterrolebinding
  cr: clusterrole
  role: role
  limit: limitrange
  quota: resourcequota
  pdb: poddisruptionbudget
  ep: endpoint
  eps: endpointslice
  netpol: networkpolicy
  rc: replicationcontroller

# Custom actions per resource type (appear in action menu after built-in actions)
# Template variables: {name}, {namespace}, {context}, {kind}, {<ColumnKey>}
custom_actions:
  # Pod:
  #   - label: "SSH to Node"
  #     command: "ssh {Node}"
  #     key: "S"
  #     description: "SSH into the pod's node"
  # Deployment:
  #   - label: "Rollout History"
  #     command: "kubectl rollout history deployment/{name} -n {namespace} --context {context}"
  #     key: "H"
  #     description: "Show rollout history"

# Pin CRD API groups after built-in categories (also manageable in-app with 'p' key)
# pinned_groups:
#   - karpenter.sh
#   - monitoring.coreos.com
#   - argoproj.io

# Show random tips on startup (default: true)
# tips: false

# Exec/shell terminal mode: "pty" (embedded in TUI, default) or "exec" (takes over terminal)
# terminal: pty

# Number of log lines to load initially (scroll up to load more)
# log_tail_lines: 1000

# Show quit confirmation on ctrl+c (default: true)
# confirm_on_exit: true
```

> For detailed documentation, see [`docs/config-reference.md`](docs/config-reference.md), [`docs/config-example.yaml`](docs/config-example.yaml), and [`docs/keybindings.md`](docs/keybindings.md).

### Built-in Color Schemes

Set a built-in color scheme in your config file:

```yaml
colorscheme: catppuccin-mocha
```

Available schemes:

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

You can also switch themes at runtime by pressing `T`. Runtime changes are not persisted - to make a theme permanent, set `colorscheme` in your config file.

Custom `theme` overrides in the config are applied on top of the selected color scheme.

### Theme Configuration

All theme colors accept CSS hex color codes. Only specify the colors you want to change; unspecified colors keep their defaults.

| Key | Default | Description |
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
| `surface` | `#1f2335` | Overlay background (namespace selector, action menu, etc.) |

### Keybinding Overrides

The following direct-action keybindings can be overridden in the config file. Other keybindings (navigation, views, tabs, etc.) are fixed.

| Config Key | Default | Action |
|---|---|---|
| `logs` | `L` | View logs for selected resource |
| `refresh` | `R` | Refresh current view |
| `restart` | `r` | Restart resource (deployments, statefulsets, daemonsets) |
| `exec` | `s` | Exec into container |
| `describe` | `v` | Describe selected resource |
| `delete` | `D` | Delete selected resource (with confirmation) |
| `force_delete` | `X` | Force destroy (remove finalizers + force delete) |
| `scale` | `S` | Scale resource (deployments, statefulsets, daemonsets) |

## Architecture

```
.
+-- main.go                    # Entry point
+-- internal/
|   +-- app/
|   |   +-- app.go             # Bubbletea model, view rendering, state
|   |   +-- update.go          # Message handling, key bindings, navigation
|   |   +-- commands.go        # Async commands (kubectl, shell, clipboard, custom actions)
|   |   +-- bookmarks.go       # Bookmark persistence
|   |   +-- session.go         # Session persistence (last context/namespace/resource)
|   +-- k8s/
|   |   +-- client.go          # Kubernetes API client, owner resolution, CRD discovery
|   +-- model/
|   |   +-- types.go           # Resource types, actions, navigation state
|   |   +-- templates.go       # Resource creation templates
|   +-- ui/
|   |   +-- explorer.go        # Column and table rendering
|   |   +-- overlay.go         # Overlay rendering (namespace, actions, bookmarks, etc.)
|   |   +-- styles.go          # Lipgloss styles and color palette
|   |   +-- theme.go           # Config file loading, theme/keybinding customization
|   |   +-- colorschemes.go    # Built-in color schemes (Tokyonight, Catppuccin, etc.)
|   |   +-- help.go            # Help screen rendering
|   |   +-- logviewer.go       # Log viewer rendering
|   +-- logger/
|       +-- logger.go          # Application logging
+-- go.mod
+-- go.sum
```

## Dependencies

- [bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [lipgloss](https://github.com/charmbracelet/lipgloss) - Styling
- [client-go](https://github.com/kubernetes/client-go) - Kubernetes API client
- [sigs.k8s.io/yaml](https://github.com/kubernetes-sigs/yaml) - YAML marshalling

## Contributing

Contributions are welcome! Here is how to get started.

### Prerequisites

- Go 1.26 or later
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

## Dependencies

### Core

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Terminal styling and layout
- [Bubbles](https://github.com/charmbracelet/bubbles) — TUI components (spinner)

### Kubernetes

- [client-go](https://github.com/kubernetes/client-go) — Kubernetes API client
- [k8s.io/api](https://github.com/kubernetes/api) — Kubernetes API types
- [k8s.io/apimachinery](https://github.com/kubernetes/apimachinery) — Kubernetes API machinery

### Other

- [testify](https://github.com/stretchr/testify) — Testing assertions
- [sigs.k8s.io/yaml](https://github.com/kubernetes-sigs/yaml) — YAML marshaling

## Support

If you find lfk useful and want to support its development:

- [GitHub Sponsors](https://github.com/sponsors/janosmiko)
- [Buy Me a Coffee](https://buymeacoffee.com/janosmiko)

## License

MIT
