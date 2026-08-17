package demo

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// NewClientset returns a fake kubernetes.Interface seeded with the demo
// cluster's typed objects and RBAC reactors that grant every action (see
// AddRBACReactors). It also carries a discovery client reporting every kind
// the dynamic client serves (see APIResourceLists).
func NewClientset() *k8sfake.Clientset {
	cs := k8sfake.NewClientset(typedObjects()...)
	AddRBACReactors(cs)
	if fd, ok := cs.Discovery().(*discoveryfake.FakeDiscovery); ok {
		fd.Resources = APIResourceLists()
	}
	return cs
}

// NewDynamicClient returns a fake dynamic.Interface seeded with the demo
// cluster's objects (converted to unstructured) plus synthetic
// metrics.k8s.io readings, registered against ListKinds so List resolves
// for every seeded GVR.
func NewDynamicClient() *dynamicfake.FakeDynamicClient {
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), ListKinds(), typedUnstructuredObjects()...)
	seedMetrics(dyn)
	return dyn
}

// seedMetrics adds the metrics.k8s.io readings directly against their real
// GVR ({metrics.k8s.io, v1beta1, pods|nodes}). They cannot go through the
// constructor's objects... path. ObjectTracker.Add guesses the resource
// name from the object's Kind via naive pluralization. "PodMetrics" /
// "NodeMetrics" guess to "podmetricses" / "nodemetricses", not the "pods" /
// "nodes" resource name the real metrics API (and internal/k8s.metricsGVR)
// actually uses. Tracker().Create takes the GVR explicitly, bypassing the guess.
func seedMetrics(dyn *dynamicfake.FakeDynamicClient) {
	podGVR := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}
	for _, m := range buildPodMetrics() {
		if err := dyn.Tracker().Create(podGVR, m, m.GetNamespace()); err != nil {
			panic(fmt.Sprintf("demo: seeding pod metrics: %v", err))
		}
	}

	nodeGVR := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}
	for _, m := range buildNodeMetrics() {
		if err := dyn.Tracker().Create(nodeGVR, m, ""); err != nil {
			panic(fmt.Sprintf("demo: seeding node metrics: %v", err))
		}
	}
}

// typedObjects returns every seed object as its typed API type: what the
// fake clientset's tracker and Discovery() need.
func typedObjects() []runtime.Object {
	objs := make([]runtime.Object, 0, 20)
	objs = append(objs,
		buildDeployment(), buildReplicaSet(), buildService(), buildConfigMap(),
		buildJob(), buildJobPod(),
		buildCronJob(), buildCronJobOwnedJob(), buildCronJobOwnedPod(),
	)
	for _, ns := range buildNamespaces() {
		objs = append(objs, ns)
	}
	for _, n := range buildNodes() {
		objs = append(objs, n)
	}
	for _, p := range buildWebPods() {
		objs = append(objs, p)
	}
	for _, e := range buildEvents() {
		objs = append(objs, e)
	}
	return objs
}

// typedUnstructuredObjects converts typedObjects to unstructured form for
// the dynamic fake client's constructor.
func typedUnstructuredObjects() []runtime.Object {
	typed := typedObjects()
	objs := make([]runtime.Object, 0, len(typed))
	for _, o := range typed {
		objs = append(objs, mustToUnstructured(o))
	}
	return objs
}
