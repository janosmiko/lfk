package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// rootGVRForKind maps a Kind that GetResourceTree accepts as an entry point
// to the GVR used to fetch the root resource for status extraction. It is
// also reused by wrapWithOwners for owner-chain lookups (a strict superset
// of what owners can be — Service and Node never appear as owners but their
// presence here is harmless).
//
// Pod is intentionally absent: buildPodTree fetches it via the typed
// clientset and sets root.Status from pod.Status.Phase directly.
var rootGVRForKind = map[string]schema.GroupVersionResource{
	"Deployment":  {Group: "apps", Version: "v1", Resource: "deployments"},
	"ReplicaSet":  {Group: "apps", Version: "v1", Resource: "replicasets"},
	"StatefulSet": {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"DaemonSet":   {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"Job":         {Group: "batch", Version: "v1", Resource: "jobs"},
	"CronJob":     {Group: "batch", Version: "v1", Resource: "cronjobs"},
	"Service":     {Group: "", Version: "v1", Resource: "services"},
	"Node":        {Group: "", Version: "v1", Resource: "nodes"},
}

func (c *Client) GetResourceTree(ctx context.Context, contextName, namespace, kind, name string) (*model.ResourceNode, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	root := &model.ResourceNode{
		Name:      name,
		Kind:      kind,
		Namespace: namespace,
	}

	// Fetch the root resource so the tree's root node renders with a status
	// consistent with how owner-chain ancestors are rendered by wrapWithOwners.
	// Skipped for Pod (buildPodTree handles it) and unknown CRD kinds (no
	// GVR mapping). Errors are non-fatal — empty status is the legacy default.
	if gvr, ok := rootGVRForKind[kind]; ok {
		var obj *unstructured.Unstructured
		var getErr error
		if kind == "Node" {
			obj, getErr = dynClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		} else {
			obj, getErr = dynClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		}
		if getErr == nil && obj != nil {
			root.Status = extractStatus(obj.Object)
		}
	}

	switch kind {
	case "Deployment":
		err = c.buildDeploymentTree(ctx, dynClient, namespace, name, root)
	case "StatefulSet", "DaemonSet", "Job":
		err = c.buildPodOwnerTree(ctx, dynClient, namespace, kind, name, root)
	case "ReplicaSet":
		err = c.buildPodOwnerTree(ctx, dynClient, namespace, "ReplicaSet", name, root)
	case "CronJob":
		err = c.buildCronJobTree(ctx, dynClient, namespace, name, root)
	case "Service":
		err = c.buildServiceTree(ctx, contextName, namespace, name, root)
	case "Node":
		err = c.buildNodeTree(ctx, dynClient, name, root)
	case "Pod":
		err = c.buildPodTree(ctx, contextName, namespace, name, root)
	default:
		err = c.buildGenericOwnerTree(ctx, dynClient, namespace, kind, name, root)
	}

	return root, err
}

func (c *Client) buildDeploymentTree(ctx context.Context, dynClient dynamic.Interface, namespace, deploymentName string, root *model.ResourceNode) error {
	rsGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	rsList, err := dynClient.Resource(rsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing replicasets: %w", err)
	}

	type rsInfo struct {
		name   string
		status string
	}
	var ownedRS []rsInfo
	for _, rs := range rsList.Items {
		for _, ref := range rs.GetOwnerReferences() {
			if ref.Kind == "Deployment" && ref.Name == deploymentName {
				ownedRS = append(ownedRS, rsInfo{
					name:   rs.GetName(),
					status: extractStatus(rs.Object),
				})
			}
		}
	}

	if len(ownedRS) == 0 {
		return nil
	}

	rsSet := make(map[string]*model.ResourceNode, len(ownedRS))
	for _, rs := range ownedRS {
		node := &model.ResourceNode{
			Name:      rs.name,
			Kind:      "ReplicaSet",
			Namespace: namespace,
			Status:    rs.status,
		}
		rsSet[rs.name] = node
		root.Children = append(root.Children, node)
	}

	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}

	// Single existsFn shared across all pods in this loop so the cache
	// dedupes ref lookups across replicas of the same Deployment.
	existsFn := newRefExistsFn(ctx, dynClient, namespace)

	for _, pod := range podList.Items {
		for _, ref := range pod.GetOwnerReferences() {
			if ref.Kind == "ReplicaSet" {
				if rsNode, ok := rsSet[ref.Name]; ok {
					podNode := &model.ResourceNode{
						Name:      pod.GetName(),
						Kind:      "Pod",
						Namespace: pod.GetNamespace(),
						Status:    extractStatus(pod.Object),
					}
					appendContainerNodes(podNode, pod.Object)
					appendPodRefs(podNode, pod.Object, pod.GetNamespace(), existsFn)
					rsNode.Children = append(rsNode.Children, podNode)
				}
			}
		}
	}

	return nil
}

func (c *Client) buildPodOwnerTree(ctx context.Context, dynClient dynamic.Interface, namespace, ownerKind, ownerName string, root *model.ResourceNode) error {
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}

	existsFn := newRefExistsFn(ctx, dynClient, namespace)

	for _, pod := range podList.Items {
		for _, ref := range pod.GetOwnerReferences() {
			if ref.Kind == ownerKind && ref.Name == ownerName {
				podNode := &model.ResourceNode{
					Name:      pod.GetName(),
					Kind:      "Pod",
					Namespace: pod.GetNamespace(),
					Status:    extractStatus(pod.Object),
				}
				appendContainerNodes(podNode, pod.Object)
				appendPodRefs(podNode, pod.Object, pod.GetNamespace(), existsFn)
				root.Children = append(root.Children, podNode)
				break
			}
		}
	}
	return nil
}

