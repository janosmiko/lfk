package app

import (
	"fmt"
	"strings"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// blastRadiusState is what one confirm dialog knows about the cost of the
// action it is asking about. It is grouped rather than spread across Model
// fields because app.go sits on the file-length cap.
type blastRadiusState struct {
	radius  *k8s.BlastRadius
	loading bool
	// req numbers each fetch, so a reply for a dialog the user already
	// closed and reopened cannot land on the new one.
	req uint64

	// pods and pdbs are kept only for the scale overlay, where the answer
	// changes with every keystroke. Recomputing from these costs nothing;
	// refetching on each digit would not.
	pods []k8s.EvictedPod
	pdbs []policyv1.PodDisruptionBudget
}

func (s *blastRadiusState) reset() {
	s.radius = nil
	s.loading = false
	s.pods = nil
	s.pdbs = nil
}

// scaleBlastRadius answers what scaling to target costs, from the pods already
// fetched. Scaling up removes nothing, so it has no blast radius.
func (s *blastRadiusState) scaleBlastRadius(target int) *k8s.BlastRadius {
	if s.pods == nil {
		return nil
	}
	// Clamped both ways. A target above the pod count removes nothing, and a
	// negative one cannot remove more than every pod. Only the digit-only key
	// filter keeps a negative out today, and memory safety must not rest on
	// the caller.
	going := min(len(s.pods)-target, len(s.pods))
	if going <= 0 {
		return nil
	}
	readyBefore := 0
	for _, pod := range s.pods {
		if pod.Ready {
			readyBefore++
		}
	}
	radius := k8s.ComputeBlastRadius(s.pods[:going], s.pdbs, readyBefore)
	return &radius
}

// blastRadiusNotes turns the computed cost into the lines the confirm box
// shows. A violation is a second, warning-styled note rather than words inside
// the first one, so the color carries the meaning on its own.
//
// Drain and bulk have no single workload, so they carry no replica count and
// the phrase is left out rather than printed as "0 of 0".
func blastRadiusNotes(r *k8s.BlastRadius, loading bool) []ui.ConfirmNote {
	if loading {
		return []ui.ConfirmNote{{Text: "Blast radius: checking disruption budgets..."}}
	}
	if r == nil {
		return nil
	}
	if r.Evicting == 0 {
		return []ui.ConfirmNote{{Text: "Blast radius: no running pods to evict"}}
	}

	parts := []string{fmt.Sprintf("%d %s", r.Evicting, plural(r.Evicting, "pod", "pods"))}
	if r.ReadyBefore > 0 {
		parts = append(parts, fmt.Sprintf("%d of %d ready after", r.ReadyAfter, r.ReadyBefore))
	}
	parts = append(parts, budgetSummary(r.PDBs))
	if r.Uncounted > 0 {
		parts = append(parts, fmt.Sprintf("%d rows not counted", r.Uncounted))
	}

	notes := []ui.ConfirmNote{{Text: "Blast radius: " + strings.Join(parts, ", ")}}
	if violated := violatedBudgets(r.PDBs, len(r.PDBs) == 1); violated != "" {
		notes = append(notes, ui.ConfirmNote{Text: violated, Warn: true})
	}
	return notes
}

// budgetSummary names one budget in full and counts the rest. Naming every
// budget on a drain would not fit the box.
func budgetSummary(pdbs []k8s.PDBImpact) string {
	switch len(pdbs) {
	case 0:
		return "no disruption budget covers them"
	case 1:
		p := pdbs[0]
		return fmt.Sprintf("PDB %s %d -> %d allowed", p.Name, p.AllowedBefore, p.AllowedAfter)
	default:
		return fmt.Sprintf("%d disruption budgets affected", len(pdbs))
	}
}

// violatedBudgets states the shortfall for each budget the action would
// breach. named is false when the line above already names the only budget,
// because repeating a long namespace and name there wraps the box for nothing.
func violatedBudgets(pdbs []k8s.PDBImpact, sole bool) string {
	var out []string
	for _, p := range pdbs {
		if !p.Violated {
			continue
		}
		over := -p.AllowedAfter
		if sole {
			return fmt.Sprintf("Over budget by %d", over)
		}
		out = append(out, fmt.Sprintf("%s/%s by %d", p.Namespace, p.Name, over))
	}
	if len(out) == 0 {
		return ""
	}
	return "Over budget: " + strings.Join(out, "; ")
}

// bulkPodTargets splits a bulk selection into the pod names to look up, keyed
// by namespace, and a count of rows it cannot resolve.
//
// A list row carries no labels and no spec, so a workload row would need its
// own API call to learn which pods it claims. One call per selected row turns
// a fifty-row selection into fifty calls, so those rows are reported as
// uncounted instead. Pod rows cost one list per namespace, however many are
// selected.
func bulkPodTargets(items []model.Item) (map[string]map[string]bool, int) {
	byNS := make(map[string]map[string]bool)
	uncounted := 0
	for _, it := range items {
		if it.Kind != "Pod" {
			uncounted++
			continue
		}
		if byNS[it.Namespace] == nil {
			byNS[it.Namespace] = make(map[string]bool)
		}
		byNS[it.Namespace][it.Name] = true
	}
	return byNS, uncounted
}

// workloadSelectorFrom reads spec.selector off the object the action was
// raised on. The list row already carries the full object, so this saves a
// second GET just to learn which pods a workload claims. Anything that is not
// the expected shape yields nil, which the caller reads as "claims no pods"
// rather than guessing.
func workloadSelectorFrom(raw map[string]any) *metav1.LabelSelector {
	spec, ok := raw["spec"].(map[string]any)
	if !ok {
		return nil
	}
	sel, ok := spec["selector"].(map[string]any)
	if !ok {
		return nil
	}
	out := &metav1.LabelSelector{}
	if labels, ok := sel["matchLabels"].(map[string]any); ok {
		out.MatchLabels = make(map[string]string, len(labels))
		for k, v := range labels {
			if s, ok := v.(string); ok {
				out.MatchLabels[k] = s
			}
		}
	}
	for _, e := range asSlice(sel["matchExpressions"]) {
		expr, ok := e.(map[string]any)
		if !ok {
			continue
		}
		key, _ := expr["key"].(string)
		op, _ := expr["operator"].(string)
		if key == "" || op == "" {
			continue
		}
		req := metav1.LabelSelectorRequirement{
			Key:      key,
			Operator: metav1.LabelSelectorOperator(op),
		}
		for _, v := range asSlice(expr["values"]) {
			if s, ok := v.(string); ok {
				req.Values = append(req.Values, s)
			}
		}
		out.MatchExpressions = append(out.MatchExpressions, req)
	}
	return out
}

// evictedPodFromRaw reads a pod's own labels and readiness off the object the
// action was raised on, so deleting a single pod costs no extra API call.
func evictedPodFromRaw(raw map[string]any, namespace string) k8s.EvictedPod {
	pod := k8s.EvictedPod{Namespace: namespace}
	if meta, ok := raw["metadata"].(map[string]any); ok {
		if labels, ok := meta["labels"].(map[string]any); ok {
			pod.Labels = make(map[string]string, len(labels))
			for k, v := range labels {
				if s, ok := v.(string); ok {
					pod.Labels[k] = s
				}
			}
		}
	}
	status, ok := raw["status"].(map[string]any)
	if !ok {
		return pod
	}
	for _, c := range asSlice(status["conditions"]) {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if cond["type"] == "Ready" {
			pod.Ready = cond["status"] == "True"
			break
		}
	}
	return pod
}

// readyReplicasFrom reads status.readyReplicas. The value arrives as int64
// from the dynamic client and as float64 when it has been through JSON.
func readyReplicasFrom(raw map[string]any) int {
	status, ok := raw["status"].(map[string]any)
	if !ok {
		return 0
	}
	switch v := status["readyReplicas"].(type) {
	case int64:
		return int(v)
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
