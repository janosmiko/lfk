package k8s

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

func populatePodDetails(ti *model.Item, obj map[string]any, status, spec map[string]any) {
	if status == nil {
		return
	}
	containerStatuses, _ := status["containerStatuses"].([]any)
	totalContainers := len(containerStatuses)
	if containers, ok := spec["containers"].([]any); ok {
		totalContainers = len(containers)
	}
	readyCount := 0
	restartCount := int64(0)
	for _, cs := range containerStatuses {
		csMap, ok := cs.(map[string]any)
		if !ok {
			continue
		}
		if ready, ok := csMap["ready"].(bool); ok && ready {
			readyCount++
		}
		if rc, ok := csMap["restartCount"].(int64); ok {
			restartCount += rc
		} else if rcf, ok := csMap["restartCount"].(float64); ok {
			restartCount += int64(rcf)
		}
	}
	ti.Ready = fmt.Sprintf("%d/%d", readyCount, totalContainers)
	ti.Restarts = fmt.Sprintf("%d", restartCount)

	ti.LastRestartAt = findLastRestartTime(containerStatuses)

	if ti.Status != "Succeeded" && readyCount < totalContainers && totalContainers > 0 {
		overridePodStatus(ti, status, containerStatuses)
	}

	if containers, ok := spec["containers"].([]any); ok {
		cpuReq, cpuLim, memReq, memLim := extractContainerResources(containers)
		addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
	}

	populatePodExtraColumns(ti, obj, status, spec)
}

func findLastRestartTime(containerStatuses []any) time.Time {
	var lastRestart time.Time
	for _, cs := range containerStatuses {
		csMap, ok := cs.(map[string]any)
		if !ok {
			continue
		}
		lastState, _ := csMap["lastState"].(map[string]any)
		if lastState == nil {
			continue
		}
		if terminated, ok := lastState["terminated"].(map[string]any); ok {
			if finishedAt, ok := terminated["finishedAt"].(string); ok {
				if t, err := time.Parse(time.RFC3339, finishedAt); err == nil {
					if t.After(lastRestart) {
						lastRestart = t
					}
				}
			}
		}
	}
	return lastRestart
}

func overridePodStatus(ti *model.Item, status map[string]any, containerStatuses []any) {
	initContainerStatuses, _ := status["initContainerStatuses"].([]any)
	reason := extractContainerNotReadyReason(initContainerStatuses)
	if reason == "" || reason == "PodInitializing" {
		reason = extractContainerNotReadyReason(containerStatuses)
	}
	if reason == "PodInitializing" && ti.Status == "Failed" {
		reason = ""
	}
	if reason != "" {
		ti.Status = reason
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: reason})
	} else if ti.Status == "Running" {
		ti.Status = "NotReady"
	}
}

func populatePodExtraColumns(ti *model.Item, _ map[string]any, status, spec map[string]any) {
	if qos, ok := status["qosClass"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "QoS", Value: qos})
	}
	if sa, ok := spec["serviceAccountName"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Service Account", Value: sa})
	}
	if podIP, ok := status["podIP"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Pod IP", Value: podIP})
	}
	if containers, ok := spec["containers"].([]any); ok {
		var images []string
		for _, c := range containers {
			if cMap, ok := c.(map[string]any); ok {
				if img, ok := cMap["image"].(string); ok {
					images = append(images, img)
				}
			}
		}
		if len(images) > 0 {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Images", Value: strings.Join(images, ", ")})
		}
	}
	if pc, ok := spec["priorityClassName"].(string); ok && pc != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Priority Class", Value: pc})
	}
	if nodeName, ok := spec["nodeName"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Node", Value: nodeName})
	}
}

func populateDeploymentDetails(ti *model.Item, obj, status, spec map[string]any) {
	if status == nil || spec == nil {
		return
	}
	var specReplicas int64 = 1
	if r, ok := spec["replicas"].(int64); ok {
		specReplicas = r
	} else if r, ok := spec["replicas"].(float64); ok {
		specReplicas = int64(r)
	}
	var readyReplicas int64
	if r, ok := status["readyReplicas"].(int64); ok {
		readyReplicas = r
	} else if r, ok := status["readyReplicas"].(float64); ok {
		readyReplicas = int64(r)
	}
	ti.Ready = fmt.Sprintf("%d/%d", readyReplicas, specReplicas)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Replicas", Value: fmt.Sprintf("%d", specReplicas)})
	if rev := resourceVersionDecimal(obj); rev != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "REV", Value: rev})
	}
	if strategy, ok := spec["strategy"].(map[string]any); ok {
		if t, ok := strategy["type"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Strategy", Value: t})
		}
	}
	if updated, ok := intFromMap(status, "updatedReplicas"); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Up-to-date", Value: fmt.Sprintf("%d", updated)})
	}
	if avail, ok := intFromMap(status, "availableReplicas"); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Available", Value: fmt.Sprintf("%d", avail)})
	}
	if unavail, ok := intFromMap(status, "unavailableReplicas"); ok && unavail > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Unavailable", Value: fmt.Sprintf("%d", unavail)})
	}
	cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)
	addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
	populateContainerImages(ti, spec)
}

