package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

// --- RenderHelpScreen ---

func TestRenderHelpScreen_DefaultState(t *testing.T) {
	// No filter: should contain keybindings content.
	result := RenderHelpScreen(80, 40, 0, "", "", "", -1)
	assert.Contains(t, result, "Keybindings")
}

func TestRenderHelpScreen_FilterApplied(t *testing.T) {
	// Filter applied: content should contain matching entries.
	result := RenderHelpScreen(80, 40, 0, "nav", "", "", -1)
	assert.Contains(t, result, "nav")
}

func TestRenderHelpScreen_FilterFiltersEntries(t *testing.T) {
	// Filter excludes non-matching entries from the visible content.
	// Box height stays constant (covered by FilterDoesNotShrinkBox);
	// what changes is which lines render.
	filtered := RenderHelpScreen(120, 100, 0, "bookmark", "", "", -1)
	assert.Contains(t, filtered, "Bookmark",
		"filter must keep matching sections visible")
	// A keybinding far from the bookmark section shouldn't appear in
	// the visible window after filtering.
	assert.NotContains(t, filtered, "Toggle help screen",
		"filter must hide non-matching entries")
}

// Filtering down to a tiny match set must not shrink the overlay
// box — the row count must match the unfiltered render so the user
// doesn't see the window collapse on each keystroke.
func TestRenderHelpScreen_FilterDoesNotShrinkBox(t *testing.T) {
	full := RenderHelpScreen(120, 100, 0, "", "", "", -1)
	narrowed := RenderHelpScreen(120, 100, 0, "thereisnokeycontainingthisstring", "", "", -1)
	fullLines := strings.Split(full, "\n")
	narrowedLines := strings.Split(narrowed, "\n")
	assert.Equal(t, len(fullLines), len(narrowedLines),
		"filter that narrows results must not shrink the box height")
}

func TestRenderHelpScreen_SearchHighlightsButDoesNotFilter(t *testing.T) {
	// Search differs from filter: matching content stays inline; non-matching
	// lines are NOT removed. The user opens search to find a key in
	// context, not to whittle the list down. Using a tall enough viewport
	// so the bookmark section is in the visible window for a meaningful
	// highlight check.
	full := RenderHelpScreen(120, 200, 0, "", "", "", -1)
	searched := RenderHelpScreen(120, 200, 0, "", "Bookmark", "", -1)

	fullLines := strings.Split(full, "\n")
	searchedLines := strings.Split(searched, "\n")
	assert.Equal(t, len(fullLines), len(searchedLines),
		"search must not remove lines — line count must match the unfiltered render")
}

// Current-match line gets a distinct style so the user can see which
// match the next n/N press will move from. Probe across line indices
// to find one that contains the search query (so the swap from
// SearchHighlightStyle → SelectedSearchHighlightStyle on that line
// produces visibly different output).
func TestRenderHelpScreen_CurrentMatchStyledDifferently(t *testing.T) {
	// Tests run without a TTY, so termenv defaults to a stripped color
	// profile and lipgloss drops the foreground/decoration codes that
	// distinguish the two highlight styles — making them render
	// identically. Force the renderer to ANSI mode and re-apply the
	// theme so SelectedSearchHighlightStyle picks up its color codes.
	// Other tests in this package toggle ConfigNoColor; we have to
	// re-apply state defensively at start because Go test ordering
	// inside a package isn't guaranteed and a prior test may have left
	// styles blank.
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigNoColor = originalNoColor
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	ApplyTheme(DefaultTheme())
	// ApplyTheme can re-detect/restore the color profile from
	// originalColorProfile, so re-force ANSI right after.

	withoutCurrent := RenderHelpScreen(120, 200, 0, "", "filter", "", -1)

	// Find a line index where flipping currentMatchLine actually changes
	// the render — i.e. the line contains a "filter" match. The exact
	// index depends on help content ordering; we just need any one.
	totalLines := len(BuildHelpLines("", "", 120))
	for i := range totalLines {
		withCurrent := RenderHelpScreen(120, 200, 0, "", "filter", "", i)
		if withoutCurrent != withCurrent {
			return // found a difference — contract holds
		}
	}
	t.Fatalf("no line index produced a different render — current-match style is not applied")
}

