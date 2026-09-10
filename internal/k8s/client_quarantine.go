package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/janosmiko/lfk/internal/logger"
)

// QuarantinedLabelsAnnotation records the label pairs QuarantinePod removed,
// so RestorePod can put back the exact values later.
const QuarantinedLabelsAnnotation = "lfk.janosmiko.dev/quarantined-labels"

// ErrNotQuarantined means the pod carries no quarantine annotation, or its
// value does not parse — RestorePod has nothing to put back.
var ErrNotQuarantined = errors.New("pod is not quarantined")

// QuarantineTargets finds the Services that route to a pod by its labels,
// and which label keys their selectors use. A selector-less Service is
// wired via Endpoints/EndpointSlice, not label matching, so it is skipped.
func (c *Client) QuarantineTargets(ctx context.Context, contextName, namespace string, podLabels map[string]string) (services []string, keys []string, err error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, nil, err
	}
	list, err := cs.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("listing services: %w", err)
	}

	keySet := make(map[string]struct{})
	for _, svc := range list.Items {
		if len(svc.Spec.Selector) == 0 {
			continue
		}
		if !labels.SelectorFromSet(svc.Spec.Selector).Matches(labels.Set(podLabels)) {
			continue
		}
		services = append(services, svc.Name)
		for k := range svc.Spec.Selector {
			keySet[k] = struct{}{}
		}
	}
	sort.Strings(services)

	keys = make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return services, keys, nil
}

// QuarantinePod strips the given label keys from a pod so the Services that
// select it stop routing traffic there, and records the removed pairs in
// QuarantinedLabelsAnnotation so RestorePod can put them back exactly.
func (c *Client) QuarantinePod(ctx context.Context, contextName, namespace, name string, keys []string) (removed map[string]string, err error) {
	logger.Info("Quarantining pod", "context", contextName, "namespace", namespace, "name", name, "keys", len(keys))
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}
	obj, err := dynClient.Resource(podsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod %s: %w", name, err)
	}

	podLabels := obj.GetLabels()
	removed = make(map[string]string, len(keys))
	labelPatch := make(map[string]any, len(keys))
	for _, k := range keys {
		if v, ok := podLabels[k]; ok {
			removed[k] = v
		}
		labelPatch[k] = nil
	}

	removedJSON, err := json.Marshal(removed)
	if err != nil {
		return nil, fmt.Errorf("marshaling quarantine annotation: %w", err)
	}
	patchData, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"labels": labelPatch,
			"annotations": map[string]any{
				QuarantinedLabelsAnnotation: string(removedJSON),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling quarantine patch: %w", err)
	}

	_, err = dynClient.Resource(podsGVR).Namespace(namespace).Patch(
		ctx, name, k8stypes.MergePatchType, patchData, metav1.PatchOptions{FieldManager: FieldManager()},
	)
	if err != nil {
		return nil, fmt.Errorf("quarantining pod %s: %w", name, err)
	}
	return removed, nil
}

// RestorePod puts back the label pairs QuarantinePod recorded and clears
// the annotation, overwriting any key already put back by something else:
// the annotation is the source of truth for what quarantine took.
func (c *Client) RestorePod(ctx context.Context, contextName, namespace, name string) (restored map[string]string, err error) {
	logger.Info("Restoring pod", "context", contextName, "namespace", namespace, "name", name)
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}
	obj, err := dynClient.Resource(podsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod %s: %w", name, err)
	}

	raw, ok := obj.GetAnnotations()[QuarantinedLabelsAnnotation]
	if !ok || raw == "" {
		return nil, ErrNotQuarantined
	}
	restored = make(map[string]string)
	if unmarshalErr := json.Unmarshal([]byte(raw), &restored); unmarshalErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotQuarantined, unmarshalErr)
	}

	labelPatch := make(map[string]any, len(restored))
	for k, v := range restored {
		labelPatch[k] = v
	}
	patchData, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"labels": labelPatch,
			"annotations": map[string]any{
				QuarantinedLabelsAnnotation: nil,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling restore patch: %w", err)
	}

	_, err = dynClient.Resource(podsGVR).Namespace(namespace).Patch(
		ctx, name, k8stypes.MergePatchType, patchData, metav1.PatchOptions{FieldManager: FieldManager()},
	)
	if err != nil {
		return nil, fmt.Errorf("restoring pod %s: %w", name, err)
	}
	return restored, nil
}
