package app

import (
	"strings"
	"unicode"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// parseExplainOutput parses the output of `kubectl explain <resource>` and
// returns the resource description and a slice of fields.
//
// The kubectl explain output format is:
//
//	GROUP:      apps
//	KIND:       Deployment
//	VERSION:    v1
//
//	DESCRIPTION:
//	    Description text here...
//
//	FIELDS:
//	  fieldName   <type>
//	    Description of the field...
//
//	  anotherField   <type>
//	    Description...
func parseExplainOutput(output, basePath string) (description string, fields []model.ExplainField) {
	lines := strings.Split(output, "\n")

	type section int
	const (
		sectionNone section = iota
		sectionDescription
		sectionFields
	)

	currentSection := sectionNone
	var descLines []string

	var currentField *model.ExplainField
	var fieldDescLines []string

	flushField := func() {
		if currentField != nil {
			currentField.Description = strings.TrimSpace(strings.Join(fieldDescLines, " "))
			fields = append(fields, *currentField)
			currentField = nil
			fieldDescLines = nil
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect section headers.
		if strings.HasPrefix(trimmed, "DESCRIPTION:") {
			flushField()
			currentSection = sectionDescription
			continue
		}
		if strings.HasPrefix(trimmed, "FIELDS:") || strings.HasPrefix(trimmed, "FIELD:") {
			flushField()
			currentSection = sectionFields
			continue
		}

		// Skip metadata headers (GROUP:, KIND:, VERSION:, RESOURCE:).
		if currentSection == sectionNone {
			continue
		}

		switch currentSection {
		case sectionDescription:
			if trimmed == "" && len(descLines) > 0 {
				descLines = append(descLines, "")
				continue
			}
			if trimmed != "" {
				descLines = append(descLines, trimmed)
			}

		case sectionFields:
			if trimmed == "" {
				continue
			}

			// Determine indentation level to distinguish field names from descriptions.
			indent := countLeadingSpaces(line)

			// In kubectl explain output:
			// - Field name lines have exactly 2 spaces of indentation (or similar small indent).
			// - Description lines have 4+ spaces of indentation.
			// A field line looks like: "  fieldName   <type>" or "  fieldName\t<type>"
			//
			// The key distinction: field names start with a valid Go identifier character
			// and are followed by whitespace + a type in angle brackets.
			if indent <= 3 && isFieldLine(trimmed) {
				name, typ, required := parseFieldLine(trimmed)
				if name != "" {
					flushField()
					fieldPath := name
					if basePath != "" {
						fieldPath = basePath + "." + name
					}
					currentField = &model.ExplainField{
						Name:     name,
						Type:     typ,
						Path:     fieldPath,
						Required: required,
					}
					continue
				}
			}

			// This is a description line for the current field.
			if currentField != nil {
				fieldDescLines = append(fieldDescLines, trimmed)
			}
		}
	}

	flushField()
	description = strings.TrimSpace(strings.Join(descLines, " "))

	return description, fields
}

// countLeadingSpaces returns the number of leading space characters in a line.
func countLeadingSpaces(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else {
			break
		}
	}
	return count
}

// isFieldLine checks if a trimmed line looks like a field definition.
// Field lines start with a valid identifier (letter or underscore), followed by
// whitespace and optionally a type in angle brackets.
func isFieldLine(trimmed string) bool {
	if len(trimmed) == 0 {
		return false
	}

	// Must start with a letter or underscore (valid identifier start).
	firstChar := rune(trimmed[0])
	if !unicode.IsLetter(firstChar) && firstChar != '_' {
		return false
	}

	// Extract the first word (the potential field name).
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return false
	}

	fieldName := parts[0]

	// Field names are typically camelCase identifiers without spaces or punctuation.
	for _, ch := range fieldName {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' && ch != '-' {
			return false
		}
	}

	// If there's a second part, check for type indicator or -required-.
	if len(parts) >= 2 {
		secondPart := parts[1]
		if strings.HasPrefix(secondPart, "<") || strings.HasPrefix(secondPart, "-required-") {
			return true
		}
	}

	// A single-word line that is a valid identifier could be a field without a type.
	// But it could also be a single-word description continuation (like "object.").
	// Heuristic: if it's a single word and doesn't end with punctuation, it's likely a field.
	if len(parts) == 1 {
		// If the word ends with a period, comma, etc., it's likely description text.
		lastChar := fieldName[len(fieldName)-1]
		if lastChar == '.' || lastChar == ',' || lastChar == ';' || lastChar == ':' || lastChar == ')' {
			return false
		}
		return true
	}

	// Multiple words without a type indicator: likely description text.
	// Exception: very short lines (2-3 words) where the field has no explicit type.
	if len(parts) <= 2 {
		// Check if any part looks like a type.
		for _, p := range parts[1:] {
			if strings.HasPrefix(p, "<") {
				return true
			}
		}
	}

	return false
}

