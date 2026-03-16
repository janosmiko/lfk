// Package k8s provides Kubernetes API access for the TUI application.
package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/model"
)

// Client wraps Kubernetes API access.
type Client struct {
	rawConfig    api.Config
	loadingRules *clientcmd.ClientConfigLoadingRules
}

// RBACCheck represents a single permission check result.
type RBACCheck struct {
	Verb    string
	Allowed bool
}

// QuotaInfo holds resource quota data for a single ResourceQuota object.
type QuotaInfo struct {
	Name      string
	Namespace string
	Resources []QuotaResource
}

// QuotaResource holds usage data for a single resource within a quota.
type QuotaResource struct {
	Name    string  // e.g. "cpu", "memory", "pods", "services"
	Hard    string  // limit
	Used    string  // current usage
	Percent float64 // usage percentage (0-100)
}

// DeploymentRevision represents a deployment revision history entry.
type DeploymentRevision struct {
	Revision  int64
	Name      string
	Replicas  int32
	Images    []string
	CreatedAt time.Time
}

// NewClient creates a new Kubernetes client, loading configs from:
// 1. KUBECONFIG env var
// 2. ~/.kube/config
// 3. All files in ~/.kube/config.d/ (recursively)
func NewClient() (*Client, error) {
	kubeconfigPaths := buildKubeconfigPaths()

	loadingRules := &clientcmd.ClientConfigLoadingRules{
		Precedence: kubeconfigPaths,
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	return &Client{
		rawConfig:    rawConfig,
		loadingRules: loadingRules,
	}, nil
}

// KubeconfigPaths returns the colon-separated kubeconfig paths used by this client.
func (c *Client) KubeconfigPaths() string {
	return strings.Join(c.loadingRules.Precedence, ":")
}

// buildKubeconfigPaths assembles the list of kubeconfig file paths to load.
func buildKubeconfigPaths() []string {
	var paths []string

	// KUBECONFIG env var (colon-separated on unix).
	if env := os.Getenv("KUBECONFIG"); env != "" {
		paths = append(paths, filepath.SplitList(env)...)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		// Default kubeconfig.
		defaultPath := filepath.Join(home, ".kube", "config")
		if !containsPath(paths, defaultPath) {
			paths = append(paths, defaultPath)
		}

		// config.d directory - recursively find all files.
		configDir := filepath.Join(home, ".kube", "config.d")
		_ = filepath.WalkDir(configDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // skip errors (dir might not exist)
			}
			if !d.IsDir() {
				paths = append(paths, path)
			}
			return nil
		})
	}

	return paths
}

func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target {
			return true
		}
	}
	return false
}

