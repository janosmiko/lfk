package k8s

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// subjectKey returns a comparable string for a WhoCanSubject so test
// assertions can sort and diff regardless of the order WhoCan returns
// rows in (which depends on RBAC list ordering, not deterministic).
func subjectKey(s WhoCanSubject) string {
	return s.Kind + "/" + s.Name + "@" + s.Namespace + "|" + s.Via
}

func sortSubjects(s []WhoCanSubject) []string {
	out := make([]string, len(s))
	for i, x := range s {
		out[i] = subjectKey(x)
	}
	sort.Strings(out)
	return out
}

// --- ClusterRoleBinding + ClusterRole (cluster-wide grant) ---

func TestWhoCan_ClusterRoleBindingMatch(t *testing.T) {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-reader"},
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"get", "list", "watch"}, APIGroups: []string{""}, Resources: []string{"pods"}},
		},
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: pmeta("pod-readers"),
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "alice"},
			{Kind: "Group", Name: "developers"},
		},
		RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "pod-reader", APIGroup: "rbac.authorization.k8s.io"},
	}
	cs := k8sfake.NewClientset(cr, crb)
	c := newFakeClient(cs, nil)

	out, err := c.WhoCan(context.Background(), "", "", "", "pods", "get")
	require.NoError(t, err)

	got := sortSubjects(out)
	assert.Contains(t, strings.Join(got, "\n"), "User/alice@",
		"alice (User) granted via the ClusterRoleBinding must appear in the result set")
	assert.Contains(t, strings.Join(got, "\n"), "Group/developers@",
		"developers (Group) likewise — both kinds of subjects are surfaced")
	assert.Contains(t, strings.Join(got, "\n"), "ClusterRoleBinding/pod-readers",
		"the Via column must record the binding so a user can audit the path")
}

// --- RoleBinding + ClusterRole (namespace-scoped grant of a cluster role) ---

func TestWhoCan_RoleBindingToClusterRoleMatch(t *testing.T) {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "edit"},
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"*"}, APIGroups: []string{""}, Resources: []string{"*"}},
		},
	}
	rb := &rbacv1.RoleBinding{
		ObjectMeta: nsMeta("ci-edit", "ci"),
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "ci-deployer", Namespace: "ci"},
		},
		RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "edit", APIGroup: "rbac.authorization.k8s.io"},
	}
	cs := k8sfake.NewClientset(cr, rb)
	c := newFakeClient(cs, nil)

	out, err := c.WhoCan(context.Background(), "", "", "", "pods", "delete")
	require.NoError(t, err)

	got := sortSubjects(out)
	assert.Contains(t, strings.Join(got, "\n"), "ServiceAccount/ci-deployer@ci",
		"SA bound via RoleBinding referencing a ClusterRole gets the namespace from the binding")
}

// --- RoleBinding + Role (purely namespace-scoped) ---

func TestWhoCan_RoleBindingToRoleMatch(t *testing.T) {
	r := &rbacv1.Role{
		ObjectMeta: nsMeta("logs-reader", "monitoring"),
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"pods/log"}},
		},
	}
	rb := &rbacv1.RoleBinding{
		ObjectMeta: nsMeta("read-logs", "monitoring"),
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "operator"},
		},
		RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "logs-reader", APIGroup: "rbac.authorization.k8s.io"},
	}
	cs := k8sfake.NewClientset(r, rb)
	c := newFakeClient(cs, nil)

	out, err := c.WhoCan(context.Background(), "", "", "", "pods/log", "get")
	require.NoError(t, err)
	got := sortSubjects(out)
	assert.Contains(t, strings.Join(got, "\n"), "User/operator@",
		"namespace-scoped Role grant must surface")
}

// --- Wildcard handling ---

func TestWhoCan_VerbWildcard(t *testing.T) {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-master"},
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"*"}, APIGroups: []string{""}, Resources: []string{"pods"}},
		},
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: pmeta("pm"),
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: "wild"}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "pod-master"},
	}
	cs := k8sfake.NewClientset(cr, crb)
	c := newFakeClient(cs, nil)
	for _, v := range []string{"get", "delete", "patch", "create"} {
		out, err := c.WhoCan(context.Background(), "", "", "", "pods", v)
		require.NoError(t, err, "verb %s", v)
		assert.NotEmpty(t, out, "verb=%q with rule.Verbs=[*] must match", v)
	}
}

func TestWhoCan_ResourceWildcard(t *testing.T) {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "core-all"},
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"*"}},
		},
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: pmeta("ca"),
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: "core-reader"}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "core-all"},
	}
	cs := k8sfake.NewClientset(cr, crb)
	c := newFakeClient(cs, nil)
	for _, r := range []string{"pods", "secrets", "configmaps"} {
		out, err := c.WhoCan(context.Background(), "", "", "", r, "get")
		require.NoError(t, err)
		assert.NotEmpty(t, out, "resource=%q with rule.Resources=[*] must match", r)
	}
}

