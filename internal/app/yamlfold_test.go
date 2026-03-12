package app

import (
	"strings"
	"testing"
)

const testYAML = `apiVersion: v1
kind: Pod
metadata:
  name: nginx
  namespace: default
  labels:
    app: nginx
spec:
  containers:
  - name: nginx
    image: nginx:latest
status:
  phase: Running
  conditions:
  - type: Ready
    status: "True"`

// testYAML has 16 lines (0-15):
// 0: apiVersion: v1         (not a section - has inline value)
// 1: kind: Pod              (not a section - has inline value)
// 2: metadata:              (section key="metadata")
// 3:   name: nginx
// 4:   namespace: default
// 5:   labels:              (section key="metadata.labels")
// 6:     app: nginx
// 7: spec:                  (section key="spec")
// 8:   containers:          (section key="spec.containers")
// 9:   - name: nginx
// 10:    image: nginx:latest
// 11: status:               (section key="status")
// 12:   phase: Running
// 13:   conditions:          (section key="status.conditions")
// 14:   - type: Ready
// 15:     status: "True"

func TestParseYAMLSections(t *testing.T) {
	sections := parseYAMLSections(testYAML)

	expected := []struct {
		key      string
		start    int
		end      int
		listItem bool
	}{
		{"metadata", 2, 6, false},
		{"metadata.labels", 5, 6, false},
		{"spec", 7, 10, false},
		{"spec.containers", 8, 10, false},
		{"spec.containers.#9", 9, 10, true},
		{"status", 11, 15, false},
		{"status.conditions", 13, 15, false},
		{"status.conditions.#14", 14, 15, true},
	}

	if len(sections) != len(expected) {
		t.Fatalf("expected %d sections, got %d: %+v", len(expected), len(sections), sections)
	}

	for i, exp := range expected {
		sec := sections[i]
		if sec.key != exp.key {
			t.Errorf("section %d: expected key %q, got %q", i, exp.key, sec.key)
		}
		if sec.startLine != exp.start {
			t.Errorf("section %d (%s): expected startLine %d, got %d", i, exp.key, exp.start, sec.startLine)
		}
		if sec.endLine != exp.end {
			t.Errorf("section %d (%s): expected endLine %d, got %d", i, exp.key, exp.end, sec.endLine)
		}
		if sec.listItem != exp.listItem {
			t.Errorf("section %d (%s): expected listItem %v, got %v", i, exp.key, exp.listItem, sec.listItem)
		}
	}
}

func TestIsMultiLineSection(t *testing.T) {
	sections := parseYAMLSections(testYAML)

	// All detected sections should be multi-line (single-line keys are not sections).
	for _, sec := range sections {
		if !isMultiLineSection(sec) {
			t.Errorf("section %q should be multi-line", sec.key)
		}
	}
}

func TestBuildVisibleLinesNoCollapse(t *testing.T) {
	sections := parseYAMLSections(testYAML)
	collapsed := map[string]bool{}

	visLines, mapping := buildVisibleLines(testYAML, sections, collapsed)

	// All 16 lines should be visible.
	if len(visLines) != 16 {
		t.Fatalf("expected 16 visible lines, got %d", len(visLines))
	}
	if len(mapping) != 16 {
		t.Fatalf("expected 16 mappings, got %d", len(mapping))
	}

	// Multi-line section headers should have fold indicators.
	// metadata: is at original line 2 (indent=0, marker at position 0).
	if !strings.ContainsRune(visLines[2], '▾') {
		t.Errorf("metadata line should contain fold indicator ▾, got %q", visLines[2])
	}
	// labels: is at original line 5 (indent=2, marker at position 2).
	if !strings.ContainsRune(visLines[5], '▾') {
		t.Errorf("labels line should contain fold indicator ▾, got %q", visLines[5])
	}
	// Non-section lines should have alignment padding.
	if visLines[0][:2] != "  " {
		t.Errorf("apiVersion line should start with padding, got %q", visLines[0][:2])
	}
}

