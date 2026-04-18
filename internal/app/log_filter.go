package app

import (
	"fmt"
	"regexp"
	"strings"
)

// Rule is the common interface implemented by all rule types.
type Rule interface {
	Kind() RuleKind
	Display() string
}

// RuleKind classifies a filter rule.
type RuleKind int

const (
	RuleInclude RuleKind = iota
	RuleExclude
	RuleSeverity
	RuleGroup
	RuleField
)

func (k RuleKind) String() string {
	switch k {
	case RuleInclude:
		return "INC"
	case RuleExclude:
		return "EXC"
	case RuleSeverity:
		return "SEV"
	case RuleGroup:
		return "GRP"
	case RuleField:
		return "FLD"
	}
	return "?"
}

// IncludeMode controls how multiple include rules combine.
type IncludeMode int

const (
	IncludeAny IncludeMode = iota
	IncludeAll
)

func (m IncludeMode) String() string {
	if m == IncludeAll {
		return "all"
	}
	return "any"
}

// PatternMode selects how a PatternRule's pattern is interpreted.
type PatternMode int

const (
	PatternSubstring PatternMode = iota
	PatternRegex
	PatternFuzzy
)

func (m PatternMode) String() string {
	switch m {
	case PatternRegex:
		return "regex"
	case PatternFuzzy:
		return "fuzzy"
	}
	return "substr"
}

// PatternRule is an Include or Exclude rule (Negate=true means Exclude).
type PatternRule struct {
	Pattern string
	Mode    PatternMode
	Negate  bool

	// re is set for PatternRegex mode.
	re *regexp.Regexp
	// lowerPattern caches the lowercase pattern for substring/fuzzy.
	lowerPattern string
}

// NewPatternRule constructs a rule, compiling regex if needed.
func NewPatternRule(pattern string, mode PatternMode, negate bool) (*PatternRule, error) {
	r := &PatternRule{
		Pattern:      pattern,
		Mode:         mode,
		Negate:       negate,
		lowerPattern: strings.ToLower(pattern),
	}
	if mode == PatternRegex {
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", pattern, err)
		}
		r.re = re
	}
	return r, nil
}

// Matches returns true if the line matches the underlying pattern.
// (Note: this does NOT account for Negate — chain logic uses Matches
// in different ways for Include vs Exclude.)
func (r *PatternRule) Matches(line string) bool {
	switch r.Mode {
	case PatternRegex:
		return r.re.MatchString(line)
	case PatternFuzzy:
		return fuzzyMatch(strings.ToLower(line), r.lowerPattern)
	default:
		return strings.Contains(strings.ToLower(line), r.lowerPattern)
	}
}

func (r *PatternRule) Kind() RuleKind {
	if r.Negate {
		return RuleExclude
	}
	return RuleInclude
}

func (r *PatternRule) Display() string {
	prefix := ""
	if r.Mode == PatternFuzzy {
		prefix = "~"
	}
	return fmt.Sprintf("%s%s  (%s)", prefix, r.Pattern, r.Mode.String())
}

// SeverityRule keeps lines whose detected severity is >= Floor.
// Unknown lines are always kept (see spec §4.4).
type SeverityRule struct {
	Floor Severity
}

func (r SeverityRule) Kind() RuleKind { return RuleSeverity }

func (r SeverityRule) Display() string {
	return ">= " + r.Floor.String()
}

// Allows returns true if a line with detected severity sev passes this rule.
func (r SeverityRule) Allows(sev Severity) bool {
	if sev == SeverityUnknown {
		return true
	}
	return sev >= r.Floor
}

// GroupRule combines a set of child rules with a boolean mode (ANY/ALL).
// Used to express nested boolean expressions like `foo AND (bar OR baz)`
// where the outer chain holds [foo-rule, <group>] in ALL mode, and
// <group> holds [bar-rule, baz-rule] in ANY mode.
//
// Children may be PatternRule, GroupRule (nested), or SeverityRule.
// Inside a group, a PatternRule with Negate=true contributes its
// NEGATED match value — this lets users express "matches bar AND NOT
// matches baz" inside a group. This differs from the top-level chain
// where Negate=true pattern rules behave as unconditional excludes
// (drop the line on match); the difference is intentional because the
// top-level exclude semantics predate groups and remain the ergonomic
// default for the common "hide healthchecks" case.
type GroupRule struct {
	Mode     IncludeMode
	Children []Rule
}

func (g *GroupRule) Kind() RuleKind { return RuleGroup }

