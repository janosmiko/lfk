// Package demo provides seed data for an in-memory fake Kubernetes cluster,
// used to run lfk against a realistic dataset without a live cluster. It
// builds a small set of Pods, a Deployment, a ReplicaSet, a Job, a Service, a
// ConfigMap, Nodes and Events spanning two namespaces, with ownerReferences,
// managedFields and status fields shaped the way a real API server would
// populate them.
//
// NewClientset and NewDynamicClient hand back fake clients already wired
// with this data, RBAC reactors that grant every action, and the discovery
// metadata internal/k8s reads through NewTestClient's injection points.
package demo