// Searching for a digit must not corrupt the rendered output.
//
// The previous "/" search path ran HighlightMatchStyled on the
// already-styled, already-truncated help lines. SGR sequences carry
// digits as parameters (e.g. \x1b[33;1m for bold + yellow fg), so a
// byte-indexed search for "1" matched bytes inside the escape
// sequences and the highlight wrapper split them into fragments
// terminals rendered as literal "[33;" / ";1m" text on screen.
//
// Asserts: stripping ANSI from the rendered output produces the same
// visible characters whether the user typed a digit query or no
// query at all. Search adds highlight color but never visible chars.
func TestRenderHelpScreen_DigitSearchDoesNotLeakEscapeFragments(t *testing.T) {
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigNoColor = originalNoColor
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	ApplyTheme(DefaultTheme())
	// ApplyTheme can re-detect/restore the color profile from
	// originalColorProfile (theme.go:109-110), so re-force TrueColor
	// here. Without this, the test runs under the harness's default
	// stripped profile, lipgloss emits no SGR digits, and the
	// regression we're guarding against — digit-query corruption
	// inside SGR sequences — could not occur in the first place.

	plain := RenderHelpScreen(120, 200, 0, "", "", "", -1)

	// Each digit query exercises a different byte that the old code
	// could match inside SGR parameters. "1" was the most visible in
	// the user report; the others guard against regressions in the
	// other common SGR digits.
	for _, q := range []string{"1", "0", "5", "33"} {
		t.Run("query="+q, func(t *testing.T) {
			searched := RenderHelpScreen(120, 200, 0, "", q, "", -1)

			assert.Equal(t, ansi.Strip(plain), ansi.Strip(searched),
				"digit search must not change the visible characters in the help screen — only the highlight color")

			assert.NotContains(t, searched, "\x1b\x1b",
				"rendered output must not contain doubled-ESC bytes (smoking gun for fragmented SGR sequences)")
		})
	}
}

// helpRecomputeMatches in the app layer iterates over BuildHelpLines
// and counts hits via MatchLine. When BuildHelpLines returned styled
// strings, a "1" query would substring-match bytes inside SGR codes
// (e.g. the "1" in \x1b[33;1m) and inflate the match count to roughly
// every styled row, pointing n/N at rows with no visible "1".
//
// Asserts: the plain lines BuildHelpLines now returns contain no ESC
// bytes, so MatchLine sees only the visible characters.
//
// Forces a TrueColor profile so lipgloss actually emits SGR escapes
// for any styling it would apply — without this, the test harness's
// default stripped profile makes lipgloss render plain text anyway
// and the assertion would pass vacuously even if BuildHelpLines went
// back to returning styled output.
// Within a single help section, every entry row's key column must have
// the same display width so descriptions align in a clean column.
// Regression: previously the column was a fixed 14 chars, so any key
// longer than that (e.g. "Ctrl+F / Ctrl+B / PgDn / PgUp") overflowed
// and pushed its description right of the rest, breaking alignment.
func TestBuildHelpSpecs_KeysAlignWithinSection(t *testing.T) {
	specs := buildHelpSpecs("", "", helpInnerWidth(120))

	var currentSection string
	widths := make([]int, 0, 16)

	check := func() {
		if len(widths) <= 1 {
			return
		}
		first := widths[0]
		for _, w := range widths {
			if w != first {
				t.Errorf("section %q: key widths inconsistent (got %v, want all %d)",
					currentSection, widths, first)
				return
			}
		}
	}

	for _, s := range specs {
		switch s.kind {
		case helpLineSectionHeader:
			check()
			currentSection = s.text
			widths = widths[:0]
		case helpLineEntry:
			widths = append(widths, lipgloss.Width(s.key))
		}
	}
	check()
}

