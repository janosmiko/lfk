package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/security"
)

func clusterRole(name string, rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name}, Rules: rules}
}

func role(ns, name string, rules ...rbacv1.PolicyRule) *rbacv1.Role {
	return &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}, Rules: rules}
}

func crb(name, refName string, subjects ...rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: refName},
		Subjects:   subjects,
	}
}

func rb(ns, name, refKind, refName string, subjects ...rbacv1.Subject) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		RoleRef:    rbacv1.RoleRef{Kind: refKind, Name: refName},
		Subjects:   subjects,
	}
}

func rule(verbs, apiGroups, resources []string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{Verbs: verbs, APIGroups: apiGroups, Resources: resources}
}

// fetchChecks maps "ns/kind/name" -> set of check ids that fired.
func fetchChecks(t *testing.T, objs ...runtime.Object) map[string]map[string]bool {
	t.Helper()
	s := NewWithClient(fake.NewSimpleClientset(objs...))
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)
	out := map[string]map[string]bool{}
	for _, f := range findings {
		assert.Equal(t, "rbac", f.Source)
		assert.Equal(t, security.CategoryMisconfig, f.Category)
		key := f.Resource.Namespace + "/" + f.Resource.Kind + "/" + f.Resource.Name
		if out[key] == nil {
			out[key] = map[string]bool{}
		}
		out[key][f.Labels["check"]] = true
	}
	return out
}

func TestSourceMetadata(t *testing.T) {
	s := NewWithClient(fake.NewSimpleClientset())
	assert.Equal(t, "rbac", s.Name())
	assert.Equal(t, []security.Category{security.CategoryMisconfig}, s.Categories())
	ok, err := s.IsAvailable(t.Context(), "")
	require.NoError(t, err)
	assert.True(t, ok)

	findings, err := New().Fetch(t.Context(), "", "")
	require.NoError(t, err)
	assert.Empty(t, findings, "nil client fetches nothing")
}

func TestRuleChecks(t *testing.T) {
	cases := []struct {
		name  string
		rules []rbacv1.PolicyRule
		check string
		fires bool
	}{
		{"impersonate users", []rbacv1.PolicyRule{rule([]string{"impersonate"}, []string{""}, []string{"users"})}, "rbac_impersonate", true},
		{"impersonate uids via authentication group", []rbacv1.PolicyRule{rule([]string{"impersonate"}, []string{"authentication.k8s.io"}, []string{"uids"})}, "rbac_impersonate", true},
		{"impersonate verb on unrelated resource", []rbacv1.PolicyRule{rule([]string{"impersonate"}, []string{""}, []string{"pods"})}, "rbac_impersonate", false},
		{"bind roles", []rbacv1.PolicyRule{rule([]string{"bind"}, []string{"rbac.authorization.k8s.io"}, []string{"clusterroles"})}, "rbac_bind_escalate", true},
		{"escalate roles", []rbacv1.PolicyRule{rule([]string{"escalate"}, []string{"rbac.authorization.k8s.io"}, []string{"roles"})}, "rbac_bind_escalate", true},
		{"pods/exec create", []rbacv1.PolicyRule{rule([]string{"create"}, []string{""}, []string{"pods/exec"})}, "rbac_pod_exec", true},
		{"pods/attach create", []rbacv1.PolicyRule{rule([]string{"create"}, []string{""}, []string{"pods/attach"})}, "rbac_pod_exec", true},
		{"wildcard verb on pods/exec", []rbacv1.PolicyRule{rule([]string{"*"}, []string{""}, []string{"pods/exec"})}, "rbac_pod_exec", true},
		{"plain pods access is not exec", []rbacv1.PolicyRule{rule([]string{"create"}, []string{""}, []string{"pods"})}, "rbac_pod_exec", false},
		{"pods/portforward", []rbacv1.PolicyRule{rule([]string{"create"}, []string{""}, []string{"pods/portforward"})}, "rbac_port_forward", true},
		{"nodes/proxy get", []rbacv1.PolicyRule{rule([]string{"get"}, []string{""}, []string{"nodes/proxy"})}, "rbac_nodes_proxy", true},
		{"csr approval update", []rbacv1.PolicyRule{rule([]string{"update"}, []string{"certificates.k8s.io"}, []string{"certificatesigningrequests/approval"})}, "rbac_csr_approval", true},
		{"webhook write", []rbacv1.PolicyRule{rule([]string{"update"}, []string{"admissionregistration.k8s.io"}, []string{"mutatingwebhookconfigurations"})}, "rbac_webhook_admin", true},
		{"webhook read-only", []rbacv1.PolicyRule{rule([]string{"get", "list"}, []string{"admissionregistration.k8s.io"}, []string{"validatingwebhookconfigurations"})}, "rbac_webhook_admin", false},
		{"wildcard verbs", []rbacv1.PolicyRule{rule([]string{"*"}, []string{""}, []string{"pods"})}, "rbac_wildcard", true},
		{"wildcard resources", []rbacv1.PolicyRule{rule([]string{"get"}, []string{""}, []string{"*"})}, "rbac_wildcard", true},
		{"no wildcard", []rbacv1.PolicyRule{rule([]string{"get"}, []string{""}, []string{"pods"})}, "rbac_wildcard", false},
		{"wildcard resource does not satisfy explicit checks", []rbacv1.PolicyRule{rule([]string{"create"}, []string{""}, []string{"*"})}, "rbac_pod_exec", false},
		{"wrong apiGroup does not match", []rbacv1.PolicyRule{rule([]string{"impersonate"}, []string{"example.io"}, []string{"users"})}, "rbac_impersonate", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checks := fetchChecks(t, clusterRole("under-test", tc.rules...))
			assert.Equal(t, tc.fires, checks["/ClusterRole/under-test"][tc.check])
		})
	}
}

