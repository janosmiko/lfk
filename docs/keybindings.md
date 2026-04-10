# Keybindings Reference

Complete list of all keybindings in `lfk`. All keybindings can be overridden in `~/.config/lfk/config.yaml` under the `keybindings` section. Only `esc`, `ctrl+c`, and `q` (quit) are hardcoded.

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
| `H` | Toggle rarely used resource types (CSI internals, webhooks, APF, leases, advanced core) in the sidebar (resets each launch) |
| `0` / `1` / `2` | Jump to clusters / types / resources level |
| `J` / `K` | Scroll preview pane down/up |
| `o` | Jump to owner/controller of selected resource |

## Views and Modes

| Key | Action |
|---|---|
| `?` | Toggle help screen |
| `P` | Toggle between details summary and YAML preview |
| | Details pane shows labels, finalizers, annotation count, and resource metadata |
| | Details view shows deletion timestamp (with warning highlight) for resources being deleted |
| `F` | Toggle fullscreen (middle column or dashboard) |
| `.` | Quick filter presets |
| `!` | Error log |
| `Ctrl+S` | Toggle secret value visibility in details pane (YAML preview always shows actual base64 values) |
| `I` | API Explorer (browse resource structure interactively) |
| `U` | RBAC permissions browser (can-i) |
| `M` | Toggle resource relationship map view |
| `w` | Toggle watch mode (auto-refresh every 2s) |
| `,` | Column visibility toggle (show/hide and reorder columns — see [Column Toggle Overlay](#column-toggle-overlay) below) |
| `>` / `<` | Sort by next / previous column |
| `=` | Toggle sort direction (ascending/descending) |
| `-` | Reset sort to default (Name ascending) |
| `W` | Save resource to file / toggle warnings-only filter (Events view) |
| `T` | Switch color scheme (live preview, not persisted) |
| `Ctrl+T` | Toggle terminal mode (pty embedded / exec takeover) |
| `Ctrl+G` | Finalizer search and remove |
| `@` | Monitoring overview (active Prometheus alerts) |
| `Q` | Namespace resource quota dashboard |
| `:` | Command bar: resource jumps (`:pod`, `:dep`), built-ins (`:ns`, `:ctx`, `:set`, `:sort`, `:export`), kubectl (`:k get pod`), shell (`:! cmd`) |

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

| Key | Action | Config key |
|---|---|---|
| `x` | Open action menu (bulk actions when items selected) | `action_menu` |
| `\` | Open namespace selector | `namespace_selector` |
| `A` | Toggle all-namespaces mode | `all_namespaces` |
| `L` | View logs for selected resource | `logs` |
| `e` | Secret/ConfigMap editor (inline key-value editing) | `secret_editor` |
| `E` | Edit selected resource in $EDITOR | `edit` |
| `R` | Refresh current view | `refresh` |
| `v` | Describe selected resource | `describe` |
| `D` | Delete resource (force delete Pod/Job if already deleting, force finalize others) | `delete` |
| `X` | Force delete with grace-period=0 (Pod/Job only) | `force_delete` |
| `S` | Scale / Export resource YAML to file | `scale` / `save_resource` |
| `Ctrl+O` | Open ingress host in browser | `open_browser` |
| `i` | Edit labels/annotations | `label_editor` |
| `a` | Create new resource from template (/ to search) | `create_template` |
| `d` | Diff two selected resources | `diff` |

Port forwarding is available via the action menu (`x`) on Pod, Service, Deployment, StatefulSet, and DaemonSet resources. After creating a port forward, the view automatically navigates to the Port Forwards list and displays the resolved local port in the status bar. Active port forwards can be managed via the "Port Forwards" virtual resource in the Networking group.

Resource-specific actions (exec, scale, restart, secret editor, etc.) are available through the action menu (`x`).

## Clipboard

| Key | Action |
|---|---|
| `y` | Copy resource name to clipboard |
| `Y` | Copy resource YAML to clipboard |
| `Ctrl+P` | Apply resource from clipboard (`kubectl apply`) |

## Multi-Selection

| Key | Action |
|---|---|
| `Space` | Toggle selection on current item (sets anchor) |
| `Ctrl+Space` | Select range from anchor to cursor |
| `Ctrl+A` | Select / deselect all visible items |
| `Esc` | Clear selection |

When items are selected, press `x` to open the bulk action menu (delete, force delete, scale, restart, diff).

## Bookmarks

lfk supports vim-style named marks for quick navigation. A bookmark stores a
resource path (context + namespace + resource type + optional resource name)
under a single-character slot.

Bookmarks come in two flavors depending on the slot case you choose:

- **Context-aware** (lowercase `a-z` / digit `0-9`): remembers the kube
  context you were in when you set it. Jumping to the bookmark switches
  clusters if needed. Use this when you want the bookmark to point at a
  specific environment (e.g., `a` for "production pods").
- **Context-free** (uppercase `A-Z`): doesn't remember a context. Jumping
  to the bookmark uses whatever cluster is currently active. Use this for
  cluster-agnostic shortcuts (e.g., `P` for "whatever cluster I'm in, go
  to Pods").

| Key | Context | Action |
|---|---|---|
| `m<key>` | Explorer | Set mark at current location (`a-z`, `0-9`) |
| `'` | Explorer | Open bookmarks list |
| `a-z` / `0-9` | Bookmark overlay | Jump directly to named mark |
| `j` / `k` | Bookmark overlay | Navigate bookmarks |
| `/` | Bookmark overlay | Filter bookmarks by name |
| `Enter` | Bookmark overlay | Jump to selected bookmark |
| `Ctrl+X` | Bookmark overlay | Delete selected bookmark (with confirmation) |
| `Alt+X` | Bookmark overlay | Delete all bookmarks (with confirmation) |

## YAML View

| Key | Action |
|---|---|
| `j` / `k` | Scroll up/down |
| `h` / `l` | Move cursor column left/right |
| `0` / `$` | Move cursor to line start/end |
| `^` | Move cursor to first non-whitespace character |
| `w` / `b` | Move cursor to next/previous word start |
| `W` / `B` | Move cursor to next/previous WORD start (whitespace-delimited) |
| `e` | Move cursor to end of word |
| `E` | Move cursor to end of WORD (whitespace-delimited) |
| `gg` / `G` | Jump to top / bottom |
| `123G` | Jump to line number |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `/` | Search in YAML |
| `n` / `N` | Next / previous search match |
| `v` | Character visual selection (from cursor column) |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection (from cursor column) |
| `h` / `l` | Move selection column left/right (in visual mode) |
| `y` | Copy selected text (in visual mode) |
| `z` | Toggle fold on section under cursor |
| `Z` | Toggle all folds (collapse/expand all) |
| `Ctrl+W` / `>` | Toggle line wrapping |
| `Ctrl+E` | Edit resource in `$EDITOR` |
| `q` / `Esc` | Back to explorer |

## Describe View

| Key | Action |
|---|---|
| `j` / `k` | Scroll up/down |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `Ctrl+W` / `>` | Toggle line wrapping |
| `q` / `Esc` | Back to explorer |

## Log Viewer

| Key | Action |
|---|---|
| `j` / `k` | Move cursor up/down |
| `h` / `l` / `Left` / `Right` | Move cursor column left/right |
| `0` / `$` | Move cursor to line start/end |
| `^` | Move cursor to first non-whitespace character |
| `w` / `b` | Move cursor to next/previous word start |
| `W` / `B` | Move cursor to next/previous WORD start (whitespace-delimited) |
| `e` | Move cursor to end of word |
| `E` | Move cursor to end of WORD (whitespace-delimited) |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Half page down / up |
| `Ctrl+F` / `Ctrl+B` | Full page down / up |
| `f` | Toggle follow mode (auto-scroll to new logs) |
| `Tab` / `z` / `>` | Toggle line wrapping |
| `#` | Toggle line numbers |
| `s` | Toggle timestamps |
| `p` | Toggle pod/container prefixes |
| `c` | Toggle previous container logs |
| `/` | Search in logs |
| `n` / `N` | Next / previous search match |
| `123G` | Jump to specific line number |
| `S` | Save loaded logs to file |
| `Ctrl+S` | Save all logs to file (full kubectl logs) |
| `v` | Character visual selection (from cursor column) |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection (from cursor column) |
| `h` / `l` | Move selection column left/right (in visual mode) |
| `y` | Copy selected text (in visual mode) |
| `\` | Switch pod / filter containers (space: select, enter: apply, / to filter) |
| `q` / `Esc` | Close log viewer |

> **Tail-first loading**: Logs load the last 1000 lines initially with follow mode enabled. Scrolling to the top automatically loads older log history. Configure with `log_tail_lines` in config.

## Exec Mode (embedded terminal)

`Ctrl+]` is a prefix key (like tmux's `Ctrl+b`). Press it once to activate, then press a follow-up key:

| Key | Action |
|---|---|
| `Ctrl+]` `Ctrl+]` | Exit terminal and return to explorer |
| `Ctrl+]` `]` | Next tab (PTY keeps running in background) |
| `Ctrl+]` `[` | Previous tab (PTY keeps running in background) |
| `Ctrl+]` `t` | New tab (clone current context) |

All other keys are forwarded to the PTY process. The PTY session continues running when you switch tabs, so you can return to it later.

## Diff View

| Key | Action |
|---|---|
| `j` / `k` | Move cursor up/down |
| `h` / `l` | Move cursor column left/right |
| `0` / `$` | Move cursor to line start/end |
| `^` | Move cursor to first non-whitespace |
| `w` / `b` | Move cursor to next/previous word start |
| `W` / `B` | Move cursor to next/previous WORD start (whitespace-delimited) |
| `e` | Move cursor to end of word |
| `E` | Move cursor to end of WORD (whitespace-delimited) |
| `Tab` | Switch cursor side (side-by-side mode) |
| `gg` / `G` | Jump to top / bottom |
| `123G` | Jump to line number |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `/` | Search in diff |
| `n` / `N` | Next / previous search match |
| `v` | Character visual selection |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection |
| `h` / `l` | Move selection column left/right (in visual mode) |
| `y` | Copy selected text (in visual mode) |
| `z` | Toggle fold unchanged section at cursor |
| `Z` | Toggle all folds |
| `#` | Toggle line numbers |
| `Ctrl+W` / `>` | Toggle line wrapping |
| `u` | Toggle unified/side-by-side view |
| `q` / `Esc` | Back to explorer |