func TestBuildVisibleLinesWithCollapse(t *testing.T) {
	sections := parseYAMLSections(testYAML)
	collapsed := map[string]bool{
		"metadata": true,
		"status":   true,
	}

	visLines, _ := buildVisibleLines(testYAML, sections, collapsed)

	// metadata has lines 3-6 hidden (4 lines), status has lines 12-15 hidden (4 lines).
	// Total: 16 - 4 - 4 = 8 visible lines.
	expectedCount := 8
	if len(visLines) != expectedCount {
		t.Fatalf("expected %d visible lines, got %d", expectedCount, len(visLines))
	}

	// metadata header should show collapsed indicator.
	found := false
	for _, line := range visLines {
		if strings.ContainsRune(line, '▸') {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected collapsed fold indicator (▸) for metadata")
	}
}

func TestBuildVisibleLinesNestedCollapse(t *testing.T) {
	sections := parseYAMLSections(testYAML)
	collapsed := map[string]bool{
		"metadata.labels": true,
	}

	visLines, _ := buildVisibleLines(testYAML, sections, collapsed)

	// metadata.labels hides line 6 only (1 line hidden).
	// Total: 16 - 1 = 15.
	expectedCount := 15
	if len(visLines) != expectedCount {
		t.Fatalf("expected %d visible lines, got %d", expectedCount, len(visLines))
	}

	// The labels line itself should show collapsed indicator.
	found := false
	for _, line := range visLines {
		if strings.ContainsRune(line, '▸') {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected collapsed fold indicator (▸) for metadata.labels")
	}
}

func TestBuildVisibleLinesParentCollapsesChild(t *testing.T) {
	sections := parseYAMLSections(testYAML)
	collapsed := map[string]bool{
		"metadata": true,
	}

	visLines, _ := buildVisibleLines(testYAML, sections, collapsed)

	// metadata collapses lines 3-6 (4 lines hidden), which includes the
	// metadata.labels header at line 5 and its content at line 6.
	expectedCount := 12
	if len(visLines) != expectedCount {
		t.Fatalf("expected %d visible lines, got %d", expectedCount, len(visLines))
	}
}

func TestVisibleLineCount(t *testing.T) {
	sections := parseYAMLSections(testYAML)

	// No collapse: 16 lines total.
	count := visibleLineCount(testYAML, sections, map[string]bool{})
	if count != 16 {
		t.Errorf("expected 16 visible lines, got %d", count)
	}

	// Collapse metadata (lines 3-6 hidden = 4 lines).
	count = visibleLineCount(testYAML, sections, map[string]bool{"metadata": true})
	if count != 12 {
		t.Errorf("expected 12 visible lines with metadata collapsed, got %d", count)
	}

	// Collapse all top-level multi-line sections.
	allCollapsed := map[string]bool{
		"metadata": true,
		"spec":     true,
		"status":   true,
	}
	count = visibleLineCount(testYAML, sections, allCollapsed)
	// 16 - 4 (metadata body: 3-6) - 3 (spec body: 8-10) - 4 (status body: 12-15) = 5
	if count != 5 {
		t.Errorf("expected 5 visible lines with all top-level collapsed, got %d", count)
	}

	// Collapse parent and child: overlap should not double-count.
	bothCollapsed := map[string]bool{
		"metadata":        true,
		"metadata.labels": true,
	}
	count = visibleLineCount(testYAML, sections, bothCollapsed)
	// metadata hides 3-6 (4 lines); metadata.labels would hide line 6,
	// but it's already hidden. Total hidden = 4. 16 - 4 = 12.
	if count != 12 {
		t.Errorf("expected 12 with overlapping collapse, got %d", count)
	}
}

func TestSectionForVisibleLine(t *testing.T) {
	sections := parseYAMLSections(testYAML)
	collapsed := map[string]bool{}
	_, mapping := buildVisibleLines(testYAML, sections, collapsed)

	// Line 0 is apiVersion (original line 0) -- not inside any section.
	// sectionForVisibleLine iterates sections in order, so it won't match
	// anything because line 0 is before the first section (metadata at line 2).
	sec := sectionForVisibleLine(0, mapping, sections)
	if sec != "" {
		t.Errorf("expected no section at visible line 0, got %q", sec)
	}

	// Line 4 is inside metadata (original line 4).
	sec = sectionForVisibleLine(4, mapping, sections)
	if sec != "metadata" {
		t.Errorf("expected metadata at visible line 4, got %q", sec)
	}

	// Line 5 is the labels: header (original line 5). It falls within both
	// metadata and metadata.labels; sectionForVisibleLine returns the exact header match.
	sec = sectionForVisibleLine(5, mapping, sections)
	if sec != "metadata.labels" {
		t.Errorf("expected metadata.labels at visible line 5, got %q", sec)
	}

	// Line 11 is status: (original line 11).
	sec = sectionForVisibleLine(11, mapping, sections)
	if sec != "status" {
		t.Errorf("expected status at visible line 11, got %q", sec)
	}
}

func TestOriginalToVisible(t *testing.T) {
	sections := parseYAMLSections(testYAML)
	collapsed := map[string]bool{"metadata": true}
	_, mapping := buildVisibleLines(testYAML, sections, collapsed)

	// Original line 0 (apiVersion) should be visible line 0.
	if idx := originalToVisible(0, mapping); idx != 0 {
		t.Errorf("expected original line 0 at visible 0, got %d", idx)
	}

	// Original line 3 (inside collapsed metadata) should be hidden.
	if idx := originalToVisible(3, mapping); idx != -1 {
		t.Errorf("expected original line 3 to be hidden, got %d", idx)
	}

	// Original line 7 (spec:) should still be visible (at shifted index).
	idx := originalToVisible(7, mapping)
	if idx < 0 {
		t.Error("expected spec line to be visible")
	}
}

func TestParseYAMLSectionsWithComments(t *testing.T) {
	yaml := `# This is a comment
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value`

	sections := parseYAMLSections(yaml)
	// metadata: has children (name: test), data: has children (key: value).
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d: %+v", len(sections), sections)
	}
	if sections[0].key != "metadata" {
		t.Errorf("first section should be metadata, got %q", sections[0].key)
	}
	if sections[1].key != "data" {
		t.Errorf("second section should be data, got %q", sections[1].key)
	}
}

func TestParseYAMLSectionsWithDocSeparator(t *testing.T) {
	yaml := `---
apiVersion: v1
kind: Service`

	sections := parseYAMLSections(yaml)
	// apiVersion and kind both have inline values, so no sections.
	if len(sections) != 0 {
		t.Fatalf("expected 0 sections, got %d: %+v", len(sections), sections)
	}
}

func TestBuildDotPath(t *testing.T) {
	yaml := `metadata:
  name: test
  annotations:
    note: hello`

	sections := parseYAMLSections(yaml)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d: %+v", len(sections), sections)
	}
	if sections[0].key != "metadata" {
		t.Errorf("expected key 'metadata', got %q", sections[0].key)
	}
	if sections[1].key != "metadata.annotations" {
		t.Errorf("expected key 'metadata.annotations', got %q", sections[1].key)
	}
}

