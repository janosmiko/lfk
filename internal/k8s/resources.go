package k8s

import (
	"context"
	"fmt"

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
