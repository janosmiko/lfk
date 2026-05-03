package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

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
	newWf := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Workflow",
		"metadata": map[string]any{
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

	ref := map[string]any{
		"name": templateName,
	}
	if clusterScope {
		ref["clusterScope"] = true
	}

	newWf := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Workflow",
		"metadata": map[string]any{
			"name":      wfName,
			"namespace": namespace,
		},
		"spec": map[string]any{
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

	status, _ := wf.Object["status"].(map[string]any)
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

	nodes, _ := status["nodes"].(map[string]any)
	if len(nodes) > 0 {
		fmt.Fprintf(&b, "%-45s %-15s %-12s %s\n", "NODE", "TYPE", "PHASE", "DURATION")
		b.WriteString(strings.Repeat("-", 90))
		b.WriteString("\n")

		var rootID string
		childrenOf := make(map[string][]string, len(nodes))
		for id, n := range nodes {
			node, ok := n.(map[string]any)
			if !ok {
				continue
			}
			nodeName, _ := node["name"].(string)
			if nodeName == name {
				rootID = id
			}
			if kids, ok := node["children"].([]any); ok {
				for _, k := range kids {
					if s, ok := k.(string); ok {
						childrenOf[id] = append(childrenOf[id], s)
					}
				}
			}
		}

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
		for id := range nodes {
			if !seen[id] {
				orderedKeys = append(orderedKeys, id)
			}
		}

		for _, key := range orderedKeys {
			node, ok := nodes[key].(map[string]any)
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
