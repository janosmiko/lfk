package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

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
	logger.Info("Updating Secret data", "context", contextName, "namespace", namespace, "name", name, "keys", sortedKeys(data))
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

// sortedKeys returns the keys of m as a deterministic slice for logging.
// Values are deliberately not logged (Secret values are sensitive).
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
	logger.Info("Updating ConfigMap data", "context", contextName, "namespace", namespace, "name", name, "keys", sortedKeys(data))
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return err
	}

	cm, err := cs.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting configmap for update: %w", err)
	}

	cm.Data = make(map[string]string, len(data))
	maps.Copy(cm.Data, data)

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
// Deleted keys are set to null in the merge patch so the API server removes them.
func (c *Client) UpdateLabelAnnotationData(ctx context.Context, contextName string, rt model.ResourceTypeEntry, namespace, name string, labels, annotations map[string]string) error {
	logger.Info("Updating labels/annotations",
		"context", contextName,
		"namespace", namespace,
		"name", name,
		"kind", rt.Kind,
		"labels", len(labels),
		"annotations", len(annotations))
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}

	// Fetch current resource to detect deleted keys.
	var obj *unstructured.Unstructured
	if rt.Namespaced {
		obj, err = dynClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = dynClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return fmt.Errorf("getting resource for label update: %w", err)
	}

	// Build patch maps with null for deleted keys (merge patch semantics).
	labelPatch := make(map[string]any, len(labels))
	for k, v := range labels {
		labelPatch[k] = v
	}
	for k := range obj.GetLabels() {
		if _, ok := labels[k]; !ok {
			labelPatch[k] = nil // delete
		}
	}

	annotPatch := make(map[string]any, len(annotations))
	for k, v := range annotations {
		annotPatch[k] = v
	}
	for k := range obj.GetAnnotations() {
		if _, ok := annotations[k]; !ok {
			annotPatch[k] = nil // delete
		}
	}

	patchData, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"labels":      labelPatch,
			"annotations": annotPatch,
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
	logger.Info("Triggering CronJob", "context", contextName, "namespace", namespace, "cronjob", cronJobName)
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