func (c *Client) buildGenericOwnerTree(ctx context.Context, dynClient dynamic.Interface, namespace, ownerKind, ownerName string, root *model.ResourceNode) error {
	intermediateGVRs := []struct {
		gvr  schema.GroupVersionResource
		kind string
	}{
		{schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, "StatefulSet"},
		{schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}, "ReplicaSet"},
		{schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, "Deployment"},
		{schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, "Job"},
	}

	ownedIntermediates := make(map[string]string)
	var intermediateNodes []*model.ResourceNode

	for _, ig := range intermediateGVRs {
		list, err := dynClient.Resource(ig.gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, item := range list.Items {
			for _, ref := range item.GetOwnerReferences() {
				if ref.Kind == ownerKind && ref.Name == ownerName {
					nodeName := item.GetName()
					ownedIntermediates[nodeName] = ig.kind
					intermediateNodes = append(intermediateNodes, &model.ResourceNode{
						Name:      nodeName,
						Kind:      ig.kind,
						Namespace: item.GetNamespace(),
						Status:    extractStatus(item.Object),
					})
					break
				}
			}
		}
	}

	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}

	intermediateMap := make(map[string]*model.ResourceNode, len(intermediateNodes))
	for _, n := range intermediateNodes {
		intermediateMap[n.Name] = n
	}

	existsFn := newRefExistsFn(ctx, dynClient, namespace)

	for _, pod := range podList.Items {
		for _, ref := range pod.GetOwnerReferences() {
			podNode := &model.ResourceNode{
				Name:      pod.GetName(),
				Kind:      "Pod",
				Namespace: pod.GetNamespace(),
				Status:    extractStatus(pod.Object),
			}
			appendContainerNodes(podNode, pod.Object)
			appendPodRefs(podNode, pod.Object, pod.GetNamespace(), existsFn)
			if parent, ok := intermediateMap[ref.Name]; ok {
				parent.Children = append(parent.Children, podNode)
				break
			}
			if ref.Kind == ownerKind && ref.Name == ownerName {
				root.Children = append(root.Children, podNode)
				break
			}
		}
	}

	root.Children = append(root.Children, intermediateNodes...)

	directChildGVRs := []struct {
		gvr  schema.GroupVersionResource
		kind string
	}{
		{schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, "Secret"},
		{schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, "ConfigMap"},
		{schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}, "Service"},
	}
	for _, dg := range directChildGVRs {
		list, err := dynClient.Resource(dg.gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, item := range list.Items {
			for _, ref := range item.GetOwnerReferences() {
				if ref.Kind == ownerKind && ref.Name == ownerName {
					root.Children = append(root.Children, &model.ResourceNode{
						Name:      item.GetName(),
						Kind:      dg.kind,
						Namespace: item.GetNamespace(),
						Status:    extractStatus(item.Object),
					})
					break
				}
			}
		}
	}

	return nil
}