func (g *GroupRule) Display() string {
	return fmt.Sprintf("Group [%s, %d]", g.Mode.String(), len(g.Children))
}

// Matches returns true when the group's children satisfy its Mode.
// Empty groups behave as identity: ALL-of-nothing is true, ANY-of-nothing
// is false. Nested groups recurse naturally.
func (g *GroupRule) Matches(line string, detector *severityDetector) bool {
	if len(g.Children) == 0 {
		return g.Mode == IncludeAll
	}
	for _, c := range g.Children {
		m := evalRuleAsMatch(c, line, detector)
		if g.Mode == IncludeAll && !m {
			return false
		}
		if g.Mode == IncludeAny && m {
			return true
		}
	}
	return g.Mode == IncludeAll
}

// evalRuleAsMatch evaluates a rule as a boolean "does this line match"
// check — used inside groups where every rule contributes a match value
// rather than triggering top-level bucket semantics (unconditional
// exclude / severity floor). Negation on PatternRule inverts here.
// SeverityRule inside a group evaluates the floor against the detected
// severity (without the Unknown-kept special-case — inside a group,
// "severity >= X" is a pure predicate and Unknown lines fail it).
// FieldRule inside a group parses the line as JSON on demand; non-JSON
// lines fail every field-rule predicate inside a group, matching the
// top-level "drop non-JSON when a FieldRule is active" gate.
func evalRuleAsMatch(r Rule, line string, detector *severityDetector) bool {
	switch rr := r.(type) {
	case *PatternRule:
		m := rr.Matches(line)
		if rr.Negate {
			return !m
		}
		return m
	case *GroupRule:
		return rr.Matches(line, detector)
	case SeverityRule:
		if detector == nil {
			return false
		}
		sev := detector.Detect(line)
		return sev >= rr.Floor
	case *FieldRule:
		j := DetectJSONLine(line)
		if !j.IsJSON {
			return false
		}
		return rr.MatchesJSON(j.Value)
	}
	return false
}

// FilterChain evaluates a sequence of rules against log lines.
type FilterChain struct {
	rules    []Rule
	mode     IncludeMode
	detector *severityDetector

	// Pre-bucketed rule subsets (built once, queried per line).
	severity *SeverityRule // at most one
	excludes []*PatternRule
	includes []Rule // *PatternRule (non-negated), *GroupRule, or *FieldRule

	// hasFieldRule is true when at least one top-level FieldRule was
	// placed in the includes bucket. When true, Keep gates every line
	// through DetectJSONLine and drops non-JSON lines — FieldRules are
	// a structured predicate and can't meaningfully evaluate against
	// free-form lines.
	hasFieldRule bool
}

// NewFilterChain builds a chain from the given rules, include mode, and severity detector.
func NewFilterChain(rules []Rule, mode IncludeMode, detector *severityDetector) *FilterChain {
	c := &FilterChain{
		rules:    append([]Rule(nil), rules...),
		mode:     mode,
		detector: detector,
	}
	for _, r := range rules {
		switch v := r.(type) {
		case SeverityRule:
			rr := v
			c.severity = &rr
		case *PatternRule:
			if v.Negate {
				c.excludes = append(c.excludes, v)
			} else {
				c.includes = append(c.includes, v)
			}
		case *GroupRule:
			c.includes = append(c.includes, v)
		case *FieldRule:
			c.includes = append(c.includes, v)
			c.hasFieldRule = true
		}
	}
	return c
}

// Keep returns true if the line should be visible after applying the chain.
// Evaluation order: severity → field-rule JSON gate → excludes → includes
// (short-circuiting).
//
// When any FieldRule is active (hasFieldRule=true), non-JSON lines are
// dropped as a hard gate before any further evaluation. This is the
// documented trade-off: field rules are a structured predicate and
// can't say anything meaningful about plain-text lines, so the cleanest
// user experience is to show only structured events while they're
// filtering by JSON fields.
func (c *FilterChain) Keep(line string) bool {
	// Severity
	if c.severity != nil {
		sev := c.detector.Detect(line)
		if !c.severity.Allows(sev) {
			return false
		}
	}
	// Field-rule JSON gate. Detect once, then pass the parsed Value
	// into evalRuleAsMatch so each FieldRule doesn't re-parse.
	if c.hasFieldRule {
		j := DetectJSONLine(line)
		if !j.IsJSON {
			return false
		}
	}
	// Excludes (any match drops line)
	for _, r := range c.excludes {
		if r.Matches(line) {
			return false
		}
	}
	// Includes
	if len(c.includes) == 0 {
		return true
	}
	switch c.mode {
	case IncludeAll:
		for _, r := range c.includes {
			if !evalRuleAsMatch(r, line, c.detector) {
				return false
			}
		}
		return true
	default: // IncludeAny
		for _, r := range c.includes {
			if evalRuleAsMatch(r, line, c.detector) {
				return true
			}
		}
		return false
	}
}

