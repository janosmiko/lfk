// Rule-based RBAC checks: dangerous verb/resource grants in Roles and
// ClusterRoles. Each check emits at most one finding per role object even
// when several rules match.

package rbac

import (
	"fmt"
	"slices"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// ruleCheck describes one dangerous grant to look for. A rule matches when
// its apiGroups, verbs, and resources all intersect the wanted sets. "*" in
// the rule's apiGroups or verbs matches anything, but resources must match
// explicitly — wildcard-resource roles are rbac_wildcard's territory.
type ruleCheck struct {
	id        string
	sev       security.Severity
	title     string
	summary   string // %s = role kind, %s = role name
	verbs     []string
	apiGroups []string
	resources []string
}

var ruleChecks = []ruleCheck{
	{
		id: "rbac_impersonate", sev: security.SeverityHigh,
		title:     "can impersonate identities",
		summary:   "%s %q grants the impersonate verb; holders can act as any allowed user, group, or ServiceAccount.",
		verbs:     []string{"impersonate"},
		apiGroups: []string{"", "authentication.k8s.io"},
		resources: []string{"users", "groups", "serviceaccounts", "uids", "userextras"},
	},
	{
		id: "rbac_bind_escalate", sev: security.SeverityHigh,
		title:     "can bind or escalate roles",
		summary:   "%s %q grants bind/escalate on RBAC objects; holders can grant themselves permissions they do not have.",
		verbs:     []string{"bind", "escalate"},
		apiGroups: []string{"rbac.authorization.k8s.io"},
		resources: []string{"roles", "clusterroles", "rolebindings", "clusterrolebindings"},
	},
	{
		id: "rbac_pod_exec", sev: security.SeverityHigh,
		title:     "can exec into pods",
		summary:   "%s %q grants pods/exec or pods/attach; holders can open a shell in any pod in scope.",
		verbs:     []string{"create"},
		apiGroups: []string{""},
		resources: []string{"pods/exec", "pods/attach"},
	},
	{
		id: "rbac_port_forward", sev: security.SeverityMedium,
		title:     "can port-forward to pods",
		summary:   "%s %q grants pods/portforward; holders can reach any pod's ports, bypassing NetworkPolicies.",
		verbs:     []string{"create"},
		apiGroups: []string{""},
		resources: []string{"pods/portforward"},
	},
	{
		id: "rbac_nodes_proxy", sev: security.SeverityHigh,
		title:     "can access the kubelet API",
		summary:   "%s %q grants nodes/proxy; holders can drive the kubelet API directly, including command execution in any pod on the node.",
		verbs:     []string{"get", "create"},
		apiGroups: []string{""},
		resources: []string{"nodes/proxy"},
	},
	{
		id: "rbac_csr_approval", sev: security.SeverityHigh,
		title:   "can approve certificate requests",
		summary: "%s %q grants update on certificatesigningrequests/approval; holders can mint certificates for arbitrary identities.",
		// patch is included alongside update: the approval subresource
		// accepts PATCH on current servers, and a patch-only grant is
		// approval-capable either way.
		verbs:     []string{"update", "patch"},
		apiGroups: []string{"certificates.k8s.io"},
		resources: []string{"certificatesigningrequests/approval"},
	},
	{
		id: "rbac_webhook_admin", sev: security.SeverityHigh,
		title:     "can modify admission webhooks",
		summary:   "%s %q grants write access to admission webhook configurations; holders can disable or subvert admission control.",
		verbs:     []string{"create", "update", "patch", "delete"},
		apiGroups: []string{"admissionregistration.k8s.io"},
		resources: []string{"validatingwebhookconfigurations", "mutatingwebhookconfigurations"},
	},
}

// ruleMatches reports whether one PolicyRule grants what the check looks
// for. apiGroups and verbs honor a "*" in the rule. Resources must be
// explicit.
func ruleMatches(r *rbacv1.PolicyRule, c *ruleCheck) bool {
	return intersectsOrWildcard(r.APIGroups, c.apiGroups) &&
		intersectsOrWildcard(r.Verbs, c.verbs) &&
		intersects(r.Resources, c.resources)
}

func intersects(have, want []string) bool {
	for _, w := range want {
		if slices.Contains(have, w) {
			return true
		}
	}
	return false
}

func intersectsOrWildcard(have, want []string) bool {
	return slices.Contains(have, "*") || intersects(have, want)
}

// hasWildcard reports whether any rule uses "*" for verbs or resources.
func hasWildcard(rules []rbacv1.PolicyRule) bool {
	for i := range rules {
		if slices.Contains(rules[i].Verbs, "*") || slices.Contains(rules[i].Resources, "*") {
			return true
		}
	}
	return false
}

// readsSecrets reports whether any rule grants get/list/watch (or "*") on
// core-group secrets.
func readsSecrets(rules []rbacv1.PolicyRule) bool {
	c := ruleCheck{
		verbs:     []string{"get", "list", "watch"},
		apiGroups: []string{""},
		resources: []string{"secrets"},
	}
	for i := range rules {
		if ruleMatches(&rules[i], &c) {
			return true
		}
	}
	return false
}

// ruleFindings audits Role and ClusterRole rules. For aggregated
// ClusterRoles (aggregationRule), .Rules is the controller-reconciled union
// of the aggregated roles by the time List returns, so the effective grants
// are always evaluated.
func (d *rbacData) ruleFindings() []security.Finding {
	var out []security.Finding
	if d.rolesOK {
		for i := range d.roles {
			r := &d.roles[i]
			if skipObject(&r.ObjectMeta) {
				continue
			}
			out = append(out, auditRules(r.Namespace, "Role", r.Name, r.Rules)...)
		}
	}
	if d.clusterRolesOK {
		boundNames := d.boundClusterRoles()
		for i := range d.clusterRoles {
			r := &d.clusterRoles[i]
			if skipObject(&r.ObjectMeta) {
				continue
			}
			out = append(out, auditRules("", "ClusterRole", r.Name, r.Rules)...)
			// Cluster-wide secret read is only an exposure once the role is
			// actually bound. An inert ClusterRole is just a definition.
			if d.crbsOK && boundNames[r.Name] && readsSecrets(r.Rules) {
				out = append(out, makeFinding("", "ClusterRole", r.Name, "rbac_secrets_cluster_wide", security.SeverityHigh,
					"cluster-wide secret read",
					fmt.Sprintf("ClusterRole %q grants read access to Secrets in every namespace and is bound by a ClusterRoleBinding; holders can harvest all cluster credentials.", r.Name)))
			}
		}
	}
	return out
}

// auditRules runs the table checks plus the wildcard check over one role's
// rules.
func auditRules(ns, kind, name string, rules []rbacv1.PolicyRule) []security.Finding {
	var out []security.Finding
	for ci := range ruleChecks {
		c := &ruleChecks[ci]
		for ri := range rules {
			if ruleMatches(&rules[ri], c) {
				out = append(out, makeFinding(ns, kind, name, c.id, c.sev, c.title,
					fmt.Sprintf(c.summary, kind, name)))
				break
			}
		}
	}
	if hasWildcard(rules) {
		out = append(out, makeFinding(ns, kind, name, "rbac_wildcard", security.SeverityHigh,
			"wildcard RBAC rule",
			fmt.Sprintf("%s %q uses \"*\" verbs or resources; grants expand silently as new API types appear. Enumerate the verbs and resources actually needed.", kind, name)))
	}
	return out
}

// boundClusterRoles returns the set of ClusterRole names referenced by at
// least one ClusterRoleBinding (including built-in bindings — a custom role
// bound by any binding is live). RoleBindings are deliberately not indexed:
// a RoleBinding referencing a ClusterRole scopes its grant to one namespace,
// which is not a cluster-wide exposure.
func (d *rbacData) boundClusterRoles() map[string]bool {
	bound := make(map[string]bool, len(d.clusterRoleBindings))
	for i := range d.clusterRoleBindings {
		b := &d.clusterRoleBindings[i]
		if b.RoleRef.Kind == "ClusterRole" {
			bound[b.RoleRef.Name] = true
		}
	}
	return bound
}
