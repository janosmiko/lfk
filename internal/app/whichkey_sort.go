package app

import (
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/janosmiko/lfk/internal/ui"
)

// wkNaturalDigits matches which-key.nvim's ("%09d") zero padding, which is what
// makes "2" sort ahead of "10" (view.lua:45-50).
const wkNaturalDigits = 9

// sortWhichKeyCells clusters entries by catalog group (whichKeyGroupOrder),
// then by modifier tier (USER DECISION: plain keys, then ctrl chords, then alt
// chords, then ctrl+alt chords), then by an opt-in explicit order
// (whichKeyAction.Order, mirroring neovim's `order` sorter), then by the rest
// of neovim's which-key chain: its configured sorters (config.lua `sort`)
// with `natural` and `case` appended (view.lua:77-93). The `local` sorter is
// dropped — it ranks buffer-local keymaps, which lfk has none of.
//
//  0. group    — declared catalog group (whichKeyGroupOrder), ungrouped last
//  1. modTier  — no modifier, then ctrl, then alt, then ctrl+alt, then any
//     other modifier combination (see wkModTier below)
//  2. order    — explicit per-entry override (whichKeyAction.Order); entries
//     that don't set one fall through unchanged
//  3. shape    — single-character alphanumerics, then single-character
//     punctuation, then multi-character named keys (see wkKeyShapeRank)
//  4. natural  — digit runs compared numerically, case-insensitively
//  5. case     — lowercase ahead of uppercase, so "d" precedes "D"
//
// The group pass is what turns the panel's per-group description color into a
// readable cue: without it the colors were scattered across the whole grid in
// whatever order the keys happened to sort to. No headers are drawn — cells
// still carry their group only to tint the description (whichKeyCellStyles).
//
// grouped == false is the leader key's second state (USER DECISION): pure key
// ordering. Passes 0 and 2 are BOTH dropped there, not just the group pass —
// `order` exists to hold "<" ">" "=" together WITHIN the Sort group, and with
// no groups left it would instead drag those three punctuation keys to the head
// of the whole panel, which is not a key ordering by any reading. The colors
// and the legend stay in both modes: order no longer says which category an
// entry belongs to, so the color is the only thing left that does.
//
// Ranks are derived once per cell rather than inside the comparator: the panel
// re-sorts on every render, and building the natural key per comparison would
// be O(n log n) string builds instead of O(n).
func sortWhichKeyCells(cells []whichKeyCell, grouped bool) {
	if len(cells) < 2 {
		return
	}
	var groupRank map[whichKeyGroup]int
	if grouped {
		groupRank = wkGroupRanks()
	}
	type ranked struct {
		cell                                whichKeyCell
		group, modTier, order, shape, upper int
		natural                             string
	}
	rs := make([]ranked, len(cells))
	for i, c := range cells {
		rs[i] = ranked{
			cell:    c,
			modTier: wkModTier(c.key),
			shape:   wkKeyShapeRank(c.key),
			upper:   wkCaseRank(c.key),
			natural: wkNaturalKey(c.key),
		}
		if grouped {
			rs[i].group = wkGroupRank(groupRank, c.group)
			rs[i].order = wkOrderRank(c.order)
		}
	}
	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		switch {
		case a.group != b.group:
			return a.group < b.group
		case a.modTier != b.modTier:
			return a.modTier < b.modTier
		case a.order != b.order:
			return a.order < b.order
		case a.shape != b.shape:
			return a.shape < b.shape
		case a.natural != b.natural:
			return a.natural < b.natural
		case a.upper != b.upper:
			return a.upper < b.upper
		default:
			return a.cell.key < b.cell.key
		}
	})
	for i, r := range rs {
		cells[i] = r.cell
	}
}

// wkGroupRanks maps each declared catalog group to its position in
// whichKeyGroupOrder, the cluster order sortWhichKeyCells sorts by before the
// within-group key sort takes over.
func wkGroupRanks() map[whichKeyGroup]int {
	order := whichKeyGroupOrder()
	ranks := make(map[whichKeyGroup]int, len(order))
	for i, g := range order {
		ranks[g] = i
	}
	return ranks
}

// wkGroupRank looks up a cell's cluster position. A cell with no group (or an
// unrecognised one — none exist today, but this keeps the mapping total)
// ranks after every declared group, so the g-prefix goto popup — whose cells
// are all ungrouped — sorts as one uniform block and keeps exactly the
// key-sort order it had before grouping was added.
func wkGroupRank(ranks map[whichKeyGroup]int, g whichKeyGroup) int {
	if r, ok := ranks[g]; ok {
		return r
	}
	return len(ranks)
}

