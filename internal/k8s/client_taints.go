package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/model"
)

// GetNodeTaints returns the node's spec.taints in API order.
func (c *Client) GetNodeTaints(ctx context.Context, contextName, name string) ([]model.Taint, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}
	node, err := cs.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting node: %w", err)
	}
	taints := make([]model.Taint, 0, len(node.Spec.Taints))
	for _, t := range node.Spec.Taints {
		taints = append(taints, model.Taint{Key: t.Key, Value: t.Value, Effect: string(t.Effect)})
	}
	return taints, nil
}

// UpdateNodeTaints re-fetches the node and writes spec.taints as the
// current list minus removals plus additions, matching by key+effect
// identity (model.ComputeFinalTaints) — taints added between the
// caller's read and this update survive untouched. Kept taints retain
// their original corev1 struct so server-set fields (TimeAdded on
// NoExecute) are preserved. The write is a plain Update, so a
// conflicting concurrent modification fails with a Conflict error
// instead of being clobbered.
func (c *Client) UpdateNodeTaints(ctx context.Context, contextName, name string, removals, additions []model.Taint) error {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return err
	}
	node, err := cs.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	type taintID struct{ key, effect string }
	existing := make([]model.Taint, 0, len(node.Spec.Taints))
	byID := make(map[taintID]corev1.Taint, len(node.Spec.Taints))
	for _, t := range node.Spec.Taints {
		existing = append(existing, model.Taint{Key: t.Key, Value: t.Value, Effect: string(t.Effect)})
		byID[taintID{t.Key, string(t.Effect)}] = t
	}

	final := model.ComputeFinalTaints(existing, removals, additions)
	out := make([]corev1.Taint, 0, len(final))
	for _, ft := range final {
		if orig, ok := byID[taintID{ft.Key, ft.Effect}]; ok {
			out = append(out, orig)
			continue
		}
		out = append(out, corev1.Taint{Key: ft.Key, Value: ft.Value, Effect: corev1.TaintEffect(ft.Effect)})
	}
	node.Spec.Taints = out

	if _, err := cs.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating node taints: %w", err)
	}
	return nil
}