// Word-wrap (issue #319 a): long descriptions must wrap onto continuation
// rows so they read in full instead of being truncated with a "~". Every
// entry row's plain width must fit innerW so RenderHelpScreen's final
// Truncate never trims it.
func TestBuildHelpSpecs_WrappedEntriesFitWidth(t *testing.T) {
	innerW := 70
	specs := buildHelpSpecs("", "", innerW)
	for _, s := range specs {
		if s.kind != helpLineEntry {
			continue
		}
		w := lipgloss.Width(helpSpecPlain(s))
		assert.LessOrEqualf(t, w, innerW,
			"entry row must fit innerW (no truncation): %q (width %d)", helpSpecPlain(s), w)
	}
}

// A narrower help width wraps long descriptions onto more rows than a wide
// one — proving wrapping actually happens (not just truncation).
func TestBuildHelpSpecs_NarrowWidthWrapsToMoreRows(t *testing.T) {
	wide := buildHelpSpecs("", "", 200)
	narrow := buildHelpSpecs("", "", 60)
	assert.Greater(t, len(narrow), len(wide),
		"narrow width must wrap long descriptions onto additional rows")
}

// Continuation rows carry a blank key column (the key shows only on the
// first row of a wrapped entry) but keep the section's key-column width so
// the wrapped description stays aligned under the original.
func TestBuildHelpSpecs_ContinuationRowsHaveBlankAlignedKey(t *testing.T) {
	specs := buildHelpSpecs("", "", 50) // narrow -> forces wrapping
	foundContinuation := false
	for _, s := range specs {
		if s.kind != helpLineEntry {
			continue
		}
		if strings.TrimSpace(s.key) == "" && s.desc != "" {
			foundContinuation = true
			break
		}
	}
	assert.True(t, foundContinuation,
		"narrow width must produce at least one continuation row with a blank key")
}

// A long slash-joined token must wrap at "/" boundaries rather than being
// hard-broken mid-word (issue #319 a follow-up: "...findi" / "ng/mark...").
func TestWrapHelpText_BreaksLongSlashTokenAtSlashes(t *testing.T) {
	desc := "Jump back through teleport history (owner/port-forward/orphan/finding/mark jumps)"
	for _, width := range []int{20, 30, 41, 60} {
		lines := wrapHelpText(desc, width)

		for _, ln := range lines {
			assert.LessOrEqualf(t, lipgloss.Width(ln), width,
				"width %d: row %q exceeds the budget", width, ln)
		}

		// No data loss: ignoring whitespace, the wrapped text reproduces the
		// original exactly (slash continuations glue with no space).
		norm := func(s string) string { return strings.ReplaceAll(s, " ", "") }
		assert.Equalf(t, norm(desc), norm(strings.Join(lines, "")),
			"width %d: wrapping dropped or duplicated characters", width)

		// At widths that comfortably fit a slash segment, segments stay whole.
		if width >= 30 {
			joined := strings.Join(lines, "\n")
			assert.Containsf(t, joined, "finding/",
				"width %d: slash segment 'finding/' must not be broken mid-word", width)
		}
	}
}

func TestBuildHelpLines_ReturnsPlainText(t *testing.T) {
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigNoColor = originalNoColor
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	ApplyTheme(DefaultTheme())

	lines := BuildHelpLines("", "", 120)
	for i, line := range lines {
		assert.NotContains(t, line, "\x1b",
			"BuildHelpLines must return plain text (no ANSI escapes) — line %d: %q", i, line)
	}
}

// --- one hotkey per line ---

// The help screen renders one binding per row, so a catalog entry with an
// empty key would draw as a prose-only line — exactly the wall of text the
// screen was reworked to remove. Long explanations live in
// docs/keybindings.md instead.
func TestHelpSections_EveryEntryHasAKey(t *testing.T) {
	for _, section := range helpSections() {
		for i, b := range section.bindings {
			assert.NotEmptyf(t, strings.TrimSpace(b.key),
				"section %q entry %d (%q) has no key — prose-only rows are not allowed",
				section.title, i, b.desc)
		}
	}
}

