# Keybindings Reference

Complete list of all keybindings in `lfk`. All keybindings can be overridden in `~/.config/lfk/config.yaml` under the `keybindings` section. Only `esc`, `ctrl+c`, and `q` (quit) are hardcoded.

## Navigation

| Key | Action |
|---|---|
| `h` / `Left` | Navigate to parent level |
| `l` / `Right` | Navigate into selected item |
| `j` / `Down` | Move cursor down |
| `k` / `Up` | Move cursor up |
| `gg` / `Home` | Jump to top of list |
| `G` / `End` | Jump to bottom of list |
| `Enter` | Open full-screen YAML view / navigate into |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `z` | Toggle expand/collapse all resource groups / toggle event grouping in the Events list |
| `p` | Pin/unpin CRD group (at resource types level) |
| `H` | Toggle rarely used resource types (CSI internals, webhooks, APF, leases, advanced core) in the sidebar (resets each launch) |
| `0` / `1` / `2` | Jump to clusters / types / resources level |
| `J` / `K` | Scroll preview pane down/up |
| `o` / `O` | `o` jumps to the owner/controller of the selected resource; `O` opens the cluster-wide orphan overview overlay |

## Views and Tools

| Key | Action |
|---|---|
| `?` | Toggle help screen |
| `P` | Toggle between details summary and YAML preview |
| | Details pane shows labels, finalizers, annotation count, and resource metadata |
| | Details view shows deletion timestamp (with warning highlight) for resources being deleted |
| `F` | Toggle fullscreen (middle column or dashboard) |
| `M` | Toggle resource relationship map view |
| `,` | Column visibility toggle (show/hide and reorder columns — see [Column Toggle Overlay](#column-toggle-overlay) below) |
| `Ctrl+S` | Toggle secret value visibility in details pane (YAML preview always shows actual base64 values) |
| `I` | API Explorer (browse resource structure interactively) |
| `U` | RBAC permissions browser (can-i) |
| `Shift+O` | Open the cluster-wide Orphan overview |
| `Ctrl+G` | Finalizer search and remove |
| `!` | Error log |
| `@` | Monitoring overview (active Prometheus alerts) |
| `Ctrl+N` | Open the Local Clusters Manager overlay (only at LevelClusters) |
| `Q` | Namespace resource quota dashboard |
| `` ` `` | Scheduler / task queue overlay (Tab toggles running / completed history; `a` toggles show-all entries in completed view) |
| `:` | Command bar: resource jumps (`:pod`, `:dep`), built-ins (`:ns`, `:ctx`, `:set`, `:sort`, `:export`, `:scheduler`), kubectl (`:k get pod`), shell (`:! cmd`) |

## Sorting

| Key | Action |
|---|---|
| `>` / `<` | Sort by next / previous column |
| `=` | Toggle sort direction (ascending/descending) |
| `-` | Reset sort to default (Name ascending) |

## Modes & Settings

| Key | Action |
|---|---|
| `w` | Toggle watch mode (auto-refresh every 2s) |
| `Ctrl+R` | Toggle read-only mode (cluster picker: highlighted row's [RO] marker; inside a context: current tab) |
| `T` | Switch color scheme (live preview, not persisted) |
| `Ctrl+T` | Cycle terminal mode (pty / exec / mux — mux skipped without tmux/zellij) |

## Orphan filter presets

### Cluster-wide overview

Press **`Shift+O`** anywhere in the explorer (or type `:orphans` with no arguments in the command bar) to open the cluster-wide orphan overview overlay. Inside the overlay:

| Key | Action |
| --- | ------ |
| `Tab` / `Shift+Tab` | Cycle kind filter chips (All / Pods / Secrets / CMs / Svcs / PVCs / HPAs / PDBs / NetPols / Roles / RBs) |
| `s` | Toggle strict / lenient — strict (default) hides items referenced by workload templates; lenient surfaces them |
| `/` | Filter by namespace + name |
| `Enter` | Jump to the highlighted resource (namespace switches automatically) |
| `R` | Re-scan the cluster |
| `Esc` / `q` / `Shift+O` | Close the overlay |

### Per-kind presets

The `.` filter-preset overlay surfaces these orphan-detection presets when the active resource list is one of the supported kinds. Every orphan preset binds to the same hotkey **`O`** so there is one mnemonic to remember; the per-kind preset name still distinguishes the underlying check.

| Resource list | Preset name | Match |
| --- | --- | --- |
| Pods | Orphans | No owner reference (excludes static / mirror pods) |
| Secrets | Unmounted | No Pod / template / Ingress / SA refers to it |
| ConfigMaps | Unmounted | No Pod or workload template refers to it |
| Services | No Endpoints | Zero ready+notReady endpoints |
| PersistentVolumeClaims | Unused | Not mounted by any Pod or template |
| HorizontalPodAutoscalers | Dangling | `scaleTargetRef` points to a missing workload |
| PodDisruptionBudgets | Dangling | Selector matches no live / templated pods |
| NetworkPolicies | Dangling | `podSelector` matches no live / templated pods |
| Roles | Unbound | No RoleBinding refers to it (ClusterRoleBinding can't reference a Role) |
| ClusterRoles | Unbound | No RoleBinding / ClusterRoleBinding refers to it |
| RoleBindings / ClusterRoleBindings | Dangling | Missing role or empty subjects |

`:orphans <kind>` (e.g. `:orphans pods`, `:orphans pvcs`, `:orphans rolebindings`) is a shortcut that jumps to the kind's list with the matching preset already applied.

Auto-excluded from "Unmounted" results:
- Helm release Secrets (`type=helm.sh/release.v1`)
- ServiceAccount tokens (`type=kubernetes.io/service-account-token`)
- `kube-root-ca.crt` ConfigMap
- Anything with an `ownerReference` (cert-manager Certificates, etc.)

Auto-excluded from Pod "Orphans":
- Static / mirror pods (kubelet-managed via `kubernetes.io/config.mirror` annotation)

Terminal pods (Succeeded/Failed) older than 1h are still flagged but the reason is `"no owner (terminal)"` to distinguish them from live workloads.

## Search and Filter

| Key | Action |
|---|---|
| `f` | Start filter mode (filter items in current view) |
| `/` | Start search mode (search and jump to match) |
| `.` | Quick filter presets |
| `Tab` | Inside `/` or `f`: toggle broad mode — also matches against column values (annotations, labels, finalizers, CRD printer columns, custom user columns). Prompt shows `filter (all):` / `search (all):` while on. Resets on Enter/Esc. |
| `Up` / `Down` | Inside `/` or `f`: cycle through previous queries (shared persistent history). |
| `n` | Jump to next search match |
| `N` | Jump to previous search match |
| `Esc` | Clear filter / cancel search |

Search supports abbreviated resource type names (e.g., `pvc`, `hpa`, `deploy`).

`/` and `f` share one persistent history at `$XDG_STATE_HOME/lfk/query-history` (default `~/.local/state/lfk/query-history`) — both inputs accept the same query syntax and match against the same fields, so a query confirmed in one mode is recallable from the other. The `:` command bar keeps its own `history` file because its inputs are kubectl-shaped commands rather than resource-name queries.

## Actions

| Key | Action | Config key |
|---|---|---|
| `x` | Open action menu (bulk actions when items selected) | `action_menu` |
| `\` | Open namespace selector | `namespace_selector` |
| `A` | Toggle all-namespaces mode (also works inside the namespace selector — clears individual selections and enables all-ns) | `all_namespaces` |
| `L` | View logs for selected resource | `logs` |
| `e` | Secret/ConfigMap editor (inline key-value editing) | `secret_editor` |
| `E` | Edit selected resource in $EDITOR | `edit` |
| `R` | Refresh current view | `refresh` |
| `v` | Describe selected resource | `describe` |
| `D` | Delete resource (force delete Pod/Job if already deleting, force finalize others) | `delete` |
| `X` | Force delete (Pod/Job only) | `force_delete` |
| `S` | Scale resource (Deployment / StatefulSet / ReplicaSet) | `scale` |
| `W` | Save resource to file / toggle warnings-only filter (Events view) | `save_resource` |
| `Ctrl+O` | Open ingress host in browser | `open_browser` |
| `i` | Edit labels/annotations | `label_editor` |
| `a` | Create new resource from template (/ to search) | `create_template` |
| `d` | Diff two selected resources | `diff` |

Events list also groups duplicate events (same Type/Reason/Message/Object) by default; press `z` to toggle grouping.

Port forwarding is available via the action menu (`x`) on Pod, Service, Deployment, StatefulSet, and DaemonSet resources. After creating a port forward, the view automatically navigates to the Port Forwards list and displays the resolved local port in the status bar. Active port forwards can be managed via the "Port Forwards" virtual resource in the Networking group.

Resource-specific actions (exec, scale, restart, secret editor, etc.) are available through the action menu (`x`).

## Clipboard

| Key | Action |
|---|---|
| `y` | Copy resource name to clipboard (with multi-selection: newline-joined names of all selected items) |
| `Y` | Copy resource YAML to clipboard (with multi-selection: multi-doc YAML, items joined with `---`) |
| `Ctrl+P` | Apply resource from clipboard (`kubectl apply`) |

When items are multi-selected (`Space` / `Ctrl+Space` / `Ctrl+A`), `y` and `Y` operate on the selection rather than the cursor row — mirroring the precedence used by `D` (delete) and other bulk actions. `Y` is capped at 50 manifests per copy (client-go's default rate limiter serializes the per-item fetches).

## Multi-Selection

| Key | Action |
|---|---|
| `Space` | Toggle selection on current item (sets anchor) |
| `Ctrl+Space` | Select range from anchor to cursor |
| `Ctrl+A` | Select / deselect all visible items |
| `Esc` | Clear selection |

When items are selected, press `x` to open the bulk action menu (delete, force delete, scale, restart, diff).

## Bookmarks

Vim-style named marks for quick navigation. A bookmark stores a resource
path (context + namespace + resource type + optional resource name) under
a single-character slot.

- **Context-aware** (`a-z` / `0-9`): remembers the kube context; jumping
  switches clusters.
- **Context-free** (`A-Z`): uses the tab's current cluster; for
  cluster-agnostic shortcuts.

| Key | Context | Action |
|---|---|---|
| `m<key>` | Explorer | Set mark at current location (`a-z`, `0-9`) |
| `'` | Explorer | Open bookmarks list |
| `a-z` / `0-9` | Bookmark overlay | Jump directly to named mark |
| `j` / `k` | Bookmark overlay | Navigate bookmarks |
| `/` | Bookmark overlay | Filter bookmarks by name |
| `Enter` | Bookmark overlay | Jump to selected bookmark |
| `Tab` | Bookmark overlay | Toggle `[LOAD NAMESPACE]` — apply the bookmark's saved namespace scope on the next jump |
| `Ctrl+X` | Bookmark overlay | Delete selected bookmark (with confirmation) |
| `Alt+X` | Bookmark overlay | Delete all bookmarks (with confirmation) |

> Namespace on jump: by default the tab's current namespace is kept. Press
> `Tab` in the overlay to arm `[LOAD NAMESPACE]`; the saved namespace is
> then applied on the next jump and the flag is cleared on close.

## Help View

| Key | Action |
|---|---|
| `/` | Search — highlights matches inline without removing non-matching lines |
| `Ctrl+N` / `Ctrl+P` | Next / previous match while typing the search input |
| `Enter` | Apply search (closes input, keeps highlights, arms `n`/`N`) |
| `n` / `N` | Jump to next / previous search match (after Enter) |
| `f` | Filter — narrows the visible list to lines matching the query |
| `Esc` | Cascades: clear active search → clear active filter → close help |
| `j` / `k` | Scroll down/up |
| `Ctrl+D` / `Ctrl+U` | Half-page scroll down/up |
| `Ctrl+F` / `Ctrl+B` / `PgDown` / `PgUp` | Full-page scroll |
| `g` / `G` | Jump to top / bottom |
| `q` / `?` / `F1` | Close help |

## YAML View

| Key | Action |
|---|---|
| `j` / `k` | Scroll up/down |
| `123j` / `123k` | Move cursor down/up N visible lines (count-prefixed motion; folds skipped) |
| `h` / `l` | Move cursor column left/right |
| `123h` / `123l` | Move cursor column left/right by N runes |
| `0` / `$` | Move cursor to line start/end |
| `^` | Move cursor to first non-whitespace character |
| `w` / `b` | Move cursor to next/previous word start |
| `W` / `B` | Move cursor to next/previous WORD start (whitespace-delimited) |
| `e` | Move cursor to end of word |
| `E` | Move cursor to end of WORD (whitespace-delimited) |
| `123w` / `123b` / `123e` (and capitals) | Apply word/WORD motion N times |
| `gg` / `Home` | Jump to top |
| `G` / `End` | Jump to bottom |
| `123G` | Jump to line number |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `123 Ctrl+D` / `123 Ctrl+U` | Scroll N lines (vim `'scroll'` semantics: sets the sticky step shared between Ctrl+D and Ctrl+U; clamped to viewport) |
| `123 Ctrl+F` / `123 Ctrl+B` | Page motion scaled by N |
| `/` | Search in YAML |
| `n` / `N` | Next / previous search match |
| `123n` / `123N` | Jump to Nth next / previous search match |
| `v` | Character visual selection (from cursor column) |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection (from cursor column) |
| `h` / `l` | Move selection column left/right (in visual mode) |
| `viw` / `vaw` / `viW` / `vaW` | Select inner/around word (or WORD) under cursor |
| `y` | Copy line under cursor (or selection in visual mode) |
| `123y` | Copy number of lines from cursor (count-prefixed yank; folds skipped) |
| `z` | Toggle fold on section under cursor |
| `Z` | Toggle all folds (collapse/expand all) |
| `Ctrl+W` / `>` | Toggle line wrapping |
| `Ctrl+E` | Edit resource in `$EDITOR` |
| `q` / `Esc` | Back to explorer |

## Describe View

| Key | Action |
|---|---|
| `j` / `k` | Move cursor up/down |
| `123j` / `123k` | Move cursor down/up N lines (count-prefixed motion) |
| `h` / `l` | Move cursor column left/right |
| `123h` / `123l` | Move cursor column left/right by N runes |
| `0` / `$` / `^` | Move cursor to line start / end / first non-whitespace |
| `w` / `b` / `e` / `W` / `B` / `E` | Word / WORD motions |
| `123w` / `123b` / `123e` (and capitals) | Apply word/WORD motion N times |
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `123G` | Jump to line number |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `123 Ctrl+D` / `123 Ctrl+U` | Scroll N lines (vim `'scroll'` semantics: sets the sticky step shared between Ctrl+D and Ctrl+U; clamped to viewport) |
| `123 Ctrl+F` / `123 Ctrl+B` | Page motion scaled by N |
| `/` | Search in content |
| `n` / `N` | Next / previous search match |
| `123n` / `123N` | Jump to Nth next / previous search match |
| `v` | Character visual selection |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection |
| `viw` / `vaw` / `viW` / `vaW` | Select inner/around word (or WORD) under cursor |
| `y` | Copy line under cursor (or selection in visual mode) |
| `123y` | Copy number of lines from cursor (count-prefixed yank) |
| `Ctrl+W` / `>` | Toggle line wrapping |
| `q` / `Esc` | Back to explorer |

## Log Viewer

| Key | Action |
|---|---|
| `j` / `k` | Move cursor up/down |
| `123j` / `123k` | Move cursor down/up N lines (count-prefixed motion) |
| `h` / `l` / `Left` / `Right` | Move cursor column left/right |
| `123h` / `123l` | Move cursor column left/right by N runes |
| `0` / `$` | Move cursor to line start/end |
| `^` | Move cursor to first non-whitespace character |
| `w` / `b` | Move cursor to next/previous word start |
| `W` / `B` | Move cursor to next/previous WORD start (whitespace-delimited) |
| `e` | Move cursor to end of word |
| `E` | Move cursor to end of WORD (whitespace-delimited) |
| `123w` / `123b` / `123e` (and capitals) | Apply word/WORD motion N times |
| `gg` / `Home` | Jump to top |
| `G` / `End` | Jump to bottom |
| `Ctrl+D` / `Ctrl+U` | Half page down / up |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Full page down / up |
| `123 Ctrl+D` / `123 Ctrl+U` | Scroll N lines (vim `'scroll'` semantics: sets the sticky step shared between Ctrl+D and Ctrl+U; clamped to viewport) |
| `123 Ctrl+F` / `123 Ctrl+B` | Page motion scaled by N |
| `f` | Toggle follow mode (auto-scroll to new logs) |
| `Tab` / `z` / `>` | Toggle line wrapping |
| `#` | Toggle line numbers |
| `s` | Toggle timestamps |
| `p` | Toggle pod/container prefixes |
| `P` | Toggle structured preview side panel (JSON / logfmt / klog / zap / nginx / envoy / java / postgres / plain text) |
| `J` / `K` | Scroll the preview side panel down / up (one row, no-op when panel is hidden) |
| `c` | Toggle previous container logs |
| `/` | Search in logs |
| `Up` / `Down` | Inside `/`: cycle through previous log search queries (persistent history). |
| `n` / `N` | Next / previous search match |
| `123n` / `123N` | Jump to Nth next / previous search match |
| `123G` | Jump to specific line number |
| `S` | Save loaded logs to file (path copied to clipboard) |
| `Ctrl+S` | Save all logs to file, full kubectl logs (path copied to clipboard) |
| `v` | Character visual selection (from cursor column) |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection (from cursor column) |
| `h` / `l` | Move selection column left/right (in visual mode) |
| `viw` / `vaw` / `viW` / `vaW` | Select inner/around word (or WORD) under cursor |
| `y` | Copy line under cursor (or selection in visual mode) |
| `123y` | Copy number of lines from cursor (count-prefixed yank) |
| `\` | Switch pod / filter containers (space: select, enter: apply, / to filter) |
| `q` / `Esc` | Close log viewer |

The log viewer's `/` keeps its own persistent history at `$XDG_STATE_HOME/lfk/log-search-history` (default `~/.local/state/lfk/log-search-history`), separate from the explorer's `query-history`. Log search matches raw log lines (substring/regex over arbitrary text) rather than resource names, so pooling the two would surface irrelevant entries on Up/Down in either context.

Tail-first loading: Full Logs (`L` key or action menu `L`) load the last 1000 lines initially (configurable via `log_tail_lines`). Tail Logs (action menu `l`) load only the last 10 lines (configurable via `log_tail_lines_short`). Scrolling to the top loads older history.

Auto-reconnect across init containers: when viewing logs for a single Pod in all-containers mode (no specific container selected via `\`), the stream automatically reconnects each time kubectl exits — e.g. as init containers transition. The reconnect is silent. After several consecutive empty reconnects the viewer stops retrying.

## Exec Mode (embedded terminal)

`Ctrl+]` is a prefix key (like tmux's `Ctrl+b`). Press it once to activate, then press a follow-up key:

| Key | Action |
|---|---|
| `Ctrl+]` `Ctrl+]` | Exit terminal and return to explorer |
| `Ctrl+]` `]` | Next tab (PTY keeps running in background) |
| `Ctrl+]` `[` | Previous tab (PTY keeps running in background) |
| `Ctrl+]` `t` | New tab (clone current context) |
| `Ctrl+]` `Ctrl+U` / `Ctrl+D` | Scroll back / forward by half a viewport |
| `Ctrl+]` `Ctrl+B` / `Ctrl+F` | Scroll back / forward by a full viewport |
| `Ctrl+]` `g` / `G` | Jump to oldest captured line / back to live |
| Mouse wheel | Scroll the PTY scrollback (1 line per tick) |

All other keys are forwarded to the PTY process. The PTY session continues running when you switch tabs, so you can return to it later. Typing any character snaps the view back to the live shell so you don't accidentally type into history.

### Scrollback

Each PTY tab keeps a ring of up to 5000 ANSI-stripped lines captured from the byte stream. Use `Ctrl+]` then `Ctrl+U` / `Ctrl+D` / `Ctrl+B` / `Ctrl+F` to navigate it; `Ctrl+]` `g` / `G` jump to the oldest captured line / back to live. The hint bar shows `scrolled N` while you're not at the live tail. Full-screen curses programs (vim, less, htop) write absolute-position output that the line-stream capture can't reconstruct cleanly — their scrollback view will look messy while they're running, but normal output cleans up afterward. If you need precise scrollback, switch to `exec` or `mux` mode (`Ctrl+T`) so the host terminal's own scrollback handles it.

### Selecting and copying text

Inside the embedded PTY view the host terminal handles selection. Use
`Shift+Drag` for a normal selection; on macOS, `Shift+Option+Drag` (or
`Alt+Drag` on Linux/Windows) selects a rectangular block.

If you need full host-terminal capabilities (scrollback, native search,
unrestricted copy), cycle to `exec` or `mux` mode with `Ctrl+T`, or set
the desired default in the config (`terminal: exec` or `terminal: mux`).

### Terminal modes

| Mode | What happens when an interactive shell runs |
|---|---|
| `pty` (default) | Shell embeds inside lfk via an internal vt10x terminal. Selection works via `Shift+Drag`. |
| `exec` | lfk hands the host terminal to the shell via `tea.ExecProcess` and resumes once it exits. |
| `mux` | Shell opens in a new window (tmux) or floating pane (zellij) of the surrounding multiplexer. lfk stays foregrounded alongside. Errors out if no multiplexer is detected. |

`Ctrl+T` cycles `pty -> exec -> mux -> pty`. Mux is skipped automatically
when no tmux/zellij is detected, so the cycle becomes `pty -> exec ->
pty` in that case. The mode is process-local — restart-persistence comes
from `terminal:` in the config.

## Diff View

| Key | Action |
|---|---|
| `j` / `k` | Move cursor up/down |
| `123j` / `123k` | Move cursor down/up N lines (count-prefixed motion) |
| `h` / `l` | Move cursor column left/right |
| `123h` / `123l` | Move cursor column left/right by N runes |
| `0` / `$` | Move cursor to line start/end |
| `^` | Move cursor to first non-whitespace |
| `w` / `b` | Move cursor to next/previous word start |
| `W` / `B` | Move cursor to next/previous WORD start (whitespace-delimited) |
| `e` | Move cursor to end of word |
| `E` | Move cursor to end of WORD (whitespace-delimited) |
| `123w` / `123b` / `123e` (and capitals) | Apply word/WORD motion N times |
| `Tab` | Switch cursor side (side-by-side mode) |
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `123G` | Jump to line number |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `123 Ctrl+D` / `123 Ctrl+U` | Scroll N lines (vim `'scroll'` semantics: sets the sticky step shared between Ctrl+D and Ctrl+U; clamped to viewport) |
| `123 Ctrl+F` / `123 Ctrl+B` | Page motion scaled by N |
| `/` | Search in diff |
| `n` / `N` | Next / previous search match |
| `123n` / `123N` | Jump to Nth next / previous search match |
| `v` | Character visual selection |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection |
| `h` / `l` | Move selection column left/right (in visual mode) |
| `viw` / `vaw` / `viW` / `vaW` | Select inner/around word (or WORD) under cursor |
| `y` | Copy line under cursor (or selection in visual mode) |
| `123y` | Copy number of lines from cursor (count-prefixed yank; empty-side lines skipped) |
| `z` | Toggle fold unchanged section at cursor |
| `Z` | Toggle all folds |
| `#` | Toggle line numbers |
| `Ctrl+W` / `>` | Toggle line wrapping |
| `u` | Toggle unified/side-by-side view |
| `q` / `Esc` | Back to explorer |

