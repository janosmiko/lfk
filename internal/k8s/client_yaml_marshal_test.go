package k8s

import (
	"strings"
	"testing"
)

// longScalar mirrors a real Kubernetes status condition message: a single
// string value long enough that gopkg.in/yaml.v2 (used by sigs.k8s.io/yaml)
// folds it across multiple physical lines at its hardcoded 80-column width.
// See https://github.com/janosmiko/lfk/issues/355.
const longScalar = `cannot compose resources: cannot check if composed resource "myappfullb3f4-queue-pe" is namespaced (a PrivateEndpoint named myappfullb3f4-queue-pe): failed to get restmapping: no matches for kind "PrivateEndpoint" in version "network.azure.m.upbound.io/v1beta1"`

func TestMarshalResourceYAMLDoesNotWrapLongScalars(t *testing.T) {
	obj := map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"message": longScalar},
			},
		},
	}

	out, err := marshalResourceYAML(obj)
	if err != nil {
		t.Fatalf("marshalResourceYAML: %v", err)
	}

	// The full value must survive on a single physical line. yaml.v2 would
	// split it into three lines at ~80 columns, breaking both the visual
	// layout and the per-line syntax highlighter.
	if !strings.Contains(out, longScalar) {
		t.Fatalf("long scalar was wrapped or altered; output:\n%s", out)
	}

	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, "is namespaced") && !strings.Contains(line, "cannot compose") {
			t.Fatalf("scalar split across lines (continuation found alone):\n%s", out)
		}
	}
}

func TestMarshalResourceYAMLScalarTypesAndIndent(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"replicas": int64(3),
			// Unstructured CRD integer fields often decode to float64; whole
			// numbers must render without a trailing ".0" or scientific notation.
			"observedGeneration": float64(3),
			"ratio":              float64(1.5),
			"enabled":            true,
			"empty":              nil,
			"version":            "123456789",
			"items":              []any{"blob", "queue"},
		},
	}

	out, err := marshalResourceYAML(obj)
	if err != nil {
		t.Fatalf("marshalResourceYAML: %v", err)
	}

	// Numeric strings stay quoted; ints/floats/bools/null render cleanly; no
	// scientific notation for whole numbers. 2-space indentation is preserved.
	for _, want := range []string{
		"replicas: 3",
		"observedGeneration: 3",
		"ratio: 1.5",
		"enabled: true",
		"empty: null",
		`version: "123456789"`,
		"spec:",
		"  items:",
		"    - blob",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q; got:\n%s", want, out)
		}
	}
}