// --- right-aligned key column ---

// Every entry row shares one key-column width, and the key sits flush
// against its right edge, so all descriptions start at the same column.
func TestBuildHelpSpecs_KeyColumnIsGloballyRightAligned(t *testing.T) {
	specs := buildHelpSpecs("", "", helpInnerWidth(160))
	width := -1
	sawEntry := false
	for _, s := range specs {
		if s.kind != helpLineEntry {
			continue
		}
		sawEntry = true
		w := lipgloss.Width(s.key)
		if width < 0 {
			width = w
		}
		assert.Equalf(t, width, w,
			"key column width must be identical on every row: %q", s.key)
		if s.keyText == "" {
			continue // wrapped continuation row: key cell is intentionally blank
		}
		assert.Falsef(t, strings.HasSuffix(s.key, " "),
			"key must be right-aligned (no trailing pad): %q", s.key)
	}
	assert.True(t, sawEntry, "expected at least one entry row")
}

// The description column therefore starts at one fixed offset on every row.
func TestHelpSpecPlain_DescriptionsStartAtOneColumn(t *testing.T) {
	specs := buildHelpSpecs("", "", helpInnerWidth(160))
	offsets := make(map[int]struct{})
	for _, s := range specs {
		if s.kind != helpLineEntry || s.desc == "" {
			continue
		}
		line := helpSpecPlain(s)
		offsets[lipgloss.Width(line)-lipgloss.Width(s.desc)] = struct{}{}
	}
	assert.Len(t, offsets, 1, "descriptions must all start at the same column, got offsets %v", offsets)
}

// --- symbol keys vs. the search index ---

// The key column draws "⌃D" while the search index keeps "Ctrl+D", so a
// user typing "ctrl" still finds the row. This is the whole reason
// helpLineSpec carries both forms.
func TestBuildHelpLines_SearchIndexKeepsTextualChord(t *testing.T) {
	originalIcons := IconMode
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		IconMode = originalIcons
		ConfigNoColor = originalNoColor
	})
	IconMode = "unicode"
	ConfigNoColor = false

	joined := strings.Join(BuildHelpLines("", "", 160), "\n")
	assert.Contains(t, joined, "Ctrl+D",
		"the search index must carry the textual chord so a 'ctrl' query matches")
	assert.NotContains(t, joined, "⌃",
		"the search index must not carry the symbol form")

	rendered := ansi.Strip(RenderHelpScreen(160, 200, 0, "", "", "", -1))
	assert.Contains(t, rendered, "⌃D",
		"the key column must draw the symbol form")

	// The two paths must stay row-for-row aligned or n/N jumps to the
	// wrong line.
	assert.Len(t, BuildHelpLines("", "", 160), len(buildHelpSpecs("", "", helpInnerWidth(160))))
}

// A "ctrl" query must select the rows whose key column draws a symbol
// chord — the search index is what MatchLine sees.
func TestBuildHelpLines_CtrlQueryMatchesSymbolRenderedRows(t *testing.T) {
	originalIcons := IconMode
	t.Cleanup(func() { IconMode = originalIcons })
	IconMode = "unicode"

	var matched []string
	for _, line := range BuildHelpLines("", "", 160) {
		if MatchLine(line, "ctrl") {
			matched = append(matched, line)
		}
	}
	assert.NotEmpty(t, matched, `a "ctrl" search must match rows drawn as symbol chords`)
	assert.Contains(t, strings.Join(matched, "\n"), "Ctrl+D")
}

// The f filter runs over the same textual form, so filtering by "ctrl"
// narrows to the chord rows instead of returning "No matching keybindings".
func TestBuildHelpSpecs_FilterMatchesTextualChord(t *testing.T) {
	originalIcons := IconMode
	t.Cleanup(func() { IconMode = originalIcons })
	IconMode = "unicode"

	specs := buildHelpSpecs("ctrl", "", helpInnerWidth(160))
	assert.NotEmpty(t, specs)
	for _, s := range specs {
		assert.NotEqual(t, helpLineMessage, s.kind, "filter must find the chord rows")
	}
	found := false
	for _, s := range specs {
		if s.kind == helpLineEntry && strings.Contains(s.keyText, "Ctrl") {
			found = true
			break
		}
	}
	assert.True(t, found, `filtering by "ctrl" must keep symbol-rendered chord rows`)
}