## Event Timeline

Press `V` on a resource (or open the Events list and press `Enter` on an event) to open the Event Timeline overlay. Press `f` to toggle between the overlay and a fullscreen viewer that takes over the whole window.

| Key | Action |
|---|---|
| `j` / `k` | Move cursor down/up |
| `123j` / `123k` | Move cursor down/up N lines (count-prefixed motion) |
| `h` / `l` / `Left` / `Right` | Move cursor column left/right |
| `123h` / `123l` | Move cursor column left/right by N runes |
| `0` / `$` | Move cursor to line start/end |
| `^` | Move cursor to first non-whitespace |
| `w` / `b` | Move cursor to next/previous word start |
| `W` / `B` | Move cursor to next/previous WORD start (whitespace-delimited) |
| `e` | Move cursor to end of word |
| `E` | Move cursor to end of WORD (whitespace-delimited) |
| `123w` / `123b` / `123e` (and capitals) | Apply word/WORD motion N times |
| `gg` / `Home` | Jump to top |
| `G` / `End` | Jump to bottom |
| `123G` | Jump to specific line number |
| `Ctrl+D` / `Ctrl+U` | Half page down / up |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Full page down / up |
| `123 Ctrl+D` / `123 Ctrl+U` | Scroll N lines (vim `'scroll'` semantics: sets the sticky step shared between Ctrl+D and Ctrl+U; clamped to viewport) |
| `123 Ctrl+F` / `123 Ctrl+B` | Page motion scaled by N |
| `f` | Toggle fullscreen event viewer |
| `Tab` / `z` / `>` | Toggle line wrapping |
| `/` | Search in events |
| `n` / `N` | Next / previous search match |
| `123n` / `123N` | Jump to Nth next / previous search match |
| `v` | Character visual selection (from cursor column) |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection (from cursor column) |
| `viw` / `vaw` / `viW` / `vaW` | Select inner/around word (or WORD) under cursor |
| `y` | Copy line under cursor (or selection in visual mode) |
| `123y` | Copy N lines from cursor (count-prefixed yank) |
| `?` / `F1` | Open this help, scrolled to the Event Timeline section |
| `q` / `Esc` | Close overlay (or exit fullscreen back to overlay) |

