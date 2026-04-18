package app

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/ui"
)

// logFilterOverlayState builds the state object for the overlay renderer.
func (m Model) logFilterOverlayState() ui.LogFilterOverlayState {
	rows := make([]ui.LogFilterRowState, 0, len(m.logRules))
	for _, r := range m.logRules {
		rows = append(rows, ruleToRowState(r))
	}
	title := fmt.Sprintf("%s: %s", m.actionCtx.kind, m.actionCtx.name)

	// Load-preset picker items (built on demand from the sidecar).
	var pickerItems []string
	if m.logLoadPresetOpen {
		if path, err := logPresetsPath(); err == nil {
			if f, err := readPresetFile(path); err == nil {
				for _, p := range f.Presets[m.actionCtx.kind] {
					label := fmt.Sprintf("%s (%d rules)", p.Name, len(p.Rules))
					if p.Default {
						label += " [default]"
					}
					pickerItems = append(pickerItems, label)
				}
			}
		}
	}

	return ui.LogFilterOverlayState{
		Title:            title,
		IncludeMode:      m.logIncludeMode.String(),
		Rules:            rows,
		ListCursor:       m.logFilterListCursor,
		FocusInput:       m.logFilterFocusInput,
		Input:            m.logFilterInput.Value,
		SavePromptActive: m.logSavePresetPrompt,
		SavePromptInput:  m.logFilterInput.Value,
		LoadPickerActive: m.logLoadPresetOpen,
		LoadPickerItems:  pickerItems,
		LoadPickerCursor: m.logLoadPresetCursor,
	}
}

// ruleToRowState projects a Rule into the three-column shape the overlay
// renderer expects (kind + mode + pattern). Keeping the projection here,
// next to the other rule-display helpers, avoids pulling app types into
// the ui package.
func ruleToRowState(r Rule) ui.LogFilterRowState {
	switch v := r.(type) {
	case *PatternRule:
		return ui.LogFilterRowState{
			Kind:    r.Kind().String(),
			Mode:    v.Mode.String(),
			Pattern: v.Pattern,
		}
	case SeverityRule:
		return ui.LogFilterRowState{
			Kind:    r.Kind().String(),
			Mode:    "",
			Pattern: ">= " + v.Floor.String(),
		}
	case *GroupRule:
		return ui.LogFilterRowState{
			Kind:    r.Kind().String(),
			Mode:    v.Mode.String(),
			Pattern: exprSummary(v),
		}
	case *FieldRule:
		// Field rules render with the operator in the Mode column (the
		// table already reserves a column for mode, and a field rule's
		// operator is its most identifying moment) and the path-value
		// pair in the Pattern column. The hard-gate semantics ("drops
		// non-JSON lines") are documented alongside the syntax in
		// docs/keybindings.md; the row itself is kept compact so the
		// rules table scales with many field rules.
		path := strings.Join(v.Path, ".")
		if v.ArrayAny {
			path += "[]"
		}
		return ui.LogFilterRowState{
			Kind:    r.Kind().String(),
			Mode:    v.Op.String(),
			Pattern: "." + path + " " + v.Value,
		}
	}
	return ui.LogFilterRowState{
		Kind:    r.Kind().String(),
		Pattern: r.Display(),
	}
}

// exprSummary renders a Rule as a readable boolean expression for the
// rules-table Pattern column. Leaf PatternRules produce "pattern" (with
// a leading "~" for fuzzy mode and "!" when negated, regex is surfaced
// as-is). SeverityRules re-use the canonical ">= LEVEL" form. GroupRules
// are joined with " AND " (IncludeAll) or " OR " (IncludeAny) and wrapped
// in parentheses — nested groups produce nested parens. FieldRules use
// the canonical ".path OP value" form from RuleToInputString so the
// summary matches what the user would type.
func exprSummary(r Rule) string {
	switch v := r.(type) {
	case *PatternRule:
		return patternRuleSummary(v)
	case SeverityRule:
		return v.Display()
	case *GroupRule:
		parts := make([]string, 0, len(v.Children))
		for _, c := range v.Children {
			parts = append(parts, exprSummary(c))
		}
		op := " OR "
		if v.Mode == IncludeAll {
			op = " AND "
		}
		return "(" + strings.Join(parts, op) + ")"
	case *FieldRule:
		return RuleToInputString(v)
	}
	return r.Display()
}

// patternRuleSummary renders a single PatternRule as a leaf expression:
// fuzzy mode gets a leading "~", negated rules get a leading "!", and
// substring/regex patterns appear verbatim (regex metacharacters are
// recognisable on their own; no extra annotation is needed).
func patternRuleSummary(p *PatternRule) string {
	out := p.Pattern
	if p.Mode == PatternFuzzy {
		out = "~" + out
	}
	if p.Negate {
		out = "!" + out
	}
	return out
}
