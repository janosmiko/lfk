package app

import (
	"sync"
)

// scrollback captures lines that flow through the PTY byte stream so the
// user can scroll back past what fits in the live vt10x viewport. It is
// independent of the vt10x terminal — vt10x has no scroll-out hook, so we
// tee the bytes and parse a coarse line-stream (newlines split, carriage
// return resets the in-progress line, backspace deletes one rune, ANSI
// escape sequences are stripped).
//
// The result is faithful for everyday shell output (prompts, ls, kubectl
// output, cat). It is intentionally lossy for full-screen curses programs
// (vim, less, htop): we capture the byte stream they wrote, not the
// rendered screen, so their scrollback view can be a bit messy. Once the
// curses program exits, normal output resumes capturing cleanly.
type scrollback struct {
	mu        sync.Mutex
	buf       []string // ring of cap entries; nil/empty when not yet full
	head      int      // index of the next slot to write
	full      bool
	cap       int
	cur       []byte // in-progress line (no terminating newline yet)
	pendingCR bool   // saw \r at the tail of the previous decoded byte
}

func newScrollback(capacity int) *scrollback {
	if capacity <= 0 {
		capacity = 1
	}
	return &scrollback{
		buf: make([]string, capacity),
		cap: capacity,
	}
}

// Write parses bytes from the PTY into committed lines + an in-progress
// line. Implements io.Writer so it can be used directly as a tee target.
//
// Line-ending handling:
//
//   - "\n" alone:           commit current line.
//   - "\r\n" (PTY default with ONLCR enabled): commit current line.
//   - "\r\r\n" (ONLCR runs over a stream that already had "\r\n",
//     producing doubled CRs — common when lfk's local PTY is a
//     pass-through for kubectl exec, which itself reads through a
//     remote PTY that already did ONLCR): commit current line. Treat
//     any run of \r terminated by \n as a single line terminator.
//   - "\r" not followed by "\n": carriage return overprint — reset
//     the in-progress line so a prompt redraw replaces what was there.
//
// The CR vs CRLF decision is made one byte later, so a "\r" at the
// tail of one Write must persist into the next Write to be paired
// with a possible "\n". pendingCR carries that state across calls.
func (s *scrollback) Write(p []byte) (int, error) {
	if s == nil {
		return len(p), nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stripped := stripAnsiBytes(p)
	for _, b := range stripped {
		if s.pendingCR {
			s.pendingCR = false
			switch b {
			case '\n':
				// CR(...)LF — commit the current line as a unit.
				s.commitLocked()
				continue
			case '\r':
				// Doubled CR (e.g. ONLCR-stacked: pod's PTY produced
				// "\r\n" which gets ONLCR'd again to "\r\r\n" by lfk's
				// local PTY). Stay in the pending state until the run
				// terminates — preserve cur so the eventual \n commits
				// real content, not an empty line. Without this guard
				// every line in such a stream is committed empty.
				s.pendingCR = true
				continue
			}
			// Lone CR — overprint: drop the partial buffer, then process
			// the current byte normally.
			s.cur = s.cur[:0]
		}
		switch b {
		case '\n':
			s.commitLocked()
		case '\r':
			s.pendingCR = true
		case '\b':
			if len(s.cur) > 0 {
				// Pop one rune (UTF-8: skip continuation bytes back to
				// the start of the rune).
				i := len(s.cur) - 1
				for i > 0 && s.cur[i]&0xC0 == 0x80 {
					i--
				}
				s.cur = s.cur[:i]
			}
		default:
			s.cur = append(s.cur, b)
		}
	}
	return len(p), nil
}

// commitLocked appends the in-progress line to the ring. Caller holds mu.
func (s *scrollback) commitLocked() {
	s.buf[s.head] = string(s.cur)
	s.cur = s.cur[:0]
	s.head++
	if s.head >= s.cap {
		s.head = 0
		s.full = true
	}
}

// Snapshot returns a copy of all committed lines in oldest-first order.
// The in-progress (no-newline-yet) line is intentionally excluded — it is
// still being built and would race with the render.
func (s *scrollback) Snapshot() []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.full {
		out := make([]string, s.head)
		copy(out, s.buf[:s.head])
		return out
	}
	out := make([]string, 0, s.cap)
	out = append(out, s.buf[s.head:]...)
	out = append(out, s.buf[:s.head]...)
	return out
}

// Len returns the number of committed lines currently held.
func (s *scrollback) Len() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.full {
		return s.cap
	}
	return s.head
}

// stripAnsiBytes removes CSI ANSI escape sequences from p. It is a coarse
// pass that handles the bulk of what shells emit (CSI / OSC /
// single-character ESC sequences) without trying to be a full terminal
// emulator — that's what vt10x is for. The output is suitable for
// scrollback storage; it must not be fed back into a vt10x.
func stripAnsiBytes(p []byte) []byte {
	out := make([]byte, 0, len(p))
	for i := 0; i < len(p); i++ {
		b := p[i]
		if b != 0x1B { // ESC
			out = append(out, b)
			continue
		}
		// ESC sequence — consume according to its kind.
		if i+1 >= len(p) {
			return out // truncated escape; drop
		}
		next := p[i+1]
		switch next {
		case '[':
			// CSI: ESC [ <params> <final byte 0x40-0x7E>
			j := i + 2
			for j < len(p) && (p[j] < 0x40 || p[j] > 0x7E) {
				j++
			}
			if j < len(p) {
				j++ // consume final byte
			}
			i = j - 1
		case ']':
			// OSC: ESC ] ... BEL or ESC \
			j := i + 2
			for j < len(p) {
				if p[j] == 0x07 { // BEL
					j++
					break
				}
				if p[j] == 0x1B && j+1 < len(p) && p[j+1] == '\\' {
					j += 2
					break
				}
				j++
			}
			i = j - 1
		default:
			// Two-byte ESC sequence (ESC = D, ESC M, ESC c, etc.) —
			// consume the next byte.
			i++
		}
	}
	return out
}
