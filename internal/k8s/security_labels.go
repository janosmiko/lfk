// Package k8s — security_labels.go
// Workload-label resolution for the security label-match ignore patterns.
// Findings whose source attaches to a workload rather than the pod (e.g. Trivy
// image CVEs, keyed by Deployment/DaemonSet/ReplicaSet) carry no labels from
// the source; this resolves the live object's labels so a label pattern can
// still match them. Wired into the security Manager only when label patterns
// exist (see app.setupSecurity) and runs on the throttled security client.
package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// securityLabelGVRForKind maps the resource kinds a security finding can be
// attached to onto the GVR used to fetch the object's labels. Limited to the
// namespaced workload kinds findings reference; unmapped kinds resolve to nil
// (the finding simply stays visible). Pod is included for sources that attach
// to a pod the heuristic source did not observe (e.g. heuristic disabled).
var securityLabelGVRForKind = map[string]schema.GroupVersionResource{
	"Pod":         {Group: "", Version: "v1", Resource: "pods"},
	"Deployment":  {Group: "apps", Version: "v1", Resource: "deployments"},
	"ReplicaSet":  {Group: "apps", Version: "v1", Resource: "replicasets"},
	"StatefulSet": {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"DaemonSet":   {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"Job":         {Group: "batch", Version: "v1", Resource: "jobs"},
	"CronJob":     {Group: "batch", Version: "v1", Resource: "cronjobs"},
}

// ResourceLabels returns the Kubernetes labels of the named object, or nil when
// the kind is unmapped, the object can't be fetched, or the throttled client is
// unavailable. Best-effort and quiet by design: any error yields nil so a
// finding simply stays visible rather than failing the scan. The lookup runs on
// the dedicated throttled security client to avoid draining the foreground API
// budget. Pass it to security.Manager.SetLabelResolver.
func (c *Client) ResourceLabels(ctx context.Context, contextName, namespace, kind, name string) map[string]string {
	gvr, ok := securityLabelGVRForKind[kind]
	// All mapped kinds are namespaced workloads, so a missing namespace can
	// only be a malformed finding — bail rather than issue a cluster-scoped Get
	// the API server would reject.
	if !ok || name == "" || namespace == "" {
		return nil
	}
	dc := c.RawDynamicForContextThrottled(contextName)
	if dc == nil {
		return nil
	}
	obj, err := dc.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil || obj == nil {
		return nil
	}
	return obj.GetLabels()
}
