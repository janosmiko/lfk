package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActionsForKind_PodHasQuarantine(t *testing.T) {
	items := ActionsForKind("Pod")

	var found *ActionMenuItem
	for i := range items {
		if items[i].Label == ActionLabelQuarantine {
			found = &items[i]
			continue
		}
		assert.NotEqual(t, "Q", items[i].Key, "another Pod action already uses key Q: %q", items[i].Label)
	}
	if assert.NotNil(t, found, "Pod menu is missing the Quarantine action") {
		assert.Equal(t, "Q", found.Key)
	}
}
