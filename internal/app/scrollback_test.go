package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScrollbackBasic(t *testing.T) {
	sb := newScrollback(10)
	_, _ = sb.Write([]byte("first\nsecond\nthird\n"))
	got := sb.Snapshot()
	assert.Equal(t, []string{"first", "second", "third"}, got)
	assert.Equal(t, 3, sb.Len())
}

func TestScrollbackPartialLine(t *testing.T) {
	sb := newScrollback(10)
	_, _ = sb.Write([]byte("hello "))
	_, _ = sb.Write([]byte("world\n"))
	assert.Equal(t, []string{"hello world"}, sb.Snapshot())
}

func TestScrollbackInProgressNotInSnapshot(t *testing.T) {
	sb := newScrollback(10)
	_, _ = sb.Write([]byte("committed\npartial without newline"))
	assert.Equal(t, []string{"committed"}, sb.Snapshot())
}

func TestScrollbackCarriageReturn(t *testing.T) {
	sb := newScrollback(10)
	// \r resets the partial — useful for shell prompts that redraw.
	_, _ = sb.Write([]byte("loading...\rdone\n"))
	assert.Equal(t, []string{"done"}, sb.Snapshot())
}

func TestScrollbackCRLFCommitsLine(t *testing.T) {
	sb := newScrollback(10)
	// PTYs with ONLCR (the default) emit \r\n for line endings. The CR
	// must NOT reset the partial line before the LF commits it — that
	// was the original "scrolled-back-shows-blank" bug.
	_, _ = sb.Write([]byte("foo\r\nbar\r\nbaz\r\n"))
	assert.Equal(t, []string{"foo", "bar", "baz"}, sb.Snapshot())
}

func TestScrollbackCRSpanningWriteBoundary(t *testing.T) {
	sb := newScrollback(10)
	// Stream chunked at the CR/LF seam — the pending CR must survive
	// the Write boundary so the next \n still commits the line.
	_, _ = sb.Write([]byte("hello\r"))
	_, _ = sb.Write([]byte("\nworld\r\n"))
	assert.Equal(t, []string{"hello", "world"}, sb.Snapshot())
}

func TestScrollbackDoubleCRLFCommitsLine(t *testing.T) {
	sb := newScrollback(10)
	// kubectl exec passes a pod's already-ONLCR-translated "\r\n" through
	// lfk's local PTY which applies ONLCR a second time, producing
	// "\r\r\n" on the wire. Each commit must still be the actual line
	// content — this was the "every line shows [0]" bug.
	_, _ = sb.Write([]byte("$ ls\r\r\nfile1\r\r\nfile2\r\r\n"))
	assert.Equal(t, []string{"$ ls", "file1", "file2"}, sb.Snapshot())
}

func TestScrollbackArbitraryCRRunCommitsLine(t *testing.T) {
	sb := newScrollback(10)
	// Even longer runs of CRs (rare but possible with multiple PTY
	// hops) must still treat the run as a line terminator.
	_, _ = sb.Write([]byte("triple\r\r\r\nquad\r\r\r\r\n"))
	assert.Equal(t, []string{"triple", "quad"}, sb.Snapshot())
}

func TestScrollbackDoubleCRWithoutLFOverprints(t *testing.T) {
	sb := newScrollback(10)
	// A run of CRs that ends with a non-\n byte still resets cur so
	// real prompt redraws keep working: "loading...\r\rdone\n" should
	// not preserve "loading...".
	_, _ = sb.Write([]byte("loading...\r\rdone\n"))
	assert.Equal(t, []string{"done"}, sb.Snapshot())
}

func TestScrollbackLoneCROverprint(t *testing.T) {
	sb := newScrollback(10)
	// CR followed by anything other than LF means overprint — reset the
	// partial buffer, then the next byte starts a fresh line.
	_, _ = sb.Write([]byte("downloading...\rcomplete\n"))
	assert.Equal(t, []string{"complete"}, sb.Snapshot())
}

func TestScrollbackBackspace(t *testing.T) {
	sb := newScrollback(10)
	_, _ = sb.Write([]byte("abc\b\bd\n"))
	assert.Equal(t, []string{"ad"}, sb.Snapshot())
}

func TestScrollbackBackspaceUTF8Safe(t *testing.T) {
	sb := newScrollback(10)
	// "ünicode" — ü is two bytes; backspace should remove the whole rune.
	_, _ = sb.Write([]byte("ü\bA\n"))
	assert.Equal(t, []string{"A"}, sb.Snapshot())
}