## Column Toggle Overlay

Press `,` in the resource list to open the column toggle overlay. It lists
every toggleable column for the current kind — both built-ins (Namespace,
Ready, Restarts, Status, Age) and extras from the resource's
`additionalPrinterColumns`.

| Key | Action |
|---|---|
| `j` / `k` | Navigate up/down |
| `Space` | Toggle the current entry |
| `J` / `K` | Reorder the current entry down/up |
| `/` | Filter entries by name |
| `c` | Clear selection (uncheck every entry) |
| `R` | Reset to defaults for the current kind |
| `Enter` | Apply the selection |
| `Esc` / `q` | Close without saving |

Built-in and extra columns can be freely interleaved — `J`/`K` moves
either kind, so you can put `Age` before `Namespace` or drop an extra
like `IP` between `Ready` and `Status`. The only fixed column is `Name`,
which always renders first and is never listed in the overlay.

The selection you apply is explicit: the table renders exactly the
columns you check, in the exact order they appear in the overlay, and
will not auto-fill the remaining space with unchecked columns. The
chosen order is remembered per resource kind for the duration of the
session (it is not persisted to disk). To start from a clean slate,
press `c` to uncheck every entry at once, then space-select only the
columns you want.

If you apply a completely empty selection (no built-ins, no extras), the
overlay interprets it as "reset to defaults for this kind" rather than
leaving the table empty. To render only built-ins with zero extras, keep
at least one built-in column checked when you press Enter.

