package model

import "testing"

func TestItem_Raw_RetainsObject(t *testing.T) {
	it := Item{}
	if it.Raw != nil {
		t.Fatalf("zero-value Item.Raw should be nil, got %v", it.Raw)
	}
	src := map[string]any{"metadata": map[string]any{"name": "x"}}
	it.Raw = src
	if it.Raw["metadata"].(map[string]any)["name"] != "x" {
		t.Fatalf("Item.Raw round-trip failed: %v", it.Raw)
	}
}