// GetContexts returns all available kube contexts.
func (c *Client) GetContexts() ([]model.Item, error) {
	items := make([]model.Item, 0, len(c.rawConfig.Contexts))
	for name := range c.rawConfig.Contexts {
		status := ""
		if name == c.rawConfig.CurrentContext {
			status = "current"
		}
		items = append(items, model.Item{Name: name, Status: status})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// CurrentContext returns the current context name from the kubeconfig.
func (c *Client) CurrentContext() string {
	return c.rawConfig.CurrentContext
}

// DefaultNamespace returns the namespace configured for the given context,
// falling back to "default" if none is set.
func (c *Client) DefaultNamespace(contextName string) string {
	if ctx, ok := c.rawConfig.Contexts[contextName]; ok && ctx.Namespace != "" {
		return ctx.Namespace
	}
	return "default"
}

// GetNamespaces returns namespaces for the given context.
func (c *Client) GetNamespaces(ctx context.Context, contextName string) ([]model.Item, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	nsList, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}

	items := make([]model.Item, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		items = append(items, model.Item{Name: ns.Name, Status: string(ns.Status.Phase)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// GetResources lists resources of a given type. For namespaced resources it
// scopes to the given namespace; for cluster-scoped resources it lists globally.
// When namespace is empty and the resource is namespaced, it lists across all namespaces.
func (c *Client) GetResources(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry) ([]model.Item, error) {
	// Special handling for virtual resource types.
	if rt.APIGroup == "_helm" && rt.Resource == "releases" {
		return c.GetHelmReleases(ctx, contextName, namespace)
	}
	if rt.APIGroup == "_portforward" {
		return nil, nil // port forwards are managed locally, not via K8s API
	}

	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}

	var lister dynamic.ResourceInterface
	if rt.Namespaced {
		lister = dynClient.Resource(gvr).Namespace(namespace) // empty string = all namespaces
	} else {
		lister = dynClient.Resource(gvr)
	}

	list, err := lister.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", rt.Resource, err)
	}

	items := make([]model.Item, 0, len(list.Items))
	for _, item := range list.Items {
		ti := model.Item{
			Name:   item.GetName(),
			Kind:   item.GetKind(),
			Status: extractStatus(item.Object),
		}

		// Populate namespace when listing across all namespaces.
		if namespace == "" && rt.Namespaced {
			ti.Namespace = item.GetNamespace()
		}

		// Populate Age from creationTimestamp.
		creationTS := item.GetCreationTimestamp()
		if !creationTS.IsZero() {
			ti.CreatedAt = creationTS.Time
			ti.Age = formatAge(time.Since(creationTS.Time))
		}

		// Populate Ready and Restarts based on kind.
		populateResourceDetails(&ti, item.Object, rt.Kind)

		// Extract owner references for navigation.
		if metadata, ok := item.Object["metadata"].(map[string]interface{}); ok {
			if ownerRefs, ok := metadata["ownerReferences"].([]interface{}); ok {
				for i, ref := range ownerRefs {
					if refMap, ok := ref.(map[string]interface{}); ok {
						kind, _ := refMap["kind"].(string)
						name, _ := refMap["name"].(string)
						apiVersion, _ := refMap["apiVersion"].(string)
						if kind != "" && name != "" {
							ti.Columns = append(ti.Columns, model.KeyValue{
								Key:   fmt.Sprintf("owner:%d", i),
								Value: apiVersion + "||" + kind + "||" + name,
							})
						}
					}
				}
			}
		}

		items = append(items, ti)
	}
	// Sort events by time (newest first); all other resources alphabetically by name.
	if rt.Kind == "Event" {
		sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	} else {
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	}
	return items, nil
}

// populateResourceDetails fills in Ready and Restarts fields for specific resource kinds.
func populateResourceDetails(ti *model.Item, obj map[string]interface{}, kind string) {
	status, _ := obj["status"].(map[string]interface{})
	spec, _ := obj["spec"].(map[string]interface{})

	switch kind {
	case "Pod":
		if status == nil {
			return
		}
		containerStatuses, _ := status["containerStatuses"].([]interface{})
		totalContainers := len(containerStatuses)
		if containers, ok := spec["containers"].([]interface{}); ok {
			totalContainers = len(containers)
		}
		readyCount := 0
		restartCount := int64(0)
		for _, cs := range containerStatuses {
			csMap, ok := cs.(map[string]interface{})
			if !ok {
				continue
			}
			if ready, ok := csMap["ready"].(bool); ok && ready {
				readyCount++
			}
			if rc, ok := csMap["restartCount"].(int64); ok {
				restartCount += rc
			} else if rcf, ok := csMap["restartCount"].(float64); ok {
				restartCount += int64(rcf)
			}
		}
		ti.Ready = fmt.Sprintf("%d/%d", readyCount, totalContainers)
		ti.Restarts = fmt.Sprintf("%d", restartCount)

		// Find the most recent restart time from container lastState.
		var lastRestart time.Time
		for _, cs := range containerStatuses {
			csMap, ok := cs.(map[string]interface{})
			if !ok {
				continue
			}
			lastState, _ := csMap["lastState"].(map[string]interface{})
			if lastState == nil {
				continue
			}
			if terminated, ok := lastState["terminated"].(map[string]interface{}); ok {
				if finishedAt, ok := terminated["finishedAt"].(string); ok {
					if t, err := time.Parse(time.RFC3339, finishedAt); err == nil {
						if t.After(lastRestart) {
							lastRestart = t
						}
					}
				}
			}
		}
		ti.LastRestartAt = lastRestart

		// Override status based on container readiness.
		// Succeeded pods stay green even with unready containers.
		if ti.Status != "Succeeded" && readyCount < totalContainers && totalContainers > 0 {
			// Extract reason from container statuses.
			reason := extractContainerNotReadyReason(containerStatuses)
			if reason != "" {
				ti.Status = reason
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: reason})
			} else if ti.Status == "Running" {
				ti.Status = "NotReady"
			}
		}

		// Resource requests/limits from container specs.
		if containers, ok := spec["containers"].([]interface{}); ok {
			cpuReq, cpuLim, memReq, memLim := extractContainerResources(containers)
			addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
		}

		// Additional columns for preview.
		if qos, ok := status["qosClass"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "QoS", Value: qos})
		}
		if sa, ok := spec["serviceAccountName"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Service Account", Value: sa})
		}
		if podIP, ok := status["podIP"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Pod IP", Value: podIP})
		}
		if containers, ok := spec["containers"].([]interface{}); ok {
			var images []string
			for _, c := range containers {
				if cMap, ok := c.(map[string]interface{}); ok {
					if img, ok := cMap["image"].(string); ok {
						images = append(images, img)
					}
				}
			}
			if len(images) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Images", Value: strings.Join(images, ", ")})
			}
		}
		// Priority class.
		if pc, ok := spec["priorityClassName"].(string); ok && pc != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Priority Class", Value: pc})
		}
		// Node at the end (lower priority in table view).
		if nodeName, ok := spec["nodeName"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Node", Value: nodeName})
		}

	case "Deployment":
		if status == nil || spec == nil {
			return
		}
		var specReplicas int64 = 1
		if r, ok := spec["replicas"].(int64); ok {
			specReplicas = r
		} else if r, ok := spec["replicas"].(float64); ok {
			specReplicas = int64(r)
		}
		var readyReplicas int64
		if r, ok := status["readyReplicas"].(int64); ok {
			readyReplicas = r
		} else if r, ok := status["readyReplicas"].(float64); ok {
			readyReplicas = int64(r)
		}
		ti.Ready = fmt.Sprintf("%d/%d", readyReplicas, specReplicas)
		// Additional columns.
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Replicas", Value: fmt.Sprintf("%d", specReplicas)})
		if strategy, ok := spec["strategy"].(map[string]interface{}); ok {
			if t, ok := strategy["type"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Strategy", Value: t})
			}
		}
		if updated, ok := status["updatedReplicas"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Up-to-date", Value: fmt.Sprintf("%d", int64(updated))})
		}
		if avail, ok := status["availableReplicas"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Available", Value: fmt.Sprintf("%d", int64(avail))})
		}
		// Aggregated resource requests/limits (per-pod from template).
		cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)
		addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
		populateContainerImages(ti, spec)

	case "StatefulSet":
		if status == nil || spec == nil {
			return
		}
		var specReplicas int64 = 1
		if r, ok := spec["replicas"].(int64); ok {
			specReplicas = r
		} else if r, ok := spec["replicas"].(float64); ok {
			specReplicas = int64(r)
		}
		var readyReplicas int64
		if r, ok := status["readyReplicas"].(int64); ok {
			readyReplicas = r
		} else if r, ok := status["readyReplicas"].(float64); ok {
			readyReplicas = int64(r)
		}
		ti.Ready = fmt.Sprintf("%d/%d", readyReplicas, specReplicas)
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Replicas", Value: fmt.Sprintf("%d", specReplicas)})
		// Aggregated resource requests/limits (per-pod from template).
		cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)
		addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
		populateContainerImages(ti, spec)

	case "DaemonSet":
		if status == nil {
			return
		}
		var desired, ready int64
		if d, ok := status["desiredNumberScheduled"].(int64); ok {
			desired = d
		} else if d, ok := status["desiredNumberScheduled"].(float64); ok {
			desired = int64(d)
		}
		if r, ok := status["numberReady"].(int64); ok {
			ready = r
		} else if r, ok := status["numberReady"].(float64); ok {
			ready = int64(r)
		}
		ti.Ready = fmt.Sprintf("%d/%d", ready, desired)
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Desired", Value: fmt.Sprintf("%d", desired)})
		// Per-pod resource requests/limits from template.
		if spec != nil {
			cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)
			addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
		}

	case "ReplicaSet":
		if status == nil || spec == nil {
			return
		}
		var specReplicas int64
		if r, ok := spec["replicas"].(int64); ok {
			specReplicas = r
		} else if r, ok := spec["replicas"].(float64); ok {
			specReplicas = int64(r)
		}
		var readyReplicas int64
		if r, ok := status["readyReplicas"].(int64); ok {
			readyReplicas = r
		} else if r, ok := status["readyReplicas"].(float64); ok {
			readyReplicas = int64(r)
		}
		ti.Ready = fmt.Sprintf("%d/%d", readyReplicas, specReplicas)

	case "Service":
		if spec == nil {
			return
		}
		if svcType, ok := spec["type"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Type", Value: svcType})
		}
		if clusterIP, ok := spec["clusterIP"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Cluster IP", Value: clusterIP})
		}
		if ports, ok := spec["ports"].([]interface{}); ok {
			var portStrs []string
			for _, p := range ports {
				if pMap, ok := p.(map[string]interface{}); ok {
					port := getInt(pMap, "port")
					proto, _ := pMap["protocol"].(string)
					s := fmt.Sprintf("%d/%s", port, proto)
					if tp := getInt(pMap, "targetPort"); tp > 0 && tp != port {
						s = fmt.Sprintf("%d→%d/%s", port, tp, proto)
					}
					portStrs = append(portStrs, s)
				}
			}
			if len(portStrs) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ports", Value: strings.Join(portStrs, ", ")})
			}
		}
		// External IPs from spec.
		if extIPs, ok := spec["externalIPs"].([]interface{}); ok && len(extIPs) > 0 {
			var ips []string
			for _, ip := range extIPs {
				if s, ok := ip.(string); ok {
					ips = append(ips, s)
				}
			}
			if len(ips) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "External IPs", Value: strings.Join(ips, ", ")})
			}
		}
		// External IP from LoadBalancer status.
		if status != nil {
			if lb, ok := status["loadBalancer"].(map[string]interface{}); ok {
				if ingress, ok := lb["ingress"].([]interface{}); ok {
					var addrs []string
					for _, i := range ingress {
						if iMap, ok := i.(map[string]interface{}); ok {
							if ip, ok := iMap["ip"].(string); ok {
								addrs = append(addrs, ip)
							} else if host, ok := iMap["hostname"].(string); ok {
								addrs = append(addrs, host)
							}
						}
					}
					if len(addrs) > 0 {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "External Address", Value: strings.Join(addrs, ", ")})
					}
				}
			}
		}
		if selector, ok := spec["selector"].(map[string]interface{}); ok {
			var parts []string
			for k, v := range selector {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			}
			sort.Strings(parts)
			if len(parts) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Selector", Value: strings.Join(parts, ", ")})
			}
		}
		if spec["sessionAffinity"] != nil {
			if sa, ok := spec["sessionAffinity"].(string); ok && sa != "None" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Session Affinity", Value: sa})
			}
		}

	case "Ingress":
		if spec == nil {
			return
		}
		// Ingress class.
		if ic, ok := spec["ingressClassName"].(string); ok && ic != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ingress Class", Value: ic})
		}
		if rules, ok := spec["rules"].([]interface{}); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Rules", Value: fmt.Sprintf("%d", len(rules))})
			var hosts []string
			for _, r := range rules {
				if rMap, ok := r.(map[string]interface{}); ok {
					if host, ok := rMap["host"].(string); ok {
						hosts = append(hosts, host)
					}
				}
			}
			if len(hosts) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Hosts", Value: strings.Join(hosts, ", ")})
			}
		}
		// Default backend.
		if defBackend, ok := spec["defaultBackend"].(map[string]interface{}); ok {
			if svc, ok := defBackend["service"].(map[string]interface{}); ok {
				svcName, _ := svc["name"].(string)
				if port, ok := svc["port"].(map[string]interface{}); ok {
					if num, ok := port["number"].(float64); ok {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Default Backend", Value: fmt.Sprintf("%s:%d", svcName, int64(num))})
					} else if name, ok := port["name"].(string); ok {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Default Backend", Value: fmt.Sprintf("%s:%s", svcName, name)})
					}
				} else if svcName != "" {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Default Backend", Value: svcName})
				}
			}
		}
		// TLS hosts.
		var tlsHostSet map[string]bool
		if tls, ok := spec["tls"].([]interface{}); ok && len(tls) > 0 {
			tlsHostSet = make(map[string]bool)
			var tlsHosts []string
			for _, t := range tls {
				if tMap, ok := t.(map[string]interface{}); ok {
					if hosts, ok := tMap["hosts"].([]interface{}); ok {
						for _, h := range hosts {
							if s, ok := h.(string); ok {
								tlsHosts = append(tlsHosts, s)
								tlsHostSet[s] = true
							}
						}
					}
				}
			}
			if len(tlsHosts) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "TLS Hosts", Value: strings.Join(tlsHosts, ", ")})
			}
		}
		// Build a URL from the first rule's host and path for "Open in Browser".
		if rules, ok := spec["rules"].([]interface{}); ok && len(rules) > 0 {
			if firstRule, ok := rules[0].(map[string]interface{}); ok {
				if host, ok := firstRule["host"].(string); ok && host != "" {
					scheme := "http"
					if tlsHostSet[host] {
						scheme = "https"
					}
					path := ""
					if httpBlock, ok := firstRule["http"].(map[string]interface{}); ok {
						if paths, ok := httpBlock["paths"].([]interface{}); ok && len(paths) > 0 {
							if firstPath, ok := paths[0].(map[string]interface{}); ok {
								if p, ok := firstPath["path"].(string); ok && p != "" && p != "/" {
									path = p
								}
							}
						}
					}
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "__ingress_url", Value: scheme + "://" + host + path})
				}
			}
		}
		if status != nil {
			if lb, ok := status["loadBalancer"].(map[string]interface{}); ok {
				if ingress, ok := lb["ingress"].([]interface{}); ok {
					var addrs []string
					for _, i := range ingress {
						if iMap, ok := i.(map[string]interface{}); ok {
							if ip, ok := iMap["ip"].(string); ok {
								addrs = append(addrs, ip)
							} else if host, ok := iMap["hostname"].(string); ok {
								addrs = append(addrs, host)
							}
						}
					}
					if len(addrs) > 0 {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Address", Value: strings.Join(addrs, ", ")})
					}
				}
			}
		}

	case "ConfigMap":
		if data, ok := obj["data"].(map[string]interface{}); ok {
			var keys []string
			for k := range data {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			// Store ConfigMap data values with "data:" prefix for preview display.
			for _, k := range keys {
				if v, ok := data[k].(string); ok {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "data:" + k, Value: v})
				}
			}
		}

	case "Secret":
		if data, ok := obj["data"].(map[string]interface{}); ok {
			var keys []string
			for k := range data {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			// Store decoded secret values with "secret:" prefix for conditional display.
			for _, k := range keys {
				if encoded, ok := data[k].(string); ok {
					decoded, err := base64.StdEncoding.DecodeString(encoded)
					if err == nil {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "secret:" + k, Value: string(decoded)})
					}
				}
			}
		}
		if sType, ok := obj["type"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Type", Value: sType})
		}

	case "Node":
		// Extract role from labels.
		if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
			if labels, ok := metadata["labels"].(map[string]interface{}); ok {
				var roles []string
				for k := range labels {
					if strings.HasPrefix(k, "node-role.kubernetes.io/") {
						role := strings.TrimPrefix(k, "node-role.kubernetes.io/")
						if role != "" {
							roles = append(roles, role)
						}
					}
				}
				if len(roles) > 0 {
					sort.Strings(roles)
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Role", Value: strings.Join(roles, ",")})
				}
			}
		}

		if status != nil {
			if addrs, ok := status["addresses"].([]interface{}); ok {
				for _, a := range addrs {
					if aMap, ok := a.(map[string]interface{}); ok {
						addrType, _ := aMap["type"].(string)
						addr, _ := aMap["address"].(string)
						if addrType != "" && addr != "" {
							ti.Columns = append(ti.Columns, model.KeyValue{Key: addrType, Value: addr})
						}
					}
				}
			}
			// Add allocatable CPU/Memory as hidden data columns for metrics enrichment.
			if alloc, ok := status["allocatable"].(map[string]interface{}); ok {
				if cpu, ok := alloc["cpu"].(string); ok {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "CPU Alloc", Value: cpu})
				}
				if mem, ok := alloc["memory"].(string); ok {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Mem Alloc", Value: mem})
				}
			}
			if nodeInfo, ok := status["nodeInfo"].(map[string]interface{}); ok {
				if v, ok := nodeInfo["kubeletVersion"].(string); ok {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Version", Value: v})
				}
				if v, ok := nodeInfo["osImage"].(string); ok {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "OS", Value: v})
				}
				if v, ok := nodeInfo["containerRuntimeVersion"].(string); ok {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Runtime", Value: v})
				}
			}
		}

		// Extract taints from spec.
		if spec != nil {
			if taints, ok := spec["taints"].([]interface{}); ok && len(taints) > 0 {
				var taintStrs []string
				for _, t := range taints {
					if tMap, ok := t.(map[string]interface{}); ok {
						key, _ := tMap["key"].(string)
						value, _ := tMap["value"].(string)
						effect, _ := tMap["effect"].(string)
						taint := key
						if value != "" {
							taint += "=" + value
						}
						taint += ":" + effect
						taintStrs = append(taintStrs, taint)
					}
				}
				if len(taintStrs) > 0 {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Taints", Value: strings.Join(taintStrs, ", ")})
				}
			}
		}

	case "PersistentVolumeClaim":
		// Phase/status.
		if status != nil {
			if phase, ok := status["phase"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Status", Value: phase})
				ti.Status = phase
			}
			// Actual capacity from status (may differ from requested).
			if cap, ok := status["capacity"].(map[string]interface{}); ok {
				if storage, ok := cap["storage"].(string); ok {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Capacity", Value: storage})
				}
			}
		}
		if spec != nil {
			// Requested storage (show if no status capacity yet).
			if res, ok := spec["resources"].(map[string]interface{}); ok {
				if req, ok := res["requests"].(map[string]interface{}); ok {
					if storage, ok := req["storage"].(string); ok {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Request", Value: storage})
					}
				}
			}
			// Volume name.
			if vol, ok := spec["volumeName"].(string); ok && vol != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Volume", Value: vol})
			}
			if am, ok := spec["accessModes"].([]interface{}); ok {
				var modes []string
				for _, m := range am {
					if s, ok := m.(string); ok {
						modes = append(modes, s)
					}
				}
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Access Modes", Value: strings.Join(modes, ", ")})
			}
			if sc, ok := spec["storageClassName"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Storage Class", Value: sc})
			}
			if vm, ok := spec["volumeMode"].(string); ok && vm != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Volume Mode", Value: vm})
			}
		}

	case "CronJob":
		if spec != nil {
			if sched, ok := spec["schedule"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Schedule", Value: sched})
			}
			if suspend, ok := spec["suspend"].(bool); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Suspend", Value: fmt.Sprintf("%v", suspend)})
			}
		}
		if status != nil {
			if lastSchedule, ok := status["lastScheduleTime"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Last Schedule", Value: lastSchedule})
			}
		}

	case "Job":
		if status != nil {
			if succeeded, ok := status["succeeded"].(float64); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Succeeded", Value: fmt.Sprintf("%d", int64(succeeded))})
			}
			if failed, ok := status["failed"].(float64); ok && failed > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Failed", Value: fmt.Sprintf("%d", int64(failed))})
			}
		}
		if spec != nil {
			if completions, ok := spec["completions"].(float64); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Completions", Value: fmt.Sprintf("%d", int64(completions))})
			}
		}

	case "HorizontalPodAutoscaler":
		// Set Ready field to show current/desired replicas in the list table.
		if status != nil {
			var currentR, desiredR int64
			if cr, ok := status["currentReplicas"].(float64); ok {
				currentR = int64(cr)
			}
			if dr, ok := status["desiredReplicas"].(float64); ok {
				desiredR = int64(dr)
			}
			// Show min/max from spec for context.
			var minR, maxR int64
			if spec != nil {
				if mr, ok := spec["minReplicas"].(float64); ok {
					minR = int64(mr)
				}
				if mr, ok := spec["maxReplicas"].(float64); ok {
					maxR = int64(mr)
				}
			}
			ti.Ready = fmt.Sprintf("%d/%d (%d-%d)", currentR, desiredR, minR, maxR)
		}
		if spec != nil {
			// Target reference.
			if scaleTargetRef, ok := spec["scaleTargetRef"].(map[string]interface{}); ok {
				refKind, _ := scaleTargetRef["kind"].(string)
				refName, _ := scaleTargetRef["name"].(string)
				if refKind != "" && refName != "" {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Target", Value: refKind + "/" + refName})
				}
			}
			if minR, ok := spec["minReplicas"].(float64); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Min Replicas", Value: fmt.Sprintf("%d", int64(minR))})
			}
			if maxR, ok := spec["maxReplicas"].(float64); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Max Replicas", Value: fmt.Sprintf("%d", int64(maxR))})
			}
			// Metrics from spec (target values).
			if metrics, ok := spec["metrics"].([]interface{}); ok {
				for _, m := range metrics {
					mMap, ok := m.(map[string]interface{})
					if !ok {
						continue
					}
					mType, _ := mMap["type"].(string)
					switch mType {
					case "Resource":
						if res, ok := mMap["resource"].(map[string]interface{}); ok {
							resName, _ := res["name"].(string)
							if target, ok := res["target"].(map[string]interface{}); ok {
								targetType, _ := target["type"].(string)
								switch targetType {
								case "Utilization":
									if avg, ok := target["averageUtilization"].(float64); ok {
										ti.Columns = append(ti.Columns, model.KeyValue{
											Key:   fmt.Sprintf("Target %s", strings.ToUpper(resName[:1])+resName[1:]),
											Value: fmt.Sprintf("%d%%", int64(avg)),
										})
									}
								case "AverageValue":
									if avg, ok := target["averageValue"].(string); ok {
										ti.Columns = append(ti.Columns, model.KeyValue{
											Key:   fmt.Sprintf("Target %s", strings.ToUpper(resName[:1])+resName[1:]),
											Value: avg,
										})
									}
								}
							}
						}
					case "Pods":
						if pods, ok := mMap["pods"].(map[string]interface{}); ok {
							metricName := ""
							if mn, ok := pods["metric"].(map[string]interface{}); ok {
								metricName, _ = mn["name"].(string)
							}
							if target, ok := pods["target"].(map[string]interface{}); ok {
								if avg, ok := target["averageValue"].(string); ok && metricName != "" {
									ti.Columns = append(ti.Columns, model.KeyValue{
										Key:   fmt.Sprintf("Target %s", metricName),
										Value: avg,
									})
								}
							}
						}
					case "Object":
						if object, ok := mMap["object"].(map[string]interface{}); ok {
							metricName := ""
							if mn, ok := object["metric"].(map[string]interface{}); ok {
								metricName, _ = mn["name"].(string)
							}
							if target, ok := object["target"].(map[string]interface{}); ok {
								if val, ok := target["value"].(string); ok && metricName != "" {
									ti.Columns = append(ti.Columns, model.KeyValue{
										Key:   fmt.Sprintf("Target %s", metricName),
										Value: val,
									})
								}
							}
						}
					}
				}
			}
		}
		if status != nil {
			if current, ok := status["currentReplicas"].(float64); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Current Replicas", Value: fmt.Sprintf("%d", int64(current))})
			}
			if desired, ok := status["desiredReplicas"].(float64); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Desired Replicas", Value: fmt.Sprintf("%d", int64(desired))})
			}
			// Current metrics from status.
			if currentMetrics, ok := status["currentMetrics"].([]interface{}); ok {
				for _, m := range currentMetrics {
					mMap, ok := m.(map[string]interface{})
					if !ok {
						continue
					}
					mType, _ := mMap["type"].(string)
					switch mType {
					case "Resource":
						if res, ok := mMap["resource"].(map[string]interface{}); ok {
							resName, _ := res["name"].(string)
							if current, ok := res["current"].(map[string]interface{}); ok {
								if avg, ok := current["averageUtilization"].(float64); ok {
									ti.Columns = append(ti.Columns, model.KeyValue{
										Key:   fmt.Sprintf("Current %s", strings.ToUpper(resName[:1])+resName[1:]),
										Value: fmt.Sprintf("%d%%", int64(avg)),
									})
								} else if avgVal, ok := current["averageValue"].(string); ok {
									ti.Columns = append(ti.Columns, model.KeyValue{
										Key:   fmt.Sprintf("Current %s", strings.ToUpper(resName[:1])+resName[1:]),
										Value: avgVal,
									})
								}
							}
						}
					case "Pods":
						if pods, ok := mMap["pods"].(map[string]interface{}); ok {
							metricName := ""
							if mn, ok := pods["metric"].(map[string]interface{}); ok {
								metricName, _ = mn["name"].(string)
							}
							if current, ok := pods["current"].(map[string]interface{}); ok {
								if avg, ok := current["averageValue"].(string); ok && metricName != "" {
									ti.Columns = append(ti.Columns, model.KeyValue{
										Key:   fmt.Sprintf("Current %s", metricName),
										Value: avg,
									})
								}
							}
						}
					}
				}
			}
			// Conditions summary.
			if conditions, ok := status["conditions"].([]interface{}); ok {
				for _, c := range conditions {
					cMap, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					cType, _ := cMap["type"].(string)
					cStatus, _ := cMap["status"].(string)
					if cType == "ScalingLimited" && cStatus == "True" {
						msg, _ := cMap["message"].(string)
						if msg != "" {
							ti.Columns = append(ti.Columns, model.KeyValue{Key: "Scaling Limited", Value: msg})
						}
					}
				}
			}
		}

	case "Kustomization", "GitRepository", "HelmRepository", "HelmChart", "OCIRepository", "Bucket",
		"Alert", "Provider", "Receiver", "ImageRepository", "ImagePolicy", "ImageUpdateAutomation":
		// FluxCD resources: extract conditions-based status.
		if spec, ok := obj["spec"].(map[string]interface{}); ok {
			if suspended, ok := spec["suspend"].(bool); ok && suspended {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Suspended", Value: "True"})
			}
		}
		if status != nil {
			// Extract Ready condition.
			if conditions, ok := status["conditions"].([]interface{}); ok {
				for _, c := range conditions {
					cond, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					condType, _ := cond["type"].(string)
					condStatus, _ := cond["status"].(string)
					condMessage, _ := cond["message"].(string)
					condReason, _ := cond["reason"].(string)
					if condType == "Ready" {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ready", Value: condStatus})
						if condReason != "" {
							ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: condReason})
						}
						if condMessage != "" && condStatus != "True" {
							ti.Columns = append(ti.Columns, model.KeyValue{Key: "Message", Value: condMessage})
						}
						if lastTransition, ok := cond["lastTransitionTime"].(string); ok && lastTransition != "" {
							if t, err := time.Parse(time.RFC3339, lastTransition); err == nil {
								ti.Columns = append(ti.Columns, model.KeyValue{Key: "Last Transition", Value: formatRelativeTime(t)})
							}
						}
						break
					}
				}
			}
			// Extract last applied revision.
			if rev, ok := status["lastAppliedRevision"].(string); ok && rev != "" {
				if len(rev) > 12 {
					rev = rev[:12]
				}
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Revision", Value: rev})
			} else if artifact, ok := status["artifact"].(map[string]interface{}); ok {
				if rev, ok := artifact["revision"].(string); ok && rev != "" {
					if len(rev) > 12 {
						rev = rev[:12]
					}
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Revision", Value: rev})
				}
			}
		}

	case "Certificate", "CertificateRequest", "Issuer", "ClusterIssuer", "Order", "Challenge":
		// cert-manager resources: extract conditions-based status and certificate-specific fields.
		if status != nil {
			if conditions, ok := status["conditions"].([]interface{}); ok {
				for _, c := range conditions {
					cond, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					condType, _ := cond["type"].(string)
					condStatus, _ := cond["status"].(string)
					condMessage, _ := cond["message"].(string)
					condReason, _ := cond["reason"].(string)
					if condType == "Ready" {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ready", Value: condStatus})
						if condReason != "" {
							ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: condReason})
						}
						if condMessage != "" && condStatus != "True" {
							ti.Columns = append(ti.Columns, model.KeyValue{Key: "Message", Value: condMessage})
						}
						if lastTransition, ok := cond["lastTransitionTime"].(string); ok && lastTransition != "" {
							if t, err := time.Parse(time.RFC3339, lastTransition); err == nil {
								ti.Columns = append(ti.Columns, model.KeyValue{Key: "Last Transition", Value: formatRelativeTime(t)})
							}
						}
						break
					}
				}
			}
			if notAfter, ok := status["notAfter"].(string); ok && notAfter != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Expires", Value: notAfter})
			}
			if renewalTime, ok := status["renewalTime"].(string); ok && renewalTime != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Renewal", Value: renewalTime})
			}
		}
		if spec != nil {
			if secretName, ok := spec["secretName"].(string); ok && secretName != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Secret", Value: secretName})
			}
		}

	case "Application", "ApplicationSet":
		// ArgoCD Application: extract health and sync sub-fields from status maps.
		if status != nil {
			if health, ok := status["health"].(map[string]interface{}); ok {
				if hs, ok := health["status"].(string); ok {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Health", Value: hs})
				}
				if msg, ok := health["message"].(string); ok && msg != "" {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Health Message", Value: msg})
				}
			}
			if sync, ok := status["sync"].(map[string]interface{}); ok {
				if ss, ok := sync["status"].(string); ok {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Sync", Value: ss})
				}
				if rev, ok := sync["revision"].(string); ok && rev != "" {
					if len(rev) > 8 {
						rev = rev[:8]
					}
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Revision", Value: rev})
				}
			}
			// Last sync operation result.
			if opState, ok := status["operationState"].(map[string]interface{}); ok {
				if phase, ok := opState["phase"].(string); ok && phase != "" {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Last Sync", Value: phase})
				}
				if msg, ok := opState["message"].(string); ok && msg != "" {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Sync Message", Value: msg})
				}
				if finishedAt, ok := opState["finishedAt"].(string); ok && finishedAt != "" {
					if t, err := time.Parse(time.RFC3339, finishedAt); err == nil {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Synced At", Value: formatRelativeTime(t)})
					}
				}
				// Extract per-resource sync errors from syncResult.
				if syncResult, ok := opState["syncResult"].(map[string]interface{}); ok {
					if resources, ok := syncResult["resources"].([]interface{}); ok {
						var errs []string
						for _, r := range resources {
							rMap, ok := r.(map[string]interface{})
							if !ok {
								continue
							}
							rStatus, _ := rMap["status"].(string)
							if rStatus != "Synced" && rStatus != "" {
								kind, _ := rMap["kind"].(string)
								name, _ := rMap["name"].(string)
								msg, _ := rMap["message"].(string)
								if msg != "" {
									errs = append(errs, fmt.Sprintf("%s/%s: %s", kind, name, msg))
								}
							}
						}
						if len(errs) > 0 {
							ti.Columns = append(ti.Columns, model.KeyValue{Key: "Sync Errors", Value: strings.Join(errs, "; ")})
						}
					}
				}
			}
			if summary, ok := status["summary"].(map[string]interface{}); ok {
				if imgs, ok := summary["images"].([]interface{}); ok && len(imgs) > 0 {
					var imageStrs []string
					for _, img := range imgs {
						if s, ok := img.(string); ok {
							imageStrs = append(imageStrs, s)
						}
					}
					if len(imageStrs) > 0 {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Images", Value: strings.Join(imageStrs, ", ")})
					}
				}
			}
		}
		if spec != nil {
			if dest, ok := spec["destination"].(map[string]interface{}); ok {
				if ns, ok := dest["namespace"].(string); ok && ns != "" {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Dest NS", Value: ns})
				}
				if server, ok := dest["server"].(string); ok && server != "" {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Dest Server", Value: server})
				}
			}
			if source, ok := spec["source"].(map[string]interface{}); ok {
				if repo, ok := source["repoURL"].(string); ok && repo != "" {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Repo", Value: repo})
				}
				if path, ok := source["path"].(string); ok && path != "" {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Path", Value: path})
				}
			}
		}

	case "Event":
		// Use event type (Normal/Warning) as status.
		if eventType, ok := obj["type"].(string); ok {
			ti.Status = eventType
		}
		// Use lastTimestamp (or eventTime) for Age so events sort by most recent activity.
		eventTime := parseEventTimestamp(obj, "lastTimestamp")
		if eventTime.IsZero() {
			eventTime = parseEventTimestamp(obj, "eventTime")
		}
		if !eventTime.IsZero() {
			ti.CreatedAt = eventTime
			ti.Age = formatAge(time.Since(eventTime))
		}
		// Extract event-specific fields.
		if involvedObj, ok := obj["involvedObject"].(map[string]interface{}); ok {
			objKind, _ := involvedObj["kind"].(string)
			objName, _ := involvedObj["name"].(string)
			if objKind != "" && objName != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Object", Value: objKind + "/" + objName})
			}
		}
		if reason, ok := obj["reason"].(string); ok && reason != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: reason})
		}
		if message, ok := obj["message"].(string); ok && message != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Message", Value: message})
		}
		// Always show occurrence count.
		eventCount := int64(1)
		if count, ok := obj["count"].(int64); ok && count > 0 {
			eventCount = count
		} else if countF, ok := obj["count"].(float64); ok && countF > 0 {
			eventCount = int64(countF)
		}
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Count", Value: fmt.Sprintf("%d", eventCount)})
		if source, ok := obj["source"].(map[string]interface{}); ok {
			if component, ok := source["component"].(string); ok && component != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Source", Value: component})
			}
		}

	case "IngressClass":
		metadata, _ := obj["metadata"].(map[string]interface{})
		annotations, _ := metadata["annotations"].(map[string]interface{})
		if val, ok := annotations["ingressclass.kubernetes.io/is-default-class"].(string); ok && val == "true" {
			ti.Name += " (default)"
			ti.Status = "default"
		}
	case "StorageClass":
		metadata, _ := obj["metadata"].(map[string]interface{})
		annotations, _ := metadata["annotations"].(map[string]interface{})
		if val, ok := annotations["storageclass.kubernetes.io/is-default-class"].(string); ok && val == "true" {
			ti.Name += " (default)"
			ti.Status = "default"
		}
		if provisioner, ok := obj["provisioner"].(string); ok && provisioner != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Provisioner", Value: provisioner})
		}
		if reclaimPolicy, ok := obj["reclaimPolicy"].(string); ok && reclaimPolicy != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reclaim Policy", Value: reclaimPolicy})
		}
		if vbm, ok := obj["volumeBindingMode"].(string); ok && vbm != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Binding Mode", Value: vbm})
		}
		if ae, ok := obj["allowVolumeExpansion"].(bool); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Allow Expansion", Value: fmt.Sprintf("%v", ae)})
		}

	case "PersistentVolume":
		if spec != nil {
			// Capacity.
			if cap, ok := spec["capacity"].(map[string]interface{}); ok {
				if storage, ok := cap["storage"].(string); ok {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Capacity", Value: storage})
				}
			}
			// Access modes.
			if am, ok := spec["accessModes"].([]interface{}); ok {
				var modes []string
				for _, m := range am {
					if s, ok := m.(string); ok {
						modes = append(modes, s)
					}
				}
				if len(modes) > 0 {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Access Modes", Value: strings.Join(modes, ", ")})
				}
			}
			if rp, ok := spec["persistentVolumeReclaimPolicy"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reclaim Policy", Value: rp})
			}
			if sc, ok := spec["storageClassName"].(string); ok && sc != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Storage Class", Value: sc})
			}
			if vm, ok := spec["volumeMode"].(string); ok && vm != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Volume Mode", Value: vm})
			}
			// Claim ref.
			if claimRef, ok := spec["claimRef"].(map[string]interface{}); ok {
				claimNS, _ := claimRef["namespace"].(string)
				claimName, _ := claimRef["name"].(string)
				if claimName != "" {
					claim := claimName
					if claimNS != "" {
						claim = claimNS + "/" + claimName
					}
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Claim", Value: claim})
				}
			}
		}
		if status != nil {
			if phase, ok := status["phase"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Status", Value: phase})
				ti.Status = phase
			}
			if reason, ok := status["reason"].(string); ok && reason != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: reason})
			}
		}

	case "ResourceQuota":
		if status != nil {
			hard, _ := status["hard"].(map[string]interface{})
			used, _ := status["used"].(map[string]interface{})
			if hard != nil {
				var quotaKeys []string
				for k := range hard {
					quotaKeys = append(quotaKeys, k)
				}
				sort.Strings(quotaKeys)
				for _, k := range quotaKeys {
					hardVal := fmt.Sprintf("%v", hard[k])
					usedVal := "0"
					if used != nil {
						if u, ok := used[k]; ok {
							usedVal = fmt.Sprintf("%v", u)
						}
					}
					ti.Columns = append(ti.Columns, model.KeyValue{
						Key:   k,
						Value: fmt.Sprintf("%s / %s", usedVal, hardVal),
					})
				}
			}
		} else if spec != nil {
			// If no status yet, show hard limits from spec.
			if hard, ok := spec["hard"].(map[string]interface{}); ok {
				var quotaKeys []string
				for k := range hard {
					quotaKeys = append(quotaKeys, k)
				}
				sort.Strings(quotaKeys)
				for _, k := range quotaKeys {
					ti.Columns = append(ti.Columns, model.KeyValue{
						Key:   k,
						Value: fmt.Sprintf("%v (hard)", hard[k]),
					})
				}
			}
		}

	case "LimitRange":
		if spec != nil {
			if limits, ok := spec["limits"].([]interface{}); ok {
				for _, l := range limits {
					lMap, ok := l.(map[string]interface{})
					if !ok {
						continue
					}
					lType, _ := lMap["type"].(string)
					prefix := lType
					if prefix == "" {
						prefix = "Unknown"
					}
					if def, ok := lMap["default"].(map[string]interface{}); ok {
						for resource, val := range def {
							ti.Columns = append(ti.Columns, model.KeyValue{
								Key:   fmt.Sprintf("%s Default %s", prefix, resource),
								Value: fmt.Sprintf("%v", val),
							})
						}
					}
					if defReq, ok := lMap["defaultRequest"].(map[string]interface{}); ok {
						for resource, val := range defReq {
							ti.Columns = append(ti.Columns, model.KeyValue{
								Key:   fmt.Sprintf("%s Default Req %s", prefix, resource),
								Value: fmt.Sprintf("%v", val),
							})
						}
					}
					if max, ok := lMap["max"].(map[string]interface{}); ok {
						for resource, val := range max {
							ti.Columns = append(ti.Columns, model.KeyValue{
								Key:   fmt.Sprintf("%s Max %s", prefix, resource),
								Value: fmt.Sprintf("%v", val),
							})
						}
					}
					if min, ok := lMap["min"].(map[string]interface{}); ok {
						for resource, val := range min {
							ti.Columns = append(ti.Columns, model.KeyValue{
								Key:   fmt.Sprintf("%s Min %s", prefix, resource),
								Value: fmt.Sprintf("%v", val),
							})
						}
					}
				}
			}
		}

	case "PodDisruptionBudget":
		if spec != nil {
			if minAvail, ok := spec["minAvailable"]; ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Min Available", Value: fmt.Sprintf("%v", minAvail)})
			}
			if maxUnavail, ok := spec["maxUnavailable"]; ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Max Unavailable", Value: fmt.Sprintf("%v", maxUnavail)})
			}
			if selector, ok := spec["selector"].(map[string]interface{}); ok {
				if matchLabels, ok := selector["matchLabels"].(map[string]interface{}); ok {
					var parts []string
					for k, v := range matchLabels {
						parts = append(parts, fmt.Sprintf("%s=%v", k, v))
					}
					if len(parts) > 0 {
						sort.Strings(parts)
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Selector", Value: strings.Join(parts, ", ")})
					}
				}
			}
		}
		if status != nil {
			if current, ok := status["currentHealthy"].(float64); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Current Healthy", Value: fmt.Sprintf("%d", int64(current))})
			}
			if desired, ok := status["desiredHealthy"].(float64); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Desired Healthy", Value: fmt.Sprintf("%d", int64(desired))})
			}
			if allowed, ok := status["disruptionsAllowed"].(float64); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Disruptions Allowed", Value: fmt.Sprintf("%d", int64(allowed))})
			}
			if expected, ok := status["expectedPods"].(float64); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Expected Pods", Value: fmt.Sprintf("%d", int64(expected))})
			}
		}

	case "NetworkPolicy":
		if spec != nil {
			// Pod selector.
			if selector, ok := spec["podSelector"].(map[string]interface{}); ok {
				if matchLabels, ok := selector["matchLabels"].(map[string]interface{}); ok {
					var parts []string
					for k, v := range matchLabels {
						parts = append(parts, fmt.Sprintf("%s=%v", k, v))
					}
					if len(parts) > 0 {
						sort.Strings(parts)
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Pod Selector", Value: strings.Join(parts, ", ")})
					}
				} else {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Pod Selector", Value: "(all pods)"})
				}
			}
			// Policy types.
			if policyTypes, ok := spec["policyTypes"].([]interface{}); ok {
				var types []string
				for _, pt := range policyTypes {
					if s, ok := pt.(string); ok {
						types = append(types, s)
					}
				}
				if len(types) > 0 {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Policy Types", Value: strings.Join(types, ", ")})
				}
			}
			// Ingress rules count.
			if ingress, ok := spec["ingress"].([]interface{}); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ingress Rules", Value: fmt.Sprintf("%d", len(ingress))})
			}
			// Egress rules count.
			if egress, ok := spec["egress"].([]interface{}); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Egress Rules", Value: fmt.Sprintf("%d", len(egress))})
			}
		}

	case "ServiceAccount":
		// Secrets count.
		if secrets, ok := obj["secrets"].([]interface{}); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Secrets", Value: fmt.Sprintf("%d", len(secrets))})
		}
		// Automount token.
		if automount, ok := obj["automountServiceAccountToken"].(bool); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Automount Token", Value: fmt.Sprintf("%v", automount)})
		}
		// Image pull secrets.
		if ips, ok := obj["imagePullSecrets"].([]interface{}); ok && len(ips) > 0 {
			var names []string
			for _, s := range ips {
				if sMap, ok := s.(map[string]interface{}); ok {
					if name, ok := sMap["name"].(string); ok {
						names = append(names, name)
					}
				}
			}
			if len(names) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Image Pull Secrets", Value: strings.Join(names, ", ")})
			}
		}

	case "PriorityClass":
		if val, ok := spec["globalDefault"].(bool); ok && val {
			ti.Name += " (default)"
			ti.Status = "default"
		}

	default:
		// For unknown/CRD resources, extract top-level status fields.
		if status != nil {
			for _, key := range []string{"phase", "state", "health", "sync", "message", "reason"} {
				if v, ok := status[key]; ok {
					label := strings.ToUpper(key[:1]) + key[1:]
					switch val := v.(type) {
					case map[string]interface{}:
						// Extract sub-fields from maps (e.g., health/sync maps).
						for subKey, subVal := range val {
							subLabel := label + " " + strings.ToUpper(subKey[:1]) + subKey[1:]
							ti.Columns = append(ti.Columns, model.KeyValue{Key: subLabel, Value: fmt.Sprintf("%v", subVal)})
						}
					default:
						ti.Columns = append(ti.Columns, model.KeyValue{Key: label, Value: fmt.Sprintf("%v", val)})
					}
				}
			}
		}
	}
}

