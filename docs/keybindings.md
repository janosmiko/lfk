# Keybindings reference

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
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `z` | Toggle expand / collapse all resource groups |
| `x` | At resource types level: pin, hide, or show the selected type (action menu) |
| `0` / `1` / `2` | Jump to clusters / types / resources level, per cluster and both ways |
| `J` / `K` | Scroll preview pane down / up |
| `o` / `O` | Jump to owner/controller / open Object Explorer |
| `Backspace` | Jump back through teleport history |

In the Events list, `z` toggles event grouping instead of expand/collapse.

`x`, at the resource types level, also lets you pin or unpin the selected type's dashboard summary through the action menu. Pin, hide, and summary choices are saved per cluster context, or per union set.

Teleport history records owner, port-forward, orphan, finding, and mark jumps. Hierarchical `h`/`l` navigation does not push a history entry.

## Goto navigation

Vim-style `g`-prefix chords that switch the active resource type while keeping the current context and namespace filter. Press `g` to open the goto which-key popup (configurable via `which_key_enabled` and `which_key_delay_ms`); `esc` or any unmapped key closes it.

| Key | Resource |
|---|---|
| `gp` | Pods |
| `gd` | Deployments |
| `gs` | Services |
| `gn` | Nodes |
| `gN` | Namespaces |
| `gi` | Ingresses |
| `gj` | Jobs |
| `gc` | CronJobs |
| `gr` | ReplicaSets |
| `gD` | DaemonSets |
| `gt` | StatefulSets |
| `gC` | ConfigMaps |
| `gS` | Secrets |
| `gh` | HorizontalPodAutoscalers |
| `gv` | PersistentVolumeClaims |
| `gV` | PersistentVolumes |
| `gb` | PodDisruptionBudgets |

Add custom chords (including CRDs) via `goto_targets` in `~/.config/lfk/config.yaml`. For example, to jump to ArgoCD Applications with `ga`:

```yaml
goto_targets:
  ga: { kind: Application, group: argoproj.io, name: ArgoCD Applications }
```

The `g` popup also lists `g\`, which jumps to the previous namespace (swaps the scope back and forth).

All built-in chords are rebindable under `keybindings`. Every chord must start
with `jump_top` (default `g`) and add one more key -- the dispatcher only ever
looks up `jump_top` + the next keypress. A chord that does not is reported at
startup and ignored. Rebinding `jump_top` therefore disables the built-in
chords, which all start with `g`; re-point the ones you use at the new prefix.

## Views and tools

| Key | Action |
|---|---|
| `F1` | Toggle help screen |
| `?` | Which-key action panel — see [Which-Key Panel](#which-key-panel) |
| `P` | Toggle between details summary and YAML preview |
| `L` | Toggle live-log preview pane for selected pod or container (streaming tail in right pane; deeper levels only) |
| `F` | Cycle layout: hide sidebar -> fullscreen -> restore (dashboards toggle fullscreen) |
| `M` | Toggle resource relationship map view |
| `,` | Column visibility toggle (show / hide and reorder columns — see [Column Toggle Overlay](#column-toggle-overlay) below) |
| `p` | Pin / unpin resource type (at resource types level) |
| `H` | Toggle rarely used and hidden resource types |
| `I` | API Explorer (browse resource structure interactively) |
| `O` | Object Explorer (browse the selected resource's live object as a drill-in tree) |
| `U` | RBAC permissions browser (can-i) |
| `Shift+Z` | Open the cluster-wide Orphan overview |
| `C` | Session manager (save/switch/delete named workspace sessions) |
| `Ctrl+G` | Finalizer search and remove |
| `!` | Error log |
| `@` | Cycle Cluster / Monitoring dashboard |
| `Ctrl+N` | Open the Local Clusters Manager overlay (only at LevelClusters) |
| `Q` | Namespace resource quota dashboard |
| `` ` `` | Scheduler / task queue overlay (Tab toggles running / completed history; `a` toggles show-all entries in completed view) |
| `:` | Command bar (resource jumps, built-ins, kubectl, shell) |

`P`'s details summary shows labels, finalizers, annotation count, and other resource metadata, including a highlighted deletion timestamp for resources being deleted.

`H`'s rarely used types are CSI internals, webhooks, APF, leases, and advanced core resources. Revealed types show dimmed. The toggle resets on each launch, but per-type hides persist.

`:`'s resource jumps use the kind name, for example `:pod` or `:dep`. Built-ins include `:ns`, `:ctx`, `:set`, `:sort`, `:export`, and `:scheduler`. `:k` runs kubectl, for example `:k get pod`. `:!` runs a shell command, for example `:! cmd`.

## Sorting

| Key | Action |
|---|---|
| `>` / `<` | Sort by next / previous column |
| `=` | Toggle sort direction (ascending/descending) |
| `-` | Reset sort to default (Name ascending) |

Your chosen sort is remembered per resource kind and per cluster context, and persists across restarts (stored in `~/.local/state/lfk/sort_memory.yaml`), so leaving a list and returning — or quitting and reopening lfk — keeps your sort instead of resetting. Use `-` (reset) to drop a remembered sort.

## Modes and settings

