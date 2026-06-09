package app

import "github.com/janosmiko/lfk/internal/ui"

// logBufferTrimSlack is how far a live log buffer may grow past
// ui.ConfigLogMaxLines before the oldest lines are dropped. Trimming copies
// the retained tail into a fresh slice so the old backing array is freed (a
// plain reslice would keep the whole array alive, defeating the cap).
// Batching the copy over this many lines keeps the amortised append cost O(1)
// instead of O(n) once the cap is reached.
const logBufferTrimSlack = 4096

// capLogLines drops the oldest lines from buf once it grows past
// ui.ConfigLogMaxLines + logBufferTrimSlack, returning the trimmed slice and
// the number of leading lines removed so callers can shift absolute offsets
// (scroll, cursor, visual anchor). It is a no-op until the slack is exceeded,
// and disabled entirely when the cap is non-positive.
func capLogLines(buf []string) ([]string, int) {
	maxLines := ui.ConfigLogMaxLines
	if maxLines <= 0 || len(buf) <= maxLines+logBufferTrimSlack {
		return buf, 0
	}
	drop := len(buf) - maxLines
	out := make([]string, maxLines)
	copy(out, buf[drop:])
	return out, drop
}

// shiftLogOffset moves an absolute log-line offset back by drop, clamping at
// zero, so scroll / cursor / visual anchors keep pointing at the same content
// after the oldest lines are trimmed. A negative offset (e.g. an inactive
// cursor of -1) is returned unchanged.
func shiftLogOffset(offset, drop int) int {
	if offset < 0 {
		return offset
	}
	if offset < drop {
		return 0
	}
	return offset - drop
}
