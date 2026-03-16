package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"

	"sigs.k8s.io/yaml"
)

// --- Helm helpers ---

// GetHelmReleases lists Helm releases by finding secrets with owner=helm label.
func (c *Client) GetHelmReleases(ctx context.Context, contextName, namespace string) ([]model.Item, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	listOpts := metav1.ListOptions{
		LabelSelector: "owner=helm",
	}

	type secretInfo struct {
		name      string
		namespace string
		labels    map[string]string
		created   time.Time
	}

	var secrets []secretInfo

	if namespace == "" {
		list, listErr := cs.CoreV1().Secrets("").List(ctx, listOpts)
		if listErr != nil {
			return nil, fmt.Errorf("listing helm secrets: %w", listErr)
		}
		for _, s := range list.Items {
			secrets = append(secrets, secretInfo{s.Name, s.Namespace, s.Labels, s.CreationTimestamp.Time})
		}
	} else {
		list, listErr := cs.CoreV1().Secrets(namespace).List(ctx, listOpts)
		if listErr != nil {
			return nil, fmt.Errorf("listing helm secrets: %w", listErr)
		}
		for _, s := range list.Items {
			secrets = append(secrets, secretInfo{s.Name, s.Namespace, s.Labels, s.CreationTimestamp.Time})
		}
	}

	// Group by release name, keep only latest version.
	type releaseInfo struct {
		name      string
		namespace string
		status    string
		version   string
		created   time.Time
	}

	releases := make(map[string]*releaseInfo)
	for _, s := range secrets {
		relName := s.labels["name"]
		relStatus := s.labels["status"]
		relVersion := s.labels["version"]
		if relName == "" {
			continue
		}

		key := s.namespace + "/" + relName
		existing, ok := releases[key]
		if !ok || s.created.After(existing.created) {
			releases[key] = &releaseInfo{
				name:      relName,
				namespace: s.namespace,
				status:    relStatus,
				version:   relVersion,
				created:   s.created,
			}
		}
	}

	items := make([]model.Item, 0, len(releases))
	for _, rel := range releases {
		status := rel.status
		// Capitalize first letter for display.
		if len(status) > 0 {
			status = strings.ToUpper(status[:1]) + status[1:]
		}

		ti := model.Item{
			Name:      rel.name,
			Kind:      "HelmRelease",
			Status:    status,
			Age:       formatAge(time.Since(rel.created)),
			CreatedAt: rel.created,
			Extra:     "v" + rel.version,
		}
		if namespace == "" {
			ti.Namespace = rel.namespace
		}
		items = append(items, ti)
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// GetHelmReleaseYAML returns a summary YAML for a Helm release.
func (c *Client) GetHelmReleaseYAML(ctx context.Context, contextName, namespace, name string) (string, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return "", err
	}

	listOpts := metav1.ListOptions{
		LabelSelector: "owner=helm,name=" + name,
	}
	secretList, err := cs.CoreV1().Secrets(namespace).List(ctx, listOpts)
	if err != nil {
		return "", fmt.Errorf("listing helm secrets: %w", err)
	}

	if len(secretList.Items) == 0 {
		return "", fmt.Errorf("no helm release found for %s", name)
	}

	// Find the latest version.
	latest := secretList.Items[0]
	for _, s := range secretList.Items[1:] {
		if s.CreationTimestamp.After(latest.CreationTimestamp.Time) {
			latest = s
		}
	}

	// Build a summary (not the compressed data).
	info := map[string]interface{}{
		"name":      latest.Labels["name"],
		"namespace": latest.Namespace,
		"version":   latest.Labels["version"],
		"status":    latest.Labels["status"],
		"created":   latest.CreationTimestamp.Format(time.RFC3339),
		"modified":  latest.Labels["modifiedAt"],
		"secret":    latest.Name,
	}

	data, err := yaml.Marshal(info)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *Client) getHelmManagedResources(ctx context.Context, contextName, namespace, releaseName string) ([]model.Item, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	// Try multiple label selectors: standard Helm label first, then legacy "release" label.
	labelSelectors := []string{
		"app.kubernetes.io/instance=" + releaseName,
		"release=" + releaseName,
	}

	seen := make(map[string]bool) // dedup key: Kind/Name
	var items []model.Item

	for _, labelSelector := range labelSelectors {
		logger.Debug("Helm: listing managed resources", "selector", labelSelector, "namespace", namespace)

		// Deployments
		depList, depErr := cs.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if depErr == nil {
			for _, d := range depList.Items {
				key := "Deployment/" + d.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				var replicas int32 = 1
				if d.Spec.Replicas != nil {
					replicas = *d.Spec.Replicas
				}
				ti := model.Item{Name: d.Name, Kind: "Deployment"}
				if d.Status.AvailableReplicas == replicas {
					ti.Status = "Available"
				} else {
					ti.Status = "Progressing"
				}
				ti.Ready = fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, replicas)
				ti.CreatedAt = d.CreationTimestamp.Time
				ti.Age = formatAge(time.Since(d.CreationTimestamp.Time))
				items = append(items, ti)
			}
		}

		// StatefulSets
		ssList, ssErr := cs.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if ssErr == nil {
			for _, ss := range ssList.Items {
				key := "StatefulSet/" + ss.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				var replicas int32 = 1
				if ss.Spec.Replicas != nil {
					replicas = *ss.Spec.Replicas
				}
				ti := model.Item{Name: ss.Name, Kind: "StatefulSet"}
				ti.Ready = fmt.Sprintf("%d/%d", ss.Status.ReadyReplicas, replicas)
				ti.CreatedAt = ss.CreationTimestamp.Time
				ti.Age = formatAge(time.Since(ss.CreationTimestamp.Time))
				items = append(items, ti)
			}
		}

		// DaemonSets
		dsList, dsErr := cs.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if dsErr == nil {
			for _, ds := range dsList.Items {
				key := "DaemonSet/" + ds.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				ti := model.Item{Name: ds.Name, Kind: "DaemonSet"}
				ti.Ready = fmt.Sprintf("%d/%d", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
				ti.CreatedAt = ds.CreationTimestamp.Time
				ti.Age = formatAge(time.Since(ds.CreationTimestamp.Time))
				items = append(items, ti)
			}
		}

		// Services
		svcList, svcErr := cs.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if svcErr == nil {
			for _, s := range svcList.Items {
				key := "Service/" + s.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, model.Item{
					Name:      s.Name,
					Kind:      "Service",
					Status:    "Active",
					CreatedAt: s.CreationTimestamp.Time,
					Age:       formatAge(time.Since(s.CreationTimestamp.Time)),
				})
			}
		}

		// ConfigMaps
		cmList, cmErr := cs.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if cmErr == nil {
			for _, cm := range cmList.Items {
				key := "ConfigMap/" + cm.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, model.Item{
					Name:      cm.Name,
					Kind:      "ConfigMap",
					CreatedAt: cm.CreationTimestamp.Time,
					Age:       formatAge(time.Since(cm.CreationTimestamp.Time)),
				})
			}
		}

		// Secrets (non-helm-release)
		secList, secErr := cs.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if secErr == nil {
			for _, s := range secList.Items {
				if s.Labels["owner"] == "helm" {
					continue // skip helm release secrets
				}
				key := "Secret/" + s.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, model.Item{
					Name:      s.Name,
					Kind:      "Secret",
					CreatedAt: s.CreationTimestamp.Time,
					Age:       formatAge(time.Since(s.CreationTimestamp.Time)),
				})
			}
		}

		// ServiceAccounts
		saList, saErr := cs.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if saErr == nil {
			for _, sa := range saList.Items {
				key := "ServiceAccount/" + sa.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, model.Item{
					Name:      sa.Name,
					Kind:      "ServiceAccount",
					CreatedAt: sa.CreationTimestamp.Time,
					Age:       formatAge(time.Since(sa.CreationTimestamp.Time)),
				})
			}
		}

		// Ingresses
		ingList, ingErr := cs.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if ingErr == nil {
			for _, ing := range ingList.Items {
				key := "Ingress/" + ing.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, model.Item{
					Name:      ing.Name,
					Kind:      "Ingress",
					CreatedAt: ing.CreationTimestamp.Time,
					Age:       formatAge(time.Since(ing.CreationTimestamp.Time)),
				})
			}
		}
	}

	if len(items) == 0 {
		logger.Info("Helm: no managed resources found for release", "release", releaseName, "namespace", namespace)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}