func TestRuleChecksApplyToNamespacedRoles(t *testing.T) {
	checks := fetchChecks(t,
		role("prod", "exec-role", rule([]string{"create"}, []string{""}, []string{"pods/exec"})),
		role("prod", "wild-role", rule([]string{"*"}, []string{""}, []string{"configmaps"})),
	)
	assert.True(t, checks["prod/Role/exec-role"]["rbac_pod_exec"])
	assert.True(t, checks["prod/Role/wild-role"]["rbac_wildcard"])
}

func TestSecretsClusterWideRequiresBinding(t *testing.T) {
	secretRead := rule([]string{"get", "list"}, []string{""}, []string{"secrets"})
	checks := fetchChecks(t,
		clusterRole("bound-reader", secretRead),
		crb("bind-reader", "bound-reader", rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "app", Namespace: "prod"}),
		clusterRole("inert-reader", secretRead),
	)
	assert.True(t, checks["/ClusterRole/bound-reader"]["rbac_secrets_cluster_wide"])
	assert.False(t, checks["/ClusterRole/inert-reader"]["rbac_secrets_cluster_wide"],
		"an unbound ClusterRole is a definition, not an exposure")

	t.Run("namespaced Role secret read is not cluster-wide", func(t *testing.T) {
		checks := fetchChecks(t, role("prod", "ns-reader", secretRead))
		assert.False(t, checks["prod/Role/ns-reader"]["rbac_secrets_cluster_wide"])
	})

	t.Run("RoleBinding referencing the ClusterRole stays namespace-scoped", func(t *testing.T) {
		checks := fetchChecks(t,
			clusterRole("ns-bound-reader", secretRead),
			rb("prod", "ns-bind", "ClusterRole", "ns-bound-reader",
				rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "app", Namespace: "prod"}),
		)
		assert.False(t, checks["/ClusterRole/ns-bound-reader"]["rbac_secrets_cluster_wide"],
			"a RoleBinding scopes the ClusterRole's grant to one namespace")
	})
}