// parseRecursiveExplainForSearch parses the output of `kubectl explain --recursive`
// and returns all fields whose names contain the search query (case-insensitive).
// The recursive output format is indented fields like:
//
//	FIELDS:
//	  apiVersion	<string>
//	  kind	<string>
//	  metadata	<ObjectMeta>
//	    annotations	<map[string]string>
//	    name	<string>
//	  spec	<DeploymentSpec>
//	    replicas	<integer>
//	    template	<PodTemplateSpec>
//	      spec	<PodSpec>
//	        containers	<[]Container>
//	          name	<string>
//	          ports	<[]ContainerPort>
//	            containerPort	<integer>
func parseRecursiveExplainForSearch(output, query string) []model.ExplainField {
	lines := strings.Split(output, "\n")
	lowerQuery := strings.ToLower(query)

	var results []model.ExplainField

	// Track indentation levels to build paths.
	// Each indentation level maps to a field name.
	type level struct {
		indent int
		name   string
	}
	var stack []level

	inFields := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "FIELDS:" {
			inFields = true
			continue
		}
		if !inFields {
			continue
		}

		// Count leading spaces (indentation).
		indent := 0
		for _, ch := range line {
			switch ch {
			case ' ':
				indent++
			case '\t':
				indent += 4
			default:
				goto doneIndent
			}
		}
	doneIndent:

		// Parse field name and type: "  fieldName\t<type>" or "  fieldName  <type>"
		parts := strings.SplitN(trimmed, "\t", 2)
		if len(parts) == 0 {
			continue
		}
		fieldName := strings.TrimSpace(parts[0])
		fieldType := ""
		if len(parts) > 1 {
			fieldType = strings.TrimSpace(parts[1])
		}

		// Skip non-field lines (descriptions, etc).
		if fieldName == "" || fieldName == "DESCRIPTION:" || strings.HasPrefix(fieldName, "GROUP:") ||
			strings.HasPrefix(fieldName, "KIND:") || strings.HasPrefix(fieldName, "VERSION:") ||
			strings.HasPrefix(fieldName, "RESOURCE:") || strings.HasPrefix(fieldName, "FIELD:") {
			continue
		}

		// Pop stack to find parent at this indentation level.
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}

		// Build full path.
		pathParts := make([]string, 0, len(stack)+1)
		for _, s := range stack {
			pathParts = append(pathParts, s.name)
		}
		pathParts = append(pathParts, fieldName)
		fullPath := strings.Join(pathParts, ".")

		// Push current field to stack.
		stack = append(stack, level{indent: indent, name: fieldName})

		// When query is empty, return all fields (for namespace-selector-style browsing).
		if lowerQuery == "" || ui.MatchLine(fieldName, query) {
			results = append(results, model.ExplainField{
				Name: fieldName,
				Type: fieldType,
				Path: fullPath,
			})
		}
	}

	return results
}

// parseFieldLine extracts the field name, type, and required status from a field line.
// Field lines look like: "fieldName   <type>" or "fieldName	<type> -required-"
func parseFieldLine(trimmed string) (name, typ string, required bool) {
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return "", "", false
	}

	name = parts[0]

	// Collect type parts, extracting -required- marker separately.
	var typeParts []string
	for _, p := range parts[1:] {
		if p == "-required-" {
			required = true
			continue
		}
		if strings.HasPrefix(p, "<") {
			typeParts = append(typeParts, p)
		}
	}

	if len(typeParts) > 0 {
		typ = strings.Join(typeParts, " ")
	}

	return name, typ, required
}