// Rules returns the rules in display order (cosmetic).
func (c *FilterChain) Rules() []Rule { return c.rules }

// Active returns true if the chain has any rules.
func (c *FilterChain) Active() bool { return len(c.rules) > 0 }

// regexHeuristic returns true if pattern looks like a regex (contains
// metacharacters that aren't trivially literal).
var regexMetaRE = regexp.MustCompile(`[\\^$.|?*+()\[\]{}]`)

func looksLikeRegex(s string) bool {
	return regexMetaRE.MatchString(s)
}

// ParseRuleInput parses the modal input string into a Rule.
// Syntax (from spec §5.3):
//
//	foo              → Include, substring (or regex if heuristic triggers)
//	~foo             → Include, fuzzy
//	-foo             → Exclude, substring (or regex)
//	-~foo            → Exclude, fuzzy
//	>error|>warn|>info|>debug → Severity floor (exact match, case-insensitive)
//	\-foo            → Include, literal "-foo"
//	(a AND b)        → Group (IncludeAll)
//	(a OR b)         → Group (IncludeAny)
//	(a AND (b OR c)) → Nested group
//	a AND b          → Group (IncludeAll) — implicit outer parens
//	a OR b           → Group (IncludeAny)  — implicit outer parens
//
// AND and OR are case-insensitive whole-word operators. A single group
// expression may use one operator per level — mix at the same level
// (e.g. `(a OR b AND c)`) is rejected; nest with parens to disambiguate.
// Severity floors are not allowed inside groups.
func ParseRuleInput(input string) (Rule, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return nil, fmt.Errorf("empty input")
	}

	// Group expression. Any leading '(' routes into the group parser —
	// the group parser rejects anything that isn't a well-formed group
	// with a clear error so the user sees a useful message.
	if s[0] == '(' {
		return parseGroupExpr(s)
	}

	// Implicit outer parens: if the input contains AND/OR as whole-word
	// operators at the top level (not nested inside parens), wrap it so
	// the user can type `foo AND bar` without the ceremony of parens.
	// Patterns that legitimately contain the string "AND" or "OR" as part
	// of their match text can still be expressed by using parens around
	// the single child — e.g. `(SERVER AND LEAVING)` evaluates as a
	// single-child group if typed that way.
	if containsTopLevelBoolOp(s) {
		return parseGroupExpr("(" + s + ")")
	}

	return parseLeafRule(s)
}

// containsTopLevelBoolOp returns true when s contains " AND " or " OR "
// (case-insensitive, whole-word — whitespace on both sides) outside any
// parenthesised sub-expression. Used to enable the implicit-outer-parens
// shorthand in ParseRuleInput.
func containsTopLevelBoolOp(s string) bool {
	up := strings.ToUpper(s)
	depth := 0
	for i := range len(up) {
		switch up[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ' ', '\t':
			if depth != 0 {
				continue
			}
			rest := up[i+1:]
			if hasKeywordPrefix(rest, "AND") || hasKeywordPrefix(rest, "OR") {
				return true
			}
		}
	}
	return false
}

// hasKeywordPrefix returns true when s starts with keyword followed by
// whitespace — i.e. keyword is a whole word, not a prefix of a longer
// identifier like "ORACLE" or "ANDROID".
func hasKeywordPrefix(s, keyword string) bool {
	if !strings.HasPrefix(s, keyword) {
		return false
	}
	if len(s) == len(keyword) {
		return false // keyword at end of string — incomplete expression
	}
	next := s[len(keyword)]
	return next == ' ' || next == '\t'
}

