package app

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Custom-action templates run via sh -c, and column values are arbitrary
// cluster-controlled data (labels, image strings, annotations). A value with
// shell metacharacters must be quoted so it can't break out of its argument
// and execute. The kubeconfig context name is the same class of risk (kubectl
// permits metacharacters in context names).
func TestExpandCustomActionTemplateQuotesInjection(t *testing.T) {
	actx := actionContext{
		name:      "my-pod",
		namespace: "default",
		context:   `prod"$(touch /tmp/pwn)"`,
		kind:      "Pod",
		columns: []model.KeyValue{
			{Key: "Image", Value: "nginx; rm -rf /"},
		},
	}

	// The metacharacters must reach the shell quoted (inert), not bare. Proof is
	// the round-trip: a real shell must echo each value literally, which it can
	// only do if $(...) / ; / etc. were neutralized rather than executed.
	gotImage := shellEcho(t, expandCustomActionTemplate("echo {Image}", actx))
	assert.Equal(t, "nginx; rm -rf /", gotImage, "column value must round-trip literally")

	gotCtx := shellEcho(t, expandCustomActionTemplate("echo {context}", actx))
	assert.Equal(t, `prod"$(touch /tmp/pwn)"`, gotCtx,
		"context value must round-trip literally — the $(...) must not execute")
}

// shellEcho runs `sh -c <cmd>` and returns trimmed stdout. The cmd is expected
// to be an `echo ...` produced by the template expander.
func shellEcho(t *testing.T, cmd string) string {
	t.Helper()
	out, err := exec.Command("sh", "-c", cmd).Output()
	require.NoError(t, err)
	return strings.TrimRight(string(out), "\n")
}
