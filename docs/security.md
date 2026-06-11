# Security Dashboard

lfk surfaces cluster security findings in a built-in **Security** sidebar
category and a per-resource **SEC badge**. Findings are aggregated from several
sources, each auto-detected by the operator or CRDs it needs.

## Sources

| Source | Config key | Requires in cluster | Findings |
|---|---|---|---|
| Heuristic | `heuristic` | nothing (built-in) | Pod- and Service-spec hardening issues: privileged, host PID/IPC/network, hostPath + runtime-socket mounts, dangerous capabilities, runAsRoot, allowPrivilegeEscalation, writable root filesystem, seccomp Unconfined, unmasked procMount, unsafe sysctls, hostPort, shared process namespace, plaintext secrets in env, entire Secrets in env (envFrom), default ServiceAccount (+ token automount), missing resource limits, unpinned image tags, leftover ephemeral debug containers, Windows HostProcess containers, Services with externalIPs, bare pods without a controller (reliability) |
| Advisor | `advisor` | nothing (built-in) | Reliability recommendations: namespaces without ResourceQuota/LimitRange, multi-replica workloads without a PodDisruptionBudget or topology spread, drain-blocking or orphaned PDBs, single-replica workloads, missing probes or resource requests, identical liveness/readiness probes, liveness without readiness, downtime rollout strategies, OnDelete update strategies, zero termination grace period, emptyDir without sizeLimit, quotas near their limit, HPAs pinned / at their ceiling / lacking target requests, PDB minAvailable above HPA minimums, PDBs without unhealthyPodEvictionPolicy, manifests pinning replicas under an HPA |
| Trivy | `trivy` | [Trivy Operator](https://github.com/aquasecurity/trivy-operator) (`VulnerabilityReport`, `ConfigAuditReport` CRDs) | Image vulnerabilities + config-audit misconfigurations |
| Kyverno | `kyverno` | Policy Reports API (`PolicyReport`, `ClusterPolicyReport` from `wgpolicyk8s.io/v1alpha2`) | Policy violations |
| Kubescape | `kubescape` | [kubescape-operator](https://github.com/kubescape/kubescape-operator) (`WorkloadConfigurationScan` CRD) | Failed compliance controls |
| Falco | `falco` | [Falco](https://falco.org) DaemonSet + falcosidekick (pod logs / K8s Events) | Runtime security events |
| Gatekeeper | `gatekeeper` | [OPA Gatekeeper](https://open-policy-agent.github.io/gatekeeper/) (Constraint CRDs under `constraints.gatekeeper.sh`) | Constraint audit violations |

**Heuristic and Advisor are always available** — they only need API access, so
the Security category is never empty unless the dashboard is disabled. The
internal ids `trivy-operator` and `policy-report` are also accepted as config
keys (aliases of `trivy` and `kyverno`).

Advisor findings are **reliability recommendations, not security findings**:
they appear in the dashboard under the Advisor source but never color the
per-resource SEC badge. The heuristic's `bare_pod` check is
reliability-categorized the same way and likewise stays off the badge. The source is best-effort under restricted RBAC —
resource types it cannot list (e.g. PDBs for a read-only user) silently skip
their checks instead of failing the source, and the `kube-system`,
`kube-public`, and `kube-node-lease` namespaces are always excluded.

The heuristic `secret_env` check (plaintext credential-looking env vars) is
tunable with `security.heuristic.secret_env_include` / `secret_env_exclude` —
case-insensitive env-var name globs added on top of the built-in keyword and
exemption lists (exclude wins). See
[config-reference.md](config-reference.md).

## Enabling / disabling

The dashboard is **on by default** and adapts to whatever is installed. Use the
`security` config section to turn it off or restrict sources.

### Global

```yaml
security:
  enabled: true          # false turns the dashboard, SEC badge, and all probing off
  sources:
    falco: false         # disable a single source (others stay enabled)
    trivy: false
```

- `enabled: false` removes the Security category, hides the SEC badge, and runs
  no source probes.
- `sources.<name>: false` disables one source; any source omitted from the map
  stays enabled. Disabling **every** source leaves the Security category empty
  (same as `enabled: false`).

### Per cluster

Per-context overrides under `clusters.<name>.security` win over the global
setting — same precedence model as `read_only`.

```yaml
security:
  enabled: true
clusters:
  prod:
    security:
      enabled: false     # off on prod (e.g. noisy kubeconfig credential plugin)
  staging:
    security:
      sources:
        falco: false     # keep the dashboard, drop Falco on staging only
```

| Setting | Wins over |
|---|---|
| `clusters.<ctx>.security.enabled` | global `security.enabled` |
| `clusters.<ctx>.security.sources.<name>` | global `security.sources.<name>` |

## Behavior

- **Lazy probing**: sources are probed the first time you focus the Security
  category in a context, not at cluster open. A cluster you never inspect for
  security makes no security API calls — which on EKS avoids invoking the
  kubeconfig `aws` credential plugin (and its "SSO session expired" stderr).
- **SEC badge**: resource rows (e.g. Pods, Deployments) show a SEC badge once
  findings are loaded. The glyph and color reflect the **highest** severity
  present (`●` critical, `◐` high, `○` medium/low), and the number is the count
  of findings at that severity only — e.g. `●3` means 3 criticals. Lower-severity
  findings are surfaced in the dashboard, not on the badge. Toggle the badge on/off
  with `B` (`security_badge_toggle`). The badge is hidden when no source is
  available or the dashboard is disabled.
- **Background scan**: once a cluster's sources have been detected (on your first
  visit to the Security category), their availability is cached to disk. On every
  later visit to that cluster the findings scan runs automatically in the
  background, so SEC badges populate without navigating to Security. A cluster you
  have never inspected stays fully lazy — no security API calls until you open its
  Security category — so the badge auto-scan never triggers the EKS `aws`
  credential plugin for clusters you don't look at.
- **Cached findings (stale-while-revalidate)**: a clean scan's findings are
  persisted per cluster + namespace (next to the availability cache, under the
  kubectl cache dir). On reopen, SEC badges paint instantly from the last scan
  while a fresh scan revalidates in the background and replaces them. Findings
  older than one hour are not painted (the live scan still runs). Partial scans
  (any source errored) are not cached, so a transient failure never persists an
  undercount.
- **Low priority**: security scans run as a low-priority background task and on a
  dedicated, throttled API client (QPS 10 / burst 20, separate from the
  foreground budget), so a multi-source scan never starves foreground resource
  lists. The foreground client rate is configurable; see
  [API client rate limits](config-reference.md#api-client-rate-limits).
- **Drill-down**: open the Security category, pick a source to list finding
  groups (one per check/CVE), then drill into a group to see affected
  resources. `Enter` on an affected resource jumps to the real object;
  `Backspace` jumps back.

## Ignoring findings

Known/accepted findings can be hidden so the dashboard surfaces only what still
needs attention. Two mechanisms exist, applied together: an **interactive
per-cluster ignore-list** and **declarative config-file patterns**.

### Interactive (action menu)

On a finding group or an affected resource, press `x` to open the action menu.
The available scopes depend on the row:

| Action | Scope |
|---|---|
| Ignore (Group) | Hide the finding across the whole cluster |
| Ignore (Namespace) | Hide the finding for every resource in the selected row's namespace |
| Ignore (This Resource) | Hide the finding for one specific resource |
| Un-ignore | Remove the most specific matching rule (resource → namespace → group) |

| Key | Action |
|---|---|
| `i` | Toggle show/hide ignored findings (only on a security view) |
| `x` | Open the action menu to ignore/un-ignore the selected finding |

The interactive ignore-list is stored per cluster in
`$XDG_STATE_HOME/lfk/security_ignores.yaml` (default `~/.local/state/lfk/`) and
persists across sessions.

### Declarative (config file)

`security.ignore_patterns` in the config file defines glob-based rules applied
at startup — useful for org-wide accepted findings. Each field is a glob (`*`,
`?`); an empty field matches anything. A finding is ignored when every non-empty
field matches it.

```yaml
security:
  ignore_patterns:
    - source: trivy-operator    # source id; "" = any
      group: "CVE-2024-*"       # CVE id / check label / rule name; "" = any
      namespace: kube-system    # "" or "*" = any namespace (hides the whole group)
      cluster: "prod-*"         # kube-context glob; "" = any cluster
      comment: accepted in system namespaces
```

Config patterns are read-only — they cannot be un-ignored from the action menu —
but the `i` toggle still reveals findings they hide. A pattern with every field
empty is ignored (it would otherwise hide everything).
