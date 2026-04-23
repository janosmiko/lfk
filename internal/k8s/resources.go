package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/model"
)

// GetOwnedResources returns resources owned by a parent, resolving through the
// ownership chain. For Deployments, it skips ReplicaSets and returns Pods directly.
// For CronJobs, it returns Jobs.
// For Services, it returns Pods matching the service selector.
func (c *Client) GetOwnedResources(ctx context.Context, contextName, namespace string, parentKind, parentName string) ([]model.Item, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	switch parentKind {
	case "Deployment":
		// Deployment -> ReplicaSets -> Pods (flatten: show pods directly)
		return c.getPodsViaReplicaSets(ctx, dynClient, namespace, parentName)
	case "StatefulSet", "DaemonSet", "Job":
		// Direct owner of pods
		return c.getPodsByOwner(ctx, dynClient, namespace, parentKind, parentName)
	case "CronJob":
		// CronJob -> Jobs
		return c.getJobsByOwner(ctx, dynClient, namespace, parentName)
	case "Service":
		// Service -> Pods via selector
		return c.getPodsForService(ctx, contextName, namespace, parentName)
	case "Kustomization":
		return c.getFluxManagedResources(ctx, dynClient, namespace, parentName)
	case "Application":
		return c.getArgoManagedResources(ctx, dynClient, contextName, namespace, parentName)
	case "HelmRelease":
		return c.getHelmManagedResources(ctx, contextName, namespace, parentName)
	case "Node":
		return c.getPodsOnNode(ctx, dynClient, parentName)
	case "PersistentVolumeClaim":
		// A PVC's "children" are the pods referencing it. Previously this
		// was computed eagerly for every PVC row in GetResources, which
		// turned a single list call into N+1 pod list calls. Now it's
		// deferred until the user selects or drills into a specific PVC.
		return c.getPodsUsingPVC(ctx, dynClient, namespace, parentName)
	default:
		// For ConfigMaps, Secrets, Ingresses - no children, show YAML preview
		return nil, nil
	}
}

// GetResourceTree builds a recursive relationship tree for the given resource.
// Unlike GetOwnedResources (which flattens), this preserves intermediate resources
// like ReplicaSets to show the full ownership hierarchy.
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
		// Generic: find pods owned (directly or transitively) by this resource.
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

	// Find ReplicaSets owned by this deployment.
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

	// Build a set for fast lookup.
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

	// Find Pods owned by those ReplicaSets.
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}

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
				root.Children = append(root.Children, podNode)
				break
			}
		}
	}
	return nil
}

