// Package rbac implements a zero-dependency security.SecuritySource that audits Roles, ClusterRoles,
// and their bindings for privilege-escalation paths and over-broad grants. Checks cover wildcard
// rules, impersonation, bind/escalate, exec/attach/port-forward, kubelet and webhook control, and CSR
// approval. They also cover cluster-wide secret reads, and bindings to anonymous users,
// system:masters, cluster-admin, or the default ServiceAccount.
//
// Kubernetes bootstrap objects (label kubernetes.io/bootstrapping:
// rbac-defaults) and system:-prefixed names are excluded — the built-in
// roles legitimately hold broad grants and flagging them is pure noise.
package rbac

import (
	"context"
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/janosmiko/lfk/internal/security"
)

// Source is the rbac SecuritySource implementation.
type Source struct {
	client kubernetes.Interface
}

// New returns an rbac source with no client. Fetch returns an empty slice
// and IsAvailable reports false.
func New() *Source { return &Source{} }

// NewWithClient returns an rbac source that lists via the given client.
func NewWithClient(client kubernetes.Interface) *Source {
	return &Source{client: client}
}

// Name returns the stable identifier.
func (s *Source) Name() string { return "rbac" }

// Categories returns the categories this source contributes to.
func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryMisconfig}
}

// IsAvailable returns true only when a kubernetes client has been injected.
func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	return s.client != nil, nil
}

// rbacData holds the four RBAC lists. Each OK flag records whether the list
// succeeded. Checks that depend on a failed list are skipped entirely
// (best-effort RBAC) instead of emitting false positives.
type rbacData struct {
	roles               []rbacv1.Role
	rolesOK             bool
	clusterRoles        []rbacv1.ClusterRole
	clusterRolesOK      bool
	roleBindings        []rbacv1.RoleBinding
	roleBindingsOK      bool
	clusterRoleBindings []rbacv1.ClusterRoleBinding
	crbsOK              bool
}

// skipObject reports whether an RBAC object is part of the Kubernetes
// built-in set: bootstrap-labeled (kubeadm defaults, aggregated admin/edit/
// view) or system:-prefixed. Built-ins legitimately hold broad grants.
//
// Known tradeoff: the API server does not reserve the system: prefix. A
// principal who can already write RBAC objects can name one "system:..."
// to evade this audit. The source is a misconfiguration finder, not an
// intrusion detector — an attacker with RBAC write access is already past
// what it can see. Documented in docs/security.md.
func skipObject(meta *metav1.ObjectMeta) bool {
	if meta.Labels["kubernetes.io/bootstrapping"] == "rbac-defaults" {
		return true
	}
	return strings.HasPrefix(meta.Name, "system:")
}

// Fetch lists Roles, ClusterRoles, RoleBindings, and ClusterRoleBindings
// (best-effort per type) and runs the RBAC checks over them. Empty namespace
// means all namespaces. Cluster-scoped objects are always included because
// their grants apply everywhere.
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	if s.client == nil {
		return nil, nil
	}
	d := &rbacData{}
	d.roles, d.rolesOK = security.Collect(func(o metav1.ListOptions) ([]rbacv1.Role, string, error) {
		l, err := s.client.RbacV1().Roles(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	d.clusterRoles, d.clusterRolesOK = security.Collect(func(o metav1.ListOptions) ([]rbacv1.ClusterRole, string, error) {
		l, err := s.client.RbacV1().ClusterRoles().List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	d.roleBindings, d.roleBindingsOK = security.Collect(func(o metav1.ListOptions) ([]rbacv1.RoleBinding, string, error) {
		l, err := s.client.RbacV1().RoleBindings(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	d.clusterRoleBindings, d.crbsOK = security.Collect(func(o metav1.ListOptions) ([]rbacv1.ClusterRoleBinding, string, error) {
		l, err := s.client.RbacV1().ClusterRoleBindings().List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})

	return slices.Concat(d.ruleFindings(), d.bindingFindings()), nil
}

func makeFinding(ns, kind, name, check string, sev security.Severity, title, summary string) security.Finding {
	return security.Finding{
		ID:       "rbac/" + ns + "/" + kind + "/" + name + "/" + check,
		Source:   "rbac",
		Category: security.CategoryMisconfig,
		Severity: sev,
		Title:    title,
		Resource: security.ResourceRef{Namespace: ns, Kind: kind, Name: name},
		Summary:  summary,
		Labels:   map[string]string{"check": check},
	}
}
