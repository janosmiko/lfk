package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestSupportsBulkYAMLCopy_IncludesContainers(t *testing.T) {
	m := Model{}
	m.nav.Level = model.LevelContainers
	assert.True(t, m.supportsBulkYAMLCopy(),
		"LevelContainers must be in the bulk allowlist so Y → picker → YAML over multiple selected containers copies all of them rather than silently fetching one Pod's whole manifest")
}