## Inline Editors (Secret / ConfigMap / Labels & Annotations)

The Secret, ConfigMap, and Labels/Annotations editors use a shared key-value
overlay. The list view supports vim-like navigation; pressing `e` or `a`
enters edit mode for the selected (or new) entry.

### List view

| Key | Action |
|---|---|
| `j` / `k` | Move cursor up/down |
| `e` | Edit selected key/value |
| `a` | Add a new key/value entry |
| `y` | Copy selected value to clipboard |
| `D` | Delete selected entry |
| `Enter` | Save changes and close (no-op if nothing changed) |
| `Esc` | Close without saving |

The Labels/Annotations editor additionally has a `Tab` binding in the list
view to switch between the labels pane and the annotations pane.

### Edit mode

| Key | Action |
|---|---|
| `Tab` | Switch between key and value fields (in-progress edits in both fields are preserved) |
| `Cmd+V` (macOS) / `Ctrl+Shift+V` (Linux) | Paste from clipboard |
| `Ctrl+S` | Commit the in-progress edit back to the list |
| `Esc` | Cancel the in-progress edit |

> Pressing `Enter` from the list view saves all pending changes via `kubectl
> apply`/`patch` and refreshes the resource. If no fields were modified, the
> overlay closes silently. The previous `s` save shortcut has been removed —
> use `Enter` instead.

