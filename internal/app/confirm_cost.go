package app

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// confirmCost is everything a destructive confirm box states, gathered in one
// place so the rows cannot contradict each other.
//
// The rows are named after the parts of the answer, not after the calls that
// found them. Scope says what else stops existing, Availability says what
// stops serving, Risk says what refuses or is left in a strange state. Each
// takes the cascade policy, which is why an earlier split by data source could
// print "3 pods removed" and "3 pods stay" in the same box.
type confirmCost struct {
	// radius is the pod side: which pods go, how many stay ready, and which
	// budgets cover them. nil where the action has no pods to measure.
	radius *k8s.BlastRadius
	// deps is the owner side: what the target owns, at any depth.
	deps dependentsState
	// kind names the target in the Scope row. Empty for a bulk selection,
	// which has no single kind.
	kind string
	// policy is the selected cascade. Read only when cascades is set.
	policy model.DeletePropagation
	// cascades marks a delete whose policy decides the outcome. Drain and
	// scale leave it clear: neither cascades, so neither can orphan anything.
	cascades bool
	// enforced marks an action a budget can actually refuse. Only the
	// eviction API honours a PodDisruptionBudget, so only a drain is stopped.
	enforced bool
	// loading covers both fetches at once. The box shows one placeholder
	// rather than half an answer that the other half would contradict.
	loading bool
}

// buildConfirmCost gathers what the open confirm box states from the model.
//
// cascades marks a delete whose policy applies; enforced marks an action a
// budget can refuse, which is only ever a drain. A bulk selection has no single
// kind, so the Scope row names the rows instead of a kind.
func (m Model) buildConfirmCost(cascades, enforced bool) confirmCost {
	kind := m.actionCtx.kind
	if m.bulkMode && len(m.bulkItems) > 0 {
		kind = ""
	}
	// Both fetches share one placeholder: half an answer would be contradicted
	// by the half still in flight.
	loading := m.blast.loading || (cascades && m.deps.loading)
	return confirmCost{
		radius:   m.blast.radius,
		deps:     m.deps,
		kind:     kind,
		policy:   m.deletePropagation(),
		cascades: cascades,
		enforced: enforced,
		loading:  loading,
	}
}

// confirmCostNotes builds the Scope, Availability and Risk rows, in that
// order. A row with nothing to say is left out rather than padded, so a bare
// pod delete keeps a small box.
func confirmCostNotes(c confirmCost) []ui.ConfirmNote {
	if c.loading {
		return []ui.ConfirmNote{{Label: "Scope", Text: "working out what this costs..."}}
	}
	notes := make([]ui.ConfirmNote, 0, 3)
	for _, row := range []ui.ConfirmNote{c.scopeRow(), c.availabilityRow(), c.riskRow()} {
		if row.Text != "" {
			notes = append(notes, row)
		}
	}
	return notes
}

// scopeRow says what stops existing besides the target itself. The target is
// named in the question a line above, so listing it again would be noise.
func (c confirmCost) scopeRow() ui.ConfirmNote {
	if !c.cascades {
		// Drain and scale own nothing; their scope is the pods they take.
		return ui.ConfirmNote{Label: "Scope", Text: c.podScope()}
	}
	if c.deps.count == nil {
		// The walk cannot follow this kind, so the box says nothing rather
		// than a zero it cannot stand behind.
		return ui.ConfirmNote{}
	}

	summary := c.deps.count.Summary()
	uncounted := uncountedSuffix(c.deps.count.Uncounted)
	switch {
	case summary == "" && uncounted == "":
		// Nothing else goes, so the row would only repeat the question.
		return ui.ConfirmNote{}
	case summary == "":
		return ui.ConfirmNote{Label: "Scope", Text: "nothing countable" + uncounted}
	case c.policy.OrphansDependents():
		return ui.ConfirmNote{Label: "Scope", Text: c.targetOnly(), Warn: true}
	case c.policy.DefersToServer():
		return ui.ConfirmNote{
			Label: "Scope",
			Text:  fmt.Sprintf("%s, plus %s if the server cascades", c.targetName(), summary),
			Warn:  true,
		}
	}
	return ui.ConfirmNote{Label: "Scope", Text: summary + uncounted}
}

