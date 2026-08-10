package demo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
)

// seededGVRs lists every GVR the dynamic fake actually carries data for
// (excludes the metrics.k8s.io/v1 routes, which are mapped for
// compatibility but intentionally left empty — see ListKinds).
func seededGVRs() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "", Version: "v1", Resource: "configmaps"},
		{Group: "", Version: "v1", Resource: "nodes"},
		{Group: "", Version: "v1", Resource: "events"},
		{Group: "", Version: "v1", Resource: "namespaces"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "replicasets"},
		{Group: "batch", Version: "v1", Resource: "jobs"},
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"},
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"},
	}
}

func TestNewDynamicClient_ListsEverySeededKind(t *testing.T) {
	dyn := NewDynamicClient()

	for _, gvr := range seededGVRs() {
		t.Run(gvr.String(), func(t *testing.T) {
			list, err := dyn.Resource(gvr).Namespace("").List(t.Context(), metav1.ListOptions{})
			require.NoError(t, err)
			assert.NotEmpty(t, list.Items, "expected at least one seeded object for %s", gvr)
		})
	}
}

// TestNamespaces_EveryReferencedNamespaceHasAnObject holds the invariant
// that made "Namespaces: 0" possible: every namespace any seeded namespaced
// object lives in must have a matching Namespace object, or GetNamespaces
// (which lists Namespace objects directly, not derived from workloads)
// reports fewer namespaces than the cluster actually uses. This guards
// against a future namespace being introduced into the seed data without a
// matching Namespace object.
func TestNamespaces_EveryReferencedNamespaceHasAnObject(t *testing.T) {
	dyn := NewDynamicClient()
	ctx := t.Context()

	nsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	nsList, err := dyn.Resource(nsGVR).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)

	seeded := map[string]bool{}
	for _, item := range nsList.Items {
		seeded[item.GetName()] = true
	}

	assert.True(t, seeded[NamespaceDemo], "expected a Namespace object for %q", NamespaceDemo)
	assert.True(t, seeded[NamespaceJobs], "expected a Namespace object for %q", NamespaceJobs)
	assert.True(t, seeded["default"], `expected a Namespace object for "default"`)
	assert.True(t, seeded["kube-system"], `expected a Namespace object for "kube-system"`)

	for _, gvr := range seededGVRs() {
		if gvr.Resource == "namespaces" {
			continue
		}
		list, err := dyn.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		for _, item := range list.Items {
			ns := item.GetNamespace()
			if ns == "" {
				continue // cluster-scoped object
			}
			assert.True(t, seeded[ns],
				"%s %s/%s references namespace %q with no matching Namespace object",
				gvr.Resource, ns, item.GetName(), ns)
		}
	}
}

func TestNewDynamicClient_SpansMultipleNamespaces(t *testing.T) {
	dyn := NewDynamicClient()

	pods, err := dyn.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}).
		Namespace("").List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)

	namespaces := map[string]bool{}
	for _, item := range pods.Items {
		namespaces[item.GetNamespace()] = true
	}
	assert.True(t, namespaces[NamespaceDemo], "expected pods in %q", NamespaceDemo)
	assert.True(t, namespaces[NamespaceJobs], "expected pods in %q", NamespaceJobs)
	assert.GreaterOrEqual(t, len(namespaces), 2)
}

func TestCrashLoopPod_HasRestartsAndWaitingReason(t *testing.T) {
	dyn := NewDynamicClient()

	obj, err := dyn.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}).
		Namespace(NamespaceDemo).Get(t.Context(), PodWebCrashLoop, metav1.GetOptions{})
	require.NoError(t, err)

	statuses, found, err := unstructured.NestedSlice(obj.Object, "status", "containerStatuses")
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, statuses, 1)

	cs, ok := statuses[0].(map[string]any)
	require.True(t, ok)

	restartCount, found, err := unstructured.NestedInt64(cs, "restartCount")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int64(7), restartCount)

	reason, found, err := unstructured.NestedString(cs, "state", "waiting", "reason")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "CrashLoopBackOff", reason)
}

func TestCrashLoopPod_HasMatchingWarningEvents(t *testing.T) {
	dyn := NewDynamicClient()

	list, err := dyn.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "events"}).
		Namespace(NamespaceDemo).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)

	var matched int
	for _, item := range list.Items {
		kind, _, _ := unstructured.NestedString(item.Object, "involvedObject", "kind")
		name, _, _ := unstructured.NestedString(item.Object, "involvedObject", "name")
		eventType, _, _ := unstructured.NestedString(item.Object, "type")
		if kind == "Pod" && name == PodWebCrashLoop {
			assert.Equal(t, "Warning", eventType)
			matched++
		}
	}
	assert.GreaterOrEqual(t, matched, 2, "expected at least two events for the crashlooping pod")
}

func TestJob_HasFailedCondition(t *testing.T) {
	dyn := NewDynamicClient()

	obj, err := dyn.Resource(schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}).
		Namespace(NamespaceJobs).Get(t.Context(), JobDBMigrate, metav1.GetOptions{})
	require.NoError(t, err)

	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	require.NoError(t, err)
	require.True(t, found)

	var sawFailed bool
	for _, c := range conditions {
		cm, ok := c.(map[string]any)
		require.True(t, ok)
		if cm["type"] == "Failed" && cm["status"] == "True" {
			sawFailed = true
		}
	}
	assert.True(t, sawFailed, "expected a Failed=True condition on the job")
}

