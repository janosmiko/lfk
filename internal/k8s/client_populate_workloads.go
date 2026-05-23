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
	if g := progressingGeneration(obj, status); g != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Progressing", Value: g})
	}
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
	if g := progressingGeneration(obj, status); g != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Progressing", Value: g})
	}
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
	if g := progressingGeneration(obj, status); g != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Progressing", Value: g})
	}
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
	if g := progressingGeneration(obj, status); g != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Progressing", Value: g})
	}
	if rev := resourceVersionDecimal(obj); rev != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "REV", Value: rev})
	}
	cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)
	addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
	populateContainerImages(ti, spec)
}

func populateCronJobDetails(ti *model.Item, obj, status, spec map[string]any) {
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
		if active, ok := status["active"].([]any); ok && len(active) > 0 {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Active", Value: fmt.Sprintf("%d", len(active))})
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
	if rev := resourceVersionDecimal(obj); rev != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "REV", Value: rev})
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

// progressingGeneration returns the desired generation as a decimal string
// when metadata.generation > status.observedGeneration (indicating an
// in-progress or stalled rollout). Returns "" when in sync, missing, or
// not parseable — so the caller emits the column only when meaningful.
func progressingGeneration(obj, status map[string]any) string {
	if obj == nil || status == nil {
		return ""
	}
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return ""
	}
	gen, ok := intFromMap(metadata, "generation")
	if !ok {
		return ""
	}
	observed, ok := intFromMap(status, "observedGeneration")
	if !ok {
		// No observedGeneration at all = controller hasn't reconciled.
		return strconv.FormatInt(gen, 10)
	}
	if gen > observed {
		return strconv.FormatInt(gen, 10)
	}
	return ""
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

func jobDuration(status map[string]any) string {
	startStr, _ := status["startTime"].(string)
	if startStr == "" {
		return ""
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return ""
	}
	var end time.Time
	if compStr, ok := status["completionTime"].(string); ok && compStr != "" {
		if t, err := time.Parse(time.RFC3339, compStr); err == nil {
			end = t
		}
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}
	d := end.Sub(start).Round(time.Second)
	if d < 0 {
		return ""
	}
	return d.String()
}

func jobStatus(status, spec map[string]any) string {
	conds, _ := status["conditions"].([]any)
	for _, c := range conds {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		ctype, _ := cm["type"].(string)
		cstat, _ := cm["status"].(string)
		if cstat != "True" {
			continue
		}
		switch ctype {
		case "Complete", "SuccessCriteriaMet":
			return "Complete"
		case "Failed", "FailureTarget":
			return "Failed"
		}
	}
	if susp, ok := spec["suspend"].(bool); ok && susp {
		return "Suspended"
	}
	if active, ok := intFromMap(status, "active"); ok && active > 0 {
		return "Running"
	}
	return ""
}

func populateJobDetails(ti *model.Item, obj, status, spec map[string]any) {
	var specCompletions int64
	if spec != nil {
		if c, ok := intFromMap(spec, "completions"); ok {
			specCompletions = c
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Completions", Value: fmt.Sprintf("%d", c)})
		}
	}
	var succeeded int64
	if status != nil {
		if s, ok := intFromMap(status, "succeeded"); ok {
			succeeded = s
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Succeeded", Value: fmt.Sprintf("%d", s)})
		}
		if f, ok := intFromMap(status, "failed"); ok && f > 0 {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Failed", Value: fmt.Sprintf("%d", f)})
		}
		if a, ok := intFromMap(status, "active"); ok && a > 0 {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Active", Value: fmt.Sprintf("%d", a)})
		}
		if dur := jobDuration(status); dur != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Duration", Value: dur})
		}
	}
	if specCompletions > 0 {
		ti.Ready = fmt.Sprintf("%d/%d", succeeded, specCompletions)
	}
	if spec != nil {
		if suspend, ok := spec["suspend"].(bool); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Suspend", Value: fmt.Sprintf("%v", suspend)})
		}
	}
	if status != nil && spec != nil {
		ti.Status = jobStatus(status, spec)
	}
	if rev := resourceVersionDecimal(obj); rev != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "REV", Value: rev})
	}
}
