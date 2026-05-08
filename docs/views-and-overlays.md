# Views and Overlays

Reference list of every view, fullscreen flag, and overlay lfk renders.
Use this when:

- finding the responsible code path while changing UI behavior,
- keeping the help screen in sync, or
- remembering which mode owns which keymap.

## Concepts

lfk's UI is built from three layered concepts:

- **View** (`viewMode`) — top-level screen mode. Only one is active at a
  time and it owns the keyboard. Switching between views replaces the
  visible UI (e.g. opening logs leaves the explorer entirely). Stored
  per tab on `TabState.mode` and mirrored to `Model.mode`.
- **Fullscreen flag** — boolean on `Model` (and sometimes `TabState`)
  that lets a sub-pane take the whole screen *without* leaving
  `modeExplorer`. Multiple flags can coexist; precedence is decided in
  `viewExplorerColumns` (see [`internal/app/view.go`](../internal/app/view.go)).
- **Overlay** — modal panel that floats over the current view. The
  underlying view is dimmed but stays mounted. Overlays own the keyboard
  while open and dismiss back to the view they appeared over. Most are
  typed via `overlayKind` and mutually exclusive; a few live as
  independent boolean fields on `Model` so they can stack on top of an
  `overlayKind` overlay (see [Boolean overlays](#boolean-overlays)).

Source of truth for the enums:
[`internal/app/app_types.go`](../internal/app/app_types.go) — `viewMode`
(line ~14) and `overlayKind` (line ~31). When those enums change, update
this doc and [`internal/ui/help.go`](../internal/ui/help.go).

Default trigger keys below come from
[`internal/ui/config_keybindings.go`](../internal/ui/config_keybindings.go);
users can rebind any of them — see [keybindings.md](./keybindings.md)
for the full reference.

## Views (`viewMode`)

Listed in `viewMode` declaration order.

| Mode              | Default trigger                       | Purpose                                                                |
| ----------------- | ------------------------------------- | ---------------------------------------------------------------------- |
| `modeExplorer`    | default                               | Three-column resource browser (clusters → resource types → resources). |
| `modeYAML`        | `Enter` on a resource                 | Full-screen YAML preview with search and copy.                         |
| `modeHelp`        | `?`                                   | Searchable, filterable keybinding reference.                           |
| `modeLogs`        | `L` on a pod / workload               | Log viewer with follow, wrap, search, visual selection.                |
| `modeDescribe`    | `v` on a resource                     | `kubectl describe`-style detail view.                                  |
| `modeDiff`        | `d` between two selected resources    | Side-by-side diff (e.g. ArgoCD live vs. desired).                      |
| `modeExec`        | action menu → `s`                     | Embedded PTY shell session.                                            |
| `modeExplain`     | `I`, `:explain <type>`                | `kubectl explain` field tree.                                          |
| `modeEventViewer` | event timeline overlay → drill-in     | Full-screen event viewer with grouping.                                |
| `modeKubetris`    | `:kubetris`                           | Easter-egg game.                                                       |
| `modeCredits`     | `:credits`                            | Scrolling credits screen.                                              |

## Fullscreen flags inside `modeExplorer`

These are boolean fields, not enum values. They take over the explorer
screen while staying inside `modeExplorer`. Multiple flags can be set at
once — `viewExplorerColumns` (in `internal/app/view.go`) decides which
wins.

| Flag                      | Scope    | Default trigger                       | What it shows                                                                                                                |
| ------------------------- | -------- | ------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `fullscreenMiddle`        | per-tab  | `F`                                   | Hides left and right columns; the middle resource list takes the whole screen.                                                |
| `fullscreenDashboard`     | per-tab  | `Enter` on `[Dashboard]`, `@`         | Cluster or monitoring dashboard (see Note below).                                                                            |
| `errorLogFullscreen`      | global   | inside the error-log overlay (`F`)    | Promotes the error-log overlay to a full-screen log buffer.                                                                   |
| `eventTimelineFullscreen` | global   | inside the event-timeline overlay     | Promotes the event timeline overlay to a full-screen viewer (also reachable via the `modeEventViewer` drill).                 |

> Note: there is **no** separate `fullscreenMonitoring` flag — Monitoring
> shares `fullscreenDashboard` and is distinguished by the selected
> item's `Extra == "__monitoring__"` (see `viewExplorerDashboard` in
> `internal/app/view.go`).

## Overlays (`overlayKind`)

Mutually exclusive — opening a new one replaces any prior open overlay.
Tables below follow `overlayKind` declaration order within each
category, and every entry maps to a constant in `app_types.go`.

### Pickers (single-shot select-then-act)

| Overlay                     | Default trigger                  | Purpose                                                                                                                                                                                                              |
| --------------------------- | -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `overlayNamespace`          | `\`, `:namespace`                | Pick / multi-select namespaces.                                                                                                                                                                                      |
| `overlayContainerSelect`    | `c` in pod log view              | Pick container when a pod has multiples.                                                                                                                                                                             |
| `overlayPodSelect`          | `\` in log view                  | Switch to a sibling pod's logs.                                                                                                                                                                                      |
| `overlayTemplates`          | template-create flow             | Pick a built-in resource template.                                                                                                                                                                                   |
| `overlayColorscheme`        | `T`                              | Theme picker (search + preview).                                                                                                                                                                                     |
| `overlayFilterPreset`       | `.`                              | Saved filter expressions.                                                                                                                                                                                            |
| `overlayCanISubject`        | `:can-i` flow                    | Pick the user / SA to evaluate as.                                                                                                                                                                                   |
| `overlayExplainSearch`      | search inside `modeExplain`      | Type / field search for `kubectl explain`.                                                                                                                                                                           |
| `overlayLogPodSelect`       | `\` in fullscreen log mode       | Switch pods within fullscreen log mode.                                                                                                                                                                              |
| `overlayLogContainerSelect` | container key in fullscreen logs | Container picker within log mode.                                                                                                                                                                                    |
| `overlayFinalizerSearch`    | `Ctrl+G`                         | Pick a finalizer to remove.                                                                                                                                                                                          |
| `overlayColumnToggle`       | `,`                              | Show / hide table columns per kind.                                                                                                                                                                                  |
| `overlayClusterColor`       | `L` (Shift+L) at cluster picker  | Pick a background tint for the highlighted cluster row.                                                                                                                                                              |

### Navigators (jump-driven)

| Overlay            | Default trigger      | Purpose                              |
| ------------------ | -------------------- | ------------------------------------ |
| `overlayBookmarks` | `'`, `:bookmarks`    | Saved navigation slots, with filter. |

### Editors / Forms

| Overlay                  | Default trigger                | Purpose                                            |
| ------------------------ | ------------------------------ | -------------------------------------------------- |
| `overlayScaleInput`      | `S` on workload                | Replica count input.                                |
| `overlayPortForward`     | `p` on Service / Pod           | Port-forward destination input.                     |
| `overlaySecretEditor`    | `e` on Secret                  | Inline edit of decoded secret values.               |
| `overlayConfigMapEditor` | `e` on ConfigMap               | Inline edit of CM keys.                             |
| `overlayLabelEditor`     | `i` on a resource              | Add / remove labels and annotations.                |
| `overlayBatchLabel`      | `i` with multi-selection       | Apply labels and annotations to selected items.     |
| `overlayPVCResize`       | resize on PVC (action menu)    | New PVC size input.                                 |

### Action menus

| Overlay         | Default trigger | Purpose                                                       |
| --------------- | --------------- | ------------------------------------------------------------- |
| `overlayAction` | `x`             | Resource-kind-specific action menu (delete, rollback, etc.).  |

### Confirmations

| Overlay              | Default trigger                  | Purpose                                                      |
| -------------------- | -------------------------------- | ------------------------------------------------------------ |
| `overlayConfirm`     | delete / drain                   | y/n confirmation for reversible actions.                      |
| `overlayConfirmType` | force delete / force finalize    | Requires typing `DELETE` for destructive ops.                 |
| `overlayQuitConfirm` | `q`                              | Confirm before exiting lfk.                                   |
| `overlayPasteConfirm`| paste into search / filter       | Confirm multi-line paste.                                     |

### Information panels

| Overlay                  | Default trigger                 | Purpose                                                        |
| ------------------------ | ------------------------------- | -------------------------------------------------------------- |
| `overlayRollback`        | action menu → `R` on Deploy/STS | Pick revision to roll back to.                                 |
| `overlayHelmRollback`    | action menu → `R` on Helm       | Pick Helm revision to roll back.                               |
| `overlayHelmHistory`     | action menu → `h` on Helm       | Browse Helm release history.                                   |
| `overlayRBAC`            | `U`                             | RBAC subject / role browser.                                   |
| `overlayPodStartup`      | action menu → `S` on Pod        | Pod init / readiness gantt.                                    |
| `overlayCrashInvestigator` | action menu → `I` on Pod      | Per-pod CrashLoopBackOff investigator.                          |
| `overlayQuotaDashboard`  | `Q`, `:quota`                   | Per-namespace ResourceQuota usage.                              |
| `overlayEventTimeline`   | `V`                             | Cluster-wide events grouped by object.                          |
| `overlayAlerts`          | from monitoring view            | Active Prometheus alerts.                                       |
| `overlayNetworkPolicy`   | from netpol view                | Visualize selected NetworkPolicy.                               |
| `overlayCanI`            | `:can-i` flow (after subject)   | Display can-i evaluation results.                               |
| `overlayAutoSync`        | ArgoCD app                      | Toggle auto-sync settings.                                      |
| `overlaySyncWave`        | action menu → `W` on Application | Per-Application ArgoCD sync wave timeline.                      |
| `overlayBackgroundTasks` | `` ` ``, `:tasks`               | In-flight + recent background tasks.                            |
| `overlayOrphans`         | `Shift+O`, `:orphans`           | Cluster-wide orphan resource overview.                          |
| `overlayLocalClusters`   | `Ctrl+N` at LevelClusters       | Manage kind/k3d/minikube clusters.                              |

## Boolean overlays

A few overlay-like states live as plain booleans on `Model` rather than
inside `overlayKind`. They can stack with a normal `overlayKind` overlay
and follow their own dismissal rules.

| Field              | Default trigger | Purpose                                                                                                               |
| ------------------ | --------------- | --------------------------------------------------------------------------------------------------------------------- |
| `overlayErrorLog`  | `!`             | Application error log overlay (lfk's own error buffer). Has its own `errorLogFullscreen` toggle for full-screen mode. |
| `commandBarActive` | `:`             | Bottom-of-screen `:command [args]` input with autocomplete, history, and ghost-text preview.                          |

## Adding a new view, fullscreen flag, or overlay

1. Pick the right shape:
   - **`viewMode`** — the new screen has its own keymap and replaces the
     explorer entirely (e.g. exec, logs).
   - **Fullscreen flag** on `TabState` and/or `Model` — the new screen
     lives inside `modeExplorer` and shares its tab state.
   - **`overlayKind`** — modal panel over the current view, mutually
     exclusive with other overlays.
   - **Boolean overlay** — *only* when the new state must coexist with
     another overlay (mirror `overlayErrorLog`'s pattern).
2. Add the constant in
   [`internal/app/app_types.go`](../internal/app/app_types.go), or the
   boolean field on `Model` / `TabState` in
   [`internal/app/app.go`](../internal/app/app.go).
3. Wire the trigger key in `internal/app/update_keys*.go`. If the
   trigger is user-facing, reserve a binding in
   [`internal/ui/config_keybindings.go`](../internal/ui/config_keybindings.go).
4. Add the renderer in `internal/ui/`.
5. For a fullscreen flag, plug it into `viewExplorerColumns`'s
   precedence switch in `internal/app/view.go`.
6. Update this doc and [`docs/keybindings.md`](./keybindings.md).
7. Surface the binding in the `?` help screen
   ([`internal/ui/help.go`](../internal/ui/help.go)).