// populateContainerImages extracts container images from a pod template spec.
func populateContainerImages(ti *model.Item, spec map[string]interface{}) {
	tmpl, ok := spec["template"].(map[string]interface{})
	if !ok {
		return
	}
	tmplSpec, ok := tmpl["spec"].(map[string]interface{})
	if !ok {
		return
	}
	containers, ok := tmplSpec["containers"].([]interface{})
	if !ok {
		return
	}
	var images []string
	for _, c := range containers {
		if cMap, ok := c.(map[string]interface{}); ok {
			if img, ok := cMap["image"].(string); ok {
				images = append(images, img)
			}
		}
	}
	if len(images) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Images", Value: strings.Join(images, ", ")})
	}
}

// extractContainerResources sums CPU and memory requests/limits from a list of container specs.
// Returns cpuReq, cpuLim, memReq, memLim as human-readable strings.
func extractContainerResources(containers []interface{}) (cpuReq, cpuLim, memReq, memLim string) {
	var cpuReqs, cpuLims, memReqs, memLims []string
	for _, c := range containers {
		cMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		resources, ok := cMap["resources"].(map[string]interface{})
		if !ok {
			continue
		}
		if requests, ok := resources["requests"].(map[string]interface{}); ok {
			if v, ok := requests["cpu"].(string); ok {
				cpuReqs = append(cpuReqs, v)
			}
			if v, ok := requests["memory"].(string); ok {
				memReqs = append(memReqs, v)
			}
		}
		if limits, ok := resources["limits"].(map[string]interface{}); ok {
			if v, ok := limits["cpu"].(string); ok {
				cpuLims = append(cpuLims, v)
			}
			if v, ok := limits["memory"].(string); ok {
				memLims = append(memLims, v)
			}
		}
	}
	if len(cpuReqs) > 0 {
		cpuReq = strings.Join(cpuReqs, "+")
	}
	if len(cpuLims) > 0 {
		cpuLim = strings.Join(cpuLims, "+")
	}
	if len(memReqs) > 0 {
		memReq = strings.Join(memReqs, "+")
	}
	if len(memLims) > 0 {
		memLim = strings.Join(memLims, "+")
	}
	return
}