## API Explorer

| Key | Action |
|---|---|
| `j` / `k` | Navigate fields |
| `l` / `Enter` | Drill into field (Object/array types) |
| `h` / `Backspace` | Go back one level |
| `/` | Search fields |
| `n` / `N` | Next / previous search match (recursive: auto-drills into children / searches parent) |
| `r` | Recursive field browser (browse all nested fields with filter) |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `q` | Close API explorer |
| `Esc` | Go back one level / close at root |

## Can-I Browser

| Key | Action |
|---|---|
| `j` / `k` | Navigate groups |
| `J` / `K` | Scroll resource list down / up |
| `/` | Search/filter groups by name |
| `a` | Toggle all/allowed-only permissions |
| `s` | Switch subject (User/Group/SA) |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `q` / `Esc` | Clear search / close |

The title bar shows the namespace scope (`ns:...`) used for the permission check, so you can see whether permissions are cluster-wide or namespaced. When checking a service account, its own namespace is used automatically. Users and groups are discovered from ClusterRoleBindings and RoleBindings.

## Can-I Subject Selector

| Key | Action |
|---|---|
| `j` / `k` | Navigate subjects |
| `/` | Filter subjects by name |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Enter` | Select subject |
| `Esc` | Clear filter / close |

## Network Policy Visualizer

| Key | Action |
|---|---|
| `j` / `k` | Scroll up/down |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `q` / `Esc` | Close visualizer |

## Error Log (`!`)

| Key | Action |
|---|---|
| `j` / `k` | Move cursor up/down |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` | Page down / up (full page) |
| `V` | Line visual selection |
| `v` | Character visual selection |
| `h` / `l` | Move cursor column left/right (in character visual mode) |
| `0` / `$` | Move cursor to line start/end (in character visual mode) |
| `y` | Copy selected lines (visual mode) or all entries (normal mode) |
| `f` | Toggle fullscreen / overlay mode |
| `d` | Toggle debug log visibility |
| `Esc` | Cancel visual selection, or close overlay |
| `q` | Close overlay |

> **Fullscreen mode**: Press `f` to expand the error log to full terminal size. This removes the overlay border, so mouse text selection works cleanly without picking up background characters. Press `f` again to return to overlay mode.

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

Press `:` to open the command bar. It supports four types of input:

| Type | Syntax | Examples |
|------|--------|---------|
| Resource jump | `:<type> [namespace...]` | `:pod`, `:dep kube-system`, `:ns prod staging` |
| Built-in | `:<command> [args]` | `:ns` (navigate), `:ns prod` (filter), `:ctx my-cluster`, `:set wrap`, `:sort Age`, `:export yaml` |
| Kubectl | `:k <cmd>` or `:kubectl <cmd>` | `:k get pod`, `:kubectl describe svc nginx` |
| Shell | `:! <command>` | `:! grep error /var/log` |

