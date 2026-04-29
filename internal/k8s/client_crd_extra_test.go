package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- extractCRDPrinterColumns: missing branches ---

func TestExtractCRDPrinterColumns_NonMapVersionEntry(t *testing.T) {
	// A version entry that is not a map should be skipped (line 148-149).
	spec := map[string]any{
		"versions": []any{
			"not-a-map",
			map[string]any{
				"name": "v1",
				"additionalPrinterColumns": []any{
					map[string]any{
						"name":     "Status",
						"type":     "string",
						"jsonPath": ".status.phase",
					},
				},
			},
		},
	}
	cols := extractCRDPrinterColumns(spec, "v1")
	assert.Len(t, cols, 1)
	assert.Equal(t, "Status", cols[0].Name)
}

func TestExtractCRDPrinterColumns_NonMapColumnEntry(t *testing.T) {
	// A column entry that is not a map should be skipped (line 164-165).
	spec := map[string]any{
		"versions": []any{
			map[string]any{
				"name": "v1",
				"additionalPrinterColumns": []any{
					"not-a-map",
					map[string]any{
						"name":     "Phase",
						"type":     "string",
						"jsonPath": ".status.phase",
					},
				},
			},
		},
	}
	cols := extractCRDPrinterColumns(spec, "v1")
	assert.Len(t, cols, 1)
	assert.Equal(t, "Phase", cols[0].Name)
}

func TestExtractCRDPrinterColumns_EmptyColumnsList(t *testing.T) {
	// An empty additionalPrinterColumns list should return nil.
	spec := map[string]any{
		"versions": []any{
			map[string]any{
				"name":                     "v1",
				"additionalPrinterColumns": []any{},
			},
		},
	}
	cols := extractCRDPrinterColumns(spec, "v1")
	assert.Nil(t, cols)
}

func TestExtractCRDPrinterColumns_CaseInsensitiveAgeSkip(t *testing.T) {
	// "age" (lowercase) should also be skipped due to EqualFold.
	spec := map[string]any{
		"versions": []any{
			map[string]any{
				"name": "v1",
				"additionalPrinterColumns": []any{
					map[string]any{
						"name":     "age",
						"type":     "date",
						"jsonPath": ".metadata.creationTimestamp",
					},
					map[string]any{
						"name":     "Ready",
						"type":     "string",
						"jsonPath": ".status.ready",
					},
				},
			},
		},
	}
	cols := extractCRDPrinterColumns(spec, "v1")
	assert.Len(t, cols, 1)
	assert.Equal(t, "Ready", cols[0].Name)
}
