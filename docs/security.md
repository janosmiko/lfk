# Security Dashboard

lfk surfaces cluster security findings in a built-in **Security** sidebar
category and a per-resource **SEC badge**. Findings are aggregated from several
sources, each auto-detected by the operator or CRDs it needs.

## Sources

| Source | Config key | Requires in cluster | Findings |
|---|---|---|---|
| Heuristic | `heuristic` | nothing (built-in) | Pod- and Service-spec hardening issues: privileged, host PID/IPC/network, hostPath + runtime-socket mounts, dangerous capabilities, runAsRoot, allowPrivilegeEscalation, writable root filesystem, seccomp Unconfined, unmasked procMount, unsafe sysctls, hostPort, shared process namespace, plaintext secrets in env, entire Secrets in env (envFrom), default ServiceAccount (+ token automount), missing resource limits, unpinned image tags, leftover ephemeral debug containers, Windows HostProcess containers, Services with externalIPs, namespaces without Pod Security enforcement labels or NetworkPolicies, credential-looking ConfigMap keys, Ingresses without TLS or with catch-all hosts, legacy ServiceAccount token Secrets, expired/expiring TLS certificates, bare pods without a controller (reliability), pods referencing missing ConfigMaps/Secrets (reliability) |
| Advisor | `advisor` | nothing (built-in) | Reliability recommendations: namespaces without ResourceQuota/LimitRange, multi-replica workloads without a PodDisruptionBudget or topology spread, drain-blocking or orphaned PDBs, single-replica workloads, missing probes or resource requests, identical liveness/readiness probes, liveness without readiness, downtime rollout strategies, OnDelete update strategies, zero termination grace period, emptyDir without sizeLimit, quotas near their limit, HPAs pinned / at their ceiling / lacking target requests, PDB minAvailable above HPA minimums, PDBs without unhealthyPodEvictionPolicy, manifests pinning replicas under an HPA, StatefulSets with missing or non-headless governing Services, Services with zero or one ready endpoint, suspended CronJobs, standalone Jobs without TTL, StatefulSet volumes on non-expandable StorageClasses |
| RBAC | `rbac` | nothing (built-in) | Privilege-escalation paths in Roles/ClusterRoles and their bindings: wildcard rules, impersonation, bind/escalate verbs, pods/exec + attach + port-forward grants, kubelet API (nodes/proxy), CSR approval, admission-webhook write access, cluster-wide secret reads, and bindings to anonymous users, system:masters, cluster-admin, or the default ServiceAccount. Kubernetes built-ins (bootstrap-labeled or system:-prefixed) are excluded — note this also skips user-created objects deliberately named `system:*`, so the source audits misconfigurations, not active evasion |
| Trivy | `trivy` | [Trivy Operator](https://github.com/aquasecurity/trivy-operator) (`VulnerabilityReport`, `ConfigAuditReport` CRDs) | Image vulnerabilities + config-audit misconfigurations |
| Kyverno | `kyverno` | Policy Reports API (`PolicyReport`, `ClusterPolicyReport` from `wgpolicyk8s.io/v1alpha2`) | Policy violations |
| Kubescape | `kubescape` | [kubescape-operator](https://github.com/kubescape/kubescape-operator) (`WorkloadConfigurationScan` CRD) | Failed compliance controls |
| Falco | `falco` | [Falco](https://falco.org) DaemonSet + falcosidekick (pod logs / K8s Events) | Runtime security events |
| Gatekeeper | `gatekeeper` | [OPA Gatekeeper](https://open-policy-agent.github.io/gatekeeper/) (Constraint CRDs under `constraints.gatekeeper.sh`) | Constraint audit violations |

**Heuristic, Advisor, and RBAC are always available** — they only need API
access, so the Security category is never empty unless the dashboard is
disabled. The
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
exemption lists (exclude wins); the same patterns also drive the
`configmap_secret_keys` check. The Secret-listing checks
(`legacy_sa_token_secret`, `tls_secret_expiry`, and Secret-reference
verification) can be turned off with `security.heuristic.scan_secrets: false`
if the source should never read Secret objects. See
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
- **Per-resource findings**: the `Security Findings` action (`x` on a resource
  row) opens a finding-group list filtered to that resource **and its owners**
  across all sources — the same set the SEC badge counts (e.g. a Pod row
  includes its Deployment's trivy CVEs). Rows carry a `Source` column;
  `Enter` drills into a group's affected resources as usual, and `Backspace`
  jumps back to the originating list.

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

#### Label matching

The optional `labels` field matches the target resource's Kubernetes labels —
the right tool for silencing whole classes of infrastructure pods (CNI, CSI,
storage) without naming each one. Each entry is a label key mapped to a glob
value (`*`, `?`); **all** entries must match (AND). `labels` combines with the
other fields, so you can still scope by source, namespace, or cluster.

Semantics:

- **Resource-scoped**: a label pattern hides matching resources within a group,
  never the whole group (labels vary per resource).
- **Coverage**: the always-on heuristic source stamps every pod's labels, and
  those propagate to other sources' findings on the **same pod**. Findings a
  source attaches to a workload instead of the pod (e.g. most Trivy image CVEs,
  keyed by Deployment/DaemonSet) have their labels resolved from the live
  object — but only when at least one `labels` pattern is configured, and only
  for the standard workload kinds (Pod, Deployment, ReplicaSet, StatefulSet,
  DaemonSet, Job, CronJob). The lookups run on the throttled security client and
  are capped per scan; other kinds (and CRDs) resolve to no labels.
- A resource with no resolvable labels is never hidden by a label pattern.
- **Cached badges**: labels are not persisted to the on-disk findings cache, so
  on reopen the cached badges paint before the live scan re-stamps labels — a
  label-ignored resource may flash its badge briefly until the background scan
  completes. The other ignore fields apply immediately.

Examples — exclude common infrastructure so only findings that might matter
remain:

```yaml
security:
  ignore_patterns:
    - labels:
        k8s-app: cilium               # Cilium CNI pods
      comment: CNI managed by the platform team
    - labels:
        app.kubernetes.io/name: "longhorn-*"   # Longhorn storage pods
      comment: storage accepted as-is
    - labels:
        app: csi-driver               # any CSI driver pod
      namespace: kube-system          # only in kube-system
      comment: CSI drivers
```

> Label ignores hide privileged host-level pods, which is usually the intent for
> CNI/CSI. Add them deliberately — a compromised infrastructure pod then
> produces no finding. They are opt-in for exactly this reason; lfk ships no
> default ignores.