func (c *Client) buildCronJobTree(ctx context.Context, dynClient dynamic.Interface, namespace, cronJobName string, root *model.ResourceNode) error {
	jobGVR := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	jobList, err := dynClient.Resource(jobGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing jobs: %w", err)
	}

	for _, job := range jobList.Items {
		for _, ref := range job.GetOwnerReferences() {
			if ref.Kind == "CronJob" && ref.Name == cronJobName {
				jobNode := &model.ResourceNode{
					Name:      job.GetName(),
					Kind:      "Job",
					Namespace: job.GetNamespace(),
					Status:    extractStatus(job.Object),
				}
				root.Children = append(root.Children, jobNode)
				_ = c.buildPodOwnerTree(ctx, dynClient, namespace, "Job", job.GetName(), jobNode)
			}
		}
	}
	return nil
}

func (c *Client) buildServiceTree(ctx context.Context, contextName, namespace, serviceName string, root *model.ResourceNode) error {
	pods, err := c.getPodsForService(ctx, contextName, namespace, serviceName)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		root.Children = append(root.Children, &model.ResourceNode{
			Name:      pod.Name,
			Kind:      "Pod",
			Namespace: pod.Namespace,
			Status:    pod.Status,
		})
	}
	return nil
}

func (c *Client) buildNodeTree(ctx context.Context, dynClient dynamic.Interface, nodeName string, root *model.ResourceNode) error {
	pods, err := c.getPodsOnNode(ctx, dynClient, nodeName)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		root.Children = append(root.Children, &model.ResourceNode{
			Name:      pod.Name,
			Kind:      "Pod",
			Namespace: pod.Namespace,
			Status:    pod.Status,
		})
	}
	return nil
}

func (c *Client) buildPodTree(ctx context.Context, contextName, namespace, podName string, root *model.ResourceNode) error {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return err
	}

	pod, err := cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting pod %s: %w", podName, err)
	}

	if pod.Status.Phase != "" {
		root.Status = string(pod.Status.Phase)
	}

	for _, ct := range pod.Spec.InitContainers {
		root.Children = append(root.Children, &model.ResourceNode{
			Name:      ct.Name,
			Kind:      "Container",
			Namespace: namespace,
			Status:    containerStatusFromPod(ct.Name, pod.Status.InitContainerStatuses),
		})
	}

	for _, ct := range pod.Spec.Containers {
		root.Children = append(root.Children, &model.ResourceNode{
			Name:      ct.Name,
			Kind:      "Container",
			Namespace: namespace,
			Status:    containerStatusFromPod(ct.Name, pod.Status.ContainerStatuses),
		})
	}

	dynClient, dynErr := c.dynamicForContext(contextName)
	if dynErr != nil {
		// Tree build proceeds without ref existence checks and without an
		// owner chain. Log so operators can see why MissingRef flags and
		// owner ancestors aren't appearing.
		logger.Warn("Resource tree: dynamic client unavailable; ref existence checks and owner chain skipped",
			"context", contextName, "error", dynErr)
	}

	if obj, convErr := runtime.DefaultUnstructuredConverter.ToUnstructured(pod); convErr == nil {
		var exists existsFn
		if dynClient != nil {
			exists = newRefExistsFn(ctx, dynClient, namespace)
		}
		appendPodRefs(root, obj, namespace, exists)
	}

	if len(pod.OwnerReferences) > 0 && dynClient != nil {
		c.wrapWithOwners(ctx, dynClient, namespace, pod.OwnerReferences[0].Kind, pod.OwnerReferences[0].Name, root)
	}

	return nil
}

