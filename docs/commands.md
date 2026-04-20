# Command Bar Reference

Press `:` to open the command bar. Input is classified by the first word:

- `!<cmd>` — shell command
- `:ns`, `:ctx`, `:sort`, … — built-in command
- `:k`, `:kubectl`, `:get`, … — kubectl command
- `:pod`, `:svc`, … — resource jump

`Tab` accepts the highlighted suggestion, `Up`/`Down` cycles, `Enter` runs.

## Built-in commands

| Command | Description |
|---|---|
| `:namespace <ns>…` &nbsp;·&nbsp; `:ns <ns>…` | Switch namespace(s); no args opens the Namespaces list |
| `:context <ctx>` &nbsp;·&nbsp; `:ctx <ctx>` | Switch kube context |
| `:sort <column>` | Sort the current list by column name |
| `:set <option>` | Toggle log viewer option (see below) |
| `:export [yaml\|json]` | Copy selected resource YAML to clipboard |
| `:tasks` | Open background-tasks overlay |
| `:quit` &nbsp;·&nbsp; `:q` &nbsp;·&nbsp; `:q!` | Exit |
| `:nyan` | Toggle Nyan mode |
| `:kubetris` | Play Kubetris |
| `:credits` | Show credits |

### `:sort <column>`

Column names match the table headers (case-sensitive): `Name`, `Namespace`, `Age`, `Status`, `CPU`, `MEM`, `Ready`, `Restarts`, etc. Tiebreaker uses `Name → Namespace → Age`, skipping whichever is primary.

### `:set <option>`

| Option | Effect |
|---|---|
| `wrap` / `nowrap` | Line wrapping |
| `linenumbers` / `nolinenumbers` | Line numbers |
| `timestamps` / `notimestamps` | Timestamps |
| `follow` / `nofollow` | Auto-scroll to tail |

### `:tasks`

Background-tasks overlay. Inside:

- `Tab` — toggle running / completed history
- `` ` `` — also opens the overlay
- `j`/`k`, `Ctrl+D`/`Ctrl+U`, `g`/`G` — scroll
- `Esc` / `q` — close

Repeated completed tasks collapse with a `×N` suffix.

## Shell commands

`:! <cmd>` runs `sh -c <cmd>` with `KUBECONFIG` and `KUBECTL_CONTEXT` pre-set.

```
:! kubectl top nodes
:! helm list -A
```

## kubectl commands

`:k <args>` and `:kubectl <args>` run kubectl with `--context` and `-n` injected automatically. Top-level kubectl subcommands (`:get`, `:describe`, `:logs`, `:exec`, `:apply`, `:edit`, `:patch`, `:scale`, `:rollout`, `:top`, `:port-forward`, …) also work without the `k` prefix.

```
:k get pods -A
:kubectl logs -f deploy/web
:k describe pod nginx-abc
```

Autocomplete offers subcommands, flags, resource types, and namespaces. Typing `:k` or `:kubectl` alone surfaces the subcommand list. Value positions (namespace, resource name, output format) accept fuzzy matches — exact > prefix > substring > subsequence — while command names themselves stay on prefix.

Namespace suggestions come from a per-context cache warmed when the context is opened and refreshed on a 60s TTL. In-app mutations — `:k create ns`, `:k delete ns`, or a template apply — invalidate the cache immediately so new namespaces appear in completions without waiting for the TTL; changes made outside the TUI (CI, cloud console, `kubectl` in another shell) surface on the next refresh. The existing list stays visible during refreshes so completions never blank out.

## Resource jumps

Type a resource name to navigate. Plural, singular, and kubectl abbreviations all work.

| Input | Destination |
|---|---|
| `:pod` / `:pods` / `:po` | Pods |
| `:deploy` / `:deployments` | Deployments |
| `:svc` / `:services` | Services |
| `:ns` / `:namespaces` | Namespaces |
| `:pvc` | PersistentVolumeClaims |
| `:cm` / `:configmaps` | ConfigMaps |
| `:secret` / `:secrets` | Secrets |
| `:node` / `:no` | Nodes |
| `:helm` / `:releases` | Helm releases |
| `:app` / `:argoapps` | ArgoCD Applications |

Any built-in kind, discovered CRD, or abbreviation from `~/.config/lfk/config.yaml` → `search_abbreviations` works. Add positional args to filter by namespace:

```
:pod kube-system
:svc default monitoring
```

## See also

- [keybindings.md](keybindings.md) — all hotkeys
- [config-reference.md](config-reference.md) — config file reference
