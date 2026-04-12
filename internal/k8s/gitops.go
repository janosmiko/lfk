package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
		items := argoStatusResourcesToItems(resources)
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		return items, nil
	}

	// Fallback: status.resources is empty (app may not be synced yet).
	// Try to discover resources by label selectors in the target namespace.
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

// argoStatusResourcesToItems converts status.resources entries from an ArgoCD
// Application into model.Item entries.
func argoStatusResourcesToItems(resources []interface{}) []model.Item {
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
	return items
}

// argoDestinationNamespace extracts the destination namespace from an ArgoCD
// Application spec, falling back to the given default.
func argoDestinationNamespace(app *unstructured.Unstructured, defaultNs string) string {
	if specMap, ok := app.Object["spec"].(map[string]interface{}); ok {
		if dest, ok := specMap["destination"].(map[string]interface{}); ok {
			if dns, ok := dest["namespace"].(string); ok && dns != "" {
				return dns
			}
		}
	}
	return defaultNs
}

// argoFallbackDiscovery discovers resources by label selectors when
// status.resources is empty for an ArgoCD Application.
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

		// Deployments
		if depList, err := cs.AppsV1().Deployments(targetNs).List(ctx, opts); err == nil {
			for _, d := range depList.Items {
				appendIfUnseen(&items, seen, "Deployment", d.Name, d.Namespace, d.CreationTimestamp.Time)
			}
		}
		// Services
		if svcList, err := cs.CoreV1().Services(targetNs).List(ctx, opts); err == nil {
			for _, s := range svcList.Items {
				appendIfUnseen(&items, seen, "Service", s.Name, s.Namespace, s.CreationTimestamp.Time)
			}
		}
		// ConfigMaps
		if cmList, err := cs.CoreV1().ConfigMaps(targetNs).List(ctx, opts); err == nil {
			for _, cm := range cmList.Items {
				appendIfUnseen(&items, seen, "ConfigMap", cm.Name, cm.Namespace, cm.CreationTimestamp.Time)
			}
		}
		// StatefulSets
		if ssList, err := cs.AppsV1().StatefulSets(targetNs).List(ctx, opts); err == nil {
			for _, ss := range ssList.Items {
				appendIfUnseen(&items, seen, "StatefulSet", ss.Name, ss.Namespace, ss.CreationTimestamp.Time)
			}
		}
		// DaemonSets
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

