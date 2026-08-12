package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
)

// The repo's cursor convention is vim-style: j/k, g/G, ctrl+d/ctrl+u for a
// half page, ctrl+f/ctrl+b for a full page, with arrows and pgup/pgdn as
// aliases. This overlay shipped with only j/k, arrows and g/G, so a user
// reaching for ctrl+d here got nothing while every other overlay moved.
func TestExportStripKey_SupportsFullVimNavigation(t *testing.T) {
	last := len(k8s.TemplateCategories) - 1

	cases := map[string]struct {
		start int
		key   string
		want  int
	}{
		"ctrl+d half page down": {0, "ctrl+d", len(k8s.TemplateCategories) / 2},
		"shift+down alias":      {0, "shift+down", len(k8s.TemplateCategories) / 2},
		"ctrl+u half page up":   {last, "ctrl+u", last - len(k8s.TemplateCategories)/2},
		"ctrl+f full page down": {0, "ctrl+f", last},
		"pgdown alias":          {0, "pgdown", last},
		"ctrl+b full page up":   {last, "ctrl+b", 0},
		"pgup alias":            {last, "pgup", 0},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := basePush80Model()
			m.overlay = overlayExportStrip
			m.exportTemplatePicker.stripCursor = tc.start

			out, _ := m.handleExportStripKey(tea.KeyPressMsg{Code: 0, Text: tc.key})
			got := out.(Model).exportTemplatePicker.stripCursor

			assert.Equal(t, tc.want, got, "%s should move the cursor to row %d", tc.key, tc.want)
		})
	}
}
