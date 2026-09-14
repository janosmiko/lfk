package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// helmWorkloadManifest covers every workload kind that gets an owner chain
// walked, the shape that used to trigger one namespace-wide LIST per workload.
const helmWorkloadManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: db
  namespace: default
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: agent
  namespace: default
---
apiVersion: batch/v1
kind: Job
metadata:
  name: migrate
  namespace: default
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: cleanup
  namespace: default
`

func treeOwned(apiVersion, kind, name, ownerKind, ownerName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]any{
				"name":      name,
				"namespace": "default",
				"ownerReferences": []any{
					map[string]any{"kind": ownerKind, "name": ownerName},
				},
			},
		},
	}
}

func treePod(name, ownerKind, ownerName string) *unstructured.Unstructured {
	pod := treeOwned("v1", "Pod", name, ownerKind, ownerName)
	pod.Object["spec"] = map[string]any{
		"containers": []any{map[string]any{"name": "app", "image": "app:1"}},
	}
	pod.Object["status"] = map[string]any{"phase": "Running"}
	return pod
}

// TestGetResourceTree_HelmReleaseSharesNamespaceLists guards the fan-out fix:
// a release tree reuses one namespace-wide LIST per kind and one ref-existence
// cache across every workload it owns, instead of one set per workload.
func TestGetResourceTree_HelmReleaseSharesNamespaceLists(t *testing.T) {
	blob := makeHelmBlobWithManifest(
		"rel", "app", "1.0.0", "1.0.0", "deployed", "Install complete", helmWorkloadManifest, 1,
	)
	secret := newFakeHelmReleaseSecret(t, "sh.helm.release.v1.rel.v1", "rel", "deployed", "1", blob, time.Now())
	cs := k8sfake.NewClientset(secret)
	dc := newFakeDynClient(
		treeOwned("apps/v1", "ReplicaSet", "web-abc", "Deployment", "web"),
		treeOwned("batch/v1", "Job", "cleanup-1", "CronJob", "cleanup"),
		treePod("web-abc-1", "ReplicaSet", "web-abc"),
		treePod("db-0", "StatefulSet", "db"),
		treePod("agent-xy", "DaemonSet", "agent"),
		treePod("migrate-1", "Job", "migrate"),
		treePod("cleanup-1-p", "Job", "cleanup-1"),
	)

	var podLists, rsLists, jobLists, saGets int
	count := func(verb, resource string, n *int) {
		dc.PrependReactor(verb, resource, func(k8stesting.Action) (bool, runtime.Object, error) {
			*n++
			return false, nil, nil
		})
	}
	count("list", "pods", &podLists)
	count("list", "replicasets", &rsLists)
	count("list", "jobs", &jobLists)
	count("get", "serviceaccounts", &saGets)

	c := newFakeClient(cs, dc)
	root, err := c.GetResourceTree(t.Context(), "", "default", "HelmRelease", "rel")
	require.NoError(t, err)

	assert.Equal(t, 1, podLists, "one Pod LIST per namespace, whatever the workload count")
	assert.Equal(t, 1, rsLists, "one ReplicaSet LIST per namespace")
	assert.Equal(t, 1, jobLists, "one Job LIST per namespace")
	assert.Equal(t, 1, saGets, "ref existence cache is shared across the tree")

	// Shape is unchanged: every workload keeps its own owner chain.
	web := findTreeChild(root, "Deployment", "web")
	require.NotNil(t, web)
	rsNode := findTreeChild(web, "ReplicaSet", "web-abc")
	require.NotNil(t, rsNode)
	assert.NotNil(t, findTreeChild(rsNode, "Pod", "web-abc-1"))

	for _, tt := range []struct{ kind, name, pod string }{
		{"StatefulSet", "db", "db-0"},
		{"DaemonSet", "agent", "agent-xy"},
		{"Job", "migrate", "migrate-1"},
	} {
		owner := findTreeChild(root, tt.kind, tt.name)
		require.NotNil(t, owner, tt.kind)
		assert.NotNil(t, findTreeChild(owner, "Pod", tt.pod), tt.kind)
	}

	cron := findTreeChild(root, "CronJob", "cleanup")
	require.NotNil(t, cron)
	cronJobNode := findTreeChild(cron, "Job", "cleanup-1")
	require.NotNil(t, cronJobNode)
	assert.NotNil(t, findTreeChild(cronJobNode, "Pod", "cleanup-1-p"))
}
