package app

import "strings"

// fieldDocMaxEntries bounds the description cache. One entry is a short string,
// so a few hundred cost little, and the bound stops a long session from growing
// the map without end.
const fieldDocMaxEntries = 256

// fieldDocKey addresses one field description. The context is part of the key
// because two clusters can serve different schema versions of the same kind,
// and the api-version because two versions of one resource differ as well.
type fieldDocKey struct {
	context    string
	apiVersion string
	resource   string
	path       string
}

// fieldDocEntry is what the schema says about one field. An empty desc is a
// real answer, not a miss: some fields carry no description at all.
type fieldDocEntry struct {
	fieldType string
	desc      string
}

// fieldDocCache holds descriptions already read from the cluster. Eviction is
// first-in-first-out, which needs only an insertion list and is enough here:
// the cost of a wrong eviction is one extra kubectl call.
type fieldDocCache struct {
	entries map[fieldDocKey]fieldDocEntry
	order   []fieldDocKey // insertion order, oldest first
}

func newFieldDocCache() *fieldDocCache {
	return &fieldDocCache{entries: make(map[fieldDocKey]fieldDocEntry)}
}

func (c *fieldDocCache) get(k fieldDocKey) (fieldDocEntry, bool) {
	if c == nil || c.entries == nil {
		return fieldDocEntry{}, false
	}
	e, ok := c.entries[k]
	return e, ok
}

func (c *fieldDocCache) put(k fieldDocKey, e fieldDocEntry) {
	if c == nil {
		return
	}
	if c.entries == nil {
		c.entries = make(map[fieldDocKey]fieldDocEntry)
	}
	// Overwriting must not add a second order record, or the cache evicts
	// live entries early and settles below the cap.
	if _, exists := c.entries[k]; !exists {
		c.order = append(c.order, k)
	}
	c.entries[k] = e

	for len(c.order) > fieldDocMaxEntries {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
}

func (c *fieldDocCache) len() int {
	if c == nil {
		return 0
	}
	return len(c.entries)
}

// fieldDocState is the footnote pane: whether it is open, what it shows, and
// which fetch it is waiting for. req numbers each fetch so a reply that arrives
// after the cursor moved on is dropped instead of overwriting the new field.
// The cache is a pointer so every copy of the Model shares one map: the
// descriptions belong to the cluster, not to a tab or a viewer.
type fieldDocState struct {
	on      bool
	loading bool
	req     uint64
	key     fieldDocKey
	entry   fieldDocEntry
	err     string
	cache   *fieldDocCache
}

// reset closes the pane. The pane is per resource and costs a fetch, so opening
// the viewer on something else starts with it closed. The cache survives: what
// the schema says does not change when the user closes a pane.
func (s *fieldDocState) reset() {
	s.on, s.loading = false, false
	s.key, s.entry, s.err = fieldDocKey{}, fieldDocEntry{}, ""
}

// fieldDocPath turns an object path into the schema path kubectl explain takes.
// Array indices ("[0]") are dropped because the schema describes the element
// type, not one element: spec.containers[0].image is spec.containers.image.
func fieldDocPath(objPath []string) string {
	segs := make([]string, 0, len(objPath))
	for _, s := range objPath {
		if strings.HasPrefix(s, "[") {
			continue
		}
		segs = append(segs, s)
	}
	return strings.Join(segs, ".")
}

// parseExplainFieldHeader reads the "FIELD: name <type>" line that kubectl
// prints when explaining a single field. parseExplainOutput consumes that line
// as a section header and keeps only the description, so the type is read here.
// A root explain has no such line and returns empty strings.
func parseExplainFieldHeader(output string) (name, fieldType string) {
	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(trimmed, "FIELD:")
		if !ok {
			continue
		}
		parts := strings.Fields(rest)
		if len(parts) == 0 {
			return "", ""
		}
		name = parts[0]
		// The remainder can hold a type, a -required- marker, or both. Only a
		// <bracketed> word is a type.
		for _, p := range parts[1:] {
			if strings.HasPrefix(p, "<") {
				return name, p
			}
		}
		return name, ""
	}
	return "", ""
}
