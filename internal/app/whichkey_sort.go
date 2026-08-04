package app

import (
	"sort"
	"strings"
)

// wkNaturalDigits matches which-key.nvim's ("%09d") zero padding, which is what
// makes "2" sort ahead of "10" (view.lua:45-50).
const wkNaturalDigits = 9

// sortWhichKeyCells clusters entries by catalog group (whichKeyGroupOrder),
// then orders each group's run the way neovim's which-key does: its
// configured sorters (config.lua `sort`) with `natural` and `case` appended
// (view.lua:77-93). The two neovim-only sorters are dropped — `local` ranks
// buffer-local keymaps, `order` is a manual override hook, and lfk has neither.
//
// 0. group    — declared catalog group (whichKeyGroupOrder), ungrouped last
// 1. alphanum — plain letter/digit keys ahead of everything else
// 2. mod      — modifier chords ahead of the remaining punctuation
// 3. natural  — digit runs compared numerically, case-insensitively
// 4. case     — lowercase ahead of uppercase, so "d" precedes "D"
//
// The group pass is what turns the panel's per-group description color into a
// readable cue: without it the colors were scattered across the whole grid in
// whatever order the keys happened to sort to. No headers are drawn — cells
// still carry their group only to tint the description (whichKeyCellStyles).
//
// Ranks are derived once per cell rather than inside the comparator: the panel
// re-sorts on every render, and building the natural key per comparison would
// be O(n log n) string builds instead of O(n).
func sortWhichKeyCells(cells []whichKeyCell) {
	if len(cells) < 2 {
		return
	}
	groupRank := wkGroupRanks()
	type ranked struct {
		cell                        whichKeyCell
		group, alphanum, mod, upper int
		natural                     string
	}
	rs := make([]ranked, len(cells))
	for i, c := range cells {
		rs[i] = ranked{
			cell:     c,
			group:    wkGroupRank(groupRank, c.group),
			alphanum: wkAlphanumRank(c.key),
			mod:      wkModRank(c.key),
			upper:    wkCaseRank(c.key),
			natural:  wkNaturalKey(c.key),
		}
	}
	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		switch {
		case a.group != b.group:
			return a.group < b.group
		case a.alphanum != b.alphanum:
			return a.alphanum < b.alphanum
		case a.mod != b.mod:
			return a.mod < b.mod
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

// wkAlphanumRank is which-key.nvim's `alphanum` sorter (view.lua:36-38).
func wkAlphanumRank(key string) int {
	if key == "" {
		return 1
	}
	for i := range len(key) {
		if !isWKAlphanum(key[i]) {
			return 1
		}
	}
	return 0
}

// wkModRank is which-key.nvim's `mod` sorter (view.lua:39-41). neovim spots a
// special key by its "<...>" notation; lfk spells chords as "ctrl+x" / "alt+y",
// so the "+" is the equivalent marker. Named keys without a modifier ("esc",
// "tab", "f1") are plain alphanumerics here and rank with the letters instead,
// which is the one place this ordering diverges from neovim's.
func wkModRank(key string) int {
	if strings.Contains(key, "+") {
		return 0
	}
	return 1
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