> Events are pulled from the cluster, correlated to the selected resource (or shown cluster-wide on the timeline overlay), and grouped when their Type/Reason/Message/Object match. The line buffer (1-9 then a motion key) is consumed after each motion, so `5j 3k` jumps down 5 then up 3 without any digit leaking into the next command.

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
| `y` | Copy: cursor row's value when nothing is selected, **opens the format picker automatically when 1+ rows are selected** (so you don't silently copy a single value while ignoring the marked bundle) |
| `Space` | Toggle selection on the current row (cursor auto-advances; works across non-adjacent rows) |
| `Y` | Always open the format picker; copies selected rows (or the cursor row) as YAML / JSON / dotenv / `key=value` / values-only |
| `/` | Filter the list by key (typing extends the query, `Enter` applies, `Esc` clears) |
| `D` | Delete selected entry |
| `Enter` | Save changes and close (no-op if nothing changed) |
| `Esc` | Close without saving |

The Labels/Annotations editor additionally has a `Tab` binding in the list
view to switch between the labels pane and the annotations pane. Switching
tabs clears the multi-row selection (label and annotation namespaces are
disjoint).

### Format picker (Shift+Y)

When the format picker is open, the bottom hint bar swaps to picker controls:

| Key | Action |
|---|---|
| `h` / `l` (or `←` / `→`) | Move the format cursor |
| `Enter` | Copy selected rows in the chosen format and close the picker |
| `Esc` | Cancel without copying |

