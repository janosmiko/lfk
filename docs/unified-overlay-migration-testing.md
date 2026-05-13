# Unified Overlay Components — Manual Testing Guide

Companion to PR #231. This document is transitional — delete after the PR merges and the new components have settled.

The PR consolidated 17 bespoke overlay renderers into three reusable components (`OverlayList`, `OverlayConfirm`, `OverlayInput`). Component-level unit tests are green; this guide covers behavior that can only be verified end-to-end.

## How to test

```bash
# Build the branch.
git switch feat/unified-overlay-components
make build

# Or run directly.
go run ./...
```

Connect to any cluster (kind / k3d / minikube / real) with enough resources to exercise filtering and scrolling.

## Visual standardisations (apply across migrated overlays)

These are intentional uniformity changes — flag if any of them break the UX in a specific overlay:

| Aspect | Before | After |
|---|---|---|
| Filter prompt | `/ <text>█` (most), `filter> <text>█` (Bookmark/Template), `/ to filter` placeholder | `filter: <text>█` everywhere; no placeholder when filter is empty and inactive |
| Cursor highlight | `> ` prefix (Template, CanISubject, ExplainSearch) OR inverse-video full row (others) | Inverse-video full row everywhere |
| Status text style | Per-status colour (Running=green, Pending=yellow, Failed=red) in PodSelect | Dim text in the Description column |
| Action row separator | `[s] Name - Description` | `[s] Name  Description` (two-space gap, no dash) |
| Active markers (Namespace) | `✓` for all/selected, `*` for current | `✓` for all three |
| Active markers (LogContainerSelect, ColumnToggle) | `✓ ` prefix | `✓ ` prefix (unchanged) but the *cursor* row no longer has it baked in — cursor highlight covers the whole row |
| Visible-column highlight (ColumnToggle) | Visible columns rendered in OverlayFilterStyle (highlight colour) | Visible columns get the active-marker ✓ but their name uses normal style |

---

## Migrated overlays

### Wave B — Confirm dialogs

#### 1. Confirm Delete (`overlayConfirm`)

- **How to open**: Press `d` on any resource that supports delete (e.g. a Pod). The "Confirm Delete: pod-name?" dialog opens.
- **Expect unchanged**:
  - Title reads "Confirm Delete".
  - Body reads "Delete <name>?" in warning colour.
  - `y` confirms, `Enter` confirms, `Esc` / `n` cancels.
  - No inline `[y] yes [n] no` hint inside the box (hint bar carries the keys).
- **Watch for**:
  - Box width still 50 cells (no visible reflow).
- [ ] Verified

#### 2. Confirm Type-to-Delete (`overlayConfirmType`)

- **How to open**: Trigger a force-delete (e.g. `Ctrl+D` on a stuck Pod, or force-finalize via the finalizers overlay).
- **Expect unchanged**:
  - Title reads "Confirm Force Delete" (or similar).
  - Warning-coloured question line.
  - `Type DELETE to confirm: <input>` row with a dim `_` placeholder when input is empty.
  - Typed input renders in filter colour; only "DELETE" exactly enables Enter.
- [ ] Verified

#### 3. Quit Confirm (`overlayQuitConfirm`)

- **How to open**: On the last tab, press `Ctrl+C`.
- **Expect unchanged**:
  - 32 × 5-cell box; the text "Quit lfk?" sits on the middle row, centered horizontally.
  - `y` / `Enter` quits, `Esc` / `n` cancels.
- **Watch for**:
  - Title is **not** offset (still vertically centered, not stuck to the top row).
- [ ] Verified

#### 4. Paste Confirm (`overlayPasteConfirm`)

- **How to open**: Paste multi-line text into any input field (e.g. command bar, filter input).
- **Expect unchanged**:
  - Title "Paste".
  - Two body lines: `Paste contains N lines.` and `Flatten and paste?`.
  - No inline y/n hints.
- [ ] Verified

#### 5. Local-cluster delete confirm

