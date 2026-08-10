package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/logger"
)

// TestAppendLoggerUIEntry_SanitizesErrorLog guards a fourth writer to
// m.errorLog beyond addLogEntry/setStatusMessage/setErrorFromErr:
// appendLoggerUIEntry relays logger.UIEntry values (which can carry
// err.Error() text from cluster interactions) into the same overlay and
// bypassed the sanitizing boundary the others use.
func TestAppendLoggerUIEntry_SanitizesErrorLog(t *testing.T) {
	m := Model{}
	m.appendLoggerUIEntry(logger.UIEntry{
		Time:    time.Now(),
		Level:   "ERR",
		Message: "boom \x9d52;c;SGVsbG8=\x9c tail",
	})
	require.Len(t, m.errorLog, 1)
	assert.NotContains(t, m.errorLog[0].Message, "\x9d")
	assert.NotContains(t, m.errorLog[0].Message, "\x9c")
}

// TestAppendLoggerUIEntry_OrdinaryContentUnaffected guards against
// sanitizing legitimate non-ASCII log messages into mush.
func TestAppendLoggerUIEntry_OrdinaryContentUnaffected(t *testing.T) {
	m := Model{}
	m.appendLoggerUIEntry(logger.UIEntry{
		Time:    time.Now(),
		Level:   "INF",
		Message: "café RÉSUMÉ déployé",
	})
	require.Len(t, m.errorLog, 1)
	assert.Equal(t, "café RÉSUMÉ déployé", m.errorLog[0].Message)
}

// TestAppendLoggerUIEntry_Caps guards the existing 500-entry cap survives
// routing through the shared sanitizing helper.
func TestAppendLoggerUIEntry_Caps(t *testing.T) {
	m := Model{}
	for range 510 {
		m.appendLoggerUIEntry(logger.UIEntry{Time: time.Now(), Level: "INF", Message: "entry"})
	}
	assert.Len(t, m.errorLog, 500)
}