Selection wins over the cursor: if `s` was used to mark rows, those rows
are the apply target; otherwise the cursor row is copied alone.

### Edit mode

The editor picks one of two modes based on the value being edited:

- **Inline edit (single-line values)** — the cursor moves into the
  table cell of the row being edited. Surrounding rows stay visible
  for context. Used for short values like passwords / tokens / labels.
- **Pane edit (multi-line values)** — the table is replaced with a
  bordered Key + Value pane that handles newlines, scrolling, and
  page navigation. Used for values containing `\n` (TLS certs,
  dotenv blocks, multi-line config files). The editor switches modes
  automatically when you insert a newline with `Enter`.

| Key | Action |
|---|---|
| `Tab` | Switch between key and value fields (in-progress edits in both fields are preserved) |
| `Cmd+V` (macOS) / `Ctrl+Shift+V` (Linux) | Paste from clipboard |
| `Ctrl+S` | Commit the in-progress edit back to the list |
| `Esc` | Cancel the in-progress edit |
| `←` / `→` | Move cursor left / right |
| `↑` / `↓` | Move cursor up / down (preserves byte column on the prev/next `\n`-delimited line; pane mode only) |
| `Ctrl+D` / `Ctrl+U` | Scroll cursor down / up by half a page (pane mode only) |
| `Ctrl+F` / `Ctrl+B` | Scroll cursor down / up by a full page (pane mode only) |
| `Ctrl+A` / `Ctrl+E` | Move cursor to start / end of the **current line** (vim-like `0` / `$`) |
| `Backspace` | Delete the character before the cursor |
| `Ctrl+W` | Delete the word before the cursor |
| `Enter` | Insert a newline (switches to pane mode if value was previously single-line) |

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
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
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
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `Tab` | Toggle to **Who-Can** (reverse RBAC: verb + resource → subjects) |
| `q` / `Esc` | Clear search / close |

