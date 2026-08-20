package ui

import (
	"encoding/json"
	"testing"
)

// TestDashKeyAgainstRealJSON makes sure the JSONPath dialect we use
// handles a dashed label key when the data path is the same as a real
// unstructured.Unstructured: a JSON object decoded into map[string]any.
func TestDashKeyAgainstRealJSON(t *testing.T) {
	raw := []byte(`{
		"metadata": {
			"name": "web-7c4d8b5-abc12",
			"labels": {
				"app": "web",
				"git-sha": "deadbeef"
			}
		}
	}`)
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	got := evalJSONPath(".metadata.labels.git-sha", obj)
	if got != "deadbeef" {
		t.Fatalf("expected deadbeef, got %q", got)
	}

	gotBracket := evalJSONPath(`.metadata.labels['git-sha']`, obj)
	if gotBracket != "deadbeef" {
		t.Fatalf("bracket form should also work; got %q", gotBracket)
	}
}