// A row whose drawn key shares no characters with the query still shows
// the hit: the whole key cell is highlighted instead of a substring.
func TestRenderHelpScreen_SymbolChordRowShowsSearchHighlight(t *testing.T) {
	originalNoColor := ConfigNoColor
	originalIcons := IconMode
	t.Cleanup(func() {
		ConfigNoColor = originalNoColor
		IconMode = originalIcons
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	IconMode = "unicode"
	ApplyTheme(DefaultTheme())

	plain := RenderHelpScreen(160, 200, 0, "", "", "", -1)
	searched := RenderHelpScreen(160, 200, 0, "", "ctrl", "", -1)

	assert.NotEqual(t, plain, searched,
		`a "ctrl" search must visibly highlight the symbol-rendered chord rows`)
	assert.Equal(t, ansi.Strip(plain), ansi.Strip(searched),
		"the highlight must add color only, never visible characters")
}

// --- chord tokenizer ---

// Catalog keys are not always one binding. Formatting has to work
// token-wise so composites keep their exact separators.
func TestHelpKeyDisplay_CompositeKeys(t *testing.T) {
	tests := []struct{ given, want string }{
		{"ctrl+d/ctrl+u", "Ctrl+D/Ctrl+U"},
		{"ctrl+] ctrl+u/ctrl+d", "Ctrl+] Ctrl+U/Ctrl+D"},
		// A bare key is never a chord, so it keeps its verbatim spelling
		// (same contract as helpKeyDisplay("space") == "space").
		{"tab/shift+tab", "tab/Shift+Tab"},
		{"m<a-z/0-9>", "m<a-z/0-9>"},
		{"123<motion>", "123<motion>"},
		{"Click ns badge", "Click ns badge"},
		{"space/Right", "space/Right"},
	}
	for _, tt := range tests {
		t.Run(tt.given, func(t *testing.T) {
			assert.Equal(t, tt.want, helpKeyDisplay(tt.given))
		})
	}
}

// helpKeySymbols draws " · " between alternative bindings instead of the
// raw "/" (spaced and dimmed at render time — see styleHelpKeyCell), so
// this test's expectations moved from "/" to " · " for every case where
// "/" joins two real alternatives. "m<a-z/0-9>" is the deliberate
// exception: its "/" is inside "<...>", part of one placeholder token
// (any letter or digit after m), not a separator — it must stay literal.
func TestHelpKeySymbols_ByIconMode(t *testing.T) {
	originalIcons := IconMode
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		IconMode = originalIcons
		ConfigNoColor = originalNoColor
	})

	ConfigNoColor = false
	IconMode = "unicode"
	assert.Equal(t, "⌃D · ⌃U", helpKeySymbols("ctrl+d/ctrl+u"))
	assert.Equal(t, "⇥ · ⇧⇥", helpKeySymbols("tab/shift+tab"))
	assert.Equal(t, "m<a-z/0-9>", helpKeySymbols("m<a-z/0-9>"))
	assert.Equal(t, "h · ←", helpKeySymbols("h/Left"))

	// Terminals that promise only ASCII keep the readable textual chord,
	// dotted the same way.
	IconMode = "none"
	assert.Equal(t, "Ctrl+D · Ctrl+U", helpKeySymbols("ctrl+d/ctrl+u"))
	IconMode = "unicode"
	ConfigNoColor = true
	assert.Equal(t, "Ctrl+D · Ctrl+U", helpKeySymbols("ctrl+d/ctrl+u"))
}

