package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// promSvcCache caches the working namespace+service for Prometheus ProxyGet per context.
var promSvcCache sync.Map // key: contextName, value: promSvcEntry

type promSvcEntry struct {
	namespace string
	service   string
}

func (c *Client) metricsGVR(resource string) []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: resource},
		{Group: "metrics.k8s.io", Version: "v1", Resource: resource},
	}
}

// GetPodMetrics fetches CPU and memory usage for a single pod using the metrics API.
func (c *Client) GetPodMetrics(ctx context.Context, contextName, namespace, podName string) (*model.PodMetrics, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	for _, gvr := range c.metricsGVR("pods") {
		obj, err := dynClient.Resource(gvr).Namespace(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			continue
		}
		return parsePodMetrics(obj)
	}
	return nil, fmt.Errorf("fetching pod metrics: metrics API unavailable")
}

// GetPodsMetrics fetches metrics for multiple pods.
func (c *Client) GetPodsMetrics(ctx context.Context, contextName, namespace string, podNames []string) ([]model.PodMetrics, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	for _, gvr := range c.metricsGVR("pods") {
		list, err := dynClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}

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
	return nil, fmt.Errorf("listing pod metrics: metrics API unavailable")
}

// GetAllPodMetrics fetches metrics for all pods in a namespace and returns a map of pod name -> PodMetrics.
func (c *Client) GetAllPodMetrics(ctx context.Context, contextName, namespace string) (map[string]model.PodMetrics, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	for _, gvr := range c.metricsGVR("pods") {
		var list *unstructured.UnstructuredList
		if namespace == "" {
			list, err = dynClient.Resource(gvr).List(ctx, metav1.ListOptions{})
		} else {
			list, err = dynClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		}
		if err != nil {
			continue
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
	return nil, fmt.Errorf("listing pod metrics: metrics API unavailable")
}

// parsePodMetrics extracts CPU and memory from a metrics API pod object.
func parsePodMetrics(obj *unstructured.Unstructured) (*model.PodMetrics, error) {
	containers, found, err := unstructured.NestedSlice(obj.Object, "containers")
	if err != nil || !found {
		return nil, fmt.Errorf("no containers in metrics")
	}

	var totalCPU, totalMem int64
	for _, c := range containers {
		cMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		usage, ok := cMap["usage"].(map[string]any)
		if !ok {
			continue
		}
		if cpuVal := usage["cpu"]; cpuVal != nil {
			if q, err := resource.ParseQuantity(fmt.Sprintf("%v", cpuVal)); err == nil {
				totalCPU += q.MilliValue()
			}
		}
		if memVal := usage["memory"]; memVal != nil {
			if q, err := resource.ParseQuantity(fmt.Sprintf("%v", memVal)); err == nil {
				totalMem += q.Value()
			}
		}
	}

	return &model.PodMetrics{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		CPU:       totalCPU,
		Memory:    totalMem,
	}, nil
}

// resolveNodeMetricsConfig returns the NodeMetrics setting and whether Prometheus is configured
// for the given context.
func resolveNodeMetricsConfig(contextName string) (nodeMetrics string, hasPrometheus bool) {
	cfg := model.ConfigMonitoring
	if cfg == nil {
		return "", false
	}

	mc, ok := cfg[contextName]
	if !ok {
		mc, ok = cfg["_global"]
	}
	if !ok {
		return "", false
	}

	return mc.NodeMetrics, mc.Prometheus != nil
}

// GetAllNodeMetrics fetches metrics for all nodes and returns a map of node name -> PodMetrics.
// Reuses PodMetrics struct since the data shape (CPU + Memory) is the same.
func (c *Client) GetAllNodeMetrics(ctx context.Context, contextName string) (map[string]model.PodMetrics, error) {
	nodeMetrics, hasPrometheus := resolveNodeMetricsConfig(contextName)

	switch {
	case nodeMetrics == "prometheus" || (nodeMetrics == "" && hasPrometheus):
		return c.getNodeMetricsFromPrometheus(contextName)
	case nodeMetrics == "metrics-api":
		return c.getNodeMetricsFromAPI(ctx, contextName)
	default:
		return c.getNodeMetricsFromAPI(ctx, contextName)
	}
}

// getNodeMetricsFromAPI fetches node metrics from the metrics.k8s.io API (v1beta1, then v1).
func (c *Client) getNodeMetricsFromAPI(ctx context.Context, contextName string) (map[string]model.PodMetrics, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	for _, gvr := range c.metricsGVR("nodes") {
		list, err := dynClient.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}

		result := make(map[string]model.PodMetrics, len(list.Items))
		for i := range list.Items {
			obj := &list.Items[i]
			usage, found, err := unstructured.NestedMap(obj.Object, "usage")
			if err != nil || !found {
				continue
			}
			var cpu, mem int64
			if cpuVal := usage["cpu"]; cpuVal != nil {
				if q, err := resource.ParseQuantity(fmt.Sprintf("%v", cpuVal)); err == nil {
					cpu = q.MilliValue()
				}
			}
			if memVal := usage["memory"]; memVal != nil {
				if q, err := resource.ParseQuantity(fmt.Sprintf("%v", memVal)); err == nil {
					mem = q.Value()
				}
			}
			result[obj.GetName()] = model.PodMetrics{
				Name:   obj.GetName(),
				CPU:    cpu,
				Memory: mem,
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("metrics API unavailable for nodes")
}

// prometheusQueryResponse represents the JSON response from Prometheus /api/v1/query.
type prometheusQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// getNodeMetricsFromPrometheus queries Prometheus directly for node CPU and memory usage.
func (c *Client) getNodeMetricsFromPrometheus(contextName string) (map[string]model.PodMetrics, error) {
	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, fmt.Errorf("failed to get clientset: %w", err)
	}

	promNs, promSvc, promPort, _, _, _ := resolveMonitoringEndpoints(contextName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cpuQuery := `sum by (node) (rate(container_cpu_usage_seconds_total{container!=""}[3m])) * 1000`
	cpuMap, cpuErr := c.queryPrometheusNodeMetric(ctx, clientset, promNs, promSvc, promPort, cpuQuery)
	if cpuErr != nil {
		logger.Debug("Prometheus node CPU query failed", "error", cpuErr)
	}

	memQuery := `sum by (node) (container_memory_working_set_bytes{container!=""})`
	memMap, memErr := c.queryPrometheusNodeMetric(ctx, clientset, promNs, promSvc, promPort, memQuery)
	if memErr != nil {
		logger.Debug("Prometheus node memory query failed", "error", memErr)
	}

	if cpuErr != nil && memErr != nil {
		return nil, fmt.Errorf("prometheus queries failed: cpu: %w, mem: %w", cpuErr, memErr)
	}

	allNodes := make(map[string]bool)
	for k := range cpuMap {
		allNodes[k] = true
	}
	for k := range memMap {
		allNodes[k] = true
	}

	result := make(map[string]model.PodMetrics, len(allNodes))
	for node := range allNodes {
		result[node] = model.PodMetrics{
			Name:   node,
			CPU:    int64(math.Round(cpuMap[node])),
			Memory: int64(math.Round(memMap[node])),
		}
	}

	logger.Debug("Prometheus node metrics fetched", "nodeCount", len(result))
	return result, nil
}

// queryPrometheusNodeMetric runs a PromQL instant query via Kubernetes service proxy
// and returns a map of node name -> float64 value.
func (c *Client) queryPrometheusNodeMetric(ctx context.Context, cs kubernetes.Interface, namespaces, services []string, port, query string) (map[string]float64, error) {
	params := map[string]string{"query": query}

	// Check cache for a known working namespace+service.
	if cached, ok := promSvcCache.Load(cs); ok {
		entry := cached.(promSvcEntry)
		result := cs.CoreV1().Services(entry.namespace).ProxyGet("http", entry.service, port, "/api/v1/query", params)
		data, err := result.DoRaw(ctx)
		if err == nil {
			nodeMap, err := parsePrometheusNodeResponse(data)
			if err == nil {
				return nodeMap, nil
			}
		}
		// Cache entry stale, remove and fall through to discovery.
		promSvcCache.Delete(cs)
	}

	var lastErr error
	for _, ns := range namespaces {
		for _, svc := range services {
			result := cs.CoreV1().Services(ns).ProxyGet("http", svc, port, "/api/v1/query", params)
			data, err := result.DoRaw(ctx)
			if err != nil {
				lastErr = err
				continue
			}

			nodeMap, err := parsePrometheusNodeResponse(data)
			if err != nil {
				lastErr = err
				continue
			}
			promSvcCache.Store(cs, promSvcEntry{namespace: ns, service: svc})
			return nodeMap, nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no prometheus service found")
}

// parsePrometheusNodeResponse parses a Prometheus /api/v1/query JSON response
// and extracts a map of node name -> float64 value.
func parsePrometheusNodeResponse(data []byte) (map[string]float64, error) {
	var resp prometheusQueryResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing prometheus response: %w", err)
	}

	if resp.Status != "success" {
		return nil, fmt.Errorf("prometheus query returned status: %s", resp.Status)
	}

	result := make(map[string]float64, len(resp.Data.Result))
	for _, r := range resp.Data.Result {
		node := r.Metric["node"]
		if node == "" {
			for _, alt := range []string{"instance", "kubernetes_node", "nodename", "host"} {
				if v := r.Metric[alt]; v != "" {
					node = v
					break
				}
			}
		}
		if node == "" {
			continue
		}
		if len(r.Value) < 2 {
			continue
		}
		var valStr string
		if err := json.Unmarshal(r.Value[1], &valStr); err != nil {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		result[node] = val
	}

	return result, nil
}
