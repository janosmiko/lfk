package k8s

import (
	"strings"

	"github.com/janosmiko/lfk/internal/model"
)

// populateCRDResource fills the summary columns for a kind with no dedicated
// handler. It keeps the Karpenter branch out of populateResourceDetailsExt,
// which sits at the gocyclo cap.
func populateCRDResource(ti *model.Item, obj map[string]any, kind string, status map[string]any) {
	populateGenericCRDResource(ti, status)
	apiVersion, _ := obj["apiVersion"].(string)
	if !strings.HasPrefix(apiVersion, "karpenter.") {
		return
	}
	conds := karpenterConditions(status)
	if len(conds) == 0 {
		return
	}
	switch kind {
	case "NodeClaim":
		populateKarpenterNodeClaim(ti, conds)
	case "NodePool", "EC2NodeClass", "AKSNodeClass":
		ti.Status = karpenterReadyStatus(conds)
		if kind == "NodePool" && conds["NodeRegistrationHealthy"].status == "False" {
			ti.Status = "RegistrationUnhealthy"
		}
	}
}

type karpenterCondition struct{ status, reason string }

func karpenterConditions(status map[string]any) map[string]karpenterCondition {
	list, _ := status["conditions"].([]any)
	out := make(map[string]karpenterCondition, len(list))
	for _, c := range list {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		t, _ := cond["type"].(string)
		s, _ := cond["status"].(string)
		r, _ := cond["reason"].(string)
		out[t] = karpenterCondition{status: s, reason: r}
	}
	return out
}

// karpenterReadyStatus exists because the generic extractor shows the last
// condition when Ready is False, an unrelated one like "ValidationSucceeded".
func karpenterReadyStatus(conds map[string]karpenterCondition) string {
	if conds["Ready"].status == "True" {
		return "Ready"
	}
	return "NotReady"
}

// populateKarpenterNodeClaim surfaces the lifecycle and disruption state that
// Karpenter keeps in NodeClaim conditions. Ready stays True on a drifted or
// disrupting node, so the generic extractor always showed "Ready".
func populateKarpenterNodeClaim(ti *model.Item, conds map[string]karpenterCondition) {
	if c := conds["Drifted"]; c.status == "True" && c.reason != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Drift Reason", Value: c.reason})
	}
	if c := conds["DisruptionReason"]; c.status == "True" && c.reason != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Disruption Reason", Value: c.reason})
	}
	ti.Status = karpenterNodeClaimStatus(conds)
}

func karpenterNodeClaimStatus(conds map[string]karpenterCondition) string {
	isTrue := func(t string) bool { return conds[t].status == "True" }
	switch {
	case isTrue("InstanceTerminating"):
		return "Terminating"
	case isTrue("DisruptionReason"):
		return "Disrupting"
	case isTrue("Rebooting"):
		return "Rebooting"
	case isTrue("Drifted"):
		return "Drifted"
	case isTrue("Ready"):
		return "Ready"
	}
	// Launched, Registered and Initialized gate Ready in this order. Unknown
	// means Karpenter is still working on that step, False means it failed.
	for _, step := range []struct{ cond, status string }{
		{"Launched", "Launching"}, {"Registered", "Registering"}, {"Initialized", "Initializing"},
	} {
		switch conds[step.cond].status {
		case "True":
			continue
		case "False":
			return "NotReady"
		}
		return step.status
	}
	return "NotReady"
}