func TestDeepNesting(t *testing.T) {
	yaml := `spec:
  template:
    spec:
      containers:
        - name: nginx
          ports:
            - containerPort: 80`

	sections := parseYAMLSections(yaml)

	expected := []struct {
		key   string
		start int
	}{
		{"spec", 0},
		{"spec.template", 1},
		{"spec.template.spec", 2},
		{"spec.template.spec.containers", 3},
		{"spec.template.spec.containers.#4", 4},
		{"spec.template.spec.containers.ports", 5},
	}

	if len(sections) != len(expected) {
		t.Fatalf("expected %d sections, got %d: %+v", len(expected), len(sections), sections)
	}
	for i, exp := range expected {
		if sections[i].key != exp.key {
			t.Errorf("section %d: expected key %q, got %q", i, exp.key, sections[i].key)
		}
		if sections[i].startLine != exp.start {
			t.Errorf("section %d (%s): expected startLine %d, got %d", i, exp.key, exp.start, sections[i].startLine)
		}
	}
}

func TestFoldableListItems(t *testing.T) {
	yaml := `spec:
  containers:
  - name: nginx
    image: nginx:latest
    ports:
    - containerPort: 80
  - name: sidecar
    image: sidecar:latest`

	sections := parseYAMLSections(yaml)

	// Should detect: spec, spec.containers, spec.containers.#2 (first list item),
	// spec.containers.ports (nested section), spec.containers.#6 (second list item)

	// Verify list items are detected as sections.
	listItemCount := 0
	for _, sec := range sections {
		if sec.listItem {
			listItemCount++
		}
	}
	if listItemCount < 2 {
		t.Errorf("expected at least 2 foldable list items, got %d. Sections: %+v", listItemCount, sections)
	}

	// Test folding a list item hides its content but not its siblings.
	// Find the first list item section.
	var firstListItem string
	for _, sec := range sections {
		if sec.listItem {
			firstListItem = sec.key
			break
		}
	}
	if firstListItem == "" {
		t.Fatal("no list item section found")
	}

	collapsed := map[string]bool{firstListItem: true}
	visLines, _ := buildVisibleLines(yaml, sections, collapsed)

	// The second list item ("- name: sidecar") should still be visible.
	foundSidecar := false
	for _, line := range visLines {
		if strings.Contains(line, "sidecar") {
			foundSidecar = true
			break
		}
	}
	if !foundSidecar {
		t.Error("collapsing first list item should not hide sibling list items")
	}

	// "image: nginx" should be hidden (it's content of the first list item).
	foundNginxImage := false
	for _, line := range visLines {
		if strings.Contains(line, "image: nginx:latest") {
			foundNginxImage = true
			break
		}
	}
	if foundNginxImage {
		t.Error("collapsing first list item should hide its content (image: nginx:latest)")
	}
}

