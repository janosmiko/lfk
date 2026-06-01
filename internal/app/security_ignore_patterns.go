// Package app — security_ignore_patterns.go
// Matching for the declarative, glob-based security ignore rules defined in
// the config file (ui.SecurityIgnorePattern). These complement the
// interactive per-cluster ignore-list in security_ignores.go: config patterns
// are read-only (cannot be un-ignored from the UI), but the show-ignored
// toggle still reveals findings they hide. The interactive action menu reads
// the YAML state directly, so it never offers a misleading "Un-ignore" for a
// config-only ignore; only modelIgnoreChecker (used by the grouping/filter
// engine) consults both layers.
package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/ui"
)

// globMatch reports whether s matches a glob pattern supporting `*`
// (zero-or-more of any character) and `?` (exactly one character). The match
// is anchored to the whole string and is slash-agnostic (unlike path.Match,
// `*` spans `/`). An empty pattern or any all-`*` pattern matches anything —
// the "any" sentinel used by ignore patterns (consecutive stars are
// equivalent to one). Matching is case-sensitive and runs in linear time via
// the classic two-pointer backtracking algorithm.
func globMatch(pattern, s string) bool {
	if pattern == "" || strings.Trim(pattern, "*") == "" {
		return true
	}
	pr := []rune(pattern)
	sr := []rune(s)
	var px, sx int
	starPx, starSx := -1, 0
	for sx < len(sr) {
		switch {
		case px < len(pr) && (pr[px] == '?' || pr[px] == sr[sx]):
			px++
			sx++
		case px < len(pr) && pr[px] == '*':
			starPx = px // remember the star and the input position it began at
			starSx = sx
			px++
		case starPx >= 0:
			px = starPx + 1 // backtrack: let the last star absorb one more char
			starSx++
			sx = starSx
		default:
			return false
		}
	}
	for px < len(pr) && pr[px] == '*' {
		px++
	}
	return px == len(pr)
}

// patternIsEmpty reports whether a pattern has no constraints at all. Such a
// pattern would match every finding, so callers skip it defensively (the
// config loader also drops these at apply time). Delegates to the single
// authoritative definition on the config type.
func patternIsEmpty(p ui.SecurityIgnorePattern) bool {
	return p.IsEmpty()
}

// patternIgnoresResource reports whether any config pattern hides the finding
// identified by (ctx, source, groupKey, namespace). Every non-empty field of
// a pattern must match. namespace is the finding's namespace (empty for
// cluster-scoped findings).
func patternIgnoresResource(patterns []ui.SecurityIgnorePattern, ctx, source, groupKey, namespace string) bool {
	for _, p := range patterns {
		if patternIsEmpty(p) {
			continue
		}
		if globMatch(p.Cluster, ctx) &&
			globMatch(p.Source, source) &&
			globMatch(p.Group, groupKey) &&
			globMatch(p.Namespace, namespace) {
			return true
		}
	}
	return false
}

// patternIgnoresGroup reports whether any config pattern hides the entire
// group (ctx, source, groupKey) regardless of namespace. Only patterns whose
// namespace field is "any" (empty or `*`) qualify; namespace-scoped patterns
// hide individual resources, not the whole group, so they are excluded here
// (mirroring how a namespace-scoped interactive rule leaves the group visible).
func patternIgnoresGroup(patterns []ui.SecurityIgnorePattern, ctx, source, groupKey string) bool {
	for _, p := range patterns {
		if patternIsEmpty(p) {
			continue
		}
		// An all-"*" namespace glob ("", "*", "**", …) means "any namespace" —
		// the same any-sentinel globMatch uses — so it hides the whole group. A
		// specific glob (e.g. "kube-*") is namespace-scoped and excluded here,
		// matching patternIgnoresResource's per-resource behavior.
		if p.Namespace != "" && strings.Trim(p.Namespace, "*") != "" {
			continue
		}
		if globMatch(p.Cluster, ctx) &&
			globMatch(p.Source, source) &&
			globMatch(p.Group, groupKey) {
			return true
		}
	}
	return false
}
