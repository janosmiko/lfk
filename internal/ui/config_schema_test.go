package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// schemaDoc is the minimal shape of docs/config.schema.json needed to assert
// that the on-disk JSON Schema stays in sync with the Go config structs.
type schemaDoc struct {
	Properties  map[string]json.RawMessage `json:"properties"`
	Definitions map[string]struct {
		Properties map[string]json.RawMessage `json:"properties"`
	} `json:"definitions"`
}

func loadConfigSchema(t *testing.T) schemaDoc {
	t.Helper()
	// Test runs from internal/ui; the schema lives at the repo's docs/ dir.
	path := filepath.Join("..", "..", "docs", "config.schema.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read %s", path)

	var doc schemaDoc
	require.NoError(t, json.Unmarshal(data, &doc), "schema is not valid JSON")
	return doc
}

// jsonTagNames returns the json tag names (sans options like ",omitempty") of
// every field on the given struct type.
func jsonTagNames(t reflect.Type) []string {
	names := make([]string, 0, t.NumField())
	for f := range t.Fields() {
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// TestConfigSchemaTopLevelInSync guards against schema drift: every json field
// on configFile must have a matching property in the schema's top-level
// properties. When a new config key is added, this fails until the schema is
// updated too.
func TestConfigSchemaTopLevelInSync(t *testing.T) {
	doc := loadConfigSchema(t)
	for _, name := range jsonTagNames(reflect.TypeFor[configFile]()) {
		_, ok := doc.Properties[name]
		require.Truef(t, ok, "config field %q missing from docs/config.schema.json top-level properties", name)
	}
}

// TestConfigSchemaKeybindingsInSync guards the keybindings definition, the most
// error-prone list (80+ entries) to keep aligned by hand.
func TestConfigSchemaKeybindingsInSync(t *testing.T) {
	doc := loadConfigSchema(t)
	def, ok := doc.Definitions["keybindings"]
	require.True(t, ok, "schema missing definitions.keybindings")
	for _, name := range jsonTagNames(reflect.TypeFor[Keybindings]()) {
		_, ok := def.Properties[name]
		require.Truef(t, ok, "keybinding %q missing from schema definitions.keybindings.properties", name)
	}
}

// TestConfigExampleParses is the fifth forcing function over the config
// surface, next to the two schema-sync tests here and the two wiring tests in
// config_wiring_test.go. Nothing else reads docs/config-example.yaml, so a
// syntax error or a mistyped value in the documented example used to ship
// unnoticed.
//
// It unmarshals into configFile rather than map[string]any: that catches a
// documented key whose value no longer matches the field's type, which is the
// way the example actually goes stale. Unknown keys still pass, so this does
// not double up on the schema-sync tests above.
func TestConfigExampleParses(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "config-example.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read %s", path)

	var cfg configFile
	require.NoError(t, yaml.Unmarshal(data, &cfg), "%s does not parse as a config file", path)
}
