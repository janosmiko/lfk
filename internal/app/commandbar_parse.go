package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// commandType classifies what kind of command bar input the user is typing.
type commandType int

const (
	cmdUnknown      commandType = iota
	cmdShell                    // :! prefix -- run a shell command
	cmdBuiltin                  // :ns, :ctx, :set, :sort, :export, :q
	cmdKubectl                  // :kubectl/k get pods, :get pods
	cmdResourceJump             // :pod, :dep, :pvc -- jump to a resource type
)

// token represents a single word in the command bar input with its position.
type token struct {
	text  string
	start int
	end   int
}

// builtinCommands maps command names and aliases to their canonical form.
var builtinCommands = map[string]string{
	"namespace": "namespace",
	"ns":        "namespace",
	"context":   "context",
	"ctx":       "context",
	"set":       "set",
	"sort":      "sort",
	"export":    "export",
	"quit":      "quit",
	"q":         "quit",
	"q!":        "quit",
	"nyan":      "nyan",
	"kubetris":  "kubetris",
	"credits":   "credits",
	"tasks":     "tasks",
	"errors":    "errors",
	"warnings":  "errors",
	"bookmarks": "bookmarks",
	"reload":    "reload",
	"refresh":   "reload",
}

// kubectlSubcommandSet contains known kubectl subcommands.
var kubectlSubcommandSet = map[string]bool{
	"get":           true,
	"describe":      true,
	"logs":          true,
	"exec":          true,
	"delete":        true,
	"apply":         true,
	"create":        true,
	"edit":          true,
	"patch":         true,
	"scale":         true,
	"rollout":       true,
	"top":           true,
	"label":         true,
	"annotate":      true,
	"port-forward":  true,
	"cp":            true,
	"cordon":        true,
	"uncordon":      true,
	"drain":         true,
	"taint":         true,
	"config":        true,
	"auth":          true,
	"api-resources": true,
	"explain":       true,
	"diff":          true,
}

// classifyInput determines the command type from the raw input string.
// The input is the text after the ":" prefix has been stripped.
//
// Priority order:
//  1. "!" prefix -> cmdShell
//  2. First word matches builtinCommands key -> cmdBuiltin
//  3. First word is kubectl/k or in kubectlSubcommandSet -> cmdKubectl
//  4. First word matches a known resource type -> cmdResourceJump
//  5. Otherwise -> cmdUnknown
func classifyInput(input string) commandType {
	if input == "" {
		return cmdUnknown
	}

	// 1. Shell commands: starts with "!"
	if input[0] == '!' {
		return cmdShell
	}

	firstWord := firstWordOf(input)

	// 2. Builtin commands
	if _, ok := builtinCommands[firstWord]; ok {
		return cmdBuiltin
	}

	// 3. Kubectl commands (only with explicit kubectl/k prefix).
	if firstWord == "kubectl" || firstWord == "k" {
		return cmdKubectl
	}

	// 4. Resource jump (built-in types only; CRDs checked by classifyInputWithCRDs).
	if isKnownResourceType(firstWord) {
		return cmdResourceJump
	}

	return cmdUnknown
}

// classifyInputWithCRDs extends classifyInput to also recognize CRD resource names.
func classifyInputWithCRDs(input string, crdNames []string) commandType {
	ct := classifyInput(input)
	if ct != cmdUnknown {
		return ct
	}
	// Check if first word matches a CRD name (plural or singular).
	firstWord := strings.ToLower(firstWordOf(strings.TrimSpace(input)))
	for _, crd := range crdNames {
		lower := strings.ToLower(crd)
		if lower == firstWord || toSingular(lower) == firstWord {
			return cmdResourceJump
		}
	}
	return cmdUnknown
}

// firstWordOf returns the first whitespace-delimited word from s.
func firstWordOf(s string) string {
	word, _, _ := strings.Cut(s, " ")
	return word
}

// isKnownResourceType checks whether name matches a known Kubernetes resource
// type by searching abbreviations and BuiltInMetadata via KnownResourceNames.
// Matches only the plural resource name — Kind matching was dropped because
// the abbreviations table covers most short-form identifiers.
func isKnownResourceType(name string) bool {
	lower := strings.ToLower(name)
	if ui.SearchAbbreviations != nil {
		if _, ok := ui.SearchAbbreviations[lower]; ok {
			return true
		}
	}
	if model.KnownResourceNames()[lower] {
		return true
	}
	return false
}

// parseTokens splits input into whitespace-delimited tokens with position info.
// If input is empty, a single empty token at position 0 is returned.
// If input ends with a space, a trailing empty token is appended to signal
// that the cursor is at a new-word position.
func parseTokens(input string, _ int) []token {
	if input == "" {
		return []token{{text: "", start: 0, end: 0}}
	}

	words := strings.Split(input, " ")
	tokens := make([]token, 0, len(words))
	pos := 0

	for _, w := range words {
		end := pos + len(w)
		tokens = append(tokens, token{text: w, start: pos, end: end})
		pos = end + 1 // skip the space
	}

	return tokens
}
