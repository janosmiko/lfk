# Manual Test Cases

Manual verification scripts for features that need a real Kubernetes cluster.
Automated tests live next to the code under test (`*_test.go`).

## Crash Investigator

### Setup

```bash
kind create cluster --name crashinv-test
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: crashy
  namespace: default
spec:
  containers:
  - name: app
    image: busybox:1.36
    command: ["sh", "-c", "echo 'about to crash'; sleep 1; exit 1"]
EOF
```

Wait until `kubectl get pod crashy` shows `STATUS=CrashLoopBackOff` and `RESTARTS>=2`.

### Cases

| # | Steps | Expected |
|---|-------|----------|
| 1 | `lfk` → navigate to `default/crashy` → `x` → `I` | Overlay opens on Summary tab; `app` row shows `Waiting`, `RESTARTS=N`, `LAST EXIT=1`, `LAST REASON=Error` |
| 2 | `Tab` (or `2`) | Events tab; recent events include `BackOff` Warning |
| 3 | `Tab` (or `3`) | Logs tab header reads `LOGS · previous · container=app`; body contains `about to crash` |
| 4 | `p` | Header switches to `current`; body usually empty or "no current logs available" |
| 5 | `Tab` (or `4`) | Describe tab shows `Name: crashy`, container metadata |
| 6 | `Shift+R` | Status line shows `Refreshing crash investigation…`; on completion, `RESTARTS` count typically incremented; tab + scroll preserved |
| 7 | `Esc` | Overlay closes; pod list re-renders |
| 8 | Multi-container variant: apply a pod with two containers (one healthy, one crashing); repeat steps 1-3 | Summary aggregates both rows; `c` switches between them; Logs tab follows the active container |
| 9 | Init-container variant: apply a pod with an init container that exits 1; repeat steps 1-3 | Summary shows the init container in the Init Containers sub-table; `c` cycles to it; Logs tab works for the init container |

### Cleanup

```bash
kind delete cluster --name crashinv-test
```

## Sync Wave Timeline (`feat/sync-wave-timeline`)

Prerequisites: a kube-context with at least one ArgoCD Application
managed by argo-cd >= 2.0. Examples below assume `my-app` in `argocd`.

1. Launch lfk: `./bin/lfk`.
2. Switch to the cluster, navigate Applications → highlight `my-app`.
3. Press `x` to open the action menu, then `W`.
   - Expected: fullscreen overlay opens; header shows `Sync Wave
     Timeline: my-app`. If the app has synced before, header shows
     `Last Sync: <Phase> · <age> · revision: <short>`.
4. Verify wave grouping:
   - Resources annotated with `argocd.argoproj.io/sync-wave: 0` appear
     under `wave 0`; those with `: 5` appear under `wave 5`. Buckets are
     in ascending order.
   - Any resource without the annotation in the live cluster also lands
     at `wave 0` — ArgoCD's default sync-wave is 0 when absent.
   - Resources whose live GET fails (e.g. RBAC denial) or whose
     annotation is unparseable land under `wave ?` at the bottom.
4a. Verify phase pipeline is visible end-to-end:
   - All seven standard phases — PreSync, Sync, PostSync, SyncFail,
     PostSyncFail, PreDelete, PostDelete — appear in fixed order.
   - Phases with no resources in the last operation render as a single
     header line ending with ` (none in last operation)` and a `▸` marker.
4b. Verify the overlay box stays a fixed size:
   - Scroll the body down with `j` / `Ctrl+D` / `G`. Scroll is global —
     a single offset applied to the flattened phase blocks, so j/k
     can lift the viewport across phase boundaries (not just within
     a single phase).
   - Expected: the outer rounded-border box does NOT visibly shrink as
     content scrolls past — the body is padded with empty rows so the
     viewport height is fixed.
5. Trigger a sync (`s` from the action menu).
   - Expected: the timeline overlay reopens or re-renders with
     `Live phase: Running` in the header. Auto-refresh ticks every 3s.
6. Cycle phase focus with `Tab` / `Shift+Tab`. The focused phase header
   should be bolded AND the body should auto-scroll so the focused
   phase's header sits at the top of the visible body — without this,
   Tab onto a phase below the fold would change the bold marker but
   leave the viewport at the top.
7. Press `Enter` on a phase: rows under it disappear (collapsed).
   Press `Enter` again: rows return.
8. Press `Esc` or `q`. Overlay closes; prior view is intact.
9. Reopen the overlay during the sync; close it; reopen.
   - Expected: no stale data flashes; auto-refresh ticks continue
     correctly; closing and immediately reopening doesn't double-fetch.

### Two-phase load (immediate overlay open)

10. Open Sync Wave Timeline on a large Application (50+ resources).
    - Expected: the overlay frame opens within ~200ms with the placeholder
      `Loading sync wave timeline…` (before the skeleton lands).
    - Within ~1s the skeleton message arrives: header now shows
      `Sync Wave Timeline: <app>`, all phases render, and every managed
      resource is bucketed at `wave ?`. The header carries an extra
      animated braille spinner + `Loading wave map…` line; the spinner
      glyph rotates ~10x/sec so the operator can confirm the overlay
      hasn't frozen.
    - After the wave fan-out completes (~10–30s on large apps): the
      `Loading wave map…` indicator disappears and resources are
      re-bucketed under their actual wave numbers.
11. Close the overlay (Esc) mid-load while waves are still fetching.
    - Expected: overlay closes cleanly. The in-flight full fetch finishes
      in the background but its message is dropped by the token check;
      no stale data leaks into a future open.
12. Reopen mid-load (close, then immediately reopen before previous
    fetches return).
    - Expected: a new skeleton fetch kicks off; previous fetches' messages
      are ignored. The header transitions placeholder → skeleton+loading
      → full again, just like a fresh open.

## Regression checks

10. With the overlay closed, run an existing flow (Crash Investigator
    on a Pod, Sync on an Application via action menu). All should still
    work — no overlay-state bleed.

## Sync Wave Timeline — two-pane layout

13. Open the overlay on an Application with at least one synced
    resource and at least one empty fail phase.
    - Expected: sidebar shows all 7 phases; empty fail/delete phases
      collapsed by default with `(none)` annotation; first non-empty
      phase is selected.

14. Press `j` in the sidebar.
    - Expected: cursor moves to the next phase; body re-renders showing
      that phase's content (or the placeholder if empty/collapsed).

15. Press `Tab`.
    - Expected: focus shifts to body. Sidebar's cursor row drops to dim
      (ParentHighlightStyle); body's cursor row promotes to bright.

16. With body focus, press `j` and `k`.
    - Expected: cursor moves through wave headers and resources.

17. With body focus on a wave header, press `Enter`.
    - Expected: wave collapses (`▸ wave N (M items)`); resources hidden.
      Press `Enter` again to expand.

18. With sidebar focus, press `Enter` on a non-empty phase.
    - Expected: phase collapses; body shows placeholder
      `<phase> collapsed — Enter to expand`.

19. Resize terminal to <50 cols wide.
    - Expected: sidebar disappears; body uses full width; Tab does
      nothing.

20. Trigger a sync (s on Application). Reopen Sync Wave Timeline.
    - Expected: spinner animates in header during wave-annotation fetch;
      cursor + scroll preserved across the 3s refresh ticks.
