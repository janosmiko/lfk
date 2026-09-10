package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// quarantineServiceListMax mirrors dependentSummaryKinds: past this many, a
// confirm box states a count instead of a half-listed set of names.
const quarantineServiceListMax = 3

// quarantineState is what the open Quarantine/Restore confirm dialog knows:
// the Services and label keys a quarantine would touch, or the label pairs
// a restore would put back.
type quarantineState struct {
	services []string
	keys     []string
	restore  map[string]string
	// req numbers each fetch, so a reply for a dialog the user already
	// closed and reopened cannot land on the new one.
	req uint64
}

// reset clears the figures and retires the request they belong to.
func (q *quarantineState) reset() {
	q.services = nil
	q.keys = nil
	q.restore = nil
	q.req++
}

// rewriteQuarantineMenuItem swaps an already-quarantined pod's Quarantine
// entry for Restore in place. Status (the hotkey chip, "Q") is kept so
// sortActionMenuItems still places it where the chip letter says it belongs.
func rewriteQuarantineMenuItem(items []model.Item, raw map[string]any) {
	if _, quarantined := quarantinedLabels(raw); !quarantined {
		return
	}
	for i, item := range items {
		if item.Name == model.ActionLabelQuarantine {
			items[i].Name = model.ActionLabelRestore
			items[i].Extra = "Put back the labels removed by Quarantine"
			return
		}
	}
}

// quarantinedLabels reports whether a pod carries QuarantinePod's
// annotation, and decodes its recorded label pairs when it does.
func quarantinedLabels(raw map[string]any) (map[string]string, bool) {
	if raw == nil {
		return nil, false
	}
	annotations, ok, err := unstructured.NestedStringMap(raw, "metadata", "annotations")
	if err != nil || !ok {
		return nil, false
	}
	value, ok := annotations[k8s.QuarantinedLabelsAnnotation]
	if !ok || value == "" {
		return nil, false
	}
	var removed map[string]string
	if err := json.Unmarshal([]byte(value), &removed); err != nil {
		return nil, false
	}
	return removed, true
}

// podHasController reports whether a pod's ownerReferences name a
// controller. IsController=true is what tells the confirm box a
// replacement pod is coming, versus an orphan pod nothing recreates.
func podHasController(raw map[string]any) bool {
	refs, ok, err := unstructured.NestedSlice(raw, "metadata", "ownerReferences")
	if err != nil || !ok {
		return false
	}
	for _, r := range refs {
		ref, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if controller, _ := ref["controller"].(bool); controller {
			return true
		}
	}
	return false
}

// quarantineConfirmNotes builds the Quarantine/Restore confirm rows in the
// grammar of confirmCostNotes: Scope, Availability, Risk.
func quarantineConfirmNotes(st quarantineState, ownedByController bool) []ui.ConfirmNote {
	if len(st.restore) > 0 {
		return quarantineRestoreNotes(st.restore)
	}
	if len(st.services) == 0 {
		return nil
	}
	riskText := "nothing recreates this pod"
	if ownedByController {
		riskText = "its controller creates a replacement pod"
	}
	return []ui.ConfirmNote{
		{Label: "Scope", Text: quarantineServiceScope(st.services)},
		{Label: "Availability", Text: quarantineLabelScope(st.keys)},
		{Label: "Risk", Text: riskText, Warn: true},
	}
}

// quarantineRestoreNotes names the label pairs a Restore puts back. Routing
// resumes rather than being taken away, so there is nothing to warn about.
func quarantineRestoreNotes(restored map[string]string) []ui.ConfirmNote {
	keys := make([]string, 0, len(restored))
	for k := range restored {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return []ui.ConfirmNote{{Label: "Scope", Text: quarantineLabelScope(keys)}}
}

// quarantineServiceScope names the Services a quarantine stops routing to,
// falling back to a count past quarantineServiceListMax.
func quarantineServiceScope(services []string) string {
	if len(services) > quarantineServiceListMax {
		return fmt.Sprintf("%d Services", len(services))
	}
	return ui.SanitizeTerminalText(strings.Join(services, ", "))
}

// quarantineLabelScope states how many label keys move, and which ones.
func quarantineLabelScope(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	return fmt.Sprintf("%d %s: %s", len(keys), plural(len(keys), "label", "labels"),
		ui.SanitizeTerminalText(strings.Join(keys, ", ")))
}