// extractTemplateResources navigates spec.template.spec.containers and extracts resource requests/limits.
func extractTemplateResources(spec map[string]interface{}) (cpuReq, cpuLim, memReq, memLim string) {
	tmpl, ok := spec["template"].(map[string]interface{})
	if !ok {
		return
	}
	tmplSpec, ok := tmpl["spec"].(map[string]interface{})
	if !ok {
		return
	}
	containers, ok := tmplSpec["containers"].([]interface{})
	if !ok {
		return
	}
	return extractContainerResources(containers)
}

// addResourceColumns appends CPU/memory request/limit columns to an item if they are non-empty.
func addResourceColumns(ti *model.Item, cpuReq, cpuLim, memReq, memLim string) {
	if cpuReq != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "CPU Req", Value: cpuReq})
	}
	if cpuLim != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "CPU Lim", Value: cpuLim})
	}
	if memReq != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Mem Req", Value: memReq})
	}
	if memLim != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Mem Lim", Value: memLim})
	}
}

// getInt extracts an integer value from a map, handling both int64 and float64.
func getInt(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(int64); ok {
		return v
	}
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return 0
}

// formatAge formats a duration into a human-readable age string.
// parseEventTimestamp extracts a timestamp from a top-level event field (e.g., "lastTimestamp", "eventTime").
func parseEventTimestamp(obj map[string]interface{}, field string) time.Time {
	val, ok := obj[field]
	if !ok || val == nil {
		return time.Time{}
	}
	v, ok := val.(string)
	if !ok || v == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		// Try RFC3339Nano for eventTime which may include nanoseconds.
		t, err = time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return time.Time{}
		}
	}
	return t
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 365 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dy", days/365)
}

