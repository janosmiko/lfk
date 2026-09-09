package k8s

import (
	"context"

	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// A helm release is not an API object, so nothing carries an ownerReference
// back to it: membership comes from the rendered manifest in the release
// secret, and only the workload children below have an owner chain to walk.
func (c *Client) buildHelmReleaseTree(ctx context.Context, dynClient dynamic.Interface, contextName, namespace, releaseName string, root *model.ResourceNode) error {
	items, err := c.getHelmManagedResources(ctx, contextName, namespace, releaseName)
	if err != nil {
		return err
	}

	for _, item := range items {
		node := &model.ResourceNode{
			Name:      item.Name,
			Kind:      item.Kind,
			Namespace: item.Namespace,
			Status:    item.Status,
		}
		root.Children = append(root.Children, node)

		// Cluster-scoped manifest entries have no namespace and no workload
		// children, so the release namespace is only a lookup fallback here.
		childNS := item.Namespace
		if childNS == "" {
			childNS = namespace
		}
		if childErr := c.buildHelmWorkloadChildren(ctx, dynClient, childNS, item.Kind, item.Name, node); childErr != nil {
			// The release tree still renders without this workload's pods.
			logger.Warn("Resource tree: building helm workload children failed; pods skipped",
				"release", releaseName, "kind", item.Kind, "name", item.Name,
				"namespace", childNS, "error", childErr)
		}
	}

	return nil
}

func (c *Client) buildHelmWorkloadChildren(ctx context.Context, dynClient dynamic.Interface, namespace, kind, name string, node *model.ResourceNode) error {
	switch kind {
	case "Deployment":
		return c.buildDeploymentTree(ctx, dynClient, namespace, name, node)
	case "StatefulSet", "DaemonSet", "Job", "ReplicaSet":
		return c.buildPodOwnerTree(ctx, dynClient, namespace, kind, name, node)
	case "CronJob":
		return c.buildCronJobTree(ctx, dynClient, namespace, name, node)
	default:
		return nil
	}
}