// TestListItemFoldPreservesDash verifies that fold indicators don't replace the dash.
func TestListItemFoldPreservesDash(t *testing.T) {
	yaml := `status:
  conditions:
    - type: Ready
      status: "True"
    - type: Initialized
      status: "True"
    - type: ContainersReady
      status: "True"`

	sections := parseYAMLSections(yaml)

	// Uncollapsed: list item fold lines should preserve the dash.
	visLines, _ := buildVisibleLines(yaml, sections, map[string]bool{})
	for _, line := range visLines {
		// Only check lines that are list items (contain "- type:").
		if strings.Contains(line, "type:") && strings.ContainsRune(line, '▾') {
			if !strings.Contains(line, "- type:") {
				t.Errorf("fold indicator should not replace dash on list item, got %q", line)
			}
		}
	}

	// Collapsed: fold indicator and dash should both be present.
	var firstListKey string
	for _, sec := range sections {
		if sec.listItem {
			firstListKey = sec.key
			break
		}
	}
	collapsed := map[string]bool{firstListKey: true}
	visLines, _ = buildVisibleLines(yaml, sections, collapsed)
	for _, line := range visLines {
		if strings.Contains(line, "type:") && strings.ContainsRune(line, '▸') {
			if !strings.Contains(line, "- type:") {
				t.Errorf("collapsed fold indicator should not replace dash, got %q", line)
			}
		}
	}

	// All 3 list items should still show dashes when first is collapsed.
	dashCount := 0
	for _, line := range visLines {
		if strings.Contains(line, "- type:") {
			dashCount++
		}
	}
	if dashCount != 3 {
		t.Errorf("all 3 list item dashes should be visible, got %d", dashCount)
	}
}