func TestScrollbackANSIStripped(t *testing.T) {
	sb := newScrollback(10)
	// CSI sequence (color), OSC sequence (window title), and a single-byte ESC.
	_, _ = sb.Write([]byte("\x1b[31mred\x1b[0m\n"))
	_, _ = sb.Write([]byte("\x1b]0;title\x07plain\n"))
	got := sb.Snapshot()
	assert.Equal(t, []string{"red", "plain"}, got)
}

func TestScrollbackRingOverflow(t *testing.T) {
	sb := newScrollback(3)
	for i := range 5 {
		_, _ = sb.Write([]byte{'l', byte('0' + i), '\n'})
	}
	got := sb.Snapshot()
	// Only the last 3 lines are retained, oldest-first.
	assert.Equal(t, []string{"l2", "l3", "l4"}, got)
	assert.Equal(t, 3, sb.Len())
}

func TestScrollbackRingExactCapacity(t *testing.T) {
	sb := newScrollback(3)
	_, _ = sb.Write([]byte("a\nb\nc\n"))
	assert.Equal(t, []string{"a", "b", "c"}, sb.Snapshot())
}

func TestScrollbackNilSafe(t *testing.T) {
	var sb *scrollback
	n, err := sb.Write([]byte("x"))
	assert.Equal(t, 1, n)
	require.NoError(t, err)
	assert.Nil(t, sb.Snapshot())
	assert.Equal(t, 0, sb.Len())
}

func TestStripAnsiBytes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain text", "hello", "hello"},
		{"CSI color", "\x1b[31mred\x1b[0m", "red"},
		{"CSI cursor", "abc\x1b[2Adef", "abcdef"},
		{"OSC title bel", "\x1b]0;title\x07after", "after"},
		{"OSC title ST", "\x1b]0;title\x1b\\after", "after"},
		{"two-byte ESC", "\x1bMfoo", "foo"},
		{"trailing escape (truncated)", "abc\x1b", "abc"},
		{"multiple sequences", "\x1b[1mbold\x1b[0m \x1b[4munderline\x1b[0m", "bold underline"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripAnsiBytes([]byte(tc.in))
			assert.Equal(t, tc.want, string(got))
		})
	}
}

func TestScrollbackSnapshotIsSafeCopy(t *testing.T) {
	sb := newScrollback(5)
	_, _ = sb.Write([]byte("a\nb\n"))
	snap := sb.Snapshot()
	// Mutate the snapshot.
	snap[0] = "MUTATED"
	// Verify the ring is unchanged.
	got := sb.Snapshot()
	assert.Equal(t, "a", got[0])
}

func TestRenderScrollbackView(t *testing.T) {
	sb := newScrollback(20)
	for _, s := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		_, _ = sb.Write([]byte(s + "\n"))
	}

	t.Run("offset 1 shows window ending one line before live", func(t *testing.T) {
		out := renderScrollbackView(sb, 1, 4, 80)
		stripped := stripANSI(out)
		assert.Contains(t, stripped, "d")
		assert.Contains(t, stripped, "g")
		assert.NotContains(t, stripped, "h", "h is the last line; offset 1 hides it")
	})

	t.Run("offset deeper than buffer returns empty (border padding fills the pane)", func(t *testing.T) {
		// Only 8 lines captured, viewH=4, offset=100 → end < 0 clamps to 0,
		// start clamps to 0, no lines shown. The border style at the
		// caller pads the box via lipgloss Height, so we don't pad here.
		out := renderScrollbackView(sb, 100, 4, 80)
		assert.Empty(t, out)
	})

	t.Run("truncates lines wider than viewW", func(t *testing.T) {
		sbWide := newScrollback(2)
		_, _ = sbWide.Write([]byte("0123456789ABCDEF\n"))
		out := renderScrollbackView(sbWide, 0, 1, 5)
		stripped := stripANSI(out)
		assert.Equal(t, "01234", stripped)
	})
}