**Navigation:**

| Key | Action |
|-----|--------|
| `Tab` | Cycle suggestions forward (auto-fills when exactly 1 match) |
| `Shift+Tab` | Cycle suggestions backward |
| `Ctrl+N` / `Down` | Cycle suggestions forward |
| `Ctrl+P` / `Up` | Cycle suggestions backward |
| `Ctrl+D` / `Ctrl+U` | Scroll suggestions (half page down/up) |
| `Ctrl+F` / `Ctrl+B` | Scroll suggestions (full page down/up) |
| `Ctrl+Space` | Open/refresh suggestions |
| `Space` / `Right` | Accept ghost text preview |
| `Enter` | Accept selected suggestion, or execute command when no suggestions |
| `Esc` | Close suggestions first, then close command bar |
| `Up` / `Down` | Browse command history (when no suggestions visible) |
| `Ctrl+W` | Delete word backwards |
| `Ctrl+A` / `Ctrl+E` | Home / End |

**Notes:**
- Resource types use singular form (`:pod`, not `:pods`)
- `:ns` without arguments navigates to Namespaces; with arguments filters to those namespaces
- Kubectl commands inject `--context` and `-n` from current selection automatically
- `Ctrl+U` scrolls suggestions when visible, deletes line before cursor when closed

## General

| Key | Action |
|---|---|
| `T` | Switch color scheme |
| `q` | Quit application (with confirmation) |
| `Esc` | Go back one level / close overlay / quit |
| `Ctrl+C` | Close current tab (quit if last tab) |

## Action Menu Items

The action menu (`x` key) shows context-specific actions based on the resource type:

### Pod Actions
`l` Logs, `s` Exec, `A` Attach, `B` Debug, `b` Debug Pod, `p` Port Forward, `S` Startup Analysis, `v` Describe, `E` Edit, `D` Delete, `X` Force Delete, `V` Events

### Deployment Actions
`l` Logs, `s` Exec, `A` Attach, `S` Scale, `r` Restart, `R` Rollback, `p` Port Forward, `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### StatefulSet Actions
`l` Logs, `s` Exec, `A` Attach, `S` Scale, `r` Restart, `p` Port Forward, `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### DaemonSet Actions
`l` Logs, `s` Exec, `A` Attach, `r` Restart, `p` Port Forward, `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### Service Actions
`l` Logs, `s` Exec (into pod behind service), `A` Attach (to pod behind service), `p` Port Forward, `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### Secret Actions
`e` Secret Editor, `v` Describe, `E` Edit, `D` Delete, `l` Labels / Annotations, `b` Debug Pod, `V` Events

### ConfigMap Actions
`e` ConfigMap Editor, `v` Describe, `E` Edit, `D` Delete, `l` Labels / Annotations, `b` Debug Pod, `V` Events

### Node Actions
`c` Cordon, `u` Uncordon, `n` Drain, `t` Taint, `T` Untaint, `s` Shell, `v` Describe, `E` Edit, `b` Debug Pod, `V` Events

### Job Actions
`l` Logs, `s` Exec, `A` Attach, `v` Describe, `E` Edit, `D` Delete, `X` Force Delete, `b` Debug Pod, `V` Events

### CronJob Actions
`l` Logs, `s` Exec, `A` Attach, `t` Trigger (create Job), `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### ArgoCD Application Actions
`s` Sync, `a` Sync (Apply Only), `f` Diff, `R` Refresh, `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### Helm Release Actions
`u` Values, `A` All Values, `E` Edit Values, `d` Diff, `U` Upgrade, `R` Rollback, `h` History, `v` Describe, `D` Delete, `b` Debug Pod, `V` Events

