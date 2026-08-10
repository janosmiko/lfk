package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// sanitizeDescribeContent is the describe view's single sanitizing choke
// point, called once by each producer (kubectl describe, helm values,
// command-bar output, trivy/misc command output, gitops watch) instead of
// on every viewDescribe render.

func TestSanitizeDescribeContent_StripsInjectedEscapes(t *testing.T) {
	content := "before\u202eafter\x1b[2Jgone\x1b]52;c;ZXZpbA==\x07tail"
	out := sanitizeDescribeContent(content)
	assert.NotContains(t, out, "\u202e", "bidi override must not survive")
	assert.NotContains(t, out, "\x1b[2J", "raw CSI (screen erase) must not survive")
	assert.NotContains(t, out, "\x1b]52", "OSC-52 clipboard sequence must not survive")
	assert.NotContains(t, out, "\x07")
}

func TestSanitizeDescribeContent_StripsInjectedEscapesAcrossLines(t *testing.T) {
	content := "line one\nbefore\u202eafter\x1b[2Jgone\nline three"
	out := sanitizeDescribeContent(content)
	assert.NotContains(t, out, "\u202e")
	assert.NotContains(t, out, "\x1b[2J")
	// Line count must be preserved so scroll/cursor line indices computed
	// against the raw content before sanitizing still line up.
	assert.Equal(t, 3, strings.Count(out, "\n")+1)
}

func TestSanitizeDescribeContent_PreservesSGRColour(t *testing.T) {
	// Realistic trivy-style coloured table output. SGR must survive so the
	// severity colouring the scanner intended still renders — the guard
	// against fixing the injection issue by breaking the feature.
	trivyLine := "\x1b[31mCRITICAL\x1b[0m CVE-2024-1234  openssl  1.1.1  1.1.1w"
	out := sanitizeDescribeContent(trivyLine)
	assert.Contains(t, out, "\x1b[31m", "SGR red-severity colour must survive")
}

func TestSanitizeDescribeContent_ExpandsTabs(t *testing.T) {
	// kubectl describe output is tab-and-space aligned; tabs must expand to
	// the same column stops the log viewer uses, not vanish or collapse.
	out := sanitizeDescribeContent("Name:\tmy-pod\nNamespace:\tdefault")
	assert.Contains(t, out, "Name:   my-pod")
	assert.Contains(t, out, "Namespace:      default")
}

func TestSanitizeDescribeContent_OrdinaryContentUnaffected(t *testing.T) {
	content := "Name:         my-pod\nNamespace:    default\nStatus:       Running"
	assert.Equal(t, content, sanitizeDescribeContent(content))
}
