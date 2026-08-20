package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// kubectl explain prints the KIND/VERSION preamble even when it fails, and the
// only useful line is the one starting with "error:". Showing the rest fills
// the pane with noise.
func TestParseExplainError(t *testing.T) {
	tests := []struct {
		name   string
		output string
		cmdErr error
		want   string
	}{
		{
			name: "field does not exist",
			output: "KIND:       Pod\nVERSION:    v1\n\n" +
				`error: field "checksum/config" does not exist` + "\n",
			cmdErr: errors.New("exit status 1"),
			want:   `field "checksum/config" does not exist`,
		},
		{
			name:   "error line without a preamble",
			output: "error: the server doesn't have a resource type \"widgets\"\n",
			cmdErr: errors.New("exit status 1"),
			want:   `the server doesn't have a resource type "widgets"`,
		},
		{
			name:   "multi-line error keeps the first line only",
			output: "error: field \"x\" does not exist\nUse \"kubectl api-resources\" for a complete list\n",
			cmdErr: errors.New("exit status 1"),
			want:   `field "x" does not exist`,
		},
		{
			name:   "no error line falls back to the output",
			output: "something went sideways\n",
			cmdErr: errors.New("exit status 1"),
			want:   "something went sideways",
		},
		{
			name:   "empty output falls back to the command error",
			output: "",
			cmdErr: errors.New("exit status 1"),
			want:   "exit status 1",
		},
		{
			name:   "preamble only, no error line",
			output: "KIND:       Pod\nVERSION:    v1\n",
			cmdErr: errors.New("exit status 1"),
			want:   "KIND:       Pod\nVERSION:    v1",
		},
		{
			name:   "error line carrying a credential is redacted",
			output: `error: could not reach server: token=hunter2-EXPLAINMARKER` + "\n",
			cmdErr: errors.New("exit status 1"),
			want:   "could not reach server: token=[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseExplainError(tt.output, tt.cmdErr)
			assert.EqualError(t, got, tt.want)
		})
	}
}

// The exit status carries no information the message does not already give, so
// it must not be prefixed onto what the pane shows.
func TestParseExplainErrorDropsExitStatus(t *testing.T) {
	got := parseExplainError(
		"KIND:       Pod\n\nerror: field \"nope\" does not exist\n",
		errors.New("exit status 1"),
	)

	assert.NotContains(t, got.Error(), "exit status")
	assert.NotContains(t, got.Error(), "KIND:")
}

// An exec credential plugin can print its secret straight into the "error:"
// line kubectl explain fails with. That line still reaches the pane.
func TestParseExplainErrorRedactsCredentials(t *testing.T) {
	got := parseExplainError(
		"KIND:       Pod\n\nerror: auth failed: token=hunter2-EXPLAINMARKER\n",
		errors.New("exit status 1"),
	)

	assert.NotContains(t, got.Error(), "hunter2-EXPLAINMARKER", "must not leak the credential")
	assert.Contains(t, got.Error(), "[REDACTED]", "must contain a redaction placeholder")
	assert.Contains(t, got.Error(), "auth failed", "must preserve the non-secret reason")
}