// helpKeySymbols must apply the same bare-named-key substitution
// KeyChordDisplay uses for the which-key panel, including inside a
// slash-joined alternative list ("h/Backspace/Left" -> "h · <backspace> ·
// <left>") — the panel and the help screen must never disagree on how a
// named key draws.
func TestHelpKeySymbols_NamedKeyIcons(t *testing.T) {
	originalIcons := IconMode
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		IconMode = originalIcons
		ConfigNoColor = originalNoColor
	})
	ConfigNoColor = false

	tests := []struct {
		icons, given, want string
	}{
		{"nerdfont", "tab", "\U000F0312"},
		{"nerdfont", "space", "\U000F1050"},
		{"nerdfont", "backspace", "\U000F006E"},
		{"nerdfont", "Left", "\U000F004D"}, // catalog labels are capitalized
		{"nerdfont", "Right", "\U000F0054"},
		{"nerdfont", "Up", "\U000F005D"},
		{"nerdfont", "Down", "\U000F0045"},
		{"nerdfont", "h/Backspace/Left", "h · \U000F006E · \U000F004D"},
		{"nerdfont", "space/Right", "\U000F1050 · \U000F0054"},
		{"unicode", "tab", "⇥"},
		{"unicode", "space", "␣"},
		{"unicode", "backspace", "⌫"},
		{"unicode", "Left", "←"},
		{"unicode", "Right", "→"},
		{"unicode", "Up", "↑"},
		{"unicode", "Down", "↓"},
		{"unicode", "h/Backspace/Left", "h · ⌫ · ←"},
		{"unicode", "space/Right", "␣ · →"},
		// enter/esc keep the earlier, narrower decision: nerdfont's large
		// keycaps already resolved them (pre-existing glyphs), unicode's
		// small ⏎/⎋ never did and this change does not revisit that.
		{"nerdfont", "enter", "\U000F0311"},
		{"nerdfont", "esc", "\U000F12B7"},
		{"unicode", "enter", "enter"},
		{"unicode", "esc", "esc"},
	}
	for _, tt := range tests {
		t.Run(tt.icons+"/"+tt.given, func(t *testing.T) {
			IconMode = tt.icons
			assert.Equal(t, tt.want, helpKeySymbols(tt.given))
		})
	}
}

// The catalog's Mouse section describes mouse actions, not keybindings, and
// happens to use the words "left"/"right" for panes and a button ("Click
// left pane", "Right-click"). Those must never draw an arrow key icon: every
// genuine list of alternative bindings in this catalog joins with "/"
// ("h/Backspace/Left"), never a space or hyphen, so helpKeySymbols uses that
// as the signal to leave prose alone. Without this guard "Right-click"
// would render as "→-click", telling the user to press an arrow key for a
// mouse action.
func TestHelpKeySymbols_MouseProseStaysTextual(t *testing.T) {
	originalIcons := IconMode
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		IconMode = originalIcons
		ConfigNoColor = originalNoColor
	})
	ConfigNoColor = false

	prose := []string{"Right-click", "Click left pane", "Click right pane"}
	for _, icons := range []string{"nerdfont", "unicode"} {
		IconMode = icons
		for _, key := range prose {
			t.Run(icons+"/"+key, func(t *testing.T) {
				assert.Equal(t, key, helpKeySymbols(key),
					"a mouse-action label must never be read as a keyboard chord")
			})
		}
	}
}

// A search for a named key's word must still find its row even though the
// key column now draws an icon: the search index (keyText) stays textual
// while only the rendered key column (key) draws the symbol.
func TestBuildHelpLines_NamedKeyQueryMatchesIconRenderedRows(t *testing.T) {
	originalIcons := IconMode
	t.Cleanup(func() { IconMode = originalIcons })

	for _, icons := range []string{"nerdfont", "unicode"} {
		IconMode = icons
		for _, query := range []string{"backspace", "tab", "left"} {
			t.Run(icons+"/"+query, func(t *testing.T) {
				var matched []string
				for _, line := range BuildHelpLines("", "", 160) {
					if MatchLine(line, query) {
						matched = append(matched, line)
					}
				}
				assert.NotEmptyf(t, matched, "a %q search must still find its row in %s mode", query, icons)
			})
		}
	}
}

