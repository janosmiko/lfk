package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// --- ArgoCD helpers ---

func (c *Client) getArgoManagedResources(ctx context.Context, dynClient dynamic.Interface, contextName, namespace, appName string) ([]model.Item, error) {
	appGVR := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	app, err := dynClient.Resource(appGVR).Namespace(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting application %s: %w", appName, err)
	}

	statusMap, _ := app.Object["status"].(map[string]interface{})
	resources, _ := statusMap["resources"].([]interface{})

	if len(resources) > 0 {
		var items []model.Item
		for _, r := range resources {
			res, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := res["name"].(string)
			kind, _ := res["kind"].(string)
			ns, _ := res["namespace"].(string)
			group, _ := res["group"].(string)
			version, _ := res["version"].(string)
			syncStatus, _ := res["status"].(string)

			healthStatus := ""
			if health, ok := res["health"].(map[string]interface{}); ok {
				healthStatus, _ = health["status"].(string)
			}

			status := healthStatus
			if syncStatus != "" && healthStatus != "" {
				status = healthStatus + "/" + syncStatus
			} else if syncStatus != "" {
				status = syncStatus
			}

			// Store group/version in Extra so the UI can resolve the correct
			// GVR when loading YAML for this owned resource.
			extra := ""
			if group != "" || version != "" {
				extra = group + "/" + version
			}

			// Build API version string for display (e.g. "apps/v1", "v1").
			apiVersion := version
			if group != "" {
				apiVersion = group + "/" + version
			}

			ti := model.Item{
				Name:      name,
				Kind:      kind,
				Namespace: ns,
				Status:    status,
				Extra:     extra,
				Columns: []model.KeyValue{
					{Key: "KIND", Value: kind},
					{Key: "APIVERSION", Value: apiVersion},
				},
			}
			items = append(items, ti)
		}

		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		return items, nil
	}

	// Fallback: status.resources is empty (app may not be synced yet).
	// Try to discover resources by label selectors in the target namespace.
	logger.Info("ArgoCD app has no status.resources, falling back to label discovery", "app", appName)

	targetNs := namespace
	if specMap, ok := app.Object["spec"].(map[string]interface{}); ok {
		if dest, ok := specMap["destination"].(map[string]interface{}); ok {
			if dns, ok := dest["namespace"].(string); ok && dns != "" {
				targetNs = dns
			}
		}
	}

	labelSelectors := []string{
		"app.kubernetes.io/instance=" + appName,
		"argocd.argoproj.io/instance=" + appName,
	}

	seen := make(map[string]bool)
	var items []model.Item

	for _, sel := range labelSelectors {
		logger.Debug("ArgoCD fallback: trying label selector", "selector", sel, "namespace", targetNs)
		opts := metav1.ListOptions{LabelSelector: sel}

		cs, csErr := c.clientsetForContext(contextName)
		if csErr != nil {
			// If we can't get a clientset, try the next selector.
			continue
		}

		// Deployments
		if depList, err := cs.AppsV1().Deployments(targetNs).List(ctx, opts); err == nil {
			for _, d := range depList.Items {
				key := "Deployment/" + d.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, model.Item{
					Name:      d.Name,
					Kind:      "Deployment",
					Namespace: d.Namespace,
					CreatedAt: d.CreationTimestamp.Time,
					Age:       formatAge(time.Since(d.CreationTimestamp.Time)),
				})
			}
		}

		// Services
		if svcList, err := cs.CoreV1().Services(targetNs).List(ctx, opts); err == nil {
			for _, s := range svcList.Items {
				key := "Service/" + s.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, model.Item{
					Name:      s.Name,
					Kind:      "Service",
					Namespace: s.Namespace,
					CreatedAt: s.CreationTimestamp.Time,
					Age:       formatAge(time.Since(s.CreationTimestamp.Time)),
				})
			}
		}

		// ConfigMaps
		if cmList, err := cs.CoreV1().ConfigMaps(targetNs).List(ctx, opts); err == nil {
			for _, cm := range cmList.Items {
				key := "ConfigMap/" + cm.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, model.Item{
					Name:      cm.Name,
					Kind:      "ConfigMap",
					Namespace: cm.Namespace,
					CreatedAt: cm.CreationTimestamp.Time,
					Age:       formatAge(time.Since(cm.CreationTimestamp.Time)),
				})
			}
		}

		// StatefulSets
		if ssList, err := cs.AppsV1().StatefulSets(targetNs).List(ctx, opts); err == nil {
			for _, ss := range ssList.Items {
				key := "StatefulSet/" + ss.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, model.Item{
					Name:      ss.Name,
					Kind:      "StatefulSet",
					Namespace: ss.Namespace,
					CreatedAt: ss.CreationTimestamp.Time,
					Age:       formatAge(time.Since(ss.CreationTimestamp.Time)),
				})
			}
		}

		// DaemonSets
		if dsList, err := cs.AppsV1().DaemonSets(targetNs).List(ctx, opts); err == nil {
			for _, ds := range dsList.Items {
				key := "DaemonSet/" + ds.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, model.Item{
					Name:      ds.Name,
					Kind:      "DaemonSet",
					Namespace: ds.Namespace,
					CreatedAt: ds.CreationTimestamp.Time,
					Age:       formatAge(time.Since(ds.CreationTimestamp.Time)),
				})
			}
		}
	}

	if len(items) == 0 {
		logger.Info("ArgoCD fallback: no resources found via label selectors", "app", appName, "namespace", targetNs)
	} else {
		logger.Info("ArgoCD fallback: found resources via label selectors", "app", appName, "count", len(items))
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

// SyncArgoApp triggers a sync on an ArgoCD Application by setting the operation field.
// It reads the application first to carry over syncOptions (e.g., ServerSideApply=true).
// If applyOnly is true, uses the "apply" strategy (no hooks); otherwise uses "hook" strategy (default).
//
// Replicates what ArgoCD's own API server does (argo.SetAppOperation):
//  1. Get the application
//  2. Set status.operationState = nil (clear stale state)
//  3. Set operation with the desired sync strategy
//  4. Update the full object in one call
//
// See: https://argo-cd.readthedocs.io/en/stable/user-guide/sync-kubectl/
func (c *Client) SyncArgoApp(contextName, namespace, name string, applyOnly bool) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	appGVR := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

	for {
		// Read the application (retry loop handles conflicts).
		app, err := dynClient.Resource(appGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("getting application %s: %w", name, err)
		}

		// Reject if another operation is already in progress.
		if app.Object["operation"] != nil {
			return fmt.Errorf("another operation is already in progress for %s", name)
		}

		// Build the sync operation with exactly one strategy key.
		strategy := "hook"
		if applyOnly {
			strategy = "apply"
		}

		syncBlock := map[string]interface{}{
			"syncStrategy": map[string]interface{}{
				strategy: map[string]interface{}{},
			},
		}

		if spec, ok := app.Object["spec"].(map[string]interface{}); ok {
			if syncPolicy, ok := spec["syncPolicy"].(map[string]interface{}); ok {
				if syncOptions, ok := syncPolicy["syncOptions"].([]interface{}); ok && len(syncOptions) > 0 {
					syncBlock["syncOptions"] = syncOptions
				}
				if automated, ok := syncPolicy["automated"].(map[string]interface{}); ok {
					if prune, ok := automated["prune"].(bool); ok {
						syncBlock["prune"] = prune
					}
				}
			}
		}

		// Clear stale syncStrategy from operationState to prevent ArgoCD from
		// merging the old strategy into the new sync.
		if status, ok := app.Object["status"].(map[string]interface{}); ok {
			if opState, ok := status["operationState"].(map[string]interface{}); ok {
				if op, ok := opState["operation"].(map[string]interface{}); ok {
					if syncMap, ok := op["sync"].(map[string]interface{}); ok {
						delete(syncMap, "syncStrategy")
					}
				}
			}
		}

		// Set operation with exactly one syncStrategy key.
		app.Object["operation"] = map[string]interface{}{
			"initiatedBy": map[string]interface{}{
				"username": "lfk",
			},
			"sync": syncBlock,
		}

		_, err = dynClient.Resource(appGVR).Namespace(namespace).Update(
			context.Background(), app, metav1.UpdateOptions{},
		)
		if err != nil {
			if apierrors.IsConflict(err) {
				logger.Warn("conflict updating application for sync, retrying", "app", name)
				continue
			}
			return fmt.Errorf("syncing application %s: %w", name, err)
		}
		return nil
	}
}