// buildGenericOwnerTree finds pods (and intermediate controllers) owned by a
// CRD resource by walking owner references. This handles resources like CNPG
// Cluster → StatefulSet → Pod.
func (c *Client) buildGenericOwnerTree(ctx context.Context, dynClient dynamic.Interface, namespace, ownerKind, ownerName string, root *model.ResourceNode) error {
	// List common intermediate controllers to discover indirect ownership.
	intermediateGVRs := []struct {
		gvr  schema.GroupVersionResource
		kind string
	}{
		{schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, "StatefulSet"},
		{schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}, "ReplicaSet"},
		{schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, "Deployment"},
		{schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, "Job"},
	}

	// Map of intermediate resource name → kind for those owned by the target.
	ownedIntermediates := make(map[string]string)
	var intermediateNodes []*model.ResourceNode

	for _, ig := range intermediateGVRs {
		list, err := dynClient.Resource(ig.gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue // Silently skip resources we can't list.
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

	// List pods and assign them to intermediate owners or directly to root.
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}

	// Index intermediate nodes by name for child assignment.
	intermediateMap := make(map[string]*model.ResourceNode, len(intermediateNodes))
	for _, n := range intermediateNodes {
		intermediateMap[n.Name] = n
	}

	for _, pod := range podList.Items {
		for _, ref := range pod.GetOwnerReferences() {
			podNode := &model.ResourceNode{
				Name:      pod.GetName(),
				Kind:      "Pod",
				Namespace: pod.GetNamespace(),
				Status:    extractStatus(pod.Object),
			}
			appendContainerNodes(podNode, pod.Object)
			// Pod owned by an intermediate controller.
			if parent, ok := intermediateMap[ref.Name]; ok {
				parent.Children = append(parent.Children, podNode)
				break
			}
			// Pod directly owned by the target resource.
			if ref.Kind == ownerKind && ref.Name == ownerName {
				root.Children = append(root.Children, podNode)
				break
			}
		}
	}

	// Attach intermediate nodes to root.
	root.Children = append(root.Children, intermediateNodes...)

	// Also find non-pod resources directly owned by the target (e.g.,
	// ExternalSecret → Secret, Operator → ConfigMap/Service).
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
				// Find pods owned by this job.
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

	// Set root status from the pod phase.
	if pod.Status.Phase != "" {
		root.Status = string(pod.Status.Phase)
	}

	// Add init containers.
	for _, ct := range pod.Spec.InitContainers {
		root.Children = append(root.Children, &model.ResourceNode{
			Name:      ct.Name,
			Kind:      "Container",
			Namespace: namespace,
			Status:    containerStatusFromPod(ct.Name, pod.Status.InitContainerStatuses),
		})
	}

	// Add regular containers.
	for _, ct := range pod.Spec.Containers {
		root.Children = append(root.Children, &model.ResourceNode{
			Name:      ct.Name,
			Kind:      "Container",
			Namespace: namespace,
			Status:    containerStatusFromPod(ct.Name, pod.Status.ContainerStatuses),
		})
	}

	// Walk owner references upward to show the ownership chain.
	// Build: topOwner → ... → controller → Pod (root) → Containers.
	if len(pod.OwnerReferences) > 0 {
		dynClient, dynErr := c.dynamicForContext(contextName)
		if dynErr == nil {
			c.wrapWithOwners(ctx, dynClient, namespace, pod.OwnerReferences[0].Kind, pod.OwnerReferences[0].Name, root)
		}
	}

	return nil
}

// wrapWithOwners walks owner references upward from a controller and
// restructures the tree so the topmost owner becomes the root. The original
// root node becomes a child of its controller, which becomes a child of the
// controller's owner, etc. This mutates the root pointer's fields in place.
func (c *Client) wrapWithOwners(ctx context.Context, dynClient dynamic.Interface, namespace, ownerKind, ownerName string, root *model.ResourceNode) {
	// Resolve common owner kinds to their GVR.
	gvrForKind := map[string]schema.GroupVersionResource{
		"ReplicaSet":  {Group: "apps", Version: "v1", Resource: "replicasets"},
		"Deployment":  {Group: "apps", Version: "v1", Resource: "deployments"},
		"StatefulSet": {Group: "apps", Version: "v1", Resource: "statefulsets"},
		"DaemonSet":   {Group: "apps", Version: "v1", Resource: "daemonsets"},
		"Job":         {Group: "batch", Version: "v1", Resource: "jobs"},
		"CronJob":     {Group: "batch", Version: "v1", Resource: "cronjobs"},
	}

	// Build the chain bottom-up: [immediateOwner, grandparent, ...]
	type ownerInfo struct {
		kind, name, status   string
		ownerKind, ownerName string
	}
	var chain []ownerInfo

	curKind, curName := ownerKind, ownerName
	for range 5 { // limit depth to prevent loops
		gvr, ok := gvrForKind[curKind]
		if !ok {
			// Unknown kind — add it but stop walking.
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

	// Restructure: the original root (Pod) becomes a leaf, wrapped by the chain.
	// Save original root data and children.
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

	// Build tree top-down: chain is [immediate, grandparent, ...], reverse it.
	// Top of chain = topmost owner.
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

// appendContainerNodes extracts init containers and containers from an
// unstructured pod object and appends them as children of the given node.
func appendContainerNodes(podNode *model.ResourceNode, obj map[string]interface{}) {
	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return
	}
	for _, key := range []string{"initContainers", "containers"} {
		containers, _ := spec[key].([]interface{})
		for _, c := range containers {
			cMap, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := cMap["name"].(string)
			if name != "" {
				podNode.Children = append(podNode.Children, &model.ResourceNode{
					Name:      name,
					Kind:      "Container",
					Namespace: podNode.Namespace,
				})
			}
		}
	}
}

// containerStatusFromPod extracts a human-readable status for a container by name.
func containerStatusFromPod(name string, statuses []corev1.ContainerStatus) string {
	for _, cs := range statuses {
		if cs.Name != name {
			continue
		}
		return containerStateString(cs.Ready, cs.State.Waiting, cs.State.Running, cs.State.Terminated)
	}
	return "Waiting"
}

// GetContainers returns the containers of a pod as items.
func (c *Client) GetContainers(ctx context.Context, contextName, namespace, podName string) ([]model.Item, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	pod, err := cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod %s: %w", podName, err)
	}

	items := make([]model.Item, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers))

	// Init containers first (in spec order).
	for _, c := range pod.Spec.InitContainers {
		isSidecar := c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways
		item := buildContainerItem(c, pod.Status.InitContainerStatuses, true, isSidecar)
		items = append(items, item)
	}

	// Regular containers (in spec order).
	for _, c := range pod.Spec.Containers {
		item := buildContainerItem(c, pod.Status.ContainerStatuses, false, false)
		items = append(items, item)
	}

	return items, nil
}