// The f filter runs over the same textual search index, so filtering by a
// named key's word must narrow to its row instead of "No matching
// keybindings" — mirroring TestBuildHelpSpecs_FilterMatchesTextualChord for
// the new icon-drawn keys.
func TestBuildHelpSpecs_FilterMatchesNamedKeyWord(t *testing.T) {
	originalIcons := IconMode
	t.Cleanup(func() { IconMode = originalIcons })
	IconMode = "nerdfont"

	specs := buildHelpSpecs("backspace", "", helpInnerWidth(160))
	assert.NotEmpty(t, specs)
	found := false
	for _, s := range specs {
		if s.kind == helpLineEntry && strings.Contains(strings.ToLower(s.keyText), "backspace") {
			found = true
			break
		}
	}
	assert.True(t, found, `filtering by "backspace" must keep the icon-rendered row`)
}

// Icons mostly narrow the key column ("backspace" 9 cols -> 1). The column
// is sized from lipgloss.Width and shared across every section (see
// helpKeyColumnWidth), so this checks every entry's key column against the
// real computed width at the icon modes and terminal sizes this change
// touches — a byte-counted width would have passed for the old ASCII words
// but silently misaligned once "Backspace" (9 cells) became one glyph.
//
// A row is allowed to exceed keyWidth (never to fall short of it): that is
// helpKeyColumnWidth's documented cap-overflow case (a key past innerW/3
// overflows its own cell rather than shrinking every description), already
// exercised by "Click middle pane" — 17 cells wide, uncapped — at width 80
// regardless of icon mode, since neither word is a named key.
func TestBuildHelpSpecs_KeysAlignWithinSection_IconModesAndSizes(t *testing.T) {
	originalIcons := IconMode
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		IconMode = originalIcons
		ConfigNoColor = originalNoColor
	})
	ConfigNoColor = false

	for _, icons := range []string{"nerdfont", "unicode"} {
		for _, screenWidth := range []int{80, 160} {
			t.Run(icons+"/", func(t *testing.T) {
				IconMode = icons
				innerW := helpInnerWidth(screenWidth)
				groups := collectHelpGroups("", "")
				keyWidth := helpKeyColumnWidth(groups, innerW)
				specs := buildHelpSpecs("", "", innerW)

				exact := 0
				for _, s := range specs {
					if s.kind != helpLineEntry {
						continue
					}
					w := lipgloss.Width(s.key)
					assert.GreaterOrEqualf(t, w, keyWidth,
						"icons=%s width=%d: key %q (width %d) padded shorter than the column width %d",
						icons, screenWidth, s.key, w, keyWidth)
					if w == keyWidth {
						exact++
					}
				}
				assert.Positive(t, exact,
					"the column-width padding path must be exercised by at least one row")
			})
		}
	}
}

// --- separator dots ---

// applyKeySeparatorDots must convert a "/" only where it genuinely
// separates two alternative bindings, and leave it alone everywhere else.
// This table enumerates every shape of "/" found in the live help catalog
// (help_sections.go) plus the bracket-guarded placeholder case, so a future
// catalog entry that adds a new shape has a home to extend.
func TestApplyKeySeparatorDots_Enumeration(t *testing.T) {
	tests := []struct {
		name, given, want string
	}{
		{"bare Search key: nothing on either side, not a separator", "/", "/"},
		{"two full alternatives", "h/Left", "h · Left"},
		{"doubled-chord alternative", "gg/Home", "gg · Home"},
		{"three alternatives", "0/1/2", "0 · 1 · 2"},
		{"two single-char alternatives", ">/<", "> · <"},
		{"two chord alternatives (post chord-mapping)", "⌃D/⌃U", "⌃D · ⌃U"},
		{"bracket-guarded range placeholder: leave alone", "m<a-z/0-9>", "m<a-z/0-9>"},
		{"unbracketed range list: three real alternatives", "a-z/A-Z/0-9", "a-z · A-Z · 0-9"},
		{"single key, no slash at all", "d", "d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, applyKeySeparatorDots(tt.given))
		})
	}
}

