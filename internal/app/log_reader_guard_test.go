package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Switching into a logs tab must not spawn a second reader when one is already
// outstanding — duplicate readers race the same channel and deliver lines out
// of order. waitForLogLineIfIdle skips the arm while a reader is in flight.
func TestWaitForLogLineIfIdleSkipsDuplicateReader(t *testing.T) {
	m := baseModelWithFakeClient()
	m.logReaderInFlight = make(map[chan string]bool)
	ch := make(chan string, 4)
	m.logView.ch = ch

	first := m.waitForLogLineIfIdle()
	require.NotNil(t, first, "first arm must spawn a reader")
	require.True(t, m.logReaderInFlight[ch], "reader must be marked outstanding")

	second := m.waitForLogLineIfIdle()
	assert.Nil(t, second, "a second arm must be skipped while a reader is outstanding")
}

// Once the outstanding reader delivers, updateLogLine re-arms exactly one
// reader, so the channel still has a single reader (not zero, not two).
func TestLogReaderReArmedAfterDelivery(t *testing.T) {
	m := baseModelWithFakeClient()
	m.logReaderInFlight = make(map[chan string]bool)
	m.mode = modeLogs
	ch := make(chan string, 4)
	m.logView.ch = ch

	require.NotNil(t, m.waitForLogLineIfIdle())
	require.True(t, m.logReaderInFlight[ch])

	mm, cmd := m.updateLogLine(logLineMsg{line: "hello", ch: ch})
	m = mm.(Model)

	assert.NotNil(t, cmd, "updateLogLine must re-arm the maintained reader")
	assert.True(t, m.logReaderInFlight[ch], "exactly one reader must remain outstanding")
}

// Stream end drops the channel from the in-flight map so closed channels are
// not retained.
func TestLogReaderFlagDroppedOnStreamEnd(t *testing.T) {
	m := baseModelWithFakeClient()
	m.logReaderInFlight = make(map[chan string]bool)
	m.mode = modeLogs
	ch := make(chan string)
	m.logView.ch = ch
	m.logReaderInFlight[ch] = true

	mm, _ := m.updateLogLine(logLineMsg{done: true, ch: ch})
	m = mm.(Model)

	_, present := m.logReaderInFlight[ch]
	assert.False(t, present, "closed channel must be removed from the in-flight map")
}

// Integration substitute for live-cluster verification: drive the real reader
// arming + updateLogLine loop over many lines while repeatedly simulating
// tab-switch arms. Asserts every line is captured exactly once, in order, and
// that exactly one reader is ever outstanding (the guard prevents duplicates
// without dropping a single line).
func TestLogStreamingNoLineLossUnderTabSwitchStorm(t *testing.T) {
	m := baseModelWithFakeClient()
	m.logReaderInFlight = make(map[chan string]bool)
	m.mode = modeLogs
	m.logView.follow = true
	ch := make(chan string, 1)
	m.logView.ch = ch

	want := make([]string, 0, 200)
	for i := range 200 {
		want = append(want, logLineForIndex(i))
	}

	// Initial arm (as a stream start would do).
	cmd := m.waitForLogLine()
	require.NotNil(t, cmd)

	for i, line := range want {
		// Simulate aggressive tab switching: every arm must be skipped because a
		// reader is already outstanding.
		if i%3 == 0 {
			assert.Nil(t, m.waitForLogLineIfIdle(),
				"tab-switch arm must be skipped while a reader is outstanding (i=%d)", i)
		}

		ch <- line
		msg := cmd()
		mm, next := m.updateLogLine(msg.(logLineMsg))
		m = mm.(Model)
		require.NotNil(t, next, "reader must stay armed mid-stream")
		cmd = next
	}

	assert.Equal(t, want, m.logView.lines, "every streamed line must be captured exactly once, in order")
}

func logLineForIndex(i int) string {
	const digits = "0123456789"
	s := "line-"
	if i == 0 {
		return s + "0"
	}
	var b []byte
	for n := i; n > 0; n /= 10 {
		b = append([]byte{digits[n%10]}, b...)
	}
	return s + string(b)
}
