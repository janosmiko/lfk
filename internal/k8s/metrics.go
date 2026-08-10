package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"runtime/debug"
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

// errPrometheusUnavailableDemo is returned by every Prometheus/Alertmanager
// service-proxy query attempted on a demo Client, instead of calling
// ProxyGet at all. The fake clientset --demo mode runs on has no proxy
// subresource support: ProxyGet itself (not just the request it builds)
// panics with a nil pointer dereference, so demo mode must be turned away
// before that call rather than after it fails.
var errPrometheusUnavailableDemo = errors.New("prometheus/alertmanager queries are unavailable in demo mode")

// promSvcCache caches the working namespace+service for Prometheus ProxyGet per context.
var promSvcCache sync.Map // key: contextName, value: promSvcEntry

type promSvcEntry struct {
	namespace string
	service   string
}

// safeProxyGetRaw calls Services(ns).ProxyGet(...).DoRaw(ctx), recovering
// from any panic the underlying REST client raises. This is defense in
// depth alongside the explicit demo-mode checks that already turn queries
// away before reaching this call: a proxy failure of any kind — including
// one this package didn't anticipate — degrades to an error instead of
// crashing the TUI.
//
// The recover stays broad rather than demo-scoped: every call reaching this
// function already ran on a real-cluster path, since queryPrometheusMetric
// checks isDemo and returns errPrometheusUnavailableDemo before ever
// calling here, so a demo-scoped recover would never fire. That means a
// real panic here is always a genuine client-go failure on a live cluster,
// which is exactly the case that must not vanish silently: the recovered
// value and a stack trace are logged at Error before the panic collapses
// into a plain error.
func safeProxyGetRaw(ctx context.Context, cs kubernetes.Interface, ns, svc, port, path string, params map[string]string) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("prometheus/alertmanager proxy request panicked",
				"namespace", ns, "service", svc, "port", port, "path", path,
				"panic", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("prometheus/alertmanager proxy request panicked: %v", r)
		}
	}()
	result := cs.CoreV1().Services(ns).ProxyGet("http", svc, port, path, params)
	return result.DoRaw(ctx)
}

func (c *Client) metricsGVR(resource string) []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: resource},
		{Group: "metrics.k8s.io", Version: "v1", Resource: resource},
	}
}

// GetPodMetrics fetches CPU and memory usage for a single pod, trying the
// configured routes (Prometheus / metrics-server) in order.
func (c *Client) GetPodMetrics(ctx context.Context, contextName, namespace, podName string) (*model.PodMetrics, error) {
	return runPodMetricsRoutes(contextName,
		func() (*model.PodMetrics, error) {
			all, err := c.getAllPodMetricsFromPrometheus(ctx, contextName, namespace)
			if err != nil {
				return nil, err
			}
			if pm, ok := all[namespace+"/"+podName]; ok {
				return &pm, nil
			}
			return nil, fmt.Errorf("pod metrics for %s/%s not present in prometheus", namespace, podName)
		},
		func() (*model.PodMetrics, error) { return c.getPodMetricsFromAPI(ctx, contextName, namespace, podName) },
	)
}