// The dotted separator is display-only: the search index (BuildHelpLines)
// must keep "/" so a user who searches for "/" itself, or for a substring
// straddling where "/" used to sit, is unaffected — and so the two forms
// stay easy to tell apart in the source.
func TestBuildHelpLines_SearchIndexKeepsSlashNotDot(t *testing.T) {
	originalIcons := IconMode
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		IconMode = originalIcons
		ConfigNoColor = originalNoColor
	})
	IconMode = "unicode"
	ConfigNoColor = false

	joined := strings.Join(BuildHelpLines("", "", 160), "\n")
	assert.Contains(t, joined, "Home", "search index must still carry the textual alternative")
	assert.NotContains(t, joined, "·", "search index must not carry the display-only dot")

	rendered := ansi.Strip(RenderHelpScreen(160, 200, 0, "", "", "", -1))
	assert.Contains(t, rendered, "gg · Home", "the rendered key column must draw the dotted separator")
}

// A user searching for one alternative among several ("Home" inside the
// row drawn as "gg · Home") must still find and highlight the row — the
// separator becoming a dot must not break the search a user already
// relies on to locate a key.
func TestRenderHelpScreen_SearchFindsRowWithDottedAlternativeKey(t *testing.T) {
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigNoColor = originalNoColor
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	ApplyTheme(DefaultTheme())

	lines := BuildHelpLines("", "", 160)
	found := false
	for _, l := range lines {
		if MatchLine(l, "Home") {
			found = true
			break
		}
	}
	assert.True(t, found, `search for "Home" must find the row rendered as "gg · Home"`)

	plain := RenderHelpScreen(160, 200, 0, "", "", "", -1)
	searched := RenderHelpScreen(160, 200, 0, "", "Home", "", -1)
	assert.NotEqual(t, plain, searched, `a "Home" search must visibly highlight the row`)
	assert.Contains(t, ansi.Strip(searched), "gg · Home",
		"search highlighting must not change the visible characters")
}

// The separator must draw dimmer than the keys around it — the whole
// point of the change — not merely spaced. Confirms the key segments and
// the separator segment open with different SGR codes.
func TestStyleHelpKeyCell_SeparatorStyledDifferentlyFromKeys(t *testing.T) {
	originalNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigNoColor = originalNoColor
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	ApplyTheme(DefaultTheme())

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true).Background(SurfaceBg)
	rendered := styleHelpKeyCell("h"+helpKeySeparator+"Left", "", SearchHighlightStyle, keyStyle)

	keyOpen := styleOpenCodes(keyStyle)
	sepOpen := styleOpenCodes(OverlayDimStyle)
	assert.NotEmpty(t, keyOpen)
	assert.NotEmpty(t, sepOpen)
	assert.NotEqual(t, keyOpen, sepOpen,
		"key style and separator style must emit different SGR codes")
	assert.Contains(t, rendered, keyOpen, "rendered cell must open with the key style")
	assert.Contains(t, rendered, sepOpen, "rendered cell must switch to the dim style for the separator")

	// A single-binding key (no separator) must render identically to the
	// pre-split path — no separator segment, no extra codes.
	single := styleHelpKeyCell("d", "", SearchHighlightStyle, keyStyle)
	assert.Equal(t, HighlightMatchStyledOver("d", "", SearchHighlightStyle, keyStyle), single)
}

// In no-color mode the separator has no distinct foreground, but it must
// still draw as literal " · " with spaces either side — the visual gap
// the user asked for does not depend on color being available.
func TestHelpKeySymbols_DottedSeparatorSurvivesNoColor(t *testing.T) {
	originalNoColor := ConfigNoColor
	originalIcons := IconMode
	t.Cleanup(func() {
		ConfigNoColor = originalNoColor
		IconMode = originalIcons
	})
	ConfigNoColor = true
	IconMode = "unicode"

	assert.Equal(t, "h · Left", helpKeySymbols("h/Left"))
}
