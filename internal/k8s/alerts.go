package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
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

	monitoringNamespaces := []string{"monitoring", "prometheus", "observability", "kube-prometheus-stack"}

	// Try Prometheus first.
	promServices := []string{
		"prometheus-kube-prometheus-prometheus",
		"prometheus-server",
		"prometheus",
		"prometheus-operated",
	}

	var lastErr error
	for _, ns := range monitoringNamespaces {
		for _, svc := range promServices {
			result := clientset.CoreV1().Services(ns).ProxyGet("http", svc, "9090", "/api/v1/alerts", nil)
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
	amServices := []string{
		"alertmanager-operated",
		"alertmanager",
		"prometheus-kube-prometheus-alertmanager",
		"alertmanager-main",
	}

	for _, ns := range monitoringNamespaces {
		for _, svc := range amServices {
			result := clientset.CoreV1().Services(ns).ProxyGet("http", svc, "9093", "/api/v2/alerts", nil)
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
	return nil, fmt.Errorf("no Prometheus/Alertmanager service found in well-known namespaces (monitoring, prometheus, observability, kube-prometheus-stack)")
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
	var alerts []AlertInfo
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

	monitoringNamespaces := []string{"monitoring", "prometheus", "observability", "kube-prometheus-stack"}

	// Try Prometheus first.
	promServices := []string{
		"prometheus-kube-prometheus-prometheus",
		"prometheus-server",
		"prometheus",
		"prometheus-operated",
	}

	var lastErr error
	for _, ns := range monitoringNamespaces {
		for _, svc := range promServices {
			result := clientset.CoreV1().Services(ns).ProxyGet("http", svc, "9090", "/api/v1/alerts", nil)
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
	amServices := []string{
		"alertmanager-operated",
		"alertmanager",
		"prometheus-kube-prometheus-alertmanager",
		"alertmanager-main",
	}

	for _, ns := range monitoringNamespaces {
		for _, svc := range amServices {
			result := clientset.CoreV1().Services(ns).ProxyGet("http", svc, "9093", "/api/v2/alerts", nil)
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
	return nil, fmt.Errorf("no Prometheus/Alertmanager service found in well-known namespaces (monitoring, prometheus, observability, kube-prometheus-stack)")
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

	var alerts []AlertInfo
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

	var alerts []AlertInfo
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
	var alerts []AlertInfo
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
