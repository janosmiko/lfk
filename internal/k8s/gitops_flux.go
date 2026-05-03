package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/model"
)

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

	status, ok := obj.Object["status"].(map[string]any)
	if !ok {
		return nil, nil
	}

	inventory, ok := status["inventory"].(map[string]any)
	if !ok {
		return nil, nil
	}

	entries, ok := inventory["entries"].([]any)
	if !ok || len(entries) == 0 {
		return nil, nil
	}

	items := make([]model.Item, 0, len(entries))
	for _, entry := range entries {
		e, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		id, _ := e["id"].(string)
		if id == "" {
			continue
		}

		parts := strings.Split(id, "_")
		if len(parts) < 4 {
			continue
		}

		entryNS := parts[0]
		entryName := parts[1]
		entryGroup := parts[2]
		entryKind := parts[3]

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
	patch := fmt.Appendf(nil, `{"metadata":{"annotations":{"reconcile.fluxcd.io/requestedAt":"%s"}}}`, now)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("reconciling %s %s: %w", gvr.Resource, name, err)
	}
	return nil
}

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

// ForceRefreshExternalSecret triggers a force sync on an ExternalSecret,
// ClusterExternalSecret, or PushSecret by setting the
// force-sync.external-secrets.io/force-sync annotation to the current timestamp.
func (c *Client) ForceRefreshExternalSecret(contextName, namespace, name string, gvr schema.GroupVersionResource) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	now := time.Now().Format(time.RFC3339Nano)
	patch := fmt.Appendf(nil, `{"metadata":{"annotations":{"force-sync.external-secrets.io/force-sync":"%s"}}}`, now)

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

	patch := []byte(`{"metadata":{"annotations":{"autoscaling.keda.sh/paused-replicas":null}}}`)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("unpausing %s %s: %w", gvr.Resource, name, err)
	}
	return nil
}