// parseLeafRule parses a single non-group rule — a severity floor, an
// include/exclude pattern, or a fuzzy/regex pattern. Used by
// ParseRuleInput for top-level scalar input and reused by parseGroupExpr
// for the terms inside a group.
func parseLeafRule(s string) (Rule, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty input")
	}

	// Severity (exact match)
	if len(s) > 1 && s[0] == '>' {
		switch strings.ToLower(s[1:]) {
		case "error":
			return SeverityRule{Floor: SeverityError}, nil
		case "warn":
			return SeverityRule{Floor: SeverityWarn}, nil
		case "info":
			return SeverityRule{Floor: SeverityInfo}, nil
		case "debug":
			return SeverityRule{Floor: SeverityDebug}, nil
		}
		// Falls through to be treated as include pattern.
	}

	// Field rule. A leading '.' routes into the field-rule parser which
	// understands `.path[=|!=|<|<=|>|>=|~] value` plus `[]` array-any.
	// Negating a field rule via a leading '-' is not the intended syntax
	// — field rules negate via `!=` internally — so reject that
	// specifically with a clear message rather than silently turning it
	// into a substring exclude.
	if strings.HasPrefix(s, "-.") {
		return nil, fmt.Errorf(`to negate a field rule, use "!=" inside the rule (e.g. .level!=debug), not a leading "-"`)
	}
	if strings.HasPrefix(s, ".") {
		return parseFieldRule(s)
	}

	// Escaped leading dash
	if strings.HasPrefix(s, `\-`) {
		return NewPatternRule(s[1:], PatternSubstring, false)
	}

	negate := false
	if strings.HasPrefix(s, "-") {
		negate = true
		s = s[1:]
	}

	mode := PatternSubstring
	if strings.HasPrefix(s, "~") {
		mode = PatternFuzzy
		s = s[1:]
	} else if looksLikeRegex(s) {
		mode = PatternRegex
	}

	return NewPatternRule(s, mode, negate)
}

// parseFieldRule parses a field-rule input string (already stripped of
// outer whitespace) starting with '.'. Syntax:
//
//	.path                      → invalid (no operator)
//	.path = value              → equality (case-insensitive for strings)
//	.path != value             → inequality
//	.path > value              → numeric greater-than (and <, <=, >=)
//	.path ~ pattern            → regex or substring match
//	.path[] OP value           → array-any: any element matches
//	.nested.field = value      → dotted path for nested access
//
// Whitespace around the operator is allowed but not required. Values
// on the right are taken verbatim (no quoting, spaces allowed); a
// trailing '\n' is trimmed so the user can't trip over it.
//
// Errors (the strings here are the user-facing messages):
//   - '.' alone or '.=x' → "field rule is missing a path after '.'"
//   - '.field' (no op) → "field rule is missing an operator (use =, !=, <, <=, >, >=, or ~)"
//   - '.field=' (no value) → "field rule is missing a value after the operator"
//   - '.field..sub=x' → "field rule path contains an empty segment"
//   - '.field~[unclosed' → "invalid regex ...".
func parseFieldRule(s string) (Rule, error) {
	if len(s) == 0 || s[0] != '.' {
		return nil, fmt.Errorf("field rule must start with '.'")
	}
	body := s[1:] // drop the leading '.'

	// Find the operator. Longest-match scan so '<=' beats '<', '!='
	// beats '!' (which isn't a valid op anyway), and so on.
	opStart, opEnd, op, err := findFieldOp(body)
	if err != nil {
		return nil, err
	}

	rawPath := strings.TrimSpace(body[:opStart])
	value := strings.TrimSpace(body[opEnd:])

	// Detect the optional "[]" array-any marker at the end of the path
	// segment. It attaches to the path, NOT to the operator (which means
	// ".tags[] = api" and ".tags[]= api" are both valid).
	arrayAny := false
	if strings.HasSuffix(rawPath, "[]") {
		arrayAny = true
		rawPath = rawPath[:len(rawPath)-2]
	}

	if rawPath == "" {
		return nil, fmt.Errorf("field rule is missing a path after '.'")
	}
	if value == "" {
		return nil, fmt.Errorf("field rule is missing a value after the operator")
	}

	// Split dotted path; empty segments (from '..') are an explicit error.
	segments := strings.Split(rawPath, ".")
	for _, seg := range segments {
		if seg == "" {
			return nil, fmt.Errorf("field rule path contains an empty segment")
		}
	}

	return NewFieldRule(segments, arrayAny, op, value)
}