// TerminateArgoSync terminates a running sync operation on an ArgoCD Application
// by setting status.operationState.phase to "Terminating".
func (c *Client) TerminateArgoSync(contextName, namespace, name string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	appGVR := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

	app, err := dynClient.Resource(appGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting application %s: %w", name, err)
	}

	status, _ := app.Object["status"].(map[string]interface{})
	if status == nil {
		return fmt.Errorf("no sync operation in progress")
	}
	opState, _ := status["operationState"].(map[string]interface{})
	if opState == nil {
		return fmt.Errorf("no sync operation in progress")
	}
	phase, _ := opState["phase"].(string)
	if phase != "Running" {
		return fmt.Errorf("no running sync operation to terminate (phase: %s)", phase)
	}

	// Set phase to Terminating and update the full object.
	opState["phase"] = "Terminating"
	_, err = dynClient.Resource(appGVR).Namespace(namespace).Update(
		context.Background(), app, metav1.UpdateOptions{},
	)
	if err != nil {
		return fmt.Errorf("terminating sync for %s: %w", name, err)
	}
	return nil
}

// RefreshArgoApp triggers a hard refresh on an ArgoCD Application by setting
// the argocd.argoproj.io/refresh annotation to "hard".
func (c *Client) RefreshArgoApp(contextName, namespace, name string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	appGVR := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

	patch := []byte(`{"metadata":{"annotations":{"argocd.argoproj.io/refresh":"hard"}}}`)
	_, err = dynClient.Resource(appGVR).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("refreshing application %s: %w", name, err)
	}
	return nil
}