The title bar shows the namespace scope (`ns:...`) used for the permission check, so you can see whether permissions are cluster-wide or namespaced. When checking a service account, its own namespace is used automatically. Users and groups are discovered from ClusterRoleBindings and RoleBindings.

## Who-Can (Reverse RBAC)

Reachable from the Can-I browser via `Tab`. Inverts the question: instead
of "what can this subject do", asks "who can do this verb on this
resource". Pure RBAC scan — walks every `ClusterRoleBinding` plus the
`RoleBinding`s in the active namespace scope, resolves their roles, and
lists subjects whose bound rules match.

Layout is two columns: a scrollable **Resources** picker on the left
(deduped union of every resource the Can-I view knows about) and the
**Subjects** result table on the right. Moving the cursor on the picker
fires a fresh query so the right pane updates as you browse.

| Key | Action |
|---|---|
| `j` / `k` (or `↓` / `↑`) | Move the resource cursor (re-queries for the new resource) |
| `J` / `K` | Scroll the subjects column (right pane) without moving the resource cursor |
| `g` / `G` (or `Home` / `End`) | Jump to top / bottom of the resource list |
| `Ctrl+D` / `Ctrl+U` | Half page down / up in the resource list |
| `Ctrl+F` / `Ctrl+B` (or `PgDn` / `PgUp`) | Full page down / up in the resource list |
| `←` / `→` (or `h` / `l`) | Cycle the verb chip (`get` `list` `watch` `create` `update` `patch` `delete` `*`) |
| `/` | Filter the resource list by substring (Enter to accept, Esc to clear) |
| `A` | Toggle namespace scope (all-namespaces ⇄ active namespace) |
| `Tab` | Back to forward Can-I view |
| `q` / `Esc` | Close overlay |

