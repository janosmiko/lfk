package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

// AlertInfo holds a single Prometheus/Alertmanager alert relevant to a resource.
type AlertInfo struct {
	Name        string
	State       string // "firing", "pending"
	Severity    string // "critical", "warning", "info"
	Summary     string
	Description string
	Since       time.Time
	Labels      map[string]string
	GrafanaURL  string // optional dashboard link
}

// prometheusAlertsResponse is the JSON structure returned by the Prometheus /api/v1/alerts endpoint.
type prometheusAlertsResponse struct {
	Status string `json:"status"`
	Data   struct {
		Alerts []prometheusAlert `json:"alerts"`
	} `json:"data"`
}

type prometheusAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	State       string            `json:"state"`
	ActiveAt    time.Time         `json:"activeAt"`
}

// GetActiveAlerts retrieves active Prometheus alerts related to a specific resource.
// It uses the Kubernetes API server proxy to reach Prometheus services in well-known
// monitoring namespaces without requiring direct network access or port-forward.
func (c *Client) GetActiveAlerts(ctx context.Context, kubeCtx, namespace, resourceName, resourceKind string) ([]AlertInfo, error) {
	clientset, err := c.clientsetForContext(kubeCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get clientset: %w", err)
	}

	promNs, promSvc, promPort, amNs, amSvc, amPort := resolveMonitoringEndpoints(kubeCtx)

	// Try Prometheus first.
	var lastErr error
	for _, ns := range promNs {
		for _, svc := range promSvc {
			result := clientset.CoreV1().Services(ns).ProxyGet("http", svc, promPort, "/api/v1/alerts", nil)
			data, err := result.DoRaw(ctx)
			if err != nil {
				lastErr = err
				continue
			}

			alerts, err := parseAndFilterAlerts(data, namespace, resourceName, resourceKind)
			if err != nil {
				lastErr = err
				continue
			}
			return alerts, nil
		}
	}

	// Try Alertmanager as fallback.
	for _, ns := range amNs {
		for _, svc := range amSvc {
			result := clientset.CoreV1().Services(ns).ProxyGet("http", svc, amPort, "/api/v2/alerts", nil)
			data, err := result.DoRaw(ctx)
			if err != nil {
				lastErr = err
				continue
			}

			alerts, err := parseAndFilterAlertmanagerAlerts(data, namespace, resourceName, resourceKind)
			if err != nil {
				lastErr = err
				continue
			}
			return alerts, nil
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("no reachable Prometheus/Alertmanager service found (last error: %w)", lastErr)
	}
	return nil, fmt.Errorf("no Prometheus/Alertmanager service found in configured/default monitoring namespaces")
}

// parseAndFilterAlerts unmarshals the Prometheus alerts API response and filters
// alerts relevant to the specified resource.
func parseAndFilterAlerts(data []byte, resourceNs, resourceName, resourceKind string) ([]AlertInfo, error) {
	var response prometheusAlertsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("unmarshal alerts response: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("prometheus returned status: %s", response.Status)
	}

	kindLower := strings.ToLower(resourceKind)
	alerts := make([]AlertInfo, 0, len(response.Data.Alerts))
	for _, a := range response.Data.Alerts {
		if !alertMatchesResource(a.Labels, resourceNs, resourceName, kindLower) {
			continue
		}
		info := AlertInfo{
			Name:        a.Labels["alertname"],
			State:       a.State,
			Severity:    a.Labels["severity"],
			Summary:     a.Annotations["summary"],
			Description: a.Annotations["description"],
			Since:       a.ActiveAt,
			Labels:      a.Labels,
		}
		// Try common Grafana/dashboard URL annotation names.
		for _, key := range []string{"grafana_url", "dashboard_url", "runbook_url"} {
			if url := a.Annotations[key]; url != "" {
				info.GrafanaURL = url
				break
			}
		}
		alerts = append(alerts, info)
	}

	return alerts, nil
}

// GetAllActiveAlerts retrieves all active alerts, optionally filtered by namespace.
// Pass an empty namespace to retrieve alerts across all namespaces.
// It tries Prometheus first, then falls back to Alertmanager if Prometheus is not reachable.
func (c *Client) GetAllActiveAlerts(ctx context.Context, kubeCtx, namespace string) ([]AlertInfo, error) {
	clientset, err := c.clientsetForContext(kubeCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get clientset: %w", err)
	}

	promNs, promSvc, promPort, amNs, amSvc, amPort := resolveMonitoringEndpoints(kubeCtx)

	// Try Prometheus first.
	var lastErr error
	for _, ns := range promNs {
		for _, svc := range promSvc {
			result := clientset.CoreV1().Services(ns).ProxyGet("http", svc, promPort, "/api/v1/alerts", nil)
			data, err := result.DoRaw(ctx)
			if err != nil {
				lastErr = err
				continue
			}

			alerts, err := parseAllAlerts(data, namespace)
			if err != nil {
				lastErr = err
				continue
			}
			return alerts, nil
		}
	}

	// Try Alertmanager as fallback.
	for _, ns := range amNs {
		for _, svc := range amSvc {
			result := clientset.CoreV1().Services(ns).ProxyGet("http", svc, amPort, "/api/v2/alerts", nil)
			data, err := result.DoRaw(ctx)
			if err != nil {
				lastErr = err
				continue
			}

			alerts, err := parseAlertmanagerAlerts(data, namespace)
			if err != nil {
				lastErr = err
				continue
			}
			return alerts, nil
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("no reachable Prometheus/Alertmanager service found (last error: %w)", lastErr)
	}
	return nil, fmt.Errorf("no Prometheus/Alertmanager service found in configured/default monitoring namespaces")
}

// parseAllAlerts unmarshals the Prometheus alerts API response and returns all alerts,
// optionally filtered to a single namespace. Pass an empty namespace to include all.
func parseAllAlerts(data []byte, namespace string) ([]AlertInfo, error) {
	var response prometheusAlertsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("unmarshal alerts response: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("prometheus returned status: %s", response.Status)
	}

	alerts := make([]AlertInfo, 0, len(response.Data.Alerts))
	for _, a := range response.Data.Alerts {
		// When a namespace filter is set, skip alerts from other namespaces.
		if namespace != "" && a.Labels["namespace"] != "" && a.Labels["namespace"] != namespace {
			continue
		}
		info := AlertInfo{
			Name:        a.Labels["alertname"],
			State:       a.State,
			Severity:    a.Labels["severity"],
			Summary:     a.Annotations["summary"],
			Description: a.Annotations["description"],
			Since:       a.ActiveAt,
			Labels:      a.Labels,
		}
		for _, key := range []string{"grafana_url", "dashboard_url", "runbook_url"} {
			if url := a.Annotations[key]; url != "" {
				info.GrafanaURL = url
				break
			}
		}
		alerts = append(alerts, info)
	}

	return alerts, nil
}

// alertmanagerAlert represents a single alert from Alertmanager's /api/v2/alerts endpoint.
type alertmanagerAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Status      struct {
		State string `json:"state"` // "active", "suppressed", "unprocessed"
	} `json:"status"`
	StartsAt time.Time `json:"startsAt"`
}

// parseAlertmanagerAlerts parses the Alertmanager v2 API response and returns alerts,
// optionally filtered to a single namespace.
func parseAlertmanagerAlerts(data []byte, namespace string) ([]AlertInfo, error) {
	var amAlerts []alertmanagerAlert
	if err := json.Unmarshal(data, &amAlerts); err != nil {
		return nil, fmt.Errorf("unmarshal alertmanager response: %w", err)
	}

	alerts := make([]AlertInfo, 0, len(amAlerts))
	for _, a := range amAlerts {
		// Skip suppressed (silenced/inhibited) alerts.
		if a.Status.State == "suppressed" {
			continue
		}
		// Filter by namespace if specified.
		if namespace != "" && a.Labels["namespace"] != "" && a.Labels["namespace"] != namespace {
			continue
		}
		state := "firing"
		if a.Status.State == "unprocessed" {
			state = "pending"
		}
		info := AlertInfo{
			Name:        a.Labels["alertname"],
			State:       state,
			Severity:    a.Labels["severity"],
			Summary:     a.Annotations["summary"],
			Description: a.Annotations["description"],
			Since:       a.StartsAt,
			Labels:      a.Labels,
		}
		for _, key := range []string{"grafana_url", "dashboard_url", "runbook_url"} {
			if url := a.Annotations[key]; url != "" {
				info.GrafanaURL = url
				break
			}
		}
		alerts = append(alerts, info)
	}

	return alerts, nil
}

// parseAndFilterAlertmanagerAlerts parses Alertmanager alerts and filters for a specific resource.
func parseAndFilterAlertmanagerAlerts(data []byte, resourceNs, resourceName, resourceKind string) ([]AlertInfo, error) {
	var amAlerts []alertmanagerAlert
	if err := json.Unmarshal(data, &amAlerts); err != nil {
		return nil, fmt.Errorf("unmarshal alertmanager response: %w", err)
	}

	kindLower := strings.ToLower(resourceKind)
	alerts := make([]AlertInfo, 0, len(amAlerts))
	for _, a := range amAlerts {
		if a.Status.State == "suppressed" {
			continue
		}
		if !alertMatchesResource(a.Labels, resourceNs, resourceName, kindLower) {
			continue
		}
		state := "firing"
		if a.Status.State == "unprocessed" {
			state = "pending"
		}
		info := AlertInfo{
			Name:        a.Labels["alertname"],
			State:       state,
			Severity:    a.Labels["severity"],
			Summary:     a.Annotations["summary"],
			Description: a.Annotations["description"],
			Since:       a.StartsAt,
			Labels:      a.Labels,
		}
		for _, key := range []string{"grafana_url", "dashboard_url", "runbook_url"} {
			if url := a.Annotations[key]; url != "" {
				info.GrafanaURL = url
				break
			}
		}
		alerts = append(alerts, info)
	}

	return alerts, nil
}

// resolveMonitoringEndpoints returns the prometheus and alertmanager service names,
// namespaces, and ports to try for the given cluster context. Uses config overrides
// if available, otherwise falls back to hardcoded defaults.
func resolveMonitoringEndpoints(contextName string) (promNs, promSvc []string, promPort string, amNs, amSvc []string, amPort string) {
	// Default values.
	promNs = []string{"monitoring", "prometheus", "observability", "kube-prometheus-stack"}
	promSvc = []string{"kube-prometheus-stack-prometheus", "prometheus-kube-prometheus-prometheus", "prometheus-server", "prometheus", "prometheus-operated"}
	promPort = "9090"
	amNs = []string{"monitoring", "prometheus", "observability", "kube-prometheus-stack"}
	amSvc = []string{"alertmanager-operated", "alertmanager", "prometheus-kube-prometheus-alertmanager", "alertmanager-main"}
	amPort = "9093"

	// Check for config overrides.
	cfg := model.ConfigMonitoring
	if cfg == nil {
		return
	}

	// Try context-specific config first, then "_global".
	mc, ok := cfg[contextName]
	if !ok {
		mc, ok = cfg["_global"]
	}
	if !ok {
		return
	}

	if mc.Prometheus != nil {
		if len(mc.Prometheus.Namespaces) > 0 {
			promNs = mc.Prometheus.Namespaces
		}
		if len(mc.Prometheus.Services) > 0 {
			promSvc = mc.Prometheus.Services
		}
		if mc.Prometheus.Port != "" {
			promPort = mc.Prometheus.Port
		}
	}
	if mc.Alertmanager != nil {
		if len(mc.Alertmanager.Namespaces) > 0 {
			amNs = mc.Alertmanager.Namespaces
		}
		if len(mc.Alertmanager.Services) > 0 {
			amSvc = mc.Alertmanager.Services
		}
		if mc.Alertmanager.Port != "" {
			amPort = mc.Alertmanager.Port
		}
	}
	return
}

// alertMatchesResource checks whether an alert's labels refer to the given resource.
func alertMatchesResource(labels map[string]string, namespace, name, kindLower string) bool {
	// Namespace must match.
	if labels["namespace"] != namespace {
		return false
	}

	// Check if any resource-identifying label matches the resource name.
	return labels["pod"] == name ||
		labels["deployment"] == name ||
		labels["statefulset"] == name ||
		labels["daemonset"] == name ||
		labels["service"] == name ||
		labels["job"] == name ||
		labels["cronjob"] == name ||
		labels["node"] == name ||
		labels[kindLower] == name ||
		// Pod names often include the deployment/replicaset prefix.
		(labels["pod"] != "" && strings.HasPrefix(labels["pod"], name+"-"))
}
