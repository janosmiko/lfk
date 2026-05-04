package k8s

import (
	"context"
	"fmt"
	"slices"
	"sort"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// WhoCanSubject is one row in the Reverse-RBAC result table: a subject
// granted the queried (verb, group, resource) by some (Cluster)RoleBinding
// that ultimately resolves to a (Cluster)Role with a matching rule.
//
// Multiple bindings can grant the same subject — each grant is emitted
// as its own row so the user can audit every path; the renderer
// shouldn't dedupe.
type WhoCanSubject struct {
	// Kind is "User" / "Group" / "ServiceAccount", straight from
	// rbacv1.Subject.Kind.
	Kind string
	// Name is the subject identifier (user@example.com, system:masters,
	// ci-deployer).
	Name string
	// Namespace is set only for ServiceAccount subjects (where it's
	// part of the SA's identity, not the binding's namespace). Users
	// and Groups have an empty Namespace.
	Namespace string
	// Via captures the RBAC chain that grants the access:
	// "ClusterRoleBinding/<name> → ClusterRole/<name>" or
	// "RoleBinding/<ns>/<name> → Role/<name>". Format is stable so
	// future audit-trail tooling can parse it.
	Via string
}

// WhoCan returns every subject that has permission to perform `verb` on
// (group, resource) in the requested namespace scope.
//
// Scope rules (matching kubectl-who-can):
//   - namespace == "" → all namespaces are in scope; every RoleBinding
//     in the cluster plus every ClusterRoleBinding is examined.
//   - namespace != "" → ClusterRoleBindings still count (cluster-wide
//     grants always apply), but RoleBindings outside `namespace` are
//     excluded since they can't grant access to the requested scope.
//
// This is a pure RBAC scan — no authorization check (SAR/SSAR) is
// issued. The result reflects the *granted* permission per the
// configured RBAC objects, ignoring webhooks or other authorizer
// plugins that might further restrict access at evaluation time.
func (c *Client) WhoCan(ctx context.Context, contextName, namespace, group, resource, verb string) ([]WhoCanSubject, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}
	clusterRoles, err := loadClusterRoles(ctx, cs)
	if err != nil {
		return nil, err
	}
	roles, err := loadRoles(ctx, cs, namespace)
	if err != nil {
		return nil, err
	}
	roleBindings, err := loadRoleBindings(ctx, cs, namespace)
	if err != nil {
		return nil, err
	}
	clusterRoleBindings, err := cs.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing ClusterRoleBindings: %w", err)
	}

	out := make([]WhoCanSubject, 0)
	for _, crb := range clusterRoleBindings.Items {
		role, ok := lookupRoleForBinding(crb.RoleRef, "", clusterRoles, roles)
		if !ok {
			continue // dangling RoleRef; treat as no grant
		}
		if !ruleSetMatches(role.rules, verb, group, resource) {
			continue
		}
		via := fmt.Sprintf("ClusterRoleBinding/%s → %s/%s",
			crb.Name, role.kind, role.name)
		for _, s := range crb.Subjects {
			out = append(out, subjectFromBinding(s, via))
		}
	}
	for _, rb := range roleBindings.Items {
		role, ok := lookupRoleForBinding(rb.RoleRef, rb.Namespace, clusterRoles, roles)
		if !ok {
			continue
		}
		if !ruleSetMatches(role.rules, verb, group, resource) {
			continue
		}
		via := fmt.Sprintf("RoleBinding/%s/%s → %s/%s",
			rb.Namespace, rb.Name, role.kind, role.name)
		for _, s := range rb.Subjects {
			out = append(out, subjectFromBinding(s, via))
		}
	}
	// Sort by Name primarily so the picker reads alphabetically;
	// fallback keys keep duplicate-name rows (same subject granted via
	// multiple bindings) deterministic between renders.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Via < out[j].Via
	})
	return out, nil
}

// roleEntry collapses Role and ClusterRole into a uniform shape so the
// matching loop above can treat them identically.
type roleEntry struct {
	kind  string // "Role" or "ClusterRole"
	name  string
	rules []rbacv1.PolicyRule
}

// loadClusterRoles fetches every ClusterRole in the cluster and indexes
// them by name. Used to resolve RoleRef targets of kind ClusterRole
// (from both ClusterRoleBindings and RoleBindings).
func loadClusterRoles(ctx context.Context, cs kubernetes.Interface) (map[string]roleEntry, error) {
	list, err := cs.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing ClusterRoles: %w", err)
	}
	out := make(map[string]roleEntry, len(list.Items))
	for _, cr := range list.Items {
		out[cr.Name] = roleEntry{kind: "ClusterRole", name: cr.Name, rules: cr.Rules}
	}
	return out, nil
}