// findFieldOp returns the byte range [opStart, opEnd) of the first
// operator in body along with the corresponding FieldOp. Returns a
// user-friendly error when no operator is found.
//
// Scanning is left-to-right, longest-match at each position; the path
// may contain runes like '[' / ']' that aren't part of an operator,
// which is why the scan is byte-based rather than "find the first '='
// in the string".
func findFieldOp(body string) (int, int, FieldOp, error) {
	for i := range len(body) {
		switch body[i] {
		case '=':
			return i, i + 1, FieldOpEq, nil
		case '!':
			if i+1 < len(body) && body[i+1] == '=' {
				return i, i + 2, FieldOpNeq, nil
			}
			// Bare '!' isn't a valid op — fall through to let the loop
			// keep scanning; it will eventually hit EOF and error out.
		case '>':
			if i+1 < len(body) && body[i+1] == '=' {
				return i, i + 2, FieldOpGte, nil
			}
			return i, i + 1, FieldOpGt, nil
		case '<':
			if i+1 < len(body) && body[i+1] == '=' {
				return i, i + 2, FieldOpLte, nil
			}
			return i, i + 1, FieldOpLt, nil
		case '~':
			return i, i + 1, FieldOpMatch, nil
		}
	}
	return 0, 0, 0, fmt.Errorf("field rule is missing an operator (use =, !=, <, <=, >, >=, or ~)")
}

// parseGroupExpr parses a group expression `(a OP b OP c ...)` into a
// GroupRule. OP is either AND (→ IncludeAll) or OR (→ IncludeAny),
// case-insensitive. All operators at the same nesting level must be
// the same — mix is rejected with a "mix" error asking the user to
// disambiguate with nested parens. Terms may be any non-severity leaf
// rule (substring, fuzzy, negated, regex) or a nested parenthesised
// group. Severity floors (>error etc.) inside a group are rejected.
//
// The parser is intentionally small: a scanner that emits tokens
// (LPAREN, RPAREN, AND, OR, TERM) and a recursive-descent consumer
// (parseGroup → parseTerm) that reuses parseLeafRule for TERM tokens.
// This avoids pulling in a parser-generator dependency for what is a
// tiny, bounded grammar.
func parseGroupExpr(input string) (Rule, error) {
	p := &groupParser{src: input}
	p.advance()
	r, err := p.parseGroup()
	if err != nil {
		return nil, err
	}
	// After a successful parseGroup the parser has already consumed the
	// closing ')' and advanced to the next token. Any non-EOF token
	// means there's extra content after the group.
	if p.tokKind != tkEOF {
		return nil, fmt.Errorf("trailing content after group: %q", strings.TrimSpace(p.src[p.tokStart:]))
	}
	return r, nil
}

// groupParser holds parser state: the source string, the current
// position, and the currently-peeked token produced by advance().
// The lookahead is always one token.
type groupParser struct {
	src string
	pos int

	tokKind  groupTokKind
	tokText  string // raw text for TERM tokens
	tokStart int    // byte offset in src where the current token starts
}

type groupTokKind int

const (
	tkEOF groupTokKind = iota
	tkLParen
	tkRParen
	tkAnd
	tkOr
	tkTerm
)

// skipWS advances pos past any ASCII whitespace.
func (p *groupParser) skipWS() {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			p.pos++
			continue
		}
		break
	}
}

// advance reads the next token from src into (tokKind, tokText). After
// EOF is reached, repeated calls continue to report tkEOF.
func (p *groupParser) advance() {
	p.skipWS()
	p.tokStart = p.pos
	if p.pos >= len(p.src) {
		p.tokKind = tkEOF
		p.tokText = ""
		return
	}
	switch p.src[p.pos] {
	case '(':
		p.tokKind = tkLParen
		p.tokText = "("
		p.pos++
		return
	case ')':
		p.tokKind = tkRParen
		p.tokText = ")"
		p.pos++
		return
	}

	// A TERM runs until the next whitespace, '(' or ')'. Embedded
	// whitespace is not allowed inside a term — AND/OR are whole-word
	// operators, so any whitespace ends the current term. If the term
	// spelled exactly "AND" or "OR" (case-insensitive), classify it as
	// the operator token instead of a literal term.
	start := p.pos
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '(' || c == ')' {
			break
		}
		p.pos++
	}
	text := p.src[start:p.pos]
	switch strings.ToUpper(text) {
	case "AND":
		p.tokKind = tkAnd
		p.tokText = text
	case "OR":
		p.tokKind = tkOr
		p.tokText = text
	default:
		p.tokKind = tkTerm
		p.tokText = text
	}
}

