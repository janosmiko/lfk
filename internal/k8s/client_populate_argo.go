package k8s

import (
	"fmt"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

func populateArgoCDApplication(ti *model.Item, _ map[string]any, status, spec map[string]any, kind string) {
	if status != nil {
		populateArgoCDHealthAndSync(ti, status)
		populateArgoCDOperationState(ti, status)
	}
	populateArgoCDConditions(ti, status)
	populateArgoCDSpec(ti, spec, kind)
}

func populateArgoCDHealthAndSync(ti *model.Item, status map[string]any) {
	if health, ok := status["health"].(map[string]any); ok {
		if msg, ok := health["message"].(string); ok && msg != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Health Message", Value: msg})
		}
	}
	if sync, ok := status["sync"].(map[string]any); ok {
		if rev, ok := sync["revision"].(string); ok && rev != "" {
			if len(rev) > 8 {
				rev = rev[:8]
			}
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Revision", Value: rev})
		}
	}
}

func populateArgoCDOperationState(ti *model.Item, status map[string]any) {
	opState, ok := status["operationState"].(map[string]any)
	if !ok {
		return
	}
	if phase, ok := opState["phase"].(string); ok && phase != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Last Sync", Value: phase})
	}
	if finishedAt, ok := opState["finishedAt"].(string); ok && finishedAt != "" {
		if t, err := time.Parse(time.RFC3339, finishedAt); err == nil {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Synced At", Value: formatRelativeTime(t)})
		}
	} else if startedAt, ok := opState["startedAt"].(string); ok && startedAt != "" {
		if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Synced At", Value: "syncing " + formatRelativeTime(t)})
		}
	}
	if msg, ok := opState["message"].(string); ok && msg != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Sync Message", Value: msg})
	}
	populateArgoCDSyncErrors(ti, opState)
}

func populateArgoCDSyncErrors(ti *model.Item, opState map[string]any) {
	syncResult, ok := opState["syncResult"].(map[string]any)
	if !ok {
		return
	}
	resources, ok := syncResult["resources"].([]any)
	if !ok {
		return
	}
	var errs []string
	for _, r := range resources {
		rMap, ok := r.(map[string]any)
		if !ok {
			continue
		}
		rStatus, _ := rMap["status"].(string)
		if rStatus != "Synced" && rStatus != "" {
			kind, _ := rMap["kind"].(string)
			name, _ := rMap["name"].(string)
			msg, _ := rMap["message"].(string)
			if msg != "" {
				errs = append(errs, fmt.Sprintf("%s/%s: %s", kind, name, msg))
			}
		}
	}
	if len(errs) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Sync Errors", Value: strings.Join(errs, "; ")})
	}
}

func populateArgoCDConditions(ti *model.Item, status map[string]any) {
	conditions, ok := status["conditions"].([]any)
	if !ok {
		return
	}
	var condTypes []string
	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		condMsg, _ := cond["message"].(string)
		if condType == "" {
			continue
		}
		if condType == "SyncError" {
			continue
		}
		condTypes = append(condTypes, condType)
		value := condMsg
		if value == "" {
			value = "(no message)"
		}
		if lastTransition, ok := cond["lastTransitionTime"].(string); ok && lastTransition != "" {
			if t, err := time.Parse(time.RFC3339, lastTransition); err == nil {
				value += " (" + formatRelativeTime(t) + ")"
			}
		}
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "condition:" + condType, Value: value})
	}
	if len(condTypes) > 0 {
		val := strings.Join(condTypes, ", ")
		if len(val) > 15 {
			val = val[:14] + "~"
		}
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Condition", Value: val})
	}
}

func populateArgoCDSpec(ti *model.Item, spec map[string]any, kind string) {
	if spec == nil {
		return
	}
	if kind == "Application" {
		populateArgoCDAutoSync(ti, spec)
	}
	if dest, ok := spec["destination"].(map[string]any); ok {
		if ns, ok := dest["namespace"].(string); ok && ns != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Dest NS", Value: ns})
		}
		if server, ok := dest["server"].(string); ok && server != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Dest Server", Value: server})
		}
	}
	if source, ok := spec["source"].(map[string]any); ok {
		if repo, ok := source["repoURL"].(string); ok && repo != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Repo", Value: repo})
		}
		if path, ok := source["path"].(string); ok && path != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Path", Value: path})
		}
	}
}

