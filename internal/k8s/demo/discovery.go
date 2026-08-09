package demo

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ListKinds maps every seeded GroupVersionResource to its List kind — the
// shape dynamicfake.NewSimpleDynamicClientWithCustomListKinds needs to serve
// List calls. metrics.k8s.io/v1 is mapped alongside v1beta1 because
// internal/k8s.metricsGVR tries both; only v1beta1 carries seed data since
// GetPodMetrics/GetNodeMetrics stop at the first route that returns.
func ListKinds() map[schema.GroupVersionResource]string {
	return map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:       "PodList",
		{Group: "", Version: "v1", Resource: "services"}:   "ServiceList",
		{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		{Group: "", Version: "v1", Resource: "nodes"}:      "NodeList",
		{Group: "", Version: "v1", Resource: "events"}:     "EventList",

		{Group: "apps", Version: "v1", Resource: "deployments"}: "DeploymentList",
		{Group: "apps", Version: "v1", Resource: "replicasets"}: "ReplicaSetList",

		{Group: "batch", Version: "v1", Resource: "jobs"}: "JobList",

		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:  "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1", Resource: "pods"}:       "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}: "NodeMetricsList",
		{Group: "metrics.k8s.io", Version: "v1", Resource: "nodes"}:      "NodeMetricsList",
	}
}

// APIResourceLists is the discovery output installed on the fake clientset's
// FakeDiscovery.Resources, covering every kind ListKinds serves so
// DiscoverAPIResources (internal/k8s/discovery.go) populates the sidebar
// from the demo cluster the same way it would from a real one.
func APIResourceLists() []*metav1.APIResourceList {
	rw := metav1.Verbs{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"}
	ro := metav1.Verbs{"get", "list"}
	return []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: rw},
				{Name: "services", Namespaced: true, Kind: "Service", Verbs: rw},
				{Name: "configmaps", Namespaced: true, Kind: "ConfigMap", Verbs: rw},
				{Name: "nodes", Namespaced: false, Kind: "Node", Verbs: rw},
				{Name: "events", Namespaced: true, Kind: "Event", Verbs: rw},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment", Verbs: rw},
				{Name: "replicasets", Namespaced: true, Kind: "ReplicaSet", Verbs: rw},
			},
		},
		{
			GroupVersion: "batch/v1",
			APIResources: []metav1.APIResource{
				{Name: "jobs", Namespaced: true, Kind: "Job", Verbs: rw},
			},
		},
		{
			GroupVersion: "metrics.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "PodMetrics", Verbs: ro},
				{Name: "nodes", Namespaced: false, Kind: "NodeMetrics", Verbs: ro},
			},
		},
	}
}