func (c *Client) wrapWithOwners(ctx context.Context, dynClient dynamic.Interface, namespace, ownerKind, ownerName string, root *model.ResourceNode) {
	// Reuse rootGVRForKind — it is a superset of legitimate owner kinds.
	gvrForKind := rootGVRForKind

	type ownerInfo struct {
		kind, name, status   string
		ownerKind, ownerName string
	}
	var chain []ownerInfo

	curKind, curName := ownerKind, ownerName
	for range 5 {
		gvr, ok := gvrForKind[curKind]
		if !ok {
			chain = append(chain, ownerInfo{kind: curKind, name: curName})
			break
		}
		obj, err := dynClient.Resource(gvr).Namespace(namespace).Get(ctx, curName, metav1.GetOptions{})
		if err != nil {
			chain = append(chain, ownerInfo{kind: curKind, name: curName})
			break
		}
		info := ownerInfo{
			kind:   curKind,
			name:   curName,
			status: extractStatus(obj.Object),
		}
		refs := obj.GetOwnerReferences()
		if len(refs) > 0 {
			info.ownerKind = refs[0].Kind
			info.ownerName = refs[0].Name
		}
		chain = append(chain, info)
		if info.ownerKind == "" {
			break
		}
		curKind, curName = info.ownerKind, info.ownerName
	}

	if len(chain) == 0 {
		return
	}

	origName := root.Name
	origKind := root.Kind
	origNs := root.Namespace
	origStatus := root.Status
	origChildren := root.Children

	podNode := &model.ResourceNode{
		Name:      origName,
		Kind:      origKind,
		Namespace: origNs,
		Status:    origStatus,
		Children:  origChildren,
	}

	top := chain[len(chain)-1]
	root.Name = top.name
	root.Kind = top.kind
	root.Namespace = namespace
	root.Status = top.status
	root.Children = nil

	current := root
	for i := len(chain) - 2; i >= 0; i-- {
		node := &model.ResourceNode{
			Name:      chain[i].name,
			Kind:      chain[i].kind,
			Namespace: namespace,
			Status:    chain[i].status,
		}
		current.Children = append(current.Children, node)
		current = node
	}
	current.Children = append(current.Children, podNode)
}

func appendContainerNodes(podNode *model.ResourceNode, obj map[string]any) {
	spec, _ := obj["spec"].(map[string]any)
	if spec == nil {
		return
	}
	// Pull container statuses from pod.status so Container nodes built from
	// the unstructured tree path render Running/Waiting/etc. consistently
	// with the typed buildPodTree path. extractContainerStatusMap returns
	// nil when the status block is absent, in which case Status is left
	// empty rather than defaulting to "Waiting".
	status, _ := obj["status"].(map[string]any)
	statusByKey := map[string]map[string]string{
		"initContainers": extractContainerStatusMap(status, "initContainerStatuses"),
		"containers":     extractContainerStatusMap(status, "containerStatuses"),
	}
	for _, key := range []string{"initContainers", "containers"} {
		containers, _ := spec[key].([]any)
		lookup := statusByKey[key]
		for _, c := range containers {
			cMap, ok := c.(map[string]any)
			if !ok {
				continue
			}
			name, _ := cMap["name"].(string)
			if name == "" {
				continue
			}
			ctStatus := ""
			if lookup != nil {
				ctStatus = lookup[name]
				if ctStatus == "" {
					// Status block exists but this container isn't listed yet
					// (e.g., just-created pod) — match the typed default.
					ctStatus = "Waiting"
				}
			}
			podNode.Children = append(podNode.Children, &model.ResourceNode{
				Name:      name,
				Kind:      "Container",
				Namespace: podNode.Namespace,
				Status:    ctStatus,
			})
		}
	}
}

// extractContainerStatusMap walks pod.status[key] (an array of container
// status entries) and returns a name → state-string map mirroring
// containerStateString's typed semantics. Returns nil when the entry is
// missing or not a list, so callers can distinguish "no status info" from
// "container missing from status list".
func extractContainerStatusMap(podStatus map[string]any, key string) map[string]string {
	if podStatus == nil {
		return nil
	}
	list, ok := podStatus[key].([]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		if name == "" {
			continue
		}
		ready, _ := m["ready"].(bool)
		state, _ := m["state"].(map[string]any)
		out[name] = containerStateFromUnstructured(state, ready)
	}
	return out
}

// containerStateFromUnstructured mirrors containerStateString for the
// unstructured shape: examines the running/waiting/terminated one-of inside
// container status.state.
func containerStateFromUnstructured(state map[string]any, ready bool) string {
	if state == nil {
		return "Unknown"
	}
	if _, ok := state["running"].(map[string]any); ok {
		if ready {
			return "Running"
		}
		return "NotReady"
	}
	if _, ok := state["waiting"].(map[string]any); ok {
		return "Waiting"
	}
	if term, ok := state["terminated"].(map[string]any); ok {
		if reason, _ := term["reason"].(string); reason == "Completed" {
			return "Completed"
		}
		return "Terminated"
	}
	return "Unknown"
}

func containerStatusFromPod(name string, statuses []corev1.ContainerStatus) string {
	for _, cs := range statuses {
		if cs.Name != name {
			continue
		}
		return containerStateString(cs.Ready, cs.State.Waiting, cs.State.Running, cs.State.Terminated)
	}
	return "Waiting"
}
