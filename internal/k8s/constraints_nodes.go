package k8s

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// nodeConstraintRows lists every node once and reports the scheduling
// terms no node satisfies (nodeSelector keys, required node affinity) and
// the taints no toleration covers.
func (c *Client) nodeConstraintRows(ctx context.Context, kubeCtx string, target ConstraintTarget) ([]ConstraintRow, error) {
	cs, err := c.clientsetForContext(kubeCtx)
	if err != nil {
		return nil, err
	}
	list, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}
	nodes := list.Items

	return slices.Concat(
		schedulingRows(nodes, target),
		taintRows(nodes, target.Tolerations),
	), nil
}

// schedulingRows suppresses a row only when one node satisfies the
// nodeSelector and the required affinity at once: node A matching the
// selector and node B the affinity still leaves the target unschedulable.
func schedulingRows(nodes []corev1.Node, target ConstraintTarget) []ConstraintRow {
	if len(target.NodeSelector) == 0 && len(target.Affinity) == 0 {
		return nil
	}
	selector := labels.SelectorFromSet(labels.Set(target.NodeSelector))
	for _, n := range nodes {
		if selector.Matches(labels.Set(n.Labels)) && nodeMatchesAffinity(n, target.Affinity) {
			return nil
		}
	}

	rows := slices.Concat(
		nodeSelectorRows(nodes, target.NodeSelector),
		nodeAffinityRows(nodes, target.Affinity),
	)
	if len(rows) > 0 {
		return rows
	}
	return []ConstraintRow{{
		Source:   "Node",
		Detail:   "nodeSelector and nodeAffinity: no single node satisfies both",
		Headroom: "0 nodes",
		Blocking: true,
	}}
}

func nodeMatchesAffinity(n corev1.Node, terms []NodeSelectorTerm) bool {
	if len(terms) == 0 {
		return true
	}
	for _, term := range terms {
		if nodeSelectorTermMatches(term, n) {
			return true
		}
	}
	return false
}

func nodeSelectorRows(nodes []corev1.Node, sel map[string]string) []ConstraintRow {
	var rows []ConstraintRow
	for k, v := range sel {
		if anyNodeHasLabel(nodes, k, v) {
			continue
		}
		rows = append(rows, ConstraintRow{
			Source:   "Node",
			Detail:   fmt.Sprintf("nodeSelector %s=%s: no node satisfies it", k, v),
			Headroom: "0 nodes",
			Blocking: true,
		})
	}
	return rows
}

func anyNodeHasLabel(nodes []corev1.Node, key, value string) bool {
	// SelectorFromSet requires the key to be present, so a node missing it
	// no longer matches an empty selector value.
	sel := labels.SelectorFromSet(labels.Set{key: value})
	for _, n := range nodes {
		if sel.Matches(labels.Set(n.Labels)) {
			return true
		}
	}
	return false
}

func nodeAffinityRows(nodes []corev1.Node, terms []NodeSelectorTerm) []ConstraintRow {
	if len(terms) == 0 {
		return nil
	}
	for _, n := range nodes {
		if nodeMatchesAffinity(n, terms) {
			return nil
		}
	}
	rows := make([]ConstraintRow, 0, len(terms))
	for _, term := range terms {
		rows = append(rows, ConstraintRow{
			Source:   "Node",
			Detail:   "nodeAffinity " + describeNodeSelectorTerm(term) + ": no node satisfies it",
			Headroom: "0 nodes",
			Blocking: true,
		})
	}
	return rows
}

func nodeSelectorTermMatches(term NodeSelectorTerm, n corev1.Node) bool {
	// Kubernetes treats a term with neither expressions nor fields as
	// matching no node at all.
	if len(term.MatchExpressions) == 0 && len(term.MatchFields) == 0 {
		return false
	}
	for _, req := range term.MatchExpressions {
		if !nodeSelectorRequirementMatches(req, n.Labels) {
			return false
		}
	}
	if len(term.MatchFields) > 0 {
		fields := nodeFieldSet(n)
		for _, req := range term.MatchFields {
			if !nodeSelectorRequirementMatches(req, fields) {
				return false
			}
		}
	}
	return true
}

// nodeFieldSet is what matchFields selects on. Kubernetes accepts only
// metadata.name there for a node.
func nodeFieldSet(n corev1.Node) map[string]string {
	return map[string]string{"metadata.name": n.Name}
}

func nodeSelectorRequirementMatches(req NodeSelectorRequirement, labels map[string]string) bool {
	val, exists := labels[req.Key]
	switch req.Operator {
	case "In":
		return exists && slices.Contains(req.Values, val)
	case "NotIn":
		return !exists || !slices.Contains(req.Values, val)
	case "Exists":
		return exists
	case "DoesNotExist":
		return !exists
	case "Gt":
		return exists && len(req.Values) == 1 && numericLess(req.Values[0], val)
	case "Lt":
		return exists && len(req.Values) == 1 && numericLess(val, req.Values[0])
	default:
		return false
	}
}

func numericLess(a, b string) bool {
	av, aerr := strconv.ParseInt(a, 10, 64)
	bv, berr := strconv.ParseInt(b, 10, 64)
	return aerr == nil && berr == nil && av < bv
}

func describeNodeSelectorTerm(term NodeSelectorTerm) string {
	var out strings.Builder
	for _, req := range slices.Concat(term.MatchExpressions, term.MatchFields) {
		if out.Len() > 0 {
			out.WriteString(", ")
		}
		fmt.Fprintf(&out, "%s %s %v", req.Key, req.Operator, req.Values)
	}
	return out.String()
}

// taintRows reports, per node, the NoSchedule/NoExecute taints none of the
// target's tolerations cover. PreferNoSchedule is advisory and never
// blocks scheduling, so it is skipped.
func taintRows(nodes []corev1.Node, tolerations []corev1.Toleration) []ConstraintRow {
	var rows []ConstraintRow
	for _, n := range nodes {
		for _, taint := range n.Spec.Taints {
			if taint.Effect == corev1.TaintEffectPreferNoSchedule {
				continue
			}
			if tolerationsTolerateTaint(tolerations, taint) {
				continue
			}
			rows = append(rows, ConstraintRow{
				Source:   "Node",
				Kind:     "Node",
				Name:     n.Name,
				Detail:   fmt.Sprintf("taint %s=%s:%s not tolerated", taint.Key, taint.Value, taint.Effect),
				Headroom: "not tolerated",
				Blocking: true,
			})
		}
	}
	return rows
}

func tolerationsTolerateTaint(tolerations []corev1.Toleration, taint corev1.Taint) bool {
	for _, t := range tolerations {
		if toleratesTaint(t, taint) {
			return true
		}
	}
	return false
}

func toleratesTaint(t corev1.Toleration, taint corev1.Taint) bool {
	if t.Effect != "" && t.Effect != taint.Effect {
		return false
	}
	if t.Key != "" && t.Key != taint.Key {
		return false
	}
	switch t.Operator {
	case "", corev1.TolerationOpEqual:
		return t.Value == taint.Value
	case corev1.TolerationOpExists:
		return true
	default:
		return false
	}
}
