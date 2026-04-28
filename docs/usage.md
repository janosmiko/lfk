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

> **Note:** macOS Terminal.app does not support shift+click text selection while
> mouse capture is active. Use `--no-mouse` or switch to a terminal that handles
> this correctly (iTerm2, Kitty, Alacritty, WezTerm, Ghostty).

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
  across context switches. If `--read-only` was passed on the command
  line, toggling off lets you mutate within the current context, but
  switching contexts re-applies the CLI flag.

### CLI flag

```sh
lfk --read-only
lfk --read-only --context prod   # combine with explicit context
```

### Discovery

The cluster picker hint bar advertises `Ctrl+R toggle RO` so users
can find the row toggle without reading docs.

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
is warm. On busy clusters with many CRDs this is a measurable launch-time
win and removes redundant load on the API server.

- **Override location:** Set `KUBECACHEDIR` (same env var `kubectl`
  honors) to relocate `<KUBECACHEDIR>/discovery/...` and
  `<KUBECACHEDIR>/http/...`.
- **Force refresh:** Press `R` (Shift+r) at the resource types level to
  invalidate the cache and re-run discovery — newly installed or removed
  CRDs show up immediately without restarting `lfk`.
- **TTL:** 5 minutes. Stale entries are refetched automatically on the
  next discovery request.

## Secret Lazy Loading

On clusters with many Helm releases or large TLS secrets, listing the
Secrets resource type can transfer tens of megabytes. Enable lazy loading
to fetch only metadata for the list and defer decoded values to hover:

- **Config file:** Add `secret_lazy_loading: true` to `~/.config/lfk/config.yaml`

Trade-off: the Type column is dropped from the list (metadata-only fetch
doesn't include it) and there's a small per-hover fetch for decoded values
(cached thereafter). See [Configuration Reference](config-reference.md#secret-lazy-loading)
for details.