The result table shows `SUBJECT | KIND | NAMESPACE | VIA`. The `VIA`
column records the binding chain (`ClusterRoleBinding/foo → ClusterRole/bar`
or `RoleBinding/ns/foo → Role/bar`) so a user can audit *why* a subject
has access.

ClusterRoleBindings always count regardless of namespace scope (cluster-wide
grants apply everywhere); RoleBindings outside the active scope are excluded.

## Can-I Subject Selector

| Key | Action |
|---|---|
| `j` / `k` | Navigate subjects |
| `/` | Filter subjects by name |
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `Enter` | Select subject |
| `Esc` | Clear filter / close |

## Network Policy Visualizer

| Key | Action |
|---|---|
| `j` / `k` | Scroll up/down |
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `q` / `Esc` | Close visualizer |

## Right-sizing Advisor

Opens via the action menu (`x` → "Right-sizing", default key `z`) on Pod, Deployment,
StatefulSet, DaemonSet, Job, CronJob. Shows per-container CPU + memory recommendations
from one of several strategies. The header chips (`Strategy: <label> [N/M]` and
`Headroom: <H>x [N/M]`) show the active strategy and headroom along with their position
in the available cycles.

Available strategies (priority order; unavailable ones are skipped):

1. **VPA** — VerticalPodAutoscaler recommender (history-based). Available when a VPA
   targets the workload. The recommender's target is multiplied by the active headroom
   (raw target at headroom = 1.0).
2. **1d-max** — Prometheus `max_over_time` peak over the last 1 day × headroom.
   Available when a Prometheus endpoint is configured for the cluster.
3. **1d-avg** — Prometheus `avg_over_time` over the last 1 day × headroom.
4. **7d-p95** — Prometheus `quantile_over_time(0.95, ...)` over the last 7 days × headroom.
5. **snapshot** — current metrics-server usage × headroom (always available as the
   fallback).

The headroom multiplier is the safety-margin factor applied on top of the strategy's raw
output. Cycle through `1.0`, `1.1`, `1.25`, `1.5`, `1.75`, `2.0` with `<` and `>`.
Default is `1.25` (the closest preset to lfk's previous hardcoded `1.2` factor —
existing recommendations stay visually similar after the upgrade).

