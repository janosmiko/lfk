<img src="docs/imgs/github-header.png" alt="lfk">

# :zap: LFK - Lightning Fast Kubernetes navigator

[![Release](https://img.shields.io/github/v/release/janosmiko/lfk)](https://github.com/janosmiko/lfk/releases) [![CI](https://img.shields.io/github/actions/workflow/status/janosmiko/lfk/ci.yml?branch=main&label=CI)](https://github.com/janosmiko/lfk/actions/workflows/ci.yml) [![Stars](https://img.shields.io/github/stars/janosmiko/lfk?style=flat)](https://github.com/janosmiko/lfk) [![Sponsor](https://img.shields.io/badge/sponsor-%E2%9D%A4-db61a2)](https://github.com/sponsors/janosmiko) [![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=janosmiko_lfk&metric=security_rating)](https://sonarcloud.io/dashboard?id=janosmiko_lfk) [![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=janosmiko_lfk&metric=vulnerabilities)](https://sonarcloud.io/dashboard?id=janosmiko_lfk) [![codecov](https://codecov.io/gh/janosmiko/lfk/graph/badge.svg)](https://codecov.io/gh/janosmiko/lfk) [![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/janosmiko/lfk/badge)](https://scorecard.dev/viewer/?uri=github.com/janosmiko/lfk) [![OpenSSF Best Practices](https://www.bestpractices.dev/projects/12677/badge)](https://www.bestpractices.dev/projects/12677)[![Cloudsmith](https://img.shields.io/badge/OSS%20hosting%20by-cloudsmith-blue?logo=cloudsmith&style=flat-square&link=https%3A%2F%2Fcloudsmith.com)](https://cloudsmith.com)

**LFK** is a keyboard-focused, yazi-inspired terminal user interface for navigating and managing Kubernetes clusters. It brings a three-column Miller columns layout with an owner-based resource hierarchy to your terminal.

## Install

Pick your platform:

```bash
# Homebrew (macOS / Linux)
brew install janosmiko/tap/lfk

# Scoop (Windows)
scoop bucket add janosmiko https://github.com/janosmiko/scoop-bucket && scoop install lfk

# Winget (Windows)
winget install janosmiko.lfk

# Chocolatey (Windows)
choco install lfk

# AUR (Arch)
yay -S lfk-bin

# APT (Debian / Ubuntu)
curl -1sLf 'https://dl.cloudsmith.io/public/janosmiko/lfk/setup.deb.sh' | sudo -E bash && sudo apt update && sudo apt install lfk

# DNF (Fedora / RHEL)
curl -1sLf 'https://dl.cloudsmith.io/public/janosmiko/lfk/setup.rpm.sh' | sudo -E bash && sudo dnf install lfk

# Go
go install github.com/janosmiko/lfk@latest

# Nix
nix run github:janosmiko/lfk
```

Then run it:

```bash
lfk
lfk --demo
```

`lfk` uses `~/.kube/config` and `~/.kube/config.d/*`. `lfk --demo` runs against a built-in fake cluster, so no kubeconfig is needed.

`kubectl` is required and must be configured. `helm` and `trivy` are optional, for Helm management and image vulnerability scanning.

[docs/installation.md](docs/installation.md) covers Docker, NixOS and home-manager, binary releases, building from source, and the full list of optional CLI dependencies.

## Usage

```bash
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

[docs/usage.md](docs/usage.md#command-line-flags) has the full CLI reference and the runtime tuning options: mouse capture, no-color mode, read-only mode, watch-mode interval, discovery cache, and Secret lazy loading.

## Sponsor

LFK is Apache-2.0 and stays free. I build it in my free time, and the tools it takes to build it are not free.

**Do you use LFK at work? Ask your team to sponsor it.** A company expenses this without noticing, and an individual feels it. Most clusters LFK gets pointed at are company clusters.

- [GitHub Sponsors](https://github.com/sponsors/janosmiko)
- [Buy Me a Coffee](https://buymeacoffee.com/janosmiko)

Sponsorship pays for the IDE licenses and AI assistants that go into development. More of it buys more hours for LFK.

Money is not the only way to help. Star the repo, file a good bug report, or tell one colleague who lives in `kubectl`.

Package repository hosting is graciously provided by [Cloudsmith](https://cloudsmith.com).
Cloudsmith is a hosted package management service that stores and serves packages in many formats.

## Screenshots

### Demo

![Demo](./docs/imgs/demo.gif)

### Themes

![Themes](./docs/imgs/themes.gif)

### Explorer

| Feature | |
| --- | --- |
| Pods | <img src="./docs/imgs/pods.png" alt="Pods" width="600"> |
| Pods fullscreen | <img src="./docs/imgs/pods-fullscreen.png" alt="Pods fullscreen" width="600"> |
| Pod action menu | <img src="./docs/imgs/pods-actions.png" alt="Pod actions" width="600"> |
| Cluster Dashboard | <img src="./docs/imgs/cluster-dashboard.png" alt="Cluster Dashboard" width="600"> |
| Object Explorer | <img src="./docs/imgs/object-explorer.png" alt="Object Explorer" width="600"> |
| API Explorer | <img src="./docs/imgs/api-explorer.png" alt="API Explorer" width="600"> |

### Viewers

| Feature | |
| --- | --- |
| YAML Viewer | <img src="./docs/imgs/yaml-preview.png" alt="YAML Viewer" width="600"> |
| Log Viewer | <img src="./docs/imgs/log-viewer.png" alt="Log Viewer" width="600"> |
| Log Top | <img src="./docs/imgs/log-top.png" alt="Log Top" width="600"> |

### Integrations

| Feature | |
| --- | --- |
| Helm integration | <img src="./docs/imgs/helm-integration.png" alt="Helm integration" width="600"> |
| ArgoCD integration | <img src="./docs/imgs/argocd-integration.png" alt="ArgoCD integration" width="600"> |
| ArgoCD AutoSync config | <img src="./docs/imgs/argocd-autosync.png" alt="ArgoCD auto-sync config" width="300"> |
| Trivy integration | <img src="./docs/imgs/trivy-integration.png" alt="Trivy integration" width="600"> |

### Analysis

| Feature | |
| --- | --- |
| Security Dashboard | <img src="./docs/imgs/security-heuristics.png" alt="Built-in security heuristics" width="600"> |
| Crash Investigator | <img src="./docs/imgs/crash-investigator.png" alt="Crash Investigator" width="600"> |
| Pod Startup Analysis | <img src="./docs/imgs/startup-analysis.png" alt="Pod Startup Analysis" width="600"> |
| Can-I RBAC permissions browser | <img src="./docs/imgs/can-i.png" alt="Can-I viewer" width="600"> |

### Workspace

| Feature | |
| --- | --- |
| Label and annotation editor | <img src="./docs/imgs/label-annotation-editor.png" alt="Label and annotation editor" width="600"> |
| Quick filters | <img src="./docs/imgs/quick-filters.png" alt="Quick filters" width="600"> |
| Column visibility and reordering | <img src="./docs/imgs/column-visibility-reordering.png" alt="Column visibility and reordering" width="600"> |
| Bookmarks | <img src="./docs/imgs/bookmarks.png" alt="Bookmarks" width="600"> |
| Multi-tab support | <img src="./docs/imgs/tab-support.png" alt="Multi-tab support" width="600"> |
| Union view | <img src="./docs/imgs/union-sets.png" alt="Union sets" width="600"> |
| Local cluster management | <img src="./docs/imgs/local-clusters.png" alt="Local clusters" width="600"> |

## Features

### Navigation

- Three-column Miller columns layout: parent, current, preview
- Owner-based hierarchy: Clusters -> Resource Types -> Resources -> Owned Resources -> Containers
- Jump to a resource's owner with `o`, back with `Backspace`
- Teleport between levels with `0` / `1` / `2`
- Cycle the layout with `F`, expand or collapse groups with `z`
- Vim keybindings throughout, remappable, including `Ctrl+Shift` chords: [keybindings.md](docs/keybindings.md#ctrlshift-chords)
- Mouse click, scroll, and `Ctrl+Option+Y` to release capture: [usage.md](docs/usage.md#mouse-support)

### Resource type list

- Groups: Dashboards, Workloads, Networking, Config, Storage, ArgoCD, Helm, Access Control, Cluster, Custom Resources
- Discovered CRDs grouped by API group, for example `argoproj.io`, `longhorn.io`
- Pin a type with `p`: [keybindings.md](docs/keybindings.md#navigation)
- Pin a type's dashboard summary with `x`: [config-reference.md](docs/config-reference.md#top-level-fields)
- Hide or show a single type with `x`, reveal hidden ones with `H`: [config-reference.md](docs/config-reference.md#top-level-fields)
- Rarely used types (CSI internals, webhooks, APF, leases) hidden until `H`: [config-reference.md](docs/config-reference.md#top-level-fields)
- Status summary band under the hovered type: [features.md](docs/features.md#list-status-summary)

### Clusters and contexts

- Multi-tab workspace with `t` / `]` / `[`: [keybindings.md](docs/keybindings.md#tabs)
- Merged kubeconfig from `~/.kube/config`, `~/.kube/config.d/*`, and `KUBECONFIG_DIR`: [usage.md](docs/usage.md#kubeconfig-directory)
- Union view merging several clusters into one table: [union-context.md](docs/union-context.md)
- Cluster Dashboard on entering a context: [config-reference.md](docs/config-reference.md#top-level-fields)
- Monitoring dashboard with Prometheus and Alertmanager alerts, `@`: [config-reference.md](docs/config-reference.md#monitoring)
- Local kind, k3d, and minikube clusters with `Ctrl+N`: [keybindings.md](docs/keybindings.md#local-clusters-manager)
- Named sessions with `C`: [keybindings.md](docs/keybindings.md#named-sessions)
- Context, namespace, filter, and cursor row restored on start: [config-reference.md](docs/config-reference.md#session-persistence)

### Safety

- Read-only mode via `--read-only` or `Ctrl+R`: [usage.md](docs/usage.md#read-only-mode)
- Actions your RBAC refuses are dropped from the action menu: [usage.md](docs/usage.md#permission-aware-actions)
- Security dashboard and SEC badges from Trivy, Kyverno, Kubescape, Falco, Gatekeeper, and three built-in scanners: [security.md](docs/security.md)
- Delete cascade picker, `Tab` cycles Background / Foreground / Orphan / None: [keybindings.md](docs/keybindings.md#delete-confirm-dialog)
- Scope, Availability, and Risk rows on the delete, drain, and scale dialogs: [keybindings.md](docs/keybindings.md#delete-confirm-dialog)
- RBAC browser with `U`, reverse lookup with `Tab`: [keybindings.md](docs/keybindings.md#can-i-browser)

### Resource actions

- Action menu `x`: logs, exec, attach, debug, scale, restart, delete, describe, edit, events, port-forward, vuln scan, PVC resize: [keybindings.md](docs/keybindings.md#action-menu-items)
- Multi-select with `Space`, range with `Ctrl+Space`, then bulk delete, scale, or restart: [keybindings.md](docs/keybindings.md#multi-selection)
- Custom shell actions per resource type: [config-reference.md](docs/config-reference.md#custom-actions)
- Port forwarding, with active forwards listed under the Networking group: [keybindings.md](docs/keybindings.md#action-menu-items)
- Right-sizing Advisor with `x` -> `z`: [features.md](docs/features.md#right-sizing-advisor), keys in [keybindings.md](docs/keybindings.md#right-sizing-advisor)
- Network policy visualizer with `x` -> `N`, Cilium aware: [keybindings.md](docs/keybindings.md#network-policy-visualizer)
- Traffic capture with `c` on a Pod or Service: [usage.md](docs/usage.md#traffic-capture)

### Finding things

- Filter with `f`, search with `/`, in substring, regex, fuzzy, or literal mode
- Abbreviated jumps such as `pvc`, `hpa`, `deploy`: [config-reference.md](docs/config-reference.md#abbreviations)
- Command bar `:` for resource jumps, built-ins, kubectl, and shell: [commands.md](docs/commands.md), autocomplete in [features.md](docs/features.md#command-bar-autocomplete)
- Quick filter presets with `.`: [config-reference.md](docs/config-reference.md#filter-presets)
- Sorting with `>` / `<` / `=` / `-`, remembered per kind: [keybindings.md](docs/keybindings.md#sorting)
- Bookmarks with `m<slot>` and `'<slot>`: [keybindings.md](docs/keybindings.md#bookmarks)
- Orphan detection with `Shift+Z` across 11 kinds: [usage.md](docs/usage.md#orphan-detection)

### Viewers

- YAML preview in the right column, details summary with `Shift+P`
- Fullscreen YAML Viewer with search, folding, and in-place editing: [keybindings.md](docs/keybindings.md#yaml-view)
- Field-manager blame with `m`, writes stamped `lfk:<os-user>`: [config-reference.md](docs/config-reference.md#top-level-fields)
- Schema side pane with `Ctrl+K`: [features.md](docs/features.md#schema-side-pane)
- Object Explorer with `O` for the live object: [keybindings.md](docs/keybindings.md#object-explorer)
- API Explorer with `I` for the resource schema: [keybindings.md](docs/keybindings.md#api-explorer)
- Describe with `v`, relationship map with `M`, events with `V` plus warnings-only and grouping toggles: [keybindings.md](docs/keybindings.md#actions)

### Logs and shells

- Live-log preview pane with `L`, fullscreen viewer with `Ctrl+L`: [keybindings.md](docs/keybindings.md#log-viewer)
- Text filter `f`, severity filter `i` / `o`, structured preview `P`: [keybindings.md](docs/keybindings.md#log-viewer)
- Log Top aggregates access logs into a metrics table, `T`: [keybindings.md](docs/keybindings.md#log-top)
- Crash Investigator with `x` -> `I`: [keybindings.md](docs/keybindings.md#crash-investigator-overlay)
- Embedded terminal for exec and shell, `Ctrl+T` cycles the mode: [features.md](docs/features.md#embedded-terminal)
- Node shell with `x` -> `s`: [usage.md](docs/usage.md#node-shell)

### Creating and copying

- Resource templates with `a`, built-ins plus your own: [features.md](docs/features.md#resource-templates)
- Export a live object as a template with `x` -> `T`: [features.md](docs/features.md#export-as-template)
- Copy name `y`, copy-as picker `Y`, single field `Ctrl+Y`: [keybindings.md](docs/keybindings.md#clipboard)
- Apply a manifest from the clipboard with `Ctrl+P`: [keybindings.md](docs/keybindings.md#clipboard)
- Save the selected manifest to a file with `W`
- Decode Secrets with `Ctrl+S`, edit them decoded with `e`: [keybindings.md](docs/keybindings.md#secret-actions)
- Finalizer search and removal with `Ctrl+G`

### Integrations

- Argo CD: browse Applications, sync, terminate, refresh: [features.md](docs/features.md#argo-cd)
- Argo Workflows: suspend, resume, stop, resubmit, submit from a template: [features.md](docs/features.md#argo-workflows)
- Helm: browse releases, values, diff, upgrade, rollback, uninstall: [keybindings.md](docs/keybindings.md#helm-release-actions)
- HPA scaling overlay with `S`: [keybindings.md](docs/keybindings.md#horizontalpodautoscaler-actions)
- KEDA: pause and unpause ScaledObjects and ScaledJobs: [features.md](docs/features.md#keda)
- External Secrets: force refresh: [features.md](docs/features.md#external-secrets)
- CRD discovery, grouped by API group

### Appearance

- 460+ built-in color schemes from [ghostty themes](https://github.com/ghostty-org/ghostty), `T` switches live: [config-reference.md](docs/config-reference.md#color-schemes)
- Auto dark and light switching with the OS appearance: [config-reference.md](docs/config-reference.md#auto-darklight-mode)
- Custom theme colors, Tokyonight by default: [config-reference.md](docs/config-reference.md#theme)
- Icon modes: auto, unicode, nerdfont, simple, emoji, none: [config-reference.md](docs/config-reference.md#icon-mode-auto-detection)
- Status colors on rows and CRD printer columns: [features.md](docs/features.md#status-colors)
- Row status tint when the Status column is hidden: [config-reference.md](docs/config-reference.md#row-status-tint)
- Transparent background: [config-reference.md](docs/config-reference.md#appearance)

### Tables and metrics

- Column visibility and order with `,`, remembered per kind: [keybindings.md](docs/keybindings.md#column-toggle-overlay)
- Configurable columns globally, per resource type, and per cluster: [config-reference.md](docs/config-reference.md#views)
- CPU and memory bars on the Cluster Dashboard: [features.md](docs/features.md#resource-usage-metrics)
- Node uptime column: [config-reference.md](docs/config-reference.md#node-uptime-column)
- Namespace resource quota dashboard with `Q`
- Watch mode auto-refresh with `w`: [usage.md](docs/usage.md#watch-mode-interval)
- CPU and MEM sparklines from Prometheus history with `~`: [keybindings.md](docs/keybindings.md#modes-and-settings)
- Random startup tips: [config-reference.md](docs/config-reference.md#top-level-fields)

## Navigation hierarchy

```
Clusters (kubeconfig contexts)
  +-- Resource Types (grouped: Workloads, Networking, Config, Storage, ArgoCD, Helm, ...)
        +-- Resources (e.g., individual Deployments)
              +-- Owned Resources (Pods via ownerReferences, Jobs for CronJobs, etc.)
                    +-- Containers (for Pods)
```

Namespaces are **not** a navigation level. The top-right corner shows the current namespace. Press `\` to open the selector, which filters as you type. All-namespaces mode is on by default, and `A` toggles it.

Inside the namespace selector:

| Key | Action |
|---|---|
| `Space` | Include a namespace |
| `Tab` | Exclude a namespace (shows all except the marked ones, each prefixed with `!`) |
| `A` | Reset to all-namespaces mode |
| `R` | Refresh the list from the cluster |
| `.` | Quick-filter to the namespace of the resource behind the overlay |

### Owner resolution

- **Deployments** show their Pods (resolved through ReplicaSets, flattened)
- **StatefulSets / DaemonSets / Jobs** show their Pods directly
- **CronJobs** show their Jobs
- **Services** show Pods matching the service selector
- **ArgoCD Applications** show managed resources (from status or label discovery)
- **Helm Releases** show managed resources (via `app.kubernetes.io/instance` label)
- **Pods** show their Containers
- **ConfigMaps / Secrets / Ingresses / PVCs** show details preview (no children)

## Keybindings

[docs/keybindings.md](docs/keybindings.md) is the complete reference, including the YAML view, Log Viewer, describe, diff, exec mode, and every sub-mode. Press `F1` in-app for the help screen, `?` for the which-key action panel.

### Move

| Key | Action |
|---|---|
| `h` / `l` | Parent level / into the selected item |
| `j` / `k` | Cursor down / up |
| `gg` / `G` | Top / bottom of list |
| `Ctrl+D` / `Ctrl+U` | Half-page scroll down / up |
| `Ctrl+F` / `Ctrl+B` | Full-page scroll down / up |
| `J` / `K` | Scroll the preview pane down / up |
| `Enter` | Fullscreen YAML view, or navigate into |

### Jump

| Key | Action |
|---|---|
| `0` / `1` / `2` | Clusters / resource types / resources level, both ways |
| `o` | Owner or controller of the selected resource |
| `Backspace` | Back through teleport history |
| `g` + key | Goto resource type (`g` opens the which-key popup) |
| `m<slot>` / `'<slot>` | Set / jump to bookmark (lowercase context-aware, uppercase context-free) |
| `\` / `A` | Namespace selector / toggle all-namespaces |
| `g\` | Previous namespace |

### Resource type list

| Key | Action |
|---|---|
| `z` | Expand or collapse all groups (Events view: toggle grouping) |
| `p` | Pin or unpin the selected type |
| `x` | Pin, hide, or show the selected type |
| `H` | Reveal rarely used and hidden types |

### Search and filter

| Key | Action |
|---|---|
| `f` | Filter items in the current view |
| `/` | Search and jump to a match |
| `n` / `N` | Next / previous match |
| `.` | Quick filter presets |
| `,` | Column visibility and order |
| `>` / `<` / `=` / `-` | Sort by next / previous column, flip direction, reset |

### Views

| Key | Action |
|---|---|
| `F1` / `?` | Help screen / which-key action panel |
| `P` | Details summary or YAML preview |
| `M` | Resource relationship map |
| `F` | Cycle layout: hide sidebar, fullscreen, restore |
| `I` / `O` | API Explorer / Object Explorer |
| `Ctrl+K` | Schema side pane for the field under the cursor |
| `U` | RBAC permissions browser (can-i) |

### Actions

| Key | Action |
|---|---|
| `x` | Action menu (logs, exec, describe, edit, delete, scale, port-forward) |
| `v` / `L` / `Ctrl+L` | Describe / live-log preview / fullscreen Log Viewer |
| `D` / `X` | Delete / force delete |
| `y` / `Y` / `Ctrl+Y` | Copy name / copy-as picker / copy a single field |
| `Ctrl+P` / `W` | Apply from clipboard / save the manifest to a file |
| `Space` | Toggle multi-selection (bulk actions via `x`) |
| `Ctrl+S` / `Ctrl+G` | Secret value visibility / finalizer search and remove |

### Tools

| Key | Action |
|---|---|
| `:` | Command bar (resource jumps, built-ins, kubectl, shell) |
| `T` | Theme selector |
| `@` / `Q` | Cluster or monitoring dashboard / namespace quotas |
| `w` | Watch mode (auto-refresh) |
| `Ctrl+T` | Cycle terminal mode (pty / exec / mux) |
| `Ctrl+R` | Read-only mode |
| `!` | Error log |
| `t` / `]` / `[` / `}` / `{` | New tab / next / previous / move right / move left |

All views (YAML, logs, describe, diff, exec) use vim-style navigation: `j`/`k`, `gg`/`G`, `Ctrl+D`/`Ctrl+U`, `/` search, `v`/`V` visual selection.

[docs/commands.md](docs/commands.md) is the command bar reference: built-in commands, shell and kubectl execution, resource jumps.

## Configuration

1. Create `~/.config/lfk/config.yaml`.
2. Add only the keys you want to change. Every other value keeps its default.
3. Press `T` in-app to browse themes, then put the name you picked in `colorscheme`.

```yaml
# Color scheme. "dark:X,light:Y" switches with the OS appearance.
colorscheme: "dark:catppuccin-mocha,light:catppuccin-latte"

# Use the terminal's own background
transparent_background: true

# auto, unicode, nerdfont, simple, emoji, or none. LFK_ICONS overrides it.
icons: auto

# Disable mouse capture (allows native terminal text selection)
mouse: false

# Keybinding overrides
keybindings:
  logs: "ctrl+l"
  toggle_preview_logs: "L"
  describe: "v"
  delete: "D"

# Search abbreviations
abbreviations:
  myapp: myapplications
```

[docs/config-reference.md](docs/config-reference.md) is the full reference, [docs/config-example.yaml](docs/config-example.yaml) a commented example of every field.

### Search modes

Every search and filter input auto-detects the mode from the query string:

| Mode | Syntax | Example |
|---|---|---|
| Substring | plain text | `nginx` |
| Regex | auto-detected | `err[0-9]+` |
| Fuzzy | `~` prefix | `~deplymnt` |
| Literal | `\` prefix | `\err.*` |

All of them accept pasted text (`Cmd+V` on macOS, `Ctrl+Shift+V` on Linux). A multiline paste shows a confirmation dialog.

`Up` / `Down` recall previous queries. `/` and `f` share one history, the Log Viewer's `/` and the `:` command bar keep their own. All three survive restarts under `$XDG_STATE_HOME/lfk/`: [keybindings.md](docs/keybindings.md#log-viewer).

## Tips and tricks

### Navigating

- Press `o` on a resource to jump to its owner (Pod -> Deployment), then `Backspace` to jump back
- Teleport between levels with `0` / `1` / `2` (clusters / resource types / resources). `1` and `2` bring you back to the view you left, per cluster
- Jump straight to a resource type from anywhere: type `:pod`, `:dep`, `:pvc`
- Set a bookmark with `m<letter>`, jump back with `'<letter>`, the saved namespace and filter come with it
- Pin your daily-driver resource types with `p` and hide noisy ones via `x`, both remembered per cluster
- Need everything except a few namespaces? In the `\` selector, `Tab` excludes namespaces instead of selecting them
- See how a resource connects to everything else with `M` (relationship map)

### Finding

- Typos are fine in search: `/~deplymnt` fuzzy-matches `deployments`
- Press `Tab` inside `/` or `f` to broaden matching to labels, annotations, finalizers, and other column values
- Recall earlier queries with `Up` / `Down` inside `/`, `f`, and the `:` command bar
- Press `.` for quick filter presets, for example only failing Pods
- Hunt down unused ConfigMaps, Secrets, PVCs and more with `Shift+Z` (orphan detection)
- Sort by any column with `>` / `<`, flip direction with `=`, reset with `-`
- Check firing Prometheus or Alertmanager alerts with `@`, and namespace quotas with `Q`

### Logs

- Peek at Pod or Deployment logs with `L`, open the fullscreen viewer with `Ctrl+L`
- Make noisy logs readable with `P`: the structured preview parses JSON, logfmt, klog, zap, nginx, envoy, Java, and postgres lines
- Save logs to a file with `S` (loaded lines) or `Ctrl+S` (full history), the path lands on your clipboard
- Switch pods or filter containers without leaving the Log Viewer: press `\`
- Investigate a crash-looping Pod with `x` -> `I`: restart history, events, previous logs, and describe in one tabbed view

### Inspecting

- Walk any resource's live object with `O`: `r` finds keys recursively, `T` expands an ASCII tree, `y` copies the field path
- Forget `kubectl explain`, `I` opens the API Explorer and `n` / `N` auto-drill into nested fields
- In the YAML Viewer, press `O` on a line to open the Object Explorer at that attribute, or `I` to see its schema
- `Ctrl+K` opens a side pane with the schema description of the field under the cursor, and keeps it in step as you move
- Fold YAML sections with `z` (`Z` folds all), edit the resource in your `$EDITOR` with `Ctrl+E`
- Replay a resource's event history as a timeline with `V`
- Every viewer speaks vim: counts (`100j`, `42G`, `5n`), visual selections (`v` / `V` / `Ctrl+V`), and text objects (`viw`)

### Acting

- Multi-select with `Space` (range-select with `Ctrl+Space`), then bulk delete, scale, or restart via `x`
- Decode Secret values in the preview with `Ctrl+S`, or edit them decoded with `e`
- Copy the resource name with `y`, press `Y` to copy as YAML, JSON, or Table
- Apply a manifest straight from your clipboard with `Ctrl+P`
- Save the selected resource manifest to a file with `W`
- Resource stuck in Terminating? `Ctrl+G` searches its finalizers and removes them
- Open an Ingress host, or an active port-forward's localhost URL, with `Ctrl+O`. On a Service it starts a port forward and opens it
- Run kubectl without leaving lfk (`:k get pods -o wide`) or any shell command (`:! curl ...`)

### Cluster work

- Lock a session against destructive actions with `Ctrl+R` (read-only mode)
- Spin up a throwaway kind, k3d, or minikube cluster with `Ctrl+N` at the cluster list
- Capture a Pod's network traffic with `c`: live decode plus pcap export
- Flip the RBAC question: inside the Can-I browser (`U`), `Tab` opens Who-Can, every subject allowed to run a verb on a resource
- Get per-container CPU and memory recommendations with `x` -> `z` (Right-sizing Advisor, from a VPA or Prometheus when available, see [features.md](docs/features.md#right-sizing-advisor))
- Watch an ArgoCD Application roll out wave by wave: `x` -> `W` opens the Sync Wave Timeline
- Try a new look without restarting: `T` live-previews 460+ themes
- Waiting for a rollout? `:nyan` and `:kubetris` are real commands

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for prerequisites, development setup, build and test commands, project layout, and the PR flow.

## License

Apache License 2.0, see [LICENSE](LICENSE).

## Star History

<a href="https://star-history.dera.page/#janosmiko/lfk&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://star-history.dera.page/svg?repos=janosmiko/lfk&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://star-history.dera.page/svg?repos=janosmiko/lfk&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://star-history.dera.page/svg?repos=janosmiko/lfk&type=date&legend=top-left" />
 </picture>
</a>
