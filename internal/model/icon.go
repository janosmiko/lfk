package model

// Icon carries the glyph variants for each IconMode. Every built-in resource
// declares its full set of variants; resolveIcon picks the right field based
// on the active mode. Keeping all variants co-located with the resource makes
// the translation maps used in prior versions unnecessary and prevents
// key-by-glyph drift when glyphs are updated.
type Icon struct {
	Unicode  string // canonical glyph, e.g. "□"
	Simple   string // ASCII label, e.g. "[Po]"
	Emoji    string // e.g. "🔵"
	NerdFont string // single-char NF/MDI codepoint, e.g. "\U000f01a7" (nf-md-cube_outline)
}

// IsEmpty reports whether no variants are set. Used by code paths that want
// to skip the icon prefix entirely (e.g. virtual entries with no decoration).
func (i Icon) IsEmpty() bool {
	return i.Unicode == "" && i.Simple == "" && i.Emoji == "" && i.NerdFont == ""
}
