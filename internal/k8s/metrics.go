package k8s

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/model"
)

// GetPodMetrics fetches CPU and memory usage for a single pod using the metrics API.
func (c *Client) GetPodMetrics(ctx context.Context, contextName, namespace, podName string) (*model.PodMetrics, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}
	obj, err := dynClient.Resource(gvr).Namespace(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetching pod metrics: %w", err)
	}

	return parsePodMetrics(obj)
}

// GetPodsMetrics fetches metrics for multiple pods.
func (c *Client) GetPodsMetrics(ctx context.Context, contextName, namespace string, podNames []string) ([]model.PodMetrics, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}
	list, err := dynClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pod metrics: %w", err)
	}

	// Build lookup set for requested pod names.
	wanted := make(map[string]bool, len(podNames))
	for _, n := range podNames {
		wanted[n] = true
	}

	metrics := make([]model.PodMetrics, 0, len(list.Items))
	for i := range list.Items {
		name := list.Items[i].GetName()
		if !wanted[name] {
			continue
		}
		pm, err := parsePodMetrics(&list.Items[i])
		if err != nil {
			continue
		}
		metrics = append(metrics, *pm)
	}
	return metrics, nil
}

// GetAllPodMetrics fetches metrics for all pods in a namespace and returns a map of pod name -> PodMetrics.
func (c *Client) GetAllPodMetrics(ctx context.Context, contextName, namespace string) (map[string]model.PodMetrics, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}
	var list *unstructured.UnstructuredList
	if namespace == "" {
		list, err = dynClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	} else {
		list, err = dynClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("listing pod metrics: %w", err)
	}

	result := make(map[string]model.PodMetrics, len(list.Items))
	for i := range list.Items {
		pm, err := parsePodMetrics(&list.Items[i])
		if err != nil {
			continue
		}
		key := pm.Name
		if namespace == "" {
			key = pm.Namespace + "/" + pm.Name
		}
		result[key] = *pm
	}
	return result, nil
}

// parsePodMetrics extracts CPU and memory from a metrics API pod object.
func parsePodMetrics(obj *unstructured.Unstructured) (*model.PodMetrics, error) {
	containers, found, err := unstructured.NestedSlice(obj.Object, "containers")
	if err != nil || !found {
		return nil, fmt.Errorf("no containers in metrics")
	}

	var totalCPU, totalMem int64
	for _, c := range containers {
		cMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		usage, ok := cMap["usage"].(map[string]interface{})
		if !ok {
			continue
		}
		if cpuStr, ok := usage["cpu"].(string); ok {
			q := resource.MustParse(cpuStr)
			totalCPU += q.MilliValue()
		}
		if memStr, ok := usage["memory"].(string); ok {
			q := resource.MustParse(memStr)
			totalMem += q.Value()
		}
	}

	return &model.PodMetrics{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		CPU:       totalCPU,
		Memory:    totalMem,
	}, nil
}

// GetAllNodeMetrics fetches metrics for all nodes and returns a map of node name -> PodMetrics.
// Reuses PodMetrics struct since the data shape (CPU + Memory) is the same.
func (c *Client) GetAllNodeMetrics(ctx context.Context, contextName string) (map[string]model.PodMetrics, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}
	list, err := dynClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing node metrics: %w", err)
	}

	result := make(map[string]model.PodMetrics, len(list.Items))
	for i := range list.Items {
		obj := &list.Items[i]
		usage, found, err := unstructured.NestedMap(obj.Object, "usage")
		if err != nil || !found {
			continue
		}
		var cpu, mem int64
		if cpuStr, ok := usage["cpu"].(string); ok {
			q := resource.MustParse(cpuStr)
			cpu = q.MilliValue()
		}
		if memStr, ok := usage["memory"].(string); ok {
			q := resource.MustParse(memStr)
			mem = q.Value()
		}
		result[obj.GetName()] = model.PodMetrics{
			Name:   obj.GetName(),
			CPU:    cpu,
			Memory: mem,
		}
	}
	return result, nil
}