- **How to open**: From the local-cluster wizard (`:lc` or similar), open the delete confirmation for an existing cluster.
- **Expect unchanged**:
  - Title "Confirm Delete Local Cluster".
  - Warning text describing what the deletion does.
  - `Type DELETE to confirm: <input>` row.
  - Wraps cleanly inside the wizard's outer box (uses `PadToHeight`).
- [ ] Verified

---

### Wave C — Single-input forms

#### 6. Scale (`overlayScaleInput`)

- **How to open**: Press `s` on a Deployment / StatefulSet / ReplicaSet.
- **Expect unchanged**:
  - Title "Scale Deployment".
  - One row: `Replicas: <input or "_">`.
  - Numeric input only; Enter applies, Esc cancels.
- [ ] Verified

#### 7. PVC Resize (`overlayPVCResize`)

- **How to open**: Action menu on a PVC → Resize.
- **Expect unchanged**:
  - Title "Resize PVC".
  - Dim `Current: <size>` line above the input.
  - `New size: <input>` row with `e.g. 10Gi` placeholder when empty.
- [ ] Verified

#### 8. Batch Label/Annotation (`overlayBatchLabel`)

- **How to open**: `:add-labels` (or remove / annotations equivalents).
- **Expect unchanged**:
  - Title reflects mode + action: "Add Labels", "Remove Annotations", etc.
  - Prompt line "Enter key=value:" (add) or "Enter key to remove:" (remove).
  - Input row below with a trailing `█` cursor block.
- **Watch for**:
  - Cursor block (`█`) appended to the right of the input value.
- [ ] Verified

#### 9. Port Forward (`overlayPortForward`)

- **How to open**: Action menu → Port Forward on a Service or Pod.
- **Expect unchanged**:
  - Title "Port Forward" + dim subtitle showing the resource name.
  - **If discovered ports exist**: a "Available ports:" candidate list with `<port> (name) [protocol]` entries; cursor (`OverlaySelectedStyle`) on the highlighted row.
  - **When a candidate is selected**: two input rows — `Remote port: <selected>` (read-only) and `Local port:  <input or "(random)">`.
  - **When no candidate is selected** (manual mode): single row `Port mapping: <input or "local:remote">`.
- **Watch for**:
  - Non-TCP protocols show `[UDP]` etc. in the candidate label.
- [ ] Verified

---

### Wave A1 — Plain single-column lists

#### 10. Action Menu (`overlayAction`)

- **How to open**: Press `a` on any selected resource.
- **Expect unchanged**:
  - Title "Actions".
  - Per-row format: `  [<key>] <Name>  <Description>` — verb key in brackets, then name, then dim description.
  - **Adaptive width**: long descriptions (e.g. Karpenter Disrupt, Knative Activate) do not wrap; box grows to fit, capped at terminal width − 10.
  - Short menus stay at the historical 70-cell floor.
- **Watch for**:
  - No row wraps to a second line on any kind (test Pods, Deployments, Nodes, Karpenter NodePool/NodeClaim, Knative Service, Helm releases).
- [ ] Verified

#### 11. Container Select (`overlayContainerSelect`)

- **How to open**: Exec / log into a Pod that has multiple containers.
- **Expect unchanged**:
  - Title "Select Container".
  - Per-row format: `  <name>  (<category>)  <status>` — non-default category only.
  - Cursor highlight covers the whole row.
- **Watch for**:
  - InitContainers labelled with `(InitContainers)` category.
- [ ] Verified

#### 12. Filter Preset (`overlayFilterPreset`)

- **How to open**: Press `.` on any resource list (Pods, Deployments, PVCs, etc.).
- **Expect unchanged**:
  - Title "Quick Filters".
  - Per-row: `[<key>] <Name>  <Description>` with the active preset showing ✓.
  - Adaptive width: PVC's `Not Bound` description fits without wrapping; long custom preset descriptions grow the box.
  - Empty-state message: "No filter presets available".
