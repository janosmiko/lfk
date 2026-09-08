package k8s

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestToKYAML_SingleDocument(t *testing.T) {
	got, err := ToKYAML("apiVersion: v1\nkind: Pod\nmetadata:\n  name: web\n")
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(got, "---\n"), "KYAML documents carry the --- header, got %q", got)
	assert.Contains(t, got, `apiVersion: "v1",`)
	assert.Contains(t, got, "metadata: {")
	assert.True(t, strings.HasSuffix(got, "}\n"), "got %q", got)
}

func TestToKYAML_QuotesAmbiguousScalars(t *testing.T) {
	got, err := ToKYAML("data:\n  enabled: yes\n  port: 8080\n")
	require.NoError(t, err)

	assert.Contains(t, got, `enabled: "yes",`, "the Norway problem is solved by quoting, not by coercion")
	assert.Contains(t, got, "port: 8080,", "numbers stay unquoted")
}

func TestToKYAML_MultiDocument(t *testing.T) {
	got, err := ToKYAML("kind: Pod\n---\nkind: Service\n")
	require.NoError(t, err)

	assert.Equal(t, 2, strings.Count(got, "---\n"), "one header per document, got %q", got)
	assert.Contains(t, got, `kind: "Pod",`)
	assert.Contains(t, got, `kind: "Service",`)
	assert.Less(t, strings.Index(got, "Pod"), strings.Index(got, "Service"), "document order preserved")
}

func TestToKYAML_MultiDocumentRoundTripsEveryDocument(t *testing.T) {
	got, err := ToKYAML("a: 1\n---\nb: 2\n---\nc: 3\n")
	require.NoError(t, err)

	docs := strings.Split(strings.TrimPrefix(got, "---\n"), "---\n")
	require.Len(t, docs, 3)
	for i, want := range []map[string]int{{"a": 1}, {"b": 2}, {"c": 3}} {
		var out map[string]int
		require.NoError(t, yaml.Unmarshal([]byte(docs[i]), &out))
		assert.Equal(t, want, out)
	}
}

func TestToKYAML_InvalidYAML(t *testing.T) {
	_, err := ToKYAML("a: [1, 2\nb: {")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "converting YAML to KYAML")
}

func TestToKYAML_RoundTripsToTheSameObject(t *testing.T) {
	src := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels:
    app: web
    "tricky.key/with.dots": "no"
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: app
          image: nginx:1.27
          args: ["--a", "--b"]
          ports:
            - containerPort: 8080
`
	got, err := ToKYAML(src)
	require.NoError(t, err)

	var want, have map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(src), &want))
	require.NoError(t, yaml.Unmarshal([]byte(got), &have))
	assert.Equal(t, want, have)
}

// kubectl and the clipboard joiner can leave a bare "---" at the end. The
// encoder turns a trailing separator into an empty document if it is kept.
func TestToKYAML_TrailingSeparatorProducesNoEmptyDocument(t *testing.T) {
	for name, in := range map[string]string{
		"lf":             "a: 1\n---\n",
		"no trailing lf": "a: 1\n---",
		"crlf":           "a: 1\r\n---\r\n",
		"blank lines":    "a: 1\n---\n\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ToKYAML(in)
			require.NoError(t, err)
			assert.Equal(t, 1, strings.Count(got, "---\n"), "one document, got %q", got)
			assert.Contains(t, got, "a: 1,")
			assert.NotContains(t, got, "---\n\n", "a separator followed by nothing is an empty document")
		})
	}
}

// A "---" indented inside a block scalar is content, not a document separator.
func TestToKYAML_KeepsSeparatorLookalikeInsideBlockScalar(t *testing.T) {
	for name, in := range map[string]string{
		"only line":  "data:\n  note: |\n    ---\n",
		"last line":  "data:\n  note: |\n    a\n    ---\n",
		"deeper":     "a:\n  b:\n    note: |\n        ---\n",
		"tab indent": "data:\n  note: |\n    ---\n",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ToKYAML(in)
			require.NoError(t, err)

			var want, have map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(in), &want))
			require.NoError(t, yaml.Unmarshal([]byte(got), &have))
			assert.Equal(t, want, have, "the block scalar must survive, got %q", got)
			assert.Contains(t, got, "---", "the scalar's own --- text is part of the value")
		})
	}
}

// Chomping keeps trailing newlines significant, so a document that does not end
// in a separator must come back untouched.
func TestToKYAML_KeepsKeepChompedTrailingNewlines(t *testing.T) {
	in := "data:\n  note: |+\n    a\n\n"
	got, err := ToKYAML(in)
	require.NoError(t, err)

	var want, have map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(in), &want))
	require.NoError(t, yaml.Unmarshal([]byte(got), &have))
	assert.Equal(t, want, have)
}

func TestToKYAML_SeparatorOnlyInput(t *testing.T) {
	got, err := ToKYAML("---\n")
	require.NoError(t, err)
	assert.Empty(t, got, "a document separator with no document is nothing to render")
}

func TestToKYAML_CRLFMultiDocument(t *testing.T) {
	got, err := ToKYAML("kind: Pod\r\n---\r\nkind: Service\r\n")
	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(got, "---\n"), "got %q", got)
	assert.Contains(t, got, `kind: "Pod",`)
	assert.Contains(t, got, `kind: "Service",`)
}

func TestToKYAML_Empty(t *testing.T) {
	got, err := ToKYAML("")
	require.NoError(t, err)
	assert.Empty(t, got)
}