// --- FluxCD helpers ---

// getFluxManagedResources retrieves resources managed by a Flux Kustomization
// from its status.inventory.entries field.
func (c *Client) getFluxManagedResources(ctx context.Context, dynClient dynamic.Interface, namespace, name string) ([]model.Item, error) {
	kustomGVR := schema.GroupVersionResource{
		Group:    "kustomize.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "kustomizations",
	}

	obj, err := dynClient.Resource(kustomGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	status, ok := obj.Object["status"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	inventory, ok := status["inventory"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	entries, ok := inventory["entries"].([]interface{})
	if !ok || len(entries) == 0 {
		return nil, nil
	}

	items := make([]model.Item, 0, len(entries))
	for _, entry := range entries {
		e, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		// Each entry has "id" in format "NAMESPACE_NAME_GROUP_KIND" and "v" for version.
		id, _ := e["id"].(string)
		if id == "" {
			continue
		}

		// Parse the ID: NAMESPACE_NAME_GROUP_KIND format.
		parts := strings.Split(id, "_")
		if len(parts) < 4 {
			continue
		}

		entryNS := parts[0]
		entryName := parts[1]
		entryGroup := parts[2]
		entryKind := parts[3]

		// Map well-known kinds to icons.
		icon := "⧫"
		switch entryKind {
		case "Deployment":
			icon = "◆"
		case "Service":
			icon = "⇌"
		case "ConfigMap":
			icon = "≡"
		case "Secret":
			icon = "⊡"
		case "Pod":
			icon = "⬤"
		case "StatefulSet":
			icon = "◇"
		case "DaemonSet":
			icon = "●"
		case "Ingress":
			icon = "↳"
		case "ServiceAccount":
			icon = "⊕"
		case "Namespace":
			icon = "▣"
		}

		displayName := entryName
		if entryNS != "" && entryNS != namespace {
			displayName = entryNS + "/" + entryName
		}

		items = append(items, model.Item{
			Name:      displayName,
			Namespace: entryNS,
			Kind:      entryKind,
			Icon:      icon,
			Extra:     entryGroup,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

// ReconcileFluxResource triggers reconciliation of a FluxCD resource by setting
// the reconcile.fluxcd.io/requestedAt annotation to the current time.
func (c *Client) ReconcileFluxResource(contextName, namespace, name string, gvr schema.GroupVersionResource) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	now := time.Now().Format(time.RFC3339Nano)
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"reconcile.fluxcd.io/requestedAt":"%s"}}}`, now))
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("reconciling %s %s: %w", gvr.Resource, name, err)
	}
	return nil
}

// SuspendFluxResource sets spec.suspend=true on a FluxCD resource.
func (c *Client) SuspendFluxResource(contextName, namespace, name string, gvr schema.GroupVersionResource) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	patch := []byte(`{"spec":{"suspend":true}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("suspending %s %s: %w", gvr.Resource, name, err)
	}
	return nil
}

// ResumeFluxResource sets spec.suspend=false on a FluxCD resource.
func (c *Client) ResumeFluxResource(contextName, namespace, name string, gvr schema.GroupVersionResource) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	patch := []byte(`{"spec":{"suspend":false}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("resuming %s %s: %w", gvr.Resource, name, err)
	}
	return nil
}
