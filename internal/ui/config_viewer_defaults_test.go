package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// snapshotViewerDefaultGlobals saves and restores the viewer-default globals so
// each test starts from known compiled defaults.
func snapshotViewerDefaultGlobals(t *testing.T) {
	t.Helper()
	prevYAML := ConfigYAMLViewerWrap
	prevDiffWrap := ConfigDiffViewerWrap
	prevDiffNums := ConfigDiffViewerLineNumbers
	prevDiffUnified := ConfigDiffViewerUnified
	prevDescribe := ConfigDescribeViewerWrap
	t.Cleanup(func() {
		ConfigYAMLViewerWrap = prevYAML
		ConfigDiffViewerWrap = prevDiffWrap
		ConfigDiffViewerLineNumbers = prevDiffNums
		ConfigDiffViewerUnified = prevDiffUnified
		ConfigDescribeViewerWrap = prevDescribe
	})
	ConfigYAMLViewerWrap = false
	ConfigDiffViewerWrap = false
	ConfigDiffViewerLineNumbers = true
	ConfigDiffViewerUnified = false
	ConfigDescribeViewerWrap = false
}

// TestViewerDefaults_GroupsApply verifies the yaml_viewer / diff_viewer /
// describe_viewer groups wire every field into its runtime global.
func TestViewerDefaults_GroupsApply(t *testing.T) {
	snapshotViewerDefaultGlobals(t)

	path := writeConfigFile(t, `yaml_viewer:
  wrap: true
diff_viewer:
  wrap: true
  line_numbers: false
  unified: true
describe_viewer:
  wrap: true
`)
	LoadConfig(path)

	assert.True(t, ConfigYAMLViewerWrap, "yaml_viewer.wrap")
	assert.True(t, ConfigDiffViewerWrap, "diff_viewer.wrap")
	assert.False(t, ConfigDiffViewerLineNumbers, "diff_viewer.line_numbers")
	assert.True(t, ConfigDiffViewerUnified, "diff_viewer.unified")
	assert.True(t, ConfigDescribeViewerWrap, "describe_viewer.wrap")
}

// TestViewerDefaults_OmittedKeysPreserveDefaults verifies a partial group leaves
// untouched globals at their compiled defaults.
func TestViewerDefaults_OmittedKeysPreserveDefaults(t *testing.T) {
	snapshotViewerDefaultGlobals(t)

	path := writeConfigFile(t, `diff_viewer:
  unified: true
`)
	LoadConfig(path)

	assert.True(t, ConfigDiffViewerUnified, "unified applied")
	assert.True(t, ConfigDiffViewerLineNumbers, "line_numbers default preserved")
	assert.False(t, ConfigDiffViewerWrap, "wrap default preserved")
	assert.False(t, ConfigYAMLViewerWrap, "yaml wrap untouched")
}
