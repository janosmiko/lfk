package app

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/model"
)

// podResizeRow is one container's editable CPU/memory request and limit
// fields in the Resize Pod overlay.
type podResizeRow struct {
	name   string
	cpuReq TextInput
	cpuLim TextInput
	memReq TextInput
	memLim TextInput
	orig   [4]string // cpuReq, cpuLim, memReq, memLim as prefilled
}

// podResizeState is the Resize Pod overlay: one podResizeRow per container,
// plus the pod-level spec.resources shown read-only.
type podResizeState struct {
	name            string
	resourceVersion string
	containers      []podResizeRow
	podLevel        []model.KeyValue
	field           int
	scroll          int
	restartWarn     []string // container names whose resizePolicy requires a restart
}

// buildPodResizeState reads a Pod's raw object into the overlay's prefill
// state. A nil raw (synthetic item / test) yields an empty state.
func buildPodResizeState(raw map[string]any) podResizeState {
	var st podResizeState
	if raw == nil {
		return st
	}
	st.name, _, _ = unstructured.NestedString(raw, "metadata", "name")
	st.resourceVersion, _, _ = unstructured.NestedString(raw, "metadata", "resourceVersion")

	spec, _ := raw["spec"].(map[string]any)
	if spec == nil {
		return st
	}
	if podResources, ok := spec["resources"].(map[string]any); ok {
		st.podLevel = resourceMapToKeyValues(podResources)
	}

	containers, _ := spec["containers"].([]any)
	for _, c := range containers {
		cMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		row := buildPodResizeRow(cMap)
		st.containers = append(st.containers, row)
		if containerRequiresRestart(cMap) {
			st.restartWarn = append(st.restartWarn, row.name)
		}
	}
	return st
}

func buildPodResizeRow(cMap map[string]any) podResizeRow {
	row := podResizeRow{}
	row.name, _ = cMap["name"].(string)

	resources, _ := cMap["resources"].(map[string]any)
	requests, _ := resources["requests"].(map[string]any)
	limits, _ := resources["limits"].(map[string]any)
	cpuReq, _ := requests["cpu"].(string)
	cpuLim, _ := limits["cpu"].(string)
	memReq, _ := requests["memory"].(string)
	memLim, _ := limits["memory"].(string)

	row.cpuReq.Set(cpuReq)
	row.cpuLim.Set(cpuLim)
	row.memReq.Set(memReq)
	row.memLim.Set(memLim)
	row.orig = [4]string{cpuReq, cpuLim, memReq, memLim}
	return row
}

// containerRequiresRestart reports whether any of the container's
// resizePolicy entries need a container restart to take effect.
func containerRequiresRestart(cMap map[string]any) bool {
	policies, _ := cMap["resizePolicy"].([]any)
	for _, p := range policies {
		pMap, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if restartPolicy, _ := pMap["restartPolicy"].(string); restartPolicy == "RestartContainer" {
			return true
		}
	}
	return false
}

// resourceMapToKeyValues flattens a resources.requests/limits map (as read
// from spec.resources) into display rows, requests before limits.
func resourceMapToKeyValues(resources map[string]any) []model.KeyValue {
	var kvs []model.KeyValue
	if requests, ok := resources["requests"].(map[string]any); ok {
		if cpu, ok := requests["cpu"].(string); ok {
			kvs = append(kvs, model.KeyValue{Key: "cpu", Value: cpu})
		}
		if mem, ok := requests["memory"].(string); ok {
			kvs = append(kvs, model.KeyValue{Key: "memory", Value: mem})
		}
	}
	if limits, ok := resources["limits"].(map[string]any); ok {
		if cpu, ok := limits["cpu"].(string); ok {
			kvs = append(kvs, model.KeyValue{Key: "cpu limit", Value: cpu})
		}
		if mem, ok := limits["memory"].(string); ok {
			kvs = append(kvs, model.KeyValue{Key: "memory limit", Value: mem})
		}
	}
	return kvs
}

// parsePodResizeForm skips unchanged containers so the patch only touches
// what the user edited.
func parsePodResizeForm(st podResizeState) ([]model.ContainerResources, error) {
	var specs []model.ContainerResources
	for _, row := range st.containers {
		cur := [4]string{row.cpuReq.Value, row.cpuLim.Value, row.memReq.Value, row.memLim.Value}
		if cur == row.orig {
			continue
		}

		spec := model.ContainerResources{Name: row.name}
		var err error
		if spec.CPURequest, err = parseQuantityField(row.name, "cpu request", row.orig[0], cur[0]); err != nil {
			return nil, err
		}
		if spec.CPULimit, err = parseQuantityField(row.name, "cpu limit", row.orig[1], cur[1]); err != nil {
			return nil, err
		}
		if spec.MemRequest, err = parseQuantityField(row.name, "memory request", row.orig[2], cur[2]); err != nil {
			return nil, err
		}
		if spec.MemLimit, err = parseQuantityField(row.name, "memory limit", row.orig[3], cur[3]); err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// parseQuantityField returns value unchanged (the API wants the original
// string form). The resize subresource can't remove a request/limit that
// was already set, so clearing a previously non-empty field is rejected.
func parseQuantityField(containerName, field, orig, value string) (string, error) {
	if value == "" {
		if orig != "" {
			return "", fmt.Errorf("%s %s: cannot be cleared, set a value", containerName, field)
		}
		return "", nil
	}
	if _, err := resource.ParseQuantity(value); err != nil {
		return "", fmt.Errorf("%s %s: %w", containerName, field, err)
	}
	return value, nil
}
