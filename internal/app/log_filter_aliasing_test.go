package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// TestCommitFilterInputDoesNotAliasOriginal ensures that adding a rule via
// the value-receiver commit path does not write into the caller's backing
// array when there is spare capacity.
//
// Before the fix, `m.logRules = append(m.logRules, rule)` on a value
// receiver could place the new rule into the caller's shared backing
// array, so index-level writes on the returned model's slice would leak
// back into the caller's slice. This test asserts the fix by mutating
// the returned slice and verifying the original is untouched.
func TestCommitFilterInputDoesNotAliasOriginal(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	r1, _ := NewPatternRule("a", PatternSubstring, false)

	// Pre-allocate capacity so append has room to reuse the backing array.
	rules := make([]Rule, 1, 4)
	rules[0] = r1

	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: true,
		logSeverityDetector: d,
		logFilterEditingIdx: -1,
		logRules:            rules,
	}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, d)

	m.logFilterInput.Set("b")
	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})

	// The returned model has the new rule.
	assert.Len(t, rm.logRules, 2)

	// The original model's slice must have its original length.
	assert.Len(t, m.logRules, 1, "original model slice length unchanged")
	assert.Same(t, r1, m.logRules[0], "original model rule unchanged")

	// If the returned slice still shares the backing array, mutating it
	// would corrupt the original. Assert it does not.
	replacement, _ := NewPatternRule("z", PatternSubstring, true)
	rm.logRules[0] = replacement
	assert.Same(t, r1, m.logRules[0], "mutating returned slice must not mutate original")
}

// TestDeleteSelectedRuleDoesNotAliasOriginal ensures that the in-place
// slice shift used by deleteSelectedRule does not corrupt the caller's
// slice.
func TestDeleteSelectedRuleDoesNotAliasOriginal(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	r1, _ := NewPatternRule("a", PatternSubstring, false)
	r2, _ := NewPatternRule("b", PatternSubstring, false)
	r3, _ := NewPatternRule("c", PatternSubstring, false)

	rules := []Rule{r1, r2, r3}
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logFilterListCursor: 0,
		logRules:            rules,
		logSeverityDetector: d,
	}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, d)

	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	assert.Len(t, rm.logRules, 2)

	// Original slice must still be [r1, r2, r3] in order.
	assert.Len(t, m.logRules, 3)
	assert.Same(t, r1, m.logRules[0], "original slice[0] unchanged")
	assert.Same(t, r2, m.logRules[1], "original slice[1] unchanged")
	assert.Same(t, r3, m.logRules[2], "original slice[2] unchanged")
}

// TestMoveSelectedRuleDownDoesNotAliasOriginal ensures swapping adjacent
// rules does not corrupt the caller's slice.
func TestMoveSelectedRuleDownDoesNotAliasOriginal(t *testing.T) {
	r1, _ := NewPatternRule("a", PatternSubstring, false)
	r2, _ := NewPatternRule("b", PatternSubstring, false)

	rules := []Rule{r1, r2}
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: false,
		logFilterListCursor: 0,
		logRules:            rules,
	}

	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})
	assert.Equal(t, []Rule{r2, r1}, rm.logRules, "swap reflected in returned model")

	// Original slice must still be [r1, r2].
	assert.Same(t, r1, m.logRules[0], "original slice[0] preserved")
	assert.Same(t, r2, m.logRules[1], "original slice[1] preserved")
}

// TestCommitFilterInputEditDoesNotAliasOriginal ensures that editing an
// existing rule via Enter writes to a fresh slice, not the caller's
// backing array.
func TestCommitFilterInputEditDoesNotAliasOriginal(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	r1, _ := NewPatternRule("a", PatternSubstring, false)
	r2, _ := NewPatternRule("b", PatternSubstring, false)

	rules := []Rule{r1, r2}
	m := Model{
		overlay:             overlayLogFilter,
		logFilterModalOpen:  true,
		logFilterFocusInput: true,
		logFilterEditingIdx: 0, // editing rule index 0
		logRules:            rules,
		logSeverityDetector: d,
	}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, d)
	m.logFilterInput.Set("replaced")

	rm, _ := m.handleLogFilterOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
	// Returned model's rule at index 0 is a new rule, not the original.
	assert.NotSame(t, r1, rm.logRules[0])

	// Original model's slice MUST still hold r1 at index 0.
	assert.Same(t, r1, m.logRules[0], "original rule preserved after edit")
	assert.Same(t, r2, m.logRules[1], "original rule[1] preserved")
}
