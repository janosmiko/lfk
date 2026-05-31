# Security Dashboard

lfk surfaces cluster security findings in a built-in **Security** sidebar
category and a per-resource **SEC badge**. Findings are aggregated from several
sources, each auto-detected by the operator or CRDs it needs.

## Sources

| Source | Config key | Requires in cluster | Findings |
|---|---|---|---|
| Heuristic | `heuristic` | nothing (built-in) | Pod-spec hardening issues: privileged, hostPath, host PID/IPC/network, default ServiceAccount, writable root filesystem, runAsRoot, allowPrivilegeEscalation |
| Trivy | `trivy` | [Trivy Operator](https://github.com/aquasecurity/trivy-operator) (`VulnerabilityReport`, `ConfigAuditReport` CRDs) | Image vulnerabilities + config-audit misconfigurations |
| Kyverno | `kyverno` | Policy Reports API (`PolicyReport`, `ClusterPolicyReport` from `wgpolicyk8s.io/v1alpha2`) | Policy violations |
| Kubescape | `kubescape` | [kubescape-operator](https://github.com/kubescape/kubescape-operator) (`WorkloadConfigurationScan` CRD) | Failed compliance controls |
| Falco | `falco` | [Falco](https://falco.org) DaemonSet + falcosidekick (pod logs / K8s Events) | Runtime security events |
| Gatekeeper | `gatekeeper` | [OPA Gatekeeper](https://open-policy-agent.github.io/gatekeeper/) (Constraint CRDs under `constraints.gatekeeper.sh`) | Constraint audit violations |

**Heuristic is always available** — it only needs API access to list Pods, so
the Security category is never empty unless the dashboard is disabled. The
internal ids `trivy-operator` and `policy-report` are also accepted as config
keys (aliases of `trivy` and `kyverno`).

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
- **SEC badge**: resource rows (e.g. Pods, Deployments) show a SEC badge with a
  severity breakdown once findings are loaded. The badge is hidden when no
  source is available or the dashboard is disabled.
- **Drill-down**: open the Security category, pick a source to list finding
  groups (one per check/CVE), then drill into a group to see affected
  resources. `Enter` on an affected resource jumps to the real object;
  `Backspace` jumps back.

## Ignoring findings

Each cluster has an ignore-list so known/accepted findings can be hidden.

| Key | Action |
|---|---|
| `Ctrl+I` | Toggle show/hide ignored findings on the active security view |

The ignore-list is stored per cluster and persists across sessions.
