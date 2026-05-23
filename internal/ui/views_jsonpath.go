package ui

import (
	"bytes"
	"fmt"
	"strings"

	"k8s.io/client-go/util/jsonpath"
)

// EvalJSONPath evaluates a JSONPath expression against the given unstructured
// object and returns the stringified result. Returns "" when the path doesn't
// resolve, the expression is invalid, or the result is empty. This is a
// best-effort display helper — errors are intentionally silent. Callers wanting
// to validate an expression should compile it via CompileJSONPath at config
// load time.
func EvalJSONPath(expr string, obj map[string]any) string {
	if expr == "" || obj == nil {
		return ""
	}
	jp, err := CompileJSONPath(expr)
	if err != nil {
		return ""
	}
	return EvalCompiled(jp, obj)
}

// CompileJSONPath compiles a k9s-style JSONPath expression (no surrounding
// braces) into a reusable template. Returns an error for malformed
// expressions so the caller (config applier) can surface a warning.
func CompileJSONPath(expr string) (*jsonpath.JSONPath, error) {
	tmpl := strings.TrimSpace(expr)
	if !strings.HasPrefix(tmpl, "{") {
		if !strings.HasPrefix(tmpl, ".") {
			return nil, fmt.Errorf("invalid JSONPath %q: expression must start with '.'", expr)
		}
		tmpl = "{" + tmpl + "}"
	}
	jp := jsonpath.New("view")
	jp.AllowMissingKeys(true)
	if err := jp.Parse(tmpl); err != nil {
		return nil, fmt.Errorf("invalid JSONPath %q: %w", expr, err)
	}
	return jp, nil
}

// EvalCompiled runs a pre-compiled JSONPath against the given object and
// returns the stringified result. Returns "" on any execution error, nil
// template, or nil object.
func EvalCompiled(jp *jsonpath.JSONPath, obj map[string]any) string {
	if jp == nil || obj == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := jp.Execute(&buf, obj); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}
