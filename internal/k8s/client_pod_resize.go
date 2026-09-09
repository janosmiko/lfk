package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

type podResizePatch struct {
	Spec podResizePatchSpec `json:"spec"`
}

type podResizePatchSpec struct {
	Containers []podResizePatchContainer `json:"containers"`
}

type podResizePatchContainer struct {
	Name      string               `json:"name"`
	Resources podResizePatchLimits `json:"resources"`
}

type podResizePatchLimits struct {
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

// ResizePodResources uses the pods/resize subresource, since a running
// Pod's spec.containers[].resources is otherwise immutable.
func (c *Client) ResizePodResources(ctx context.Context, contextName, namespace, name string, specs []model.ContainerResources) error {
	logger.Info("Resizing pod resources", "context", contextName, "namespace", namespace, "name", name)
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	body, err := json.Marshal(buildPodResizePatch(specs))
	if err != nil {
		return fmt.Errorf("building resize patch for pod %s: %w", name, err)
	}

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		ctx, name, k8stypes.StrategicMergePatchType, body, metav1.PatchOptions{FieldManager: FieldManager()}, "resize",
	)
	if err != nil {
		return fmt.Errorf("resizing pod %s: %w", name, err)
	}
	return nil
}

func buildPodResizePatch(specs []model.ContainerResources) podResizePatch {
	containers := make([]podResizePatchContainer, 0, len(specs))
	for _, spec := range specs {
		requests := map[string]string{}
		if spec.CPURequest != "" {
			requests["cpu"] = spec.CPURequest
		}
		if spec.MemRequest != "" {
			requests["memory"] = spec.MemRequest
		}
		limits := map[string]string{}
		if spec.CPULimit != "" {
			limits["cpu"] = spec.CPULimit
		}
		if spec.MemLimit != "" {
			limits["memory"] = spec.MemLimit
		}
		if len(requests) == 0 {
			requests = nil
		}
		if len(limits) == 0 {
			limits = nil
		}
		containers = append(containers, podResizePatchContainer{
			Name:      spec.Name,
			Resources: podResizePatchLimits{Requests: requests, Limits: limits},
		})
	}
	return podResizePatch{Spec: podResizePatchSpec{Containers: containers}}
}
