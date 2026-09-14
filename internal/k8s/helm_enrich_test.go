package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/model"
)

func TestEnrichHelmWorkloadStatus_OneListPerKind(t *testing.T) {
	replicas := int32(2)
	dep := &appsv1.Deployment{
		Name:      "argocd-server",
		Namespace: "argo-cd",
		Labels:    map[string]string{"app.kubernetes.io/instance": "argocd"},
		Spec:      appsv1.DeploymentSpec{Replicas: &replicas},
		Status:    appsv1.DeploymentStatus{AvailableReplicas: 2, ReadyReplicas: 2},
	}
	sts := &appsv1.StatefulSet{
		Name: "argocd-app-controller", Namespace: "argo-cd",
		Spec:   appsv1.StatefulSetSpec{Replicas: &replicas},
		Status: appsv1.StatefulSetStatus{ReadyReplicas: 2},
	}
	cs := k8sfake.NewSimpleClientset(dep, sts)

	items := []model.Item{
		{Kind: "Deployment", Name: "argocd-server", Namespace: "argo-cd"},
		{Kind: "StatefulSet", Name: "argocd-app-controller", Namespace: "argo-cd"},
	}
	mergeIndex := map[string]int{
		helmRefKey("Deployment", "argo-cd", "argocd-server"):          0,
		helmRefKey("StatefulSet", "argo-cd", "argocd-app-controller"): 1,
	}

	enrichHelmWorkloadStatus(t.Context(), cs, "argo-cd", items, mergeIndex)

	// The status merge is what the lists are for: a workload without the
	// release's instance label must still get its live Ready count.
	assert.Equal(t, "2/2", items[0].Ready)
	assert.Equal(t, "2/2", items[1].Ready)

	lists := map[string]int{}
	for _, a := range cs.Actions() {
		if a.GetVerb() != "list" {
			continue
		}
		lists[a.GetResource().Resource]++
	}
	require.NotEmpty(t, lists)
	for resource, n := range lists {
		assert.Equalf(t, 1, n, "%s listed %d times, want 1", resource, n)
	}
}
