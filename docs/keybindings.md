# Keybindings Reference

Complete list of all keybindings in `lfk`. Keybindings marked with **(configurable)** can be overridden in `~/.config/lfk/config.yaml` under the `keybindings` section.

## Navigation

| Key | Action |
|---|---|
| `h` / `Left` | Navigate to parent level |
| `l` / `Right` | Navigate into selected item |
| `j` / `Down` | Move cursor down |
| `k` / `Up` | Move cursor up |
| `gg` | Jump to top of list |
| `G` | Jump to bottom of list |
| `Enter` | Open full-screen YAML view / navigate into |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `z` | Toggle expand/collapse all resource groups |
| `p` | Pin/unpin CRD group (at resource types level) |
| `0` / `1` / `2` | Jump to clusters / types / resources level |
| `J` / `K` | Scroll preview pane down/up |
| `o` | Jump to owner/controller of selected resource |

## Views and Modes

| Key | Action |
|---|---|
| `?` | Toggle help screen |
| `P` | Toggle between details summary and YAML preview |
| `F` | Toggle fullscreen (middle column or dashboard) |
| `.` | Quick filter presets |
| `!` | Error log |
| `Ctrl+S` | Toggle secret value visibility (decode base64) |
| `m` | Toggle resource relationship map view |
| `w` | Toggle watch mode (auto-refresh every 2s) |
| `,` | Cycle sort mode (name / age / status) |
| `W` | Save resource to file / toggle warnings-only filter (Events view) |
| `T` | Switch color scheme (live preview, not persisted) |

## Search and Filter

| Key | Action |
|---|---|
| `f` | Start filter mode (filter items in current view) |
| `/` | Start search mode (search and jump to match) |
| `n` | Jump to next search match |
| `N` | Jump to previous search match |
| `Esc` | Clear filter / cancel search |

Search supports abbreviated resource type names (e.g., `pvc`, `hpa`, `deploy`).

## Actions

