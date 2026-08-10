package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSingleLineCell_SanitizesTerminalEscapes guards the missing sink found
// in review: SingleLineCell only stripped newline/CR/tab, so a revealed
// Secret/ConfigMap/Label value containing an 8-bit OSC-52 clipboard write or
// 8-bit CSI SGR sequence passed through byte for byte into the K/V editor's
// key/value cells.
func TestSingleLineCell_SanitizesTerminalEscapes(t *testing.T) {
	osc52 := "\x9d52;c;SGVsbG8=\x9c"
	csi := "\x9b31m"

	outOSC := SingleLineCell("secret"+osc52+"value", 80)
	assert.NotContains(t, outOSC, "\x9d")
	assert.NotContains(t, outOSC, "\x9c")

	outCSI := SingleLineCell("secret"+csi+"value", 80)
	assert.NotContains(t, outCSI, "\x9b")
}

// TestSingleLineCell_OrdinaryContentUnaffected guards against fixing the
// injection issue by mangling legitimate values, including non-ASCII text.
func TestSingleLineCell_OrdinaryContentUnaffected(t *testing.T) {
	cases := []string{
		"plain-secret-value",
		"café RÉSUMÉ token",
		"日本語のパスワード",
	}
	for _, s := range cases {
		assert.Equal(t, s, SingleLineCell(s, 80))
	}
}