// TestListItemFoldBoundary verifies folding one list item doesn't swallow siblings.
func TestListItemFoldBoundary(t *testing.T) {
	yaml := `status:
  conditions:
    - type: Ready
      status: "True"
      reason: KubeletReady
    - type: Initialized
      status: "True"
      reason: PodInitialized
    - type: ContainersReady
      status: "True"
      reason: ContainersReady`

	sections := parseYAMLSections(yaml)

	var firstListKey string
	for _, sec := range sections {
		if sec.listItem {
			firstListKey = sec.key
			break
		}
	}

	collapsed := map[string]bool{firstListKey: true}
	visLines, _ := buildVisibleLines(yaml, sections, collapsed)

	// First item's content should be hidden.
	for _, line := range visLines {
		if strings.Contains(line, "KubeletReady") {
			t.Error("first list item's content should be hidden")
		}
	}
	// Second and third items should be fully visible.
	found := map[string]bool{}
	for _, line := range visLines {
		if strings.Contains(line, "Initialized") {
			found["Initialized"] = true
		}
		if strings.Contains(line, "PodInitialized") {
			found["PodInitialized"] = true
		}
		if strings.Contains(line, "ContainersReady") {
			found["ContainersReady"] = true
		}
	}
	for _, key := range []string{"Initialized", "PodInitialized", "ContainersReady"} {
		if !found[key] {
			t.Errorf("sibling content %q should be visible after folding first item", key)
		}
	}
}

// TestBlockHeaderListItemBoundary verifies that "- backend:" style sections
// (list item + block header) correctly stop at sibling list items.
func TestBlockHeaderListItemBoundary(t *testing.T) {
	// After indentYAMLListItems, this is what list-of-objects looks like.
	yaml := `spec:
  paths:
    - backend:
        service:
          name: nginx
    - backend:
        service:
          name: api`

	sections := parseYAMLSections(yaml)

	// Find backend sections (both are at indent 4).
	var backends []yamlSection
	for _, sec := range sections {
		if strings.HasSuffix(sec.key, "backend") {
			backends = append(backends, sec)
		}
	}
	if len(backends) < 2 {
		t.Fatalf("expected at least 2 backend sections, got %d: %+v", len(backends), sections)
	}

	// The first backend section must NOT extend into the second "- backend:".
	// Line 2: "    - backend:" (first), line 5: "    - backend:" (second).
	// endLine of first should be < 5 (before the second "- backend:" starts).
	if backends[0].endLine >= 5 {
		t.Errorf("first '- backend:' section endLine=%d should be < 5 (before sibling list item)", backends[0].endLine)
	}

	// Second backend should cover its own content only.
	if backends[1].startLine != 5 {
		t.Errorf("second backend startLine=%d, expected 5", backends[1].startLine)
	}
	if backends[1].endLine < 7 {
		t.Errorf("second backend endLine=%d should be >= 7 (includes name: api)", backends[1].endLine)
	}
}

// TestBlockHeaderListItemFoldVisibility verifies folding with unique list-item
// block headers doesn't swallow siblings.
func TestBlockHeaderListItemFoldVisibility(t *testing.T) {
	yaml := `spec:
  rules:
    - http:
        paths:
          - path: /
    - tcp:
        ports:
          - port: 80`

	sections := parseYAMLSections(yaml)

	// Find "http" section (it's "- http:" — a list item block header).
	var httpSec *yamlSection
	for i := range sections {
		if strings.HasSuffix(sections[i].key, "http") {
			httpSec = &sections[i]
			break
		}
	}
	if httpSec == nil {
		t.Fatal("http section not found")
	}

	// Fold the http section.
	collapsed := map[string]bool{httpSec.key: true}
	visLines, _ := buildVisibleLines(yaml, sections, collapsed)

	// The tcp sibling and its content should remain visible.
	foundTCP := false
	foundPort := false
	for _, line := range visLines {
		if strings.Contains(line, "tcp:") {
			foundTCP = true
		}
		if strings.Contains(line, "port: 80") {
			foundPort = true
		}
	}
	if !foundTCP {
		t.Error("folding '- http:' should not hide sibling '- tcp:'")
	}
	if !foundPort {
		t.Error("folding '- http:' should not hide sibling's content 'port: 80'")
	}
}

