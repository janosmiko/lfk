package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/ui"
)

// The mode 2027 report is what tells lfk which width the renderer settled on.
// Reading it wrong puts the icon normalization out of step with the renderer,
// which is the misalignment of #604 in reverse.
func TestUpdateModeReport_UnicodeCore(t *testing.T) {
	tests := []struct {
		name  string
		value ansi.ModeSetting
		want  bool
	}{
		// Bubble Tea switches its renderer to grapheme width on any of these:
		// the terminal answering at all is what proves it knows the mode.
		{"set", ansi.ModeSet, true},
		{"reset", ansi.ModeReset, true},
		{"permanently set", ansi.ModePermanentlySet, true},
		// No grapheme clustering to be had, so the renderer stays on wcwidth.
		{"permanently reset", ansi.ModePermanentlyReset, false},
		{"not recognized", ansi.ModeNotRecognized, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			old := ui.UnicodeCoreActive
			defer func() { ui.UnicodeCoreActive = old }()
			ui.UnicodeCoreActive = !tc.want // start from the wrong answer

			m := Model{}
			mdl, cmd := m.updateModeReport(tea.ModeReportMsg{
				Mode:  ansi.ModeUnicodeCore,
				Value: tc.value,
			})

			assert.Equal(t, tc.want, ui.UnicodeCoreActive)
			assert.Nil(t, cmd, "the next render picks the change up; nothing to schedule")
			assert.IsType(t, Model{}, mdl)
		})
	}
}

func TestUpdateModeReport_LeavesOtherModesAlone(t *testing.T) {
	for _, active := range []bool{true, false} {
		old := ui.UnicodeCoreActive
		ui.UnicodeCoreActive = active

		m := Model{}
		_, cmd := m.updateModeReport(tea.ModeReportMsg{
			Mode:  ansi.ModeSynchronizedOutput,
			Value: ansi.ModeReset,
		})

		assert.Equal(t, active, ui.UnicodeCoreActive,
			"a report about another mode says nothing about grapheme clustering")
		assert.Nil(t, cmd)
		ui.UnicodeCoreActive = old
	}
}