| Key | Action | Configurable |
|---|---|---|
| `x` | Open action menu (bulk actions when items selected) | No |
| `\` | Open namespace selector | No |
| `A` | Toggle all-namespaces mode | No |
| `L` | View logs for selected resource | Yes (`logs`) |
| `R` | Refresh current view | Yes (`refresh`) |
| `v` | Describe selected resource (default: `v`) | Yes (`describe`) |
| `D` | Delete selected resource (with confirmation) | Yes (`delete`) |
| `X` | Force destroy (remove finalizers + force delete) | Yes (`force_delete`) |
| `o` | Jump to owner/controller of selected resource | No |
| `i` | Edit labels/annotations | No |
| `@` | Monitoring overview (active Prometheus alerts) | No |
| `Q` | Namespace resource quota dashboard | No |

Port forwarding is available via the action menu (`x`) on Pod, Service, Deployment, StatefulSet, and DaemonSet resources. After creating a port forward, the view automatically navigates to the Port Forwards list and displays the resolved local port in the status bar. Active port forwards can be managed via the "Port Forwards" virtual resource in the Networking group.

Resource-specific actions (exec, scale, restart, secret editor, etc.) are available through the action menu (`x`).

## Clipboard

| Key | Action |
|---|---|
| `y` | Copy resource name to clipboard |
| `Ctrl+Y` | Copy resource YAML to clipboard |
| `Ctrl+P` | Apply resource from clipboard (`kubectl apply`) |

## Multi-Selection

| Key | Action |
|---|---|
| `Space` | Toggle selection on current item |
| `Ctrl+A` | Select / deselect all visible items |
| `Esc` | Clear selection |

When items are selected, press `x` to open the bulk action menu (delete, force delete, scale, restart, diff).

## Bookmarks

| Key | Context | Action |
|---|---|---|
| `B` | Explorer | Bookmark current location |
| `b` | Explorer | Open bookmarks list |
| `j` / `k` | Bookmark overlay | Navigate bookmarks |
| `/` | Bookmark overlay | Filter bookmarks by name |
| `Enter` | Bookmark overlay | Jump to selected bookmark |
| `d` | Bookmark overlay | Delete selected bookmark |
| `D` | Bookmark overlay | Delete all bookmarks |

## YAML View

| Key | Action |
|---|---|
| `j` / `k` | Scroll up/down |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `/` | Search in YAML |
| `n` / `N` | Next / previous search match |
| `Tab` / `z` | Toggle fold on section under cursor |
| `Z` | Toggle all folds (collapse/expand all) |
| `e` | Edit resource in `$EDITOR` |
| `q` / `Esc` | Back to explorer |

## Log Viewer

| Key | Action |
|---|---|
| `j` / `k` | Scroll up/down |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `f` | Toggle follow mode (auto-scroll to new logs) |
| `w` | Toggle line wrapping |
| `l` | Toggle line numbers |
| `/` | Search in logs |
| `n` / `N` / `p` | Next / previous search match |
| `123G` | Jump to specific line number |
| `P` | Switch pod (group resources only) |
| `q` / `Esc` | Close log viewer |

## Diff View

| Key | Action |
|---|---|
| `j` / `k` | Scroll up/down |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `l` | Toggle line numbers |
| `123G` | Jump to line number |
| `u` | Toggle unified/side-by-side view |
| `q` / `Esc` | Back to explorer |

## Network Policy Visualizer

| Key | Action |
|---|---|
| `j` / `k` | Scroll up/down |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `q` / `Esc` | Close visualizer |

## Tabs

| Key | Action |
|---|---|
| `t` | New tab (clone current view) |
| `]` | Next tab |
| `[` | Previous tab |

## Mouse

| Input | Action |
|---|---|
| Click | Select item / navigate |
| Scroll | Navigate up/down |
| Shift+Drag | Select text (terminal native) |

## Command Bar

Press `:` to open the command bar. It supports both kubectl commands and arbitrary shell commands.

| Input | Behavior |
|---|---|
| `get pods` | Runs `kubectl get pods` with current context/namespace auto-injected |
| `kubectl get pods -A` | Same as above (explicit `kubectl` prefix is optional) |
| `helm list` | Runs as a shell command (`sh -c "helm list"`) with `KUBECONFIG` set |
| `curl http://example.com` | Runs as a shell command |
| `q` / `quit` | Quits the application |

Autocompletion is available for kubectl commands (subcommands, resource types, resource names, namespaces, flags).

| Key | Action |
|---|---|
| `Tab` | Accept suggestion |
| `Shift+Tab` / `Left` / `Right` | Cycle through suggestions |
| `Enter` | Execute command |
| `Esc` / `Ctrl+C` | Cancel |
| `Ctrl+W` | Delete word backwards |

## General

| Key | Action |
|---|---|
| `T` | Switch color scheme |
| `q` | Quit application |
| `Esc` | Go back one level / close overlay / quit |
| `Ctrl+C` | Close current tab (quit if last tab) |

## Action Menu Items

The action menu (`x` key) shows context-specific actions based on the resource type:

### Pod Actions
`l` Logs, `e` Exec, `A` Attach, `b` Debug, `p` Port Forward, `S` Startup Analysis, `d` Describe, `E` Edit, `D` Delete, `v` Events

### Deployment Actions
`l` Logs, `e` Exec, `A` Attach, `s` Scale, `r` Restart, `R` Rollback, `p` Port Forward, `d` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `v` Events

### StatefulSet Actions
`l` Logs, `e` Exec, `A` Attach, `s` Scale, `r` Restart, `p` Port Forward, `d` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `v` Events

### DaemonSet Actions
`l` Logs, `e` Exec, `A` Attach, `r` Restart, `p` Port Forward, `d` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `v` Events

### Service Actions
`l` Logs, `e` Exec, `A` Attach, `p` Port Forward, `d` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `v` Events

### Node Actions
`c` Cordon, `u` Uncordon, `n` Drain, `t` Taint, `T` Untaint, `s` Shell, `d` Describe, `E` Edit, `v` Events

### CronJob Actions
`l` Logs, `e` Exec, `A` Attach, `t` Trigger (create Job), `d` Describe, `E` Edit, `D` Delete, `v` Events

### ArgoCD Application Actions
`s` Sync, `f` Diff, `R` Refresh, `d` Describe, `E` Edit, `D` Delete, `v` Events

### Helm Release Actions
`V` Values, `A` All Values, `E` Edit Values, `R` Rollback, `d` Describe, `D` Delete, `v` Events

### Ingress Actions
`o` Open in Browser, `d` Describe, `E` Edit, `D` Delete, `v` Events

### PVC Actions
`m` Debug Mount, `d` Describe, `E` Edit, `D` Delete, `v` Events

### Default Actions (all other resources)
`d` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `v` Events

### Bulk Actions (when items multi-selected)
`D` Delete, `X` Force Delete, `s` Scale, `r` Restart

Custom actions defined in the config file appear after the built-in actions.

## Configuring Keybindings

Add to `~/.config/lfk/config.yaml`:

```yaml
keybindings:
  logs: "L"           # Default: L
  refresh: "R"        # Default: R
  restart: "r"        # Default: r
  exec: "s"           # Default: s
  describe: "v"        # Default: v
  delete: "D"         # Default: D
  force_delete: "X"   # Default: X
  scale: "S"          # Default: S
```

Only specify the keys you want to change.