- **Watch for**:
  - The new `Not Running` (x) preset on Pods / Workloads / Jobs and `Not Bound` (x) on PVCs render correctly with the active marker when applied.
- [ ] Verified

---

### Wave A2 — Filter-enabled lists

#### 13. Pod Select (log viewer pod switcher) (`overlayPodSelect`, `overlayLogPodSelect`)

- **How to open**: From the log viewer, press the pod-switcher key (typically the same as the pod overlay trigger inside logs).
- **Expect unchanged**:
  - Title "Select Pod".
  - `/` enters filter mode → `filter: <text>█` shows; `Esc` exits filter mode but keeps the filter text visible as `filter: <text>` (no cursor block).
  - Cursor navigation + Enter to switch.
- **Visual change**:
  - Pod status (Running / Pending / etc.) renders in **dim** colour, not the per-status colour palette.
- [ ] Verified

#### 14. CanI Subject (`overlayCanISubject`)

- **How to open**: Open the Can-I overlay (`:cani`) → press the subject-switcher key.
- **Expect unchanged**:
  - Title "Select Subject".
  - Filter via `/`; Enter selects.
  - Loading / no-match dim messages still appear.
- [ ] Verified

#### 15. Bookmarks (`overlayBookmarks`)

- **How to open**: Press `'` (bookmark recall) or `:bookmarks`.
- **Expect unchanged**:
  - Title "Bookmarks" with the chip `[LOAD NAMESPACE]` appended when load-namespace mode is on (toggle via the standard key).
  - Slot letter (`a`, `A`, `1`, etc.) shown as `[<slot>]` prefix.
  - Filter mode opens with `/` → renders as `filter: <text>█`.
- **Visual changes**:
  - Filter prompt is `filter: …` instead of `filter> …`.
  - Per-row format collapses to the unified `[<slot>] <Name>` shape — no separate namespace bracket (e.g. `My Pods [default]` becomes just `My Pods`).
  - **Confirm with user**: if the namespace bracket was useful, flag and we restore it via Description.
- **Watch for**:
  - Empty state shows "No bookmarks yet — press m<key> in the explorer to set a mark".
- [ ] Verified — **may need follow-up on missing namespace bracket**

#### 16. Templates (`overlayTemplates`)

- **How to open**: `:template`.
- **Expect unchanged**:
  - Title "Create from Template".
  - Category renders as `[<Category>]` Status badge before the name.
  - Filter mode via `/`.
  - Scroll respects overlay height (no overflow into chrome).
- **Visual changes**:
  - Cursor row no longer prefixed with `> `; full-row highlight instead.
  - Category bracket no longer renders in dim — it inherits the row style.
- [ ] Verified

---

### Wave A3 — Multi-select lists

#### 17. Log Container Select (`overlayLogContainerSelect`)

- **How to open**: From the log viewer, press the container-filter key.
- **Expect unchanged**:
  - Title "Filter Containers".
  - Virtual "all" row at top: ✓ when no per-container selection is active.
  - Per-container rows: ✓ when the container is currently in the selection.
  - Filter via `/`.
  - When the log viewer allows pod-switching, the footer hint reads `tab to switch pod`.
- **Visual change**:
  - The "currently in selection" indicator is now `✓ ` (was already `✓ ` — no visible change, just confirming).
- [ ] Verified

#### 18. Column Toggle (`overlayColumnToggle`)

- **How to open**: Press the column-toggle key in any resource list (typically `:columns` or a dedicated overlay).
- **Expect unchanged**:
  - Title "Column Visibility".
  - Visible columns get a ✓ marker.
  - Filter via `/`.
- **Visual change**:
  - Visible column rows no longer render in `OverlayFilterStyle` (highlight colour) — the visibility cue is now the ✓ marker only. If this loses readability, flag and we restore the colour via a per-item style hook.
- [ ] Verified — **may need follow-up on visible-column colouring**

---

### Wave A4 — Namespace overlay

#### 19. Namespace (`overlayNamespace`)

