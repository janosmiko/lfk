package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatCopiedLines(t *testing.T) {
	assert.Equal(t, "Copied 1 line", formatCopiedLines(1))
	assert.Equal(t, "Copied 2 lines", formatCopiedLines(2))
	assert.Equal(t, "Copied 100 lines", formatCopiedLines(100))
}
