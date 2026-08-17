package k8s

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ManifestResourceRef represents a single resource declared in a helm release
// manifest. Fields are extracted from apiVersion, kind, metadata.name, and
// metadata.namespace. Cluster-scoped resources have an empty Namespace.
type ManifestResourceRef struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
}

// manifestDoc is the minimal subset of a Kubernetes object the manifest parser
// needs to identify a resource. Decoding into this small struct keeps the
// parser independent of the kubernetes API types and avoids the cost of
// unmarshaling unrelated spec content.
type manifestDoc struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

// parseHelmManifest parses the multi-document YAML manifest stored inside a
// helm release blob and returns one ManifestResourceRef per non-empty document
// that has both an apiVersion and a kind. Documents that are empty,
// comment-only, or malformed are silently skipped so a single bad template
// cannot prevent the rest of a chart from rendering.
//
// The error return value is reserved for a future fatal-failure signal (such
// as an input that exceeds a size cap). The current best-effort implementation
// never uses it, but keeping the signature lets callers add error handling
// without breaking compatibility.
//
// Implementation note: splitting is done by scanning for "---" at the start of
// a line (column 0) rather than by yaml.v3's streaming Decoder. A malformed
// document makes Decode return the same parse error indefinitely without
// advancing past the bad document, so a streaming loop would hang. Splitting
// first, then decoding each chunk in isolation, is both safe and correct for
// helm manifests: helm emits one resource per doc and block scalar content is
// always indented, so a "---" line at column 0 is always a YAML document
// separator and never part of a string value.
//
//nolint:unparam // error return is reserved for fatal failures (see above).
func parseHelmManifest(manifest string) ([]ManifestResourceRef, error) {
	if strings.TrimSpace(manifest) == "" {
		return nil, nil
	}

	docs := splitYAMLDocuments(manifest)
	if len(docs) == 0 {
		return nil, nil
	}

	refs := make([]ManifestResourceRef, 0, len(docs))
	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		var parsed manifestDoc
		if err := yaml.Unmarshal([]byte(doc), &parsed); err != nil {
			// Best effort: skip a document we can't parse rather than aborting.
			continue
		}
		if parsed.Kind == "" || parsed.APIVersion == "" || parsed.Metadata.Name == "" {
			continue
		}
		refs = append(refs, ManifestResourceRef{
			APIVersion: parsed.APIVersion,
			Kind:       parsed.Kind,
			Name:       parsed.Metadata.Name,
			Namespace:  parsed.Metadata.Namespace,
		})
	}
	if len(refs) == 0 {
		return nil, nil
	}
	return refs, nil
}

// splitYAMLDocuments splits a multi-document YAML string on document separator
// lines. A separator is a line whose first non-whitespace characters are
// exactly "---" (optionally followed by whitespace or a "#" comment). Because
// YAML block scalar contents are always indented below their key, a "---" at
// column 0 is never part of a quoted or literal string value, so this split is
// safe for helm manifest payloads.
func splitYAMLDocuments(manifest string) []string {
	lines := strings.Split(manifest, "\n")
	var docs []string
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		docs = append(docs, current.String())
		current.Reset()
	}
	for _, line := range lines {
		if isYAMLDocSeparator(line) {
			flush()
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	flush()
	return docs
}

// isYAMLDocSeparator reports whether a single line is a YAML document
// separator. The separator must start at column 0 with "---" and may be
// followed only by whitespace or a "#" comment. A line like "---foo" is not a
// separator because YAML requires the "---" token to be followed by either
// end-of-line or whitespace.
func isYAMLDocSeparator(line string) bool {
	if !strings.HasPrefix(line, "---") {
		return false
	}
	rest := line[3:]
	// Empty or whitespace-only tail -> separator.
	trimmed := strings.TrimRight(rest, " \t\r")
	if trimmed == "" {
		return true
	}
	// The character immediately following "---" must be whitespace for the
	// rest-of-line content to even be considered. Otherwise "---foo" is just
	// a document body line that happens to start with dashes.
	if rest[0] != ' ' && rest[0] != '\t' && rest[0] != '\r' {
		return false
	}
	return strings.HasPrefix(strings.TrimLeft(rest, " \t\r"), "#")
}