// getPodMetricsFromAPI fetches a single pod's metrics from the metrics.k8s.io API.
func (c *Client) getPodMetricsFromAPI(ctx context.Context, contextName, namespace, podName string) (*model.PodMetrics, error) {
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

// GetPodsMetrics fetches metrics for multiple pods, trying the configured
// routes (Prometheus / metrics-server) in order.
func (c *Client) GetPodsMetrics(ctx context.Context, contextName, namespace string, podNames []string) ([]model.PodMetrics, error) {
	return runPodMetricsRoutes(contextName,
		func() ([]model.PodMetrics, error) {
			all, err := c.getAllPodMetricsFromPrometheus(ctx, contextName, namespace)
			if err != nil {
				return nil, err
			}
			out := make([]model.PodMetrics, 0, len(podNames))
			for _, n := range podNames {
				if pm, ok := all[namespace+"/"+n]; ok {
					out = append(out, pm)
				}
			}
			return out, nil
		},
		func() ([]model.PodMetrics, error) {
			return c.getPodsMetricsFromAPI(ctx, contextName, namespace, podNames)
		},
	)
}

// getPodsMetricsFromAPI fetches metrics for multiple pods from the metrics.k8s.io API.
func (c *Client) getPodsMetricsFromAPI(ctx context.Context, contextName, namespace string, podNames []string) ([]model.PodMetrics, error) {
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

// GetAllPodMetrics fetches metrics for all pods in a namespace (or the whole
// cluster when namespace is empty) and returns a map keyed by "namespace/name".
// It tries the configured routes (Prometheus / metrics-server) in order, so a
// cluster served only by Prometheus no longer reports "metrics API unavailable".
func (c *Client) GetAllPodMetrics(ctx context.Context, contextName, namespace string) (map[string]model.PodMetrics, error) {
	return runPodMetricsRoutes(contextName,
		func() (map[string]model.PodMetrics, error) {
			return c.getAllPodMetricsFromPrometheus(ctx, contextName, namespace)
		},
		func() (map[string]model.PodMetrics, error) {
			return c.getAllPodMetricsFromAPI(ctx, contextName, namespace)
		},
	)
}

// getAllPodMetricsFromAPI fetches all pod metrics from the metrics.k8s.io API.
func (c *Client) getAllPodMetricsFromAPI(ctx context.Context, contextName, namespace string) (map[string]model.PodMetrics, error) {
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
			// Always key by "namespace/name" so callers can build lookup
			// keys the same way regardless of whether the query was scoped
			// to a single namespace or ran across all namespaces. The
			// metrics-enrichment handler on the app side always builds
			// its keys from the item's Namespace + Name, and a previous
			// conditional key format here silently broke enrichment in
			// single-namespace mode (map had bare "pod-a" but the lookup
			// was "default/pod-a").
			key := pm.Namespace + "/" + pm.Name
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

// ContainerUsage is the per-container metrics-server reading.
// CPUMilli is millicores, MemBytes is bytes — both raw integers so
// callers can do their own snapping/aggregation.
type ContainerUsage struct {
	CPUMilli int64
	MemBytes int64
}

// parsePodMetricsByContainer reads each container's usage from a
// metrics.k8s.io PodMetrics object and returns a name → ContainerUsage
// map. Used by the right-sizing advisor's metrics-server fallback
// path, which needs per-container values (parsePodMetrics above
// collapses to pod totals).
func parsePodMetricsByContainer(obj *unstructured.Unstructured) (map[string]ContainerUsage, error) {
	containers, found, err := unstructured.NestedSlice(obj.Object, "containers")
	if err != nil || !found {
		return nil, fmt.Errorf("no containers in metrics")
	}
	out := make(map[string]ContainerUsage, len(containers))
	for _, c := range containers {
		cMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		name, _ := cMap["name"].(string)
		if name == "" {
			continue
		}
		usage, ok := cMap["usage"].(map[string]any)
		if !ok {
			continue
		}
		var u ContainerUsage
		if cpuVal := usage["cpu"]; cpuVal != nil {
			if q, err := resource.ParseQuantity(fmt.Sprintf("%v", cpuVal)); err == nil {
				u.CPUMilli = q.MilliValue()
			}
		}
		if memVal := usage["memory"]; memVal != nil {
			if q, err := resource.ParseQuantity(fmt.Sprintf("%v", memVal)); err == nil {
				u.MemBytes = q.Value()
			}
		}
		out[name] = u
	}
	return out, nil
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

// nodeMetricsRoute names a single attempt strategy for node metrics:
// either the metrics.k8s.io API (metrics-server) or a Prometheus query.
type nodeMetricsRoute int

const (
	nodeMetricsRouteAPI nodeMetricsRoute = iota
	nodeMetricsRoutePrometheus
)

func (r nodeMetricsRoute) String() string {
	if r == nodeMetricsRoutePrometheus {
		return "prometheus"
	}
	return "metrics-api"
}

// selectNodeMetricsRoutes returns the ordered list of routes to try, given
// the resolved monitoring config. The first route is the preferred path;
// the second (if present) is a fallback attempted only when the primary
// errors. This keeps `_global` Prometheus convenience working for users
// who set it once and forget, but stops a wrong default from killing
// metrics on clusters that don't match the global routing (e.g. a
// metrics-server-only EKS cluster picking up a shared `_global`
// Prometheus block, then silently rendering n/a everywhere).
func selectNodeMetricsRoutes(nodeMetrics string, hasPrometheus bool) []nodeMetricsRoute {
	switch {
	case nodeMetrics == "prometheus":
		// Explicit Prometheus: try Prometheus, fall back to metrics-api.
		return []nodeMetricsRoute{nodeMetricsRoutePrometheus, nodeMetricsRouteAPI}
	case nodeMetrics == "metrics-api":
		// Explicit metrics-api: try metrics-api, fall back to Prometheus
		// only if one is even configured (avoids attempting a guaranteed-
		// to-fail service-proxy probe).
		if hasPrometheus {
			return []nodeMetricsRoute{nodeMetricsRouteAPI, nodeMetricsRoutePrometheus}
		}
		return []nodeMetricsRoute{nodeMetricsRouteAPI}
	case nodeMetrics == "" && hasPrometheus:
		// Implicit Prometheus (typically from a shared `_global` entry):
		// try Prometheus, fall back to metrics-api. The fallback is the
		// fix for clusters that inherit a global Prometheus pointer but
		// only actually have metrics-server.
		return []nodeMetricsRoute{nodeMetricsRoutePrometheus, nodeMetricsRouteAPI}
	default:
		// Nothing configured: metrics-api only.
		return []nodeMetricsRoute{nodeMetricsRouteAPI}
	}
}

// GetAllNodeMetrics fetches metrics for all nodes and returns a map of node name -> PodMetrics.
// Reuses PodMetrics struct since the data shape (CPU + Memory) is the same.
//
// Routing follows selectNodeMetricsRoutes: the configured path is tried
// first, and the other path is attempted as a fallback when the primary
// errors. Each attempted route is logged at Debug, and any demotion to
// the fallback is logged at Warn so users can self-diagnose missing
// metrics from a single log read.
func (c *Client) GetAllNodeMetrics(ctx context.Context, contextName string) (map[string]model.PodMetrics, error) {
	nodeMetrics, hasPrometheus := resolveNodeMetricsConfig(contextName)
	routes := selectNodeMetricsRoutes(nodeMetrics, hasPrometheus)

	var lastErr error
	for i, route := range routes {
		logger.Debug("node metrics route", "context", contextName, "route", route.String(), "attempt", i+1)
		var (
			result map[string]model.PodMetrics
			err    error
		)
		switch route {
		case nodeMetricsRoutePrometheus:
			result, err = c.getNodeMetricsFromPrometheus(contextName)
		default:
			result, err = c.getNodeMetricsFromAPI(ctx, contextName)
		}
		if err == nil {
			return result, nil
		}
		lastErr = err
		if i+1 < len(routes) {
			logger.Warn("node metrics route failed, trying fallback",
				"context", contextName, "failed_route", route.String(),
				"fallback_route", routes[i+1].String(), "error", err)
		}
	}
	return nil, lastErr
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
	cpuMap, cpuErr := c.queryPrometheusNodeMetric(ctx, contextName, clientset, promNs, promSvc, promPort, cpuQuery)
	if cpuErr != nil {
		logger.Debug("Prometheus node CPU query failed", "error", cpuErr)
	}

	memQuery := `sum by (node) (container_memory_working_set_bytes{container!=""})`
	memMap, memErr := c.queryPrometheusNodeMetric(ctx, contextName, clientset, promNs, promSvc, promPort, memQuery)
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
func (c *Client) queryPrometheusNodeMetric(ctx context.Context, contextName string, cs kubernetes.Interface, namespaces, services []string, port, query string) (map[string]float64, error) {
	return queryPrometheusMetric(ctx, contextName, cs, c.demo, namespaces, services, port, query, parsePrometheusNodeResponse)
}

// queryPrometheusMetric runs a PromQL instant query via Kubernetes service proxy
// and parses the response with the caller-supplied parser. A free function
// (rather than a method) so it can be generic over the parsed result type T --
// callers needing a different shape (e.g. node uptime, which splits into
// name-keyed and address-keyed maps) reuse the same service-discovery and
// promSvcCache logic instead of duplicating it. isDemo is threaded in
// (rather than accessed via a *Client) for the same reason: it's a plain
// value the caller already has, and keeps this function testable without a
// full Client.
func queryPrometheusMetric[T any](ctx context.Context, contextName string, cs kubernetes.Interface, isDemo bool, namespaces, services []string, port, query string, parse func([]byte) (T, error)) (T, error) {
	var zero T
	if isDemo {
		return zero, errPrometheusUnavailableDemo
	}
	params := map[string]string{"query": query}

	// Check cache for a known working namespace+service. Keyed by contextName
	// (not the clientset): clientsetForContext builds a fresh clientset per
	// call, so a clientset key would never hit and re-run service discovery
	// on every poll.
	if cached, ok := promSvcCache.Load(contextName); ok {
		entry := cached.(promSvcEntry)
		data, err := safeProxyGetRaw(ctx, cs, entry.namespace, entry.service, port, "/api/v1/query", params)
		if err == nil {
			parsed, err := parse(data)
			if err == nil {
				return parsed, nil
			}
		}
		// Cache entry stale, remove and fall through to discovery.
		promSvcCache.Delete(contextName)
	}

	var lastErr error
	for _, ns := range namespaces {
		for _, svc := range services {
			data, err := safeProxyGetRaw(ctx, cs, ns, svc, port, "/api/v1/query", params)
			if err != nil {
				lastErr = err
				continue
			}

			parsed, err := parse(data)
			if err != nil {
				lastErr = err
				continue
			}
			promSvcCache.Store(contextName, promSvcEntry{namespace: ns, service: svc})
			return parsed, nil
		}
	}

	if lastErr != nil {
		return zero, lastErr
	}
	return zero, fmt.Errorf("no prometheus service found")
}

// parsePrometheusVector parses a Prometheus /api/v1/query instant-vector
// response into a map keyed by keyFunc(metricLabels). Samples for which keyFunc
// returns ok=false, or whose scalar value can't be parsed, are skipped. Shared
// by the node, pod, and container readers, which differ only in how the result
// key is derived from the metric's labels.
func parsePrometheusVector(data []byte, keyFunc func(labels map[string]string) (string, bool)) (map[string]float64, error) {
	var resp prometheusQueryResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing prometheus response: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("prometheus query returned status: %s", resp.Status)
	}

	result := make(map[string]float64, len(resp.Data.Result))
	for _, r := range resp.Data.Result {
		key, ok := keyFunc(r.Metric)
		if !ok {
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
		result[key] = val
	}
	return result, nil
}

// parsePrometheusNodeResponse extracts a node name -> value map. The node name
// comes from the "node" label, falling back to common alternates emitted by
// different exporters.
func parsePrometheusNodeResponse(data []byte) (map[string]float64, error) {
	return parsePrometheusVector(data, func(labels map[string]string) (string, bool) {
		node := labels["node"]
		if node == "" {
			for _, alt := range []string{"instance", "kubernetes_node", "nodename", "host"} {
				if v := labels[alt]; v != "" {
					node = v
					break
				}
			}
		}
		return node, node != ""
	})
}
