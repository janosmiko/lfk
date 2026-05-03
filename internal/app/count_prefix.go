package app

import "strconv"

// maxCountPrefix caps the parsed count to keep `cursor + n` arithmetic in the
// call sites from overflowing on a pathologically large prefix. One million
// lines is comfortably larger than any real viewer fixture (log buffers cap
// well below this) so the cap is invisible to actual users; what it prevents
// is a 19-digit count silently wrapping into a negative cursor and panicking
// on slice indexing downstream.
const maxCountPrefix = 1_000_000

// parseCountPrefix parses the digit-prefix buffer that powers count-prefixed
// commands (e.g. `123y`, `123G`, `123j`, `123k`) and returns the count to
// apply. An empty or invalid buffer falls back to 1 so plain `j`/`k`/`y` keep
// their single-step behaviour. Counts above maxCountPrefix saturate at the
// cap.
func parseCountPrefix(buf string) int {
	if buf == "" {
		return 1
	}
	n, err := strconv.Atoi(buf)
	if err != nil || n < 1 {
		return 1
	}
	return min(n, maxCountPrefix)
}

// consumeCountPrefix parses the digit-prefix buffer and clears it in one step.
// Every count-prefixed handler must consume the buffer (so the digits don't
// leak into the next command), so doing parse + clear together keeps the call
// sites symmetric and prevents anyone forgetting the second half.
func consumeCountPrefix(buf *string) int {
	n := parseCountPrefix(*buf)
	*buf = ""
	return n
}

// vimScrollStep mirrors vim's `[count]CTRL-D` / `[count]CTRL-U` semantics. Vim
// keeps a per-window 'scroll' option (default: half the window height) that
// drives both keys; an explicit count first sets 'scroll' to min(count,
// winheight), then scrolls by that amount. Subsequent uncounted presses reuse
// the new value (sticky), and the same option is shared between CTRL-D and
// CTRL-U so `5CTRL-D` then `CTRL-U` moves 5 lines back up.
//
// buf is the viewer's count-prefix buffer (consumed unconditionally). option
// points to the viewer's sticky 'scroll' field, updated in place when a count
// is given. viewport is the visible content height (analogous to vim's
// w_height); the count is clamped to it so 999<C-d> in a 30-line viewport
// behaves like vim's empirical cap (no scroll past one screen per press).
//
// option == 0 is the "default" sentinel: no counted press has happened yet,
// so plain <C-d>/<C-u> falls back to viewport/2 — vim's default.
func vimScrollStep(buf *string, option *int, viewport int) int {
	hadCount := *buf != ""
	n := consumeCountPrefix(buf)
	if hadCount {
		clamped := min(n, max(viewport, 1))
		*option = clamped
		return clamped
	}
	return scrollStep(*option, viewport)
}

// scrollStep returns the current sticky `<C-d>/<C-u>` step value, falling back
// to viewport/2 when no counted press has set it yet. Used by visual mode and
// any other call site that doesn't consume a count itself.
func scrollStep(option, viewport int) int {
	if option > 0 {
		return option
	}
	return max(viewport/2, 1)
}