- **How to open**: Press `n`.
- **Expect unchanged**:
  - Title "Select Namespace".
  - Filter via `/` (with the standardised `filter: …` prompt).
  - Virtual "All Namespaces" row at top.
  - Multi-select pinning still works (`Space` to toggle, `Enter` to apply).
  - Mouse-click resolution: clicking a row still selects the correct namespace even when scrolled.
- **Visual change**:
  - The current-namespace marker (`*`) is gone — current namespace renders with the same ✓ as selected / all-on rows.
- **Watch for**:
  - In a workspace with > 20 namespaces, scrolling and mouse-click still land on the right row (the scroll state syncs back to `ui.SetOverlayNsScroll`).
- [ ] Verified — **mouse-click parity is the highest-risk regression here**

---

## Type-E overlays — should be visually unchanged

These were intentionally left bespoke. Verify they render identically to `main`.

| Overlay | How to open |
|---|---|
| Colorscheme picker | `:scheme` or `T` |
| Cluster colour picker | `:cluster-color` |
| Explain search | `:explain <kind>` and search |
| Finalizer search | `:finalizers` |
| RBAC overlay | `:rbac` |
| Quota dashboard | `:quota` |
| Pod startup | Pod actions → Pod Startup |
| Network policy | `:netpol` on a NetworkPolicy |
| Error log | `:errors` |
| Event timeline | Event-related overlay |
| Crash investigator | Pod actions → Investigate |
| Rightsizing | Workload actions → Rightsize |
| Helm history / rollback / autosync | Helm release actions |
| Traffic capture | Pod actions → Capture traffic |
| Secret / ConfigMap / Label editors | Edit actions |
| Local-cluster wizard (except the delete-confirm sub-screen — that **is** migrated) | `:lc` |
| Orphans overlay | `:orphans <kind>` |

- [ ] All Type-E overlays render unchanged

---

## Cross-cutting regression watch

Things that aren't overlay-specific but the migration could plausibly break:

- [ ] **Hint bar contents** — the bottom hint bar still renders the right keymap for each overlay (the migrated renderers no longer carry inline hints; the hint bar is the only source).
- [ ] **Overlay dimming** — pressing `?` for help while another overlay is open still dims the background correctly.
- [ ] **Layered overlays** — opening the namespace selector from inside the RBAC overlay (or similar nested flows) draws both correctly.
- [ ] **No-color mode** (`--no-color` / `no_color: true` in config) — overlays render readably without ANSI styling.
- [ ] **Theme switching** — overlays inherit colours from the active theme.
- [ ] **Resizing** — shrink / grow the terminal while an overlay is open; the box adapts (or stays put within reasonable bounds).
- [ ] **Mouse clicks** — clicking rows in any migrated overlay lands on the correct item, especially when scrolled.
- [ ] **Filter typing** — multi-byte / wide glyphs (CJK, emoji) in the filter input don't push the cursor block off the row.

---

## Known visual changes worth a second look

Listed in order of "most likely to be objected to":

1. **Namespace `*` current marker** removed (collapsed into ✓).
2. **PodSelect status colours** lost — Running/Pending/Failed all render dim.
3. **Template `> ` cursor prefix** gone — full-row highlight only.
4. **ColumnToggle visible-column highlight** dropped — only the ✓ marker indicates visibility.
5. **Bookmark `[namespace]` bracket** dropped per-row.
6. **Filter prompt** standardised to `filter: …` (previously varied per overlay).

Any of these is a one- or two-line fix if you want them back — flag the specific overlay and I'll restore.

---

## After testing

- If everything passes: approve PR #231, merge.
- If something regresses: comment on the PR (or open follow-up issues) referencing the section above. Most regressions are fixable inside `OverlayList` config flags or in the per-overlay helper in `internal/app/view_overlay_lists.go`.
- Delete this file (`docs/unified-overlay-migration-testing.md`) once the PR is merged and the new components have shipped a stable release.
