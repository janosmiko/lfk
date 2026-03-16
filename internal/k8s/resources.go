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
	default:
		// For ConfigMaps, Secrets, Ingresses, PVCs - no children, show YAML preview
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
					rsNode.Children = append(rsNode.Children, &model.ResourceNode{
						Name:      pod.GetName(),
						Kind:      "Pod",
						Namespace: pod.GetNamespace(),
						Status:    extractStatus(pod.Object),
					})
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
				root.Children = append(root.Children, &model.ResourceNode{
					Name:      pod.GetName(),
					Kind:      "Pod",
					Namespace: pod.GetNamespace(),
					Status:    extractStatus(pod.Object),
				})
				break
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
	for _, c := range pod.Spec.InitContainers {
		node := &model.ResourceNode{
			Name:      c.Name,
			Kind:      "Container",
			Namespace: namespace,
			Status:    containerStatusFromPod(c.Name, pod.Status.InitContainerStatuses),
		}
		root.Children = append(root.Children, node)
	}

	// Add regular containers.
	for _, c := range pod.Spec.Containers {
		node := &model.ResourceNode{
			Name:      c.Name,
			Kind:      "Container",
			Namespace: namespace,
			Status:    containerStatusFromPod(c.Name, pod.Status.ContainerStatuses),
		}
		root.Children = append(root.Children, node)
	}

	return nil
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
