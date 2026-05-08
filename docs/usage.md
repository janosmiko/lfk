# Usage

CLI flags, environment variables, and runtime tuning options for `lfk`.

## Command-line flags

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

# Disable all mutating actions (delete, edit, scale, restart, exec, etc.)
lfk --read-only

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

## Mouse Support

By default, lfk captures mouse input for click navigation, scroll, and tab
switching. If you need native terminal text selection (e.g., shift+click to
select text), you can disable mouse capture:

- **CLI flag:** `lfk --no-mouse`
- **Config file:** Add `mouse: false` to `~/.config/lfk/config.yaml`

### Explorer click semantics

The three-pane explorer maps mouse clicks to navigation actions:

| Click | Action |
|---|---|
| Left pane | Drill out one level (same as `h` / Left arrow) |
| Middle pane (different row) | Select that row and load its preview in the right pane |
| Middle pane (cursored row) | Drill into it (same as `Enter` / Right arrow) |
| Right pane | Drill into the selected item |
| Table header row | Sort by that column; click again toggles direction |
| Right-click on middle pane | Move cursor to the clicked row and open the action menu |
| Right-click on right pane | Open the action menu for the currently selected item |
| Right-click on left pane | No-op |
| Wheel up / down | Scroll the focused pane |

The two-step "select then click again to drill" flow lets you scan items and
preview them in the right pane without committing — equivalent to using `j` /
`k` followed by `Enter` on the keyboard.

### Overlay click semantics

While an overlay is open (action menu, namespace selector, confirm dialog,
etc.) the mouse interacts with the overlay rather than the explorer
underneath:

| Click | Action |
|---|---|
| Click outside the overlay box | Dismiss the overlay (same as `Esc`) |
| Click on a row in the action menu | Run that action (same as `Enter`) |
| Click on a row in the namespace selector | Apply that namespace and close |
| Click on the namespace badge in the title bar | Open the namespace selector |
| Wheel up / down inside an overlay | Scroll the list cursor — three rows per tick (same as pressing `j` / `k`) |
| Click inside the box on padding/title/border | No-op (use `Esc` or click outside to dismiss) |

Other list overlays (color scheme picker, bookmarks, templates, container /
pod / log selectors, column toggle, etc.) accept wheel scroll; row-click
activation is being added incrementally — for now use `Enter` after wheel
scrolling to the desired row.

Fullscreen overlays (the secret / config-map / label editors, rollback /
helm-history pickers, the auto-sync dialog, the Can-I browser, and the
NetworkPolicy viewer) cover the entire screen, so there is no "outside" to
click; press `Esc` to close them.

> macOS Terminal.app does not support shift+click text selection while mouse
> capture is active. Use `--no-mouse` or switch to a terminal that handles this
> correctly (iTerm2, Kitty, Alacritty, WezTerm, Ghostty).

## No-Color Mode

Disable all foreground and background colors while keeping selection and
other highlights visible via bold, underline, and reverse-video SGR codes.
Useful for monochrome terminals, piped output, or lower CPU usage.