// GetResourceYAML returns the YAML representation of a specific resource.
// For cluster-scoped resources (rt.Namespaced == false), the namespace is ignored.
// The resourceNamespace parameter is used when in all-namespaces mode to target
// the specific namespace of the resource.
func (c *Client) GetResourceYAML(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry, name string) (string, error) {
	// Special handling for Helm virtual resource type.
	if rt.APIGroup == "_helm" && rt.Resource == "releases" {
		return c.GetHelmReleaseYAML(ctx, contextName, namespace, name)
	}
	if rt.APIGroup == "_portforward" {
		return "", nil // port forwards are managed locally, not via K8s API
	}

	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return "", err
	}

	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}

	var getter dynamic.ResourceInterface
	if rt.Namespaced {
		getter = dynClient.Resource(gvr).Namespace(namespace)
	} else {
		getter = dynClient.Resource(gvr)
	}

	obj, err := getter.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting resource: %w", err)
	}

	obj.SetManagedFields(nil)

	data, err := yaml.Marshal(obj.Object)
	if err != nil {
		return "", fmt.Errorf("marshalling to YAML: %w", err)
	}
	return reorderYAMLFields(string(data)), nil
}

// GetPodYAML returns the YAML for a pod.
func (c *Client) GetPodYAML(ctx context.Context, contextName, namespace, podName string) (string, error) {
	return c.GetResourceYAML(ctx, contextName, namespace, model.ResourceTypeEntry{
		APIGroup: "", APIVersion: "v1", Resource: "pods",
	}, podName)
}

