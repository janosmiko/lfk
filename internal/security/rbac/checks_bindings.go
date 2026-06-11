// Binding-based RBAC checks: dangerous subjects and role references in
// RoleBindings and ClusterRoleBindings.

package rbac

import (
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// bindingFindings audits RoleBinding and ClusterRoleBinding subjects and
// role references.
func (d *rbacData) bindingFindings() []security.Finding {
	var out []security.Finding
	if d.roleBindingsOK {
		for i := range d.roleBindings {
			b := &d.roleBindings[i]
			out = append(out, auditBinding(&b.ObjectMeta, b.Namespace, "RoleBinding", b.RoleRef, b.Subjects)...)
		}
	}
	if d.crbsOK {
		for i := range d.clusterRoleBindings {
			b := &d.clusterRoleBindings[i]
			out = append(out, auditBinding(&b.ObjectMeta, "", "ClusterRoleBinding", b.RoleRef, b.Subjects)...)
		}
	}
	return out
}

// auditBinding runs the subject/roleRef checks over one binding.
func auditBinding(meta *metav1.ObjectMeta, ns, kind string, ref rbacv1.RoleRef, subjects []rbacv1.Subject) []security.Finding {
	if skipObject(meta) {
		return nil
	}
	var out []security.Finding
	name := meta.Name
	if names := matchSubjects(subjects, func(s *rbacv1.Subject) bool {
		return (s.Kind == rbacv1.UserKind && s.Name == "system:anonymous") ||
			(s.Kind == rbacv1.GroupKind && s.Name == "system:unauthenticated")
	}); len(names) > 0 {
		out = append(out, makeFinding(ns, kind, name, "rbac_anonymous_binding", security.SeverityCritical,
			"granted to anonymous users",
			fmt.Sprintf("%s %q grants %q to unauthenticated subjects (%s); anyone who can reach the API server holds these permissions.", kind, name, ref.Name, strings.Join(names, ", "))))
	}
	if names := matchSubjects(subjects, func(s *rbacv1.Subject) bool {
		return s.Kind == rbacv1.GroupKind && s.Name == "system:masters"
	}); len(names) > 0 {
		out = append(out, makeFinding(ns, kind, name, "rbac_system_masters", security.SeverityCritical,
			"uses the system:masters group",
			fmt.Sprintf("%s %q binds the system:masters group, which bypasses RBAC entirely; membership is a permanent, unauditable backdoor.", kind, name)))
	}
	if ref.Kind == "ClusterRole" && ref.Name == "cluster-admin" {
		if names := matchSubjects(subjects, nonSystemSubject); len(names) > 0 {
			out = append(out, makeFinding(ns, kind, name, "rbac_cluster_admin", security.SeverityHigh,
				"cluster-admin granted",
				fmt.Sprintf("%s %q binds cluster-admin to %s; full cluster control. Scope a narrower role instead.", kind, name, strings.Join(names, ", "))))
		}
	}
	if names := matchSubjects(subjects, func(s *rbacv1.Subject) bool {
		return s.Kind == rbacv1.ServiceAccountKind && s.Name == "default"
	}); len(names) > 0 {
		out = append(out, makeFinding(ns, kind, name, "rbac_default_sa_binding", security.SeverityMedium,
			"role granted to default ServiceAccount",
			fmt.Sprintf("%s %q grants %q to the default ServiceAccount; every pod without a dedicated SA inherits it. Bind a dedicated ServiceAccount instead.", kind, name, ref.Name)))
	}
	return out
}

// nonSystemSubject reports whether a subject is outside the system:
// namespace of identities — ServiceAccounts always count; Users and Groups
// count unless system:-prefixed.
func nonSystemSubject(s *rbacv1.Subject) bool {
	if s.Kind == rbacv1.ServiceAccountKind {
		return true
	}
	return !strings.HasPrefix(s.Name, "system:")
}

// matchSubjects returns display names ("Kind name") of subjects matching
// the predicate.
func matchSubjects(subjects []rbacv1.Subject, match func(*rbacv1.Subject) bool) []string {
	var names []string
	for i := range subjects {
		s := &subjects[i]
		if match(s) {
			names = append(names, s.Kind+" "+s.Name)
		}
	}
	return names
}
