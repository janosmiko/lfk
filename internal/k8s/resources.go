package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/model"
)

func (c *Client) GetOwnedResources(ctx context.Context, contextName, namespace string, parentKind, parentName string) ([]model.Item, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	switch parentKind {
	case "Deployment":
		return c.getPodsViaReplicaSets(ctx, dynClient, namespace, parentName)
	case "StatefulSet", "DaemonSet", "Job":
		return c.getPodsByOwner(ctx, dynClient, namespace, parentKind, parentName)
	case "CronJob":
		return c.getJobsByOwner(ctx, dynClient, namespace, parentName)
	case "Service":
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
		return c.getPodsUsingPVC(ctx, dynClient, namespace, parentName)
	default:
		return nil, nil
	}
}

func (c *Client) GetContainers(ctx context.Context, contextName, namespace, podName string) ([]model.Item, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	pod, err := cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod %s: %w", podName, err)
	}

	items := make([]model.Item, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers)+len(pod.Spec.EphemeralContainers))

	for _, c := range pod.Spec.InitContainers {
		isSidecar := c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways
		item := buildContainerItem(c, pod.Status.InitContainerStatuses, true, isSidecar, false)
		items = append(items, item)
	}

	for _, c := range pod.Spec.Containers {
		item := buildContainerItem(c, pod.Status.ContainerStatuses, false, false, false)
		items = append(items, item)
	}

	// Ephemeral containers live in their own spec/status arrays and are
	// runtime-attached (kubectl debug). We project only Name/Image into a
	// corev1.Container shell because Resources and Ports are disallowed by
	// the API for ephemeral containers, and buildContainerItem reads only
	// those four fields plus statuses.
	for _, ec := range pod.Spec.EphemeralContainers {
		c := corev1.Container{Name: ec.Name, Image: ec.Image}
		item := buildContainerItem(c, pod.Status.EphemeralContainerStatuses, false, false, true)
		items = append(items, item)
	}

	return items, nil
}

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