// DeleteResource deletes a Kubernetes resource by type and name.
func (c *Client) DeleteResource(contextName, namespace string, rt model.ResourceTypeEntry, name string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}

	var deleter dynamic.ResourceInterface
	if rt.Namespaced {
		deleter = dynClient.Resource(gvr).Namespace(namespace)
	} else {
		deleter = dynClient.Resource(gvr)
	}

	err = deleter.Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("deleting %s/%s: %w", rt.Resource, name, err)
	}
	return nil
}

// ScaleDeployment scales a deployment to the specified replica count.
func (c *Client) ScaleDeployment(contextName, namespace, name string, replicas int32) error {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return err
	}

	scale, err := cs.AppsV1().Deployments(namespace).GetScale(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting scale for %s: %w", name, err)
	}

	scale.Spec.Replicas = replicas
	_, err = cs.AppsV1().Deployments(namespace).UpdateScale(context.Background(), name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("scaling %s to %d: %w", name, replicas, err)
	}
	return nil
}

// RestartDeployment performs a rolling restart by patching the pod template annotation.
func (c *Client) RestartDeployment(contextName, namespace, name string) error {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return err
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshalling patch: %w", err)
	}

	_, err = cs.AppsV1().Deployments(namespace).Patch(
		context.Background(), name, k8stypes.StrategicMergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("restarting deployment %s: %w", name, err)
	}
	return nil
}

// RollbackDeployment rolls back a deployment to a specific revision.
func (c *Client) RollbackDeployment(ctx context.Context, contextName, namespace, name string, revision int64) error {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return err
	}

	// List ReplicaSets owned by this deployment.
	deploy, err := cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting deployment: %w", err)
	}

	rsList, err := cs.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing replicasets: %w", err)
	}

	// Find the RS with the target revision.
	for _, rs := range rsList.Items {
		// Check ownership.
		owned := false
		for _, ref := range rs.OwnerReferences {
			if ref.UID == deploy.UID {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}

		rev, _ := strconv.ParseInt(rs.Annotations["deployment.kubernetes.io/revision"], 10, 64)
		if rev == revision {
			// Patch the deployment with this RS's pod template.
			patchData, err := json.Marshal(map[string]interface{}{
				"spec": map[string]interface{}{
					"template": rs.Spec.Template,
				},
			})
			if err != nil {
				return fmt.Errorf("marshaling patch: %w", err)
			}

			_, err = cs.AppsV1().Deployments(namespace).Patch(ctx, name, k8stypes.MergePatchType, patchData, metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("patching deployment: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("revision %d not found", revision)
}

// GetDeploymentRevisions returns the list of revisions for a deployment.
func (c *Client) GetDeploymentRevisions(ctx context.Context, contextName, namespace, name string) ([]DeploymentRevision, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	deploy, err := cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting deployment: %w", err)
	}

	rsList, err := cs.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing replicasets: %w", err)
	}

	revisions := make([]DeploymentRevision, 0, len(rsList.Items))
	for _, rs := range rsList.Items {
		owned := false
		for _, ref := range rs.OwnerReferences {
			if ref.UID == deploy.UID {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}

		rev, _ := strconv.ParseInt(rs.Annotations["deployment.kubernetes.io/revision"], 10, 64)

		// Extract images from the template.
		var images []string
		for _, container := range rs.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}

		revisions = append(revisions, DeploymentRevision{
			Revision:  rev,
			Name:      rs.Name,
			Replicas:  rs.Status.Replicas,
			Images:    images,
			CreatedAt: rs.CreationTimestamp.Time,
		})
	}

	// Sort by revision descending.
	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].Revision > revisions[j].Revision
	})

	return revisions, nil
}

// --- internal helpers ---

func (c *Client) restConfigForContext(contextName string) (*rest.Config, error) {
	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(c.loadingRules, overrides)
	cfg, err := cc.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building rest config for context %q: %w", contextName, err)
	}
	return cfg, nil
}

func (c *Client) clientsetForContext(contextName string) (*kubernetes.Clientset, error) {
	cfg, err := c.restConfigForContext(contextName)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating clientset: %w", err)
	}
	return cs, nil
}

func (c *Client) dynamicForContext(contextName string) (dynamic.Interface, error) {
	cfg, err := c.restConfigForContext(contextName)
	if err != nil {
		return nil, err
	}
	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}
	return dynClient, nil
}

// DiscoverCRDs lists all installed CRDs on the given cluster context and returns
// them as ResourceTypeEntry values suitable for navigation. Each CRD is mapped
// to a resource type using its spec metadata (group, versions, names, scope).
func (c *Client) DiscoverCRDs(ctx context.Context, contextName string) ([]model.ResourceTypeEntry, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	list, err := dynClient.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing CRDs: %w", err)
	}

	entries := make([]model.ResourceTypeEntry, 0, len(list.Items))
	for _, item := range list.Items {
		spec, ok := item.Object["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		group, _ := spec["group"].(string)

		names, ok := spec["names"].(map[string]interface{})
		if !ok {
			continue
		}
		plural, _ := names["plural"].(string)
		kind, _ := names["kind"].(string)
		if plural == "" || kind == "" {
			continue
		}

		// Determine the preferred/served version.
		apiVersion := preferredCRDVersion(spec, item.Object)

		// Determine scope.
		scope, _ := spec["scope"].(string)
		namespaced := strings.EqualFold(scope, "Namespaced")

		// Build a display name from the plural name (title case the first letter).
		displayName := strings.ToUpper(plural[:1]) + plural[1:]

		entry := model.ResourceTypeEntry{
			DisplayName: displayName,
			Kind:        kind,
			APIGroup:    group,
			APIVersion:  apiVersion,
			Resource:    plural,
			Icon:        "⧫",
			Namespaced:  namespaced,
		}

		// Check if this CRD uses a deprecated API version.
		if dep, found := CheckDeprecation(group, apiVersion, plural); found {
			entry.Deprecated = true
			entry.DeprecationMsg = dep.Message
		}

		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].APIGroup != entries[j].APIGroup {
			return entries[i].APIGroup < entries[j].APIGroup
		}
		return entries[i].DisplayName < entries[j].DisplayName
	})

	return entries, nil
}

// preferredCRDVersion extracts the preferred or first served version from a CRD object.
func preferredCRDVersion(spec, obj map[string]interface{}) string {
	// Try spec.versions (v1 CRDs): pick the first one marked as "served", preferring "storage".
	if versions, ok := spec["versions"].([]interface{}); ok && len(versions) > 0 {
		var firstServed, storageVersion string
		for _, v := range versions {
			vm, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := vm["name"].(string)
			served, _ := vm["served"].(bool)
			storage, _ := vm["storage"].(bool)
			if storage && served && name != "" {
				storageVersion = name
			}
			if served && firstServed == "" && name != "" {
				firstServed = name
			}
		}
		if storageVersion != "" {
			return storageVersion
		}
		if firstServed != "" {
			return firstServed
		}
	}

	// Fallback: status.storedVersions.
	if status, ok := obj["status"].(map[string]interface{}); ok {
		if stored, ok := status["storedVersions"].([]interface{}); ok && len(stored) > 0 {
			if v, ok := stored[0].(string); ok && v != "" {
				return v
			}
		}
	}

	return "v1"
}