| Key | Action |
|---|---|
| `w` | Toggle watch mode (auto-refresh every 2s) |
| `Ctrl+R` | Toggle read-only mode (cluster picker: highlighted row's [RO] marker; inside a context: current tab) |
| `T` | Switch color scheme (live preview, not persisted) |
| `Ctrl+T` | Cycle terminal mode (pty/exec/mux — mux skipped without tmux/zellij) |
| `~` | Cycle CPU/MEM columns between values and sparklines over 5m, 15m, 1h by default (needs Prometheus) |
| `Ctrl+S` | Toggle secret value visibility in details pane (YAML preview always shows actual base64 values) |
| `B` | Show / hide the per-resource SEC severity badge on explorer rows |
| `i` | Show / hide ignored security findings (security view only — shadows the Label Editor there, which is a no-op on synthetic finding rows) |

## Orphan filter presets

### Cluster-wide overview

Press **`Shift+Z`** anywhere in the explorer (or type `:orphans` with no arguments in the command bar) to open the cluster-wide orphan overview overlay. Inside the overlay:

| Key | Action |
| --- | ------ |
| `Tab` / `Shift+Tab` | Cycle kind filter chips |
| `s` | Toggle strict / lenient filtering |
| `/` | Filter by namespace + name |
| `Enter` | Jump to the highlighted resource (namespace switches automatically) |
| `R` | Re-scan the cluster |
| `Esc` / `q` / `Shift+Z` | Close the overlay |

The kind filter chips are All, Pods, Secrets, CMs, Svcs, PVCs, HPAs, PDBs, NetPols, Roles, and RBs.

Strict mode (the default) hides items referenced by workload templates. Lenient mode surfaces them.

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

## Search and filter

| Key | Action | Config key |
|---|---|---|
| `f` | Start filter mode (filter items in current view) | `filter` |
| `/` | Start search mode (search and jump to match) | `search` |
| `.` | Quick filter presets | `filter_presets` |
| `Tab` | Inside `/` or `f`: toggle broad mode — also matches against column values (annotations, labels, finalizers, CRD printer columns, custom user columns). Prompt shows `filter (all):` / `search (all):` while on. Resets on Enter/Esc. | |
| `Up` / `Down` | Inside `/` or `f`: cycle through previous queries (shared persistent history). | |
| `n` | Jump to next search match | `next_match` |
| `N` | Jump to previous search match | `prev_match` |
| `Esc` | Clear filter / cancel search | |
| `\` | Open namespace selector (then `.` to filter to current item's namespace) | `namespace_selector` |
| `A` | Toggle all-namespaces mode (also works inside the namespace selector — clears individual selections and enables all-ns) | `all_namespaces` |
| `g\` | Jump to previous namespace (swaps the scope back and forth) | `previous_namespace` |

Each list remembers its `f` filter per tab: drilling into a resource (logs, containers, owned objects) and navigating back keeps the filter applied. A different list starts unfiltered; press `Esc` to clear a list's filter.

Search supports abbreviated resource type names (e.g., `pvc`, `hpa`, `deploy`).

`/` and `f` share one persistent history at `$XDG_STATE_HOME/lfk/query-history` (default `~/.local/state/lfk/query-history`) — both inputs accept the same query syntax and match against the same fields, so a query confirmed in one mode is recallable from the other. The `:` command bar keeps its own `history` file because its inputs are kubectl-shaped commands rather than resource-name queries.

## Actions

| Key | Action | Config key |
|---|---|---|
| `x` | Open action menu (bulk actions when items selected) | `action_menu` |
| `Ctrl+L` | Open fullscreen log viewer for selected resource | `logs` |
| `e` | Secret/ConfigMap editor (inline key-value editing) | `secret_editor` |
| `E` | Edit selected resource in $KUBE_EDITOR or $EDITOR | `edit` |
| `R` | Refresh current view (also works inside the namespace selector — re-fetches the namespace list from the cluster) | `refresh` |
| `v` | Describe selected resource | `describe` |
| `D` | Delete resource (force delete Pod/Job if already deleting, force finalize others) | `delete` |
| `X` | Force delete (Pod/Job only) | `force_delete` |
| `S` | Scale resource (Deployment / StatefulSet / ReplicaSet / HPA) | `scale` |
| `W` | Save resource to file / toggle warnings-only filter (Events view) | `save_resource` |
| `Ctrl+O` | Open in browser: ingress host, port-forward localhost URL, or (on a Service) start a port forward and open it | `open_browser` |
| `i` | Edit labels/annotations | `label_editor` |
| `a` | Create new resource from template (/ to search) | `create_template` |
| `d` | Diff two selected resources | `diff` |

Events list also groups duplicate events (same Type/Reason/Message/Object) by default; press `z` to toggle grouping.

### Delete confirm dialog

| Key | Action |
|---|---|
| `Enter` / `y` | Confirm |
| `Tab` | Cycle cascade policy: Background / Foreground / Orphan / None |
| `Esc` / `n` | Cancel |

Cascade controls what happens to dependent objects (a Job's pods, a Deployment's ReplicaSets):

| Policy | Effect |
|---|---|
| `Background` | Delete the object now; the garbage collector removes dependents (kubectl's default) |
| `Foreground` | Keep the object until every dependent is gone |
| `Orphan` | Leave dependents running |
| `None` | Send no policy; the API server applies its own per-resource default |

`Orphan` and `None` are highlighted in the dialog because both can leave workloads running. The server default behind `None` is Background for most kinds but Orphan for Jobs and ReplicationControllers, so deleting a Job with `None` leaves its pods running.

Set the starting policy with `delete_propagation_policy` in the config. Bulk delete uses the policy shown in the dialog.

The dialog also states what the action costs, as three labelled rows. Each row is built from the selected cascade policy, so no two rows can contradict each other.

| Row | Answers |
|---|---|
| `Scope` | What else stops existing |
| `Availability` | What stops serving |
| `Risk` | What refuses the action, or what it leaves with no owner |

Deleting a Deployment with four ReplicaSets under it:

```text
Cascade:      Background
Scope:        4 replicasets, 3 pods
Availability: 0 of 3 ready after
Risk:         kyverno allows 2 at once, this removes 3
```

Press `Tab` for `Orphan` and every row is rewritten in place:

```text
Cascade:      Orphan
Scope:        the deployment only
Availability: unchanged, the 3 pods keep running
Risk:         4 replicasets, 3 pods left with no owner
```

A row with nothing to say is left out, so deleting a bare pod shows `Availability` alone. `Risk` names risks only: no budget covering the pods means no row, not a row saying that none applies. `Risk` is never more than one line, because a policy that keeps dependents evicts nothing, so a budget breach and an orphan hazard cannot both apply.

Only a drain is refused by a budget, because only the eviction API honours one; a direct delete or a scale-down exceeds the budget without being stopped, and the wording says so. Deleting a node uses the pods on that node, like a drain. The scale overlay updates the rows as you type.

`Scope` walks `ownerReferences` down from the target, so a deep chain is counted in full: deleting a Deployment counts its ReplicaSets and their pods. Countable owners are Deployment, StatefulSet, DaemonSet, ReplicaSet, ReplicationController, Job, CronJob, and Service. For any other kind the row is left out rather than showing a zero it cannot stand behind. Where Kubernetes guarantees that the children carry the owner's labels, the walk narrows the list to that selector, so counting one small workload in a namespace of ten thousand pods stays cheap. That covers pods, ReplicaSets and ControllerRevisions under a workload, and EndpointSlices under a Service. A StatefulSet's PersistentVolumeClaims, a CronJob's Jobs, and every bulk selection are listed unnarrowed, because no selector there is promised to match.

Both fetches share one placeholder, `working out what this costs...`, so no row renders against a half-loaded answer. Bulk delete shows one figure for the whole selection, at one pod list and one list per child kind per namespace. A row nothing can be resolved for, a ConfigMap say, is reported as `N rows not counted`.

### Force delete confirm dialog

| Key | Action |
|---|---|
| Type `DELETE` + `Enter` | Confirm |
| `Tab` | Cycle cascade policy: Background / Foreground / Orphan |
| `Esc` | Cancel |

`None` is not offered here: force delete runs through `kubectl delete`, which always sends a policy, so a configured `none` default is clamped to `background`. `Tab` is inert on the other typed confirmations (Force Finalize, Finalizer Remove, Disrupt) and on Longhorn nodes, since none of them cascade through `kubectl delete`.

Force delete with `Orphan` is the widest-reaching combination in lfk: bulk force delete strips finalizers before deleting, so the owner disappears immediately while its pods keep running with no owner reference and no finalizer trail to trace. Prefer `Background` unless you specifically intend to keep the dependents.

Port forwarding is available via the action menu (`x`) on Pod, Service, Deployment, StatefulSet, and DaemonSet resources. In the port-forward dialog, select an exposed port with `j`/`k`, or type a `local:remote` mapping (e.g. `8080:80`) to choose a specific local port — omit the local port (e.g. `:80`) for a random one. After creating a port forward, the view automatically navigates to the Port Forwards list and displays the resolved local port in the status bar. Active port forwards can be managed via the "Port Forwards" virtual resource in the Networking group; press `D` there to remove the selected forward. On a Service, `Ctrl+O` (or the "Port Forward & Open" action) starts a port forward and opens the resolved `http://localhost:<port>` in the browser once it is ready.

Resource-specific actions (exec, scale, restart, secret editor, etc.) are available through the action menu (`x`).

## Clipboard

| Key | Action |
|---|---|
| `y` | Copy resource name to clipboard (with multi-selection: newline-joined names of all selected items) |
| `Y` | Open copy-as picker (YAML / JSON / Table) |
| `Ctrl+Y` | Copy a single field |
| `Ctrl+P` | Apply resource from clipboard (`kubectl apply`) |

`Y`'s YAML and JSON formats support multi-selection: multi-doc YAML joined with `---`, or a JSON array. Table is a kubectl-style aligned plain-text view of the displayed columns. At LevelClusters and LevelResourceTypes only Table is offered. At LevelContainers, YAML and JSON extract the container spec block from the Pod manifest.

`Ctrl+Y` opens instantly on the visible table columns (Name, Status, extras). `Enter` copies the cell value. `Tab` switches to the full manifest field list, where array elements are labeled semantically (`status.addresses[ExternalIP].address` for a node's external IP), so filtering `ExternalIP` finds the address row. With multi-selection the chosen column/field is extracted from every selected item, one value per line. Labeled array elements resolve per manifest, not by index, and items missing the field are skipped. `Ctrl+Y` remembers the last-copied entry per resource kind for the session and preselects it next time.

When items are multi-selected (`Space` / `Ctrl+Space` / `Ctrl+A`), `y`, `Y`, and `Ctrl+Y` operate on the selection rather than the cursor row — mirroring the precedence used by `D` (delete) and other bulk actions. `Y` and `Ctrl+Y` are capped at 50 manifests per copy (client-go's default rate limiter serializes the per-item fetches).

## Multi-Selection

| Key | Action |
|---|---|
| `Space` | Toggle selection on current item (sets anchor) |
| `Ctrl+Space` | Select range from anchor to cursor (legacy `ctrl+@` spelling still accepted) |
| `Ctrl+A` | Select / deselect all visible items |
| `Esc` | Clear selection |

When items are selected, press `x` to open the bulk action menu (delete, force delete, scale, restart, diff).

See [Which-Key Panel](#which-key-panel) for the `?` action panel.

## Which-Key Panel

`?` opens a panel above the status bar listing the hotkeys actionable right now as one flat list, no section headers -- like neovim's which-key. Entries flow down each column before moving right, clustered by category (below) so each color forms one contiguous run, and sorted within a category by modifier: plain keys first, then `Ctrl` chords, then `Alt`, then `Ctrl+Alt`. Within each of those, letters and digits come first, then punctuation, then named keys (`F1`, `Space`, `Tab`). The panel is as tall as its content, capped at 25 rows and at the terminal height; longer content scrolls.

The panel is context-aware per view. In the explorer it lists what the current row supports; in a fullscreen viewer it lists what that viewer supports in its current state -- visual mode swaps the yank and hides the normal-mode keys, an armed count prefix relabels `y`, the log viewer's follow, severity, and `--previous` toggles read their current direction, and the diff viewer's `Tab` disappears in unified mode.

| View | Panel |
|---|---|
| Explorer | Yes |
| YAML view | Yes |
| Log viewer | Yes |
| Describe view | Yes |
| Diff view | Yes |
| API Explorer | Yes |
| Object Explorer | Yes |
| Log Top | Yes |
| Event viewer (fullscreen) | Yes |
| Exec mode | No -- every key goes to the PTY, including `?` |
| Overlays (can-i, error log, sync waves, network policy) | No -- overlay keys are handled before the leader |

| Key | Action |
|---|---|
| `?` | Open the panel; press again to switch between category order and key order |
| `Ctrl+D` / `Ctrl+U` | Scroll half a page (only when the content overflows) |
| `Esc` | Close |
| any other key | Close, and still run normally |

`?` no longer closes the panel -- it toggles the entry order. The chosen order is saved the moment you press the key and survives a restart; it is stored in `~/.local/state/lfk/whichkey_prefs.yaml`, never written back to your config file.

Precedence: the saved choice wins over `which_key_grouped` (default `true`), which is only the startup default for someone who has never toggled. The state file also records the `which_key_grouped` value in force at the time, so changing that setting afterwards retires the saved choice and the new default applies again. Delete the state file to reset.

Descriptions are colored by category, since there are no headers to say it. Keys keep one accent throughout -- the same green the hint bar draws hotkeys in, and no category uses it, so the key never matches the description beside it. In key order the color is the only category cue left, so both modes carry it. A legend row at the bottom of the panel names each color in that color, so the mapping doesn't have to be memorized -- only the categories actually offered on the current row appear in it, and it is omitted with `no_color`.

| Category | Color | Examples |
|---|---|---|
| Actions | magenta | Delete, Edit, Logs, Copy |
| Views | blue | Resource map, RBAC browser, Task queue |
| Filter | cyan | Filter, Search, Namespace selector |
| Selection | purple | Toggle selection, Select range, Diff |
| Sort | amber | Sort next/previous, Flip, Reset |
| Settings | orange | Watch mode, Read-only, Color scheme |

Keys render as glyphs when `icons` allows them; `simple`, `none`, and `no_color` keep the textual form. The help screen's (`F1`) key column uses this same substitution.

| Binding | `nerdfont` | `unicode` / `emoji` | `simple` / `none` |
|---|---|---|---|
| `ctrl+d` | `󰘴 D` | `⌃D` | `ctrl+d` |
| `ctrl+shift+x` | `󰘴 󰘶 X` | `⌃⇧X` | `ctrl+shift+x` |
| `space` | `󱁐` | `␣` | `space` |
| `tab` | `󰌒` | `⇥` | `tab` |
| `backspace` | `󰁮` | `⌫` | `backspace` |
| `left` / `right` / `up` / `down` | `󰁍` / `󰁔` / `󰁝` / `󰁅` | `←` / `→` / `↑` / `↓` | `left` / `right` / `up` / `down` |
| `enter` / `esc` | `󰌑` / `󱊷` | `enter` / `esc` | `enter` / `esc` |

`nerdfont` uses which-key.nvim's keycap glyphs (plain arrow keycaps for `left`/`right`/`up`/`down` — Material Design Icons has no dedicated keyboard-arrow set) and pads each modifier with a space, since the proportional Nerd Font variants draw a keycap wider than one cell. `unicode` keeps `enter`/`esc` as words: `⏎`/`⎋` are easily confused at one cell; `tab`/`backspace`/the arrows each have one unambiguous glyph, so those switch. The goto popup (`g`) has no categories and keeps a single description color.

`?` is the leader in every view that has a panel, so `F1` is the help key there. Inside a search or filter prompt `?` stays a literal character. Rebind with `which_key_leader`; set `which_key_enabled: false` to turn the panel off (`?` then opens help again). `which_key_leader_delay_ms` (default `0`) delays the reveal, `which_key_grouped` (default `true`) sets the startup entry order until the leader key toggles it (saved in `~/.local/state/lfk/whichkey_prefs.yaml`).

## Bookmarks

Vim-style named marks for quick navigation. A bookmark stores a resource
path (context + namespace + resource type + optional resource name) under
a single-character slot. Any list filter active when the mark is set is
saved with it and reapplied on jump; filtered slots show a `> /<filter>`
suffix in their name.

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
| `Tab` | Bookmark overlay | Toggle `[KEEP CURRENT NS]` |
| `Ctrl+X` | Bookmark overlay | Delete selected bookmark (with confirmation) |
| `Alt+X` | Bookmark overlay | Delete all bookmarks (with confirmation) |

> Namespace on jump: each bookmark shows its saved namespace scope, and by
> default the jump applies it. Press `Tab` to opt out (`[KEEP CURRENT NS]`)
> and keep the tab's current scope instead; the flag resets on close.

## Help View

The in-app screen is a quick reference: one binding per line, keys right-aligned, modifier chords drawn as symbols (`⌃D`, `⇧Tab`) when the terminal supports them. Search and filter match the textual chord, so typing `ctrl` finds a row drawn as `⌃D`. This document is the exhaustive reference.

| Key | Action |
|---|---|
| `/` | Search — highlights matches inline without removing non-matching lines |
| `Ctrl+N` / `Ctrl+P` | Next / previous match while typing the search input |
| `Enter` | Apply search (closes input, keeps highlights, arms `n`/`N`) |
| `n` / `N` | Jump to next / previous search match (after Enter) |
| `f` | Filter — narrows the visible list to lines matching the query |
| `Esc` | Cascades: clear active search → clear active filter → close help |
| `j` / `k` | Scroll down / up |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Half-page scroll down / up |
| `Ctrl+F` / `Ctrl+B` / `PgDown` / `PgUp` | Full-page scroll |
| `g` / `G` | Jump to top / bottom |
| `q` / `?` / `F1` | Close help |

## YAML View

> Search (`/`), help (`F1`), match navigation (`n`/`N`), and the display toggles (wrap, fold, line numbers, timestamps, prefixes, unified, preview, follow) shown across the viewers below are rebindable — see [Keybindings](config-reference.md#keybindings). Core cursor navigation (`hjkl`, `g`/`G`, page motions, word-motions) is fixed. `123 Ctrl+D` / `123 Ctrl+U` set the scroll step shared by both keys, following vim's `'scroll'` option. The result is clamped to the viewport.

| Key | Action |
|---|---|
| `j` / `k` | Scroll up / down |
| `123j` / `123k` | Move cursor down / up N visible lines (count-prefixed motion; folds skipped) |
| `h` / `l` | Move cursor column left / right |
| `123h` / `123l` | Move cursor column left / right by N runes |
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
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `123 Ctrl+D` / `123 Ctrl+U` | Scroll N lines (sets shared Ctrl+D/Ctrl+U step) |
| `123 Ctrl+F` / `123 Ctrl+B` | Page motion scaled by N |
| `/` | Search in YAML |
| `n` / `N` | Next / previous search match |
| `123n` / `123N` | Jump to Nth next / previous search match |
| `v` | Character visual selection (from cursor column) |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection (from cursor column) |
| `h` / `l` | Move selection column left / right (in visual mode) |
| `viw` / `vaw` / `viW` / `vaW` | Select inner/around word (or WORD) under cursor |
| `y` | Copy line under cursor (or selection in visual mode) |
| `123y` | Copy number of lines from cursor (count-prefixed yank; folds skipped) |
| `z` | Toggle fold on section under cursor |
| `Z` | Toggle all folds (collapse / expand all) |
| `>` | Toggle line wrapping (configurable via `toggle_wrap`) |
| `m` | Toggle inline field-manager blame on the cursor line |
| `R` | Re-fetch the resource and refresh the view, keeping cursor/scroll (configurable via `refresh`) |
| `Ctrl+E` | Edit resource in `$KUBE_EDITOR` or `$EDITOR` |
| `O` | Switch to the Object Explorer at the attribute under the cursor (keeps position) |
| `I` | Open the API Explorer at the schema of the attribute under the cursor |
| `Ctrl+K` | Toggle the schema side pane for the attribute under the cursor (configurable via `field_doc`) |
| `?` | Which-key panel for this view — see [Which-Key Panel](#which-key-panel) |
| `F1` | Full help |
| `q` / `Esc` | Back to explorer |

The top breadcrumb shows the resource name and the attribute path under the cursor (e.g. `lfk > ctx > Pods > my-pod > spec.containers[0].image`), like the Object Explorer location.

## Object Explorer

> Drill-in tree over the selected resource's live object, opened with `O`. Under watch mode the browsed object live-refreshes as the resource changes; pause it to read a stable snapshot.

| Key | Action |
|---|---|
| `j` / `k` | Navigate fields |
| `l` / `Enter` / `→` | Drill into object/array field |
| `h` / `Backspace` / `←` | Go back one level |
| `J` / `K` | Scroll the YAML preview pane |
| `/` | Filter the current level by key (in tree view: keys anywhere in the subtree) |
| `r` | Recursive find overlay (search keys across the whole object) |
| `T` | Toggle tree view — expand the whole subtree with ASCII-art guides (configurable via `tree_view`) |
| `Space` / `z` | Fold / unfold the subtree under the cursor (tree view; `z` configurable via `toggle_fold`) |
| `R` | Manually refresh the browsed object now (configurable via `refresh`) |
| `w` | Toggle live refresh on / off — title shows `[PAUSED]` when off (configurable via `watch_mode`) |
| `y` / `Y` | Yank the selected node's path / full YAML |
| `P` | Open the whole resource in the full YAML viewer |
| `I` | Open the API Explorer at the selected item's schema |
| `Ctrl+K` | Toggle the schema side pane for the selected item (configurable via `field_doc`) |
| `?` | Which-key panel for this view — see [Which-Key Panel](#which-key-panel) |
| `F1` | Full help |
| `q` | Close the Object Explorer |
| `Esc` | Clear filter / back one level / close at root |

Live refresh defaults to on; set `object_explorer.live: false` to start paused. Set `object_explorer.tree: true` to open in the tree view (`api_explorer.tree` for the API Explorer). See [Viewer Defaults](config-reference.md#viewer-defaults).

## Describe View

| Key | Action |
|---|---|
| `j` / `k` | Move cursor up / down |
| `123j` / `123k` | Move cursor down / up N lines (count-prefixed motion) |
| `h` / `l` | Move cursor column left / right |
| `123h` / `123l` | Move cursor column left / right by N runes |
| `0` / `$` / `^` | Move cursor to line start / end / first non-whitespace |
| `w` / `b` / `e` / `W` / `B` / `E` | Word / WORD motions |
| `123w` / `123b` / `123e` (and capitals) | Apply word/WORD motion N times |
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `123G` | Jump to line number |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `123 Ctrl+D` / `123 Ctrl+U` | Scroll N lines (sets shared Ctrl+D/Ctrl+U step) |
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
| `>` | Toggle line wrapping (configurable via `toggle_wrap`) |
| `?` | Which-key panel for this view — see [Which-Key Panel](#which-key-panel) |
| `F1` | Full help |
| `q` / `Esc` | Back to explorer |

## Log Viewer

| Key | Action |
|---|---|
| `j` / `k` | Move cursor up / down |
| `123j` / `123k` | Move cursor down / up N lines (count-prefixed motion) |
| `h` / `l` / `Left` / `Right` | Move cursor column left / right |
| `123h` / `123l` | Move cursor column left / right by N runes |
| `0` / `$` | Move cursor to line start/end |
| `^` | Move cursor to first non-whitespace character |
| `w` / `b` | Move cursor to next/previous word start |
| `W` / `B` | Move cursor to next/previous WORD start (whitespace-delimited) |
| `e` | Move cursor to end of word |
| `E` | Move cursor to end of WORD (whitespace-delimited) |
| `123w` / `123b` / `123e` (and capitals) | Apply word/WORD motion N times |
| `gg` / `Home` | Jump to top |
| `G` / `End` | Jump to bottom |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Half page down / up |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Full page down / up |
| `123 Ctrl+D` / `123 Ctrl+U` | Scroll N lines (sets shared Ctrl+D/Ctrl+U step) |
| `123 Ctrl+F` / `123 Ctrl+B` | Page motion scaled by N |
| `F` | Toggle follow mode (auto-scroll to new logs) |
| `f` | Filter log lines live (`~`fuzzy, regex auto-detected, `\`literal); narrows the view to matching lines |
| `i` / `o` | Lower / raise the minimum log severity shown |
| `>` | Toggle line wrapping (configurable via `toggle_wrap`) |
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
| `h` / `l` | Move selection column left / right (in visual mode) |
| `viw` / `vaw` / `viW` / `vaW` | Select inner/around word (or WORD) under cursor |
| `y` | Copy line under cursor (or selection in visual mode) |
| `123y` | Copy number of lines from cursor (count-prefixed yank) |
| `\` | Switch pod / filter containers (space: select, enter: apply, / to filter) |
| `?` | Which-key panel for this view — see [Which-Key Panel](#which-key-panel) |
| `F1` | Full help |
| `q` / `Esc` | Close log viewer |

The display toggles (`>` wrap, `s` timestamps, `p` prefixes, `P` preview panel, `~` CPU/MEM columns, and their counterparts in the YAML, diff, describe, and explorer views) are remembered across restarts in `~/.local/state/lfk/viewer_prefs.yaml`, so a layout you set once survives the next open and the next launch. They outrank the `log_viewer` and viewer-default config groups. Delete that file to go back to the configured defaults.

`i` / `o` cycle the minimum severity through `off`, `INFO+`, `WARN+`, and `ERROR+`. All source levels merge into these three tiers. Structured logs use the parsed level. Plain-text logs are classified by keyword (error/warn/debug), defaulting to INFO. `INFO+` hides debug/trace lines, `WARN+` shows only warn and error lines, and `ERROR+` shows only error and failure lines.

The log viewer's `/` keeps its own persistent history at `$XDG_STATE_HOME/lfk/log-search-history` (default `~/.local/state/lfk/log-search-history`), separate from the explorer's `query-history`. Log search matches raw log lines (substring/regex over arbitrary text) rather than resource names, so pooling the two would surface irrelevant entries on Up/Down in either context.

Tail-first loading: Full Logs (`Ctrl+L` key or action menu `L`) load the last 100 lines initially (configurable via `log_viewer.tail_lines`). Tail Logs (action menu `l`) load only the last 10 lines (configurable via `log_viewer.tail_lines_short`). Scrolling to the top loads older history.

Auto-reconnect across init containers: when viewing logs for a single Pod in all-containers mode (no specific container selected via `\`), the stream automatically reconnects each time kubectl exits — e.g. as init containers transition. The reconnect is silent. After several consecutive empty reconnects the viewer stops retrying.

## Log Top

Log Top aggregates a resource's logs into a table grouped by parsed attributes (e.g. method + path for HTTP, or any JSON/logfmt keys). Columns auto-fit the terminal width - wider terminals show more (up to all of REQ, REQ/s, ERR%, %, ERR, 4XX, 5XX, AVG, P50/P95/P99, MAX); narrower ones show the highest-priority columns. `,` toggles/reorders columns explicitly. Launch from the resource action menu ("Log Top", quick-key `t`) or press `T` in the open log viewer. Offered for Pods, Services, Deployments, StatefulSets and DaemonSets; not for Jobs or CronJobs, whose batch output does not carry the access-log shape this view parses. Auto-detects Traefik JSON, ingress-nginx, Envoy, NCSA common/combined (nginx, Apache, Traefik default access logs), JSON, and logfmt. Config: `log_top_default_profile` (`auto` | `traefik-json` | `ingress-nginx` | `nginx-combined` | `envoy` | `json` | `logfmt`).

| Key | Action |
|---|---|
| `j` / `k` / `↓` / `↑` | Move cursor down / up |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Half-page down / up |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Full-page down / up |
| `g` / `home` / `G` / `end` | Jump to top / bottom |
| `5j` / `10G` | Count-prefixed motion (repeat / go to row N) |
| `>` / `<` | Cycle sort column (dimensions + REQ / REQ/s / ERR% / 4XX / 5XX / AVG / p50 / p95 / p99 / MAX) |
| `=` | Toggle sort direction (the active column shows ↑/↓) |
| `-` | Reset sort to REQ descending |
| `.` | Open group-by field picker (multi-select) |
| `p` | Open profile picker (traefik-json / ingress-nginx / nginx-combined / envoy / json / logfmt / auto) |
| `,` | Open column picker: show / hide and reorder dimension columns, show / hide metric columns |
| `f` | Filter rows (matches dimension values) |
| `/` | Search and jump to matching row |
| `n` / `N` | Next / previous search match |
| `Tab` | Cycle the dimension `Enter` drills into |
| `Enter` | Drill into selected group (descends to the next unused dimension, marked `▸` in its column header) |
| `?` | Which-key panel for this view — see [Which-Key Panel](#which-key-panel) |
| `Esc` / `q` | Pop drill level, or return to log viewer |

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
| `PgUp` / `PgDown` | Scroll back / forward by a full viewport (no prefix needed; passed through to full-screen programs) |
| Mouse wheel | Scroll the PTY scrollback (1 line per tick) |

All other keys are forwarded to the PTY process. The PTY session continues running when you switch tabs, so you can return to it later. Typing any character snaps the view back to the live shell so you don't accidentally type into history.

### Scrollback

Each PTY tab keeps a ring of up to 5000 ANSI-stripped lines captured from the byte stream. Use `Ctrl+]` then `Ctrl+U` / `Ctrl+D` / `Ctrl+B` / `Ctrl+F` to navigate it; `Ctrl+]` `g` / `G` jump to the oldest captured line / back to live. `PgUp` / `PgDown` scroll by a full viewport without any prefix, except while a full-screen program is on the alternate screen — there they are forwarded to the program so its own paging keeps working. The hint bar shows `scrolled N` while you're not at the live tail. Full-screen curses programs (vim, less, htop) write absolute-position output that the line-stream capture can't reconstruct cleanly — their scrollback view will look messy while they're running, but normal output cleans up afterward. If you need precise scrollback, switch to `exec` or `mux` mode (`Ctrl+T`) so the host terminal's own scrollback handles it.

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
| `j` / `k` | Move cursor up / down |
| `123j` / `123k` | Move cursor down / up N lines (count-prefixed motion) |
| `h` / `l` | Move cursor column left / right |
| `123h` / `123l` | Move cursor column left / right by N runes |
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
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `123 Ctrl+D` / `123 Ctrl+U` | Scroll N lines (sets shared Ctrl+D/Ctrl+U step) |
| `123 Ctrl+F` / `123 Ctrl+B` | Page motion scaled by N |
| `/` | Search in diff |
| `n` / `N` | Next / previous search match |
| `123n` / `123N` | Jump to Nth next / previous search match |
| `v` | Character visual selection |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection |
| `h` / `l` | Move selection column left / right (in visual mode) |
| `viw` / `vaw` / `viW` / `vaW` | Select inner/around word (or WORD) under cursor |
| `y` | Copy line under cursor (or selection in visual mode) |
| `123y` | Copy number of lines from cursor (count-prefixed yank; empty-side lines skipped) |
| `z` | Toggle fold unchanged section at cursor |
| `Z` | Toggle all folds |
| `#` | Toggle line numbers |
| `>` | Toggle line wrapping (configurable via `toggle_wrap`) |
| `u` | Toggle unified/side-by-side view |
| `?` | Which-key panel for this view — see [Which-Key Panel](#which-key-panel) |
| `F1` | Full help |
| `q` / `Esc` | Back to explorer |

## Event Timeline

Press `V` on a resource (or open the Events list and press `Enter` on an event) to open the Event Timeline overlay. Press `Shift+F` to toggle between the overlay and a fullscreen viewer that takes over the whole window.

| Key | Action |
|---|---|
| `j` / `k` | Move cursor down / up |
| `123j` / `123k` | Move cursor down / up N lines (count-prefixed motion) |
| `h` / `l` / `Left` / `Right` | Move cursor column left / right |
| `123h` / `123l` | Move cursor column left / right by N runes |
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
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Half page down / up |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Full page down / up |
| `123 Ctrl+D` / `123 Ctrl+U` | Scroll N lines (sets shared Ctrl+D/Ctrl+U step) |
| `123 Ctrl+F` / `123 Ctrl+B` | Page motion scaled by N |
| `Shift+F` | Toggle fullscreen event viewer (also minimizes back) |
| `>` | Toggle line wrapping (configurable via `toggle_wrap`) |
| `/` | Search in events |
| `n` / `N` | Next / previous search match |
| `123n` / `123N` | Jump to Nth next / previous search match |
| `v` | Character visual selection (from cursor column) |
| `V` | Visual line selection |
| `Ctrl+V` | Block (column) visual selection (from cursor column) |
| `viw` / `vaw` / `viW` / `vaW` | Select inner/around word (or WORD) under cursor |
| `y` | Copy line under cursor (or selection in visual mode) |
| `123y` | Copy N lines from cursor (count-prefixed yank) |
| `?` | Which-key panel (fullscreen viewer only) — see [Which-Key Panel](#which-key-panel) |
| `F1` | Open this help, scrolled to the Event Timeline section (`?` too, in the overlay) |
| `q` / `Esc` | Close overlay (or exit fullscreen back to overlay) |

> Events are pulled from the cluster, correlated to the selected resource (or shown cluster-wide on the timeline overlay), and grouped when their Type/Reason/Message/Object match. The line buffer (1-9 then a motion key) is consumed after each motion, so `5j 3k` jumps down 5 then up 3 without any digit leaking into the next command.

## Column Toggle Overlay

Press `,` in the resource list to open the column toggle overlay. It lists
every toggleable column for the current kind — built-ins (Name, Namespace,
Ready, Restarts, Status, Age) and extras from the resource's
`additionalPrinterColumns`.

| Key | Action |
|---|---|
| `j` / `k` | Navigate up / down |
| `Space` | Toggle the current entry |
| `J` / `K` | Reorder the current entry down / up |
| `/` | Filter entries by name |
| `c` | Clear selection (uncheck every entry) |
| `R` | Reset to defaults for the current kind |
| `Enter` | Apply the selection |
| `Esc` / `q` | Close without saving |

Built-in and extra columns can be freely interleaved — `J`/`K` moves
either kind, so you can put `Age` before `Namespace` or drop an extra
like `IP` between `Ready` and `Status`. `Name` is listed too: it renders
first by default but can be reordered (e.g. after a `Catalog`/`Task`
extra) or hidden like any other column. When visible, `Name` still
absorbs the row's leftover width and compresses (truncates) when the
other columns leave it little room.

The selection you apply is explicit: the table renders exactly the
columns you check, in the exact order they appear in the overlay, and
will not auto-fill the remaining space with unchecked columns. The
chosen visibility and order are remembered per resource kind and per
cluster context, and persist across restarts (stored in
`~/.local/state/lfk/column_prefs.yaml`) — committed on `Enter`, cleared
by `R`; `Esc` discards uncommitted edits. To start from a clean slate,
press `c` to uncheck every entry at once, then space-select only the
columns you want.

If you apply a completely empty selection (no built-ins, no extras), the
overlay interprets it as "reset to defaults for this kind" rather than
leaving the table empty. To render only built-ins with zero extras, keep
at least one built-in column checked when you press Enter.

## Inline Editors (Secret / ConfigMap / labels and annotations)

The Secret, ConfigMap, and Labels/Annotations editors use a shared key-value
overlay. The list view supports vim-like navigation; pressing `e` or `a`
enters edit mode for the selected (or new) entry.

### List view

| Key | Action |
|---|---|
| `j` / `k` | Move cursor up / down |
| `e` | Edit selected key/value |
| `a` | Add a new key/value entry |
| `y` | Copy value (opens the format picker if rows are selected) |
| `Space` | Toggle selection on the current row |
| `Y` | Open the format picker |
| `/` | Filter the list by key (typing extends the query, `Enter` applies, `Esc` clears) |
| `D` | Delete selected entry |
| `Enter` | Save changes and close (no-op if nothing changed) |
| `Esc` | Close without saving |

`y` copies the cursor row's value when nothing is selected. It opens the format picker automatically when one or more rows are selected, so you don't silently copy a single value while ignoring the marked bundle.

`Space` auto-advances the cursor after toggling, and works across non-adjacent rows.

The Labels/Annotations editor additionally has a `Tab` binding in the list
view to switch between the labels pane and the annotations pane. Switching
tabs clears the multi-row selection (label and annotation namespaces are
disjoint).

### Format picker (Shift+Y)

`Y` always opens the picker and copies the selected rows, or the cursor row, as YAML, JSON, dotenv, `key=value`, or values-only.

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
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Scroll cursor down / up by half a page (pane mode only) |
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
| `T` | Toggle tree view (configurable via `tree_view`) |
| `Space` / `z` | Fold / unfold the subtree under the cursor |
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `?` | Which-key panel for this view — see [Which-Key Panel](#which-key-panel) |
| `F1` | Full help |
| `q` | Close API explorer |
| `Esc` | Go back one level / close at root |

`T`'s tree view shows the whole field subtree with ASCII-art guides, and stays on while drilling in or going back, until you toggle it off again.

`Space` / `z` folding applies only in tree view (`z` is configurable via `toggle_fold`).

## Can-I Browser

| Key | Action |
|---|---|
| `j` / `k` | Navigate groups |
| `J` / `K` | Scroll resource list down / up |
| `/` | Search / filter groups by name |
| `a` | Toggle all/allowed-only permissions |
| `s` | Switch subject (User/Group/SA) |
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Page down / up (half page) |
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
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Half page down / up in the resource list |
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

## Can-I subject selector

| Key | Action |
|---|---|
| `j` / `k` | Navigate subjects |
| `/` | Filter subjects by name |
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `Enter` | Select subject |
| `Esc` | Clear filter / close |

## Network Policy Visualizer

Opens via the action menu (`x` → "Visualize", default key `N`) on a
NetworkPolicy, CiliumNetworkPolicy, or CiliumClusterwideNetworkPolicy, or via
`x` → "Network Policies" (`N`) on a Pod or Service to see every policy whose
pod selector matches the pod (or the service's backing pods) — Cilium
policies included when the CRDs are installed. When no policy selects the
resource, the view says so explicitly — no policy restrictions apply.

| Key | Action |
|---|---|
| `j` / `k` / Mouse wheel | Scroll up / down |
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `/` | Search (matches highlighted; `Enter` jumps to the first hit) |
| `n` / `N` | Next / previous match |
| `q` / `Esc` | Close visualizer (`Esc` clears an active search first) |

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
existing recommendations stay visually similar after the upgrade). If the active
value isn't one of the presets, the first press of `<` or `>` snaps to the nearest
one instead of stepping from it.

| Key | Action |
|---|---|
| `y` | Copy recommendations as a strategic-merge `containers[]` YAML block (pasteable into `kubectl patch`) |
| `r` | Force-refresh (invalidate the cached entry for the active strategy + headroom and re-fetch) |
| `]` | Cycle to the next available strategy (wraps around) |
| `[` | Cycle to the previous available strategy (wraps around) |
| `>` | Cycle to the next headroom multiplier (wraps around) |
| `<` | Cycle to the previous headroom multiplier (wraps around; same snap behavior as `>`) |
| `j` / `k` | Scroll up / down |
| `g` / `G` (or `Home` / `End`) | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Page down / up (half page) |
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
| `j` / `k` | Move cursor up / down |
| `gg` / `G` / `Home` / `End` | Jump to top / bottom |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Page down / up (half page) |
| `Ctrl+F` / `Ctrl+B` / `PgDn` / `PgUp` | Page down / up (full page) |
| `V` | Line visual selection |
| `v` | Character visual selection (from cursor column) |
| `h` / `l` / `Left` / `Right` | Move cursor column left / right |
| `0` / `$` / `^` | Move cursor to line start / end / first non-whitespace |
| `w` / `b` / `e` / `W` / `B` / `E` | Word / WORD motions |
| `y` | Copy selected lines (visual mode) or all entries (normal mode) |
| `Shift+F` | Toggle fullscreen / overlay mode |
| `d` | Toggle debug log visibility |
| `Esc` | Cancel visual selection, or close overlay |
| `q` | Close overlay |

> **Fullscreen mode**: Press `Shift+F` to expand the error log to full terminal size. This removes the overlay border, so mouse text selection works cleanly without picking up background characters. Press `Shift+F` again to return to overlay mode.

> **Line wrapping**: Long entries wrap onto continuation lines indented under the message column, so the full text stays readable instead of being truncated.

## Tabs

| Key | Action |
|---|---|
| `t` | New tab (clone current view) |
| `]` | Next tab |
| `[` | Previous tab |
| `}` | Move current tab right (shift+]) |
| `{` | Move current tab left (shift+[) |
| `Ctrl+C` | Close current tab (quit if last tab) |

`Ctrl+C` cancels an in-progress bulk action on the current tab first, rather than closing it. It only cancels a bulk action started on that tab, not one running in another tab.

## Read-only mode

| Key | Action |
|---|---|
| `Ctrl+R` (inside a context) | Toggle read-only mode for the current tab. Recorded as a session override for that context, so it survives re-entry and stays in sync with the picker's `[RO]` marker; does not write to config and never leaks across contexts. Blocked when `--read-only` is set. |
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

## Cluster color coding

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
| Wheel up / down inside a centered overlay | Scroll the list cursor (same as `j` / `k` / arrow keys) |
| Scroll wheel over middle pane | Move the row selection up / down |
| Scroll wheel over right pane | Scroll the preview under the pointer |
| Scroll wheel in the Object Explorer / log viewer | Same per-pane routing — over the preview pane it scrolls the preview, over the list it moves the cursor |
| `Ctrl+Option+Y` | Toggle mouse capture — release it to select text where `Shift+Drag` doesn't work, press again to re-enable |
| Shift+Drag | Select text (host terminal) |
| Shift+Option+Drag (macOS) / Alt+Drag (Linux, Windows) | Block-select text inside the embedded PTY |

The status bar shows a `[MOUSE OFF]` chip while capture is suspended. Mouse capture (`Ctrl+Option+Y`) is rebindable via the `mouse_toggle` config key.

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
| `Ctrl+D` / `Ctrl+U` | Scroll suggestions (half page down / up) |
| `Ctrl+F` / `Ctrl+B` | Scroll suggestions (full page down / up) |
| `Ctrl+Space` | Open / refresh suggestions |
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

## Named Sessions

A session is the whole multi-tab workspace (each tab's context, namespace scope, resource type, filter, and cursor). lfk always has one **active session** that it auto-saves to on quit and restores on start.

- **Default session**: the built-in workspace stored in `session.yaml`. It shows as `default` in the picker and is what you get with no named session active.
- **Named sessions** live in `sessions/<name>.yaml` and are separate — working in one never overwrites another (or the default).

| Key / Command | Action |
|---|---|
| `C` (or `:sessions`) | Open the session manager |
| `:session save <name>` | Save the current workspace as a named session |
| `:session delete <name>` | Delete a named session |

Inside the manager: `enter` switch (auto-saves the one you're leaving), `s` save current workspace under a new name (becomes active), `d` delete the highlighted session (not `default`), `/` filter, `esc` close. A checkmark marks the active session.

**Startup:** `lfk --session <name>` (or the `LFK_SESSION` env var) opens that session, creating it on first save if new. The active session is remembered across restarts; `--session` is mutually exclusive with `--context`.

## General

| Key | Action |
|---|---|
| `T` | Switch color scheme |
| `q` | Quit application (with confirmation) |
| `Esc` | Go back one level / close overlay / quit |

## Action Menu items

The action menu (`x` key) shows context-specific actions based on the resource type.

Every kind also offers `x` → `T` Export Template: strips server-set fields, then asks for a destination (clipboard, file, or template list). Secret values are blanked, keys kept. Available in read-only mode.

The destination picker: `j`/`k` (or `↓`/`↑`, `Ctrl+N`/`Ctrl+P`) move the cursor, `c` clipboard, `f` file, `t` template list, `Enter` export to the highlighted destination, `s` fields to remove, `Esc`/`q` cancel.

The field picker (`s` from the destination picker) chooses which categories the export removes. `j`/`k` move, `space` toggles, `r` restores the defaults, `Esc` returns to the destinations. Choices persist across restarts in `export_strip_prefs.yaml` in the state directory.

| Category | Default | Removes |
|---|---|---|
| Namespace | removed | `metadata.namespace` |
| Labels | kept | every author-written label |
| Annotations | kept | every author-written annotation |
| Helm ownership | removed | Helm chart / release labels and annotations |
| Vendor runtime annotations | removed | `cni.projectcalico.org/*`, `field.cattle.io/*`, `management.cattle.io/*` |
| Secret values | removed | Secret values; keys and `type` kept |

Helm ownership removes `helm.sh/chart`, `meta.helm.sh/*`, `heritage`, `release`, `chart`, and `app.kubernetes.io/managed-by` when its value is `Helm`.

Locked rows offer no choice — keeping them yields a manifest that will not apply: `status` and the server-set metadata (`uid`, `resourceVersion`, `generation`, `creationTimestamp`, `managedFields`, `selfLink`, `ownerReferences`), `last-applied-configuration`, finalizers, and controller-generated labels (`pod-template-hash`, `controller-uid`, `job-name`).

The template picker (`a`): `Enter` create, `/` filter, `d` delete the highlighted saved template after a confirmation, `Esc`/`q` close. `d` works on your own templates only — the built-ins have no file behind them.

### Pod actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec, `A` Attach, `B` Debug, `b` Debug Pod, `p` Port Forward, `c` Capture Traffic, `N` Network Policies (policies whose pod selector matches this pod), `S` Startup Analysis, `I` Crash Investigator, `v` Describe, `E` Edit, `z` Right-sizing, `D` Delete, `X` Force Delete, `V` Events

### Deployment actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec, `A` Attach, `S` Scale, `r` Restart, `R` Rollback, `p` Port Forward, `v` Describe, `E` Edit, `z` Right-sizing, `D` Delete, `b` Debug Pod, `V` Events

### StatefulSet actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec, `A` Attach, `S` Scale, `r` Restart, `p` Port Forward, `v` Describe, `E` Edit, `z` Right-sizing, `D` Delete, `b` Debug Pod, `V` Events

### DaemonSet actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec, `A` Attach, `r` Restart, `p` Port Forward, `v` Describe, `E` Edit, `z` Right-sizing, `D` Delete, `b` Debug Pod, `V` Events

### HorizontalPodAutoscaler actions
`S` Scale (edit min/max bounds & target replicas), `E` Edit, `D` Delete, `v` Describe, `b` Debug Pod, `V` Events

The Scale overlay edits the HPA's `spec.minReplicas` / `spec.maxReplicas` (the HPA keeps autoscaling within the new range) and, optionally, scales the target workload directly. Fields prefill from the HPA's current values. `j`/`k` (or `↓`/`↑`) move between the three fields; `h`/`-` and `l`/`+` decrement/increment the active field; `←`/`→` move the cursor within the field; digits type a value directly. Target replica changes may be reverted by the HPA on its next reconcile. The same `h`/`-`/`l`/`+` steppers work in the workload Scale overlay.

### Service actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec (into pod behind service), `A` Attach (to pod behind service), `p` Port Forward, `O` Port Forward & Open (forward a port and open it in the browser), `c` Capture Traffic, `N` Network Policies (policies affecting the service's backing pods), `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### Secret actions
`e` Secret Editor, `v` Describe, `E` Edit, `D` Delete, `l` Labels / Annotations, `P` Permissions, `b` Debug Pod, `V` Events

### ConfigMap actions
`e` ConfigMap Editor, `v` Describe, `E` Edit, `D` Delete, `l` Labels / Annotations, `P` Permissions, `b` Debug Pod, `V` Events

### Node actions
`c` Cordon/Uncordon (toggle schedulability), `n` Drain, `t` Taints (editor: mark taints for removal with `space`, add with `a`, pick a common taint with `p`, apply with `enter`), `s` Shell, `v` Describe, `E` Edit, `b` Debug Pod, `V` Events

Drain streams the eviction progress live: in PTY terminal mode (default on macOS/Linux) the `kubectl drain` output renders in lfk's embedded scrollable terminal; in Exec mode the host terminal is handed over. The same applies to Drain Node on a Karpenter NodeClaim.

### Longhorn Node actions
`e` Evict Replicas, `C` Cancel Eviction, `v` Describe, `E` Edit, `D` Delete, `X` Force Delete, `V` Events, `P` Permissions

The Longhorn Nodes list shows a `REPLICAS` column with the count of replicas scheduled on each node. Force Delete disables scheduling, then deletes (the `validator.longhorn.io` webhook rejects deleting a still-schedulable node). Evict Replicas disables scheduling and sets `spec.evictionRequested`, so Longhorn rebuilds each replica on another node before removing it.

### Job actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec, `A` Attach, `v` Describe, `E` Edit, `z` Right-sizing, `D` Delete, `X` Force Delete, `b` Debug Pod, `V` Events

### CronJob actions
`l` Tail Logs (last N lines + follow), `L` Logs (full), `s` Exec, `A` Attach, `r` Trigger (create Job), `S` Suspend/Resume (pause/resume schedule), `v` Describe, `E` Edit, `z` Right-sizing, `D` Delete, `b` Debug Pod, `V` Events

### ArgoCD Application actions
`s` Sync, `a` Sync (Apply Only), `f` Diff, `R` Refresh, `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### Helm Release actions
`u` Values, `A` All Values, `E` Edit Values, `d` Diff, `U` Upgrade, `R` Rollback, `h` History, `v` Describe, `D` Delete, `b` Debug Pod, `V` Events

### Ingress actions
`o` Open in Browser, `v` Describe, `E` Edit, `D` Delete, `b` Debug Pod, `V` Events

### PVC actions
`g` Go to Pod, `b` Debug Mount, `B` Debug Pod, `v` Describe, `E` Edit, `D` Delete, `V` Events

### Default actions (all other resources)
`v` Describe, `E` Edit, `D` Delete, `l` Labels / Annotations, `P` Permissions, `b` Debug Pod, `V` Events

### Bulk actions (when items multi-selected)
`D` Delete, `X` Force Delete, `S` Scale, `r` Restart

ArgoCD Application bulk actions (when Application resources are multi-selected):
`s` Sync, `a` Sync (Apply Only), `R` Refresh

Custom actions defined in the config file appear after the built-in actions.

## Configuring keybindings

All keybindings can be overridden in `~/.config/lfk/config.yaml`. Only specify the keys you want to change — defaults apply for everything else.

### Key string syntax

| Form | Example | Notes |
|---|---|---|
| Plain key | `j`, `G`, `/` | Single character |
| Named key | `enter`, `esc`, `tab`, `space`, `backspace`, `up`, `pgdown`, `f1` | Use `space`, not `" "` |
| Modified key | `ctrl+d`, `alt+y`, `shift+tab` | |
| Modifier chord | `ctrl+alt+y`, `ctrl+shift+x` | Order: `ctrl`, `alt`, `shift`, `meta`, `hyper`, `super` |

Case is significant — `d` (diff) and `D` (delete) are different keys. The one exception is `shift`, which already implies the shifted character, so these are normalised on load:

| Written | Stored | Why |
|---|---|---|
| `shift+ctrl+x` | `ctrl+shift+X` | Modifiers reordered; terminals report the shifted character |
| `ctrl+shift+x` | `ctrl+shift+X` | Same |
| `shift+x` | `X` | Shift alone is not reported as a modifier |
| `shift+tab` | `shift+tab` | Named keys keep the modifier |

The legacy `" "` spelling for space is also accepted and normalised to `space`.

### Ctrl+Shift chords

`Ctrl+Shift+<key>` is bindable and stays distinct from plain `Ctrl+<key>`, so `ctrl+x` and `ctrl+shift+x` can trigger different actions.

**This requires terminal support.** A terminal that reports only legacy control codes cannot express the difference — `Ctrl+Shift+X` arrives as plain `Ctrl+X`, and the chord binding silently never fires. lfk requests the capability on every render via both mechanisms below; the terminal has to answer:

- **Kitty keyboard protocol** — Ghostty, kitty, WezTerm, foot, recent Alacritty
- **xterm `modifyOtherKeys`** — xterm and close derivatives

macOS Terminal.app supports neither and cannot use Ctrl+Shift bindings.

Under tmux you also need extended keys forwarded, or tmux flattens the chord before lfk sees it:

```tmux
set -s extended-keys on
set -as terminal-features '<your-outer-TERM>:extkeys'   # e.g. xterm-ghostty
```

Check what tmux thinks your outer terminal is with `tmux display -p '#{client_termname}'` — the `terminal-features` pattern must match it.

No default binding uses Ctrl+Shift, so nothing breaks on a terminal without support. To verify yours, bind one temporarily and press it:

```yaml
keybindings:
  force_delete: "ctrl+shift+x"
```

Avoid `ctrl+i`, `ctrl+m`, and `ctrl+[` — terminals emit those as `tab`, `enter`, and `esc`.

For example, to swap the live-log preview and the fullscreen log viewer (fullscreen on plain `L`, preview on `Ctrl+L`):

```yaml
keybindings:
  logs: "L"                      # fullscreen log viewer
  toggle_preview_logs: "ctrl+l"  # live-log preview pane
```

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
  jump_back: "backspace"     # Jump back through teleport history
  toggle_rare: "H"       # Toggle rarely used resource types in the sidebar

  # Views and Modes
  help: "f1"             # Toggle help (default "?" is taken by which_key_leader)
  filter: "f"            # Filter items
  search: "/"            # Search and jump
  toggle_preview: "P"    # Toggle YAML preview
  resource_map: "M"      # Resource map
  fullscreen: "F"        # Cycle layout: hide sidebar / fullscreen / restore
  watch_mode: "w"        # Watch mode
  command_bar: ":"        # Command bar
  theme_selector: "T"    # Theme selector
  finalizer_search: "ctrl+g"  # Finalizer search
  api_explorer: "I"      # API Explorer
  field_doc: "ctrl+k"    # Schema side pane for the field under the cursor
  object_explorer: "O"   # Object Explorer (browse live object)
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
  toggle_preview_logs: "L"  # Toggle live-log preview pane (deeper levels only)
  logs: "ctrl+l"         # Open fullscreen log viewer
  refresh: "R"           # Refresh view
  edit: "E"              # Edit in $KUBE_EDITOR or $EDITOR
  describe: "v"          # Describe resource
  delete: "D"            # Delete resource
  force_delete: "X"      # Force delete
  scale: "S"             # Scale resource
  label_editor: "i"      # Labels/annotations
  secret_editor: "e"     # Secret/configmap editor
  create_template: "a"   # Create from template
  copy_name: "y"         # Copy name
  copy_yaml: "Y"         # Open copy-as picker (YAML/JSON/Table)
  copy_field: "ctrl+y"   # Copy a single manifest field (filterable picker)
  paste_apply: "ctrl+p"  # Apply from clipboard
  open_browser: "ctrl+o" # Open in browser
  diff: "d"              # Diff resources

  # Multi-selection
  which_key_leader: "?"  # Open the which-key action panel
  toggle_select: "space" # Toggle selection (space bar)
  select_range: "ctrl+space" # Select range (legacy "ctrl+@" still accepted)
  select_all: "ctrl+a"   # Select all

  # Tabs
  new_tab: "t"           # New tab
  next_tab: "]"          # Next tab
  prev_tab: "["          # Previous tab
  move_tab_left: "{"     # Move current tab left
  move_tab_right: "}"    # Move current tab right

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

| Key | Action |
| --- | --- |
| `Tab` / `S-Tab` | Cycle tabs forward / backward |
| `1` / `2` / `3` / `4` | Jump to Summary / Events / Logs / Describe |
| `c` | Cycle active container (init + app) |
| `p` | Toggle previous / current logs (Logs tab only) |
| `j` / `k` | Scroll within tab body |
| `g` / `G` | Jump to top / bottom of tab body |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Half-page down / up |
| `Ctrl+F` / `Ctrl+B` | Full-page down / up (also `PgDn` / `PgUp`) |
| `Shift+R` | Refresh — re-fetch all sections, preserves cursor state |
| `Esc` / `q` | Close overlay |

### Sync Wave Timeline (Applications)

Open from an `Application` row: press `x` for the action menu, then `W`.
Two-pane layout: phases on the left sidebar, the selected phase's content
on the right. `Tab` toggles which pane has focus.

**When sidebar has focus:**

| Key | Action |
| --- | --- |
| `j` / `↓` | Move sidebar cursor down (wraps); resets body cursor + scroll |
| `k` / `↑` | Move sidebar cursor up (wraps) |
| `Enter` / `Space` | Toggle phase collapse |
| `Tab` / `Shift+Tab` | Switch focus to body |
| `g` / `G` | First / last phase |

**When body has focus:**

| Key | Action |
| --- | --- |
| `j` / `↓` | Move body cursor down (wave headers + visible resources) |
| `k` / `↑` | Move body cursor up |
| `Enter` / `Space` | Toggle wave / phase collapse (no-op on a resource) |
| `Tab` / `Shift+Tab` | Switch focus to sidebar |
| `g` / `G` | First / last visible body row |
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Half-page scroll |
| `Ctrl+F` / `Ctrl+B` (or `PgDn` / `PgUp`) | Full-page scroll |

**Shared:**

| Key | Action |
| --- | --- |
| `R` | Refresh |
| `q` / `Esc` | Close overlay |

In the sidebar, `Enter` / `Space` toggles collapse for the phase under the cursor: the sidebar marker switches between `▾` and `▸`, and the body shows a placeholder for a collapsed phase.

In the body, `Enter` / `Space` depends on the cursor: on a wave header it toggles that wave's collapse, on a placeholder it toggles the phase collapse, and on a resource row it does nothing.

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
| `Ctrl+D` / `Ctrl+U` / `Shift+↓` / `Shift+↑` | Half-page scroll older / newer |
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
