package k8s

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// orphanKindItems maps a report field name to its slice, so gating
// cases below can be data rather than a chain of if/else per kind.
func orphanKindItems(kind string, r OrphanReport) []OrphanItem {
	switch kind {
	case "Pods":
		return r.Pods
	case "Secrets":
		return r.Secrets
	case "ConfigMaps":
		return r.ConfigMaps
	case "Services":
		return r.Services
	case "PVCs":
		return r.PVCs
	case "HPAs":
		return r.HPAs
	case "PDBs":
		return r.PDBs
	case "NetworkPolicies":
		return r.NetworkPolicies
	case "Roles":
		return r.Roles
	case "ClusterRoles":
		return r.ClusterRoles
	case "RoleBindings":
		return r.RoleBindings
	case "ClusterRoleBindings":
		return r.ClusterRoleBindings
	}
	return nil
}

// orphanGateCase exercises one gated kind's dependency in isolation:
// fail exactly one list it depends on, and assert both that the target
// kind reports nothing and that a kind NOT depending on the failed list
// still reports normally. The second assertion is the one that catches
// a fix that suppresses everything rather than the specific dependency.
type orphanGateCase struct {
	name string
	// failList is the fake-clientset resource name to fail "list" on.
	failList string
	objs     []runtime.Object

	suppressedKind string // must report zero items
	unaffectedKind string // must still report normally
	unaffectedName string // the object name expected in unaffectedKind
}

// TestDetectOrphans_GatedKindsSkipOnDependencyFailure covers the 8
// gated kinds not already exercised by
// TestDetectOrphans_IngressListFailureSkipsSecrets (Secrets) and
// TestDetectOrphans_RoleBindingListFailureSkipsRoleAndClusterRole
// (Roles, ClusterRoles). Each case fails a list unique to the target
// kind's dependency where the dependency map allows one, so the case
// discriminates rather than proving suppression by failing every list
// at once.
func TestDetectOrphans_GatedKindsSkipOnDependencyFailure(t *testing.T) {
	badHPA := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "bad-hpa"},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: "missing"},
		},
	}
	unboundRole := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "leftover-role"}}
	unboundClusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "leftover-cr"}}
	emptySubjectsCRB := func(name string) *rbacv1.ClusterRoleBinding {
		return &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "does-not-matter"},
		}
	}

	cases := []orphanGateCase{
		{
			name:     "ConfigMaps depend on the Job PodTemplate list",
			failList: "jobs",
			objs: []runtime.Object{
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "stale-cm"}},
				badHPA,
			},
			suppressedKind: "ConfigMaps",
			unaffectedKind: "HPAs", // HPA's dependency doesn't include Jobs.
			unaffectedName: "bad-hpa",
		},
		{
			name:     "PVCs depend on the CronJob PodTemplate list",
			failList: "cronjobs",
			objs: []runtime.Object{
				&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "stale-pvc"}},
				badHPA,
			},
			suppressedKind: "PVCs",
			unaffectedKind: "HPAs", // HPA's dependency doesn't include CronJobs.
			unaffectedName: "bad-hpa",
		},
		{
			name:     "HPAs depend on the StatefulSet list",
			failList: "statefulsets",
			objs: []runtime.Object{
				badHPA,
				unboundRole,
			},
			suppressedKind: "HPAs",
			unaffectedKind: "Roles", // Role bound-status depends on RoleBindings, not StatefulSets.
			unaffectedName: "leftover-role",
		},
		{
			name:     "PDBs depend on the DaemonSet PodTemplate list",
			failList: "daemonsets",
			objs: []runtime.Object{
				&policyv1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "stale-pdb"},
					Spec: policyv1.PodDisruptionBudgetSpec{
						Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "deleted"}},
					},
				},
				unboundClusterRole,
			},
			suppressedKind: "PDBs",
			unaffectedKind: "ClusterRoles", // ClusterRole bound-status depends on binding lists, not DaemonSets.
			unaffectedName: "leftover-cr",
		},
		{
			name:     "NetworkPolicies depend on the live Pod list",
			failList: "pods",
			objs: []runtime.Object{
				&networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "stale-np"},
					Spec: networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "deleted"}},
					},
				},
				emptySubjectsCRB("empty-crb"),
			},
			suppressedKind: "NetworkPolicies",
			unaffectedKind: "ClusterRoleBindings", // Doesn't depend on the Pod list at all.
			unaffectedName: "empty-crb",
		},
		{
			name:     "RoleBindings depend on the Role list (reverse of the Role/ClusterRole case)",
			failList: "roles",
			objs: []runtime.Object{
				&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "empty-rb"},
					RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "does-not-matter"},
				},
				emptySubjectsCRB("empty-crb2"),
			},
			suppressedKind: "RoleBindings",
			unaffectedKind: "ClusterRoleBindings", // Depends on ClusterRoles only, not Roles.
			unaffectedName: "empty-crb2",
		},
		{
			name:     "ClusterRoleBindings depend on the ClusterRole list (reverse of the Role/ClusterRole case)",
			failList: "clusterroles",
			objs: []runtime.Object{
				emptySubjectsCRB("empty-crb3"),
				unboundRole,
			},
			suppressedKind: "ClusterRoleBindings",
			unaffectedKind: "Roles", // Role bound-status depends on RoleBindings, not ClusterRoles.
			unaffectedName: "leftover-role",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs := k8sfake.NewSimpleClientset(tc.objs...)
			cs.PrependReactor("list", tc.failList, func(action k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("forbidden")
			})

			c := newFakeClient(cs, nil)
			report, err := c.DetectOrphans(t.Context(), "", "")

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.failList)
			assert.Empty(t, orphanKindItems(tc.suppressedKind, report),
				"%s must be omitted when its dependency (%s) fails to load", tc.suppressedKind, tc.failList)

			unaffected := orphanKindItems(tc.unaffectedKind, report)
			require.NotEmpty(t, unaffected,
				"%s doesn't depend on %s and must still report", tc.unaffectedKind, tc.failList)
			names := make([]string, 0, len(unaffected))
			for _, item := range unaffected {
				names = append(names, item.Name)
			}
			assert.Contains(t, names, tc.unaffectedName)
		})
	}
}