// containerStateString returns a human-readable container state.
// buildContainerItem creates a model.Item for a container with enriched details.
func buildContainerItem(c corev1.Container, statuses []corev1.ContainerStatus, isInit, isSidecar bool) model.Item {
	item := model.Item{
		Name:  c.Name,
		Kind:  "Container",
		Extra: c.Image,
	}

	switch {
	case isSidecar:
		item.Category = "Sidecar Containers"
		item.Status = "Waiting"
	case isInit:
		item.Category = "Init Containers"
		item.Status = "Init"
	default:
		item.Category = "Containers"
		item.Status = "Waiting"
	}

	// Find matching container status.
	for _, cs := range statuses {
		if cs.Name != c.Name {
			continue
		}

		item.Status = containerStateString(cs.Ready, cs.State.Waiting, cs.State.Running, cs.State.Terminated)
		item.Ready = fmt.Sprintf("%v", cs.Ready)
		item.Restarts = fmt.Sprintf("%d", cs.RestartCount)

		// Started time for age calculation.
		if cs.State.Running != nil && !cs.State.Running.StartedAt.IsZero() {
			item.CreatedAt = cs.State.Running.StartedAt.Time
			item.Age = formatAge(time.Since(cs.State.Running.StartedAt.Time))
		}

		// Add reason to columns if not ready.
		if !cs.Ready {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
				item.Status = cs.State.Waiting.Reason
				item.Columns = append(item.Columns, model.KeyValue{Key: "Reason", Value: cs.State.Waiting.Reason})
				if cs.State.Waiting.Message != "" {
					item.Columns = append(item.Columns, model.KeyValue{Key: "Message", Value: cs.State.Waiting.Message})
				}
			}
			if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
				item.Status = cs.State.Terminated.Reason
				item.Columns = append(item.Columns, model.KeyValue{Key: "Reason", Value: cs.State.Terminated.Reason})
				if cs.State.Terminated.Message != "" {
					item.Columns = append(item.Columns, model.KeyValue{Key: "Message", Value: cs.State.Terminated.Message})
				}
				item.Columns = append(item.Columns, model.KeyValue{Key: "Exit Code", Value: fmt.Sprintf("%d", cs.State.Terminated.ExitCode)})
			}
		}

		// Last terminated state (useful for CrashLoopBackOff).
		if cs.LastTerminationState.Terminated != nil {
			lt := cs.LastTerminationState.Terminated
			item.Columns = append(item.Columns, model.KeyValue{Key: "Last Terminated", Value: lt.Reason})
			if lt.ExitCode != 0 {
				item.Columns = append(item.Columns, model.KeyValue{Key: "Last Exit Code", Value: fmt.Sprintf("%d", lt.ExitCode)})
			}
		}

		break
	}

	// Resource requests/limits.
	if req := c.Resources.Requests; req != nil {
		if cpu, ok := req[corev1.ResourceCPU]; ok {
			item.Columns = append(item.Columns, model.KeyValue{Key: "CPU Request", Value: cpu.String()})
		}
		if mem, ok := req[corev1.ResourceMemory]; ok {
			item.Columns = append(item.Columns, model.KeyValue{Key: "Memory Request", Value: mem.String()})
		}
	}
	if lim := c.Resources.Limits; lim != nil {
		if cpu, ok := lim[corev1.ResourceCPU]; ok {
			item.Columns = append(item.Columns, model.KeyValue{Key: "CPU Limit", Value: cpu.String()})
		}
		if mem, ok := lim[corev1.ResourceMemory]; ok {
			item.Columns = append(item.Columns, model.KeyValue{Key: "Memory Limit", Value: mem.String()})
		}
	}

	// Ports.
	if len(c.Ports) > 0 {
		ports := make([]string, 0, len(c.Ports))
		for _, p := range c.Ports {
			port := fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol)
			if p.Name != "" {
				port = p.Name + ":" + port
			}
			ports = append(ports, port)
		}
		item.Columns = append(item.Columns, model.KeyValue{Key: "Ports", Value: strings.Join(ports, ", ")})
	}

	return item
}

func containerStateString(ready bool, waiting interface{}, running interface{}, terminated interface{}) string {
	if running != nil {
		if ready {
			return "Running"
		}
		return "NotReady"
	}
	if waiting != nil {
		return "Waiting"
	}
	if terminated != nil {
		return "Terminated"
	}
	return "Unknown"
}

// extractStatus pulls a human-readable status string from an unstructured object.
// extractContainerNotReadyReason extracts the reason why a container is not ready
// from container statuses (e.g., CrashLoopBackOff, ImagePullBackOff, OOMKilled).
func extractContainerNotReadyReason(containerStatuses []interface{}) string {
	for _, cs := range containerStatuses {
		csMap, ok := cs.(map[string]interface{})
		if !ok {
			continue
		}
		if ready, ok := csMap["ready"].(bool); ok && ready {
			continue
		}
		state, _ := csMap["state"].(map[string]interface{})
		if state == nil {
			continue
		}
		// Check waiting state.
		if waiting, ok := state["waiting"].(map[string]interface{}); ok {
			if reason, ok := waiting["reason"].(string); ok && reason != "" {
				return reason
			}
		}
		// Check terminated state.
		if terminated, ok := state["terminated"].(map[string]interface{}); ok {
			if reason, ok := terminated["reason"].(string); ok && reason != "" {
				return reason
			}
		}
	}
	return ""
}

func extractStatus(obj map[string]interface{}) string {
	status, ok := obj["status"]
	if !ok {
		return ""
	}
	statusMap, ok := status.(map[string]interface{})
	if !ok {
		return ""
	}
	if phase, ok := statusMap["phase"].(string); ok {
		return phase
	}
	// ArgoCD Application: prefer health status + sync status.
	if health, ok := statusMap["health"].(map[string]interface{}); ok {
		if healthStatus, ok := health["status"].(string); ok {
			if sync, ok := statusMap["sync"].(map[string]interface{}); ok {
				if syncStatus, ok := sync["status"].(string); ok {
					return healthStatus + "/" + syncStatus
				}
			}
			return healthStatus
		}
	}
	// FluxCD resources: check suspend and Ready condition.
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		if suspended, ok := spec["suspend"].(bool); ok && suspended {
			return "Suspended"
		}
	}
	if conditions, ok := statusMap["conditions"].([]interface{}); ok && len(conditions) > 0 {
		// Prefer "Available" condition with status "True" over other conditions
		// (e.g., Deployments often have "Progressing" as the last condition even
		// when fully available).
		for _, c := range conditions {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			if condType == "Available" && condStatus == "True" {
				return "Available"
			}
		}
		// Fall back to the last condition's type.
		if cond, ok := conditions[len(conditions)-1].(map[string]interface{}); ok {
			if t, ok := cond["type"].(string); ok {
				return t
			}
		}
	}
	return ""
}

// GetSecretData fetches a secret and returns its decoded key-value pairs.
func (c *Client) GetSecretData(ctx context.Context, contextName, namespace, name string) (*model.SecretData, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	secret, err := cs.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting secret: %w", err)
	}

	data := make(map[string]string, len(secret.Data))
	keys := make([]string, 0, len(secret.Data))
	for k, v := range secret.Data {
		keys = append(keys, k)
		data[k] = string(v)
	}
	sort.Strings(keys)

	return &model.SecretData{
		Keys: keys,
		Data: data,
	}, nil
}

// UpdateSecretData updates a secret's data with the provided values.
func (c *Client) UpdateSecretData(contextName, namespace, name string, data map[string]string) error {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return err
	}

	secret, err := cs.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting secret for update: %w", err)
	}

	secret.Data = make(map[string][]byte, len(data))
	for k, v := range data {
		secret.Data[k] = []byte(v)
	}

	_, err = cs.CoreV1().Secrets(namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating secret: %w", err)
	}
	return nil
}

// GetConfigMapData fetches a ConfigMap and returns its key-value pairs.
func (c *Client) GetConfigMapData(ctx context.Context, contextName, namespace, name string) (*model.ConfigMapData, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	cm, err := cs.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting configmap: %w", err)
	}

	data := make(map[string]string, len(cm.Data))
	keys := make([]string, 0, len(cm.Data))
	for k, v := range cm.Data {
		keys = append(keys, k)
		data[k] = v
	}
	sort.Strings(keys)

	return &model.ConfigMapData{
		Keys: keys,
		Data: data,
	}, nil
}

// UpdateConfigMapData updates a ConfigMap's data with the provided values.
func (c *Client) UpdateConfigMapData(contextName, namespace, name string, data map[string]string) error {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return err
	}

	cm, err := cs.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting configmap for update: %w", err)
	}

	cm.Data = make(map[string]string, len(data))
	for k, v := range data {
		cm.Data[k] = v
	}

	_, err = cs.CoreV1().ConfigMaps(namespace).Update(context.Background(), cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating configmap: %w", err)
	}
	return nil
}

// GetLabelAnnotationData fetches labels and annotations for any resource.
func (c *Client) GetLabelAnnotationData(ctx context.Context, contextName string, rt model.ResourceTypeEntry, namespace, name string) (*model.LabelAnnotationData, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}

	var obj *unstructured.Unstructured
	if rt.Namespaced {
		obj, err = dynClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = dynClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("getting resource: %w", err)
	}

	labels := obj.GetLabels()
	annotations := obj.GetAnnotations()
	if labels == nil {
		labels = make(map[string]string)
	}
	if annotations == nil {
		annotations = make(map[string]string)
	}

	labelKeys := make([]string, 0, len(labels))
	for k := range labels {
		labelKeys = append(labelKeys, k)
	}
	sort.Strings(labelKeys)

	annotKeys := make([]string, 0, len(annotations))
	for k := range annotations {
		annotKeys = append(annotKeys, k)
	}
	sort.Strings(annotKeys)

	return &model.LabelAnnotationData{
		Labels:      labels,
		LabelKeys:   labelKeys,
		Annotations: annotations,
		AnnotKeys:   annotKeys,
	}, nil
}

// UpdateLabelAnnotationData updates labels and annotations for any resource.
func (c *Client) UpdateLabelAnnotationData(ctx context.Context, contextName string, rt model.ResourceTypeEntry, namespace, name string, labels, annotations map[string]string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}

	patchData, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels":      labels,
			"annotations": annotations,
		},
	})
	if err != nil {
		return fmt.Errorf("marshaling patch: %w", err)
	}

	if rt.Namespaced {
		_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(ctx, name, k8stypes.MergePatchType, patchData, metav1.PatchOptions{})
	} else {
		_, err = dynClient.Resource(gvr).Patch(ctx, name, k8stypes.MergePatchType, patchData, metav1.PatchOptions{})
	}
	if err != nil {
		return fmt.Errorf("updating labels/annotations: %w", err)
	}
	return nil
}

