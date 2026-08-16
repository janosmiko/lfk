package model

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Taint is a node taint (spec.taints entry).
type Taint struct {
	Key    string
	Value  string
	Effect string
}

// ValidTaintEffects are the effects the Kubernetes API accepts.
var ValidTaintEffects = []string{"NoSchedule", "PreferNoSchedule", "NoExecute"}

// String renders the taint in kubectl notation: key=value:effect, or
// key:effect when the value is empty.
func (t Taint) String() string {
	if t.Value != "" {
		return t.Key + "=" + t.Value + ":" + t.Effect
	}
	return t.Key + ":" + t.Effect
}

var (
	// qualifiedNameRe matches the name part of a taint key (k8s
	// qualified name): alphanumeric ends, -_. inside, max 63 chars.
	qualifiedNameRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]*[A-Za-z0-9])?$`)
	// dns1123SubdomainRe matches an optional key prefix (max 253 chars).
	dns1123SubdomainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)
	// labelValueRe matches a taint value: empty, or alphanumeric ends
	// with -_. inside, max 63 chars.
	labelValueRe = regexp.MustCompile(`^([A-Za-z0-9]([A-Za-z0-9._-]*[A-Za-z0-9])?)?$`)
)

// ValidateTaint checks key, value, and effect against the Kubernetes
// API's validation rules so a bad taint is rejected at staging time
// instead of surfacing as a server error on apply.
func ValidateTaint(t Taint) error {
	if t.Key == "" {
		return fmt.Errorf("key is required")
	}
	if err := validateTaintKey(t.Key); err != nil {
		return err
	}
	if len(t.Value) > 63 || !labelValueRe.MatchString(t.Value) {
		return fmt.Errorf("invalid value %q: max 63 alphanumeric, -, _, . characters", t.Value)
	}
	if slices.Contains(ValidTaintEffects, t.Effect) {
		return nil
	}
	return fmt.Errorf("invalid effect %q: must be one of %s", t.Effect, strings.Join(ValidTaintEffects, ", "))
}

// validateTaintKey checks a qualified name: optional DNS-subdomain
// prefix (max 253 chars), "/", then the name (max 63 chars).
func validateTaintKey(key string) error {
	name := key
	if prefix, rest, found := strings.Cut(key, "/"); found {
		name = rest
		if strings.Contains(name, "/") {
			return fmt.Errorf("invalid key %q: at most one / allowed", key)
		}
		if prefix == "" || len(prefix) > 253 || !dns1123SubdomainRe.MatchString(prefix) {
			return fmt.Errorf("invalid key prefix %q: must be a DNS subdomain (max 253 chars)", prefix)
		}
	}
	if name == "" || len(name) > 63 || !qualifiedNameRe.MatchString(name) {
		return fmt.Errorf("invalid key %q: name must be max 63 alphanumeric, -, _, . characters", key)
	}
	return nil
}

// taintIdentity is the key+effect pair Kubernetes treats as a taint's
// identity — a node cannot carry two taints with the same key and
// effect, and kubectl's `key-` removal matches on it.
type taintIdentity struct{ key, effect string }

// ComputeFinalTaints returns existing minus removals plus additions,
// matching by key+effect identity (never by position). Removals of
// taints that no longer exist are no-ops. Additions whose identity is
// already present are dropped (the server would reject them). Taints
// present on the node but unknown to the caller — added concurrently —
// survive untouched. Inputs are not mutated.
func ComputeFinalTaints(existing, removals, additions []Taint) []Taint {
	remove := make(map[taintIdentity]bool, len(removals))
	for _, r := range removals {
		remove[taintIdentity{r.Key, r.Effect}] = true
	}
	final := make([]Taint, 0, len(existing)+len(additions))
	present := make(map[taintIdentity]bool, len(existing))
	for _, t := range existing {
		id := taintIdentity{t.Key, t.Effect}
		if remove[id] {
			continue
		}
		final = append(final, t)
		present[id] = true
	}
	for _, a := range additions {
		id := taintIdentity{a.Key, a.Effect}
		if present[id] {
			continue
		}
		final = append(final, a)
		present[id] = true
	}
	return final
}