// loadRoles fetches Roles. When namespace is empty it walks every
// namespace; otherwise it scopes to the one in question. Indexed by
// "<namespace>/<name>" because Role names aren't unique cluster-wide.
func loadRoles(ctx context.Context, cs kubernetes.Interface, namespace string) (map[string]roleEntry, error) {
	list, err := cs.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing Roles: %w", err)
	}
	out := make(map[string]roleEntry, len(list.Items))
	for _, r := range list.Items {
		key := r.Namespace + "/" + r.Name
		out[key] = roleEntry{kind: "Role", name: r.Name, rules: r.Rules}
	}
	return out, nil
}

// loadRoleBindings honors the same scope contract as WhoCan itself.
func loadRoleBindings(ctx context.Context, cs kubernetes.Interface, namespace string) (*rbacv1.RoleBindingList, error) {
	list, err := cs.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing RoleBindings: %w", err)
	}
	return list, nil
}

// lookupRoleForBinding resolves a RoleRef into the underlying role's
// rules. RoleBindings can reference either a Role (in their own
// namespace) or a ClusterRole; ClusterRoleBindings always reference
// a ClusterRole. Returns false when the referenced role isn't loaded
// (orphan ref, RBAC drift) so the caller skips silently — kubectl
// does the same.
func lookupRoleForBinding(ref rbacv1.RoleRef, bindingNamespace string, clusterRoles, roles map[string]roleEntry) (roleEntry, bool) {
	switch ref.Kind {
	case "ClusterRole":
		role, ok := clusterRoles[ref.Name]
		return role, ok
	case "Role":
		role, ok := roles[bindingNamespace+"/"+ref.Name]
		return role, ok
	}
	return roleEntry{}, false
}

// subjectFromBinding converts an rbacv1.Subject into our flat result
// row. ServiceAccount kept its namespace (part of identity); other
// kinds normalise namespace to empty so the table doesn't show
// misleading values.
func subjectFromBinding(s rbacv1.Subject, via string) WhoCanSubject {
	out := WhoCanSubject{Kind: s.Kind, Name: s.Name, Via: via}
	if s.Kind == "ServiceAccount" {
		out.Namespace = s.Namespace
	}
	return out
}

// ruleSetMatches returns true when at least one rule in the set grants
// the (verb, group, resource) tuple. Skips:
//   - nonResourceURLs rules (don't grant resource permissions)
//   - resourceNames-scoped rules (restrict to named objects; the picker
//     does not model object names, so reporting these as generic access
//     would over-report permissions).
func ruleSetMatches(rules []rbacv1.PolicyRule, verb, group, resource string) bool {
	for _, r := range rules {
		if len(r.Resources) == 0 && len(r.NonResourceURLs) > 0 {
			continue
		}
		if len(r.ResourceNames) > 0 {
			continue
		}
		if !verbMatches(r.Verbs, verb) {
			continue
		}
		if !groupMatches(r.APIGroups, group) {
			continue
		}
		if !resourceMatches(r.Resources, resource) {
			continue
		}
		return true
	}
	return false
}

// verbMatches: rule grants verb when rule.Verbs contains the verb or "*".
//
// Query verb "*" means "any verb" — match any rule that has at least
// one verb. Without this, "*" would only match rules whose Verbs list
// itself contains "*", silently dropping subjects whose role grants
// only list/watch/etc.
func verbMatches(ruleVerbs []string, verb string) bool {
	if verb == "*" {
		return len(ruleVerbs) > 0
	}
	return slices.Contains(ruleVerbs, "*") || slices.Contains(ruleVerbs, verb)
}

// groupMatches: rule grants the group when rule.APIGroups contains
// the group or "*". Empty group ("") is the core API group; the rule
// must list it explicitly (or "*") for a match — there is no "any"
// fallback because authors who write "" mean the core group.
func groupMatches(ruleGroups []string, group string) bool {
	return slices.Contains(ruleGroups, "*") || slices.Contains(ruleGroups, group)
}

// resourceMatches: rule grants the resource when rule.Resources
// contains the bare resource name or "*". Subresource matching
// ("pods/log") is a literal compare for now — RBAC's "pods/*"
// pattern isn't expanded, since we don't know all of pods's
// subresources in this context. First iteration constraint, see
// "out of scope" in the design doc.
func resourceMatches(ruleResources []string, resource string) bool {
	return slices.Contains(ruleResources, "*") || slices.Contains(ruleResources, resource)
}
