package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

func (c *Client) getArgoManagedResources(ctx context.Context, dynClient dynamic.Interface, contextName, namespace, appName string) ([]model.Item, error) {
	appGVR := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	app, err := dynClient.Resource(appGVR).Namespace(namespace).Get(ctx, appName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting application %s: %w", appName, err)
	}

	statusMap, _ := app.Object["status"].(map[string]any)
	resources, _ := statusMap["resources"].([]any)

	if len(resources) > 0 {
		items := argoStatusResourcesToItems(resources)
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		return items, nil
	}

	logger.Info("ArgoCD app has no status.resources, falling back to label discovery", "app", appName)

	targetNs := argoDestinationNamespace(app, namespace)
	items := c.argoFallbackDiscovery(ctx, contextName, targetNs, appName)

	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func argoStatusResourcesToItems(resources []any) []model.Item {
	var items []model.Item
	for _, r := range resources {
		res, ok := r.(map[string]any)
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
		if health, ok := res["health"].(map[string]any); ok {
			healthStatus, _ = health["status"].(string)
		}

		status := healthStatus
		if syncStatus != "" && healthStatus != "" {
			status = healthStatus + "/" + syncStatus
		} else if syncStatus != "" {
			status = syncStatus
		}

		extra := ""
		if group != "" || version != "" {
			extra = group + "/" + version
		}

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
	return items
}

func argoDestinationNamespace(app *unstructured.Unstructured, defaultNs string) string {
	if specMap, ok := app.Object["spec"].(map[string]any); ok {
		if dest, ok := specMap["destination"].(map[string]any); ok {
			if dns, ok := dest["namespace"].(string); ok && dns != "" {
				return dns
			}
		}
	}
	return defaultNs
}

func (c *Client) argoFallbackDiscovery(ctx context.Context, contextName, targetNs, appName string) []model.Item {
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
			continue
		}

		if depList, err := cs.AppsV1().Deployments(targetNs).List(ctx, opts); err == nil {
			for _, d := range depList.Items {
				appendIfUnseen(&items, seen, "Deployment", d.Name, d.Namespace, d.CreationTimestamp.Time)
			}
		}
		if svcList, err := cs.CoreV1().Services(targetNs).List(ctx, opts); err == nil {
			for _, s := range svcList.Items {
				appendIfUnseen(&items, seen, "Service", s.Name, s.Namespace, s.CreationTimestamp.Time)
			}
		}
		if cmList, err := cs.CoreV1().ConfigMaps(targetNs).List(ctx, opts); err == nil {
			for _, cm := range cmList.Items {
				appendIfUnseen(&items, seen, "ConfigMap", cm.Name, cm.Namespace, cm.CreationTimestamp.Time)
			}
		}
		if ssList, err := cs.AppsV1().StatefulSets(targetNs).List(ctx, opts); err == nil {
			for _, ss := range ssList.Items {
				appendIfUnseen(&items, seen, "StatefulSet", ss.Name, ss.Namespace, ss.CreationTimestamp.Time)
			}
		}
		if dsList, err := cs.AppsV1().DaemonSets(targetNs).List(ctx, opts); err == nil {
			for _, ds := range dsList.Items {
				appendIfUnseen(&items, seen, "DaemonSet", ds.Name, ds.Namespace, ds.CreationTimestamp.Time)
			}
		}
	}

	if len(items) == 0 {
		logger.Info("ArgoCD fallback: no resources found via label selectors", "app", appName, "namespace", targetNs)
	} else {
		logger.Info("ArgoCD fallback: found resources via label selectors", "app", appName, "count", len(items))
	}
	return items
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
		app, err := dynClient.Resource(appGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("getting application %s: %w", name, err)
		}

		if app.Object["operation"] != nil {
			return fmt.Errorf("another operation is already in progress for %s", name)
		}

		strategy := "hook"
		if applyOnly {
			strategy = "apply"
		}

		syncBlock := map[string]any{
			"syncStrategy": map[string]any{
				strategy: map[string]any{},
			},
		}

		if spec, ok := app.Object["spec"].(map[string]any); ok {
			if syncPolicy, ok := spec["syncPolicy"].(map[string]any); ok {
				if syncOptions, ok := syncPolicy["syncOptions"].([]any); ok && len(syncOptions) > 0 {
					syncBlock["syncOptions"] = syncOptions
				}
				if automated, ok := syncPolicy["automated"].(map[string]any); ok {
					if prune, ok := automated["prune"].(bool); ok {
						syncBlock["prune"] = prune
					}
				}
			}
		}

		if status, ok := app.Object["status"].(map[string]any); ok {
			if opState, ok := status["operationState"].(map[string]any); ok {
				if op, ok := opState["operation"].(map[string]any); ok {
					if syncMap, ok := op["sync"].(map[string]any); ok {
						delete(syncMap, "syncStrategy")
					}
				}
			}
		}

		app.Object["operation"] = map[string]any{
			"initiatedBy": map[string]any{
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

	status, _ := app.Object["status"].(map[string]any)
	if status == nil {
		return fmt.Errorf("no sync operation in progress")
	}
	opState, _ := status["operationState"].(map[string]any)
	if opState == nil {
		return fmt.Errorf("no sync operation in progress")
	}
	phase, _ := opState["phase"].(string)
	if phase != "Running" {
		return fmt.Errorf("no running sync operation to terminate (phase: %s)", phase)
	}

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

// RefreshArgoAppSet triggers a refresh on an ArgoCD ApplicationSet by setting
// the argocd.argoproj.io/refresh annotation.
func (c *Client) RefreshArgoAppSet(contextName, namespace, name string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applicationsets"}

	patch := []byte(`{"metadata":{"annotations":{"argocd.argoproj.io/refresh":"true"}}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("refreshing applicationset %s: %w", name, err)
	}
	return nil
}

// GetAutoSyncConfig reads the autosync configuration from an ArgoCD Application.
func (c *Client) GetAutoSyncConfig(ctx context.Context, contextName, namespace, name string) (enabled, selfHeal, prune bool, err error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return false, false, false, err
	}

	gvr := schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	app, err := dynClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, false, false, fmt.Errorf("getting application: %w", err)
	}

	automated, found, _ := unstructured.NestedMap(app.Object, "spec", "syncPolicy", "automated")
	if !found || automated == nil {
		return false, false, false, nil
	}

	enabled = true
	if sh, ok := automated["selfHeal"].(bool); ok {
		selfHeal = sh
	}
	if pr, ok := automated["prune"].(bool); ok {
		prune = pr
	}

	return enabled, selfHeal, prune, nil
}

// UpdateAutoSyncConfig updates the autosync configuration for an ArgoCD Application.
func (c *Client) UpdateAutoSyncConfig(ctx context.Context, contextName, namespace, name string, enabled, selfHeal, prune bool) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	var patchData []byte
	if !enabled {
		patchData, err = json.Marshal(map[string]any{
			"spec": map[string]any{
				"syncPolicy": map[string]any{
					"automated": nil,
				},
			},
		})
	} else {
		patchData, err = json.Marshal(map[string]any{
			"spec": map[string]any{
				"syncPolicy": map[string]any{
					"automated": map[string]any{
						"selfHeal": selfHeal,
						"prune":    prune,
					},
				},
			},
		})
	}
	if err != nil {
		return fmt.Errorf("marshaling patch: %w", err)
	}

	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		ctx, name, k8stypes.MergePatchType, patchData, metav1.PatchOptions{},
	)

	return err
}