func TestBindingChecks(t *testing.T) {
	checks := fetchChecks(t,
		crb("anon", "some-role", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "system:anonymous"}),
		crb("unauth", "some-role", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:unauthenticated"}),
		crb("masters", "some-role", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:masters"}),
		crb("admin-user", "cluster-admin", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
		crb("admin-system", "cluster-admin", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:bootstrappers"}),
		crb("admin-sa", "cluster-admin", rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "deployer", Namespace: "ci"}),
		rb("prod", "default-sa", "Role", "writer", rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "default", Namespace: "prod"}),
		rb("prod", "dedicated-sa", "Role", "writer", rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "app", Namespace: "prod"}),
	)
	assert.True(t, checks["/ClusterRoleBinding/anon"]["rbac_anonymous_binding"])
	assert.True(t, checks["/ClusterRoleBinding/unauth"]["rbac_anonymous_binding"])
	assert.True(t, checks["/ClusterRoleBinding/masters"]["rbac_system_masters"])
	assert.True(t, checks["/ClusterRoleBinding/admin-user"]["rbac_cluster_admin"])
	assert.False(t, checks["/ClusterRoleBinding/admin-system"]["rbac_cluster_admin"],
		"system:-prefixed groups are infrastructure, not user grants")
	assert.True(t, checks["/ClusterRoleBinding/admin-sa"]["rbac_cluster_admin"],
		"a ServiceAccount with cluster-admin always counts")
	assert.True(t, checks["prod/RoleBinding/default-sa"]["rbac_default_sa_binding"])
	assert.False(t, checks["prod/RoleBinding/dedicated-sa"]["rbac_default_sa_binding"])
}

func TestBuiltInObjectsExcluded(t *testing.T) {
	bootstrap := clusterRole("looks-scary", rule([]string{"*"}, []string{"*"}, []string{"*"}))
	bootstrap.Labels = map[string]string{"kubernetes.io/bootstrapping": "rbac-defaults"}
	bootstrapBinding := crb("public-info", "looks-scary", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:unauthenticated"})
	bootstrapBinding.Labels = map[string]string{"kubernetes.io/bootstrapping": "rbac-defaults"}

	checks := fetchChecks(t,
		bootstrap,
		bootstrapBinding,
		clusterRole("system:custom-thing", rule([]string{"*"}, []string{"*"}, []string{"*"})),
	)
	assert.Empty(t, checks["/ClusterRole/looks-scary"], "bootstrap-labeled roles are excluded")
	assert.Empty(t, checks["/ClusterRoleBinding/public-info"], "bootstrap-labeled bindings are excluded")
	assert.Empty(t, checks["/ClusterRole/system:custom-thing"], "system:-prefixed names are excluded")
}

// TestForbiddenListsSkipDependentChecks: each check only requires its own
// list; a Forbidden ClusterRole list must not break binding checks, and a
// Forbidden binding list must not break rule checks (and must disable the
// binding-dependent secrets check).
func TestForbiddenListsSkipDependentChecks(t *testing.T) {
	secretRead := rule([]string{"get"}, []string{""}, []string{"secrets"})
	objs := []runtime.Object{
		clusterRole("reader", secretRead),
		crb("bind-reader", "reader", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "alice"}),
		crb("masters", "x", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:masters"}),
	}

	t.Run("clusterroles forbidden", func(t *testing.T) {
		client := fake.NewSimpleClientset(objs...)
		client.PrependReactor("list", "clusterroles", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "clusterroles"}, "", nil)
		})
		findings, err := NewWithClient(client).Fetch(t.Context(), "", "")
		require.NoError(t, err)
		var masters bool
		for _, f := range findings {
			masters = masters || f.Labels["check"] == "rbac_system_masters"
			assert.NotEqual(t, "rbac_secrets_cluster_wide", f.Labels["check"])
		}
		assert.True(t, masters, "binding checks must survive a forbidden role list")
	})

	t.Run("bindings forbidden", func(t *testing.T) {
		client := fake.NewSimpleClientset(objs...)
		client.PrependReactor("list", "clusterrolebindings", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "clusterrolebindings"}, "", nil)
		})
		findings, err := NewWithClient(client).Fetch(t.Context(), "", "")
		require.NoError(t, err)
		for _, f := range findings {
			assert.NotEqual(t, "rbac_secrets_cluster_wide", f.Labels["check"],
				"the bound-role join needs the binding list")
			assert.NotEqual(t, "rbac_system_masters", f.Labels["check"])
		}
	})
}