// TestListItemBlockHeaderSiblingKeys verifies that "- args:" sections don't
// swallow sibling keys (like "name:") within the same list item.
func TestListItemBlockHeaderSiblingKeys(t *testing.T) {
	// After indentYAMLListItems, a container list item with args + name looks like:
	yaml := `spec:
  containers:
    - args:
        - --leader-elect=true
        - --log-level=info
      name: controller
      image: controller:latest
    - name: sidecar
      image: sidecar:latest`

	sections := parseYAMLSections(yaml)

	// Find the "args" section (it's "- args:" — a list item block header).
	var argsSec *yamlSection
	for i := range sections {
		if strings.HasSuffix(sections[i].key, "args") {
			argsSec = &sections[i]
			break
		}
	}
	if argsSec == nil {
		t.Fatalf("args section not found. Sections: %+v", sections)
	}

	// "- args:" is at line 2. Its children are:
	//   line 3: "        - --leader-elect=true"
	//   line 4: "        - --log-level=info"
	// Sibling keys in the same list item:
	//   line 5: "      name: controller"
	//   line 6: "      image: controller:latest"
	// The args section should NOT include lines 5-6.
	if argsSec.endLine > 4 {
		t.Errorf("args section endLine=%d should be <= 4 (should not include sibling keys 'name:' and 'image:')",
			argsSec.endLine)
	}

	// Folding args should only hide its list children, not sibling keys.
	collapsed := map[string]bool{argsSec.key: true}
	visLines, _ := buildVisibleLines(yaml, sections, collapsed)

	foundName := false
	foundImage := false
	for _, line := range visLines {
		if strings.Contains(line, "name: controller") {
			foundName = true
		}
		if strings.Contains(line, "image: controller:latest") {
			foundImage = true
		}
	}
	if !foundName {
		t.Error("folding '- args:' should not hide sibling key 'name: controller'")
	}
	if !foundImage {
		t.Error("folding '- args:' should not hide sibling key 'image: controller:latest'")
	}

	// The args values themselves should be hidden.
	for _, line := range visLines {
		if strings.Contains(line, "--leader-elect") {
			t.Error("folding '- args:' should hide its content '--leader-elect=true'")
		}
	}
}