// parseGroup consumes `( term (OP term)* )` where OP is uniform within
// this level. Mix at the same level is rejected.
func (p *groupParser) parseGroup() (Rule, error) {
	if p.tokKind != tkLParen {
		return nil, fmt.Errorf("expected '(' at start of group")
	}
	p.advance() // consume '('

	// Empty group: "()" is explicitly invalid — the user almost
	// certainly meant to type something.
	if p.tokKind == tkRParen {
		return nil, fmt.Errorf("empty group: type at least one term inside parentheses")
	}

	children := make([]Rule, 0, 2)
	first, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	children = append(children, first)

	var mode IncludeMode
	haveMode := false
	for {
		switch p.tokKind {
		case tkRParen:
			p.advance()
			if !haveMode {
				// Single-child group: default to IncludeAny (matches
				// "just wrap this expression in parens"). Neither AND
				// nor OR was seen.
				mode = IncludeAny
			}
			return &GroupRule{Mode: mode, Children: children}, nil
		case tkAnd, tkOr:
			thisMode := IncludeAll
			if p.tokKind == tkOr {
				thisMode = IncludeAny
			}
			if haveMode && thisMode != mode {
				return nil, fmt.Errorf("cannot mix AND and OR at the same level; disambiguate with nested parentheses")
			}
			mode = thisMode
			haveMode = true
			p.advance() // consume OP
			// Right operand.
			switch p.tokKind {
			case tkRParen, tkAnd, tkOr, tkEOF:
				return nil, fmt.Errorf("missing right operand after operator")
			}
			next, err := p.parseTerm()
			if err != nil {
				return nil, err
			}
			children = append(children, next)
		case tkEOF:
			return nil, fmt.Errorf("unclosed group: missing ')'")
		case tkTerm:
			// Two consecutive terms without an operator.
			return nil, fmt.Errorf("missing operator between terms (use AND or OR)")
		default:
			return nil, fmt.Errorf("unexpected token %q in group", p.tokText)
		}
	}
}

// parseTerm consumes either a nested group (when current token is '(')
// or a leaf term (when current token is a TERM). Operators and closing
// parens are rejected here — the caller (parseGroup) surfaces a clearer
// "missing operand" error.
func (p *groupParser) parseTerm() (Rule, error) {
	switch p.tokKind {
	case tkLParen:
		return p.parseGroup()
	case tkTerm:
		text := p.tokText
		// Severity floors inside groups are intentionally not supported
		// — top-level severity has an "Unknown-kept" escape hatch that
		// would confuse users if transplanted mid-expression. Reject
		// early with a clear message rather than silently dropping it
		// to the leaf parser.
		if len(text) > 1 && text[0] == '>' {
			switch strings.ToLower(text[1:]) {
			case "error", "warn", "info", "debug":
				return nil, fmt.Errorf("severity floors (%s) are not supported inside groups — use severity at the top level", text)
			}
		}
		p.advance()
		return parseLeafRule(text)
	case tkAnd, tkOr:
		return nil, fmt.Errorf("missing left operand before %q", p.tokText)
	case tkRParen:
		return nil, fmt.Errorf("unexpected ')' before any term")
	case tkEOF:
		return nil, fmt.Errorf("unexpected end of input; expected term or '('")
	}
	return nil, fmt.Errorf("unexpected token %q", p.tokText)
}

// RuleToInputString renders a rule back to the prefix syntax that ParseRuleInput
// would parse it from. Used for edit mode.
func RuleToInputString(r Rule) string {
	switch v := r.(type) {
	case *PatternRule:
		out := ""
		if v.Negate {
			out = "-"
		}
		if v.Mode == PatternFuzzy {
			out += "~"
		}
		out += v.Pattern
		return out
	case SeverityRule:
		return ">" + strings.ToLower(v.Floor.String())
	case *FieldRule:
		// Render without surrounding whitespace around the operator so
		// the text is stable under parse → render → parse round-trips.
		suffix := ""
		if v.ArrayAny {
			suffix = "[]"
		}
		return "." + strings.Join(v.Path, ".") + suffix + v.Op.String() + v.Value
	}
	return ""
}

// BuildVisibleIndices returns the indices of lines that pass the chain.
// Always returns a non-nil slice (empty rather than nil for empty input).
func BuildVisibleIndices(lines []string, chain *FilterChain) []int {
	indices := make([]int, 0, len(lines))
	if !chain.Active() {
		for i := range lines {
			indices = append(indices, i)
		}
		return indices
	}
	for i, line := range lines {
		if chain.Keep(line) {
			indices = append(indices, i)
		}
	}
	return indices
}

// fuzzyMatch returns true if all characters of pattern appear in order in line.
func fuzzyMatch(line, pattern string) bool {
	if pattern == "" {
		return true
	}
	pi := 0
	for i := 0; i < len(line) && pi < len(pattern); i++ {
		if line[i] == pattern[pi] {
			pi++
		}
	}
	return pi == len(pattern)
}