func TestExecScrollBy(t *testing.T) {
	sb := newScrollback(200)
	for range 100 {
		_, _ = sb.Write([]byte{'x', '\n'})
	}
	// height=24, no tab bar -> viewH = 24-1-4 = 19 -> max usable
	// offset = 100 - 19 = 81.
	m := Model{height: 24, tabs: []TabState{{}}, execScrollback: sb}

	t.Run("negative delta scrolls back, clamped so a viewport stays visible", func(t *testing.T) {
		m := m
		m = m.execScrollBy(-10)
		assert.Equal(t, 10, m.execScrollOffset)
		m = m.execScrollBy(-1000) // beyond top
		assert.Equal(t, 81, m.execScrollOffset, "clamped to Len - viewH so oldest line stays at top of viewport, not past it")
	})

	t.Run("positive delta scrolls forward, clamped at 0 (live)", func(t *testing.T) {
		m := m
		m.execScrollOffset = 5
		m = m.execScrollBy(10)
		assert.Equal(t, 0, m.execScrollOffset, "clamped at 0")
	})

	t.Run("nil scrollback is a no-op", func(t *testing.T) {
		m := Model{height: 24, tabs: []TabState{{}}}
		m = m.execScrollBy(-100)
		assert.Equal(t, 0, m.execScrollOffset)
	})

	t.Run("scrollback smaller than viewport disables scrolling", func(t *testing.T) {
		small := newScrollback(50)
		for range 5 {
			_, _ = small.Write([]byte{'a', '\n'})
		}
		mm := Model{height: 24, tabs: []TabState{{}}, execScrollback: small}
		mm = mm.execScrollBy(-100)
		assert.Equal(t, 0, mm.execScrollOffset, "all lines already fit in viewport — nothing to scroll up to")
	})
}

func TestExecViewportRowsMatchesRender(t *testing.T) {
	// The scroll-clamp viewport calculation has to agree with
	// viewExecTerminal's viewH or the top of scrollback gets trimmed.
	cases := []struct {
		name   string
		height int
		tabs   int
		want   int
	}{
		{"24 rows, no tab bar", 24, 1, max(24-1-4, 3)},
		{"24 rows, with tab bar", 24, 2, max(24-1-1-4, 3)},
		{"50 rows, with tab bar", 50, 3, max(50-1-1-4, 3)},
		{"tiny terminal clamps to 3", 5, 1, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tabs := make([]TabState, tc.tabs)
			m := Model{height: tc.height, tabs: tabs}
			assert.Equal(t, tc.want, m.execViewportRows())
		})
	}
}

func TestExecScrollToTop(t *testing.T) {
	sb := newScrollback(200)
	for range 100 {
		_, _ = sb.Write([]byte{'a', '\n'})
	}
	// height=24, no tab bar -> viewH = 24-1-4 = 19 -> top means
	// oldest at top of viewport, i.e. offset = max(Len - viewH, 0) =
	// 100 - 19 = 81.
	m := Model{height: 24, tabs: []TabState{{}}, execScrollback: sb}
	m = m.execScrollToTop()
	assert.Equal(t, 81, m.execScrollOffset)
}

func TestScrollbackRealShellOutput(t *testing.T) {
	sb := newScrollback(50)
	// Mimic an actual bash session byte stream: OSC title, color CSI for
	// the prompt, command echo with CRLF, output with CRLF, prompt redraw.
	stream := []byte(
		"\x1b]0;user@host:~\x07" + // OSC title
			"\x1b[01;32muser@host\x1b[00m:\x1b[01;34m~\x1b[00m\\$ " + // colored prompt
			"ls\r\n" + // command + CRLF
			"\x1b[01;34mDocuments\x1b[00m  Downloads  README.md\r\n" + // output with one color
			"\x1b]0;user@host:~\x07" + // new OSC title for next prompt
			"\x1b[01;32muser@host\x1b[00m:\x1b[01;34m~\x1b[00m\\$ ", // new prompt
	)
	_, _ = sb.Write(stream)

	got := sb.Snapshot()
	require.Len(t, got, 2, "expected two committed lines after one ls round-trip")

	// First committed line: the prompt + the typed command.
	assert.Contains(t, got[0], "user@host:~")
	assert.Contains(t, got[0], "$ ls")
	// Second committed line: the ls output.
	assert.Contains(t, got[1], "Documents")
	assert.Contains(t, got[1], "Downloads")
	assert.Contains(t, got[1], "README.md")
	// Lines must not be empty/whitespace-only — that was the
	// "scrolled-back-shows-blank" symptom from CRLF mishandling.
	for i, line := range got {
		assert.NotEmpty(t, strings.TrimSpace(line), "line %d is whitespace-only: %q", i, line)
	}
}

func TestScrollbackHandlesLargeBatch(t *testing.T) {
	sb := newScrollback(100)
	var sbWrite strings.Builder
	for i := range 250 {
		sbWrite.WriteString("line ")
		sbWrite.WriteByte(byte('0' + (i % 10)))
		sbWrite.WriteByte('\n')
	}
	_, _ = sb.Write([]byte(sbWrite.String()))
	got := sb.Snapshot()
	require.Len(t, got, 100)
	// Last line should be "line 9" (i=249, 249%10=9).
	assert.Equal(t, "line 9", got[len(got)-1])
}
