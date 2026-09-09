package k8s

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/model"
)

// populateResourceClaim adds the allocated device summary and the Ready
// condition to a ResourceClaim's list columns. An unallocated claim has no
// status.allocation, so the allocation columns are left blank.
func populateResourceClaim(ti *model.Item, status map[string]any) {
	if status == nil {
		return
	}
	if allocation, ok := status["allocation"].(map[string]any); ok {
		populateResourceClaimAllocation(ti, allocation)
	}
	if conditions, ok := status["conditions"].([]any); ok {
		extractGenericConditions(ti, conditions)
	}
}

func populateResourceClaimAllocation(ti *model.Item, allocation map[string]any) {
	if devices, ok := allocation["devices"].(map[string]any); ok {
		if results, ok := devices["results"].([]any); ok {
			var drivers, pools, deviceNames []string
			for _, r := range results {
				result, ok := r.(map[string]any)
				if !ok {
					continue
				}
				if v, ok := result["driver"].(string); ok {
					drivers = append(drivers, v)
				}
				if v, ok := result["pool"].(string); ok {
					pools = append(pools, v)
				}
				if v, ok := result["device"].(string); ok {
					deviceNames = append(deviceNames, v)
				}
			}
			if len(drivers) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Driver", Value: strings.Join(drivers, ", ")})
			}
			if len(pools) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Pool", Value: strings.Join(pools, ", ")})
			}
			if len(deviceNames) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Device", Value: strings.Join(deviceNames, ", ")})
			}
		}
	}
	if node := resourceClaimAllocationNode(allocation); node != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Node", Value: node})
	}
}

// resourceClaimAllocationNode extracts the node name a claim was allocated
// to from status.allocation.nodeSelector, which the API server expresses as
// a matchFields term on "metadata.name" rather than a dedicated field.
func resourceClaimAllocationNode(allocation map[string]any) string {
	nodeSelector, ok := allocation["nodeSelector"].(map[string]any)
	if !ok {
		return ""
	}
	terms, ok := nodeSelector["nodeSelectorTerms"].([]any)
	if !ok {
		return ""
	}
	var names []string
	for _, t := range terms {
		term, ok := t.(map[string]any)
		if !ok {
			continue
		}
		matchFields, ok := term["matchFields"].([]any)
		if !ok {
			continue
		}
		for _, f := range matchFields {
			field, ok := f.(map[string]any)
			if !ok {
				continue
			}
			if key, _ := field["key"].(string); key != "metadata.name" {
				continue
			}
			names = append(names, stringsFromAny(field["values"])...)
		}
	}
	return strings.Join(names, ", ")
}

// populateResourceClaimTemplate adds the number of device requests a
// generated ResourceClaim will carry to a ResourceClaimTemplate's list
// columns.
func populateResourceClaimTemplate(ti *model.Item, spec map[string]any) {
	if spec == nil {
		return
	}
	claimSpec, _ := spec["spec"].(map[string]any)
	devices, _ := claimSpec["devices"].(map[string]any)
	requests, _ := devices["requests"].([]any)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Devices", Value: fmt.Sprintf("%d", len(requests))})
}

// populateResourceSlice adds the publishing driver, the node or pool the
// slice belongs to, and its device count to a ResourceSlice's list columns.
func populateResourceSlice(ti *model.Item, spec map[string]any) {
	if spec == nil {
		return
	}
	if driver, ok := spec["driver"].(string); ok && driver != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Driver", Value: driver})
	}
	nodeOrPool := ""
	if nodeName, ok := spec["nodeName"].(string); ok && nodeName != "" {
		nodeOrPool = nodeName
	} else if pool, ok := spec["pool"].(map[string]any); ok {
		if name, ok := pool["name"].(string); ok {
			nodeOrPool = name
		}
	}
	if nodeOrPool != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Node or Pool", Value: nodeOrPool})
	}
	devices, _ := spec["devices"].([]any)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Devices", Value: fmt.Sprintf("%d", len(devices))})
}

// populateDeviceClass adds the number of device selectors to a DeviceClass's
// list columns.
func populateDeviceClass(ti *model.Item, spec map[string]any) {
	if spec == nil {
		return
	}
	selectors, _ := spec["selectors"].([]any)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Selectors", Value: fmt.Sprintf("%d", len(selectors))})
}
