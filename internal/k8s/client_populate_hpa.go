package k8s

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/model"
)

func populateHPADetails(ti *model.Item, status, spec map[string]any) {
	populateHPAReady(ti, status, spec)
	populateHPASpecColumns(ti, spec)
	populateHPAStatusColumns(ti, status)
}

func populateHPAReady(ti *model.Item, status, spec map[string]any) {
	if status == nil {
		return
	}
	var currentR, desiredR int64
	if cr, ok := status["currentReplicas"].(float64); ok {
		currentR = int64(cr)
	}
	if dr, ok := status["desiredReplicas"].(float64); ok {
		desiredR = int64(dr)
	}
	var minR, maxR int64
	if spec != nil {
		if mr, ok := spec["minReplicas"].(float64); ok {
			minR = int64(mr)
		}
		if mr, ok := spec["maxReplicas"].(float64); ok {
			maxR = int64(mr)
		}
	}
	ti.Ready = fmt.Sprintf("%d/%d (%d-%d)", currentR, desiredR, minR, maxR)
}

func populateHPASpecColumns(ti *model.Item, spec map[string]any) {
	if spec == nil {
		return
	}
	if scaleTargetRef, ok := spec["scaleTargetRef"].(map[string]any); ok {
		refKind, _ := scaleTargetRef["kind"].(string)
		refName, _ := scaleTargetRef["name"].(string)
		if refKind != "" && refName != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Target", Value: refKind + "/" + refName})
		}
	}
	if minR, ok := spec["minReplicas"].(float64); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Min Replicas", Value: fmt.Sprintf("%d", int64(minR))})
	}
	if maxR, ok := spec["maxReplicas"].(float64); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Max Replicas", Value: fmt.Sprintf("%d", int64(maxR))})
	}
	if metrics, ok := spec["metrics"].([]any); ok {
		populateHPASpecMetrics(ti, metrics)
	}
}

func populateHPASpecMetrics(ti *model.Item, metrics []any) {
	for _, m := range metrics {
		mMap, ok := m.(map[string]any)
		if !ok {
			continue
		}
		mType, _ := mMap["type"].(string)
		switch mType {
		case "Resource":
			populateHPAResourceMetric(ti, mMap, "Target")
		case "Pods":
			populateHPAPodsMetric(ti, mMap, "target", "Target")
		case "Object":
			populateHPAObjectMetric(ti, mMap)
		}
	}
}

func populateHPAResourceMetric(ti *model.Item, mMap map[string]any, prefix string) {
	res, ok := mMap["resource"].(map[string]any)
	if !ok {
		return
	}
	resName, _ := res["name"].(string)
	target, ok := res["target"].(map[string]any)
	if !ok {
		return
	}
	targetType, _ := target["type"].(string)
	switch targetType {
	case "Utilization":
		if avg, ok := target["averageUtilization"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{
				Key:   fmt.Sprintf("%s %s", prefix, strings.ToUpper(resName[:1])+resName[1:]),
				Value: fmt.Sprintf("%d%%", int64(avg)),
			})
		}
	case "AverageValue":
		if avg, ok := target["averageValue"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{
				Key:   fmt.Sprintf("%s %s", prefix, strings.ToUpper(resName[:1])+resName[1:]),
				Value: avg,
			})
		}
	}
}

func populateHPAPodsMetric(ti *model.Item, mMap map[string]any, dataKey, prefix string) {
	pods, ok := mMap["pods"].(map[string]any)
	if !ok {
		return
	}
	metricName := ""
	if mn, ok := pods["metric"].(map[string]any); ok {
		metricName, _ = mn["name"].(string)
	}
	data, ok := pods[dataKey].(map[string]any)
	if !ok {
		return
	}
	if avg, ok := data["averageValue"].(string); ok && metricName != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   fmt.Sprintf("%s %s", prefix, metricName),
			Value: avg,
		})
	}
}

func populateHPAObjectMetric(ti *model.Item, mMap map[string]any) {
	object, ok := mMap["object"].(map[string]any)
	if !ok {
		return
	}
	metricName := ""
	if mn, ok := object["metric"].(map[string]any); ok {
		metricName, _ = mn["name"].(string)
	}
	target, ok := object["target"].(map[string]any)
	if !ok {
		return
	}
	if val, ok := target["value"].(string); ok && metricName != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   fmt.Sprintf("Target %s", metricName),
			Value: val,
		})
	}
}

func populateHPAStatusColumns(ti *model.Item, status map[string]any) {
	if status == nil {
		return
	}
	if current, ok := status["currentReplicas"].(float64); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Current Replicas", Value: fmt.Sprintf("%d", int64(current))})
	}
	if desired, ok := status["desiredReplicas"].(float64); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Desired Replicas", Value: fmt.Sprintf("%d", int64(desired))})
	}
	if currentMetrics, ok := status["currentMetrics"].([]any); ok {
		populateHPACurrentMetrics(ti, currentMetrics)
	}
	if conditions, ok := status["conditions"].([]any); ok {
		populateHPAConditions(ti, conditions)
	}
}

func populateHPACurrentMetrics(ti *model.Item, currentMetrics []any) {
	for _, m := range currentMetrics {
		mMap, ok := m.(map[string]any)
		if !ok {
			continue
		}
		mType, _ := mMap["type"].(string)
		switch mType {
		case "Resource":
			populateHPACurrentResourceMetric(ti, mMap)
		case "Pods":
			populateHPACurrentPodsMetric(ti, mMap)
		}
	}
}

func populateHPACurrentResourceMetric(ti *model.Item, mMap map[string]any) {
	res, ok := mMap["resource"].(map[string]any)
	if !ok {
		return
	}
	resName, _ := res["name"].(string)
	current, ok := res["current"].(map[string]any)
	if !ok {
		return
	}
	if avg, ok := current["averageUtilization"].(float64); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   fmt.Sprintf("Current %s", strings.ToUpper(resName[:1])+resName[1:]),
			Value: fmt.Sprintf("%d%%", int64(avg)),
		})
	} else if avgVal, ok := current["averageValue"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   fmt.Sprintf("Current %s", strings.ToUpper(resName[:1])+resName[1:]),
			Value: avgVal,
		})
	}
}

func populateHPACurrentPodsMetric(ti *model.Item, mMap map[string]any) {
	pods, ok := mMap["pods"].(map[string]any)
	if !ok {
		return
	}
	metricName := ""
	if mn, ok := pods["metric"].(map[string]any); ok {
		metricName, _ = mn["name"].(string)
	}
	current, ok := pods["current"].(map[string]any)
	if !ok {
		return
	}
	if avg, ok := current["averageValue"].(string); ok && metricName != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   fmt.Sprintf("Current %s", metricName),
			Value: avg,
		})
	}
}

func populateHPAConditions(ti *model.Item, conditions []any) {
	for _, c := range conditions {
		cMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		cType, _ := cMap["type"].(string)
		cStatus, _ := cMap["status"].(string)
		if cType == "ScalingLimited" && cStatus == "True" {
			msg, _ := cMap["message"].(string)
			if msg != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Scaling Limited", Value: msg})
			}
		}
	}
}
