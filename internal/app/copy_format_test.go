package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestAvailableCopyFormats(t *testing.T) {
	tests := []struct {
		name  string
		level model.Level
		want  []CopyFormat
	}{
		{"Clusters", model.LevelClusters, []CopyFormat{CopyFormatTable}},
		{"ResourceTypes", model.LevelResourceTypes, []CopyFormat{CopyFormatTable}},
		{"Resources", model.LevelResources, []CopyFormat{CopyFormatYAML, CopyFormatJSON, CopyFormatTable}},
		{"Owned", model.LevelOwned, []CopyFormat{CopyFormatYAML, CopyFormatJSON, CopyFormatTable}},
		{"Containers", model.LevelContainers, []CopyFormat{CopyFormatYAML, CopyFormatJSON, CopyFormatTable}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := availableCopyFormats(tt.level)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCopyFormatLabelAndKey(t *testing.T) {
	assert.Equal(t, "YAML", CopyFormatYAML.Label())
	assert.Equal(t, "y", CopyFormatYAML.ShortcutKey())
	assert.Equal(t, "JSON", CopyFormatJSON.Label())
	assert.Equal(t, "J", CopyFormatJSON.ShortcutKey(),
		"JSON uses uppercase J so it does not collide with lowercase j cursor-down navigation")
	assert.Equal(t, "Table", CopyFormatTable.Label())
	assert.Equal(t, "t", CopyFormatTable.ShortcutKey())
}