### Ingress Actions
`o` Open in Browser, `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### PVC Actions
`g` Go to Pod, `b` Debug Mount, `B` Debug Pod, `v` Describe, `E` Edit, `D` Delete, `V` Events

### Default Actions (all other resources)
`v` Describe, `E` Edit, `D` Delete, `l` Labels / Annotations, `b` Debug Pod, `V` Events

### Bulk Actions (when items multi-selected)
`D` Delete, `X` Force Delete, `S` Scale, `r` Restart

ArgoCD Application bulk actions (when Application resources are multi-selected):
`s` Sync, `a` Sync (Apply Only), `R` Refresh

Custom actions defined in the config file appear after the built-in actions.

## Configuring Keybindings

All keybindings can be overridden in `~/.config/lfk/config.yaml`. Only specify the keys you want to change — defaults apply for everything else.

```yaml
keybindings:
  # Navigation
  left: "h"              # Navigate to parent
  right: "l"             # Navigate into item
  down: "j"              # Move cursor down
  up: "k"                # Move cursor up
  jump_top: "g"          # Jump to top (gg)
  jump_bottom: "G"       # Jump to bottom
  page_down: "ctrl+d"    # Half-page down
  page_up: "ctrl+u"      # Half-page up
  page_forward: "ctrl+f" # Full-page down
  page_back: "ctrl+b"    # Full-page up
  preview_down: "J"      # Scroll preview down
  preview_up: "K"        # Scroll preview up
  jump_owner: "o"        # Jump to owner
  toggle_rare: "H"       # Toggle rarely used resource types in the sidebar

  # Views and Modes
  help: "?"              # Toggle help
  filter: "f"            # Filter items
  search: "/"            # Search and jump
  toggle_preview: "P"    # Toggle YAML preview
  resource_map: "M"      # Resource map
  fullscreen: "F"        # Fullscreen toggle
  watch_mode: "w"        # Watch mode
  command_bar: ":"        # Command bar
  theme_selector: "T"    # Theme selector
  finalizer_search: "ctrl+g"  # Finalizer search
  api_explorer: "I"      # API Explorer
  rbac_browser: "U"      # RBAC browser
  secret_toggle: "ctrl+s" # Secret visibility
  error_log: "!"         # Error log
  column_toggle: ","     # Column visibility toggle
  sort_next: ">"         # Sort by next column
  sort_prev: "<"         # Sort by previous column
  sort_flip: "="         # Toggle sort direction
  sort_reset: "-"        # Reset sort to default
  filter_presets: "."    # Quick filter presets
  monitoring: "@"        # Monitoring dashboard
  quota_dashboard: "Q"   # Quota dashboard
  terminal_toggle: "ctrl+t"  # Toggle terminal mode (pty/exec)

  # Actions
  action_menu: "x"       # Action menu
  namespace_selector: "\\" # Namespace selector
  all_namespaces: "A"    # Toggle all-namespaces
  logs: "L"              # View logs
  refresh: "R"           # Refresh view
  restart: "r"           # Restart resource (action menu only)
  exec: "s"              # Exec into container (action menu only)
  edit: "E"              # Edit in $EDITOR
  describe: "v"          # Describe resource
  delete: "D"            # Delete resource
  force_delete: "X"      # Force delete
  scale: "S"             # Scale resource
  label_editor: "i"      # Labels/annotations
  secret_editor: "e"     # Secret/configmap editor
  create_template: "a"   # Create from template
  copy_name: "y"         # Copy name
  copy_yaml: "Y"         # Copy YAML
  paste_apply: "ctrl+p"  # Apply from clipboard
  open_browser: "ctrl+o" # Open in browser
  diff: "d"              # Diff resources

  # Multi-selection
  toggle_select: " "     # Toggle selection (space)
  select_range: "ctrl+@" # Select range (Ctrl+Space)
  select_all: "ctrl+a"   # Select all

  # Tabs
  new_tab: "t"           # New tab
  next_tab: "]"          # Next tab
  prev_tab: "["          # Previous tab

  # Bookmarks
  set_mark: "m"          # Set mark
  open_marks: "'"        # Open bookmarks
```