// appendIfUnseen appends an item to items if the kind/name key has not been seen.
func appendIfUnseen(items *[]model.Item, seen map[string]bool, kind, name, namespace string, createdAt time.Time) {
	key := kind + "/" + name
	if seen[key] {
		return
	}
	seen[key] = true
	*items = append(*items, model.Item{
		Name:      name,
		Kind:      kind,
		Namespace: namespace,
		CreatedAt: createdAt,
		Age:       formatAge(time.Since(createdAt)),
	})
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
		// Remove automated section entirely.
		patchData, err = json.Marshal(map[string]interface{}{
			"spec": map[string]interface{}{
				"syncPolicy": map[string]interface{}{
					"automated": nil,
				},
			},
		})
	} else {
		patchData, err = json.Marshal(map[string]interface{}{
			"spec": map[string]interface{}{
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
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
			icon = "■"
		case "Service":
			icon = "⇌"
		case "ConfigMap":
			icon = "≡"
		case "Secret":
			icon = "⊡"
		case "Pod":
			icon = "□"
		case "StatefulSet":
			icon = "▥"
		case "DaemonSet":
			icon = "●"
		case "Ingress":
			icon = "↳"
		case "ServiceAccount":
			icon = "⚇"
		case "Namespace":
			icon = "❐"
		}

		// Item.Name must be the raw Kubernetes resource name (no
		// namespace prefix) because it is forwarded to the dynamic client
		// as a resource name when the user loads YAML or labels for the
		// item, and the API server rejects any name containing '/'. The
		// renderer surfaces entryNS via the NAMESPACE column whenever
		// any item carries one.
		items = append(items, model.Item{
			Name:      entryName,
			Namespace: entryNS,
			Kind:      entryKind,
			Icon:      model.Icon{Unicode: icon},
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

// --- cert-manager helpers ---

// ForceRenewCertificate triggers re-issuance of a cert-manager Certificate by
// patching its status to add the Issuing condition (replicating what cmctl renew does).
func (c *Client) ForceRenewCertificate(contextName, namespace, name string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}
	patch := []byte(`{"status":{"conditions":[{"type":"Issuing","status":"True","reason":"ManuallyTriggered","message":"Certificate re-issuance triggered via lfk"}]}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{}, "status",
	)
	if err != nil {
		return fmt.Errorf("triggering renewal for certificate %s: %w", name, err)
	}
	return nil
}

// --- Argo Workflows helpers ---

// SuspendArgoWorkflow sets spec.suspend=true on an Argo Workflow.
func (c *Client) SuspendArgoWorkflow(contextName, namespace, name string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "workflows"}
	patch := []byte(`{"spec":{"suspend":true}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("suspending workflow %s: %w", name, err)
	}
	return nil
}

// ResumeArgoWorkflow sets spec.suspend=false on an Argo Workflow.
func (c *Client) ResumeArgoWorkflow(contextName, namespace, name string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "workflows"}
	patch := []byte(`{"spec":{"suspend":false}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("resuming workflow %s: %w", name, err)
	}
	return nil
}

// StopArgoWorkflow sets spec.shutdown="Stop" on an Argo Workflow.
// This stops new steps from running but allows exit handlers to execute.
func (c *Client) StopArgoWorkflow(contextName, namespace, name string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "workflows"}
	patch := []byte(`{"spec":{"shutdown":"Stop"}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("stopping workflow %s: %w", name, err)
	}
	return nil
}

// TerminateArgoWorkflow sets spec.shutdown="Terminate" on an Argo Workflow.
// This immediately terminates the workflow without running exit handlers.
func (c *Client) TerminateArgoWorkflow(contextName, namespace, name string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "workflows"}
	patch := []byte(`{"spec":{"shutdown":"Terminate"}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("terminating workflow %s: %w", name, err)
	}
	return nil
}

// ResubmitArgoWorkflow creates a new Workflow from an existing one's spec.
func (c *Client) ResubmitArgoWorkflow(contextName, namespace, name string) (string, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return "", err
	}

	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "workflows"}
	original, err := dynClient.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting workflow %s: %w", name, err)
	}

	spec, ok := original.Object["spec"]
	if !ok {
		return "", fmt.Errorf("workflow %s has no spec", name)
	}

	newName := name + "-resubmit-" + time.Now().Format("20060102-150405")
	newWf := map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Workflow",
		"metadata": map[string]interface{}{
			"name":      newName,
			"namespace": namespace,
		},
		"spec": spec,
	}

	obj := &unstructured.Unstructured{Object: newWf}
	_, err = dynClient.Resource(gvr).Namespace(namespace).Create(context.Background(), obj, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("creating resubmitted workflow: %w", err)
	}
	return newName, nil
}

// SubmitWorkflowFromTemplate creates a new Workflow that references a WorkflowTemplate or
// ClusterWorkflowTemplate. If clusterScope is true, the reference uses clusterScope: true.
func (c *Client) SubmitWorkflowFromTemplate(contextName, namespace, templateName string, clusterScope bool) (string, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return "", err
	}

	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "workflows"}
	wfName := templateName + "-" + time.Now().Format("20060102-150405")

	ref := map[string]interface{}{
		"name": templateName,
	}
	if clusterScope {
		ref["clusterScope"] = true
	}

	newWf := map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Workflow",
		"metadata": map[string]interface{}{
			"name":      wfName,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"workflowTemplateRef": ref,
		},
	}

	obj := &unstructured.Unstructured{Object: newWf}
	_, err = dynClient.Resource(gvr).Namespace(namespace).Create(context.Background(), obj, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("submitting workflow from template %s: %w", templateName, err)
	}
	return wfName, nil
}

// GetWorkflowStatus fetches an Argo Workflow and returns a formatted status string
// showing the phase and each node's name, type, phase, and duration.
func (c *Client) GetWorkflowStatus(contextName, namespace, name string) (string, bool, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return "", false, err
	}

	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "workflows"}
	wf, err := dynClient.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", false, fmt.Errorf("getting workflow %s: %w", name, err)
	}

	status, _ := wf.Object["status"].(map[string]interface{})
	phase, _ := status["phase"].(string)
	startedAt, _ := status["startedAt"].(string)
	finishedAt, _ := status["finishedAt"].(string)
	message, _ := status["message"].(string)

	var b strings.Builder
	fmt.Fprintf(&b, "Workflow:  %s\n", name)
	fmt.Fprintf(&b, "Phase:    %s\n", phase)
	if startedAt != "" {
		fmt.Fprintf(&b, "Started:  %s\n", startedAt)
	}
	if finishedAt != "" {
		fmt.Fprintf(&b, "Finished: %s\n", finishedAt)
	}
	if message != "" {
		fmt.Fprintf(&b, "Message:  %s\n", message)
	}
	b.WriteString("\n")

	// Format nodes table, ordered by DAG/steps execution order.
	nodes, _ := status["nodes"].(map[string]interface{})
	if len(nodes) > 0 {
		fmt.Fprintf(&b, "%-45s %-15s %-12s %s\n", "NODE", "TYPE", "PHASE", "DURATION")
		b.WriteString(strings.Repeat("-", 90))
		b.WriteString("\n")

		// Walk the DAG via children arrays to preserve execution order.
		// Find the root node (whose name matches the workflow name).
		var rootID string
		childrenOf := make(map[string][]string, len(nodes))
		for id, n := range nodes {
			node, ok := n.(map[string]interface{})
			if !ok {
				continue
			}
			nodeName, _ := node["name"].(string)
			if nodeName == name {
				rootID = id
			}
			if kids, ok := node["children"].([]interface{}); ok {
				for _, k := range kids {
					if s, ok := k.(string); ok {
						childrenOf[id] = append(childrenOf[id], s)
					}
				}
			}
		}

		// BFS from root to get execution order.
		var orderedKeys []string
		seen := make(map[string]bool)
		queue := []string{rootID}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if seen[cur] || cur == "" {
				continue
			}
			seen[cur] = true
			orderedKeys = append(orderedKeys, cur)
			queue = append(queue, childrenOf[cur]...)
		}
		// Append any unseen nodes (shouldn't happen but safety net).
		for id := range nodes {
			if !seen[id] {
				orderedKeys = append(orderedKeys, id)
			}
		}

		for _, key := range orderedKeys {
			node, ok := nodes[key].(map[string]interface{})
			if !ok {
				continue
			}
			nodeName, _ := node["displayName"].(string)
			if nodeName == "" {
				nodeName, _ = node["name"].(string)
			}
			nodeType, _ := node["type"].(string)
			nodePhase, _ := node["phase"].(string)
			duration := ""
			if ns, ok := node["startedAt"].(string); ok {
				if nf, ok := node["finishedAt"].(string); ok {
					st, _ := time.Parse(time.RFC3339, ns)
					ft, _ := time.Parse(time.RFC3339, nf)
					if !st.IsZero() && !ft.IsZero() {
						duration = ft.Sub(st).Truncate(time.Second).String()
					}
				} else if !phaseIsTerminal(nodePhase) {
					st, _ := time.Parse(time.RFC3339, ns)
					if !st.IsZero() {
						duration = time.Since(st).Truncate(time.Second).String()
					}
				}
			}

			fmt.Fprintf(&b, "%-45s %-15s %-12s %s\n", truncate(nodeName, 45), nodeType, nodePhase, duration)
		}
	} else {
		b.WriteString("No nodes yet.\n")
	}

	running := phase == "" || phase == "Running" || phase == "Pending"
	return b.String(), running, nil
}

func phaseIsTerminal(phase string) bool {
	return phase == "Succeeded" || phase == "Failed" || phase == "Error" || phase == "Skipped" || phase == "Omitted"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "~"
}

// SuspendCronWorkflow sets spec.suspend=true on an Argo CronWorkflow.
func (c *Client) SuspendCronWorkflow(contextName, namespace, name string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "cronworkflows"}
	patch := []byte(`{"spec":{"suspend":true}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("suspending cron workflow %s: %w", name, err)
	}
	return nil
}

// ResumeCronWorkflow sets spec.suspend=false on an Argo CronWorkflow.
func (c *Client) ResumeCronWorkflow(contextName, namespace, name string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "cronworkflows"}
	patch := []byte(`{"spec":{"suspend":false}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("resuming cron workflow %s: %w", name, err)
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

// --- External Secrets Operator helpers ---

// ForceRefreshExternalSecret triggers a force sync on an ExternalSecret,
// ClusterExternalSecret, or PushSecret by setting the
// force-sync.external-secrets.io/force-sync annotation to the current timestamp.
func (c *Client) ForceRefreshExternalSecret(contextName, namespace, name string, gvr schema.GroupVersionResource) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	now := time.Now().Format(time.RFC3339Nano)
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"force-sync.external-secrets.io/force-sync":"%s"}}}`, now))

	var patchErr error
	if namespace != "" {
		_, patchErr = dynClient.Resource(gvr).Namespace(namespace).Patch(
			context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
		)
	} else {
		_, patchErr = dynClient.Resource(gvr).Patch(
			context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
		)
	}
	if patchErr != nil {
		return fmt.Errorf("force refreshing %s %s: %w", gvr.Resource, name, patchErr)
	}
	return nil
}

// --- KEDA helpers ---

// PauseKEDAResource pauses a KEDA ScaledObject or ScaledJob by setting the
// autoscaling.keda.sh/paused-replicas annotation to "0".
func (c *Client) PauseKEDAResource(contextName, namespace, name string, gvr schema.GroupVersionResource) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	patch := []byte(`{"metadata":{"annotations":{"autoscaling.keda.sh/paused-replicas":"0"}}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("pausing %s %s: %w", gvr.Resource, name, err)
	}
	return nil
}

// UnpauseKEDAResource unpauses a KEDA ScaledObject or ScaledJob by removing
// the autoscaling.keda.sh/paused-replicas annotation.
func (c *Client) UnpauseKEDAResource(contextName, namespace, name string, gvr schema.GroupVersionResource) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	// JSON merge patch with null removes the key.
	patch := []byte(`{"metadata":{"annotations":{"autoscaling.keda.sh/paused-replicas":null}}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("unpausing %s %s: %w", gvr.Resource, name, err)
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
