package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// condPolarity classifies how a status condition's type relates to health.
type condPolarity int

const (
	condInfo    condPolarity = iota // neutral marker (Issuing, Progressing, Reconciling)
	condReady                       // good when True, bad when False (Ready, Available, Synced)
	condError                       // bad when True / present (Error, Failed, Degraded, Stalled)
	condWarning                     // always amber (any "*Warning" type)
)

// conditionPolarities is a curated map of well-known custom-resource condition
// types to their polarity. It makes the supported CRs explicit and overrides
// the heuristic where the type name alone would misclassify (e.g. cert-manager
// "Issuing", whose False state is normal, not a failure). Keys are lowercase.
var conditionPolarities = map[string]condPolarity{
	// ArgoCD Application (status-less, the type's presence is the signal).
	"comparisonerror":         condError,
	"invalidspecerror":        condError,
	"unknownerror":            condError,
	"syncerror":               condError,
	"deletionerror":           condError,
	"sharedresourcewarning":   condWarning,
	"repeatedresourcewarning": condWarning,
	"excludedresourcewarning": condWarning,
	"orphanedresourcewarning": condWarning,
	// cert-manager.
	"ready":   condReady,
	"issuing": condInfo, // actively issuing. False is the normal idle state
	// external-secrets.
	"secretsynced": condReady,
	// FluxCD.
	"reconciling": condInfo,
	"stalled":     condError,
	"healthy":     condReady,
}

var (
	conditionErrorKeywords = []string{"error", "fail", "degrad", "invalid", "unhealthy", "pressure", "stalled", "lost", "backoff", "misconfig"}
	conditionReadyKeywords = []string{"ready", "available", "synced", "healthy", "succeeded", "complete", "established", "approved", "provisioned", "bound", "scheduled", "initialized"}
)

// conditionPolarity returns the polarity of a condition type, consulting the
// curated map first and falling back to a keyword heuristic.
func conditionPolarity(condType string) condPolarity {
	lower := strings.ToLower(condType)
	if p, ok := conditionPolarities[lower]; ok {
		return p
	}
	// Match keyword stems against whole camelCase tokens, not raw substrings,
	// so negated types do not invert: "Unbound" -> ["unbound"] does not start
	// with "bound", and "Incomplete" -> ["incomplete"] not with "complete".
	tokens := conditionTokens(condType)
	switch {
	case tokenHasPrefix(tokens, []string{"warn"}):
		return condWarning
	case tokenHasPrefix(tokens, conditionErrorKeywords):
		return condError
	case tokenHasPrefix(tokens, conditionReadyKeywords):
		return condReady
	default:
		return condInfo
	}
}

// conditionTokens splits a PascalCase/camelCase condition type into lowercase
// word tokens (e.g. "ContainersReady" -> ["containers", "ready"]). Runs of
// non-letters and case transitions are token boundaries.
func conditionTokens(condType string) []string {
	var tokens []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			tokens = append(tokens, strings.ToLower(string(cur)))
			cur = nil
		}
	}
	for _, r := range condType {
		switch {
		case r >= 'A' && r <= 'Z':
			flush()
			cur = append(cur, r)
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			cur = append(cur, r)
		default:
			flush()
		}
	}
	flush()
	return tokens
}

// tokenHasPrefix reports whether any token starts with one of the keyword stems.
func tokenHasPrefix(tokens, keywords []string) bool {
	for _, tok := range tokens {
		for _, kw := range keywords {
			if strings.HasPrefix(tok, kw) {
				return true
			}
		}
	}
	return false
}

// ConditionStyle returns the color style for a status condition, combining its
// type's polarity with its status value. Conditions without a True/False status
// (e.g. ArgoCD application conditions) are colored by type polarity alone.
func ConditionStyle(condType, status string) lipgloss.Style {
	if status == "Unknown" {
		return DimStyle
	}
	switch conditionPolarity(condType) {
	case condWarning:
		return StatusWarning
	case condError:
		if status == "False" {
			return StatusRunning // a failure condition that is False = healthy
		}
		return StatusFailed // True or status-less = problem
	case condReady:
		if status == "False" {
			return StatusFailed // a readiness condition that is False = problem
		}
		return StatusRunning // True or status-less = healthy
	default: // condInfo
		if status == "True" {
			return StatusProgressing // active / in progress
		}
		return DimStyle // False or status-less = neutral
	}
}
