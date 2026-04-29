package app

import (
	"slices"
	"strings"
	"testing"
)

func TestParseExplainOutput(t *testing.T) {
	input := `GROUP:      apps
KIND:       Deployment
VERSION:    v1

DESCRIPTION:
    Deployment enables declarative updates for Pods and ReplicaSets.

FIELDS:
  apiVersion   <string>
    APIVersion defines the versioned schema of this representation of an
    object.

  kind   <string>
    Kind is a string value representing the REST resource this object
    represents.

  metadata   <ObjectMeta>
    Standard object's metadata.

  spec   <DeploymentSpec>
    Specification of the desired behavior of the Deployment.

  status   <DeploymentStatus>
    Most recently observed status of the Deployment.
`

	desc, fields := parseExplainOutput(input, "")

	if desc == "" {
		t.Error("expected non-empty description")
	}

	if len(fields) == 0 {
		t.Fatal("expected fields to be parsed")
	}

	// Check we got the expected fields.
	expectedNames := []string{"apiVersion", "kind", "metadata", "spec", "status"}
	if len(fields) != len(expectedNames) {
		t.Errorf("expected %d fields, got %d", len(expectedNames), len(fields))
		for _, f := range fields {
			t.Logf("  field: %q type: %q desc: %q", f.Name, f.Type, f.Description)
		}
	}

	for i, name := range expectedNames {
		if i < len(fields) && fields[i].Name != name {
			t.Errorf("field %d: expected name %q, got %q", i, name, fields[i].Name)
		}
	}

	// Check types.
	if len(fields) >= 1 && fields[0].Type != "<string>" {
		t.Errorf("expected apiVersion type <string>, got %q", fields[0].Type)
	}
	if len(fields) >= 3 && fields[2].Type != "<ObjectMeta>" {
		t.Errorf("expected metadata type <ObjectMeta>, got %q", fields[2].Type)
	}

	// Check descriptions are not empty.
	for _, f := range fields {
		if f.Description == "" {
			t.Errorf("field %q has empty description", f.Name)
		}
	}
}

func TestParseExplainOutputRequiredFields(t *testing.T) {
	input := `KIND:     Deployment
VERSION:  v1

DESCRIPTION:
    Test.

FIELDS:
  selector   <Object> -required-
    Label selector for pods. Existing ReplicaSets whose pods are selected by
    this will be the ones affected by this deployment.

  template   <PodTemplateSpec> -required-
    Template describes the pods that will be created.
`

	_, fields := parseExplainOutput(input, "spec")

	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}

	if fields[0].Name != "selector" {
		t.Errorf("expected first field 'selector', got %q", fields[0].Name)
	}
	if fields[0].Type != "<Object>" {
		t.Errorf("expected type '<Object>', got %q", fields[0].Type)
	}
	if !fields[0].Required {
		t.Error("expected selector to be required")
	}
	if fields[0].Path != "spec.selector" {
		t.Errorf("expected path 'spec.selector', got %q", fields[0].Path)
	}

	// Description should include multi-line text.
	if !strings.Contains(fields[0].Description, "Label selector") {
		t.Errorf("expected description to contain 'Label selector', got %q", fields[0].Description)
	}

	// Second field should also be required.
	if fields[1].Name != "template" {
		t.Errorf("expected second field 'template', got %q", fields[1].Name)
	}
	if fields[1].Type != "<PodTemplateSpec>" {
		t.Errorf("expected type '<PodTemplateSpec>', got %q", fields[1].Type)
	}
	if !fields[1].Required {
		t.Error("expected template to be required")
	}
}

func TestParseExplainOutputEmpty(t *testing.T) {
	desc, fields := parseExplainOutput("", "")
	if desc != "" {
		t.Errorf("expected empty description, got %q", desc)
	}
	if len(fields) != 0 {
		t.Errorf("expected no fields, got %d", len(fields))
	}
}

// --- parseRecursiveExplainForSearch ---

