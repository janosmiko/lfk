# Views and Overlays

Reference list of every full-screen view and overlay lfk renders. Use this
to find the responsible code path when changing UI behavior, to keep the
help screen in sync, or to remember which mode owns which keymap.

The two concepts are distinct in code:

- A **view** is a top-level screen mode. Only one is active at a time and
  it owns the keyboard. Switching between views replaces the visible UI
  (e.g. opening logs leaves the explorer entirely).
- An **overlay** is a modal panel that floats over the current view. The
  underlying view is dimmed but stays mounted. Overlays own the keyboard
  while open and dismiss back to the view they appeared over.

Source of truth: [`internal/app/app.go`](../internal/app/app.go) defines
`viewMode` (line ~22) and `overlayKind` (line ~40). Keep this doc in
sync when those enums change.

## Views (top-level modes)

| Mode             | Trigger / Source            | Purpose                                        |
| ---------------- | --------------------------- | ---------------------------------------------- |
| `modeExplorer`   | default                     | Three-column resource browser (clusters → resource types → resources). |
| `modeYAML`       | `Enter` on a resource       | Full-screen YAML preview with search and copy. |
| `modeHelp`       | `?`                         | Searchable, filterable keybinding reference.   |
| `modeLogs`       | `l` on a pod / container    | Log viewer with follow, wrap, search, visual selection. |
| `modeDescribe`   | `d` on a resource           | `kubectl describe`-style detail view.          |
| `modeDiff`       | drift / yaml comparisons    | Side-by-side diff (e.g. ArgoCD live vs. desired). |
| `modeExec`       | `x` (exec into container)   | Embedded PTY shell session.                    |
| `modeExplain`    | `:explain <type>`           | `kubectl explain` field tree.                  |
| `modeEventViewer`| Event timeline overlay → drill | Full-screen event viewer with grouping.   |
| `modeKubetris`   | `:kubetris`                 | Easter-egg game.                               |
| `modeCredits`    | `:credits`                  | Scrolling credits screen.                      |

## Fullscreen view variants

These flip a flag while staying inside `modeExplorer`; the explorer's
right pane (or middle, depending on flag) takes the whole screen.

| Flag                   | Trigger                  | What it shows                                |
| ---------------------- | ------------------------ | -------------------------------------------- |
| `fullscreenDashboard`  | `Enter` on `[Dashboard]` | Cluster dashboard (nodes, pods, namespaces, alerts). |
| `fullscreenMonitoring` | `Enter` on `[Monitoring]`| Monitoring dashboard (Prometheus alerts, etc.). |
| `errorLogFullscreen`   | from error notifications | Process stderr / lfk log buffer in full screen. |

## Overlays (modal panels)

Listed in the order they appear in `overlayKind`. Overlays are mutually
exclusive — opening a new one replaces any prior open overlay.

### Selectors

| Overlay                  | Trigger                             | Purpose                                  |
| ------------------------ | ----------------------------------- | ---------------------------------------- |
| `overlayNamespace`       | `n` (or `:namespace`)               | Pick / multi-select namespaces.          |
| `overlayContainerSelect` | `c` in pod log view                 | Pick container when a pod has multiples. |
| `overlayPodSelect`       | `\` in log view                     | Switch to a sibling pod's logs.          |
| `overlayBookmarks`       | `b` (or `:bookmarks`)               | Saved navigation slots, with filter.     |
| `overlayTemplates`       | `T` in resource type list           | Pick a built-in resource template.       |
| `overlayColorscheme`     | `Shift+T`                           | Theme picker (search + preview).         |
| `overlayFilterPreset`    | filter preset key                   | Saved filter expressions.                |
| `overlayLogPodSelect`    | `\` in log fullscreen               | Switch pods within fullscreen log mode.  |
| `overlayLogContainerSelect` | `\` in log fullscreen           | Container picker within log mode.        |
| `overlayCanISubject`     | `:can-i` flow                       | Pick the user/SA to evaluate as.         |
| `overlayCanI`            | `:can-i` flow                       | Display can-i evaluation results.        |
| `overlayExplainSearch`   | `:explain` start                    | Type/field search for kubectl explain.   |
| `overlayFinalizerSearch` | finalizer-edit flow                 | Pick a finalizer to remove.              |
| `overlayColumnToggle`    | column-visibility key               | Show/hide table columns per kind.        |

### Editors / Forms

| Overlay                  | Trigger                            | Purpose                                |
| ------------------------ | ---------------------------------- | -------------------------------------- |
| `overlayScaleInput`      | `s` on workload                    | Replica count input.                   |
| `overlayPortForward`     | `p` on Service / Pod               | Port-forward destination input.        |
| `overlaySecretEditor`    | edit on Secret                     | Inline edit of decoded secret values.  |
| `overlayConfigMapEditor` | edit on ConfigMap                  | Inline edit of CM keys.                |
| `overlayLabelEditor`     | label-edit flow                    | Add / remove labels.                   |
| `overlayBatchLabel`      | bulk label-edit                    | Apply labels to selected items.        |
| `overlayPVCResize`       | resize on PVC                      | New size input.                        |

### Action Menus / Confirmations

| Overlay              | Trigger                       | Purpose                                       |
| -------------------- | ----------------------------- | --------------------------------------------- |
| `overlayAction`      | `a`                           | Resource-kind-specific action menu.           |
| `overlayConfirm`     | delete / drain                | y/n confirmation for reversible actions.      |
| `overlayConfirmType` | force delete / force finalize | Requires typing `DELETE` for destructive ops. |
| `overlayQuitConfirm` | `q`                           | Confirm before exiting lfk.                   |
| `overlayPasteConfirm`| paste into search/filter      | Confirm multi-line paste.                     |
| `overlayAutoSync`    | ArgoCD app                    | Toggle auto-sync settings.                    |
| `overlayRollback`    | `Ctrl+r` on deploy/sts        | Pick revision to roll back to.                |
| `overlayHelmRollback`| Helm release                  | Pick Helm revision to roll back.              |
| `overlayHelmHistory` | Helm release                  | Browse Helm release history.                  |

### Information / Telemetry

| Overlay                  | Trigger                            | Purpose                                |
| ------------------------ | ---------------------------------- | -------------------------------------- |
| `overlayQuotaDashboard`  | `:quota`                           | Per-namespace ResourceQuota usage.     |
| `overlayEventTimeline`   | event timeline key                 | Cluster-wide events grouped by object. |
| `overlayAlerts`          | alert pop-out                      | Active Prometheus alerts.              |
| `overlayNetworkPolicy`   | netpol pop-out                     | Visualize selected NetworkPolicy.      |
| `overlayPodStartup`      | pod startup view                   | Pod init / readiness gantt.            |
| `overlayRBAC`            | RBAC view                          | RBAC subject/role browser.             |
| `overlayBackgroundTasks` | `:tasks`                           | In-flight + recent background tasks.   |

## Adding a new view or overlay

1. Define the new constant in `viewMode` / `overlayKind` (in
   [`internal/app/app.go`](../internal/app/app.go)).
2. Wire the trigger key in `update_keys*.go`.
3. Add the renderer in `internal/ui/`.
4. Update this doc and [`docs/keybindings.md`](./keybindings.md).
5. Surface the binding in the `?` help screen content
   (`internal/ui/help.go`).