- **CLI flag:** `lfk --no-color`
- **Environment variable:** `NO_COLOR=1 lfk` (any non-empty value; see
  [no-color.org](https://no-color.org/))
- **Config file:** Add `no_color: true` to `~/.config/lfk/config.yaml`

Precedence: `--no-color` flag > `NO_COLOR` env var > config file.

## Read-Only Mode

Read-only mode disables every action that changes cluster state — delete,
edit, scale, restart, rollback, exec, attach, port-forward, drain, cordon,
taint, label/annotation edits, secret/configmap edits, paste-apply, and
template create. Listing, describing, viewing logs, viewing YAML, diff,
and other read paths still work.

Read-only is per-context (and per-tab). The UI surfaces it in two places:

- **Inside a context** (any level deeper than the cluster picker): the
  title bar shows a `[RO]` badge for the current tab, and mutating
  shortcuts are filtered out of the action menu and hint bar.
- **At the cluster picker**: each context row shows a `[RO]` suffix when
  it is configured read-only (per-context config, global config, or the
  `--read-only` CLI flag). The title-bar badge is suppressed here — it
  has no specific context to refer to. The per-row marker is the
  declarative view of which clusters are locked.

Read-only is opt-in at four levels (precedence highest first):

1. **CLI flag (sticky):** `lfk --read-only`. Once set, the flag stays on
   for the life of the process — context switches cannot drop it, and
   the picker row toggle is rejected with a status hint.
2. **Session row toggle (`Ctrl+R` at the cluster picker):** highlights
   a context and presses `Ctrl+R` to flip its `[RO]` marker. The toggle
   is recorded for the session, persists across re-navigation to the
   picker, and is honored when entering that context.
3. **Per-context config:** lock specific clusters by name.
   ```yaml
   clusters:
     prod:
       read_only: true
     audit-cluster:
       read_only: true
   ```
4. **Global config:** `read_only: true` at the top level applies to every
   context.

Read-only state is per-tab. New tabs inherit the setting from the active
tab when created and re-evaluate on context switch.

### In-app toggle

`Ctrl+R` behavior depends on where you are:

- **At the cluster picker**: flips the `[RO]` marker on the highlighted
  context row. The toggle is stored as a session override that wins
  over per-context and global config when entering that context.
  Persists across back-and-forth navigation to the picker; cleared on
  process exit. Blocked when `--read-only` is set.
- **Inside a context**: flips read-only for the current tab.
  Session-scoped — does not write to the config file, and does not leak
  across context switches. Blocked when `--read-only` is set; the CLI
  flag is the strongest precedence level and cannot be defeated within
  the running process.

### CLI flag

```sh
lfk --read-only            # all contexts read-only for the session
lfk --context prod --read-only
```

`--read-only` is process-wide. `--context` only selects the starting
context; it does not scope the read-only flag.

### Discovery

The cluster picker hint bar advertises `Ctrl+R toggle RO` so users
can find the row toggle without reading docs.

## Node Shell

From the Node action menu (`x` → `s`), lfk launches a privileged debug pod
that `nsenter`s into PID 1 on the selected node, giving an interactive shell
with the host's mount, network, PID, and IPC namespaces.

The pod runs in `kube-system` with `priorityClassName: system-node-critical`
so it can land on nodes reporting DiskPressure / MemoryPressure / PIDPressure,
and tolerates all taints. It is created with `--rm --restart=Never`, so it
is removed when the shell exits.

Node shell is a mutating action and is gated by the read-only check.

## Cluster Color Coding

Tag any cluster with a background color so the title bar tints the
moment you enter it — useful for "I am unmistakably in prod" feedback
when a stray `D` would do real damage.

- **Open the picker**: at the cluster picker, highlight a row and press
  `L` (Shift+L). Same overlay opens from the action menu (`x` →
  "Set color…").
- **Pick a color**: 8 named choices (`red`, `yellow`, `green`, `blue`,
  `magenta`, `cyan`, `white`, `gray`) plus `None` to clear. The cursor
  pre-seeds on the cluster's current color, or on `None` if it has none.
- **Visual treatment**: the picker row gets a `██` swatch in the chosen
  color so all your clusters are recognizable at a glance, and entering
  the context tints the entire title-bar background with the same color
  while the existing badges (`[RO]`, watch indicator, namespace, etc.)
  stay legible on top.
- **Persistence**: the assignment is saved to
  `$XDG_STATE_HOME/lfk/cluster-colors.yaml` (defaults to
  `~/.local/state/lfk/cluster-colors.yaml`) so colors survive restarts.
  The file is lfk-managed — don't hand-edit; use the in-app picker.
  Unknown color names in the file are ignored on load (with a warning
  written to the lfk log) so a typo doesn't poison neighbouring entries.

Four of the colors follow lfk's active theme so they re-skin when you
switch colorschemes:

| Picker name | Theme token             |
| ----------- | ----------------------- |
| `red`       | `theme.Error`           |
| `yellow`    | `theme.Warning`         |
| `green`     | `theme.Secondary`       |
| `blue`      | `theme.Primary`         |

The remaining four (`magenta`, `cyan`, `white`, `gray`) stay on ANSI
bright codes so they look the same regardless of which lfk theme is
active — useful when none of the theme accent colors fit a particular
cluster's identity.

## Watch-Mode Interval

Watch mode (toggle with `w`) polls the current resource list on an interval.
The default 2-second interval is a good balance between freshness and API
load. Tune it with:

- **CLI flag:** `lfk --watch-interval 5s` (accepts Go durations: `500ms`,
  `2s`, `1m`, ...)
- **Config file:** Add `watch_interval: 5s` to `~/.config/lfk/config.yaml`

Values outside `[500ms, 10m]` are clamped to the bounds; invalid values fall
back to 2s.

## Discovery Cache

API discovery (the list of resource types and CRDs the server exposes) is
cached on disk under `~/.kube/cache/discovery/<host>/` with a 5-minute TTL.
Layout matches `kubectl` and `k9s` so the same cache is shared across all
three tools — a cold start hits zero discovery round-trips when the cache
is warm. On busy clusters with many CRDs this eliminates redundant
discovery round-trips on the API server and reduces startup time.

- **Override location:** Set `KUBECACHEDIR` (same env var `kubectl`
  honors) to relocate `<KUBECACHEDIR>/discovery/...` and
  `<KUBECACHEDIR>/http/...`.
- **Force refresh:** Press `R` (Shift+r) at the resource types level to
  invalidate the cache and re-run discovery — newly installed or removed
  CRDs show up immediately without restarting `lfk`.
- **TTL:** 5 minutes. Stale entries are refetched automatically on the
  next discovery request.

## Endpoint Visibility

The right-pane preview for the **Endpoints** and **EndpointSlices** kinds
shows every endpoint individually, with target pod, node, and ready state:

```text
READY      3
NOT READY  1
PORTS      http:80/TCP

ENDPOINTS
  192.168.1.5  → pod/foo-7d9         on node-a
  192.168.1.6  → pod/foo-7d9-9fr     on node-b
  192.168.1.7  → pod/foo-7d9-2qx     on node-a
  192.168.1.8  → pod/foo-7d9-broken  on node-c   (NotReady)
```

Ready endpoints render with no status suffix; only `(NotReady)` is shown
inline so the eye is drawn to the broken ones. A row degrades gracefully
when `targetRef` (no target pod) or `nodeName` (host-network endpoints)
is absent. Multiple addresses on a single EndpointSlice entry (dual-stack
IPv4/IPv6 setups) each get their own line.

The **Service** preview includes its own rollup, fetched lazily on hover
from the matching `EndpointSlice` resources (label
`kubernetes.io/service-name=<svc>`):

```text
BACKING ENDPOINTS  3 ready / 1 not ready

ENDPOINTS
  10.0.0.1  → pod/foo-7d9         on node-a
  10.0.0.2  → pod/foo-7d9-9fr     on node-b
  10.0.0.3  → pod/foo-7d9-2qx     on node-a
  10.0.0.4  → pod/foo-7d9-broken  on node-c   (NotReady)
```

The fetch is stale-while-revalidate: the cached rollup paints instantly
and a fresh fetch fires in parallel, so pod churn is reflected within one
watch tick without blanking the row mid-rebuild.

Headless Services (`clusterIP: None`) and `ExternalName` Services are
skipped — they have no backing EndpointSlices to roll up.

For a fuller `kubectl describe`-style view of a Service (events,
session affinity, etc.), press `v` (Describe).

## Orphan detection

`lfk` flags Kubernetes resources that have no live consumer or controller, across 11 kinds:

- **Pods without owners** — typically debug / one-off pods that escaped a controller. Static pods (kubelet-managed) are excluded; terminal pods older than an hour are tagged separately.
- **Secrets / ConfigMaps not mounted** — anything not referenced by a Pod (volumes, env, envFrom, imagePullSecrets), an Ingress (`spec.tls.secretName`), or a ServiceAccount.
- **Services with no endpoints** — non-Headless, non-ExternalName Services with zero backing addresses.
- **PersistentVolumeClaims not mounted** — bound but no Pod or workload template references them.
- **HorizontalPodAutoscalers with a missing target** — `scaleTargetRef` doesn't resolve.
- **PodDisruptionBudgets / NetworkPolicies that match nothing** — selector resolves to zero live or templated pods.
- **Roles / ClusterRoles with no binding** — no RoleBinding or ClusterRoleBinding refers to them.
- **RoleBindings / ClusterRoleBindings with a missing role or empty subjects.**

### Cluster-wide overview

Press **`Shift+O`** anywhere in the explorer (or type `:orphans` in the command bar) to open the orphan overview overlay. It scans the cluster and lists every orphan in one table grouped by kind:

- `Tab` / `Shift+Tab` cycle through every kind filter chip (All plus all supported orphan kinds rendered in the strip)
- `s` toggles strict / lenient — strict (default) hides items referenced by workload templates (e.g. CronJob between firings, scaled-to-zero Deployment); lenient surfaces them
- `/` filters by namespace + name
- `Enter` jumps straight to the highlighted resource (the namespace switches automatically)
- `R` re-scans the cluster
- `Esc`, `q`, or `Shift+O` close the overlay

Partial-RBAC denials surface as a warning banner at the top of the overlay; whatever could be listed is still shown.

### Per-kind filter presets

Inside a list for any supported kind, press **`.`** to open the filter-preset overlay and pick the orphan preset. `:orphans <kind>` (e.g. `:orphans secrets`, `:orphans pvcs`, `:orphans rolebindings`) jumps to the kind's list with the preset already applied.

### Auto-exclusions

To avoid false positives, the detector excludes these system-managed resources:

| Resource    | Excluded When                                                  |
| ----------- | -------------------------------------------------------------- |
| Pod         | Static pod (`kubernetes.io/config.mirror` annotation)          |
| Secret      | `type=helm.sh/release.v1` (Helm release storage)               |
| Secret      | `type=kubernetes.io/service-account-token` (auto-generated)    |
| Secret/CM   | Has any `ownerReference` (managed by another controller)       |
| ConfigMap   | Named `kube-root-ca.crt` (auto-injected per namespace)         |
| Service     | Headless (`clusterIP=None`) or `type=ExternalName`             |

## Traffic capture

Press `c` on a Pod or Service to open the capture overlay. Backends
auto-detected:

| Backend | Mechanism | Requirements |
|---|---|---|
| `kubectl debug` | Ephemeral container streaming `tcpdump` | kubectl 1.30+, ephemeral containers enabled, target pod can grant `NET_ADMIN` / `NET_RAW` |
| kubeshark | Hand-off (port-forward + browser) | `kubeshark-hub` Service in `kubeshark` namespace (override via `kubeshark.namespace`) |

Hardened non-root pods can't grant `NET_ADMIN` / `NET_RAW`; capture fails
with a message pointing at kubeshark. Install with
`helm install kubeshark kubeshark/kubeshark -n kubeshark --create-namespace`.

pcap output: `$XDG_STATE_HOME/lfk/captures/` (default
`~/.local/state/lfk/captures/`). The full per-phase keymap lives in
[`docs/keybindings.md`](keybindings.md#traffic-capture-overlay); the
load-bearing keys:

- `s` — stop, stay in overlay
- `Esc` — stop and stay; second `Esc` dismisses + deletes the pcap
  unless `Y` was pressed
- `Y` — copy pcap path to system clipboard, marks the capture as saved
- `e` (from stopped phase) — re-open config to tweak the filter, then
  `Enter` to restart
- `__captures__` (Networking sidebar group) — cluster-wide list of
  running and stopped captures

Read-only mode (`--read-only`, or `Ctrl+R`) blocks the kubectl-debug
backend (which creates an ephemeral container). The kubeshark hand-off
stays available even though it uses port-forward internally — the
listed [Read-Only](#read-only-mode) `port-forward` block applies to the
user-triggered Port Forward action, not to the kubeshark backend's
read-only tunnel to its in-cluster hub.

## Secret Lazy Loading

On clusters with many Helm releases or large TLS secrets, listing the
Secrets resource type can transfer tens of megabytes. Enable lazy loading
to fetch only metadata for the list and defer decoded values to hover:

- **Config file:** Add `secret_lazy_loading: true` to `~/.config/lfk/config.yaml`

Trade-off: the Type column is dropped from the list (metadata-only fetch
doesn't include it) and there's a small per-hover fetch for decoded values
(cached thereafter). See [Configuration Reference](config-reference.md#secret-lazy-loading)
for details.