// availabilityRow says what stops serving. It reads the pod fetch, which
// counts the running pods and the ready replicas behind them.
func (c confirmCost) availabilityRow() ui.ConfirmNote {
	if c.radius == nil {
		return ui.ConfirmNote{}
	}
	if c.cascades {
		switch {
		case c.policy.OrphansDependents():
			return ui.ConfirmNote{Label: "Availability", Text: c.unchangedText()}
		case c.policy.DefersToServer():
			return ui.ConfirmNote{Label: "Availability", Text: "depends on the server default"}
		}
	}
	if c.radius.Evicting == 0 {
		return ui.ConfirmNote{Label: "Availability", Text: "no running pods"}
	}
	if c.radius.ReadyBefore == 0 {
		// Drain and bulk have no single workload, so there is no replica count
		// to measure against and the Scope row already gave the number.
		return ui.ConfirmNote{}
	}
	return ui.ConfirmNote{
		Label: "Availability",
		Text:  fmt.Sprintf("%d of %d ready after", c.radius.ReadyAfter, c.radius.ReadyBefore),
	}
}

// riskRow says what refuses the action, or what the action leaves behind in a
// state nothing manages.
//
// It is always one line: a policy that keeps dependents evicts nothing, so a
// budget breach and an orphan hazard can never both apply.
func (c confirmCost) riskRow() ui.ConfirmNote {
	if c.cascades {
		switch {
		case c.policy.OrphansDependents():
			summary := ""
			if c.deps.count != nil {
				summary = c.deps.count.Summary()
			}
			if summary == "" {
				return ui.ConfirmNote{}
			}
			return ui.ConfirmNote{
				Label: "Risk", Text: summary + " left with no owner", Warn: true,
			}
		case c.policy.DefersToServer():
			return ui.ConfirmNote{
				Label: "Risk",
				Text: "the default is Background for most kinds, " +
					"Orphan for Job and ReplicationController",
				Warn: true,
			}
		}
	}
	if c.radius == nil || c.radius.Evicting == 0 || len(c.radius.PDBs) == 0 {
		// No pod goes, or no budget covers the ones that do. An absent
		// constraint is not a risk, and the row names risks, so it is left out
		// rather than stating that nothing applies.
		return ui.ConfirmNote{}
	}
	text, warn := budgetRow(c.radius.PDBs, c.enforced)
	return ui.ConfirmNote{Label: "Risk", Text: text, Warn: warn}
}

// podScope states the pods an action takes, for the actions that do not
// cascade and so have no owned objects to list.
func (c confirmCost) podScope() string {
	if c.radius == nil {
		return ""
	}
	uncounted := uncountedSuffix(c.radius.Uncounted)
	if c.radius.Evicting == 0 {
		if uncounted == "" {
			return ""
		}
		return "nothing countable" + uncounted
	}
	return fmt.Sprintf("%d %s%s",
		c.radius.Evicting, plural(c.radius.Evicting, "pod", "pods"), uncounted)
}

// unchangedText says an orphaning delete costs no availability, naming the
// pods that keep running where there are any.
func (c confirmCost) unchangedText() string {
	if c.radius == nil || c.radius.Evicting == 0 {
		return "unchanged"
	}
	return fmt.Sprintf("unchanged, the %d %s keep running",
		c.radius.Evicting, plural(c.radius.Evicting, "pod", "pods"))
}

// targetName names the object the question asked about, in the lower case the
// rows read in. A bulk selection has no single kind, so it says so.
func (c confirmCost) targetName() string {
	if c.kind == "" {
		return "the selected rows"
	}
	return "the " + strings.ToLower(c.kind)
}

func (c confirmCost) targetOnly() string {
	return c.targetName() + " only"
}