func populateStatefulSetDetails(ti *model.Item, obj, status, spec map[string]any) {
	if status == nil || spec == nil {
		return
	}
	var specReplicas int64 = 1
	if r, ok := spec["replicas"].(int64); ok {
		specReplicas = r
	} else if r, ok := spec["replicas"].(float64); ok {
		specReplicas = int64(r)
	}
	var readyReplicas int64
	if r, ok := status["readyReplicas"].(int64); ok {
		readyReplicas = r
	} else if r, ok := status["readyReplicas"].(float64); ok {
		readyReplicas = int64(r)
	}
	ti.Ready = fmt.Sprintf("%d/%d", readyReplicas, specReplicas)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Replicas", Value: fmt.Sprintf("%d", specReplicas)})
	if rev := resourceVersionDecimal(obj); rev != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "REV", Value: rev})
	}
	if updated, ok := intFromMap(status, "updatedReplicas"); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Up-to-date", Value: fmt.Sprintf("%d", updated)})
	}
	if us, ok := spec["updateStrategy"].(map[string]any); ok {
		if t, ok := us["type"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Update Strategy", Value: t})
		}
	}
	if cr, ok := status["currentRevision"].(string); ok && cr != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Current Revision", Value: cr})
	}
	if ur, ok := status["updateRevision"].(string); ok && ur != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Update Revision", Value: ur})
	}
	cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)
	addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
	populateContainerImages(ti, spec)
}

func populateDaemonSetDetails(ti *model.Item, obj, status, spec map[string]any) {
	if status == nil {
		return
	}
	var desired, ready int64
	if d, ok := status["desiredNumberScheduled"].(int64); ok {
		desired = d
	} else if d, ok := status["desiredNumberScheduled"].(float64); ok {
		desired = int64(d)
	}
	if r, ok := status["numberReady"].(int64); ok {
		ready = r
	} else if r, ok := status["numberReady"].(float64); ok {
		ready = int64(r)
	}
	ti.Ready = fmt.Sprintf("%d/%d", ready, desired)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Desired", Value: fmt.Sprintf("%d", desired)})
	if rev := resourceVersionDecimal(obj); rev != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "REV", Value: rev})
	}
	if current, ok := intFromMap(status, "currentNumberScheduled"); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Current", Value: fmt.Sprintf("%d", current)})
	}
	if updated, ok := intFromMap(status, "updatedNumberScheduled"); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Up-to-date", Value: fmt.Sprintf("%d", updated)})
	}
	if avail, ok := intFromMap(status, "numberAvailable"); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Available", Value: fmt.Sprintf("%d", avail)})
	}
	if miss, ok := intFromMap(status, "numberMisscheduled"); ok && miss > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Misscheduled", Value: fmt.Sprintf("%d", miss)})
	}
	if spec != nil {
		cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)
		addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
		populateContainerImages(ti, spec)
	}
}

func populateReplicaSetDetails(ti *model.Item, obj, status, spec map[string]any) {
	if status == nil || spec == nil {
		return
	}
	var specReplicas int64
	if r, ok := spec["replicas"].(int64); ok {
		specReplicas = r
	} else if r, ok := spec["replicas"].(float64); ok {
		specReplicas = int64(r)
	}
	var readyReplicas int64
	if r, ok := status["readyReplicas"].(int64); ok {
		readyReplicas = r
	} else if r, ok := status["readyReplicas"].(float64); ok {
		readyReplicas = int64(r)
	}
	ti.Ready = fmt.Sprintf("%d/%d", readyReplicas, specReplicas)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Desired", Value: fmt.Sprintf("%d", specReplicas)})
	if rev := resourceVersionDecimal(obj); rev != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "REV", Value: rev})
	}
	cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)
	addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
	populateContainerImages(ti, spec)
}

func populateCronJobDetails(ti *model.Item, status, spec map[string]any) {
	var (
		schedule string
		timeZone string
		suspend  bool
	)
	if spec != nil {
		if sched, ok := spec["schedule"].(string); ok {
			schedule = sched
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Schedule", Value: sched})
		}
		if tz, ok := spec["timeZone"].(string); ok {
			timeZone = tz
		}
		if s, ok := spec["suspend"].(bool); ok {
			suspend = s
		}
	}
	if status != nil {
		if lastSchedule, ok := status["lastScheduleTime"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Last Schedule", Value: lastSchedule})
		}
	}
	if !suspend && schedule != "" {
		if next, ok := nextCronFire(schedule, timeZone, time.Now()); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Next", Value: formatAge(time.Until(next))})
		}
	}
	if spec != nil {
		if _, ok := spec["suspend"].(bool); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Suspend", Value: fmt.Sprintf("%v", suspend)})
		}
	}
}

// resourceVersionDecimal returns metadata.resourceVersion as a decimal string,
// or "" when the field is missing or not parseable as a non-negative integer.
// The Kubernetes resourceVersion is opaque to clients; we validate it as a
// uint64 so the comparator can sort numerically.
func resourceVersionDecimal(obj map[string]any) string {
	if obj == nil {
		return ""
	}
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return ""
	}
	rv, ok := metadata["resourceVersion"].(string)
	if !ok || rv == "" {
		return ""
	}
	if _, err := strconv.ParseUint(rv, 10, 64); err != nil {
		return ""
	}
	return rv
}

// intFromMap reads key from m as int64, accepting both int64 and float64 wire types.
func intFromMap(m map[string]any, key string) (int64, bool) {
	if v, ok := m[key].(int64); ok {
		return v, true
	}
	if v, ok := m[key].(float64); ok {
		return int64(v), true
	}
	return 0, false
}

func populateJobDetails(ti *model.Item, status, spec map[string]any) {
	if spec != nil {
		if completions, ok := spec["completions"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Completions", Value: fmt.Sprintf("%d", int64(completions))})
		}
	}
	if status != nil {
		if succeeded, ok := status["succeeded"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Succeeded", Value: fmt.Sprintf("%d", int64(succeeded))})
		}
		if failed, ok := status["failed"].(float64); ok && failed > 0 {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Failed", Value: fmt.Sprintf("%d", int64(failed))})
		}
	}
	if spec != nil {
		if suspend, ok := spec["suspend"].(bool); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Suspend", Value: fmt.Sprintf("%v", suspend)})
		}
	}
}
