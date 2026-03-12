package model

import (
	"fmt"
	"testing"
)

func TestDebugPrintResourceTypes(t *testing.T) {
	items := FlattenedResourceTypesFiltered(nil)
	lastCat := ""
	for i, item := range items {
		if item.Category != lastCat {
			if i > 0 {
				fmt.Println("  --- separator ---")
			}
			if item.Category != "" {
				fmt.Printf("  [HEADER: %s]\n", item.Category)
			}
			lastCat = item.Category
		}
		fmt.Printf("  %d: Cat=%-15s Kind=%-30s Name=%q Icon=%q\n", i, item.Category, item.Kind, item.Name, item.Icon)
	}
}