func TestOwnerReferences_ChainResolvesPodToDeployment(t *testing.T) {
	dyn := NewDynamicClient()
	ctx := t.Context()

	pod, err := dyn.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}).
		Namespace(NamespaceDemo).Get(ctx, PodWebCrashLoop, metav1.GetOptions{})
	require.NoError(t, err)
	podOwners := pod.GetOwnerReferences()
	require.Len(t, podOwners, 1)
	assert.Equal(t, "ReplicaSet", podOwners[0].Kind)
	assert.Equal(t, ReplicaSetWeb, podOwners[0].Name)

	rs, err := dyn.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}).
		Namespace(NamespaceDemo).Get(ctx, ReplicaSetWeb, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, string(podOwners[0].UID), string(rs.GetUID()))

	rsOwners := rs.GetOwnerReferences()
	require.Len(t, rsOwners, 1)
	assert.Equal(t, "Deployment", rsOwners[0].Kind)
	assert.Equal(t, DeploymentWeb, rsOwners[0].Name)

	deploy, err := dyn.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).
		Namespace(NamespaceDemo).Get(ctx, DeploymentWeb, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, string(rsOwners[0].UID), string(deploy.GetUID()))
}

func TestOwnerReferences_JobPodResolvesToJob(t *testing.T) {
	dyn := NewDynamicClient()
	ctx := t.Context()

	pod, err := dyn.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}).
		Namespace(NamespaceJobs).Get(ctx, PodDBMigrate, metav1.GetOptions{})
	require.NoError(t, err)
	owners := pod.GetOwnerReferences()
	require.Len(t, owners, 1)
	assert.Equal(t, "Job", owners[0].Kind)
	assert.Equal(t, JobDBMigrate, owners[0].Name)

	job, err := dyn.Resource(schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}).
		Namespace(NamespaceJobs).Get(ctx, JobDBMigrate, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, string(owners[0].UID), string(job.GetUID()))
}

func TestSeedObjects_CarryManagedFields(t *testing.T) {
	dyn := NewDynamicClient()
	ctx := t.Context()

	cases := []struct {
		name string
		gvr  schema.GroupVersionResource
		ns   string
		obj  string
	}{
		{"deployment", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, NamespaceDemo, DeploymentWeb},
		{"crashloop pod", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}, NamespaceDemo, PodWebCrashLoop},
		{"service", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}, NamespaceDemo, ServiceWeb},
		{"configmap", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, NamespaceDemo, ConfigMapWeb},
		{"job", schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, NamespaceJobs, JobDBMigrate},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj, err := dyn.Resource(tc.gvr).Namespace(tc.ns).Get(ctx, tc.obj, metav1.GetOptions{})
			require.NoError(t, err)

			mf := obj.GetManagedFields()
			require.NotEmpty(t, mf, "expected managedFields on %s/%s", tc.ns, tc.obj)
			for _, e := range mf {
				assert.NotEmpty(t, e.Manager)
				assert.NotNil(t, e.FieldsV1)
			}
		})
	}
}

func TestNewClientset_DiscoveryReportsSeededKinds(t *testing.T) {
	cs := NewClientset()

	fd, ok := cs.Discovery().(*discoveryfake.FakeDiscovery)
	require.True(t, ok)
	require.NotEmpty(t, fd.Resources)

	var kinds []string
	for _, list := range fd.Resources {
		for _, r := range list.APIResources {
			kinds = append(kinds, r.Kind)
		}
	}
	assert.Contains(t, kinds, "Pod")
	assert.Contains(t, kinds, "Deployment")
	assert.Contains(t, kinds, "Job")
	assert.Contains(t, kinds, "Service")
	assert.Contains(t, kinds, "ConfigMap")
	assert.Contains(t, kinds, "Node")
	assert.Contains(t, kinds, "Event")
	assert.Contains(t, kinds, "PodMetrics")
	assert.Contains(t, kinds, "NodeMetrics")
}

func TestNewClientset_SelfSubjectAccessReviewIsAllowed(t *testing.T) {
	cs := NewClientset()

	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: NamespaceDemo, Verb: "delete", Group: "apps", Resource: "deployments",
			},
		},
	}
	result, err := cs.AuthorizationV1().SelfSubjectAccessReviews().Create(t.Context(), sar, metav1.CreateOptions{})
	require.NoError(t, err)
	assert.True(t, result.Status.Allowed)
}

func TestNewClientset_SelfSubjectRulesReviewIsAllowed(t *testing.T) {
	cs := NewClientset()

	review := &authorizationv1.SelfSubjectRulesReview{
		Spec: authorizationv1.SelfSubjectRulesReviewSpec{Namespace: NamespaceDemo},
	}
	result, err := cs.AuthorizationV1().SelfSubjectRulesReviews().Create(t.Context(), review, metav1.CreateOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Status.ResourceRules)
	assert.Contains(t, result.Status.ResourceRules[0].Verbs, "*")
}
