package k8s

import (
	"encoding/base64"
	"sort"
	"strings"

	"github.com/janosmiko/lfk/internal/model"
)

func populateNodeDetails(ti *model.Item, obj map[string]any, status, spec map[string]any) {
	populateNodeRoles(ti, obj)
	populateNodeStatus(ti, status)
	populateNodeTaints(ti, spec)
}

func populateNodeRoles(ti *model.Item, obj map[string]any) {
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return
	}
	labels, ok := metadata["labels"].(map[string]any)
	if !ok {
		return
	}
	var roles []string
	for k := range labels {
		if after, ok0 := strings.CutPrefix(k, "node-role.kubernetes.io/"); ok0 {
			role := after
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	if len(roles) > 0 {
		sort.Strings(roles)
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Role", Value: strings.Join(roles, ",")})
	}
}

func populateNodeStatus(ti *model.Item, status map[string]any) {
	if status == nil {
		return
	}
	if addrs, ok := status["addresses"].([]any); ok {
		for _, a := range addrs {
			if aMap, ok := a.(map[string]any); ok {
				addrType, _ := aMap["type"].(string)
				addr, _ := aMap["address"].(string)
				if addrType != "" && addr != "" {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: addrType, Value: addr})
				}
			}
		}
	}
	if alloc, ok := status["allocatable"].(map[string]any); ok {
		if cpu, ok := alloc["cpu"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "CPU Alloc", Value: cpu})
		}
		if mem, ok := alloc["memory"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Mem Alloc", Value: mem})
		}
	}
	if nodeInfo, ok := status["nodeInfo"].(map[string]any); ok {
		if v, ok := nodeInfo["kubeletVersion"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Version", Value: v})
		}
		if v, ok := nodeInfo["osImage"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "OS", Value: v})
		}
		if v, ok := nodeInfo["containerRuntimeVersion"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Runtime", Value: v})
		}
	}
}

func populateNodeTaints(ti *model.Item, spec map[string]any) {
	if spec == nil {
		return
	}
	taints, ok := spec["taints"].([]any)
	if !ok || len(taints) == 0 {
		return
	}
	var taintStrs []string
	for _, t := range taints {
		if tMap, ok := t.(map[string]any); ok {
			key, _ := tMap["key"].(string)
			value, _ := tMap["value"].(string)
			effect, _ := tMap["effect"].(string)
			taint := key
			if value != "" {
				taint += "=" + value
			}
			taint += ":" + effect
			taintStrs = append(taintStrs, taint)
		}
	}
	if len(taintStrs) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Taints", Value: strings.Join(taintStrs, ", ")})
	}
}

func populatePVCDetails(ti *model.Item, status, spec map[string]any) {
	if status != nil {
		if phase, ok := status["phase"].(string); ok {
			ti.Status = phase
		}
		if cap, ok := status["capacity"].(map[string]any); ok {
			if storage, ok := cap["storage"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Capacity", Value: storage})
			}
		}
	}
	if spec == nil {
		return
	}
	if res, ok := spec["resources"].(map[string]any); ok {
		if req, ok := res["requests"].(map[string]any); ok {
			if storage, ok := req["storage"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Request", Value: storage})
			}
		}
	}
	if vol, ok := spec["volumeName"].(string); ok && vol != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Volume", Value: vol})
	}
	if am, ok := spec["accessModes"].([]any); ok {
		var modes []string
		for _, m := range am {
			if s, ok := m.(string); ok {
				modes = append(modes, s)
			}
		}
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Access Modes", Value: strings.Join(modes, ", ")})
	}
	if sc, ok := spec["storageClassName"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Storage Class", Value: sc})
	}
	if vm, ok := spec["volumeMode"].(string); ok && vm != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Volume Mode", Value: vm})
	}
}

func populateConfigMapDetails(ti *model.Item, obj map[string]any) {
	data, ok := obj["data"].(map[string]any)
	if !ok {
		return
	}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if v, ok := data[k].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "data:" + k, Value: v})
		}
	}
}

func populateSecretDetails(ti *model.Item, obj map[string]any) {
	if data, ok := obj["data"].(map[string]any); ok {
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if encoded, ok := data[k].(string); ok {
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				if err == nil {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "secret:" + k, Value: string(decoded)})
				}
			}
		}
	}
	if sType, ok := obj["type"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Type", Value: sType})
	}
}