| Key | Action |
|---|---|
| `y` | Copy recommendations as a strategic-merge `containers[]` YAML block (pasteable into `kubectl patch`) |
| `r` | Force-refresh (invalidate the cached entry for the active strategy + headroom and re-fetch) |
| `]` | Cycle to the next available strategy (wraps around) |
| `[` | Cycle to the previous available strategy (wraps around) |
| `>` | Cycle to the next headroom multiplier (wraps around; snaps to nearest preset on first press if the active value isn't in the list) |
| `<` | Cycle to the previous headroom multiplier (wraps around; same snap behavior as `>`) |
| `j` / `k` | Scroll up / down |
| `g` / `G` (or `Home` / `End`) | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` (or `PgDn` / `PgUp`) | Page down / up (full page) |
| `esc` / `q` | Close |

> The Usage column always reflects live metrics-server usage regardless of strategy —
> only the SUGGESTION column changes based on the algorithm and headroom. Each
> (strategy, headroom) pair's payload is cached for the user session via
> `Model.rightsizingCache`; reopening the overlay reuses the cache so revisits
> are instant. Cleared on `r` refresh or when the kube context / namespace changes.

### Defaults & stickiness

Strategy and headroom selections are sticky for the session. The first-open
seed comes from two optional config keys:

```yaml
rightsizing_defaults:
  strategy: vpa       # vpa | prom_max_1d | prom_avg_1d | prom_p95_7d | snapshot
  headroom: 1.25      # 1.0 | 1.1 | 1.25 | 1.5 | 1.75 | 2.0
```

Fallback chain (highest priority first): sticky session value → config
default → built-in default (first available strategy + `1.25` headroom).
Invalid config values are dropped at startup with a warning in the error log.

## Error Log (`!`)

| Key | Action |
|---|---|
| `j` / `k` | Move cursor up/down |
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
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

## Read-Only Mode

| Key | Action |
|---|---|
| `Ctrl+R` (inside a context) | Toggle read-only mode for the current tab. Session-scoped — does not write to config and does not leak across context switches. Blocked when `--read-only` is set. |
| `Ctrl+R` (at the cluster picker) | Toggle the `[RO]` marker on the highlighted context row. Stored as a session override that wins over per-context config and is honored on context entry. Blocked when `--read-only` is set (the CLI flag forces every context RO). |

The `[RO]` badge appears in the title bar only when you're inside a
context that's locked. At the cluster picker each row shows a `[RO]`
suffix for contexts configured read-only (per-context config, global
config, or the `--read-only` CLI flag). Mutating actions (delete, edit,
scale, restart, exec, port-forward, drain, cordon, etc.) are filtered
out of the action menu and gated at the dispatcher with a "Read-only
mode: X disabled" toast. See [Read-Only Mode](usage.md#read-only-mode)
for the full precedence rules across the CLI flag, per-context config,
and global config.

## Cluster Color Coding

| Key | Action |
|---|---|
| `L` (Shift+L) (at the cluster picker) | Open color picker for the highlighted cluster (saved to `$XDG_STATE_HOME/lfk/cluster-colors.yaml`). Also reachable via `x` → "Set color…". |

When a context has a color assigned, the cluster picker row shows a
small background-tinted suffix swatch on the right edge in that color,
and entering the context applies the same color as a background tint to
the entire title bar so it's impossible to miss which environment you're
acting on. Contexts without a color render a neutral placeholder in the
swatch column so all rows stay aligned.

Four colors (`red`, `yellow`, `green`, `blue`) follow lfk's active
theme tokens (`theme.Error`, `theme.Warning`, `theme.Secondary`,
`theme.Primary`) so a colorscheme switch re-skins them. The remaining
four (`magenta`, `cyan`, `white`, `gray`) stay on ANSI bright codes
(8, 13–15) and look the same regardless of theme.

## Local Clusters Manager

The manager overlay (`Ctrl+N` at LevelClusters) is the single home for
creating, switching, and lifecycle-managing kind / k3d / minikube
clusters from inside lfk.

### List view

| Key | Action |
|---|---|
| `j` / `k` / `Up` / `Down` | Move cursor |
| `n` | New local cluster (opens 5-step wizard) |
| `s` | Start the highlighted cluster (greyed for kind) |
| `Shift+S` | Stop the highlighted cluster (greyed for kind) |
| `Shift+D` | Delete the highlighted cluster (asks for `DELETE` typed confirmation) |
| `Shift+R` | Refresh the list |
| `Enter` | Switch to the highlighted cluster's context and close the overlay |
| `q` / `Esc` / `Ctrl+N` | Close the overlay |

### Wizard

| Key | Action |
|---|---|
| `j` / `k` (provider step) | Pick provider |
| Type | Fill the active text field (name / version / nodes) |
| `Enter` | Advance to the next step (blocks on validation errors) |
| `Esc` | Back up one step (or close from step 1) |

### Delete confirmation

Type the literal word `DELETE` (uppercase) and press `Enter` to confirm,
or `Esc` to cancel.

## Mouse

| Input | Action |
|---|---|
| Click left pane | Drill out one level (same as `h` / Left) |
| Click middle pane (different row) | Select row and preview it in the right pane |
| Click middle pane (already-cursored row) | Drill into it (same as `Enter` / Right) |
| Click right pane | Drill into the selected item |
| Click table header | Sort by that column; click again toggles direction |
| Right-click middle pane | Move cursor to clicked row and open action menu |
| Right-click right pane | Open action menu for the currently selected item |
| Right-click left pane | No-op |
| Click action menu row | Run that action (same as `Enter`) |
| Click namespace badge in title bar | Open the namespace selector |
| Click row in namespace selector | Apply that namespace and close |
| Click outside a centered overlay | Dismiss it (same as `Esc`) — fullscreen / custom overlays are keyboard-only |
| Wheel up/down inside a centered overlay | Scroll the list cursor (same as `j` / `k` / arrow keys) |
| Scroll wheel in explorer | Scroll up/down |
| Shift+Drag | Select text (host terminal) |
| Shift+Option+Drag (macOS) / Alt+Drag (Linux, Windows) | Block-select text inside the embedded PTY |

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
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec, `A` Attach, `B` Debug, `b` Debug Pod, `p` Port Forward, `c` Capture Traffic, `S` Startup Analysis, `I` Crash Investigator, `v` Describe, `E` Edit, `z` Right-sizing, `D` Delete, `X` Force Delete, `V` Events

### Deployment Actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec, `A` Attach, `S` Scale, `r` Restart, `R` Rollback, `p` Port Forward, `v` Describe, `E` Edit, `z` Right-sizing, `D` Delete, `b` Debug Pod, `V` Events

### StatefulSet Actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec, `A` Attach, `S` Scale, `r` Restart, `p` Port Forward, `v` Describe, `E` Edit, `z` Right-sizing, `D` Delete, `b` Debug Pod, `V` Events

### DaemonSet Actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec, `A` Attach, `r` Restart, `p` Port Forward, `v` Describe, `E` Edit, `z` Right-sizing, `D` Delete, `b` Debug Pod, `V` Events

### Service Actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec (into pod behind service), `A` Attach (to pod behind service), `p` Port Forward, `c` Capture Traffic, `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### Secret Actions
`e` Secret Editor, `v` Describe, `E` Edit, `D` Delete, `l` Labels / Annotations, `P` Permissions, `b` Debug Pod, `V` Events

### ConfigMap Actions
`e` ConfigMap Editor, `v` Describe, `E` Edit, `D` Delete, `l` Labels / Annotations, `P` Permissions, `b` Debug Pod, `V` Events

### Node Actions
`c` Cordon, `u` Uncordon, `n` Drain, `t` Taint, `T` Untaint, `s` Shell, `v` Describe, `E` Edit, `b` Debug Pod, `V` Events

### Job Actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec, `A` Attach, `v` Describe, `E` Edit, `z` Right-sizing, `D` Delete, `X` Force Delete, `b` Debug Pod, `V` Events

### CronJob Actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec, `A` Attach, `t` Trigger (create Job), `v` Describe, `E` Edit, `z` Right-sizing, `D` Delete, `b` Debug Pod, `V` Events

### ArgoCD Application Actions
`s` Sync, `a` Sync (Apply Only), `f` Diff, `R` Refresh, `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### Helm Release Actions
`u` Values, `A` All Values, `E` Edit Values, `d` Diff, `U` Upgrade, `R` Rollback, `h` History, `v` Describe, `D` Delete, `b` Debug Pod, `V` Events

### Ingress Actions
`o` Open in Browser, `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### PVC Actions
`g` Go to Pod, `b` Debug Mount, `B` Debug Pod, `v` Describe, `E` Edit, `D` Delete, `V` Events

### Default Actions (all other resources)
`v` Describe, `E` Edit, `D` Delete, `l` Labels / Annotations, `P` Permissions, `b` Debug Pod, `V` Events

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
  terminal_toggle: "ctrl+t"  # Cycle terminal mode (pty/exec/mux)

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

  # Read-only mode
  readonly_toggle: "ctrl+r"  # At cluster picker: toggle highlighted row's [RO] marker. Inside a context: toggle the current tab.

  # Cluster color picker (cluster picker only)
  cluster_color_picker: "L"

  # Local Clusters Manager overlay (cluster picker only)
  local_cluster_manager: "ctrl+n"
```

### Crash Investigator overlay

Opened from the Pod action menu (`x` → `I`). Combines events, restart history,
last logs, and describe for the failing container in one tabbed panel.

| Key            | Action                                                   |
| -------------- | -------------------------------------------------------- |
| `Tab` / `S-Tab`| Cycle tabs forward / backward                            |
| `1` / `2` / `3` / `4` | Jump to Summary / Events / Logs / Describe        |
| `c`            | Cycle active container (init + app)                       |
| `p`            | Toggle previous / current logs (Logs tab only)            |
| `j` / `k`      | Scroll within tab body                                    |
| `g` / `G`      | Jump to top / bottom of tab body                          |
| `Ctrl+D` / `Ctrl+U` | Half-page down / up                                  |
| `Ctrl+F` / `Ctrl+B` | Full-page down / up (also `PgDn` / `PgUp`)           |
| `Shift+R`      | Refresh — re-fetch all sections, preserves cursor state  |
| `Esc` / `q`    | Close overlay                                             |

### Sync Wave Timeline (Applications)

Open from an `Application` row: press `x` for the action menu, then `W`.
Two-pane layout: phases on the left sidebar, the selected phase's content
on the right. `Tab` toggles which pane has focus.

**When sidebar has focus:**

| Key | Action |
| --- | --- |
| `j` / `↓` | Move sidebar cursor down (wraps); resets body cursor + scroll |
| `k` / `↑` | Move sidebar cursor up (wraps) |
| `Enter` / `Space` | Toggle phase collapse (sidebar marker `▾`/`▸`; body shows placeholder when collapsed) |
| `Tab` / `Shift+Tab` | Switch focus to body |
| `g` / `G` | First / last phase |

**When body has focus:**

| Key | Action |
| --- | --- |
| `j` / `↓` | Move body cursor down (wave headers + visible resources) |
| `k` / `↑` | Move body cursor up |
| `Enter` / `Space` | If on wave header: toggle wave collapse. If on placeholder: toggle phase collapse. If on resource: no-op |
| `Tab` / `Shift+Tab` | Switch focus to sidebar |
| `g` / `G` | First / last visible body row |
| `Ctrl+D` / `Ctrl+U` | Half-page scroll |
| `Ctrl+F` / `Ctrl+B` (or `PgDn` / `PgUp`) | Full-page scroll |

**Shared:**

| Key | Action |
| --- | --- |
| `R` | Refresh |
| `q` / `Esc` | Close overlay |

While `Application.status.operationState.phase == "Running"`, the overlay
auto-refreshes every 3 seconds. A spinner animates in the header during
the wave-annotation fetch phase.

Below 50 cols of terminal width, the overlay falls back to single-pane
mode (sidebar hidden, body uses full width). Tab becomes a no-op in this
mode.

### Traffic Capture overlay

Open: Pod or Service action menu (`x`) → Capture Traffic (`c`).

#### Configuration phase

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | Cycle focus between fields (Backend → Interface → Filter → Preset) |
| `j` / `k` / `↓` / `↑` | Next / previous field (when focus is not on Filter input) |
| `h` / `l` / `←` / `→` | Cycle the value of the focused field (backend, interface, preset) |
| (text) | Type BPF filter (when filter input has focus) |
| `Backspace` | Edit filter |
| `Enter` | Start capture (or launch kubeshark hand-off) |
| `Esc` | Close overlay |

For Service targets, an endpoint picker appears first:

| Key | Action |
|---|---|
| `j` / `k` | Navigate endpoints |
| `Enter` | Pick endpoint and proceed to config |
| `Esc` / `q` | Close overlay |

#### Live phase

| Key | Action |
|---|---|
| `s` | Stop capture; transitions to stopped phase, overlay stays |
| `Esc` / `q` | Stop capture and stay in the overlay; second Esc dismisses |
| `t` | Toggle live table vs status-only view |
| `Y` | Copy pcap path to system clipboard; marks capture saved |
| `/` | Search within live table |
| `j` / `k` | Scroll older / newer (tail-anchored: `0` = latest at bottom) |
| `Ctrl+D` / `Ctrl+U` | Half-page scroll older / newer |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Full-page scroll older / newer |
| `g` / `G` | Jump to oldest / return to live (latest) |

#### Stopped phase

| Key | Action |
|---|---|
| `Enter` | Restart capture with the same params |
| `e` | Edit filter — re-opens config phase, previous filter is preserved |
| `Y` | Copy pcap path to clipboard (mark saved so dismiss won't delete) |
| `j` / `k` / `Ctrl+D/U` / `Ctrl+F/B` / `g` / `G` | Scroll (same semantics as live) |
| `Esc` / `q` | Dismiss; deletes the pcap unless `Y` was pressed |