// wkKeyShapeRank ranks a binding by the SHAPE of the key under its modifiers:
//
//	0  a single alphanumeric character   "d", "7", "ctrl+d"
//	1  any other single character        "?", "\\", "ctrl+/"
//	2  a multi-character named key       "f1", "space", "ctrl+space"
//	3  anything else                     a typo'd binding, "ctrl++"
//
// Ranks 0 and 1 are which-key.nvim's `alphanum` sorter (view.lua:36-38), which
// this replaces. That sorter matched the whole binding against ^[a-zA-Z0-9]+$,
// and a key NAME is letters and digits too — so "f1" and "space" ranked as
// plain keys and the natural sort then filed them under their first letter,
// drawing "f1" between "F" and "H" and "space" under "s". A name is a shape of
// its own (USER DECISION: it sorts after the single-character keys, which puts
// it at the end of its tier, next to the multi-character chords that follow).
//
// The shape is read from the key UNDER the modifiers, so the rule holds inside
// every tier — "ctrl+space" trails "ctrl+z" exactly as "space" trails "z".
// ui.IsNamedKey is the same table ui.IsSingleKeypress consults; the panel must
// not grow a second opinion on what a key name is.
//
// Rank 3 covers what is not a keypress shape at all: a binding
// SplitModifierChord refuses (the literal "+" chords) has no key to read, and a
// mistyped one ("ga") is not a key. Both sort last within their tier rather
// than being silently filed among the real names.
func wkKeyShapeRank(key string) int {
	base := key
	if _, last, ok := ui.SplitModifierChord(key); ok {
		base = last
	}
	switch {
	case len(base) == 1 && isWKAlphanum(base[0]):
		return 0
	case utf8.RuneCountInString(base) == 1:
		return 1
	case ui.IsNamedKey(base):
		return 2
	default:
		return 3
	}
}

// wkModTier is the USER-REQUESTED primary ordering within a group: plain keys
// first, then ctrl-only chords, then alt-only chords, then ctrl+alt chords
// together. Reuses ui.SplitModifierChord (extracted from the same parser
// helpKeyDisplay uses) rather than a second hand-rolled chord splitter.
//
// The user named exactly four tiers. Everything else that carries a modifier
// — shift alone, meta/super/cmd, hyper, or any combination that mixes one of
// those in (including ctrl+shift or shift+alt+ctrl) — has no requested slot,
// so it is treated simply as "modified, but not one of the four" and sorted
// into a fifth, catch-all tier after ctrl+alt. That keeps the rule flat and
// predictable instead of inventing a deeper hierarchy the user never asked
// for, and it still degrades safely: an exotic chord always sorts after every
// plain key and after every ctrl/alt/ctrl+alt chord, never in between them.
//
// Named keys without a modifier ("esc", "tab", "f1", "space") are plain
// bindings to SplitModifierChord (no "+"), so they land in tier 0 with the
// letters — the same divergence from neovim's own `mod` sorter that the
// removed wkModRank used to document. Within that tier wkKeyShapeRank then
// files them after every single-character key.
func wkModTier(key string) int {
	mods, _, ok := ui.SplitModifierChord(key)
	if !ok {
		return 0
	}
	var hasCtrl, hasAlt, hasOther bool
	for _, m := range mods {
		switch m {
		case "ctrl":
			hasCtrl = true
		case "alt":
			hasAlt = true
		default:
			hasOther = true
		}
	}
	switch {
	case hasOther:
		return 4
	case hasCtrl && hasAlt:
		return 3
	case hasAlt:
		return 2
	default: // hasCtrl
		return 1
	}
}

// wkOrderRank turns whichKeyAction.Order into a comparable rank: the unset
// default (0) maps to the largest possible int so an entry that didn't opt in
// always sorts after every entry that did, then falls through to the next
// comparator on a tie — mirroring neovim's own `order` sorter, which defaults
// every item's order to 1000 (view.lua's M.fields.order) for the same reason.
func wkOrderRank(order int) int {
	if order == 0 {
		return math.MaxInt
	}
	return order
}

// wkCaseRank is which-key.nvim's `case` sorter (view.lua:42-44).
func wkCaseRank(key string) int {
	if key == strings.ToLower(key) {
		return 0
	}
	return 1
}

// wkNaturalKey is which-key.nvim's `natural` sorter (view.lua:45-50): every
// digit run is zero-padded to a fixed width so a plain string compare orders
// numbers the way a human reads them, and the result is lowercased.
func wkNaturalKey(key string) string {
	if !strings.ContainsAny(key, "0123456789") {
		return strings.ToLower(key)
	}
	var sb strings.Builder
	sb.Grow(len(key) + wkNaturalDigits)
	for i := 0; i < len(key); {
		if !isWKDigit(key[i]) {
			sb.WriteByte(key[i])
			i++
			continue
		}
		j := i
		for j < len(key) && isWKDigit(key[j]) {
			j++
		}
		run := strings.TrimLeft(key[i:j], "0")
		if run == "" {
			run = "0"
		}
		for p := len(run); p < wkNaturalDigits; p++ {
			sb.WriteByte('0')
		}
		sb.WriteString(run)
		i = j
	}
	return strings.ToLower(sb.String())
}

func isWKDigit(c byte) bool { return c >= '0' && c <= '9' }

func isWKAlphanum(c byte) bool {
	return isWKDigit(c) || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
