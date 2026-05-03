package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/model"
)

func (c *Client) getPodsViaReplicaSets(ctx context.Context, dynClient dynamic.Interface, namespace, deploymentName string) ([]model.Item, error) {
	rsGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	rsList, err := dynClient.Resource(rsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing replicasets: %w", err)
	}

	var rsNames []string
	for _, rs := range rsList.Items {
		for _, ref := range rs.GetOwnerReferences() {
			if ref.Kind == "Deployment" && ref.Name == deploymentName {
				rsNames = append(rsNames, rs.GetName())
			}
		}
	}

	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	rsSet := make(map[string]bool, len(rsNames))
	for _, n := range rsNames {
		rsSet[n] = true
	}

	var items []model.Item
	for _, pod := range podList.Items {
		for _, ref := range pod.GetOwnerReferences() {
			if ref.Kind == "ReplicaSet" && rsSet[ref.Name] {
				ti := model.Item{
					Name:      pod.GetName(),
					Namespace: pod.GetNamespace(),
					Kind:      "Pod",
					Status:    extractStatus(pod.Object),
				}
				creationTS := pod.GetCreationTimestamp()
				if !creationTS.IsZero() {
					ti.CreatedAt = creationTS.Time
					ti.Age = formatAge(time.Since(creationTS.Time))
				}
				populateResourceDetails(&ti, pod.Object, "Pod")
				items = append(items, ti)
				break
			}
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (c *Client) getPodsByOwner(ctx context.Context, dynClient dynamic.Interface, namespace, ownerKind, ownerName string) ([]model.Item, error) {
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var items []model.Item
	for _, pod := range podList.Items {
		for _, ref := range pod.GetOwnerReferences() {
			if ref.Kind == ownerKind && ref.Name == ownerName {
				ti := model.Item{
					Name:      pod.GetName(),
					Namespace: pod.GetNamespace(),
					Kind:      "Pod",
					Status:    extractStatus(pod.Object),
				}
				creationTS := pod.GetCreationTimestamp()
				if !creationTS.IsZero() {
					ti.CreatedAt = creationTS.Time
					ti.Age = formatAge(time.Since(creationTS.Time))
				}
				populateResourceDetails(&ti, pod.Object, "Pod")
				items = append(items, ti)
				break
			}
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (c *Client) getPodsOnNode(ctx context.Context, dynClient dynamic.Interface, nodeName string) ([]model.Item, error) {
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods on node %s: %w", nodeName, err)
	}

	items := make([]model.Item, 0, len(podList.Items))
	for _, pod := range podList.Items {
		ti := model.Item{
			Name:      pod.GetName(),
			Namespace: pod.GetNamespace(),
			Kind:      "Pod",
			Status:    extractStatus(pod.Object),
		}
		creationTS := pod.GetCreationTimestamp()
		if !creationTS.IsZero() {
			ti.CreatedAt = creationTS.Time
			ti.Age = formatAge(time.Since(creationTS.Time))
		}
		populateResourceDetails(&ti, pod.Object, "Pod")
		items = append(items, ti)
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (c *Client) getPodsUsingPVC(ctx context.Context, dynClient dynamic.Interface, namespace, pvcName string) ([]model.Item, error) {
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}
	items := make([]model.Item, 0)
	for _, pod := range podList.Items {
		if !podReferencesPVC(pod.Object, pvcName) {
			continue
		}
		ti := model.Item{
			Name:      pod.GetName(),
			Namespace: pod.GetNamespace(),
			Kind:      "Pod",
			Status:    extractStatus(pod.Object),
		}
		creationTS := pod.GetCreationTimestamp()
		if !creationTS.IsZero() {
			ti.CreatedAt = creationTS.Time
			ti.Age = formatAge(time.Since(creationTS.Time))
		}
		populateResourceDetails(&ti, pod.Object, "Pod")
		items = append(items, ti)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func podReferencesPVC(podObj map[string]any, pvcName string) bool {
	spec, ok := podObj["spec"].(map[string]any)
	if !ok {
		return false
	}
	volumes, ok := spec["volumes"].([]any)
	if !ok {
		return false
	}
	for _, v := range volumes {
		vol, ok := v.(map[string]any)
		if !ok {
			continue
		}
		pvc, ok := vol["persistentVolumeClaim"].(map[string]any)
		if !ok {
			continue
		}
		if claim, _ := pvc["claimName"].(string); claim == pvcName {
			return true
		}
	}
	return false
}

func (c *Client) getJobsByOwner(ctx context.Context, dynClient dynamic.Interface, namespace, cronJobName string) ([]model.Item, error) {
	jobGVR := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	jobList, err := dynClient.Resource(jobGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}

	var items []model.Item
	for _, job := range jobList.Items {
		for _, ref := range job.GetOwnerReferences() {
			if ref.Kind == "CronJob" && ref.Name == cronJobName {
				items = append(items, model.Item{
					Name:      job.GetName(),
					Namespace: job.GetNamespace(),
					Kind:      "Job",
					Status:    extractStatus(job.Object),
				})
				break
			}
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (c *Client) getPodsForService(ctx context.Context, contextName, namespace, serviceName string) ([]model.Item, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	svc, err := cs.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting service %s: %w", serviceName, err)
	}

	if len(svc.Spec.Selector) == 0 {
		return nil, nil
	}

	selectorParts := make([]string, 0, len(svc.Spec.Selector))
	for k, v := range svc.Spec.Selector {
		selectorParts = append(selectorParts, k+"="+v)
	}
	sort.Strings(selectorParts)
	labelSelector := strings.Join(selectorParts, ",")

	podList, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods for service: %w", err)
	}

	items := make([]model.Item, 0, len(podList.Items))
	for _, pod := range podList.Items {
		items = append(items, model.Item{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Kind:      "Pod",
			Status:    string(pod.Status.Phase),
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}