// GetPodResourceRequests extracts CPU/memory requests and limits from a pod spec.
func (c *Client) GetPodResourceRequests(ctx context.Context, contextName, namespace, podName string) (cpuReq, cpuLim, memReq, memLim int64, err error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	pod, getErr := cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if getErr != nil {
		return 0, 0, 0, 0, fmt.Errorf("getting pod: %w", getErr)
	}

	for _, container := range pod.Spec.Containers {
		if req, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
			cpuReq += req.MilliValue()
		}
		if lim, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
			cpuLim += lim.MilliValue()
		}
		if req, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
			memReq += req.Value()
		}
		if lim, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
			memLim += lim.Value()
		}
	}
	return cpuReq, cpuLim, memReq, memLim, nil
}

// TriggerCronJob creates a Job from a CronJob template.
func (c *Client) TriggerCronJob(ctx context.Context, contextName, namespace, cronJobName string) (string, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return "", err
	}

	cronJob, err := cs.BatchV1().CronJobs(namespace).Get(ctx, cronJobName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting cronjob: %w", err)
	}

	jobName := fmt.Sprintf("%s-manual-%d", cronJobName, time.Now().Unix())
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Annotations: map[string]string{
				"cronjob.kubernetes.io/instantiate": "manual",
			},
		},
		Spec: cronJob.Spec.JobTemplate.Spec,
	}

	created, err := cs.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("creating job: %w", err)
	}

	return created.Name, nil
}

// reorderYAMLFields reorders top-level YAML fields so that common Kubernetes
// fields appear in a logical order: apiVersion, kind, metadata, type, spec, data, status.
func reorderYAMLFields(yamlStr string) string {
	lines := strings.Split(yamlStr, "\n")

	type section struct {
		key   string
		lines []string
	}

	var sections []section
	var current *section

	for _, line := range lines {
		switch {
		case len(line) > 0 && line[0] != ' ' && line[0] != '#' && strings.Contains(line, ":"):
			key := strings.SplitN(line, ":", 2)[0]
			sections = append(sections, section{key: key})
			current = &sections[len(sections)-1]
			current.lines = append(current.lines, line)
		case current != nil:
			current.lines = append(current.lines, line)
		default:
			// Preamble (comments, etc.)
			sections = append(sections, section{key: "", lines: []string{line}})
		}
	}

	priority := map[string]int{
		"apiVersion": 0, "kind": 1, "metadata": 2, "type": 3,
		"spec": 4, "data": 5, "stringData": 6, "status": 7,
	}

	sort.SliceStable(sections, func(i, j int) bool {
		pi, oki := priority[sections[i].key]
		pj, okj := priority[sections[j].key]
		if oki && okj {
			return pi < pj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return false
	})

	var result []string
	for _, s := range sections {
		result = append(result, s.lines...)
	}
	return strings.Join(result, "\n")
}

// formatRelativeTime returns a human-readable relative time string.
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// CheckRBAC checks what verbs the current user can perform on the given resource.
func (c *Client) CheckRBAC(ctx context.Context, contextName, namespace, group, resource string) ([]RBACCheck, error) {
	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	verbs := []string{"get", "list", "watch", "create", "update", "patch", "delete"}
	results := make([]RBACCheck, 0, len(verbs))

	for _, verb := range verbs {
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Namespace: namespace,
					Verb:      verb,
					Group:     group,
					Resource:  resource,
				},
			},
		}

		result, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("RBAC check failed for verb %s: %w", verb, err)
		}
		results = append(results, RBACCheck{
			Verb:    verb,
			Allowed: result.Status.Allowed,
		})
	}

	return results, nil
}

// GetNamespaceQuotas lists ResourceQuota objects in the given namespace
// and computes per-resource usage percentages.
func (c *Client) GetNamespaceQuotas(ctx context.Context, kubeCtx, namespace string) ([]QuotaInfo, error) {
	dynClient, err := c.dynamicForContext(kubeCtx)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"}

	var lister dynamic.ResourceInterface
	if namespace != "" {
		lister = dynClient.Resource(gvr).Namespace(namespace)
	} else {
		lister = dynClient.Resource(gvr)
	}

	list, err := lister.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing resourcequotas: %w", err)
	}

	quotas := make([]QuotaInfo, 0, len(list.Items))
	for _, item := range list.Items {
		qi := QuotaInfo{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
		}

		spec, _ := item.Object["spec"].(map[string]interface{})
		status, _ := item.Object["status"].(map[string]interface{})

		hardMap, _ := spec["hard"].(map[string]interface{})
		usedMap := map[string]interface{}{}
		if status != nil {
			usedMap, _ = status["used"].(map[string]interface{})
		}

		// Collect resource names from hard limits.
		for resName, hardVal := range hardMap {
			hardStr := fmt.Sprintf("%v", hardVal)
			usedStr := "0"
			if uv, ok := usedMap[resName]; ok {
				usedStr = fmt.Sprintf("%v", uv)
			}

			pct := computeQuotaPercent(resName, hardStr, usedStr)

			qi.Resources = append(qi.Resources, QuotaResource{
				Name:    resName,
				Hard:    hardStr,
				Used:    usedStr,
				Percent: pct,
			})
		}

		// Sort resources by name for stable display order.
		sort.Slice(qi.Resources, func(i, j int) bool {
			return qi.Resources[i].Name < qi.Resources[j].Name
		})

		quotas = append(quotas, qi)
	}

	return quotas, nil
}

// computeQuotaPercent computes the usage percentage for a quota resource.
// For resources like cpu and memory, it parses Kubernetes quantity strings.
// For simple numeric resources (pods, services, etc.), it parses as integers.
func computeQuotaPercent(_, hardStr, usedStr string) float64 {
	// Try to parse as Kubernetes quantities (handles cpu, memory, storage, etc.)
	hardQty, errH := resource.ParseQuantity(hardStr)
	usedQty, errU := resource.ParseQuantity(usedStr)
	if errH == nil && errU == nil {
		hardVal := hardQty.AsApproximateFloat64()
		usedVal := usedQty.AsApproximateFloat64()
		if hardVal > 0 {
			pct := (usedVal / hardVal) * 100
			if pct > 100 {
				pct = 100
			}
			return pct
		}
		return 0
	}

	// Fallback: shouldn't normally be reached since resource.ParseQuantity
	// handles both quantities and plain integers, but guard against it.
	return 0
}

// EventInfo holds a single Kubernetes event with its key fields.
type EventInfo struct {
	Timestamp    time.Time
	Type         string // "Normal" or "Warning"
	Reason       string
	Message      string
	Source       string // e.g. "kubelet", "scheduler"
	Count        int32
	InvolvedName string
	InvolvedKind string
}

// GetResourceEvents fetches events related to the named resource and its owned
// resources (using a name-prefix heuristic). Events are returned sorted by
// timestamp descending (most recent first).
func (c *Client) GetResourceEvents(ctx context.Context, kubeCtx, namespace, name, kind string) ([]EventInfo, error) {
	dynClient, err := c.dynamicForContext(kubeCtx)
	if err != nil {
		return nil, err
	}

	eventGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "events",
	}

	list, err := dynClient.Resource(eventGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing events: %w", err)
	}

	namePrefix := name + "-"
	events := make([]EventInfo, 0, len(list.Items))
	for _, item := range list.Items {
		involved, _, _ := unstructured.NestedMap(item.Object, "involvedObject")
		if involved == nil {
			continue
		}
		involvedName, _ := involved["name"].(string)
		if involvedName != name && !strings.HasPrefix(involvedName, namePrefix) {
			continue
		}

		involvedKind, _ := involved["kind"].(string)

		eventType, _, _ := unstructured.NestedString(item.Object, "type")
		reason, _, _ := unstructured.NestedString(item.Object, "reason")
		message, _, _ := unstructured.NestedString(item.Object, "message")

		// Extract source component.
		source, _, _ := unstructured.NestedString(item.Object, "source", "component")
		if source == "" {
			source, _, _ = unstructured.NestedString(item.Object, "reportingComponent")
		}

		// Extract count.
		countVal, _, _ := unstructured.NestedInt64(item.Object, "count")

		// Extract timestamp: prefer lastTimestamp, fall back to eventTime, then metadata.creationTimestamp.
		var ts time.Time
		if lastTS, _, _ := unstructured.NestedString(item.Object, "lastTimestamp"); lastTS != "" {
			ts, _ = time.Parse(time.RFC3339, lastTS)
		}
		if ts.IsZero() {
			if eventTime, _, _ := unstructured.NestedString(item.Object, "eventTime"); eventTime != "" {
				ts, _ = time.Parse(time.RFC3339, eventTime)
			}
		}
		if ts.IsZero() {
			if ct, _, _ := unstructured.NestedString(item.Object, "metadata", "creationTimestamp"); ct != "" {
				ts, _ = time.Parse(time.RFC3339, ct)
			}
		}

		events = append(events, EventInfo{
			Timestamp:    ts,
			Type:         eventType,
			Reason:       reason,
			Message:      message,
			Source:       source,
			Count:        int32(countVal),
			InvolvedName: involvedName,
			InvolvedKind: involvedKind,
		})
	}

	// Sort by timestamp descending (most recent first).
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	return events, nil
}

// PatchLabels patches the labels on a resource using a merge patch.
func (c *Client) PatchLabels(ctx context.Context, contextName, namespace, name string, gvr schema.GroupVersionResource, labels map[string]interface{}) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": labels,
		},
	}

	data, err := json.Marshal(patch)
	if err != nil {
		return err
	}

	if namespace != "" {
		_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(ctx, name, k8stypes.MergePatchType, data, metav1.PatchOptions{})
	} else {
		_, err = dynClient.Resource(gvr).Patch(ctx, name, k8stypes.MergePatchType, data, metav1.PatchOptions{})
	}
	return err
}

// PatchAnnotations patches the annotations on a resource using a merge patch.
func (c *Client) PatchAnnotations(ctx context.Context, contextName, namespace, name string, gvr schema.GroupVersionResource, annotations map[string]interface{}) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotations,
		},
	}

	data, err := json.Marshal(patch)
	if err != nil {
		return err
	}

	if namespace != "" {
		_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(ctx, name, k8stypes.MergePatchType, data, metav1.PatchOptions{})
	} else {
		_, err = dynClient.Resource(gvr).Patch(ctx, name, k8stypes.MergePatchType, data, metav1.PatchOptions{})
	}
	return err
}
