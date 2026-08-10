package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/ui"
)

// sanitizeDescribeContent sanitizes multi-line content before it is stored
// in describeView.content. Callers are the describe view's five producers:
// kubectl describe, helm values, command-bar output, trivy/misc command
// output, and gitops watch — none of it lfk-controlled.
//
// This used to run on every call to viewDescribe, a View function bubbletea
// invokes every frame, re-splitting and re-sanitizing the whole buffer each
// render even though the content only changes on load. Running it once here,
// at load time, means viewDescribe just splits already-sanitized content.
//
// Sanitize before any styling/highlighting is applied, never after, or
// lfk's own escapes get stripped along with an injected one. Uses the
// ANSI-aware body sanitizer (not ui.SanitizeTerminalText) so SGR colour and
// tab alignment survive.
func sanitizeDescribeContent(content string) string {
	rawLines := strings.Split(content, "\n")
	lines := make([]string, len(rawLines))
	for i, l := range rawLines {
		lines[i] = ui.SanitizeLogBody(ui.StripBidiOverrides(l), true)
	}
	return strings.Join(lines, "\n")
}