// TestSplitListItemBlockHeader verifies that lines which are BOTH a list item
// and a block header (e.g., "- args:") are split into two display lines:
// a dash line for folding the entire list element and a content line for
// folding just the block header's content.
func TestSplitListItemBlockHeader(t *testing.T) {
	yaml := `spec:
  containers:
    - args:
        - --leader-elect=true
        - --log-level=info
      name: controller
      image: controller:latest`

	sections := parseYAMLSections(yaml)

	// Should have both a listItem (#2) and a section (args) for line 2.
	var listItemSec, argsSec *yamlSection
	for i := range sections {
		if sections[i].startLine == 2 {
			if sections[i].listItem {
				listItemSec = &sections[i]
			} else {
				argsSec = &sections[i]
			}
		}
	}
	if listItemSec == nil {
		t.Fatal("expected listItem section for '- args:' line")
	}
	if argsSec == nil {
		t.Fatal("expected regular section for '- args:' line")
	}

	// Test 1: Uncollapsed - both dash and content lines visible.
	visLines, mapping := buildVisibleLines(yaml, sections, map[string]bool{})

	// Find the dash line (should contain just "-" without "args")
	dashIdx := -1
	contentIdx := -1
	for i, line := range visLines {
		if i < len(mapping) && mapping[i] == 2 {
			if dashIdx == -1 {
				dashIdx = i
			} else {
				contentIdx = i
			}
		}
		_ = line
	}
	if dashIdx == -1 || contentIdx == -1 {
		t.Fatalf("expected two visible lines for orig line 2, dash=%d content=%d", dashIdx, contentIdx)
	}
	if !strings.Contains(visLines[dashIdx], "-") || strings.Contains(visLines[dashIdx], "args") {
		t.Errorf("dash line should contain '-' but not 'args', got %q", visLines[dashIdx])
	}
	if !strings.Contains(visLines[contentIdx], "args:") {
		t.Errorf("content line should contain 'args:', got %q", visLines[contentIdx])
	}

	// Test 2: sectionForVisibleLine returns listItem for dash, section for content.
	dashSection := sectionForVisibleLine(dashIdx, mapping, sections)
	if dashSection != listItemSec.key {
		t.Errorf("dash line should target listItem %q, got %q", listItemSec.key, dashSection)
	}
	contentSection := sectionForVisibleLine(contentIdx, mapping, sections)
	if contentSection != argsSec.key {
		t.Errorf("content line should target section %q, got %q", argsSec.key, contentSection)
	}

	// Test 3: Folding listItem hides everything (args content + sibling keys).
	collapsed := map[string]bool{listItemSec.key: true}
	visLines, _ = buildVisibleLines(yaml, sections, collapsed)
	for _, line := range visLines {
		if strings.Contains(line, "args:") {
			t.Error("folding listItem should hide the content line 'args:'")
		}
		if strings.Contains(line, "--leader-elect") {
			t.Error("folding listItem should hide args content")
		}
		if strings.Contains(line, "name: controller") {
			t.Error("folding listItem should hide sibling key 'name:'")
		}
	}
	// Dash line should still be visible with collapsed indicator.
	foundDash := false
	for _, line := range visLines {
		if strings.ContainsRune(line, '▸') && strings.Contains(line, "-") && !strings.Contains(line, "args") {
			foundDash = true
		}
	}
	if !foundDash {
		t.Error("collapsed listItem dash line should show ▸ indicator")
	}

	// Test 4: Folding only section hides args content but not sibling keys.
	collapsed = map[string]bool{argsSec.key: true}
	visLines, _ = buildVisibleLines(yaml, sections, collapsed)
	foundName := false
	foundLeader := false
	for _, line := range visLines {
		if strings.Contains(line, "name: controller") {
			foundName = true
		}
		if strings.Contains(line, "--leader-elect") {
			foundLeader = true
		}
	}
	if !foundName {
		t.Error("folding section should not hide sibling key 'name: controller'")
	}
	if foundLeader {
		t.Error("folding section should hide args content '--leader-elect'")
	}
}

// TestVisibleLineCountWithSplit verifies visibleLineCount accounts for extra
// lines produced by the split display of list-item block headers.
func TestVisibleLineCountWithSplit(t *testing.T) {
	yaml := `spec:
  containers:
    - args:
        - --leader-elect=true
      name: controller`

	sections := parseYAMLSections(yaml)

	// Uncollapsed: 5 original lines + 1 extra from split = 6
	count := visibleLineCount(yaml, sections, map[string]bool{})
	if count != 6 {
		t.Errorf("expected 6 visible lines uncollapsed, got %d", count)
	}

	// Find the listItem for line 2
	var listKey string
	for _, sec := range sections {
		if sec.startLine == 2 && sec.listItem {
			listKey = sec.key
		}
	}

	// Collapse listItem: hides lines 3-4 (2 hidden). Content line not shown (collapsed).
	// = 5 - 2 + 0 = 3
	count = visibleLineCount(yaml, sections, map[string]bool{listKey: true})
	if count != 3 {
		t.Errorf("expected 3 visible lines with listItem collapsed, got %d", count)
	}
}

func TestIndentYAMLListItems(t *testing.T) {
	input := `apiVersion: v1
kind: Pod
spec:
  containers:
  - name: nginx
    image: nginx:latest
    ports:
    - containerPort: 80
  volumes:
  - name: data
    emptyDir: {}
metadata:
  labels:
    app: nginx`

	expected := `apiVersion: v1
kind: Pod
spec:
  containers:
    - name: nginx
      image: nginx:latest
      ports:
        - containerPort: 80
  volumes:
    - name: data
      emptyDir: {}
metadata:
  labels:
    app: nginx`

	result := indentYAMLListItems(input)
	if result != expected {
		t.Errorf("indentYAMLListItems mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, result)
	}
}