func TestWhoCan_GroupWildcard(t *testing.T) {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "any-group"},
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"get"}, APIGroups: []string{"*"}, Resources: []string{"deployments"}},
		},
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: pmeta("ag"),
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: "deploy-reader"}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "any-group"},
	}
	cs := k8sfake.NewClientset(cr, crb)
	c := newFakeClient(cs, nil)

	out, err := c.WhoCan(context.Background(), "", "", "apps", "deployments", "get")
	require.NoError(t, err)
	assert.NotEmpty(t, out, "group=apps with rule.APIGroups=[*] must match")
}

// --- Namespace scope ---

func TestWhoCan_NamespaceScopeFiltersRoleBindingsButNotClusterBindings(t *testing.T) {
	// Setup: one RoleBinding in namespace "team-a" and another in "team-b",
	// plus a ClusterRoleBinding. When the user asks "who can list pods in
	// team-a", they should see team-a's RB subject and the CRB subject —
	// NOT team-b's RB subject.
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-reader"},
		Rules:      []rbacv1.PolicyRule{{Verbs: []string{"list"}, APIGroups: []string{""}, Resources: []string{"pods"}}},
	}
	rbA := &rbacv1.RoleBinding{
		ObjectMeta: nsMeta("rbA", "team-a"),
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: "alice-A"}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "pod-reader"},
	}
	rbB := &rbacv1.RoleBinding{
		ObjectMeta: nsMeta("rbB", "team-b"),
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: "bob-B"}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "pod-reader"},
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: pmeta("cluster-pod-readers"),
		Subjects:   []rbacv1.Subject{{Kind: "Group", Name: "ops"}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "pod-reader"},
	}
	cs := k8sfake.NewClientset(cr, rbA, rbB, crb)
	c := newFakeClient(cs, nil)

	out, err := c.WhoCan(context.Background(), "", "team-a", "", "pods", "list")
	require.NoError(t, err)
	got := sortSubjects(out)
	joined := strings.Join(got, "\n")
	assert.Contains(t, joined, "User/alice-A@",
		"team-a's RoleBinding subject must appear when scoping to team-a")
	assert.Contains(t, joined, "Group/ops@",
		"ClusterRoleBindings always count regardless of namespace scope")
	assert.NotContains(t, joined, "User/bob-B@",
		"team-b's RoleBinding subject must NOT leak when scoping to team-a")
}

func TestWhoCan_AllNamespacesIncludesEveryRoleBinding(t *testing.T) {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-reader"},
		Rules:      []rbacv1.PolicyRule{{Verbs: []string{"list"}, APIGroups: []string{""}, Resources: []string{"pods"}}},
	}
	rbA := &rbacv1.RoleBinding{
		ObjectMeta: nsMeta("rbA", "team-a"),
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: "alice"}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "pod-reader"},
	}
	rbB := &rbacv1.RoleBinding{
		ObjectMeta: nsMeta("rbB", "team-b"),
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: "bob"}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "pod-reader"},
	}
	cs := k8sfake.NewClientset(cr, rbA, rbB)
	c := newFakeClient(cs, nil)

	out, err := c.WhoCan(context.Background(), "", "", "", "pods", "list")
	require.NoError(t, err)
	joined := strings.Join(sortSubjects(out), "\n")
	assert.Contains(t, joined, "User/alice@", "all-namespaces (ns=\"\") includes team-a's RB")
	assert.Contains(t, joined, "User/bob@", "...and team-b's RB")
}

// --- Empty result ---

func TestWhoCan_NoMatchReturnsEmpty(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	out, err := c.WhoCan(context.Background(), "", "", "", "pods", "delete")
	require.NoError(t, err)
	assert.Empty(t, out, "empty cluster (no roles, no bindings) → no subjects")
}

func TestWhoCan_BindingWithoutMatchingRuleIsSkipped(t *testing.T) {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "secret-reader"},
		Rules:      []rbacv1.PolicyRule{{Verbs: []string{"get"}, APIGroups: []string{""}, Resources: []string{"secrets"}}},
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: pmeta("sr"),
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: "secret-only"}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "secret-reader"},
	}
	cs := k8sfake.NewClientset(cr, crb)
	c := newFakeClient(cs, nil)

	out, err := c.WhoCan(context.Background(), "", "", "", "pods", "get")
	require.NoError(t, err)
	assert.Empty(t, out, "binding's rules don't grant pods/get → skip subject")
}

// --- nonResourceURLs ---

func TestWhoCan_NonResourceURLRulesIgnored(t *testing.T) {
	// A rule that only sets nonResourceURLs (e.g. /healthz) must NOT
	// match a resource query. kubectl-who-can returns nothing for
	// resource queries against /healthz-only roles.
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "healthz"},
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"get"}, NonResourceURLs: []string{"/healthz"}},
		},
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: pmeta("h"),
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: "monitor"}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "healthz"},
	}
	cs := k8sfake.NewClientset(cr, crb)
	c := newFakeClient(cs, nil)

	out, err := c.WhoCan(context.Background(), "", "", "", "pods", "get")
	require.NoError(t, err)
	assert.Empty(t, out, "nonResourceURLs rules don't grant resource permissions")
}

// --- Helpers used only in this file ---

func pmeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name}
}

func nsMeta(name, ns string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: ns}
}