// GetContainerPorts returns the container ports for a pod.
func (c *Client) GetContainerPorts(ctx context.Context, contextName, namespace, podName string) ([]ContainerPort, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	pod, err := cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod %s: %w", podName, err)
	}

	var ports []ContainerPort
	for _, container := range pod.Spec.Containers {
		for _, p := range container.Ports {
			ports = append(ports, ContainerPort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Protocol:      string(p.Protocol),
			})
		}
	}
	return ports, nil
}

// GetServicePorts returns the ports for a service.
func (c *Client) GetServicePorts(ctx context.Context, contextName, namespace, svcName string) ([]ContainerPort, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	svc, err := cs.CoreV1().Services(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting service %s: %w", svcName, err)
	}

	ports := make([]ContainerPort, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		ports = append(ports, ContainerPort{
			Name:          p.Name,
			ContainerPort: p.Port,
			Protocol:      string(p.Protocol),
		})
	}
	return ports, nil
}

// GetDeploymentPorts returns the container ports for the pod template of a deployment.
func (c *Client) GetDeploymentPorts(ctx context.Context, contextName, namespace, name string) ([]ContainerPort, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	dep, err := cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting deployment %s: %w", name, err)
	}

	var ports []ContainerPort
	for _, container := range dep.Spec.Template.Spec.Containers {
		for _, p := range container.Ports {
			ports = append(ports, ContainerPort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Protocol:      string(p.Protocol),
			})
		}
	}
	return ports, nil
}

// GetStatefulSetPorts returns the container ports for the pod template of a statefulset.
func (c *Client) GetStatefulSetPorts(ctx context.Context, contextName, namespace, name string) ([]ContainerPort, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	sts, err := cs.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting statefulset %s: %w", name, err)
	}

	var ports []ContainerPort
	for _, container := range sts.Spec.Template.Spec.Containers {
		for _, p := range container.Ports {
			ports = append(ports, ContainerPort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Protocol:      string(p.Protocol),
			})
		}
	}
	return ports, nil
}

// GetDaemonSetPorts returns the container ports for the pod template of a daemonset.
func (c *Client) GetDaemonSetPorts(ctx context.Context, contextName, namespace, name string) ([]ContainerPort, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	ds, err := cs.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting daemonset %s: %w", name, err)
	}

	var ports []ContainerPort
	for _, container := range ds.Spec.Template.Spec.Containers {
		for _, p := range container.Ports {
			ports = append(ports, ContainerPort{
				Name:          p.Name,
				ContainerPort: p.ContainerPort,
				Protocol:      string(p.Protocol),
			})
		}
	}
	return ports, nil
}