func populateArgoCDAutoSync(ti *model.Item, spec map[string]any) {
	autoSyncVal := "Off"
	if syncPolicy, ok := spec["syncPolicy"].(map[string]any); ok {
		if automated, ok := syncPolicy["automated"].(map[string]any); ok && automated != nil {
			autoSyncVal = "On"
			if sh, ok := automated["selfHeal"].(bool); ok && sh {
				autoSyncVal += "/SH"
			}
			if pr, ok := automated["prune"].(bool); ok && pr {
				autoSyncVal += "/P"
			}
		}
	}
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "AutoSync", Value: autoSyncVal})
}

func populateArgoWorkflow(ti *model.Item, status map[string]any) {
	if status == nil {
		return
	}
	if progress, ok := status["progress"].(string); ok && progress != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Progress", Value: progress})
	}
	populateArgoWorkflowDuration(ti, status)
	if msg, ok := status["message"].(string); ok && msg != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Message", Value: msg})
	}
	populateArgoWorkflowConditions(ti, status)
	populateArgoWorkflowSteps(ti, status)
}

func populateArgoWorkflowDuration(ti *model.Item, status map[string]any) {
	startedStr, _ := status["startedAt"].(string)
	finishedStr, _ := status["finishedAt"].(string)
	if startedStr == "" {
		return
	}
	started, err := time.Parse(time.RFC3339, startedStr)
	if err != nil {
		return
	}
	end := time.Now()
	if finishedStr != "" {
		if finished, err := time.Parse(time.RFC3339, finishedStr); err == nil {
			end = finished
		}
	}
	dur := end.Sub(started).Truncate(time.Second)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Duration", Value: dur.String()})
}

func populateArgoWorkflowConditions(ti *model.Item, status map[string]any) {
	conditions, ok := status["conditions"].([]any)
	if !ok {
		return
	}
	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		condStatus, _ := cond["status"].(string)
		condMessage, _ := cond["message"].(string)
		if condType != "" {
			ti.Conditions = append(ti.Conditions, model.ConditionEntry{
				Type:    condType,
				Status:  condStatus,
				Message: condMessage,
			})
		}
	}
}

func populateArgoWorkflowSteps(ti *model.Item, status map[string]any) {
	nodes, ok := status["nodes"].(map[string]any)
	if !ok {
		return
	}
	type nodeInfo struct {
		id, displayName, phase, message string
		children                        []string
	}
	nodeMap := make(map[string]nodeInfo, len(nodes))
	var rootID string
	for id, n := range nodes {
		node, ok := n.(map[string]any)
		if !ok {
			continue
		}
		info := nodeInfo{id: id}
		info.displayName, _ = node["displayName"].(string)
		if info.displayName == "" {
			info.displayName, _ = node["name"].(string)
		}
		info.phase, _ = node["phase"].(string)
		info.message, _ = node["message"].(string)
		if kids, ok := node["children"].([]any); ok {
			for _, k := range kids {
				if s, ok := k.(string); ok {
					info.children = append(info.children, s)
				}
			}
		}
		nodeMap[id] = info
		nodeName, _ := node["name"].(string)
		if nodeName == ti.Name {
			rootID = id
		}
	}

	var ordered []nodeInfo
	seen := make(map[string]bool)
	queue := []string{rootID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if seen[cur] || cur == "" {
			continue
		}
		seen[cur] = true
		info, ok := nodeMap[cur]
		if !ok {
			continue
		}
		if cur != rootID {
			ordered = append(ordered, info)
		}
		queue = append(queue, info.children...)
	}

	for _, s := range ordered {
		val := s.phase
		if s.message != "" {
			val += ": " + s.message
		}
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "step:" + s.displayName, Value: val})
	}
}