func TestParseRecursiveExplainForSearch(t *testing.T) {
	explainOutput := `KIND:     Deployment
VERSION:  v1

RESOURCE: spec <DeploymentSpec>

DESCRIPTION:
    Specification of the desired behavior of the Deployment.

FIELDS:
    replicas	<integer>
    selector	<Object>
      matchLabels	<map[string]string>
    template	<PodTemplateSpec>
      metadata	<ObjectMeta>
        name	<string>
      spec	<PodSpec>
        containers	<[]Container>
          name	<string>
          ports	<[]ContainerPort>
            containerPort	<integer>
`

	t.Run("empty query returns all fields", func(t *testing.T) {
		results := parseRecursiveExplainForSearch(explainOutput, "")
		if len(results) == 0 {
			t.Fatal("expected fields, got none")
		}
		// Should contain top-level and nested fields.
		names := make([]string, 0, len(results))
		for _, r := range results {
			names = append(names, r.Name)
		}
		if !containsString(names, "replicas") {
			t.Error("expected 'replicas' in results")
		}
		if !containsString(names, "containerPort") {
			t.Error("expected 'containerPort' in results")
		}
	})

	t.Run("query filters by field name", func(t *testing.T) {
		results := parseRecursiveExplainForSearch(explainOutput, "container")
		for _, r := range results {
			if !strings.Contains(strings.ToLower(r.Name), "container") {
				t.Errorf("unexpected result %q for query 'container'", r.Name)
			}
		}
		if len(results) == 0 {
			t.Error("expected results for query 'container'")
		}
	})

	t.Run("builds correct paths", func(t *testing.T) {
		results := parseRecursiveExplainForSearch(explainOutput, "containerPort")
		if len(results) == 0 {
			t.Fatal("expected results for query 'containerPort'")
		}
		found := false
		for _, r := range results {
			if r.Name == "containerPort" {
				if !strings.Contains(r.Path, "containers") || !strings.Contains(r.Path, "containerPort") {
					t.Errorf("expected path to include 'containers.containerPort', got %q", r.Path)
				}
				found = true
			}
		}
		if !found {
			t.Error("expected to find 'containerPort' in results")
		}
	})

	t.Run("case insensitive query", func(t *testing.T) {
		results := parseRecursiveExplainForSearch(explainOutput, "REPLICAS")
		if len(results) == 0 {
			t.Error("expected results for case-insensitive query 'REPLICAS'")
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		results := parseRecursiveExplainForSearch(explainOutput, "nonexistentfield")
		if len(results) != 0 {
			t.Errorf("expected no results, got %d", len(results))
		}
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		results := parseRecursiveExplainForSearch("", "anything")
		if len(results) != 0 {
			t.Errorf("expected no results, got %d", len(results))
		}
	})
}

func containsString(slice []string, s string) bool {
	return slices.Contains(slice, s)
}

func TestParseExplainOutputWithPath(t *testing.T) {
	input := `GROUP:      apps
KIND:       Deployment
VERSION:    v1

DESCRIPTION:
    DeploymentSpec is the specification of the desired behavior of the
    Deployment.

FIELDS:
  minReadySeconds   <integer>
    Minimum number of seconds for which a newly created pod should be ready
    without any of its container crashing.

  replicas   <integer>
    Number of desired pods.

  selector   <Object> -required-
    Label selector for pods.

  template   <PodTemplateSpec> -required-
    Template describes the pods that will be created.
`

	desc, fields := parseExplainOutput(input, "spec")

	if desc == "" {
		t.Error("expected non-empty description")
	}

	if len(fields) == 0 {
		t.Fatal("expected fields to be parsed")
	}

	// Check path includes basePath.
	for _, f := range fields {
		if f.Path == "" || f.Path[:4] != "spec" {
			t.Errorf("field %q has path %q, expected to start with 'spec'", f.Name, f.Path)
		}
	}
}