// --- ownership resolution helpers ---

func (c *Client) getPodsViaReplicaSets(ctx context.Context, dynClient dynamic.Interface, namespace, deploymentName string) ([]model.Item, error) {
	rsGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	rsList, err := dynClient.Resource(rsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing replicasets: %w", err)
	}

	// Find ReplicaSets owned by this deployment.
	var rsNames []string
	for _, rs := range rsList.Items {
		for _, ref := range rs.GetOwnerReferences() {
			if ref.Kind == "Deployment" && ref.Name == deploymentName {
				rsNames = append(rsNames, rs.GetName())
			}
		}
	}

	// Find Pods owned by those ReplicaSets.
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	rsSet := make(map[string]bool, len(rsNames))
	for _, n := range rsNames {
		rsSet[n] = true
	}

	var items []model.Item
	for _, pod := range podList.Items {
		for _, ref := range pod.GetOwnerReferences() {
			if ref.Kind == "ReplicaSet" && rsSet[ref.Name] {
				ti := model.Item{
					Name:      pod.GetName(),
					Namespace: pod.GetNamespace(),
					Kind:      "Pod",
					Status:    extractStatus(pod.Object),
				}
				creationTS := pod.GetCreationTimestamp()
				if !creationTS.IsZero() {
					ti.CreatedAt = creationTS.Time
					ti.Age = formatAge(time.Since(creationTS.Time))
				}
				populateResourceDetails(&ti, pod.Object, "Pod")
				items = append(items, ti)
				break
			}
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (c *Client) getPodsByOwner(ctx context.Context, dynClient dynamic.Interface, namespace, ownerKind, ownerName string) ([]model.Item, error) {
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var items []model.Item
	for _, pod := range podList.Items {
		for _, ref := range pod.GetOwnerReferences() {
			if ref.Kind == ownerKind && ref.Name == ownerName {
				ti := model.Item{
					Name:      pod.GetName(),
					Namespace: pod.GetNamespace(),
					Kind:      "Pod",
					Status:    extractStatus(pod.Object),
				}
				creationTS := pod.GetCreationTimestamp()
				if !creationTS.IsZero() {
					ti.CreatedAt = creationTS.Time
					ti.Age = formatAge(time.Since(creationTS.Time))
				}
				populateResourceDetails(&ti, pod.Object, "Pod")
				items = append(items, ti)
				break
			}
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// getPodsOnNode lists all pods running on a specific node across all namespaces.
func (c *Client) getPodsOnNode(ctx context.Context, dynClient dynamic.Interface, nodeName string) ([]model.Item, error) {
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods on node %s: %w", nodeName, err)
	}

	items := make([]model.Item, 0, len(podList.Items))
	for _, pod := range podList.Items {
		ti := model.Item{
			Name:      pod.GetName(),
			Namespace: pod.GetNamespace(),
			Kind:      "Pod",
			Status:    extractStatus(pod.Object),
		}
		creationTS := pod.GetCreationTimestamp()
		if !creationTS.IsZero() {
			ti.CreatedAt = creationTS.Time
			ti.Age = formatAge(time.Since(creationTS.Time))
		}
		populateResourceDetails(&ti, pod.Object, "Pod")
		items = append(items, ti)
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// getPodsUsingPVC returns the pods in the given namespace that mount the
// named PVC, as model.Item values suitable for the owned-resources view.
// It is the lazy replacement for the old "Used By" list-column: the
// per-PVC pod lookup now runs only when the user actually selects or
// drills into a PVC, collapsing the previous N+1 cost on the PVC list
// load into a single list call on demand.
func (c *Client) getPodsUsingPVC(ctx context.Context, dynClient dynamic.Interface, namespace, pvcName string) ([]model.Item, error) {
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}
	items := make([]model.Item, 0)
	for _, pod := range podList.Items {
		if !podReferencesPVC(pod.Object, pvcName) {
			continue
		}
		ti := model.Item{
			Name:      pod.GetName(),
			Namespace: pod.GetNamespace(),
			Kind:      "Pod",
			Status:    extractStatus(pod.Object),
		}
		creationTS := pod.GetCreationTimestamp()
		if !creationTS.IsZero() {
			ti.CreatedAt = creationTS.Time
			ti.Age = formatAge(time.Since(creationTS.Time))
		}
		populateResourceDetails(&ti, pod.Object, "Pod")
		items = append(items, ti)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// podReferencesPVC reports whether a pod spec includes a volume backed by
// the given PVC claim name.
func podReferencesPVC(podObj map[string]any, pvcName string) bool {
	spec, ok := podObj["spec"].(map[string]any)
	if !ok {
		return false
	}
	volumes, ok := spec["volumes"].([]any)
	if !ok {
		return false
	}
	for _, v := range volumes {
		vol, ok := v.(map[string]any)
		if !ok {
			continue
		}
		pvc, ok := vol["persistentVolumeClaim"].(map[string]any)
		if !ok {
			continue
		}
		if claim, _ := pvc["claimName"].(string); claim == pvcName {
			return true
		}
	}
	return false
}

func (c *Client) getJobsByOwner(ctx context.Context, dynClient dynamic.Interface, namespace, cronJobName string) ([]model.Item, error) {
	jobGVR := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	jobList, err := dynClient.Resource(jobGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}

	var items []model.Item
	for _, job := range jobList.Items {
		for _, ref := range job.GetOwnerReferences() {
			if ref.Kind == "CronJob" && ref.Name == cronJobName {
				items = append(items, model.Item{
					Name:      job.GetName(),
					Namespace: job.GetNamespace(),
					Kind:      "Job",
					Status:    extractStatus(job.Object),
				})
				break
			}
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (c *Client) getPodsForService(ctx context.Context, contextName, namespace, serviceName string) ([]model.Item, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	svc, err := cs.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting service %s: %w", serviceName, err)
	}

	if len(svc.Spec.Selector) == 0 {
		return nil, nil
	}

	// Build label selector string.
	selectorParts := make([]string, 0, len(svc.Spec.Selector))
	for k, v := range svc.Spec.Selector {
		selectorParts = append(selectorParts, k+"="+v)
	}
	sort.Strings(selectorParts)
	labelSelector := strings.Join(selectorParts, ",")

	podList, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods for service: %w", err)
	}

	items := make([]model.Item, 0, len(podList.Items))
	for _, pod := range podList.Items {
		items = append(items, model.Item{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Kind:      "Pod",
			Status:    string(pod.Status.Phase),
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// GetPodSelector returns a label selector string for the pods owned by the given
// parent resource. This is used to stream logs from all pods matching the selector
// via a single kubectl process, avoiding multiple OIDC auth flows.
func (c *Client) GetPodSelector(ctx context.Context, contextName, namespace, kind, name string) (string, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return "", err
	}

	var labels map[string]string

	switch kind {
	case "Deployment":
		obj, err := cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting deployment %s: %w", name, err)
		}
		if obj.Spec.Selector != nil {
			labels = obj.Spec.Selector.MatchLabels
		}
	case "StatefulSet":
		obj, err := cs.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting statefulset %s: %w", name, err)
		}
		if obj.Spec.Selector != nil {
			labels = obj.Spec.Selector.MatchLabels
		}
	case "DaemonSet":
		obj, err := cs.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting daemonset %s: %w", name, err)
		}
		if obj.Spec.Selector != nil {
			labels = obj.Spec.Selector.MatchLabels
		}
	case "Job":
		obj, err := cs.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting job %s: %w", name, err)
		}
		if obj.Spec.Selector != nil {
			labels = obj.Spec.Selector.MatchLabels
		}
	case "Service":
		obj, err := cs.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting service %s: %w", name, err)
		}
		labels = obj.Spec.Selector
	default:
		return "", nil
	}

	if len(labels) == 0 {
		return "", nil
	}

	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ","), nil
}
